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

// SourceFactory makes the log source of a project. remote means there are no file system events
// to watch (SSH, HTTP): the directory is polled instead.
type SourceFactory func(project store.Project) (src source.Source, remote bool, err error)

// LocalSources is the factory for projects of the local server.
func LocalSources(project store.Project) (source.Source, bool, error) {
	return source.NewLocal(project.LogsDir, project.ConfigPath), false, nil
}

// ManagerOptions tune the manager.
type ManagerOptions struct {
	PollInterval       time.Duration // fallback rescan period of local projects
	RemotePollInterval time.Duration // rescan period of remote projects (plan 10.18: SFTP stat every 1–2 s); 2 s
	Sources            SourceFactory // LocalSources by default
	Logger             *slog.Logger
	// OnChange is called after a scan that created or updated requests of a project.
	OnChange func(projectID int64, created, updated []int64)
}

type running struct {
	project store.Project
	source  source.Source
	remote  bool
	cancel  context.CancelFunc
	done    chan struct{}
}

// Manager runs one import loop per project and restarts it when the project changes.
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
	if options.RemotePollInterval == 0 {
		options.RemotePollInterval = 2 * time.Second
	}
	if options.Sources == nil {
		options.Sources = LocalSources
	}
	return &Manager{store: s, importer: New(s), options: options, running: map[int64]*running{}}
}

// Start launches a loop for every project. Loops stop when ctx is canceled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()

	projects, err := m.store.ListProjects(ctx, 0)
	if err != nil {
		return err
	}
	for _, project := range projects {
		m.Add(project)
	}
	return nil
}

// Add starts the loop of a project, or restarts it with the new settings. A project whose source
// cannot be made is skipped and logged.
func (m *Manager) Add(project store.Project) {
	m.Remove(project.ID)

	src, remote, err := m.options.Sources(project)
	if err != nil {
		m.options.Logger.Warn("no log source", "project", project.Name, "error", err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil || m.ctx.Err() != nil {
		_ = src.Close()
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	r := &running{
		project: project,
		source:  src,
		remote:  remote,
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	m.running[project.ID] = r
	go func() {
		defer close(r.done)
		defer func() { _ = r.source.Close() }()
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

// loop scans a local project right after file changes (debounced so a growing file is not re-read
// per line) and every PollInterval as a fallback; a remote project every RemotePollInterval.
func (m *Manager) loop(ctx context.Context, r *running) {
	var signals <-chan struct{}
	if r.remote {
		signals = watch.Every(ctx, m.options.RemotePollInterval)
	} else {
		signals = watch.Dir(ctx, r.project.LogsDir, watch.Options{PollInterval: m.options.PollInterval, Debounce: 100 * time.Millisecond})
	}
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
