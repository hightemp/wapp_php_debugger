package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/version"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionCommand(t *testing.T) {
	for _, arg := range []string{"version", "--version"} {
		code, stdout, _ := run(arg)
		if code != 0 {
			t.Fatalf("%s: exit code = %d, want 0", arg, code)
		}
		if want := "wapp-debugger " + version.String() + "\n"; stdout != want {
			t.Fatalf("%s: stdout = %q, want %q", arg, stdout, want)
		}
	}
}

func TestHelpPrintsUsageToStdout(t *testing.T) {
	code, stdout, _ := run("help")
	if code != 0 || !strings.Contains(stdout, "Usage: wapp-debugger") {
		t.Fatalf("help: code=%d stdout=%q", code, stdout)
	}
}

func TestNoArgumentsIsUsageError(t *testing.T) {
	code, _, stderr := run()
	if code != 2 || !strings.Contains(stderr, "Usage: wapp-debugger") {
		t.Fatalf("no args: code=%d stderr=%q", code, stderr)
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	code, _, stderr := run("frobnicate")
	if code != 2 || !strings.Contains(stderr, `unknown command "frobnicate"`) || !strings.Contains(stderr, "Usage:") {
		t.Fatalf("unknown: code=%d stderr=%q", code, stderr)
	}
}

func TestParseServeDefaults(t *testing.T) {
	cfg, err := parseServe(nil, func() (string, error) { return "/home/u/.config", nil })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:8820" || cfg.DBPath != "/home/u/.config/wapp-debugger/wapp-debugger.db" || cfg.LogsDir != "" || cfg.ScanInterval != 5*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestTheUpdateCheckIsOffUnlessAsked(t *testing.T) {
	cfg, err := parseServe(nil, func() (string, error) { return "/home/u/.config", nil })
	if err != nil || cfg.UpdateCheck != nil {
		t.Fatalf("by default the server never calls GitHub: %v", err)
	}
	cfg, err = parseServe([]string{"--update-check"}, func() (string, error) { return "/home/u/.config", nil })
	if err != nil || cfg.UpdateCheck == nil || !cfg.UpdateCheck() {
		t.Fatalf("--update-check: %v", err)
	}
}

func TestParseServeLocalProject(t *testing.T) {
	cfg, err := parseServe([]string{"--logs-dir", "/var/www/shop/tmp/logs", "--root", "/var/www/shop", "--addr", "127.0.0.1:9000", "--db", "/tmp/x.db"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogsDir != "/var/www/shop/tmp/logs" || cfg.RootPath != "/var/www/shop" || cfg.Addr != "127.0.0.1:9000" || cfg.DBPath != "/tmp/x.db" {
		t.Fatalf("cfg = %+v", cfg)
	}
	// The project name and the library config default to values derived from the project root
	if cfg.ProjectName != "shop" || cfg.ConfigPath != "/var/www/shop/dws_debugger.json" {
		t.Fatalf("derived = %+v", cfg)
	}
}

func TestParseServeRootDefaultsToParentOfLogsDir(t *testing.T) {
	cfg, err := parseServe([]string{"--logs-dir", "/srv/app/logs", "--db", "/tmp/x.db"}, nil)
	if err != nil || cfg.RootPath != "/srv/app" || cfg.ProjectName != "app" {
		t.Fatalf("cfg = %+v, %v", cfg, err)
	}
}

func TestParseServeRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{{"--scan-interval", "0s", "--db", "x"}, {"--nope"}, {"extra-arg", "--db", "x"}} {
		if _, err := parseServe(args, nil); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}

func TestServeHelpListsFlags(t *testing.T) {
	code, _, stderr := run("serve", "--help")
	if code != 0 || !strings.Contains(stderr, "-logs-dir") || !strings.Contains(stderr, "-addr") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

// Logs usually live two levels deep (tmp/logs, tmpFiles/dws_debugger): the project root is the
// nearest parent of the logs directory that holds the library config.
func TestParseServeFindsTheRootByTheLibraryConfig(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	logs := filepath.Join(root, "tmp", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dws_debugger.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseServe([]string{"--logs-dir", logs, "--db", "/tmp/x.db"}, nil)
	if err != nil || cfg.RootPath != root || cfg.ProjectName != "shop" || cfg.ConfigPath != filepath.Join(root, "dws_debugger.json") {
		t.Fatalf("cfg = %+v, %v", cfg, err)
	}
}

func TestParseServeRefusesTheNetworkWithoutAToken(t *testing.T) {
	t.Setenv("WAPP_DEBUGGER_TOKEN", "")
	for _, addr := range []string{"0.0.0.0:8820", ":8820", "192.168.1.10:8820", "[::]:8820"} {
		if _, err := parseServe([]string{"--addr", addr, "--db", "/tmp/x.db"}, nil); err == nil || !strings.Contains(err.Error(), "token") {
			t.Errorf("%s: err = %v, want a token error", addr, err)
		}
	}
	for _, addr := range []string{"127.0.0.1:0", "localhost:8820", "[::1]:8820"} {
		if _, err := parseServe([]string{"--addr", addr, "--db", "/tmp/x.db"}, nil); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
	cfg, err := parseServe([]string{"--addr", "0.0.0.0:8820", "--token", "s3cret", "--db", "/tmp/x.db"}, nil)
	if err != nil || cfg.Token != "s3cret" {
		t.Fatalf("flag token: %+v, %v", cfg, err)
	}
}

// The environment keeps the token out of the process list
func TestParseServeTakesTheTokenFromTheEnvironment(t *testing.T) {
	t.Setenv("WAPP_DEBUGGER_TOKEN", "from-env")
	cfg, err := parseServe([]string{"--addr", "0.0.0.0:8820", "--db", "/tmp/x.db"}, nil)
	if err != nil || cfg.Token != "from-env" {
		t.Fatalf("cfg = %+v, %v", cfg, err)
	}
}
