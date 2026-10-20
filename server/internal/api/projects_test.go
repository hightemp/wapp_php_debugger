package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// fakeRuntime records what the API asks the import runtime to do.
type fakeRuntime struct {
	mu      sync.Mutex
	added   []int64
	removed []int64
	sources map[int64]*source.Memory
}

func (f *fakeRuntime) Add(project store.Project) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, project.ID)
	if f.sources[project.ID] == nil {
		f.sources[project.ID] = source.NewMemory(map[string][]byte{"1792400000000": []byte("x"), "1792400000001": []byte("y")})
	}
}

func (f *fakeRuntime) Remove(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	delete(f.sources, id)
}

func (f *fakeRuntime) Source(id int64) (source.Source, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	src, ok := f.sources[id]
	return src, ok
}

type projectsEnv struct {
	server  *httptest.Server
	store   *store.Store
	runtime *fakeRuntime
	local   int64
}

func newProjectsEnv(t *testing.T) projectsEnv {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	local, err := s.EnsureLocalServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime := &fakeRuntime{sources: map[int64]*source.Memory{}}
	server := httptest.NewServer(NewHandler(Deps{Store: s, Broker: events.NewBroker(4), Projects: runtime, UI: fstest.MapFS{"index.html": {}}}))
	t.Cleanup(server.Close)
	return projectsEnv{server: server, store: s, runtime: runtime, local: local}
}

func send(t *testing.T, method, url string, body any) (*http.Response, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	request, _ := http.NewRequest(method, url, reader)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(response.Body)
	return response, string(data)
}

func (e projectsEnv) createProject(t *testing.T, name string) Project {
	t.Helper()
	response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/projects", map[string]any{
		"server_id": e.local, "name": name, "root_path": "/var/www/" + name, "logs_dir": "/var/www/" + name + "/tmp/logs",
	})
	var project Project
	if response.StatusCode != http.StatusCreated || json.Unmarshal([]byte(body), &project) != nil {
		t.Fatalf("create: %d %s", response.StatusCode, body)
	}
	return project
}

func TestListAndGetServers(t *testing.T) {
	e := newProjectsEnv(t)
	_, body := get(t, e.server.URL+"/api/v1/servers")
	var list struct{ Items []Server }
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("servers = %s", body)
	}
	local := list.Items[0]
	if local.Kind != ServerKindLocal || local.Connection.State != Online || !local.Permissions.AllowDelete || local.Permissions.ReadOnly {
		t.Fatalf("local = %s", body)
	}
	response, body := get(t, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, local.Id))
	if response.StatusCode != 200 {
		t.Fatalf("get: %d %s", response.StatusCode, body)
	}
	response, body = get(t, e.server.URL+"/api/v1/servers/999")
	assertProblem(t, response, body, 404)
}

func TestCreateProjectStartsItsImport(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	if project.ConfigPath != "/var/www/shop/dws_debugger.json" || project.Editor != EditorVscode || project.PendingFiles != 0 {
		t.Fatalf("project = %+v", project)
	}
	if len(e.runtime.added) != 1 || e.runtime.added[0] != project.Id {
		t.Fatalf("runtime.added = %v", e.runtime.added)
	}

	_, body := get(t, e.server.URL+"/api/v1/projects")
	if !bytes.Contains([]byte(body), []byte(`"name":"shop"`)) {
		t.Fatalf("list = %s", body)
	}

	response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/projects", map[string]any{
		"server_id": e.local, "name": "shop", "root_path": "/x", "logs_dir": "/x/logs",
	})
	assertProblem(t, response, body, 409)
}

func TestCreateProjectValidates(t *testing.T) {
	e := newProjectsEnv(t)
	cases := map[string]map[string]any{
		"relative logs dir": {"server_id": e.local, "name": "a", "root_path": "/a", "logs_dir": "logs"},
		"relative root":     {"server_id": e.local, "name": "a", "root_path": "a", "logs_dir": "/a/logs"},
		"empty name":        {"server_id": e.local, "name": "", "root_path": "/a", "logs_dir": "/a/logs"},
		"unknown editor":    {"server_id": e.local, "name": "a", "root_path": "/a", "logs_dir": "/a/logs", "editor": "notepad"},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/projects", input)
			assertProblem(t, response, body, 400)
		})
	}
	response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/projects", map[string]any{"server_id": 999, "name": "a", "root_path": "/a", "logs_dir": "/a/logs"})
	assertProblem(t, response, body, 404)
}

func TestUpdateProjectRestartsItsImport(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	response, body := send(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/projects/%d", e.server.URL, project.Id), map[string]any{
		"local_path": "/home/dev/shop", "editor": "phpstorm",
	})
	var updated Project
	if response.StatusCode != 200 || json.Unmarshal([]byte(body), &updated) != nil || *updated.LocalPath != "/home/dev/shop" || updated.Editor != EditorPhpstorm {
		t.Fatalf("update: %d %s", response.StatusCode, body)
	}
	if len(e.runtime.added) != 2 {
		t.Fatalf("the import restarts with the new settings: added = %v", e.runtime.added)
	}
	response, body = send(t, http.MethodPatch, e.server.URL+"/api/v1/projects/999", map[string]any{"name": "x"})
	assertProblem(t, response, body, 404)
}

func TestDeleteProjectStopsItsImport(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	response, _ := send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/projects/%d", e.server.URL, project.Id), nil)
	if response.StatusCode != http.StatusNoContent || len(e.runtime.removed) != 1 {
		t.Fatalf("delete: %d removed=%v", response.StatusCode, e.runtime.removed)
	}
	response, body := get(t, fmt.Sprintf("%s/api/v1/projects/%d", e.server.URL, project.Id))
	assertProblem(t, response, body, 404)
}

func TestSwitchTheDebugger(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	url := fmt.Sprintf("%s/api/v1/projects/%d/debugger", e.server.URL, project.Id)

	_, body := get(t, url)
	if body != "{\"enabled\":false}\n" {
		t.Fatalf("initial = %s", body)
	}
	response, body := send(t, http.MethodPut, url, map[string]bool{"enabled": true})
	if response.StatusCode != 200 || body != "{\"enabled\":true}\n" {
		t.Fatalf("switch: %d %s", response.StatusCode, body)
	}
	if enabled, _ := e.runtime.sources[project.Id].Enabled(context.Background()); !enabled {
		t.Fatal("the switch goes to the project source")
	}
}

func TestClearProjectLogs(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	response, body := send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/projects/%d/logs", e.server.URL, project.Id), nil)
	if response.StatusCode != 200 || body != "{\"removed_files\":2}\n" {
		t.Fatalf("clear: %d %s", response.StatusCode, body)
	}
	if files, _ := e.runtime.sources[project.Id].List(context.Background()); len(files) != 0 {
		t.Fatalf("files left: %v", files)
	}
}

func TestServerPermissionsGuardChanges(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	server := fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, e.local)
	response, body := send(t, http.MethodPatch, server, map[string]any{
		"permissions": map[string]bool{"read_only": false, "allow_delete": false, "allow_control": false},
	})
	if response.StatusCode != 200 {
		t.Fatalf("patch server: %d %s", response.StatusCode, body)
	}

	response, body = send(t, http.MethodPut, fmt.Sprintf("%s/api/v1/projects/%d/debugger", e.server.URL, project.Id), map[string]bool{"enabled": true})
	assertProblem(t, response, body, 403)
	response, body = send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/projects/%d/logs", e.server.URL, project.Id), nil)
	assertProblem(t, response, body, 403)

	send(t, http.MethodPatch, server, map[string]any{"permissions": map[string]bool{"read_only": true, "allow_delete": true, "allow_control": true}})
	response, body = send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/projects/%d/logs", e.server.URL, project.Id), nil)
	assertProblem(t, response, body, 403)
}

func TestProjectRetentionIsSetAndCleared(t *testing.T) {
	e := newProjectsEnv(t)
	project := e.createProject(t, "shop")
	url := fmt.Sprintf("%s/api/v1/projects/%d", e.server.URL, project.Id)

	_, body := send(t, http.MethodPatch, url, map[string]any{"retention": map[string]any{"days": 14, "max_bytes": 1048576}})
	if !strings.Contains(body, `"retention":{"days":14,"max_bytes":1048576}`) {
		t.Fatalf("set: %s", body)
	}
	_, body = send(t, http.MethodPatch, url, map[string]any{"name": "shop2"})
	if !strings.Contains(body, `"days":14`) {
		t.Fatalf("a patch without retention keeps it: %s", body)
	}
	_, body = send(t, http.MethodPatch, url, map[string]any{"retention": map[string]any{"days": nil, "max_bytes": 2097152}})
	if !strings.Contains(body, `"retention":{"max_bytes":2097152}`) {
		t.Fatalf("days cleared: %s", body)
	}
}
