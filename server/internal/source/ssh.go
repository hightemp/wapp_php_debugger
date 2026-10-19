package source

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/pkg/sftp"
)

// SSH is the logs directory of a project on a remote server, read over SFTP (plan 7.4).
// The connection belongs to the pool and is shared by all projects of the server.
type SSH struct {
	pool       *sshconn.Pool
	serverID   int64
	target     sshconn.Target
	logsDir    string // remote absolute paths
	configPath string
}

var _ Source = (*SSH)(nil)

// NewSSH creates a source for the project with logsDir and configPath on the server.
func NewSSH(pool *sshconn.Pool, serverID int64, target sshconn.Target, logsDir, configPath string) *SSH {
	return &SSH{pool: pool, serverID: serverID, target: target, logsDir: logsDir, configPath: configPath}
}

func (s *SSH) files(ctx context.Context) (*sftp.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := s.pool.Get(ctx, s.serverID, s.target)
	if err != nil {
		return nil, err
	}
	return conn.SFTP, nil
}

// List returns regular files of the logs directory; directories and symlinks are skipped.
func (s *SSH) List(ctx context.Context) ([]FileInfo, error) {
	files, err := s.files(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := files.ReadDir(s.logsDir)
	if err != nil {
		return nil, err
	}
	list := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Mode().IsRegular() {
			list = append(list, FileInfo{Name: entry.Name(), Size: entry.Size(), ModTime: entry.ModTime()})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// file validates the name and returns the path and the size of a regular file in the logs directory.
func (s *SSH) file(files *sftp.Client, name string) (string, int64, error) {
	if err := ValidateName(name); err != nil {
		return "", 0, err
	}
	filePath := path.Join(s.logsDir, name)
	info, err := files.Lstat(filePath)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return "", 0, ErrNotFound
	}
	if err != nil {
		return "", 0, err
	}
	return filePath, info.Size(), nil
}

// ReadAt reads up to limit bytes from offset.
func (s *SSH) ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error) {
	files, err := s.files(ctx)
	if err != nil {
		return nil, err
	}
	filePath, size, err := s.file(files, name)
	if err != nil {
		return nil, err
	}
	if offset > size {
		return nil, ErrOffsetBeyondEnd
	}
	file, err := files.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, min(int64(limit), size-offset))
	n, err := file.ReadAt(buffer, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buffer[:n], nil
}

// Remove deletes a log file.
func (s *SSH) Remove(ctx context.Context, name string) error {
	files, err := s.files(ctx)
	if err != nil {
		return err
	}
	filePath, _, err := s.file(files, name)
	if err != nil {
		return err
	}
	return files.Remove(filePath)
}

func (s *SSH) readConfig(files *sftp.Client) ([]byte, error) {
	file, err := files.Open(s.configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoConfig
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

// Enabled reads the flag from the config; no config means the debugger is off.
func (s *SSH) Enabled(ctx context.Context) (bool, error) {
	files, err := s.files(ctx)
	if err != nil {
		return false, err
	}
	data, err := s.readConfig(files)
	if errors.Is(err, ErrNoConfig) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabledIn(data)
}

// SetEnabled changes only the flag, keeping other keys and the file mode. The new content goes to a
// temporary file next to the config, then replaces it with posix-rename: atomic, unlike plain SFTP rename.
func (s *SSH) SetEnabled(ctx context.Context, enabled bool) error {
	files, err := s.files(ctx)
	if err != nil {
		return err
	}
	data, err := s.readConfig(files)
	if err != nil {
		return err
	}
	if data, err = withEnabled(data, enabled); err != nil {
		return err
	}
	info, err := files.Stat(s.configPath)
	if err != nil {
		return err
	}
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	temp := path.Join(path.Dir(s.configPath), ".dws_debugger-"+hex.EncodeToString(suffix)+".json")
	// pkg/sftp maps the local os flags to SFTP ones
	file, err := files.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	defer func() { _ = files.Remove(temp) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := files.Chmod(temp, info.Mode().Perm()); err != nil {
		return err
	}
	return files.PosixRename(temp, s.configPath)
}

// Close does nothing: the pool owns the connection.
func (s *SSH) Close() error { return nil }
