package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/ingest"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// localRuntime gives every project a source over its directory on this machine.
type localRuntime struct{ src source.Source }

func (r localRuntime) Add(store.Project)                  {}
func (r localRuntime) Remove(int64)                       {}
func (r localRuntime) Source(int64) (source.Source, bool) { return r.src, r.src != nil }

func TestMessageSourcePreview(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755)
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	write("app/Controller/ItemController.php", []byte(strings.Join(lines, "\n")+"\n"))
	write("public/index.php", []byte("<?php\nrequire 'app.php';\n"))
	write("app/Model/Item.php", []byte("binary\x00data"))

	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	ids, _ := s.EnsureLocalProject(ctx, "shop", root, filepath.Join(root, "tmp", "logs"))
	fixture, _ := os.ReadFile(filepath.Join(filepath.Dir(specPath(t)), "fixtures/v2/web-request.jsonl"))
	if _, err := ingest.New(s).Scan(ctx, ids.ProjectID, source.NewMemory(map[string][]byte{"web.jsonl": fixture})); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(Deps{Store: s, Broker: events.NewBroker(4), UI: fstest.MapFS{"index.html": {}},
		Projects: localRuntime{src: source.NewLocal(filepath.Join(root, "tmp", "logs"), filepath.Join(root, "dws_debugger.json"))}}))
	t.Cleanup(server.Close)

	_, body := get(t, server.URL+"/api/v1/requests?limit=10")
	var feed RequestPage
	_ = json.Unmarshal([]byte(body), &feed)
	_, body = get(t, fmt.Sprintf("%s/api/v1/requests/%d/messages?limit=100", server.URL, feed.Items[0].Id))
	var page MessagePage
	_ = json.Unmarshal([]byte(body), &page)
	bySeq := map[int]int64{}
	for _, m := range page.Items {
		bySeq[m.Seq] = m.Id
	}
	url := func(seq int, query string) string {
		return fmt.Sprintf("%s/api/v1/messages/%d/source%s", server.URL, bySeq[seq], query)
	}

	response, body := get(t, url(3, ""))
	var preview SourcePreview
	if response.StatusCode != http.StatusOK || json.Unmarshal([]byte(body), &preview) != nil {
		t.Fatalf("message source: %d %s", response.StatusCode, body)
	}
	if preview.File != "app/Controller/ItemController.php" || preview.Line != 31 || preview.FirstLine != 19 ||
		len(preview.Lines) != 22 || preview.Lines[31-19] != "line 31" {
		t.Fatalf("preview = %+v", preview)
	}

	_, body = get(t, url(3, "?frame=1&around=1"))
	_ = json.Unmarshal([]byte(body), &preview)
	if preview.File != "public/index.php" || preview.Line != 12 || len(preview.Lines) != 0 {
		t.Fatalf("a frame past the end of a short file: %s", body)
	}

	for _, c := range []struct {
		query  string
		seq    int
		status int
	}{
		{"?frame=5", 3, http.StatusNotFound},     // no such frame
		{"", 4, http.StatusUnsupportedMediaType}, // not text
		{"?around=0", 3, http.StatusBadRequest},  // out of range
	} {
		if response, body := get(t, url(c.seq, c.query)); response.StatusCode != c.status {
			t.Errorf("seq %d %s: %d %s", c.seq, c.query, response.StatusCode, body)
		}
	}
}
