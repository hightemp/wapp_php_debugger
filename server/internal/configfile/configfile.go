// Package configfile exports the servers and projects of the viewer to YAML and imports them back:
// to move to another machine, share a setup with a colleague or edit it by hand. Secrets never go
// into the file: tokens of http servers are set again after an import.
package configfile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"go.yaml.in/yaml/v3"
)

// Version is the version of the file format.
const Version = 1

// File is the whole file.
type File struct {
	Version int      `yaml:"version"`
	Servers []Server `yaml:"servers"`
}

// Server is a server with its projects. The local server stands for this machine wherever the
// file is imported.
type Server struct {
	Name        string       `yaml:"name"`
	Kind        string       `yaml:"kind"`
	Environment string       `yaml:"environment,omitempty"`
	Config      *Config      `yaml:"config,omitempty"`
	Permissions *Permissions `yaml:"permissions,omitempty"`
	Projects    []Project    `yaml:"projects,omitempty"`
}

// Config holds the connection settings: the same keys as in the API, never a secret.
type Config struct {
	Host      string   `yaml:"host,omitempty" json:"host,omitempty"`
	Port      int      `yaml:"port,omitempty" json:"port,omitempty"`
	User      string   `yaml:"user,omitempty" json:"user,omitempty"`
	JumpHosts []string `yaml:"jump_hosts,omitempty" json:"jump_hosts,omitempty"`
	BaseURL   string   `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

// Permissions of a server.
type Permissions struct {
	ReadOnly     bool `yaml:"read_only"`
	AllowDelete  bool `yaml:"allow_delete"`
	AllowControl bool `yaml:"allow_control"`
}

// Project is a project of a server.
type Project struct {
	Name       string     `yaml:"name"`
	RootPath   string     `yaml:"root_path"`
	LogsDir    string     `yaml:"logs_dir"`
	LocalPath  string     `yaml:"local_path,omitempty"`
	ConfigPath string     `yaml:"config_path,omitempty"`
	Editor     string     `yaml:"editor,omitempty"`
	Retention  *Retention `yaml:"retention,omitempty"`
}

// Retention of a project in the viewer's database.
type Retention struct {
	Days     *int   `yaml:"days,omitempty"`
	MaxBytes *int64 `yaml:"max_bytes,omitempty"`
}

// Result says what an import did.
type Result struct {
	ServersCreated []string
	Projects       []store.Project // created; their import is to be started
	Skipped        []string        // "server" or "server/project" that exist already
	NeedsToken     []string        // created http servers: their token is not in the file
}

// Export writes every server and project.
func Export(ctx context.Context, s *store.Store) ([]byte, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	file := File{Version: Version, Servers: []Server{}}
	for _, srv := range servers {
		out := Server{Name: srv.Name, Kind: srv.Kind}
		if srv.Kind != "local" {
			out.Environment = srv.Environment
			var config Config
			if err := json.Unmarshal([]byte(srv.Config), &config); err != nil {
				return nil, fmt.Errorf("server %s: %w", srv.Name, err)
			}
			out.Config = &config
			out.Permissions = &Permissions{ReadOnly: srv.ReadOnly, AllowDelete: srv.AllowDelete, AllowControl: srv.AllowControl}
		}
		projects, err := s.ListProjects(ctx, srv.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			project := Project{Name: p.Name, RootPath: p.RootPath, LogsDir: p.LogsDir, LocalPath: p.LocalPath, Editor: p.Editor}
			// The default config path is not written: it follows the root
			if p.ConfigPath != path.Join(p.RootPath, "dws_debugger.json") {
				project.ConfigPath = p.ConfigPath
			}
			if p.RetentionDays != nil || p.RetentionMaxBytes != nil {
				project.Retention = &Retention{Days: p.RetentionDays, MaxBytes: p.RetentionMaxBytes}
			}
			out.Projects = append(out.Projects, project)
		}
		file.Servers = append(file.Servers, out)
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(file); err != nil {
		return nil, err
	}
	return buffer.Bytes(), encoder.Close()
}

// Import adds the servers and projects of the file that the viewer does not have yet: a server is
// matched by name (the local one by kind), a project by name on its server. A file with any
// mistake is rejected whole, before anything is written.
func Import(ctx context.Context, s *store.Store, data []byte) (Result, error) {
	var file File
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // a misspelt key, or a secret, is not silently dropped
	if err := decoder.Decode(&file); err != nil {
		return Result{}, fmt.Errorf("not a viewer config: %w", err)
	}
	if err := validate(file); err != nil {
		return Result{}, err
	}

	var result Result
	for _, in := range file.Servers {
		srv, existed, err := findServer(ctx, s, in)
		if err != nil {
			return result, err
		}
		if !existed {
			srv, err = createServer(ctx, s, in)
			if err != nil {
				return result, fmt.Errorf("server %s: %w", in.Name, err)
			}
			result.ServersCreated = append(result.ServersCreated, in.Name)
			if in.Kind == "http" {
				result.NeedsToken = append(result.NeedsToken, in.Name)
			}
		} else if in.Kind != "local" {
			result.Skipped = append(result.Skipped, in.Name)
		}
		existing, err := s.ListProjects(ctx, srv.ID)
		if err != nil {
			return result, err
		}
		for _, p := range in.Projects {
			if has(existing, p.Name) || (srv.Kind == "http" && len(existing) > 0) {
				result.Skipped = append(result.Skipped, srv.Name+"/"+p.Name)
				continue
			}
			input := store.ProjectInput{ServerID: srv.ID, Name: p.Name, RootPath: p.RootPath, LogsDir: p.LogsDir,
				LocalPath: p.LocalPath, ConfigPath: p.ConfigPath, Editor: p.Editor}
			if p.Retention != nil {
				input.RetentionDays, input.RetentionMaxBytes = p.Retention.Days, p.Retention.MaxBytes
			}
			created, err := s.CreateProject(ctx, input)
			if err != nil {
				return result, fmt.Errorf("project %s/%s: %w", srv.Name, p.Name, err)
			}
			existing = append(existing, created)
			result.Projects = append(result.Projects, created)
		}
	}
	return result, nil
}

var (
	kinds        = map[string]bool{"local": true, "ssh": true, "http": true}
	environments = map[string]bool{"": true, "dev": true, "stage": true, "prod": true}
	editors      = map[string]bool{"": true, "vscode": true, "phpstorm": true, "cursor": true, "sublime": true, "none": true}
)

func validate(file File) error {
	if file.Version != Version {
		return fmt.Errorf("unknown config version %d, this viewer reads version %d", file.Version, Version)
	}
	names := map[string]bool{}
	for _, srv := range file.Servers {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("server %q: %s", srv.Name, fmt.Sprintf(format, args...))
		}
		switch {
		case srv.Name == "":
			return errors.New("a server without a name")
		case !kinds[srv.Kind]:
			return fail("unknown kind %q", srv.Kind)
		case srv.Kind != "local" && !environments[srv.Environment]:
			return fail("unknown environment %q", srv.Environment)
		case srv.Kind != "local" && names[srv.Name]:
			return fail("listed twice")
		}
		names[srv.Name] = true
		switch srv.Kind {
		case "ssh":
			if srv.Config == nil || srv.Config.Host == "" {
				return fail("an ssh server needs config.host")
			}
		case "http":
			if srv.Config == nil || !absoluteURL(srv.Config.BaseURL) {
				return fail("an http server needs config.base_url, an https:// address")
			}
			if len(srv.Projects) > 1 {
				return fail("an http server is one application and holds one project")
			}
		}
		projects := map[string]bool{}
		for _, p := range srv.Projects {
			switch {
			case p.Name == "" || projects[p.Name]:
				return fail("a project without a name, or listed twice: %q", p.Name)
			case !absolute(p.RootPath) || !absolute(p.LogsDir):
				return fail("project %q: root_path and logs_dir must be absolute paths", p.Name)
			case !editors[p.Editor]:
				return fail("project %q: unknown editor %q", p.Name, p.Editor)
			}
			projects[p.Name] = true
		}
	}
	return nil
}

func absolute(name string) bool { return path.IsAbs(name) || filepath.IsAbs(name) }

func absoluteURL(raw string) bool {
	base, err := url.Parse(raw)
	return err == nil && (base.Scheme == "https" || base.Scheme == "http") && base.Host != ""
}

func findServer(ctx context.Context, s *store.Store, in Server) (store.Server, bool, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return store.Server{}, false, err
	}
	for _, srv := range servers {
		if (in.Kind == "local" && srv.Kind == "local") || (in.Kind != "local" && srv.Name == in.Name) {
			return srv, true, nil
		}
	}
	if in.Kind == "local" {
		srv, err := s.EnsureLocalServer(ctx)
		if err != nil {
			return store.Server{}, false, err
		}
		local, _, err := s.GetServer(ctx, srv)
		return local, true, err
	}
	return store.Server{}, false, nil
}

func createServer(ctx context.Context, s *store.Store, in Server) (store.Server, error) {
	config, err := json.Marshal(in.Config)
	if err != nil {
		return store.Server{}, err
	}
	input := store.ServerInput{Name: in.Name, Kind: in.Kind, Environment: in.Environment, Config: string(config)}
	if in.Permissions != nil {
		input.Permissions = &store.Permissions{ReadOnly: in.Permissions.ReadOnly, AllowDelete: in.Permissions.AllowDelete, AllowControl: in.Permissions.AllowControl}
	}
	return s.CreateServer(ctx, input)
}

func has(projects []store.Project, name string) bool {
	for _, p := range projects {
		if p.Name == name {
			return true
		}
	}
	return false
}
