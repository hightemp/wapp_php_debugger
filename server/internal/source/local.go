package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Local is a logs directory on this machine (plan 7.3).
type Local struct {
	logsDir    string
	configPath string
}

var _ Source = (*Local)(nil)

// NewLocal creates a source: logsDir holds the log files, configPath is the library dws_debugger.json.
func NewLocal(logsDir, configPath string) *Local {
	return &Local{logsDir: logsDir, configPath: configPath}
}

// List returns regular files of the logs directory; directories and symlinks are skipped.
func (l *Local) List(ctx context.Context) ([]FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(l.logsDir)
	if err != nil {
		return nil, err
	}
	list := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue // the file was removed between ReadDir and Info
		}
		list = append(list, FileInfo{Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// path validates the name and returns the path of a regular file inside the logs directory.
func (l *Local) path(name string) (string, os.FileInfo, error) {
	if err := ValidateName(name); err != nil {
		return "", nil, err
	}
	path := filepath.Join(l.logsDir, name)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return "", nil, ErrNotFound
	}
	return path, info, err
}

// ReadAt reads up to limit bytes from offset.
func (l *Local) ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, info, err := l.path(name)
	if err != nil {
		return nil, err
	}
	if offset > info.Size() {
		return nil, ErrOffsetBeyondEnd
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	buffer := make([]byte, min(int64(limit), info.Size()-offset))
	n, err := file.ReadAt(buffer, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buffer[:n], nil
}

// Remove deletes a log file.
func (l *Local) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, _, err := l.path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// Enabled reads the flag from the config; no config means the debugger is off.
func (l *Local) Enabled(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	data, err := os.ReadFile(l.configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabledIn(data)
}

// SetEnabled changes only the flag, keeping other keys and the file mode; the write is atomic.
func (l *Local) SetEnabled(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(l.configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoConfig
	}
	if err != nil {
		return err
	}
	if data, err = withEnabled(data, enabled); err != nil {
		return err
	}
	info, err := os.Stat(l.configPath)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(l.configPath), ".dws_debugger-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(temp.Name(), l.configPath)
}

// Close does nothing.
func (l *Local) Close() error { return nil }
