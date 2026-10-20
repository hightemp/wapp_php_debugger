package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// findRequest returns the feed id of the request with the given key.
func findRequest(t *testing.T, e testEnv, key string) int64 {
	t.Helper()
	_, body := get(t, e.server.URL+"/api/v1/requests?limit=100")
	var page RequestPage
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	for _, r := range page.Items {
		if r.RequestKey == key {
			return r.Id
		}
	}
	t.Fatalf("request %s not found", key)
	return 0
}

const webRequestKey = "01JAZ7Q5X0K4M8T2V6N9P3R5S7"

func TestGetRequestWithChildren(t *testing.T) {
	e := newTestEnv(t)
	id := findRequest(t, e, webRequestKey)
	response, body := get(t, fmt.Sprintf("%s/api/v1/requests/%d", e.server.URL, id))
	var detail RequestDetail
	if response.StatusCode != 200 || json.Unmarshal([]byte(body), &detail) != nil {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
	if detail.PhpVersion == nil || *detail.PhpVersion != "7.4.33" || detail.Children == nil || len(*detail.Children) != 2 ||
		detail.MemoryPeakBytes == nil || *detail.Lang != "php" {
		t.Fatalf("detail = %s", body)
	}

	response, body = get(t, e.server.URL+"/api/v1/requests/999999")
	assertProblem(t, response, body, 404)
}

func TestListMessagesWithFiltersAndCursor(t *testing.T) {
	e := newTestEnv(t)
	base := fmt.Sprintf("%s/api/v1/requests/%d/messages", e.server.URL, findRequest(t, e, webRequestKey))

	_, body := get(t, base+"?level=warning,info&limit=2")
	var page MessagePage
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	// web-request.jsonl: info message (seq 3), info timer (4), warning php_error (6), warning message (7)
	if len(page.Items) != 2 || page.NextCursor == nil || page.Items[0].Seq != 3 || page.Items[1].Seq != 4 {
		t.Fatalf("page = %s", body)
	}
	_, body = get(t, base+"?level=warning,info&limit=2&cursor="+*page.NextCursor)
	if err := json.Unmarshal([]byte(body), &page); err != nil || len(page.Items) != 2 || page.Items[0].Level != LevelWarning || page.NextCursor != nil {
		t.Fatalf("second page = %s", body)
	}

	_, body = get(t, base+"?type=timer")
	if !strings.Contains(body, `"message":"db"`) || !strings.Contains(body, `"duration_ms":12.4`) {
		t.Fatalf("timers = %s", body)
	}
	_, body = get(t, base+"?tag=sql")
	if strings.Count(body, `"id":`) != 1 {
		t.Fatalf("sql = %s", body)
	}

	response, body := get(t, base+"?level=critical")
	assertProblem(t, response, body, 400)
	response, body = get(t, fmt.Sprintf("%s/api/v1/requests/999999/messages", e.server.URL))
	assertProblem(t, response, body, 404)
}

func TestGetMessageWithContextTraceAndEditorLinks(t *testing.T) {
	e := newTestEnv(t)
	base := fmt.Sprintf("%s/api/v1/requests/%d/messages", e.server.URL, findRequest(t, e, webRequestKey))
	_, body := get(t, base+"?level=info")
	var page MessagePage
	if err := json.Unmarshal([]byte(body), &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("messages = %s", body)
	}

	response, body := get(t, fmt.Sprintf("%s/api/v1/messages/%d", e.server.URL, page.Items[0].Id))
	var detail MessageDetail
	if response.StatusCode != 200 || json.Unmarshal([]byte(body), &detail) != nil {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
	if detail.Context == nil || (*detail.Context)["item_id"] != float64(42) || detail.Trace == nil || len(*detail.Trace) != 2 {
		t.Fatalf("detail = %s", body)
	}
	source := detail.Source
	// v2 paths are relative to the project root (/var/www/app in the test project)
	if source == nil || *source.File != "app/Controller/ItemController.php" ||
		*source.EditorUrl != "vscode://file/var/www/app/app/Controller/ItemController.php:31" || *source.CallType != "->" {
		t.Fatalf("source = %s", body)
	}

	response, body = get(t, e.server.URL+"/api/v1/messages/999999")
	assertProblem(t, response, body, 404)
}

func TestV1TraceLinksMapAbsolutePaths(t *testing.T) {
	e := newTestEnv(t)
	base := fmt.Sprintf("%s/api/v1/requests/%d/messages", e.server.URL, findRequest(t, e, "web-get.log"))
	_, body := get(t, base+"?limit=1")
	var page MessagePage
	if err := json.Unmarshal([]byte(body), &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("messages = %s", body)
	}
	_, body = get(t, fmt.Sprintf("%s/api/v1/messages/%d", e.server.URL, page.Items[0].Id))
	// v1 writes absolute server paths: inside the project root they are shown relative
	if !strings.Contains(body, `"file":"Debugger.class.php"`) || !strings.Contains(body, `"editor_url":"vscode://file/var/www/app/Debugger.class.php:`) {
		t.Fatalf("v1 trace = %s", body)
	}
}

func TestSearchAndFilterTheFeed(t *testing.T) {
	e := newTestEnv(t)
	keys := func(query string) []string {
		_, body := get(t, e.server.URL+"/api/v1/requests?limit=100&"+query)
		var page RequestPage
		if err := json.Unmarshal([]byte(body), &page); err != nil {
			t.Fatalf("%s: %s", query, body)
		}
		var result []string
		for _, r := range page.Items {
			result = append(result, r.RequestKey)
		}
		return result
	}
	// a context value of the AJAX child only; a common message word matches v1 and v2 alike
	if got := keys("q=" + url.QueryEscape("c-981")); len(got) != 1 || got[0] != "01JAZ7Q5X0K4M8T2V6N9P3R5S8" {
		t.Fatalf("q: %v", got)
	}
	if got := keys("q=" + url.QueryEscape("кеша")); len(got) != 8 { // 7 v1 logs + web-request.jsonl
		t.Fatalf("q common word: %v", got)
	}
	if got := keys("has_errors=true&sapi=cli"); len(got) != 4 { // cli, killed and pre-hotfix v1 logs are CLI runs with an error, plus cli-exception.jsonl
		t.Fatalf("has_errors+cli: %v", got)
	}
	if got := keys("since=1792400100000&until=1792400100001"); len(got) != 1 {
		t.Fatalf("time range: %v", got)
	}

	id := findRequest(t, e, webRequestKey)
	_, body := get(t, fmt.Sprintf("%s/api/v1/requests/%d/messages?q=%s", e.server.URL, id, url.QueryEscape("кеша")))
	if strings.Count(body, `"id":`) != 1 {
		t.Fatalf("messages q: %s", body)
	}
}

func TestFeedFiltersByMethodStatusDurationAjaxTagAndLevel(t *testing.T) {
	e := newTestEnv(t)
	count := func(query string) int {
		t.Helper()
		response, body := get(t, e.server.URL+"/api/v1/requests?limit=100&"+query)
		var page RequestPage
		if response.StatusCode != 200 || json.Unmarshal([]byte(body), &page) != nil {
			t.Fatalf("%s: %d %s", query, response.StatusCode, body)
		}
		return len(page.Items)
	}
	for query, want := range map[string]int{
		"method=post":               count("method=POST"),
		"status=2xx&sapi=web":       count("status=2xx&sapi=web"),
		"status=201":                1,
		"is_ajax=true&status=201":   1,
		"tag=queue":                 1,
		"level=error&tag=queue":     1,
		"min_duration_ms=100000000": 0,
	} {
		if got := count(query); got != want {
			t.Errorf("%s: %d requests, want %d", query, got, want)
		}
	}
	if count("status=2xx") == count("") {
		t.Error("status=2xx filtered nothing out")
	}
	if response, body := get(t, e.server.URL+"/api/v1/requests?status=abc"); response.StatusCode != 400 {
		t.Errorf("a bad status: %d %s", response.StatusCode, body)
	}
}

func TestPinAndNoteARequest(t *testing.T) {
	e := newTestEnv(t)
	id := findRequest(t, e, webRequestKey)
	url := fmt.Sprintf("%s/api/v1/requests/%d", e.server.URL, id)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := e.broker.Subscribe(ctx, 0)
	response, body := send(t, http.MethodPatch, url, map[string]any{"pinned": true, "note": "checkout bug"})
	// The feed hears about it like about any other change of the request
	select {
	case event := <-stream:
		if event.Name != "request_updated" || !strings.Contains(string(event.Data), `"pinned":true`) {
			t.Fatalf("event %s %s", event.Name, event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no request_updated after marking")
	}
	if response.StatusCode != 200 || !strings.Contains(body, `"pinned":true`) || !strings.Contains(body, `"note":"checkout bug"`) {
		t.Fatalf("mark: %d %s", response.StatusCode, body)
	}
	if _, detail := get(t, url); !strings.Contains(detail, `"pinned":true`) || !strings.Contains(detail, `"note":"checkout bug"`) {
		t.Fatalf("detail: %s", detail)
	}
	_, body = get(t, e.server.URL+"/api/v1/requests?pinned=true")
	if strings.Count(body, `"request_key"`) != 1 || !strings.Contains(body, webRequestKey) {
		t.Fatalf("pinned feed: %s", body)
	}
	if response, _ := send(t, http.MethodPatch, e.server.URL+"/api/v1/requests/999999", map[string]any{"pinned": true}); response.StatusCode != 404 {
		t.Fatalf("missing: %d", response.StatusCode)
	}
}
