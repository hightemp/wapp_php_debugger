package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/ingest"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

type testEnv struct {
	server *httptest.Server
	broker *events.Broker
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	ids, err := s.EnsureLocalProject(ctx, "shop", "/var/www/app", "/var/www/app/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}

	paths, _ := filepath.Glob(filepath.Join(filepath.Dir(specPath(t)), "fixtures/v1/*.log"))
	files := map[string][]byte{}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		files[filepath.Base(path)] = data
	}
	if _, err := ingest.New(s).Scan(ctx, ids.ProjectID, source.NewMemory(files)); err != nil {
		t.Fatal(err)
	}

	broker := events.NewBroker(16)
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>wapp-debugger</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	server := httptest.NewServer(NewHandler(Deps{Store: s, Broker: broker, UI: ui, Heartbeat: 20 * time.Millisecond}))
	t.Cleanup(server.Close)
	return testEnv{server: server, broker: broker}
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	return response, string(body)
}

func TestGetVersion(t *testing.T) {
	e := newTestEnv(t)
	response, body := get(t, e.server.URL+"/api/v1/version")
	var version Version
	if response.StatusCode != 200 || json.Unmarshal([]byte(body), &version) != nil || version.Version != "dev" {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
}

func TestListRequestsPaginatesNewestFirst(t *testing.T) {
	e := newTestEnv(t)
	var keys []string
	url := e.server.URL + "/api/v1/requests?limit=3"
	for range 5 {
		response, body := get(t, url)
		if response.StatusCode != 200 {
			t.Fatalf("status=%d body=%s", response.StatusCode, body)
		}
		var page RequestPage
		if err := json.Unmarshal([]byte(body), &page); err != nil {
			t.Fatal(err)
		}
		for _, r := range page.Items {
			keys = append(keys, r.RequestKey)
		}
		if page.NextCursor == nil {
			break
		}
		url = e.server.URL + "/api/v1/requests?limit=3&cursor=" + *page.NextCursor
	}
	if len(keys) != 7 {
		t.Fatalf("got %d requests over all pages, want 7: %v", len(keys), keys)
	}
	_, body := get(t, e.server.URL+"/api/v1/requests?limit=1")
	if !strings.Contains(body, `"request_key"`) || !strings.Contains(body, `"format_version":1`) {
		t.Fatalf("fields must be snake_case per spec: %s", body)
	}
}

func assertProblem(t *testing.T, response *http.Response, body string, status int) {
	t.Helper()
	var problem Problem
	if response.StatusCode != status || response.Header.Get("Content-Type") != "application/problem+json" ||
		json.Unmarshal([]byte(body), &problem) != nil || problem.Status != status || problem.Title == "" {
		t.Fatalf("want problem %d, got status=%d type=%q body=%s", status, response.StatusCode, response.Header.Get("Content-Type"), body)
	}
}

func TestListRequestsValidatesParameters(t *testing.T) {
	e := newTestEnv(t)
	for _, query := range []string{"cursor=garbage", "limit=0", "limit=501", "limit=abc"} {
		t.Run(query, func(t *testing.T) {
			response, body := get(t, e.server.URL+"/api/v1/requests?"+query)
			assertProblem(t, response, body, 400)
		})
	}
}

func TestNotImplementedAndUnknownAPI(t *testing.T) {
	e := newTestEnv(t)
	response, body := get(t, e.server.URL+"/api/v1/servers")
	assertProblem(t, response, body, 501)
	response, body = get(t, e.server.URL+"/api/v1/no-such-thing")
	assertProblem(t, response, body, 404)
}

func TestServesUIWithSPAFallback(t *testing.T) {
	e := newTestEnv(t)
	for _, path := range []string{"/", "/requests/42", "/settings"} {
		response, body := get(t, e.server.URL+path)
		if response.StatusCode != 200 || !strings.Contains(body, "<title>wapp-debugger</title>") {
			t.Fatalf("%s: status=%d body=%q", path, response.StatusCode, body)
		}
	}
	response, body := get(t, e.server.URL+"/assets/app.js")
	if response.StatusCode != 200 || body != "console.log(1)" || !strings.Contains(response.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("asset: status=%d type=%q body=%q", response.StatusCode, response.Header.Get("Content-Type"), body)
	}
	if response, _ := get(t, e.server.URL+"/assets/missing.js"); response.StatusCode != 404 {
		t.Fatalf("missing asset must be 404, not index.html: %d", response.StatusCode)
	}
}

// readSSE reads the stream up to the first event with data (heartbeat comments are skipped and counted).
func readSSE(t *testing.T, reader *bufio.Reader) (id, name, data string, pings int) {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, ":"):
			pings++
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			return id, name, data, pings
		}
	}
}

func openStream(t *testing.T, url, lastEventID string) *bufio.Reader {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if lastEventID != "" {
		request.Header.Set("Last-Event-ID", lastEventID)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("status=%d headers=%v", response.StatusCode, response.Header)
	}
	return bufio.NewReader(response.Body)
}

func TestStreamDeliversEventsAndHeartbeats(t *testing.T) {
	e := newTestEnv(t)
	reader := openStream(t, e.server.URL+"/api/v1/stream", "")
	time.Sleep(60 * time.Millisecond) // a few heartbeats before the first event
	e.broker.Publish("request_created", map[string]any{"event": "request_created", "request_id": 7})

	id, name, data, pings := readSSE(t, reader)
	if id != "1" || name != "request_created" || data != `{"event":"request_created","request_id":7}` {
		t.Fatalf("id=%q name=%q data=%q", id, name, data)
	}
	if pings == 0 {
		t.Fatal("heartbeat comments must keep the connection alive")
	}
}

func TestStreamResumesAfterLastEventID(t *testing.T) {
	e := newTestEnv(t)
	for i := range 3 {
		e.broker.Publish("request_created", map[string]int{"n": i})
	}
	id, _, data, _ := readSSE(t, openStream(t, e.server.URL+"/api/v1/stream", "2"))
	if id != "3" || data != `{"n":2}` {
		t.Fatalf("after Last-Event-ID 2 expected event 3, got id=%q data=%q", id, data)
	}
}
