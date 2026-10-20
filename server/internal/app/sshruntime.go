package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// sshRuntime is api.SSHRuntime over the shared pool.
type sshRuntime struct {
	pool   *sshconn.Pool
	dialer *sshconn.Dialer
	config SSHConfig
}

var _ api.SSHRuntime = (*sshRuntime)(nil)

func (r *sshRuntime) target(server store.Server) (sshconn.Target, error) {
	settings, err := sshconn.ParseServerSettings(server.Config)
	if err != nil {
		return sshconn.Target{}, err
	}
	resolver, err := r.config.resolver()
	if err != nil {
		return sshconn.Target{}, err
	}
	return resolver.ResolveServer(settings)
}

func (r *sshRuntime) Status(serverID int64) sshconn.Status {
	return r.pool.Status(serverID)
}

// Check connects right away (no back-off) and reports each step; an unknown host key comes with
// its fingerprint for the user to confirm. Then it looks at the logs directory and the library
// config of each project and asks the server for its PHP version.
func (r *sshRuntime) Check(ctx context.Context, server store.Server, projects []store.Project) (api.ServerCheck, error) {
	check := api.ServerCheck{Ok: true}
	add := func(s api.ServerCheckStep, err error, detail string) bool {
		s.Ok = err == nil
		if err != nil {
			detail = err.Error()
			if s.Optional == nil {
				check.Ok = false
			}
		}
		if detail != "" {
			s.Detail = &detail
		}
		check.Steps = append(check.Steps, s)
		return err == nil
	}
	step := func(name string, err error, detail string) bool {
		return add(api.ServerCheckStep{Name: name}, err, detail)
	}
	optional := true

	target, err := r.target(server)
	if !step("config", err, "") {
		return check, nil
	}
	r.pool.Reset(server.ID)
	conn, err := r.pool.Get(ctx, server.ID, target)
	var unknown *sshconn.UnknownHostError
	if errors.As(err, &unknown) {
		check.HostKeyFingerprint = &unknown.Fingerprint
		keyType := unknown.Key.Type()
		check.HostKeyType = &keyType
	}
	if !step("connect", err, fmt.Sprintf("%s@%s", target.User, target.Address())) {
		return check, nil
	}
	_, err = conn.SFTP.Getwd()
	if !step("sftp", err, "") {
		return check, nil
	}
	for _, project := range projects {
		name := project.Name
		readable := add(api.ServerCheckStep{Name: "logs_dir", Project: &name}, readableDir(conn.SFTP, project.LogsDir), project.LogsDir)
		if readable && !server.ReadOnly {
			add(api.ServerCheckStep{Name: "logs_dir_writable", Project: &name}, writableDir(conn.SFTP, project.LogsDir), "")
		}
		_, err := conn.SFTP.Stat(project.ConfigPath)
		add(api.ServerCheckStep{Name: "library_config", Project: &name, Optional: &optional}, err, project.ConfigPath)
	}
	version, err := phpVersion(ctx, conn.SSH)
	add(api.ServerCheckStep{Name: "php", Optional: &optional}, err, version)
	return check, nil
}

func readableDir(client *sftp.Client, dir string) error {
	info, err := client.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if _, err := client.ReadDir(dir); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	return nil
}

// writableDir creates and removes a file in dir: the mode bits do not tell what this user may do.
func writableDir(client *sftp.Client, dir string) error {
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	name := path.Join(dir, ".wapp-debugger-check-"+hex.EncodeToString(suffix))
	file, err := client.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	_ = file.Close()
	return client.Remove(name)
}

// phpVersion runs php -v on the server: the first line of its answer.
func phpVersion(ctx context.Context, client *ssh.Client) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("exec is not allowed: %w", err)
	}
	defer func() { _ = session.Close() }()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	var stderr strings.Builder
	session.Stderr = &stderr
	output, err := session.Output("php -v")
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", fmt.Errorf("php -v: %s", message)
		}
		return "", fmt.Errorf("php -v: %w", err)
	}
	first, _, _ := strings.Cut(string(output), "\n")
	return strings.TrimSpace(first), nil
}

// TrustHostKey connects again and trusts the key only if it still has the confirmed fingerprint.
func (r *sshRuntime) TrustHostKey(ctx context.Context, server store.Server, fingerprint string) error {
	target, err := r.target(server)
	if err != nil {
		return err
	}
	client, err := r.dialer.Dial(ctx, target)
	if err == nil {
		return client.Close() // every key on the way is known already
	}
	var unknown *sshconn.UnknownHostError
	var changed *sshconn.HostKeyChangedError
	switch {
	case errors.As(err, &unknown):
		if unknown.Fingerprint != fingerprint {
			return fmt.Errorf("%w: %s presents %s", api.ErrHostKeyMismatch, unknown.Address, unknown.Fingerprint)
		}
		if err := r.dialer.HostKeys.Trust(unknown.Address, unknown.Key); err != nil {
			return err
		}
		r.pool.Reset(server.ID)
		return nil
	case errors.As(err, &changed):
		return fmt.Errorf("%w: %v", api.ErrHostKeyChanged, err)
	default:
		return err
	}
}

func (r *sshRuntime) Forget(serverID int64) {
	r.pool.Drop(serverID)
	r.pool.Reset(serverID)
}
