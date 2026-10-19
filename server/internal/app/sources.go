package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/ingest"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// SSHConfig says where the SSH settings of the user live.
type SSHConfig struct {
	ConfigFile       string // ~/.ssh/config; a missing file means no aliases
	Home             string // for ~ in the ssh config
	User             string // the default login name
	TrustedHostsFile string // host keys confirmed in the viewer; the user's known_hosts is never written
	AgentSocket      string // SSH_AUTH_SOCK
}

// DefaultSSH is the SSH setup of the current user.
func DefaultSSH() SSHConfig {
	home, _ := os.UserHomeDir()
	name := os.Getenv("USER")
	if current, err := user.Current(); err == nil && current.Username != "" {
		name = current.Username
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = home
	}
	return SSHConfig{
		ConfigFile:       filepath.Join(home, ".ssh", "config"),
		Home:             home,
		User:             name,
		TrustedHostsFile: filepath.Join(configDir, "wapp-debugger", "known_hosts"),
		AgentSocket:      os.Getenv("SSH_AUTH_SOCK"),
	}
}

// resolver reads the ssh config again every time, so edits apply without a restart.
func (c SSHConfig) resolver() (*sshconn.Resolver, error) {
	var config io.Reader
	file, err := os.Open(c.ConfigFile)
	switch {
	case err == nil:
		defer func() { _ = file.Close() }()
		config = file
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	return sshconn.NewResolver(config, c.Home, c.User)
}

// sourceFactory gives each project the source of its server: a local directory (watched) or SFTP (polled).
func sourceFactory(ctx context.Context, s *store.Store, pool *sshconn.Pool, sshConfig SSHConfig) ingest.SourceFactory {
	return func(project store.Project) (source.Source, bool, error) {
		server, found, err := s.GetServer(ctx, project.ServerID)
		if err != nil {
			return nil, false, err
		}
		if !found {
			return nil, false, fmt.Errorf("server %d not found", project.ServerID)
		}
		switch server.Kind {
		case "local":
			return source.NewLocal(project.LogsDir, project.ConfigPath), false, nil
		case "ssh":
			settings, err := sshconn.ParseServerSettings(server.Config)
			if err != nil {
				return nil, false, err
			}
			resolver, err := sshConfig.resolver()
			if err != nil {
				return nil, false, err
			}
			target, err := resolver.ResolveServer(settings)
			if err != nil {
				return nil, false, err
			}
			return source.NewSSH(pool, server.ID, target, project.LogsDir, project.ConfigPath), true, nil
		default:
			return nil, false, fmt.Errorf("servers of kind %s are not supported yet", server.Kind)
		}
	}
}

func serverStatusEvent(serverID int64, status sshconn.Status) api.StreamEvent {
	connection := api.ServerConnection{State: api.ConnectionState(status.State)}
	if status.Error != "" {
		connection.Error = &status.Error
	}
	checkedAt := status.Since.UnixMilli()
	if status.Since.IsZero() {
		checkedAt = time.Now().UnixMilli()
	}
	connection.CheckedAt = &checkedAt
	return api.StreamEvent{Event: api.ServerStatus, ServerId: &serverID, Connection: &connection}
}
