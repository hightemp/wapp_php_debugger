package app

import (
	"context"
	"errors"
	"fmt"

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
// its fingerprint for the user to confirm.
func (r *sshRuntime) Check(ctx context.Context, server store.Server) (api.ServerCheck, error) {
	check := api.ServerCheck{Ok: true}
	step := func(name string, err error, detail string) bool {
		s := api.ServerCheckStep{Name: name, Ok: err == nil}
		if err != nil {
			detail = err.Error()
			check.Ok = false
		}
		if detail != "" {
			s.Detail = &detail
		}
		check.Steps = append(check.Steps, s)
		return err == nil
	}

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
	step("sftp", err, "")
	return check, nil
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
