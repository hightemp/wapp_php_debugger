package source_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
)

// phpApp serves a tiny application with the one-file build of the library under php -S.
func phpApp(t *testing.T) (baseURL string) {
	t.Helper()
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("php is not installed")
	}
	library, err := filepath.Abs("../../../client-php")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if out, err := exec.Command(php, filepath.Join(library, "tools/bin/bundle"), filepath.Join(library, "src"), filepath.Join(dir, "wapp-debugger.php")).CombinedOutput(); err != nil {
		t.Fatalf("bundle: %v\n%s", err, out)
	}
	config := fmt.Sprintf(`{"enabled": false, "project_root": %q, "logs_dir": %q, "http_token": %q}`, dir, filepath.Join(dir, "logs"), libToken)
	app := `<?php
require __DIR__ . '/wapp-debugger.php';
\WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
\WappDebugger\Debugger::instance()->info('hello from the app');
echo 'app';
`
	for name, content := range map[string]string{"dws_debugger.json": config, "index.php": app} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	server := exec.Command(php, "-d", "display_errors=0", "-S", addr, "index.php")
	server.Dir = dir
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Process.Kill(); _ = server.Wait() })
	baseURL = "http://" + addr
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if response, err := http.Get(baseURL + "/ready"); err == nil {
			_ = response.Body.Close()
			return baseURL
		}
		if time.Now().After(deadline) {
			t.Fatal("php -S did not start")
		}
	}
}

func TestHTTPAgainstTheRealLibrary(t *testing.T) {
	baseURL := phpApp(t)
	ctx := context.Background()
	src, err := source.NewHTTP(source.HTTPOptions{BaseURL: baseURL, Token: libToken})
	if err != nil {
		t.Fatal(err)
	}

	status, err := src.Status(ctx)
	if err != nil || status.Enabled || status.LibraryVersion == "" {
		t.Fatalf("status = %+v, %v", status, err)
	}
	if err := src.SetEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := src.Enabled(ctx); err != nil || !enabled {
		t.Fatalf("Enabled = %v, %v", enabled, err)
	}

	response, err := http.Get(baseURL + "/orders?id=7")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "app" {
		t.Fatalf("the application answered %q", body)
	}

	files, err := src.List(ctx)
	if err != nil || len(files) != 1 || files[0].Size == 0 {
		t.Fatalf("List = %+v, %v", files, err)
	}
	log, err := src.ReadAt(ctx, files[0].Name, 0, 1<<20)
	if err != nil || int64(len(log)) != files[0].Size || !strings.Contains(string(log), "hello from the app") {
		t.Fatalf("ReadAt = %q, %v", log, err)
	}
	if _, err := src.ReadAt(ctx, files[0].Name, files[0].Size+1, 10); !errors.Is(err, source.ErrOffsetBeyondEnd) {
		t.Fatalf("beyond the end: %v", err)
	}
	if _, err := src.ReadAt(ctx, "1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl", 0, 10); !errors.Is(err, source.ErrNotFound) {
		t.Fatalf("missing file: %v", err)
	}
	if err := src.Remove(ctx, files[0].Name); err != nil {
		t.Fatal(err)
	}
	if files, err := src.List(ctx); err != nil || len(files) != 0 {
		t.Fatalf("after Remove: %+v, %v", files, err)
	}

	stranger, _ := source.NewHTTP(source.HTTPOptions{BaseURL: baseURL, Token: strings.Repeat("x", 32)})
	if _, err := stranger.List(ctx); !errors.Is(err, source.ErrNoLibrary) {
		t.Fatalf("wrong token: %v, want ErrNoLibrary", err)
	}
}
