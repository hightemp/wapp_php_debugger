// Package discover finds projects on a server by the library config (dws_debugger.json) under the
// given roots: on this machine or over SFTP. The walk is bounded and never follows symlinks.
package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/sftp"
)

// ConfigName is the library config file a project is recognised by.
const ConfigName = "dws_debugger.json"

// configLimit bounds the config read: a real one is a few hundred bytes.
const configLimit = 64 << 10

// skipped are directories with dependencies or caches: they hold no projects of their own.
var skipped = map[string]bool{".git": true, ".svn": true, ".hg": true, "node_modules": true, "vendor": true, ".cache": true}

// Entry is an item of a directory; a symlink is neither a directory nor a regular file.
type Entry struct {
	Name    string
	Dir     bool
	Regular bool
}

// FS is a file system of a server: local or SFTP.
type FS interface {
	ReadDir(ctx context.Context, dir string) ([]Entry, error)
	ReadFile(ctx context.Context, name string, limit int64) ([]byte, error)
	Join(elem ...string) string
	Dir(name string) string
	Base(name string) string
	IsAbs(name string) bool
}

// Options bound the walk; zero values take the defaults.
type Options struct {
	MaxDepth int // directories below a root; 4
	MaxDirs  int // directories read in all; 5000
}

// Found is a project found by its config, with paths resolved the way the library resolves them.
type Found struct {
	Name       string
	Root       string
	LogsDir    string
	ConfigPath string
	Enabled    bool
	Problem    string // why the config could not be read; the paths are the library defaults then
}

// Find walks the roots and returns the projects under them. truncated means MaxDirs stopped the walk.
func Find(ctx context.Context, fsys FS, roots []string, options Options) (found []Found, truncated bool, err error) {
	if options.MaxDepth == 0 {
		options.MaxDepth = 4
	}
	if options.MaxDirs == 0 {
		options.MaxDirs = 5000
	}
	budget := options.MaxDirs
	seen := map[string]bool{}

	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if budget == 0 {
			truncated = true
			return nil
		}
		budget--
		entries, err := fsys.ReadDir(ctx, dir)
		if err != nil {
			if depth == 0 {
				return err
			}
			return nil // an unreadable directory deep down: skip it
		}
		for _, entry := range entries {
			if entry.Regular && entry.Name == ConfigName {
				project := resolve(ctx, fsys, fsys.Join(dir, entry.Name))
				if !seen[project.Root] {
					seen[project.Root] = true
					found = append(found, project)
				}
			}
		}
		if depth == options.MaxDepth {
			return nil
		}
		for _, entry := range entries {
			if entry.Dir && !skipped[entry.Name] {
				if err := walk(fsys.Join(dir, entry.Name), depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, root := range roots {
		if !fsys.IsAbs(root) {
			return nil, false, fmt.Errorf("root %q is not an absolute path", root)
		}
		if err := walk(root, 0); err != nil {
			return found, truncated, fmt.Errorf("%s: %w", root, err)
		}
	}
	return found, truncated, nil
}

// resolve reads a config the way the library does: project_root against the config directory,
// logs_dir against the project root, and the keys of library v1.
func resolve(ctx context.Context, fsys FS, configPath string) Found {
	dir := fsys.Dir(configPath)
	project := Found{ConfigPath: configPath, Root: dir}
	var config map[string]any
	data, err := fsys.ReadFile(ctx, configPath, configLimit)
	if err == nil {
		err = json.Unmarshal(data, &config)
	}
	if err != nil {
		project.Problem = err.Error()
	}
	text := func(keys ...string) string {
		for _, key := range keys {
			if value, ok := config[key].(string); ok && value != "" {
				return value
			}
		}
		return ""
	}
	against := func(base, name string) string {
		if fsys.IsAbs(name) {
			return fsys.Join(name)
		}
		return fsys.Join(base, name)
	}
	if root := text("project_root", "sProjectRootPath"); root != "" {
		project.Root = against(dir, root)
	}
	project.LogsDir = fsys.Join(project.Root, "tmp", "logs")
	if logs := text("logs_dir", "sMessagesFilesPath"); logs != "" {
		project.LogsDir = against(project.Root, logs)
	}
	enabled, ok := config["enabled"].(bool)
	if !ok {
		enabled, _ = config["bIsOnFlag"].(bool)
	}
	project.Enabled = enabled
	project.Name = fsys.Base(project.Root)
	return project
}

// Local is the file system of this machine.
type Local struct{}

// ReadDir lists a directory; symlinks are reported as neither directories nor files.
// The directory comes from the user of the viewer on purpose: it is a root they asked to search on
// their own machine (behind loopback or the token); only dws_debugger.json files are read.
func (Local) ReadDir(_ context.Context, dir string) ([]Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		entries = append(entries, Entry{Name: item.Name(), Dir: item.Type().IsDir(), Regular: item.Type().IsRegular()})
	}
	return entries, nil
}

// ReadFile reads up to limit bytes; a longer file is an error.
func (Local) ReadFile(_ context.Context, name string, limit int64) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, errors.New("config is too large")
	}
	return os.ReadFile(name)
}

// Join joins path elements.
func (Local) Join(elem ...string) string { return filepath.Join(elem...) }

// Dir is the directory of a path.
func (Local) Dir(name string) string { return filepath.Dir(name) }

// Base is the last element of a path.
func (Local) Base(name string) string { return filepath.Base(name) }

// IsAbs reports an absolute path.
func (Local) IsAbs(name string) bool { return filepath.IsAbs(name) }

// SFTP is the file system of a server over SFTP; paths are POSIX.
type SFTP struct{ Client *sftp.Client }

// ReadDir lists a directory; SFTP reports symlinks as they are (lstat), so they are skipped.
func (s SFTP) ReadDir(ctx context.Context, dir string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := s.Client.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		entries = append(entries, Entry{Name: item.Name(), Dir: item.Mode().IsDir(), Regular: item.Mode().IsRegular()})
	}
	return entries, nil
}

// ReadFile reads up to limit bytes; a longer file is an error.
func (s SFTP) ReadFile(ctx context.Context, name string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := s.Client.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, errors.New("config is too large")
	}
	file, err := s.Client.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(io.LimitReader(file, limit))
}

// Join joins path elements.
func (SFTP) Join(elem ...string) string { return path.Join(elem...) }

// Dir is the directory of a path.
func (SFTP) Dir(name string) string { return path.Dir(name) }

// Base is the last element of a path.
func (SFTP) Base(name string) string { return path.Base(name) }

// IsAbs reports an absolute path.
func (SFTP) IsAbs(name string) bool { return path.IsAbs(name) }
