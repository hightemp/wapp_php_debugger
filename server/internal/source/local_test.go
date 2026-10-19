package source_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/source/sourcetest"
)

// newLocal creates a logs directory with a v1 library config next to it, like in a real project.
func newLocal(t *testing.T, files map[string][]byte, config string) (*source.Local, string) {
	t.Helper()
	root := t.TempDir()
	logsDir := filepath.Join(root, "tmp", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(logsDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(root, "dws_debugger.json")
	if config != "" {
		if err := os.WriteFile(configPath, []byte(config), 0o664); err != nil {
			t.Fatal(err)
		}
		// WriteFile is subject to the umask (002 locally, 022 in CI); the tests need exactly 0664
		if err := os.Chmod(configPath, 0o664); err != nil {
			t.Fatal(err)
		}
	}
	return source.NewLocal(logsDir, configPath), configPath
}

const v1Config = `{"sProjectRootPath":"/var/www/app","sMessagesFilesPath":"/var/www/app/tmp/logs","bIsOnFlag":false,"iPageSize":15}`

func TestLocalSourceContract(t *testing.T) {
	sourcetest.Run(t, func(t *testing.T, files map[string][]byte) source.Source {
		src, _ := newLocal(t, files, v1Config)
		return src
	})
}

func readConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestLocalSetEnabledKeepsV1ConfigAndMode(t *testing.T) {
	src, configPath := newLocal(t, nil, v1Config)
	if err := src.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	config := readConfig(t, configPath)
	if config["bIsOnFlag"] != true || config["sMessagesFilesPath"] != "/var/www/app/tmp/logs" || config["iPageSize"] != float64(15) {
		t.Fatalf("config after SetEnabled = %v", config)
	}
	if _, ok := config["enabled"]; ok {
		t.Fatal("v1 config must keep using bIsOnFlag")
	}
	info, err := os.Stat(configPath)
	if err != nil || info.Mode().Perm() != 0o664 {
		t.Fatalf("config mode = %v, %v; want 0664 kept (PHP writes it as another user)", info.Mode().Perm(), err)
	}
}

func TestLocalUsesEnabledKeyForV2Config(t *testing.T) {
	src, configPath := newLocal(t, nil, `{"logs_dir":"tmp/logs","enabled":true}`)
	enabled, err := src.Enabled(context.Background())
	if err != nil || !enabled {
		t.Fatalf("Enabled = %v, %v", enabled, err)
	}
	if err := src.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if config := readConfig(t, configPath); config["enabled"] != false || config["logs_dir"] != "tmp/logs" {
		t.Fatalf("config = %v", config)
	}
}

func TestLocalWithoutConfig(t *testing.T) {
	src, _ := newLocal(t, nil, "")
	if enabled, err := src.Enabled(context.Background()); err != nil || enabled {
		t.Fatalf("missing config means the debugger is off: %v, %v", enabled, err)
	}
	// The developer creates the config with paths; the viewer does not invent it for them
	if err := src.SetEnabled(context.Background(), true); !errors.Is(err, source.ErrNoConfig) {
		t.Fatalf("SetEnabled err = %v, want ErrNoConfig", err)
	}
}

func TestLocalDoesNotFollowSymlinksOrListDirectories(t *testing.T) {
	src, configPath := newLocal(t, map[string][]byte{"1792400000000": []byte("log\n")}, v1Config)
	logsDir := filepath.Join(filepath.Dir(configPath), "tmp", "logs")
	if err := os.Symlink(configPath, filepath.Join(logsDir, "1792400000001")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(logsDir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}

	list, err := src.List(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "1792400000000" {
		t.Fatalf("List = %+v, %v; only regular files are logs", list, err)
	}
	if _, err := src.ReadAt(context.Background(), "1792400000001", 0, 100); !errors.Is(err, source.ErrNotFound) {
		t.Fatalf("ReadAt via symlink err = %v, want ErrNotFound", err)
	}
}
