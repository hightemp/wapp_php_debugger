package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/pkg/sftp"
)

// TestAgainstRealOpenSSH drives the viewer against a real OpenSSH server: make ssh-e2e starts one
// in Docker and sets WAPP_E2E_SSH_ADDR (host:port), WAPP_E2E_SSH_USER and WAPP_E2E_SSH_KEY.
// The path is the one a user takes: add the server, confirm its key, add a project, read its logs,
// switch the debugger.
func TestAgainstRealOpenSSH(t *testing.T) {
	addr, user, key := os.Getenv("WAPP_E2E_SSH_ADDR"), os.Getenv("WAPP_E2E_SSH_USER"), os.Getenv("WAPP_E2E_SSH_KEY")
	if addr == "" || user == "" || key == "" {
		t.Skip("set WAPP_E2E_SSH_ADDR, WAPP_E2E_SSH_USER and WAPP_E2E_SSH_KEY (make ssh-e2e)")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	knownHosts := filepath.Join(dir, "known_hosts")
	sshConfig := filepath.Join(dir, "ssh_config")
	_ = os.WriteFile(sshConfig, []byte(fmt.Sprintf(
		"Host e2e\n    HostName %s\n    Port %s\n    User %s\n    IdentityFile %s\n    IdentitiesOnly yes\n    UserKnownHostsFile %s\n",
		host, port, user, key, knownHosts)), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	// No agent: only the key of the test reaches the server
	cfgSSH := SSHConfig{ConfigFile: sshConfig, Home: dir, User: user, TrustedHostsFile: filepath.Join(dir, "trusted_hosts")}
	core, err := Start(ctx, Config{DBPath: filepath.Join(dir, "viewer.db"), RemoteScanInterval: 100 * time.Millisecond, SSH: cfgSSH})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); core.Close() })

	status, body := call(t, core, http.MethodPost, "/api/v1/servers", map[string]any{"name": "e2e", "kind": "ssh", "config": map[string]any{"host": "e2e"}})
	if status != http.StatusCreated {
		t.Fatalf("create server: %d %s", status, body)
	}
	var server struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &server)
	checkPath := fmt.Sprintf("/api/v1/servers/%d/check", server.ID)

	// A new host: its key is confirmed by fingerprint
	var check struct {
		Ok          bool   `json:"ok"`
		Fingerprint string `json:"host_key_fingerprint"`
		KeyType     string `json:"host_key_type"`
		Steps       []struct {
			Name     string `json:"name"`
			Ok       bool   `json:"ok"`
			Project  string `json:"project"`
			Optional bool   `json:"optional"`
			Detail   string `json:"detail"`
		} `json:"steps"`
	}
	_, body = call(t, core, http.MethodPost, checkPath, nil)
	_ = json.Unmarshal([]byte(body), &check)
	if check.Ok || !strings.HasPrefix(check.Fingerprint, "SHA256:") || check.KeyType == "" {
		t.Fatalf("first check: %s", body)
	}
	t.Logf("host key %s %s", check.KeyType, check.Fingerprint)
	if status, body := call(t, core, http.MethodPost, fmt.Sprintf("/api/v1/servers/%d/host-key", server.ID), map[string]any{"fingerprint": check.Fingerprint}); status != http.StatusNoContent {
		t.Fatalf("trust: %d %s", status, body)
	}

	// The project directory on the server, over a connection of the test that trusts the same keys
	conn := dialRemote(t, cfgSSH)
	home, err := conn.SFTP.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := path.Join(home, fmt.Sprintf("e2e-%d", time.Now().UnixNano()))
	logsDir := path.Join(root, "tmp", "logs")
	configPath := path.Join(root, "dws_debugger.json")
	t.Cleanup(func() { removeAll(conn, root) })
	if err := conn.SFTP.MkdirAll(logsDir); err != nil {
		t.Fatal(err)
	}
	_, here, _, _ := runtime.Caller(0)
	fixture, err := os.ReadFile(filepath.Join(filepath.Dir(here), "../../../spec/fixtures/v2-library/web-page.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	writeRemote(t, conn, path.Join(logsDir, "1792400000000-01M5AKMC85PGPCY0000000000.jsonl"), fixture, 0o644)
	writeRemote(t, conn, configPath, []byte(`{"enabled": false, "logs_dir": "tmp/logs"}`), 0o664)

	// The project is found by its config before it is added
	status, body = call(t, core, http.MethodPost, fmt.Sprintf("/api/v1/servers/%d/discover", server.ID), map[string]any{"roots": []string{home}})
	if status != http.StatusOK || !strings.Contains(body, `"logs_dir":"`+logsDir+`"`) {
		t.Fatalf("discover: %d %s", status, body)
	}

	status, body = call(t, core, http.MethodPost, "/api/v1/projects", map[string]any{
		"server_id": server.ID, "name": "shop", "root_path": root, "logs_dir": logsDir,
	})
	if status != http.StatusCreated {
		t.Fatalf("create project: %d %s", status, body)
	}
	var project struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &project)

	_, body = call(t, core, http.MethodPost, checkPath, nil)
	check.Steps = nil
	_ = json.Unmarshal([]byte(body), &check)
	if !check.Ok {
		t.Fatalf("check after trust: %s", body)
	}
	for _, want := range []string{"shop/logs_dir", "shop/logs_dir_writable", "shop/library_config"} {
		found := false
		for _, step := range check.Steps {
			found = found || (step.Project+"/"+step.Name == want && step.Ok)
		}
		if !found {
			t.Errorf("step %s did not pass: %s", want, body)
		}
	}

	// The container shares the host clock: the measured offset is about zero
	_, body = call(t, core, http.MethodGet, fmt.Sprintf("/api/v1/servers/%d", server.ID), nil)
	var got struct {
		Connection struct {
			ClockOffsetMs *int `json:"clock_offset_ms"`
		} `json:"connection"`
	}
	_ = json.Unmarshal([]byte(body), &got)
	if got.Connection.ClockOffsetMs == nil || *got.Connection.ClockOffsetMs > 2000 || *got.Connection.ClockOffsetMs < -2000 {
		t.Fatalf("clock offset of the server: %s", body)
	}
	t.Logf("clock offset %d ms", *got.Connection.ClockOffsetMs)

	waitFor(t, func() bool {
		_, list := call(t, core, http.MethodGet, "/api/v1/requests", nil)
		return strings.Contains(list, "01M5AKMC85PGPCY")
	})

	// The switch rewrites the config at once and keeps its mode
	if status, body := call(t, core, http.MethodPut, fmt.Sprintf("/api/v1/projects/%d/debugger", project.ID), map[string]any{"enabled": true}); status != http.StatusOK {
		t.Fatalf("debugger: %d %s", status, body)
	}
	data, err := readRemote(conn, configPath)
	if err != nil || !strings.Contains(string(data), `"enabled": true`) || !strings.Contains(string(data), `"logs_dir": "tmp/logs"`) {
		t.Fatalf("config after the switch: %s, %v", data, err)
	}
	info, err := conn.SFTP.Stat(configPath)
	if err != nil || info.Mode().Perm() != 0o664 {
		t.Fatalf("config mode %v, %v", info.Mode(), err)
	}
	entries, _ := conn.SFTP.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Errorf("temporary file left: %s", entry.Name())
		}
	}
}

func dialRemote(t *testing.T, cfg SSHConfig) *sshconn.Conn {
	t.Helper()
	resolver, err := cfg.resolver()
	if err != nil {
		t.Fatal(err)
	}
	target, err := resolver.ResolveServer(sshconn.ServerSettings{Host: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	dialer := &sshconn.Dialer{HostKeys: sshconn.NewHostKeys(cfg.TrustedHostsFile), Timeout: 10 * time.Second}
	client, err := dialer.Dial(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	files, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close(); _ = client.Close() })
	return &sshconn.Conn{SSH: client, SFTP: files}
}

func writeRemote(t *testing.T, conn *sshconn.Conn, name string, data []byte, mode os.FileMode) {
	t.Helper()
	file, err := conn.SFTP.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.SFTP.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

func readRemote(conn *sshconn.Conn, name string) ([]byte, error) {
	file, err := conn.SFTP.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var data strings.Builder
	_, err = file.WriteTo(&data)
	return []byte(data.String()), err
}

func removeAll(conn *sshconn.Conn, dir string) {
	walker := conn.SFTP.Walk(dir)
	var files, dirs []string
	for walker.Step() {
		if walker.Err() != nil {
			continue
		}
		if walker.Stat().IsDir() {
			dirs = append(dirs, walker.Path())
		} else {
			files = append(files, walker.Path())
		}
	}
	for _, file := range files {
		_ = conn.SFTP.Remove(file)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = conn.SFTP.RemoveDirectory(dirs[i])
	}
}
