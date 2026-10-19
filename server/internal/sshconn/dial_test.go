package sshconn

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsFile writes a user known_hosts trusting the given servers.
func knownHostsFile(t *testing.T, servers ...*sshtest.Server) string {
	t.Helper()
	var lines []string
	for _, s := range servers {
		lines = append(lines, knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, s.HostKey))
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func targetOf(s *sshtest.Server, knownHosts string) Target {
	return Target{Alias: s.Host, Host: s.Host, Port: s.Port, User: s.User, IdentityFiles: []string{s.ClientKeyFile}, KnownHostsFiles: []string{knownHosts}}
}

func dialer(t *testing.T) *Dialer {
	t.Helper()
	return &Dialer{HostKeys: NewHostKeys(filepath.Join(t.TempDir(), "trusted_hosts")), Timeout: 5 * time.Second}
}

func run(t *testing.T, client *ssh.Client, command string) string {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	output, err := session.Output(command)
	if err != nil {
		t.Fatalf("%s: %v", command, err)
	}
	return string(output)
}

func TestDialWithAKeyFile(t *testing.T) {
	server := sshtest.Start(t, sshtest.WithExec("echo hi", "hi\n"))
	client, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if got := run(t, client, "echo hi"); got != "hi\n" {
		t.Fatalf("output %q", got)
	}
}

func TestUnknownHostIsConfirmedOnce(t *testing.T) {
	server := sshtest.Start(t)
	userKnownHosts := knownHostsFile(t) // empty
	d := dialer(t)

	_, err := d.Dial(context.Background(), targetOf(server, userKnownHosts))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want UnknownHostError", err)
	}
	if unknown.Fingerprint != ssh.FingerprintSHA256(server.HostKey) || unknown.Address != server.Addr {
		t.Fatalf("unknown = %+v", unknown)
	}
	if err := d.HostKeys.Trust(unknown.Address, unknown.Key); err != nil {
		t.Fatal(err)
	}
	client, err := d.Dial(context.Background(), targetOf(server, userKnownHosts))
	if err != nil {
		t.Fatalf("after trust: %v", err)
	}
	_ = client.Close()
	if data, _ := os.ReadFile(userKnownHosts); strings.TrimSpace(string(data)) != "" {
		t.Fatalf("the user's known_hosts must stay untouched: %q", data)
	}
}

func TestChangedHostKeyIsRefused(t *testing.T) {
	server := sshtest.Start(t)
	impostor := sshtest.NewSigner(t)
	path := filepath.Join(t.TempDir(), "known_hosts")
	_ = os.WriteFile(path, []byte(knownhosts.Line([]string{knownhosts.Normalize(server.Addr)}, impostor.PublicKey())+"\n"), 0o600)

	_, err := dialer(t).Dial(context.Background(), targetOf(server, path))
	var changed *HostKeyChangedError
	if !errors.As(err, &changed) || changed.Fingerprint != ssh.FingerprintSHA256(server.HostKey) {
		t.Fatalf("err = %v, want HostKeyChangedError", err)
	}
	if !strings.Contains(err.Error(), "man-in-the-middle") {
		t.Fatalf("the message should warn: %v", err)
	}
}

func TestEncryptedKeyFileAsksForTheAgent(t *testing.T) {
	server := sshtest.Start(t)
	_, keyFile := sshtest.NewKeyFile(t, t.TempDir(), "id_locked", "secret")
	target := targetOf(server, knownHostsFile(t, server))
	target.IdentityFiles = []string{keyFile}

	_, err := dialer(t).Dial(context.Background(), target)
	if err == nil || !strings.Contains(err.Error(), "ssh-agent") {
		t.Fatalf("err = %v, want a hint about ssh-agent", err)
	}
}

func TestDialWithTheAgent(t *testing.T) {
	signer := sshtest.NewSigner(t)
	server := sshtest.Start(t, sshtest.WithAuthorizedKey(signer.PublicKey()))
	socket := startAgent(t, signer)

	target := targetOf(server, knownHostsFile(t, server))
	target.IdentityFiles = []string{filepath.Join(t.TempDir(), "missing_key")} // missing files are skipped
	d := dialer(t)
	d.AgentSocket = socket
	client, err := d.Dial(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

func TestProxyJump(t *testing.T) {
	bastion := sshtest.Start(t)
	server := sshtest.Start(t, sshtest.WithExec("hostname", "app-1\n"))
	knownHosts := knownHostsFile(t, bastion, server)
	target := targetOf(server, knownHosts)
	target.Jumps = []Target{targetOf(bastion, knownHosts)}

	client, err := dialer(t).Dial(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if got := run(t, client, "hostname"); got != "app-1\n" || bastion.Forwards() != 1 {
		t.Fatalf("output %q, forwards through the bastion %d", got, bastion.Forwards())
	}
}

func TestWrongKeyIsAnAuthError(t *testing.T) {
	server := sshtest.Start(t)
	_, other := sshtest.NewKeyFile(t, t.TempDir(), "id_other", "")
	target := targetOf(server, knownHostsFile(t, server))
	target.IdentityFiles = []string{other}

	_, err := dialer(t).Dial(context.Background(), target)
	if err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("err = %v", err)
	}
}

func TestDialRespectsTheContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") // accepts but never speaks SSH
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	host, _, _ := net.SplitHostPort(listener.Addr().String())
	started := time.Now()
	_, err = dialer(t).Dial(ctx, Target{Host: host, Port: listener.Addr().(*net.TCPAddr).Port, User: "x"})
	if err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(started))
	}
}

func startAgent(t *testing.T, signers ...ssh.Signer) string {
	t.Helper()
	keyring := agent.NewKeyring()
	for _, signer := range signers {
		// The keyring needs the private key itself; the test signers wrap ed25519 keys
		key, ok := sshtest.PrivateKey(signer)
		if !ok {
			t.Fatal("no private key for the agent")
		}
		if err := keyring.Add(agent.AddedKey{PrivateKey: key}); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.MkdirTemp("", "agent")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "agent.sock") // short path: unix sockets have a length limit
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { _ = agent.ServeAgent(keyring, conn); _ = conn.Close() }()
		}
	}()
	return socket
}

// sshd drops a client after MaxAuthTries failed keys: the configured key goes before the agent's
// other keys, as in OpenSSH, or a developer with many keys in the agent could never connect.
func TestTheConfiguredKeyGoesBeforeTheAgentKeys(t *testing.T) {
	server := sshtest.Start(t, sshtest.WithMaxAuthTries(3))
	var others []ssh.Signer
	for range 6 {
		others = append(others, sshtest.NewSigner(t))
	}
	d := dialer(t)
	d.AgentSocket = startAgent(t, others...)

	client, err := d.Dial(context.Background(), targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

// IdentitiesOnly yes: only the configured keys, the agent's others are not offered.
func TestIdentitiesOnly(t *testing.T) {
	allowed := sshtest.NewSigner(t)
	server := sshtest.Start(t, sshtest.WithAuthorizedKey(allowed.PublicKey()))
	d := dialer(t)
	d.AgentSocket = startAgent(t, allowed)

	target := targetOf(server, knownHostsFile(t, server))
	_, otherFile := sshtest.NewKeyFile(t, t.TempDir(), "id_other", "")
	target.IdentityFiles = []string{otherFile}
	target.IdentitiesOnly = true
	if _, err := d.Dial(context.Background(), target); err == nil {
		t.Fatal("the agent key is not configured: it must not be offered")
	}
	target.IdentitiesOnly = false
	client, err := d.Dial(context.Background(), target)
	if err != nil {
		t.Fatalf("without IdentitiesOnly the agent key works: %v", err)
	}
	_ = client.Close()
}

// A new host shows its ed25519 key: that is the fingerprint people usually compare.
func TestANewHostShowsItsEd25519Key(t *testing.T) {
	server := sshtest.Start(t, sshtest.WithECDSAHostKey())
	_, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t)))
	var unknown *UnknownHostError
	if !errors.As(err, &unknown) || unknown.Key.Type() != ssh.KeyAlgoED25519 {
		t.Fatalf("err = %v; want the ed25519 key", err)
	}
}

// known_hosts holding a key of another type than the client would pick is not a changed key: the
// known type is asked for first, as OpenSSH does.
func TestAKnownKeyOfAnotherTypeIsAskedFor(t *testing.T) {
	server := sshtest.Start(t, sshtest.WithECDSAHostKey())
	path := filepath.Join(t.TempDir(), "known_hosts")
	_ = os.WriteFile(path, []byte(knownhosts.Line([]string{knownhosts.Normalize(server.Addr)}, server.HostKeyECDSA)+"\n"), 0o600)

	client, err := dialer(t).Dial(context.Background(), targetOf(server, path))
	if err != nil {
		t.Fatalf("a known ECDSA key: %v", err)
	}
	_ = client.Close()
}

func TestAKnownEd25519KeyIsAskedFor(t *testing.T) {
	server := sshtest.Start(t, sshtest.WithECDSAHostKey())
	client, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t, server))) // the ed25519 key
	if err != nil {
		t.Fatalf("a known ed25519 key: %v", err)
	}
	_ = client.Close()
}
