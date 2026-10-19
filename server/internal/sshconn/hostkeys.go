package sshconn

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// UnknownHostError means no known_hosts file knows the host: the user confirms the fingerprint
// (trust on first use), then HostKeys.Trust remembers the key.
type UnknownHostError struct {
	Address     string
	Key         ssh.PublicKey
	Fingerprint string // SHA256:…, as ssh shows it
}

func (e *UnknownHostError) Error() string {
	return fmt.Sprintf("unknown host %s, key %s %s: confirm it to connect", e.Address, e.Key.Type(), e.Fingerprint)
}

// HostKeyChangedError means the host presents another key than known_hosts holds. It is never
// confirmed from the viewer: the user checks the server and fixes known_hosts by hand.
type HostKeyChangedError struct {
	Address     string
	Fingerprint string
}

func (e *HostKeyChangedError) Error() string {
	return fmt.Sprintf("host key of %s has changed (now %s): possible man-in-the-middle attack; "+
		"if the server was really reinstalled, remove its old key from known_hosts", e.Address, e.Fingerprint)
}

// HostKeys checks host keys against the user's known_hosts files and a file of keys confirmed in
// the viewer. Only the viewer's own file is ever written.
type HostKeys struct {
	trustedFile string
	mu          sync.Mutex
}

// NewHostKeys uses trustedFile for keys confirmed in the viewer.
func NewHostKeys(trustedFile string) *HostKeys {
	return &HostKeys{trustedFile: trustedFile}
}

// Trust remembers the key of a host the user has confirmed.
func (h *HostKeys) Trust(address string, key ssh.PublicKey) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(h.trustedFile), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(h.trustedFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(file, knownhosts.Line([]string{knownhosts.Normalize(address)}, key)); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// callback checks a host against the given known_hosts files and the trusted file.
func (h *HostKeys) callback(knownHostsFiles []string) (ssh.HostKeyCallback, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var files []string
	for _, file := range append(append([]string{}, knownHostsFiles...), h.trustedFile) {
		if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
			files = append(files, file)
		}
	}
	check := func(string, net.Addr, ssh.PublicKey) error {
		return &knownhosts.KeyError{} // no file at all: every host is unknown
	}
	if len(files) > 0 {
		var err error
		if check, err = knownhosts.New(files...); err != nil {
			return nil, fmt.Errorf("known_hosts: %w", err)
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := check(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				return &UnknownHostError{Address: hostname, Key: key, Fingerprint: ssh.FingerprintSHA256(key)}
			}
			return &HostKeyChangedError{Address: hostname, Fingerprint: ssh.FingerprintSHA256(key)}
		}
		return err
	}, nil
}
