package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNotFound means the referenced row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a unique constraint was violated (a name or a logs directory is taken).
	ErrConflict = errors.New("conflict")
)

// classify turns SQLite constraint errors into ErrConflict and ErrNotFound.
func classify(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "UNIQUE constraint failed"):
		return errors.Join(ErrConflict, err)
	case strings.Contains(err.Error(), "FOREIGN KEY constraint failed"):
		return errors.Join(ErrNotFound, err)
	default:
		return err
	}
}

// Server is a machine that hosts projects.
type Server struct {
	ID           int64
	Name         string
	Kind         string
	Environment  string
	Config       string // JSON object without secrets
	ReadOnly     bool
	AllowDelete  bool
	AllowControl bool
	CreatedAt    int64
	UpdatedAt    int64
}

const selectServers = `SELECT id, name, kind, environment, config, read_only, allow_delete, allow_control, created_at, updated_at FROM servers`

func scanServer(row rowScanner) (Server, error) {
	var s Server
	err := row.Scan(&s.ID, &s.Name, &s.Kind, &s.Environment, &s.Config, &s.ReadOnly, &s.AllowDelete, &s.AllowControl, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// EnsureLocalServer creates (or finds) the "local" server and returns its id.
func (s *Store) EnsureLocalServer(ctx context.Context) (int64, error) {
	now := time.Now().UnixMilli()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO servers (name, kind, environment, created_at, updated_at) VALUES ('local', 'local', 'local', ?, ?)
		ON CONFLICT (name) DO UPDATE SET name = excluded.name
		RETURNING id`, now, now).Scan(&id)
	return id, err
}

// ListServers returns servers by name.
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, selectServers+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var servers []Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// GetServer returns a server by id.
func (s *Store) GetServer(ctx context.Context, id int64) (Server, bool, error) {
	server, err := scanServer(s.db.QueryRowContext(ctx, selectServers+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, false, nil
	}
	return server, err == nil, err
}

// Project is an application on a server.
type Project struct {
	ID                int64
	ServerID          int64
	Name              string
	RootPath          string
	LogsDir           string
	LocalPath         string
	ConfigPath        string // library dws_debugger.json; defaults to <root_path>/dws_debugger.json
	Editor            string
	RetentionDays     *int
	RetentionMaxBytes *int64
	IngestEnabled     bool // has a token for logs pushed over HTTP
	CreatedAt         int64
	UpdatedAt         int64
}

const selectProjects = `
	SELECT id, server_id, name, root_path, logs_dir, COALESCE(local_path, ''),
		COALESCE(config_path, rtrim(root_path, '/') || '/dws_debugger.json'), editor, retention_days, retention_max_bytes,
		ingest_token_hash IS NOT NULL, created_at, updated_at
	FROM projects`

func scanProject(row rowScanner) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.ServerID, &p.Name, &p.RootPath, &p.LogsDir, &p.LocalPath, &p.ConfigPath, &p.Editor, &p.RetentionDays,
		&p.RetentionMaxBytes, &p.IngestEnabled, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// ListProjects returns projects by name; serverID = 0 means all servers.
func (s *Store) ListProjects(ctx context.Context, serverID int64) ([]Project, error) {
	query, args := selectProjects+` ORDER BY name, id`, []any{}
	if serverID != 0 {
		query, args = selectProjects+` WHERE server_id = ? ORDER BY name, id`, []any{serverID}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var projects []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// GetProject returns a project by id.
func (s *Store) GetProject(ctx context.Context, id int64) (Project, bool, error) {
	project, err := scanProject(s.db.QueryRowContext(ctx, selectProjects+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, false, nil
	}
	return project, err == nil, err
}

// ProjectInput holds the fields of a new project.
type ProjectInput struct {
	ServerID          int64
	Name              string
	RootPath          string
	LogsDir           string
	LocalPath         string
	ConfigPath        string
	Editor            string
	RetentionDays     *int
	RetentionMaxBytes *int64
}

// CreateProject adds a project; ErrConflict means the name or the logs directory is taken on that server.
func (s *Store) CreateProject(ctx context.Context, input ProjectInput) (Project, error) {
	if input.Editor == "" {
		input.Editor = "vscode"
	}
	now := time.Now().UnixMilli()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO projects (server_id, name, root_path, logs_dir, local_path, config_path, editor, retention_days, retention_max_bytes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		input.ServerID, input.Name, input.RootPath, input.LogsDir, nullString(input.LocalPath), nullString(input.ConfigPath), input.Editor,
		input.RetentionDays, input.RetentionMaxBytes, now, now).Scan(&id)
	if err != nil {
		return Project{}, classify(err)
	}
	project, _, err := s.GetProject(ctx, id)
	return project, err
}

// ProjectPatch holds the fields to change; nil fields stay as they are. An empty LocalPath clears it.
type ProjectPatch struct {
	Name              *string
	RootPath          *string
	LogsDir           *string
	LocalPath         *string
	ConfigPath        *string
	Editor            *string
	RetentionDays     *int
	RetentionMaxBytes *int64
	// SetRetention replaces both limits with the two fields above, nil meaning no limit;
	// without it nil keeps a limit as it is
	SetRetention bool
}

// UpdateProject changes a project.
func (s *Store) UpdateProject(ctx context.Context, id int64, patch ProjectPatch) (Project, error) {
	project, found, err := s.GetProject(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if !found {
		return Project{}, ErrNotFound
	}
	if patch.Name != nil {
		project.Name = *patch.Name
	}
	if patch.RootPath != nil {
		project.RootPath = *patch.RootPath
	}
	if patch.LogsDir != nil {
		project.LogsDir = *patch.LogsDir
	}
	if patch.LocalPath != nil {
		project.LocalPath = *patch.LocalPath
	}
	var configPath any
	if patch.ConfigPath != nil {
		configPath = nullString(*patch.ConfigPath)
	} else if err := s.db.QueryRowContext(ctx, `SELECT config_path FROM projects WHERE id = ?`, id).Scan(&configPath); err != nil {
		return Project{}, err
	}
	if patch.Editor != nil {
		project.Editor = *patch.Editor
	}
	if patch.SetRetention || patch.RetentionDays != nil {
		project.RetentionDays = patch.RetentionDays
	}
	if patch.SetRetention || patch.RetentionMaxBytes != nil {
		project.RetentionMaxBytes = patch.RetentionMaxBytes
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE projects SET name = ?, root_path = ?, logs_dir = ?, local_path = ?, config_path = ?, editor = ?, retention_days = ?,
			retention_max_bytes = ?, updated_at = ? WHERE id = ?`,
		project.Name, project.RootPath, project.LogsDir, nullString(project.LocalPath), configPath, project.Editor, project.RetentionDays,
		project.RetentionMaxBytes, time.Now().UnixMilli(), id)
	if err != nil {
		return Project{}, classify(err)
	}
	updated, _, err := s.GetProject(ctx, id)
	return updated, err
}

// DeleteProject removes a project and everything imported for it (log files on disk are not touched).
func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Permissions limit what the viewer may do on a server.
type Permissions struct {
	ReadOnly     bool
	AllowDelete  bool
	AllowControl bool
}

// ServerInput holds the fields of a new server; nil Permissions means everything is allowed.
type ServerInput struct {
	Name        string
	Kind        string // ssh or http; the local server is made by EnsureLocalServer
	Environment string // dev by default
	Config      string // JSON object without secrets; {} by default
	Permissions *Permissions
}

// CreateServer adds a server; ErrConflict means the name is taken.
func (s *Store) CreateServer(ctx context.Context, input ServerInput) (Server, error) {
	if input.Environment == "" {
		input.Environment = "dev"
	}
	if input.Config == "" {
		input.Config = "{}"
	}
	permissions := Permissions{AllowDelete: true, AllowControl: true}
	if input.Permissions != nil {
		permissions = *input.Permissions
	}
	now := time.Now().UnixMilli()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO servers (name, kind, environment, config, read_only, allow_delete, allow_control, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		input.Name, input.Kind, input.Environment, input.Config, permissions.ReadOnly, permissions.AllowDelete, permissions.AllowControl,
		now, now).Scan(&id)
	if err != nil {
		return Server{}, classify(err)
	}
	server, _, err := s.GetServer(ctx, id)
	return server, err
}

// DeleteServer removes a server with its projects and their data (logs on the server stay).
// The local server cannot be deleted.
func (s *Store) DeleteServer(ctx context.Context, id int64) error {
	server, found, err := s.GetServer(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if server.Kind == "local" {
		return errors.Join(ErrConflict, errors.New("the local server cannot be deleted"))
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

// ServerPatch holds server fields to change; nil fields stay as they are.
type ServerPatch struct {
	Name         *string
	Environment  *string
	Config       *string
	ReadOnly     *bool
	AllowDelete  *bool
	AllowControl *bool
}

// UpdateServer changes a server.
func (s *Store) UpdateServer(ctx context.Context, id int64, patch ServerPatch) (Server, error) {
	server, found, err := s.GetServer(ctx, id)
	if err != nil {
		return Server{}, err
	}
	if !found {
		return Server{}, ErrNotFound
	}
	if patch.Name != nil {
		server.Name = *patch.Name
	}
	if patch.Environment != nil {
		server.Environment = *patch.Environment
	}
	if patch.Config != nil {
		server.Config = *patch.Config
	}
	if patch.ReadOnly != nil {
		server.ReadOnly = *patch.ReadOnly
	}
	if patch.AllowDelete != nil {
		server.AllowDelete = *patch.AllowDelete
	}
	if patch.AllowControl != nil {
		server.AllowControl = *patch.AllowControl
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE servers SET name = ?, environment = ?, config = ?, read_only = ?, allow_delete = ?, allow_control = ?, updated_at = ?
		WHERE id = ?`, server.Name, server.Environment, server.Config, server.ReadOnly, server.AllowDelete, server.AllowControl,
		time.Now().UnixMilli(), id)
	if err != nil {
		return Server{}, classify(err)
	}
	updated, _, err := s.GetServer(ctx, id)
	return updated, err
}

// PendingFiles counts the project log files that are known but not read to the end.
func (s *Store) PendingFiles(ctx context.Context, projectID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM log_files WHERE project_id = ? AND read_offset < size`, projectID).Scan(&n)
	return n, err
}
