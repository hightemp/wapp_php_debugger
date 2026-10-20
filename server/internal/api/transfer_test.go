package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func TestExportAndImportRequest(t *testing.T) {
	e := newTestEnv(t)
	id := findRequest(t, e, webRequestKey)
	response, exported := get(t, fmt.Sprintf("%s/api/v1/requests/%d/export", e.server.URL, id))
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("export: %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	if disposition := response.Header.Get("Content-Disposition"); disposition != `attachment; filename="request-`+webRequestKey+`.jsonl"` {
		t.Fatalf("disposition %q", disposition)
	}
	if !strings.HasPrefix(exported, `{"v":2,"type":"request_start","request_id":"`+webRequestKey+`"`) {
		t.Fatalf("export: %.200s", exported)
	}

	// Into another viewer: the request comes in and the feed hears about it
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "other.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	other := store.New(db)
	ids, _ := other.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	broker := events.NewBroker(16)
	server := httptest.NewServer(NewHandler(Deps{Store: other, Broker: broker, UI: fstest.MapFS{"index.html": {}}}))
	t.Cleanup(server.Close)
	stream := broker.Subscribe(ctx, 0)

	importURL := fmt.Sprintf("%s/api/v1/projects/%d/import", server.URL, ids.ProjectID)
	response, body := postNDJSON(t, importURL, exported)
	var result ImportRequest200JSONResponse
	_ = json.Unmarshal([]byte(body), &result)
	if response.StatusCode != http.StatusOK || !result.Created || result.RequestId == 0 {
		t.Fatalf("import: %d %s", response.StatusCode, body)
	}
	select {
	case event := <-stream:
		if event.Name != "request_created" {
			t.Fatalf("event %s", event.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("no request_created after import")
	}

	// Once more: it is already there, and the answer says where
	response, body = postNDJSON(t, importURL, exported)
	var again ImportRequest200JSONResponse
	_ = json.Unmarshal([]byte(body), &again)
	if response.StatusCode != http.StatusOK || again.Created || again.RequestId != result.RequestId {
		t.Fatalf("import again: %d %s", response.StatusCode, body)
	}

	for name, bad := range map[string]string{
		"not a stream":    "hello\n",
		"no such project": exported,
	} {
		url := importURL
		if name == "no such project" {
			url = fmt.Sprintf("%s/api/v1/projects/999/import", server.URL)
		}
		response, body := postNDJSON(t, url, bad)
		if response.StatusCode/100 != 4 || !strings.Contains(response.Header.Get("Content-Type"), "problem+json") {
			t.Errorf("%s: %d %s", name, response.StatusCode, body)
		}
	}
	response, _ = get(t, server.URL+"/api/v1/requests/999/export")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("export of a missing request: %d", response.StatusCode)
	}
}

func postNDJSON(t *testing.T, url, body string) (*http.Response, string) {
	t.Helper()
	response, err := http.Post(url, "application/x-ndjson", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(response.Body)
	return response, string(data)
}

func TestImportAThirdPartyLog(t *testing.T) {
	e := newTestEnv(t)
	laravel, err := os.ReadFile(filepath.Join(filepath.Dir(specPath(t)), "fixtures/thirdparty/laravel.log"))
	if err != nil {
		t.Fatal(err)
	}
	url := e.server.URL + "/api/v1/projects/1/import?name=laravel-2026-10-20.log"
	response, body := postNDJSON(t, url, string(laravel))
	var result ImportRequest200JSONResponse
	_ = json.Unmarshal([]byte(body), &result)
	if response.StatusCode != http.StatusOK || !result.Created {
		t.Fatalf("import: %d %s", response.StatusCode, body)
	}
	_, detail := get(t, fmt.Sprintf("%s/api/v1/requests/%d", e.server.URL, result.RequestId))
	if !strings.Contains(detail, `"argv":["laravel-2026-10-20.log"]`) || !strings.Contains(detail, `"error_count":1`) {
		t.Fatalf("detail: %s", detail)
	}
	response, body = postNDJSON(t, url, string(laravel))
	var again ImportRequest200JSONResponse
	_ = json.Unmarshal([]byte(body), &again)
	if again.Created || again.RequestId != result.RequestId {
		t.Fatalf("again: %d %s", response.StatusCode, body)
	}
}
