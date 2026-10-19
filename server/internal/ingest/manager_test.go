package ingest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

type managerEnv struct {
	store   *store.Store
	manager *Manager
	server  int64
	mu      sync.Mutex
	changes map[int64]int
}

func newManagerEnv(t *testing.T) *managerEnv {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	server, err := s.EnsureLocalServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	e := &managerEnv{store: s, server: server, changes: map[int64]int{}}
	e.manager = NewManager(s, ManagerOptions{
		PollInterval: time.Hour,
		OnChange: func(projectID int64, created, updated []int64) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.changes[projectID] += len(created) + len(updated)
		},
	})
	return e
}

func (e *managerEnv) project(t *testing.T, name string) store.Project {
	t.Helper()
	root := t.TempDir()
	logs := filepath.Join(root, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	project, err := e.store.CreateProject(context.Background(), store.ProjectInput{ServerID: e.server, Name: name, RootPath: root, LogsDir: logs})
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func (e *managerEnv) requests(t *testing.T, projectID int64) int {
	t.Helper()
	page, err := e.store.ListRequests(context.Background(), store.RequestFilter{ProjectID: projectID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Items)
}

func writeLog(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), v1Fixtures(t)["web-get.log"], 0o644); err != nil {
		t.Fatal(err)
	}
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within 3s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestManagerImportsEveryProject(t *testing.T) {
	e := newManagerEnv(t)
	a, b := e.project(t, "a"), e.project(t, "b")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	writeLog(t, a.LogsDir, "1792400000001")
	writeLog(t, b.LogsDir, "1792400000002")
	eventually(t, "both projects imported", func() bool { return e.requests(t, a.ID) == 1 && e.requests(t, b.ID) == 1 })
	eventually(t, "changes reported", func() bool {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.changes[a.ID] == 1 && e.changes[b.ID] == 1
	})
	cancel()
	e.manager.Wait()
}

func TestManagerAddsAndRemovesProjectsAtRuntime(t *testing.T) {
	e := newManagerEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}

	added := e.project(t, "added")
	e.manager.Add(added)
	writeLog(t, added.LogsDir, "1792400000003")
	eventually(t, "added project imported", func() bool { return e.requests(t, added.ID) == 1 })

	e.manager.Remove(added.ID)
	writeLog(t, added.LogsDir, "1792400000004")
	time.Sleep(300 * time.Millisecond)
	if n := e.requests(t, added.ID); n != 1 {
		t.Fatalf("a removed project is not imported any more, got %d requests", n)
	}
	if _, ok := e.manager.Source(added.ID); ok {
		t.Fatal("no source for a removed project")
	}
	cancel()
	e.manager.Wait()
}

func TestManagerFollowsAChangedLogsDir(t *testing.T) {
	e := newManagerEnv(t)
	project := e.project(t, "moving")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}

	moved := filepath.Join(t.TempDir(), "new-logs")
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	project.LogsDir = moved
	e.manager.Add(project) // same id, new settings: the loop restarts
	writeLog(t, moved, "1792400000005")
	eventually(t, "new directory watched", func() bool { return e.requests(t, project.ID) == 1 })
	cancel()
	e.manager.Wait()
}

func TestManagerGivesTheProjectSource(t *testing.T) {
	e := newManagerEnv(t)
	project := e.project(t, "switchable")
	if err := os.WriteFile(project.ConfigPath, []byte(`{"bIsOnFlag":false}`), 0o664); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	src, ok := e.manager.Source(project.ID)
	if !ok {
		t.Fatal("source expected")
	}
	if err := src.SetEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := src.Enabled(ctx); !enabled {
		t.Fatal("the switch goes to the project config")
	}
	cancel()
	e.manager.Wait()
}
