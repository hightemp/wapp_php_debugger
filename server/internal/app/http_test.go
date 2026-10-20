package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/libclient/libtest"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

const libToken = "0123456789abcdef0123456789abcdef"

func call(t *testing.T, core *Core, method, path string, body any) (int, string) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	request := httptest.NewRequest(method, path, &payload)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	core.Handler().ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

// A project behind the library HTTP mode: added through the API, its logs come by polling.
func TestImportsTheLogsOfAProjectOverHTTP(t *testing.T) {
	lib := libtest.Start(t, libToken, map[string][]byte{"1792400000001": fixture(t, "web-get.log")})
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	core, err := Start(ctx, Config{DBPath: filepath.Join(dir, "viewer.db"), RemoteScanInterval: 30 * time.Millisecond, HTTPClient: lib.Client()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); core.Close() })

	status, body := call(t, core, http.MethodPost, "/api/v1/servers", map[string]any{
		"name": "hosting", "kind": "http", "config": map[string]any{"base_url": lib.URL}, "token": libToken,
	})
	if status != http.StatusCreated {
		t.Fatalf("create server: %d %s", status, body)
	}
	var server struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &server)
	secrets, err := os.ReadFile(filepath.Join(dir, "secrets.json"))
	if err != nil || !strings.Contains(string(secrets), libToken) {
		t.Fatalf("the token is not in the secrets file: %s, %v", secrets, err)
	}
	if info, _ := os.Stat(filepath.Join(dir, "secrets.json")); info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets mode %v", info.Mode())
	}

	status, body = call(t, core, http.MethodPost, "/api/v1/projects", map[string]any{
		"server_id": server.ID, "name": "shop", "root_path": "/home/u/shop", "logs_dir": "/",
	})
	if status != http.StatusCreated {
		t.Fatalf("create project: %d %s", status, body)
	}
	waitFor(t, func() bool {
		_, list := call(t, core, http.MethodGet, "/api/v1/requests", nil)
		return strings.Contains(list, `"request_key":"1792400000001"`)
	})
	waitFor(t, func() bool {
		_, got := call(t, core, http.MethodGet, fmt.Sprintf("/api/v1/servers/%d", server.ID), nil)
		return strings.Contains(got, `"state":"online"`)
	})

	status, body = call(t, core, http.MethodPost, fmt.Sprintf("/api/v1/servers/%d/check", server.ID), nil)
	if status != http.StatusOK || !strings.Contains(body, `"ok":true`) || !strings.Contains(body, "2.0.0") {
		t.Fatalf("check: %d %s", status, body)
	}

	if status, _ := call(t, core, http.MethodDelete, fmt.Sprintf("/api/v1/servers/%d", server.ID), nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if secrets, _ := os.ReadFile(filepath.Join(dir, "secrets.json")); strings.Contains(string(secrets), libToken) {
		t.Fatalf("the token outlived its server: %s", secrets)
	}
}

func checkSteps(check []stepView) map[string]stepView {
	steps := map[string]stepView{}
	for _, step := range check {
		steps[step.Name] = step
	}
	return steps
}

type stepView struct {
	Name     string
	Ok       bool
	Optional bool
	Detail   string
}

func httpCheck(t *testing.T, client *http.Client, baseURL, token string) (bool, map[string]stepView) {
	t.Helper()
	runtime := newHTTPRuntime(filepath.Join(t.TempDir(), "secrets.json"), client, nil)
	if token != "" {
		if err := runtime.SetToken(5, token); err != nil {
			t.Fatal(err)
		}
	}
	check, err := runtime.Check(context.Background(), store.Server{ID: 5, Kind: "http", Config: fmt.Sprintf(`{"base_url":%q}`, baseURL)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var steps []stepView
	for _, step := range check.Steps {
		view := stepView{Name: step.Name, Ok: step.Ok}
		if step.Optional != nil {
			view.Optional = *step.Optional
		}
		if step.Detail != nil {
			view.Detail = *step.Detail
		}
		steps = append(steps, view)
	}
	return check.Ok, checkSteps(steps)
}

func TestHTTPCheck(t *testing.T) {
	lib := libtest.Start(t, libToken, nil)

	ok, steps := httpCheck(t, lib.Client(), lib.URL, libToken)
	if !ok || !steps["tls"].Ok || !strings.Contains(steps["library"].Detail, "2.0.0") || !steps["logs_dir_writable"].Ok || steps["php"].Detail != "PHP 7.4.33" {
		t.Fatalf("ok=%v steps=%+v", ok, steps)
	}

	ok, steps = httpCheck(t, lib.Client(), lib.URL, strings.Repeat("w", 32))
	if ok || steps["library"].Ok || !strings.Contains(steps["library"].Detail, "token") {
		t.Fatalf("wrong token: ok=%v steps=%+v", ok, steps)
	}

	ok, steps = httpCheck(t, lib.Client(), lib.URL, "")
	if ok || steps["config"].Ok {
		t.Fatalf("no token: ok=%v steps=%+v", ok, steps)
	}

	plain := httptest.NewServer(lib.Config.Handler)
	t.Cleanup(plain.Close)
	ok, steps = httpCheck(t, plain.Client(), plain.URL, libToken)
	if !ok || steps["tls"].Ok || !steps["tls"].Optional || !steps["library"].Ok {
		t.Fatalf("plain http is a warning: ok=%v steps=%+v", ok, steps)
	}
}

func TestHTTPRuntimeTracksTheClockOfTheServer(t *testing.T) {
	lib := libtest.Start(t, libToken, nil)
	lib.ClockAhead.Store(int64(-20 * time.Second))
	var published []sshconn.Status
	runtime := newHTTPRuntime(filepath.Join(t.TempDir(), "secrets.json"), lib.Client(), func(_ int64, status sshconn.Status) {
		published = append(published, status)
	})
	if err := runtime.SetToken(5, libToken); err != nil {
		t.Fatal(err)
	}
	src, err := runtime.source(store.Server{ID: 5, Kind: "http", Config: fmt.Sprintf(`{"base_url":%q}`, lib.URL)})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := src.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	status := runtime.Status(5)
	if status.ClockOffset == nil || *status.ClockOffset > -19*time.Second || *status.ClockOffset < -21*time.Second {
		t.Fatalf("status = %+v", status)
	}
	if len(published) > 2 {
		t.Fatalf("every poll published a status: %d", len(published))
	}
}

func TestDiscoversProjectsOnThisMachine(t *testing.T) {
	www := t.TempDir()
	if err := os.MkdirAll(filepath.Join(www, "shop", "tmp", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(www, "shop", "dws_debugger.json"), []byte(`{"enabled": true}`), 0o644)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	core, err := Start(ctx, Config{DBPath: filepath.Join(dir, "viewer.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); core.Close() })

	_, list := call(t, core, http.MethodGet, "/api/v1/servers", nil)
	var servers struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal([]byte(list), &servers)
	status, body := call(t, core, http.MethodPost, fmt.Sprintf("/api/v1/servers/%d/discover", servers.Items[0].ID), map[string]any{"roots": []string{www}})
	if status != http.StatusOK || !strings.Contains(body, `"logs_dir":"`+filepath.Join(www, "shop", "tmp", "logs")+`"`) {
		t.Fatalf("discover: %d %s", status, body)
	}
}
