// Package sshtest runs a real SSH server inside a test: public key auth, the sftp subsystem over a
// directory, exec with canned answers, direct-tcpip forwarding (for ProxyJump) and keepalive.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Server is a running test SSH server.
type Server struct {
	Addr          string
	Host          string
	Port          int
	HostKey       ssh.PublicKey
	User          string
	ClientKeyFile string // OpenSSH private key accepted by the server

	root         string
	execs        map[string]string
	maxAuthTries int
	authKeys     []ssh.PublicKey
	listener     net.Listener
	mu           sync.Mutex
	conns        map[*ssh.ServerConn]struct{}
	forwards     atomic.Int64
	keepalive    atomic.Int64
	accepts      atomic.Int64
}

// Option changes the server before it starts.
type Option func(*Server)

// WithExec answers the exec request command with output and exit status 0.
func WithExec(command, output string) Option {
	return func(s *Server) { s.execs[command] = output }
}

// WithMaxAuthTries limits authentication attempts per connection, like MaxAuthTries of sshd (6 by default there).
func WithMaxAuthTries(tries int) Option {
	return func(s *Server) { s.maxAuthTries = tries }
}

// WithAuthorizedKey accepts another public key (for example one held by an agent).
func WithAuthorizedKey(key ssh.PublicKey) Option {
	return func(s *Server) { s.authKeys = append(s.authKeys, key) }
}

// Start listens on 127.0.0.1 and stops with the test.
func Start(t *testing.T, options ...Option) *Server {
	t.Helper()
	dir := t.TempDir()
	hostSigner := NewSigner(t)
	clientSigner, clientFile := NewKeyFile(t, dir, "id_ed25519", "")
	s := &Server{
		HostKey:       hostSigner.PublicKey(),
		User:          "deploy",
		ClientKeyFile: clientFile,
		root:          t.TempDir(),
		execs:         map[string]string{},
		authKeys:      []ssh.PublicKey{clientSigner.PublicKey()},
		conns:         map[*ssh.ServerConn]struct{}{},
	}
	for _, option := range options {
		option(s)
	}

	config := &ssh.ServerConfig{
		MaxAuthTries: s.maxAuthTries,
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if meta.User() != s.User {
				return nil, errDenied
			}
			for _, allowed := range s.authKeys {
				if string(allowed.Marshal()) == string(key.Marshal()) {
					return &ssh.Permissions{}, nil
				}
			}
			return nil, errDenied
		},
	}
	config.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.listener = listener
	s.Addr = listener.Addr().String()
	host, port, _ := net.SplitHostPort(s.Addr)
	s.Host = host
	s.Port, _ = strconv.Atoi(port)
	go s.serve(config)
	t.Cleanup(s.Close)
	return s
}

// Accepts counts SSH connections that passed the handshake.
func (s *Server) Accepts() int64 { return s.accepts.Load() }

// Root is a directory for files served over sftp (any absolute path works: sftp serves the file system).
func (s *Server) Root() string { return s.root }

// Forwards counts direct-tcpip channels: connections made through this server as a jump host.
func (s *Server) Forwards() int64 { return s.forwards.Load() }

// Keepalives counts keepalive@openssh.com requests.
func (s *Server) Keepalives() int64 { return s.keepalive.Load() }

// DropConnections closes every client connection, like a network failure; the server keeps listening.
func (s *Server) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

// Close stops the server.
func (s *Server) Close() {
	_ = s.listener.Close()
	s.DropConnections()
}

type denied struct{}

func (denied) Error() string { return "access denied" }

var errDenied = denied{}

func (s *Server) serve(config *ssh.ServerConfig) {
	for {
		raw, err := s.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			conn, channels, requests, err := ssh.NewServerConn(raw, config)
			if err != nil {
				_ = raw.Close()
				return
			}
			s.accepts.Add(1)
			s.mu.Lock()
			s.conns[conn] = struct{}{}
			s.mu.Unlock()
			go s.globalRequests(requests)
			for newChannel := range channels {
				switch newChannel.ChannelType() {
				case "session":
					go s.session(newChannel)
				case "direct-tcpip":
					go s.forward(newChannel)
				default:
					_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported")
				}
			}
			s.mu.Lock()
			delete(s.conns, conn)
			s.mu.Unlock()
		}()
	}
}

func (s *Server) globalRequests(requests <-chan *ssh.Request) {
	for request := range requests {
		if request.Type == "keepalive@openssh.com" {
			s.keepalive.Add(1)
		}
		if request.WantReply {
			_ = request.Reply(request.Type == "keepalive@openssh.com", nil)
		}
	}
}

func (s *Server) session(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer func() { _ = channel.Close() }()
	for request := range requests {
		switch request.Type {
		case "subsystem":
			if len(request.Payload) < 4 || string(request.Payload[4:]) != "sftp" {
				_ = request.Reply(false, nil)
				continue
			}
			_ = request.Reply(true, nil)
			server, err := sftp.NewServer(channel)
			if err != nil {
				return
			}
			_ = server.Serve()
			return
		case "exec":
			command := string(request.Payload[4:])
			output, known := s.execs[command]
			_ = request.Reply(true, nil)
			status := uint32(0)
			if known {
				_, _ = io.WriteString(channel, output)
			} else {
				_, _ = io.WriteString(channel.Stderr(), command+": command not found\n")
				status = 127
			}
			_, _ = channel.SendRequest("exit-status", false, binary.BigEndian.AppendUint32(nil, status))
			return
		default:
			if request.WantReply {
				_ = request.Reply(false, nil)
			}
		}
	}
}

func (s *Server) forward(newChannel ssh.NewChannel) {
	var payload struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, "bad payload")
		return
	}
	target, err := net.Dial("tcp", net.JoinHostPort(payload.Host, strconv.Itoa(int(payload.Port))))
	if err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		_ = target.Close()
		return
	}
	s.forwards.Add(1)
	go ssh.DiscardRequests(requests)
	go func() {
		_, _ = io.Copy(channel, target)
		_ = channel.CloseWrite()
	}()
	_, _ = io.Copy(target, channel)
	_ = target.Close()
	_ = channel.Close()
}

var privateKeys sync.Map // ssh.Signer → ed25519.PrivateKey, for test agents

// NewSigner makes a fresh ed25519 key.
func NewSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	privateKeys.Store(signer, private)
	return signer
}

// PrivateKey returns the key behind a signer made by NewSigner, to load it into a test agent.
func PrivateKey(signer ssh.Signer) (ed25519.PrivateKey, bool) {
	key, ok := privateKeys.Load(signer)
	if !ok {
		return nil, false
	}
	private, ok := key.(ed25519.PrivateKey)
	return private, ok
}

// NewKeyFile writes a fresh ed25519 key in the OpenSSH format (with a passphrase when one is given).
func NewKeyFile(t *testing.T, dir, name, passphrase string) (ssh.Signer, string) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(private, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer, path
}
