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
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"github.com/hightemp/wapp_php_debugger/server/internal/watch"
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
	db       interface{ Close() error }
	handler  http.Handler
	broker   *events.Broker
	cancel   context.CancelFunc
	scanDone chan struct{}
}

// Start opens the database and starts the background import; Close stops them.
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
	broker := events.NewBroker(1024)

	ctx, cancel := context.WithCancel(ctx)
	core := &Core{
		db:       db,
		handler:  api.NewHandler(api.Deps{Store: s, Broker: broker, UI: cfg.UI}),
		broker:   broker,
		cancel:   cancel,
		scanDone: make(chan struct{}),
	}
	go func() {
		defer close(core.scanDone)
		if cfg.LogsDir != "" {
			scanLoop(ctx, cfg, s, broker)
		}
	}()
	return core, nil
}

// Handler serves /api/v1/*, SSE and the UI.
func (c *Core) Handler() http.Handler { return c.handler }

// Broker is the event stream: the desktop publishes its own events into it (self-test).
func (c *Core) Broker() *events.Broker { return c.broker }

// Close stops the import, cancels SSE streams and closes the database.
func (c *Core) Close() {
	c.cancel()
	<-c.scanDone
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

func scanLoop(ctx context.Context, cfg Config, s *store.Store, broker *events.Broker) {
	ids, err := s.EnsureLocalProject(ctx, cfg.ProjectName, cfg.RootPath, cfg.LogsDir)
	if err != nil {
		cfg.Logger.Error("register local project", "error", err)
		return
	}
	src := source.NewLocal(cfg.LogsDir, cfg.ConfigPath)
	importer := ingest.New(s)
	// Scan right after file changes (debounced so a growing file is not re-read per line)
	// and every ScanInterval as a fallback for file systems without notifications
	for range watch.Dir(ctx, cfg.LogsDir, watch.Options{PollInterval: cfg.ScanInterval, Debounce: 100 * time.Millisecond}) {
		result, err := importer.Scan(ctx, ids.ProjectID, src)
		if err != nil && ctx.Err() == nil {
			cfg.Logger.Warn("scan logs", "error", err)
		}
		for name, fileErr := range result.Failed {
			cfg.Logger.Warn("skip log file", "file", name, "error", fileErr)
		}
		publish(ctx, s, broker, "request_created", result.Created)
		publish(ctx, s, broker, "request_updated", result.Updated)
	}
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
