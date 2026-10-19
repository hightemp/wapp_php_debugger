package app

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/v1", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Сквозной сценарий walking skeleton: лог в папке → импорт → API → событие SSE о новом логе.
func TestRunServesImportedLogsAndStreamsNewOnes(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "1792400000001"), fixture(t, "web-get.log"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{
			Addr:         "127.0.0.1:0",
			DBPath:       filepath.Join(dir, "viewer.db"),
			ProjectName:  "shop",
			RootPath:     dir,
			LogsDir:      logsDir,
			ConfigPath:   filepath.Join(dir, "dws_debugger.json"),
			ScanInterval: 20 * time.Millisecond,
			UI:           fstest.MapFS{"index.html": {Data: []byte("ui")}},
		}, func(addr string) { ready <- addr })
	}()

	var base string
	select {
	case addr := <-ready:
		base = "http://" + addr
	case err := <-done:
		t.Fatalf("Run exited: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}

	waitFor(t, func() bool {
		return strings.Contains(getBody(t, base+"/api/v1/requests"), `"request_key":"1792400000001"`)
	})

	streamCtx, stopStream := context.WithCancel(ctx)
	defer stopStream()
	request, _ := http.NewRequestWithContext(streamCtx, http.MethodGet, base+"/api/v1/stream", nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if err := os.WriteFile(filepath.Join(logsDir, "1792400000002"), fixture(t, "cli.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := nextEvent(t, bufio.NewReader(response.Body), "request_created")
	var event struct {
		Event   string `json:"event"`
		Request struct {
			RequestKey string `json:"request_key"`
			Sapi       string `json:"sapi"`
		} `json:"request"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil || event.Event != "request_created" ||
		event.Request.RequestKey != "1792400000002" || event.Request.Sapi != "cli" {
		t.Fatalf("event = %s (%v)", payload, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run must stop cleanly on cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	return string(body)
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func nextEvent(t *testing.T, reader *bufio.Reader, name string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	current := ""
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		if strings.HasPrefix(line, "event: ") {
			current = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") && current == name {
			return strings.TrimPrefix(line, "data: ")
		}
	}
	t.Fatalf("no %s event", name)
	return ""
}

func TestRunWithoutLogsDirStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{Addr: "127.0.0.1:0", DBPath: filepath.Join(dir, "viewer.db"), UI: fstest.MapFS{"index.html": {}}},
			func(addr string) { ready <- addr })
	}()
	addr := <-ready
	if body := getBody(t, "http://"+addr+"/api/v1/requests"); body != "{\"items\":[],\"next_cursor\":null}\n" {
		t.Fatalf("body = %q", body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunFailsOnBusyAddress(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	go func() {
		_ = Run(ctx, Config{Addr: "127.0.0.1:0", DBPath: filepath.Join(dir, "a.db"), UI: fstest.MapFS{}}, func(addr string) { ready <- addr })
	}()
	busy := <-ready
	err := Run(context.Background(), Config{Addr: busy, DBPath: filepath.Join(dir, "b.db"), UI: fstest.MapFS{}}, func(string) {})
	if err == nil {
		t.Fatal("second server on the same address must fail")
	}
}
