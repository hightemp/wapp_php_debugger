package agent_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/agent"
	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

const token = "push-token-of-the-agent-test-0123456789ab"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A viewer with one project that takes pushed logs; down, when set, answers 503 instead.
func viewer(t *testing.T, down *atomic.Bool) (*httptest.Server, *store.Store, int64) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "viewer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	if err := s.SetIngestToken(ctx, ids.ProjectID, &hash); err != nil {
		t.Fatal(err)
	}
	receiver := api.NewIngestHandler(api.IngestDeps{Store: s})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down != nil && down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		receiver.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server, s, ids.ProjectID
}

func requests(t *testing.T, s *store.Store, projectID int64) []store.RequestRow {
	t.Helper()
	page, err := s.ListRequests(context.Background(), store.RequestFilter{ProjectID: projectID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func TestAgentPushesNewLinesOfTheLogsDir(t *testing.T) {
	var down atomic.Bool
	server, s, projectID := viewer(t, &down)
	dir := t.TempDir()
	web := fixture(t, "v2/web-request.jsonl")
	half := len(web) / 2
	for half > 0 && web[half-1] != '\n' {
		half--
	}
	if err := os.WriteFile(filepath.Join(dir, "1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl"), web[:half+5], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1792400002000-legacy"), fixture(t, "v1/web-get.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{Source: source.NewLocal(dir, ""), URL: server.URL + "/ingest/v2", Token: token}
	ctx := context.Background()

	// The half of a line still being written waits; the v1 file is not for pushing
	if err := a.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if rows := requests(t, s, projectID); len(rows) != 1 || rows[0].Finished {
		t.Fatalf("after the first half: %+v", rows)
	}

	// The viewer is away: nothing is lost, the lines go once it is back
	if err := os.WriteFile(filepath.Join(dir, "1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl"), web, 0o644); err != nil {
		t.Fatal(err)
	}
	down.Store(true)
	if err := a.Sync(ctx); err == nil {
		t.Fatal("a failed push is an error")
	}
	down.Store(false)
	if err := a.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	rows := requests(t, s, projectID)
	if len(rows) != 1 || !rows[0].Finished || rows[0].MessageCount != 6 {
		t.Fatalf("after the rest: %+v", rows)
	}

	// A new agent (restarted, offsets lost) sends it all again; the viewer keeps one copy
	again := &agent.Agent{Source: source.NewLocal(dir, ""), URL: server.URL + "/ingest/v2", Token: token}
	if err := again.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if rows := requests(t, s, projectID); len(rows) != 1 || rows[0].MessageCount != 6 {
		t.Fatalf("after a restart: %+v", rows)
	}
}

func TestAgentRefusedByTheViewer(t *testing.T) {
	server, _, _ := viewer(t, nil)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl"), fixture(t, "v2/web-request.jsonl"), 0o644)
	a := &agent.Agent{Source: source.NewLocal(dir, ""), URL: server.URL + "/ingest/v2", Token: "wrong"}
	var refused *agent.RefusedError
	if err := a.Sync(context.Background()); !errors.As(err, &refused) {
		t.Fatalf("a wrong token must be an error the user sees: %v", err)
	}
}
