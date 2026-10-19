package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshRuntimeEnv: an empty known_hosts, so every host starts unknown.
func sshRuntimeEnv(t *testing.T, sshConfig string) *sshRuntime {
	t.Helper()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "ssh_config")
	if err := os.WriteFile(configFile, []byte(sshConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := SSHConfig{ConfigFile: configFile, Home: dir, User: "dev", TrustedHostsFile: filepath.Join(dir, "trusted_hosts")}
	dialer := &sshconn.Dialer{HostKeys: sshconn.NewHostKeys(cfg.TrustedHostsFile), Timeout: 5 * time.Second}
	pool := sshconn.NewPool(dialer, sshconn.PoolOptions{})
	t.Cleanup(pool.Close)
	return &sshRuntime{pool: pool, dialer: dialer, config: cfg}
}

func hostEntry(alias string, s *sshtest.Server, knownHosts string) string {
	return fmt.Sprintf("Host %s\n    HostName %s\n    Port %d\n    User %s\n    IdentityFile %s\n    UserKnownHostsFile %s\n",
		alias, s.Host, s.Port, s.User, s.ClientKeyFile, knownHosts)
}

func TestCheckAndTrustAnUnknownHost(t *testing.T) {
	remote := sshtest.Start(t)
	empty := filepath.Join(t.TempDir(), "known_hosts")
	r := sshRuntimeEnv(t, hostEntry("stage-web", remote, empty))
	server := store.Server{ID: 2, Kind: "ssh", Config: `{"host":"stage-web"}`}

	check, err := r.Check(context.Background(), server)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := ssh.FingerprintSHA256(remote.HostKey)
	if check.Ok || check.HostKeyFingerprint == nil || *check.HostKeyFingerprint != fingerprint {
		t.Fatalf("check = %+v", check)
	}

	impostor := ssh.FingerprintSHA256(sshtest.NewSigner(t).PublicKey())
	if err := r.TrustHostKey(context.Background(), server, impostor); !errors.Is(err, api.ErrHostKeyMismatch) {
		t.Fatalf("another fingerprint: %v, want ErrHostKeyMismatch", err)
	}
	if err := r.TrustHostKey(context.Background(), server, fingerprint); err != nil {
		t.Fatal(err)
	}
	check, err = r.Check(context.Background(), server)
	if err != nil || !check.Ok || len(check.Steps) < 2 {
		t.Fatalf("after trust: %+v, %v", check, err)
	}
	if r.Status(2).State != sshconn.StateOnline {
		t.Fatalf("status %+v", r.Status(2))
	}
	if err := r.TrustHostKey(context.Background(), server, fingerprint); err != nil {
		t.Fatalf("trusting a known host again is fine: %v", err)
	}
}

func TestAChangedHostKeyIsNeverTrusted(t *testing.T) {
	remote := sshtest.Start(t)
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	old := sshtest.NewSigner(t).PublicKey()
	_ = os.WriteFile(knownHosts, []byte(knownhosts.Line([]string{knownhosts.Normalize(remote.Addr)}, old)+"\n"), 0o600)
	r := sshRuntimeEnv(t, hostEntry("stage-web", remote, knownHosts))
	server := store.Server{ID: 2, Kind: "ssh", Config: `{"host":"stage-web"}`}

	check, _ := r.Check(context.Background(), server)
	if check.Ok || check.HostKeyFingerprint != nil {
		t.Fatalf("a changed key offers no fingerprint to confirm: %+v", check)
	}
	if err := r.TrustHostKey(context.Background(), server, ssh.FingerprintSHA256(remote.HostKey)); !errors.Is(err, api.ErrHostKeyChanged) {
		t.Fatalf("err = %v, want ErrHostKeyChanged", err)
	}
}

func TestJumpHostKeysAreConfirmedOneByOne(t *testing.T) {
	bastion := sshtest.Start(t)
	remote := sshtest.Start(t)
	empty := filepath.Join(t.TempDir(), "known_hosts")
	r := sshRuntimeEnv(t, hostEntry("bastion", bastion, empty)+hostEntry("stage-web", remote, empty)+"    ProxyJump bastion\n")
	server := store.Server{ID: 2, Kind: "ssh", Config: `{"host":"stage-web"}`}

	for _, host := range []*sshtest.Server{bastion, remote} {
		check, _ := r.Check(context.Background(), server)
		if check.HostKeyFingerprint == nil || *check.HostKeyFingerprint != ssh.FingerprintSHA256(host.HostKey) {
			t.Fatalf("check = %+v; want the key of %s", check, host.Addr)
		}
		if err := r.TrustHostKey(context.Background(), server, *check.HostKeyFingerprint); err != nil {
			t.Fatal(err)
		}
	}
	if check, err := r.Check(context.Background(), server); err != nil || !check.Ok {
		t.Fatalf("after both: %+v, %v", check, err)
	}
}
