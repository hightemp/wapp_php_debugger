// Package app собирает просмотрщик: база, импорт логов, поток событий и HTTP-сервер.
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
)

// Config — параметры запуска.
type Config struct {
	Addr         string // адрес HTTP; по умолчанию слушаем только 127.0.0.1
	DBPath       string
	ProjectName  string
	RootPath     string
	LogsDir      string // пусто — без локального проекта
	ConfigPath   string // dws_debugger.json библиотеки
	ScanInterval time.Duration
	UI           fs.FS
	Logger       *slog.Logger
}

// Run работает до отмены ctx; ready вызывается с фактическим адресом, когда сервер принимает соединения.
func Run(ctx context.Context, cfg Config, ready func(addr string)) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = time.Second
	}

	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	s := store.New(db)
	broker := events.NewBroker(1024)

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           api.NewHandler(api.Deps{Store: s, Broker: broker, UI: cfg.UI}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		if cfg.LogsDir != "" {
			scanLoop(ctx, cfg, s, broker)
		}
	}()

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()
	ready(listener.Addr().String())
	cfg.Logger.Info("wapp-debugger started", "addr", "http://"+listener.Addr().String(), "logs_dir", cfg.LogsDir)

	select {
	case err = <-serveErr:
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		// Потоки SSE держат соединения: Shutdown дожидается их закрытия по отмене контекста запросов
		httpServer.RegisterOnShutdown(cancel)
		err = httpServer.Shutdown(shutdownCtx)
	}
	cancel()
	<-scanDone
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
	ticker := time.NewTicker(cfg.ScanInterval)
	defer ticker.Stop()
	for {
		result, err := importer.Scan(ctx, ids.ProjectID, src)
		if err != nil && ctx.Err() == nil {
			cfg.Logger.Warn("scan logs", "error", err)
		}
		for name, fileErr := range result.Failed {
			cfg.Logger.Warn("skip log file", "file", name, "error", fileErr)
		}
		publish(ctx, s, broker, "request_created", result.Created)
		publish(ctx, s, broker, "request_updated", result.Updated)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
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
