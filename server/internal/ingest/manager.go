package ingest

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"github.com/hightemp/wapp_php_debugger/server/internal/watch"
)

// ManagerOptions tune the manager.
type ManagerOptions struct {
	PollInterval time.Duration // fallback rescan period of every project
	Logger       *slog.Logger
	// OnChange is called after a scan that created or updated requests of a project.
	OnChange func(projectID int64, created, updated []int64)
}

type running struct {
	project store.Project
	source  source.Source
	cancel  context.CancelFunc
	done    chan struct{}
}

// Manager runs one import loop per project of the local server and restarts it when the project changes.
type Manager struct {
	store    *store.Store
	importer *Importer
	options  ManagerOptions

	mu      sync.Mutex
	ctx     context.Context
	running map[int64]*running
}

// NewManager creates a manager; Start launches the loops.
func NewManager(s *store.Store, options ManagerOptions) *Manager {
	if options.PollInterval == 0 {
		options.PollInterval = 5 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.OnChange == nil {
		options.OnChange = func(int64, []int64, []int64) {}
	}
	return &Manager{store: s, importer: New(s), options: options, running: map[int64]*running{}}
}

// Start launches a loop for every project of local servers. Loops stop when ctx is canceled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()

	servers, err := m.store.ListServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if server.Kind != "local" {
			continue // SSH and HTTP sources arrive in phase 7
		}
		projects, err := m.store.ListProjects(ctx, server.ID)
		if err != nil {
			return err
		}
		for _, project := range projects {
			m.Add(project)
		}
	}
	return nil
}

// Add starts the loop of a project, or restarts it with the new settings.
func (m *Manager) Add(project store.Project) {
	m.Remove(project.ID)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	r := &running{
		project: project,
		source:  source.NewLocal(project.LogsDir, project.ConfigPath),
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	m.running[project.ID] = r
	go func() {
		defer close(r.done)
		m.loop(ctx, r)
	}()
}

// Remove stops the loop of a project and waits for it.
func (m *Manager) Remove(projectID int64) {
	m.mu.Lock()
	r, ok := m.running[projectID]
	delete(m.running, projectID)
	m.mu.Unlock()
	if ok {
		r.cancel()
		<-r.done
	}
}

// Source returns the log source of a running project (to switch the debugger or delete logs).
func (m *Manager) Source(projectID int64) (source.Source, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.running[projectID]
	if !ok {
		return nil, false
	}
	return r.source, true
}

// Wait waits for every loop after the Start context was canceled.
func (m *Manager) Wait() {
	m.mu.Lock()
	loops := make([]*running, 0, len(m.running))
	for _, r := range m.running {
		loops = append(loops, r)
	}
	m.mu.Unlock()
	for _, r := range loops {
		<-r.done
	}
}

// loop scans right after file changes (debounced so a growing file is not re-read per line)
// and every PollInterval as a fallback for file systems without notifications.
func (m *Manager) loop(ctx context.Context, r *running) {
	signals := watch.Dir(ctx, r.project.LogsDir, watch.Options{PollInterval: m.options.PollInterval, Debounce: 100 * time.Millisecond})
	for range signals {
		result, err := m.importer.Scan(ctx, r.project.ID, r.source)
		if err != nil && ctx.Err() == nil {
			m.options.Logger.Warn("scan logs", "project", r.project.Name, "error", err)
		}
		for name, fileErr := range result.Failed {
			m.options.Logger.Warn("skip log file", "project", r.project.Name, "file", name, "error", fileErr)
		}
		if len(result.Created)+len(result.Updated) > 0 {
			m.options.OnChange(r.project.ID, result.Created, result.Updated)
		}
	}
}
