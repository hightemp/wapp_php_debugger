package sshconn

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Dialer opens SSH connections: keys from ssh-agent and key files, host keys checked by HostKeys,
// through the ProxyJump hosts of the target.
type Dialer struct {
	HostKeys    *HostKeys
	AgentSocket string        // SSH_AUTH_SOCK; empty means no agent
	Timeout     time.Duration // per hop: TCP connect and SSH handshake
}

// Dial connects to the target. Closing the returned client closes the jump connections too.
func (d *Dialer) Dial(ctx context.Context, target Target) (*ssh.Client, error) {
	var client *ssh.Client
	for _, hop := range append(append([]Target{}, target.Jumps...), target) {
		next, err := d.hop(ctx, client, hop)
		if err != nil {
			if client != nil {
				_ = client.Close()
			}
			return nil, err
		}
		if client != nil {
			previous := client
			go func() { _ = next.Wait(); _ = previous.Close() }()
		}
		client = next
	}
	return client, nil
}

func (d *Dialer) hop(ctx context.Context, through *ssh.Client, target Target) (*ssh.Client, error) {
	if target.User == "" {
		return nil, fmt.Errorf("%s: no user", target.Address())
	}
	hostKeys, err := d.HostKeys.callback(target.KnownHostsFiles)
	if err != nil {
		return nil, err
	}
	auth, closeAgent, err := d.auth(target)
	if err != nil {
		return nil, err
	}
	defer closeAgent()

	timeout := d.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var conn net.Conn
	if through == nil {
		conn, err = (&net.Dialer{}).DialContext(dialCtx, "tcp", target.Address())
	} else {
		conn, err = through.DialContext(dialCtx, "tcp", target.Address())
	}
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", target.Address(), err)
	}

	// The handshake has no context: closing the connection interrupts it
	type result struct {
		client *ssh.Client
		err    error
	}
	done := make(chan result, 1)
	go func() {
		c, channels, requests, err := ssh.NewClientConn(conn, target.Address(), &ssh.ClientConfig{
			User:            target.User,
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: hostKeys,
		})
		if err != nil {
			done <- result{err: err}
			return
		}
		done <- result{client: ssh.NewClient(c, channels, requests)}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ssh %s@%s: %w", target.User, target.Address(), r.err)
		}
		return r.client, nil
	case <-dialCtx.Done():
		_ = conn.Close()
		<-done
		return nil, fmt.Errorf("ssh %s: %w", target.Address(), dialCtx.Err())
	}
}

// auth offers the agent keys, then the key files. A key protected by a passphrase cannot be
// used directly: the user adds it to ssh-agent (asking for passphrases in the UI comes later).
func (d *Dialer) auth(target Target) (ssh.AuthMethod, func(), error) {
	var signers []ssh.Signer
	closeAgent := func() {}
	if d.AgentSocket != "" {
		if conn, err := net.Dial("unix", d.AgentSocket); err == nil {
			closeAgent = func() { _ = conn.Close() }
			if agentSigners, err := agent.NewClient(conn).Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}
	var locked []string
	for _, file := range target.IdentityFiles {
		data, err := os.ReadFile(file)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			closeAgent()
			return nil, nil, fmt.Errorf("key %s: %w", file, err)
		}
		signer, err := ssh.ParsePrivateKey(data)
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			locked = append(locked, file)
			continue
		}
		if err != nil {
			closeAgent()
			return nil, nil, fmt.Errorf("key %s: %w", file, err)
		}
		signers = append(signers, signer)
	}
	if len(signers) == 0 {
		closeAgent()
		if len(locked) > 0 {
			return nil, nil, fmt.Errorf("%s: the key %s is protected by a passphrase; add it to ssh-agent (ssh-add %s)",
				target.Address(), strings.Join(locked, ", "), locked[0])
		}
		return nil, nil, fmt.Errorf("%s: no SSH key: start ssh-agent with your key or set IdentityFile in ~/.ssh/config", target.Address())
	}
	return ssh.PublicKeys(signers...), closeAgent, nil
}
