package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

type fakeSSH struct {
	mu        sync.Mutex
	statuses  map[int64]sshconn.Status
	trusted   []string
	forgotten []int64
	check     ServerCheck
	trustErr  error
}

func (f *fakeSSH) Status(serverID int64) sshconn.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	if status, ok := f.statuses[serverID]; ok {
		return status
	}
	return sshconn.Status{State: sshconn.StateUnknown}
}

func (f *fakeSSH) Check(context.Context, store.Server) (ServerCheck, error) { return f.check, nil }

func (f *fakeSSH) TrustHostKey(_ context.Context, server store.Server, fingerprint string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trusted = append(f.trusted, fmt.Sprintf("%d %s", server.ID, fingerprint))
	return f.trustErr
}

func (f *fakeSSH) Forget(serverID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, serverID)
}

type serversEnv struct {
	projectsEnv
	ssh *fakeSSH
}

func newServersEnv(t *testing.T) serversEnv {
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
	ssh := &fakeSSH{statuses: map[int64]sshconn.Status{}}
	server := httptest.NewServer(NewHandler(Deps{Store: s, Broker: events.NewBroker(4), Projects: runtime, SSH: ssh, UI: fstest.MapFS{"index.html": {}}}))
	t.Cleanup(server.Close)
	return serversEnv{projectsEnv: projectsEnv{server: server, store: s, runtime: runtime, local: local}, ssh: ssh}
}

func (e serversEnv) createSSHServer(t *testing.T, name string) Server {
	t.Helper()
	response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/servers", map[string]any{
		"name": name, "kind": "ssh", "environment": "stage", "config": map[string]any{"host": "stage-web", "user": "deploy"},
		"permissions": map[string]any{"read_only": true, "allow_delete": false, "allow_control": false},
	})
	var server Server
	if response.StatusCode != http.StatusCreated || json.Unmarshal([]byte(body), &server) != nil {
		t.Fatalf("create: %d %s", response.StatusCode, body)
	}
	return server
}

func TestCreateAnSSHServer(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	if server.Kind != "ssh" || server.Environment != "stage" || *server.Config.Host != "stage-web" || !server.Permissions.ReadOnly || server.Connection.State != Unknown {
		t.Fatalf("server = %+v", server)
	}

	for _, c := range []struct {
		body   map[string]any
		status int
		detail string
	}{
		{map[string]any{"name": "x", "kind": "ssh", "config": map[string]any{}}, 400, "host"},
		{map[string]any{"name": "x", "kind": "local"}, 400, "local server"},
		{map[string]any{"name": "stage", "kind": "ssh", "config": map[string]any{"host": "h"}}, 409, ""},
	} {
		response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/servers", c.body)
		if response.StatusCode != c.status || !strings.Contains(body, c.detail) {
			t.Errorf("%v: %d %s", c.body, response.StatusCode, body)
		}
	}
}

func TestServersShowTheirConnection(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	e.ssh.statuses[server.Id] = sshconn.Status{State: sshconn.StateOffline, Error: "connection refused"}

	_, body := get(t, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, server.Id))
	var got Server
	_ = json.Unmarshal([]byte(body), &got)
	if got.Connection.State != Offline || got.Connection.Error == nil || *got.Connection.Error != "connection refused" {
		t.Fatalf("connection = %+v", got.Connection)
	}
	_, list := get(t, e.server.URL+"/api/v1/servers")
	if !strings.Contains(list, `"state":"online"`) || !strings.Contains(list, `"state":"offline"`) {
		t.Fatalf("the local server is online, the SSH one offline: %s", list)
	}
}

func TestCheckServers(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	fingerprint := "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"
	e.ssh.check = ServerCheck{Ok: false, Steps: []ServerCheckStep{{Name: "connect", Ok: false}}, HostKeyFingerprint: &fingerprint}

	_, body := send(t, http.MethodPost, fmt.Sprintf("%s/api/v1/servers/%d/check", e.server.URL, server.Id), nil)
	if !strings.Contains(body, fingerprint) || !strings.Contains(body, `"ok":false`) {
		t.Fatalf("check ssh: %s", body)
	}
	_, body = send(t, http.MethodPost, fmt.Sprintf("%s/api/v1/servers/%d/check", e.server.URL, e.local), nil)
	if !strings.Contains(body, `"ok":true`) {
		t.Fatalf("check local: %s", body)
	}
}

func TestTrustAHostKey(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	url := fmt.Sprintf("%s/api/v1/servers/%d/host-key", e.server.URL, server.Id)
	fingerprint := "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"

	if response, body := send(t, http.MethodPost, url, map[string]any{"fingerprint": fingerprint}); response.StatusCode != http.StatusNoContent {
		t.Fatalf("trust: %d %s", response.StatusCode, body)
	}
	if len(e.ssh.trusted) != 1 || e.ssh.trusted[0] != fmt.Sprintf("%d %s", server.Id, fingerprint) {
		t.Fatalf("trusted %v", e.ssh.trusted)
	}

	e.ssh.trustErr = fmt.Errorf("%w: the server now presents SHA256:other", ErrHostKeyMismatch)
	if response, body := send(t, http.MethodPost, url, map[string]any{"fingerprint": fingerprint}); response.StatusCode != http.StatusConflict || !strings.Contains(body, "SHA256:other") {
		t.Fatalf("mismatch: %d %s", response.StatusCode, body)
	}
	if response, _ := send(t, http.MethodPost, url, map[string]any{"fingerprint": "md5:aa"}); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad fingerprint: %d", response.StatusCode)
	}
	local := fmt.Sprintf("%s/api/v1/servers/%d/host-key", e.server.URL, e.local)
	if response, _ := send(t, http.MethodPost, local, map[string]any{"fingerprint": fingerprint}); response.StatusCode != http.StatusBadRequest {
		t.Fatalf("local server: %d", response.StatusCode)
	}
}

func TestDeleteAServer(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	response, body := send(t, http.MethodPost, e.server.URL+"/api/v1/projects", map[string]any{
		"server_id": server.Id, "name": "shop", "root_path": "/var/www/shop", "logs_dir": "/var/www/shop/tmp/logs",
	})
	var project Project
	if response.StatusCode != http.StatusCreated || json.Unmarshal([]byte(body), &project) != nil {
		t.Fatalf("project on the ssh server: %d %s", response.StatusCode, body)
	}

	if response, body := send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, server.Id), nil); response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d %s", response.StatusCode, body)
	}
	if len(e.runtime.removed) != 1 || e.runtime.removed[0] != project.Id || len(e.ssh.forgotten) != 1 {
		t.Fatalf("removed %v, forgotten %v", e.runtime.removed, e.ssh.forgotten)
	}
	if response, _ := get(t, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, server.Id)); response.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete: %d", response.StatusCode)
	}
	if response, _ := send(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, e.local), nil); response.StatusCode != http.StatusConflict {
		t.Fatalf("delete local: %d", response.StatusCode)
	}
}

func TestChangingAnSSHServerReconnectsItsProjects(t *testing.T) {
	e := newServersEnv(t)
	server := e.createSSHServer(t, "stage")
	_, _ = send(t, http.MethodPost, e.server.URL+"/api/v1/projects", map[string]any{
		"server_id": server.Id, "name": "shop", "root_path": "/var/www/shop", "logs_dir": "/var/www/shop/tmp/logs",
	})
	added := len(e.runtime.added)

	response, body := send(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/servers/%d", e.server.URL, server.Id), map[string]any{"config": map[string]any{"host": "stage-web-2"}})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d %s", response.StatusCode, body)
	}
	if len(e.ssh.forgotten) != 1 || len(e.runtime.added) != added+1 {
		t.Fatalf("forgotten %v, restarts %d", e.ssh.forgotten, len(e.runtime.added)-added)
	}
}
