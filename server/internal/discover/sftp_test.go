package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestFindsProjectsOverSFTP(t *testing.T) {
	server := sshtest.Start(t)
	www := t.TempDir()
	write(t, filepath.Join(www, "shop", "dws_debugger.json"), `{"enabled": true, "logs_dir": "var/logs"}`)
	if err := os.Symlink(t.TempDir(), filepath.Join(www, "link")); err != nil {
		t.Fatal(err)
	}

	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	_ = os.WriteFile(knownHosts, []byte(knownhosts.Line([]string{knownhosts.Normalize(server.Addr)}, server.HostKey)+"\n"), 0o600)
	dialer := &sshconn.Dialer{HostKeys: sshconn.NewHostKeys(filepath.Join(t.TempDir(), "trusted")), Timeout: 5 * time.Second}
	client, err := dialer.Dial(context.Background(), sshconn.Target{Host: server.Host, Port: server.Port, User: server.User,
		IdentityFiles: []string{server.ClientKeyFile}, KnownHostsFiles: []string{knownHosts}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	files, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })

	found, _, err := Find(context.Background(), SFTP{Client: files}, []string{www}, Options{})
	if err != nil || len(found) != 1 {
		t.Fatalf("found %+v, %v", found, err)
	}
	if shop := found[0]; shop.Name != "shop" || shop.LogsDir != www+"/shop/var/logs" || !shop.Enabled {
		t.Fatalf("shop = %+v", shop)
	}
}
