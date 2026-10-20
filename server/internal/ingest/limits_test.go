package ingest

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// slowSource lists slowly and counts how many lists run at once.
type slowSource struct {
	*source.Memory
	delay   time.Duration
	active  *atomic.Int32
	peak    *atomic.Int32
	listErr error
	lists   atomic.Int32
}

func (s *slowSource) List(ctx context.Context) ([]source.FileInfo, error) {
	s.lists.Add(1)
	now := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		peak := s.peak.Load()
		if now <= peak || s.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Memory.List(ctx)
}

func startManager(t *testing.T, e *managerEnv, options ManagerOptions) {
	t.Helper()
	e.manager = NewManager(e.store, options)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); e.manager.Wait() })
	if err := e.manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestManagerLimitsScansRunningAtOnce(t *testing.T) {
	e := newManagerEnv(t)
	var active, peak atomic.Int32
	startManager(t, e, ManagerOptions{
		PollInterval: time.Hour, RemotePollInterval: 10 * time.Millisecond, MaxScans: 2,
		Sources: func(store.Project) (source.Source, bool, error) {
			return &slowSource{Memory: source.NewMemory(nil), delay: 30 * time.Millisecond, active: &active, peak: &peak}, true, nil
		},
	})
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		e.manager.Add(e.project(t, name))
	}
	time.Sleep(300 * time.Millisecond)
	if peak.Load() > 2 {
		t.Fatalf("%d scans at once, limit 2", peak.Load())
	}
	if peak.Load() < 2 {
		t.Fatalf("peak %d: the limit is not used", peak.Load())
	}
}

func TestManagerGivesUpAHangingScan(t *testing.T) {
	e := newManagerEnv(t)
	var active, peak atomic.Int32
	hanging := &slowSource{Memory: source.NewMemory(nil), delay: time.Hour, active: &active, peak: &peak}
	startManager(t, e, ManagerOptions{
		PollInterval: time.Hour, RemotePollInterval: 10 * time.Millisecond, ScanTimeout: 50 * time.Millisecond, MaxScans: 1,
		Sources: func(project store.Project) (source.Source, bool, error) {
			if project.Name == "hanging" {
				return hanging, true, nil
			}
			memory := source.NewMemory(nil)
			memory.Append("1792400000001", v1Fixtures(t)["web-get.log"])
			return memory, true, nil
		},
	})
	e.manager.Add(e.project(t, "hanging"))
	healthy := e.project(t, "healthy")
	e.manager.Add(healthy)
	// One slot only: the healthy project gets it once the hanging scan times out
	eventually(t, "the healthy project is imported", func() bool { return e.requests(t, healthy.ID) == 1 })
	eventually(t, "the hanging source is tried again", func() bool { return hanging.lists.Load() >= 2 })
}

func TestManagerLogsAFailingSourceOnce(t *testing.T) {
	e := newManagerEnv(t)
	var logs bytes.Buffer
	var mu sync.Mutex
	logger := slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logs.Write(p)
	}), nil))
	var active, peak atomic.Int32
	failing := &slowSource{Memory: source.NewMemory(nil), active: &active, peak: &peak, listErr: errors.New("connection refused")}
	startManager(t, e, ManagerOptions{
		PollInterval: time.Hour, RemotePollInterval: 5 * time.Millisecond, Logger: logger,
		Sources: func(store.Project) (source.Source, bool, error) { return failing, true, nil },
	})
	e.manager.Add(e.project(t, "shop"))
	eventually(t, "several scans", func() bool { return failing.lists.Load() >= 5 })
	mu.Lock()
	defer mu.Unlock()
	if n := strings.Count(logs.String(), "connection refused"); n != 1 {
		t.Fatalf("the same error logged %d times:\n%s", n, logs.String())
	}
}

func TestManagerPrunesByRetention(t *testing.T) {
	e := newManagerEnv(t)
	memory := source.NewMemory(nil)
	memory.Append("1792400000001", v1Fixtures(t)["web-get.log"]) // a request of 2026: older than a day by now
	startManager(t, e, ManagerOptions{
		PollInterval: time.Hour, RemotePollInterval: 10 * time.Millisecond,
		Now:     func() time.Time { return time.UnixMilli(1792400000000).Add(48 * time.Hour) },
		Sources: func(store.Project) (source.Source, bool, error) { return memory, true, nil },
	})
	project := e.project(t, "shop")
	days := 1
	project, err := e.store.UpdateProject(context.Background(), project.ID, store.ProjectPatch{RetentionDays: &days})
	if err != nil {
		t.Fatal(err)
	}
	e.manager.Add(project)
	eventually(t, "the file is read", func() bool {
		_, found, _ := e.store.FileState(context.Background(), project.ID, "1792400000001")
		return found
	})
	eventually(t, "the old request is pruned", func() bool { return e.requests(t, project.ID) == 0 })
	time.Sleep(50 * time.Millisecond)
	if e.requests(t, project.ID) != 0 {
		t.Fatal("the pruned request came back")
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
