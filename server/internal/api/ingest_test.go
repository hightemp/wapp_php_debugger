package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func ingestEnv(t *testing.T) (*httptest.Server, *store.Store, *events.Broker, int64) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	sum := sha256.Sum256([]byte("push-token-of-the-shop-project-0123456789"))
	hash := hex.EncodeToString(sum[:])
	if err := s.SetIngestToken(ctx, ids.ProjectID, &hash); err != nil {
		t.Fatal(err)
	}
	broker := events.NewBroker(16)
	client := fstest.MapFS{"debugger.js": {Data: []byte("(function(){})();")}, "index.html": {Data: []byte("ui")}}
	server := httptest.NewServer(NewIngestHandler(IngestDeps{Store: s, Broker: broker, UI: client}))
	t.Cleanup(server.Close)
	return server, s, broker, ids.ProjectID
}

func push(t *testing.T, url string, header map[string]string, body []byte) (*http.Response, string) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/x-ndjson")
	for name, value := range header {
		request.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(response.Body)
	return response, string(data)
}

func TestIngestTakesEventsOfTheTokensProject(t *testing.T) {
	server, s, broker, projectID := ingestEnv(t)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(specPath(t)), "fixtures/v2/web-request.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	stream := broker.Subscribe(context.Background(), 0)
	auth := map[string]string{"Authorization": "Bearer push-token-of-the-shop-project-0123456789"}

	response, body := push(t, server.URL+"/ingest/v2", auth, data)
	if response.StatusCode != http.StatusOK || body != `{"created":1,"updated":0,"rejected":0}`+"\n" {
		t.Fatalf("push: %d %s", response.StatusCode, body)
	}
	if response.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("the JS client of a page needs CORS")
	}
	select {
	case event := <-stream:
		if event.Name != "request_created" || !strings.Contains(string(event.Data), `"project_id":`) {
			t.Fatalf("event %s %s", event.Name, event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no request_created after a push")
	}
	page, _ := s.ListRequests(context.Background(), store.RequestFilter{ProjectID: projectID, Limit: 10})
	if len(page.Items) != 1 {
		t.Fatalf("%d requests", len(page.Items))
	}

	// The library sends its token in the header it uses for the HTTP mode too
	response, body = push(t, server.URL+"/ingest/v2", map[string]string{"X-Debugger-Token": "push-token-of-the-shop-project-0123456789"}, data)
	if response.StatusCode != http.StatusOK || !strings.Contains(body, `"created":0`) {
		t.Fatalf("again: %d %s", response.StatusCode, body)
	}
}

// A page cannot set headers on a beacon: its token comes in the query, the body as text/plain.
func TestIngestTakesTheTokenOfAPage(t *testing.T) {
	server, _, _, _ := ingestEnv(t)
	data, _ := os.ReadFile(filepath.Join(filepath.Dir(specPath(t)), "fixtures/v2/js-page.jsonl"))
	response, err := http.Post(server.URL+"/ingest/v2?token=push-token-of-the-shop-project-0123456789", "text/plain;charset=UTF-8", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("beacon: %d", response.StatusCode)
	}

	script, err := http.Get(server.URL + "/ingest/debugger.js")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(script.Body)
	_ = script.Body.Close()
	if script.StatusCode != http.StatusOK || !strings.HasPrefix(script.Header.Get("Content-Type"), "text/javascript") || string(body) != "(function(){})();" {
		t.Fatalf("debugger.js: %d %s %q", script.StatusCode, script.Header.Get("Content-Type"), body)
	}
}

func TestIngestRefuses(t *testing.T) {
	server, _, _, _ := ingestEnv(t)
	for name, header := range map[string]map[string]string{
		"no token":    {},
		"wrong token": {"Authorization": "Bearer nope"},
	} {
		response, body := push(t, server.URL+"/ingest/v2", header, []byte("{}\n"))
		if response.StatusCode != http.StatusUnauthorized || !strings.Contains(response.Header.Get("Content-Type"), "problem+json") {
			t.Errorf("%s: %d %s", name, response.StatusCode, body)
		}
	}
	auth := map[string]string{"Authorization": "Bearer push-token-of-the-shop-project-0123456789"}
	response, _ := push(t, server.URL+"/ingest/v2", auth, bytes.Repeat([]byte("x"), 9<<20))
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("a big batch: %d", response.StatusCode)
	}
	// Only the receiver lives here: not the viewer API, not the UI
	for _, path := range []string{"/api/v1/projects", "/"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, response.StatusCode)
		}
	}

	preflight, _ := http.NewRequest(http.MethodOptions, server.URL+"/ingest/v2", nil)
	preflight.Header.Set("Origin", "https://shop.test")
	preflight.Header.Set("Access-Control-Request-Method", "POST")
	answer, err := http.DefaultClient.Do(preflight)
	if err != nil {
		t.Fatal(err)
	}
	_ = answer.Body.Close()
	if answer.StatusCode != http.StatusNoContent || !strings.Contains(answer.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight: %d %v", answer.StatusCode, answer.Header)
	}
}

func TestIngestTokenOfAProject(t *testing.T) {
	e := newTestEnv(t)
	_, body := get(t, e.server.URL+"/api/v1/projects")
	if !strings.Contains(body, `"ingest_enabled":false`) {
		t.Fatalf("projects: %s", body)
	}
	var projects struct {
		Items []Project `json:"items"`
	}
	_ = jsonUnmarshal(body, &projects)
	url := e.server.URL + "/api/v1/projects/" + itoa(projects.Items[0].Id) + "/ingest-token"

	response, body := send(t, http.MethodPost, url, nil)
	var made CreateIngestToken200JSONResponse
	_ = jsonUnmarshal(body, &made)
	if response.StatusCode != http.StatusOK || len(made.Token) < 32 {
		t.Fatalf("make: %d %s", response.StatusCode, body)
	}
	if _, body := get(t, e.server.URL+"/api/v1/projects"); !strings.Contains(body, `"ingest_enabled":true`) {
		t.Fatalf("after making: %s", body)
	}
	again, _ := send(t, http.MethodPost, url, nil)
	if again.StatusCode != http.StatusOK {
		t.Fatalf("again: %d", again.StatusCode)
	}
	if response, _ := send(t, http.MethodDelete, url, nil); response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", response.StatusCode)
	}
	if response, _ := send(t, http.MethodPost, e.server.URL+"/api/v1/projects/999/ingest-token", nil); response.StatusCode != http.StatusNotFound {
		t.Fatalf("no project: %d", response.StatusCode)
	}
}

func jsonUnmarshal(body string, v any) error { return json.Unmarshal([]byte(body), v) }

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
