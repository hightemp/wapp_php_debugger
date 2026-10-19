package sshconn

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
)

type statusLog struct {
	mu     sync.Mutex
	states []string
}

func (l *statusLog) add(_ int64, status Status) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.states = append(l.states, status.State)
}

func (l *statusLog) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.states...)
}

func newPool(t *testing.T, keepalive time.Duration) (*Pool, *statusLog) {
	t.Helper()
	log := &statusLog{}
	pool := NewPool(dialer(t), PoolOptions{Keepalive: keepalive, OnStatus: log.add, FirstRetry: 50 * time.Millisecond})
	t.Cleanup(pool.Close)
	return pool, log
}

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if condition() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPoolSharesOneConnectionWithSFTP(t *testing.T) {
	server := sshtest.Start(t)
	pool, log := newPool(t, time.Minute)
	target := targetOf(server, knownHostsFile(t, server))
	path := filepath.Join(server.Root(), "1792400000000.log")
	if err := os.WriteFile(path, []byte("log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := pool.Get(context.Background(), 1, target); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	conn, err := pool.Get(context.Background(), 1, target)
	if err != nil {
		t.Fatal(err)
	}
	file, err := conn.SFTP.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(file)
	_ = file.Close()

	if string(data) != "log line\n" || server.Accepts() != 1 {
		t.Fatalf("data %q, connections %d", data, server.Accepts())
	}
	if got := log.get(); strings.Join(got, ",") != "connecting,online" || pool.Status(1).State != StateOnline {
		t.Fatalf("statuses %v", got)
	}
}

func TestPoolKeepsTheConnectionAlive(t *testing.T) {
	server := sshtest.Start(t)
	pool, _ := newPool(t, 30*time.Millisecond)
	if _, err := pool.Get(context.Background(), 1, targetOf(server, knownHostsFile(t, server))); err != nil {
		t.Fatal(err)
	}
	eventually(t, "keepalives", func() bool { return server.Keepalives() >= 2 })
}

func TestPoolNoticesADropAndReconnects(t *testing.T) {
	server := sshtest.Start(t)
	pool, log := newPool(t, 30*time.Millisecond)
	target := targetOf(server, knownHostsFile(t, server))
	if _, err := pool.Get(context.Background(), 1, target); err != nil {
		t.Fatal(err)
	}

	server.DropConnections()
	eventually(t, "offline", func() bool { return pool.Status(1).State == StateOffline })
	eventually(t, "a reconnect", func() bool {
		_, err := pool.Get(context.Background(), 1, target)
		return err == nil
	})
	if server.Accepts() != 2 || !strings.HasPrefix(strings.Join(log.get(), ","), "connecting,online,offline") {
		t.Fatalf("connections %d, statuses %v", server.Accepts(), log.get())
	}
}

func TestPoolBacksOffAfterAFailure(t *testing.T) {
	server := sshtest.Start(t)
	target := targetOf(server, knownHostsFile(t, server))
	server.Close()
	pool, _ := newPool(t, time.Minute)
	pool.options.FirstRetry = time.Hour

	if _, err := pool.Get(context.Background(), 1, target); err == nil {
		t.Fatal("want an error from a stopped server")
	}
	started := time.Now()
	_, err := pool.Get(context.Background(), 1, target)
	if err == nil || !strings.Contains(err.Error(), "retry in") || time.Since(started) > 100*time.Millisecond {
		t.Fatalf("second attempt: %v after %v; want the last error at once", err, time.Since(started))
	}
	if status := pool.Status(1); status.State != StateOffline || status.Error == "" {
		t.Fatalf("status %+v", status)
	}

	pool.Reset(1) // for example after the user confirmed a host key
	if _, err := pool.Get(context.Background(), 1, target); err == nil || strings.Contains(err.Error(), "retry in") {
		t.Fatalf("after reset it dials again: %v", err)
	}
}

func TestPoolDropClosesTheConnection(t *testing.T) {
	server := sshtest.Start(t)
	pool, _ := newPool(t, time.Minute)
	conn, err := pool.Get(context.Background(), 1, targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	pool.Drop(1)
	if _, err := conn.SFTP.Getwd(); err == nil {
		t.Fatal("the connection is still open after Drop")
	}
	if pool.Status(1).State != StateOffline {
		t.Fatalf("status %+v", pool.Status(1))
	}
}
