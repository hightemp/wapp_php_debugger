package source_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/source/sourcetest"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
	"golang.org/x/crypto/ssh/knownhosts"
)

// newSSH serves a project directory over a real SSH server: <root>/tmp/logs and <root>/dws_debugger.json.
func newSSH(t *testing.T, files map[string][]byte, config string) (*source.SSH, string) {
	t.Helper()
	src, root, _ := newSSHWithTarget(t, files, config, nil)
	return src, root
}

// newSSHWithTarget lets a test change the target before the source is made.
func newSSHWithTarget(t *testing.T, files map[string][]byte, config string, change func(*sshconn.Target)) (*source.SSH, string, *sshtest.Server) {
	t.Helper()
	server := sshtest.Start(t)
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
		if err := os.Chmod(configPath, 0o664); err != nil {
			t.Fatal(err)
		}
	}

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(server.Addr)}, server.HostKey)
	if err := os.WriteFile(knownHosts, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pool := sshconn.NewPool(&sshconn.Dialer{HostKeys: sshconn.NewHostKeys(filepath.Join(t.TempDir(), "trusted")), Timeout: 5 * time.Second}, sshconn.PoolOptions{})
	t.Cleanup(pool.Close)
	target := sshconn.Target{Host: server.Host, Port: server.Port, User: server.User, IdentityFiles: []string{server.ClientKeyFile}, KnownHostsFiles: []string{knownHosts}}
	if change != nil {
		change(&target)
	}
	return source.NewSSH(pool, 1, target, logsDir, configPath), root, server
}

func TestSSHSourceContract(t *testing.T) {
	sourcetest.Run(t, func(t *testing.T, files map[string][]byte) source.Source {
		src, _ := newSSH(t, files, `{"enabled": false, "logs_dir": "tmp/logs"}`)
		return src
	})
}

func TestSSHSetEnabledIsAtomicAndKeepsTheFile(t *testing.T) {
	src, root := newSSH(t, nil, `{"bIsOnFlag": false, "sMessagesFilesPath": "/var/www/app/tmp/logs"}`)
	if err := src.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "dws_debugger.json"))
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil || config["bIsOnFlag"] != true || config["sMessagesFilesPath"] != "/var/www/app/tmp/logs" {
		t.Fatalf("config = %s (%v)", data, err)
	}
	if info, _ := os.Stat(filepath.Join(root, "dws_debugger.json")); info.Mode().Perm() != 0o664 {
		t.Fatalf("mode %v, want 0664 kept", info.Mode().Perm())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(root, ".dws_debugger-*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left: %v", leftovers)
	}
}

func TestSSHWithoutConfig(t *testing.T) {
	src, _ := newSSH(t, nil, "")
	if enabled, err := src.Enabled(context.Background()); err != nil || enabled {
		t.Fatalf("Enabled = %v, %v", enabled, err)
	}
	if err := src.SetEnabled(context.Background(), true); err != source.ErrNoConfig {
		t.Fatalf("SetEnabled err = %v, want ErrNoConfig", err)
	}
}

func TestSSHDoesNotFollowSymlinksOrListDirectories(t *testing.T) {
	src, root := newSSH(t, map[string][]byte{"1792400000000": []byte("log\n")}, "")
	logsDir := filepath.Join(root, "tmp", "logs")
	secret := filepath.Join(root, "secret.txt")
	_ = os.WriteFile(secret, []byte("password"), 0o600)
	_ = os.Symlink(secret, filepath.Join(logsDir, "1792400000001"))
	_ = os.Mkdir(filepath.Join(logsDir, "1792400000002"), 0o755)

	list, err := src.List(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "1792400000000" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if _, err := src.ReadAt(context.Background(), "1792400000001", 0, 100); err != source.ErrNotFound {
		t.Fatalf("ReadAt through a symlink: %v, want ErrNotFound", err)
	}
	if err := src.Remove(context.Background(), "1792400000002"); err != source.ErrNotFound {
		t.Fatalf("Remove of a directory: %v, want ErrNotFound", err)
	}
}

func TestSSHUnreachableServerIsAnError(t *testing.T) {
	src, _, _ := newSSHWithTarget(t, nil, "", func(target *sshconn.Target) { target.Port = 1 }) // nothing listens there
	if _, err := src.List(context.Background()); err == nil {
		t.Fatal("want an error")
	}
}

func TestSSHProjectFiles(t *testing.T) {
	sourcetest.RunProjectFiles(t, func(t *testing.T, root string) source.ProjectFiles {
		src, _ := newSSH(t, nil, "")
		return src
	})
}
