// Package app assembles the viewer: database, log import, event stream and HTTP server.
package app

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/ingest"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// Config holds launch settings.
type Config struct {
	Addr         string // HTTP address; only 127.0.0.1 is listened on by default
	DBPath       string
	ProjectName  string
	RootPath     string
	LogsDir      string // empty means no local project
	ConfigPath   string // library dws_debugger.json
	ScanInterval time.Duration
	UI           fs.FS
	Logger       *slog.Logger
}

// Core is the running viewer core: database, background import and HTTP handler.
// The server (Run) puts the handler on a network port, the desktop into the Wails asset handler.
type Core struct {
	db      interface{ Close() error }
	handler http.Handler
	broker  *events.Broker
	manager *ingest.Manager
	cancel  context.CancelFunc
}

// Start opens the database and starts the background import of every project; Close stops it.
// With LogsDir set, that directory is registered as a project of the local server first.
func Start(ctx context.Context, cfg Config) (*Core, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = 5 * time.Second
	}

	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	s := store.New(db)
	if cfg.LogsDir != "" {
		if err := registerProject(ctx, s, cfg); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	broker := events.NewBroker(1024)
	manager := ingest.NewManager(s, ingest.ManagerOptions{
		PollInterval: cfg.ScanInterval,
		Logger:       cfg.Logger,
		OnChange: func(_ int64, created, updated []int64) {
			publish(ctx, s, broker, "request_created", created)
			publish(ctx, s, broker, "request_updated", updated)
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	if err := manager.Start(ctx); err != nil {
		cancel()
		_ = db.Close()
		return nil, err
	}
	return &Core{
		db:      db,
		handler: api.NewHandler(api.Deps{Store: s, Broker: broker, Projects: manager, UI: cfg.UI}),
		broker:  broker,
		manager: manager,
		cancel:  cancel,
	}, nil
}

// registerProject makes sure the --logs-dir project exists on the local server with these paths.
func registerProject(ctx context.Context, s *store.Store, cfg Config) error {
	ids, err := s.EnsureLocalProject(ctx, cfg.ProjectName, cfg.RootPath, cfg.LogsDir)
	if err != nil {
		return err
	}
	if cfg.ConfigPath == "" {
		return nil
	}
	_, err = s.UpdateProject(ctx, ids.ProjectID, store.ProjectPatch{ConfigPath: &cfg.ConfigPath})
	return err
}

// Handler serves /api/v1/*, SSE and the UI.
func (c *Core) Handler() http.Handler { return c.handler }

// Broker is the event stream: the desktop publishes its own events into it (self-test).
func (c *Core) Broker() *events.Broker { return c.broker }

// Close stops the import, cancels SSE streams and closes the database.
func (c *Core) Close() {
	c.cancel()
	c.manager.Wait()
	_ = c.db.Close()
}

// Run serves until ctx is canceled; ready is called with the actual address once the server accepts connections.
func Run(ctx context.Context, cfg Config, ready func(addr string)) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	core, err := Start(ctx, cfg)
	if err != nil {
		_ = listener.Close()
		return err
	}
	defer core.Close()

	httpServer := &http.Server{Handler: core.Handler(), ReadHeaderTimeout: 10 * time.Second}
	// SSE streams hold connections open: canceling the core closes them on shutdown
	httpServer.RegisterOnShutdown(core.cancel)

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()
	ready(listener.Addr().String())
	cfg.Logger.Info("wapp-debugger started", "addr", "http://"+listener.Addr().String(), "logs_dir", cfg.LogsDir)

	select {
	case err = <-serveErr:
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		err = httpServer.Shutdown(shutdownCtx)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func publish(ctx context.Context, s *store.Store, broker *events.Broker, name string, ids []int64) {
	for _, id := range ids {
		row, found, err := s.GetRequest(ctx, id)
		if err != nil || !found {
			continue
		}
		dto := api.RequestDTO(row)
		broker.Publish(name, api.StreamEvent{Event: api.StreamEventEvent(name), Request: &dto})
	}
}
