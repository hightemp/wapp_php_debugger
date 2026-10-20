package app

import (
	"context"
	"fmt"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/discover"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// discoverer is api.Discoverer: this machine's file system, or SFTP over the shared SSH connection.
type discoverer struct{ ssh *sshRuntime }

var _ api.Discoverer = discoverer{}

func (d discoverer) Discover(ctx context.Context, server store.Server, roots []string, maxDepth int) ([]discover.Found, bool, error) {
	options := discover.Options{MaxDepth: maxDepth}
	switch server.Kind {
	case "local":
		return discover.Find(ctx, discover.Local{}, roots, options)
	case "ssh":
		target, err := d.ssh.target(server)
		if err != nil {
			return nil, false, err
		}
		conn, err := d.ssh.pool.Get(ctx, server.ID, target)
		if err != nil {
			return nil, false, err
		}
		return discover.Find(ctx, discover.SFTP{Client: conn.SFTP}, roots, options)
	default:
		return nil, false, fmt.Errorf("servers of kind %s cannot be searched", server.Kind)
	}
}
