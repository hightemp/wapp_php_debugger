package ingest

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
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

// Remote sources have no file system events: their directory is polled.
func TestManagerPollsRemoteSources(t *testing.T) {
	e := newManagerEnv(t)
	remote := source.NewMemory(nil)
	e.manager = NewManager(e.store, ManagerOptions{
		PollInterval:       time.Hour,
		RemotePollInterval: 30 * time.Millisecond,
		Sources: func(project store.Project) (source.Source, bool, error) {
			return remote, true, nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); e.manager.Wait() })
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	project := e.project(t, "shop")
	e.manager.Add(project)

	remote.Append("1792400000001", v1Fixtures(t)["web-get.log"])
	eventually(t, "the polled file is imported", func() bool { return e.requests(t, project.ID) == 1 })
	if src, ok := e.manager.Source(project.ID); !ok || src != remote {
		t.Fatalf("Source = %v, %v", src, ok)
	}
}

// A project whose source cannot be made (a server of an unknown kind, a broken config) is skipped.
func TestManagerSkipsAProjectWithoutASource(t *testing.T) {
	e := newManagerEnv(t)
	e.manager = NewManager(e.store, ManagerOptions{
		Sources: func(project store.Project) (source.Source, bool, error) {
			return nil, false, errors.New("no source")
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); e.manager.Wait() })
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	project := e.project(t, "shop")
	e.manager.Add(project)
	if _, ok := e.manager.Source(project.ID); ok {
		t.Fatal("no source, no loop")
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A project that takes pushed logs may have no logs directory here at all: that is not a problem.
func TestManagerIsQuietAboutAMissingDirOfAPushedProject(t *testing.T) {
	e := newManagerEnv(t)
	var log syncBuffer
	e.manager.options.Logger = slog.New(slog.NewTextHandler(&log, nil))
	ctx := context.Background()
	root := t.TempDir()
	pushed, _ := e.store.CreateProject(ctx, store.ProjectInput{ServerID: e.server, Name: "pushed", RootPath: root, LogsDir: filepath.Join(root, "missing")})
	hash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := e.store.SetIngestToken(ctx, pushed.ID, &hash); err != nil {
		t.Fatal(err)
	}
	plain, _ := e.store.CreateProject(ctx, store.ProjectInput{ServerID: e.server, Name: "plain", RootPath: root, LogsDir: filepath.Join(root, "gone")})
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := e.manager.Start(run); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the plain project's missing dir is reported", func() bool { return strings.Contains(log.String(), "project="+plain.Name) })
	time.Sleep(100 * time.Millisecond)
	if strings.Contains(log.String(), "project="+pushed.Name) {
		t.Fatalf("warned about the pushed project:\n%s", log.String())
	}
	cancel()
	e.manager.Wait()
}
