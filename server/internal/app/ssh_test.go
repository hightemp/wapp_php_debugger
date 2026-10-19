package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"golang.org/x/crypto/ssh/knownhosts"
)

// A project on an SSH server: its logs come over SFTP and new files are picked up by polling.
func TestImportsTheLogsOfAProjectOverSSH(t *testing.T) {
	remote := sshtest.Start(t)
	dir := t.TempDir()
	logsDir := filepath.Join(t.TempDir(), "var", "www", "shop", "tmp", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "1792400000001"), fixture(t, "web-get.log"), 0o644); err != nil {
		t.Fatal(err)
	}

	knownHosts := filepath.Join(dir, "known_hosts")
	_ = os.WriteFile(knownHosts, []byte(knownhosts.Line([]string{knownhosts.Normalize(remote.Addr)}, remote.HostKey)+"\n"), 0o600)
	sshConfig := filepath.Join(dir, "ssh_config")
	_ = os.WriteFile(sshConfig, []byte(fmt.Sprintf("Host stage-web\n    HostName %s\n    Port %d\n    User %s\n    IdentityFile %s\n    UserKnownHostsFile %s\n",
		remote.Host, remote.Port, remote.User, remote.ClientKeyFile, knownHosts)), 0o600)

	dbPath := filepath.Join(dir, "viewer.db")
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	server, err := s.CreateServer(context.Background(), store.ServerInput{Name: "stage", Kind: "ssh", Config: `{"host":"stage-web"}`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(context.Background(), store.ProjectInput{ServerID: server.ID, Name: "shop", RootPath: filepath.Dir(filepath.Dir(logsDir)), LogsDir: logsDir}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	core, err := Start(ctx, Config{
		DBPath:             dbPath,
		RemoteScanInterval: 30 * time.Millisecond,
		SSH:                SSHConfig{ConfigFile: sshConfig, Home: dir, User: "dev", TrustedHostsFile: filepath.Join(dir, "trusted_hosts")},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); core.Close() })

	requests := func() string {
		recorder := httptest.NewRecorder()
		core.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil))
		return recorder.Body.String()
	}
	waitFor(t, func() bool { return strings.Contains(requests(), `"request_key":"1792400000001"`) })

	if err := os.WriteFile(filepath.Join(logsDir, "1792400000002"), fixture(t, "cli.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return strings.Contains(requests(), `"request_key":"1792400000002"`) })

	// A dropped connection goes to the stream as server_status
	stream := core.Broker().Subscribe(ctx, 0)
	remote.DropConnections()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-stream:
			if event.Name != "server_status" {
				continue
			}
			var payload struct {
				ServerID   int64 `json:"server_id"`
				Connection struct {
					State string `json:"state"`
					Error string `json:"error"`
				} `json:"connection"`
			}
			data, _ := json.Marshal(event.Data)
			_ = json.Unmarshal(data, &payload)
			if payload.ServerID != server.ID || payload.Connection.State != "offline" || payload.Connection.Error == "" {
				t.Fatalf("event %s", data)
			}
			return
		case <-deadline:
			t.Fatal("no server_status event")
		}
	}
}
