package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func next(t *testing.T, signals <-chan struct{}, within time.Duration) bool {
	t.Helper()
	select {
	case _, ok := <-signals:
		return ok
	case <-time.After(within):
		return false
	}
}

func drain(signals <-chan struct{}, during time.Duration) int {
	count := 0
	deadline := time.After(during)
	for {
		select {
		case <-signals:
			count++
		case <-deadline:
			return count
		}
	}
}

func TestSignalsSoonAfterAFileChanges(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The poll interval is long: a quick signal can only come from file system notifications
	signals := Dir(ctx, dir, Options{PollInterval: time.Hour, Debounce: 20 * time.Millisecond})
	if !next(t, signals, time.Second) {
		t.Fatal("the first signal (initial scan) must come right away")
	}

	if err := os.WriteFile(filepath.Join(dir, "1792400000000"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !next(t, signals, time.Second) {
		t.Fatal("no signal after a file was created")
	}
}

func TestDebounceMergesBurstsOfWrites(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := Dir(ctx, dir, Options{PollInterval: time.Hour, Debounce: 100 * time.Millisecond})
	next(t, signals, time.Second)

	file, err := os.Create(filepath.Join(dir, "1792400000000"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	for range 50 {
		_, _ = file.WriteString("line\n")
		time.Sleep(2 * time.Millisecond)
	}
	if got := drain(signals, 500*time.Millisecond); got < 1 || got > 2 {
		t.Fatalf("50 writes in a burst produced %d signals, want 1 or 2", got)
	}
}

func TestFallsBackToPollingWhenWatchingFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A missing directory cannot be watched; polling must keep the import going (it may appear later)
	signals := Dir(ctx, filepath.Join(t.TempDir(), "missing"), Options{PollInterval: 30 * time.Millisecond, Debounce: 10 * time.Millisecond})
	if got := drain(signals, 200*time.Millisecond); got < 3 {
		t.Fatalf("polling produced %d signals in 200ms, want at least 3", got)
	}
}

func TestStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := Dir(ctx, t.TempDir(), Options{PollInterval: time.Hour, Debounce: 10 * time.Millisecond})
	next(t, signals, time.Second)
	cancel()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-signals:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("channel not closed after cancel")
		}
	}
}

func TestEverySignalsAtOnceThenPeriodically(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := Every(ctx, 20*time.Millisecond)
	for i := range 3 {
		select {
		case <-signals:
		case <-time.After(time.Second):
			t.Fatalf("signal %d did not come", i+1)
		}
	}
	cancel()
	for range signals {
	}
}
