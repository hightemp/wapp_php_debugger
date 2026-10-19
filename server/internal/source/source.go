// Package source describes where project logs live: a local directory, a server over SSH
// or the PHP library HTTP mode. The shared sourcetest.Run suite checks the contract.
package source

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNotFound means the log file does not exist.
	ErrNotFound = errors.New("log file not found")
	// ErrInvalidName means the name points outside the logs directory (empty, with path separators, "..").
	ErrInvalidName = errors.New("invalid log file name")
	// ErrOffsetBeyondEnd means the offset is past the end of the file: it was truncated or replaced.
	ErrOffsetBeyondEnd = errors.New("offset beyond end of file")
	// ErrNoConfig means the project has no debugger config, so there is nothing to switch.
	ErrNoConfig = errors.New("debugger config not found")
)

// FileInfo is a log file in the project logs directory.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// Source is the logs directory of one project. File names have no paths and stay inside the logs directory.
type Source interface {
	// List returns log files sorted by name.
	List(ctx context.Context) ([]FileInfo, error)
	// ReadAt returns up to limit bytes of the file starting at offset; an empty result means end of file.
	ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error)
	// Remove deletes a log file.
	Remove(ctx context.Context, name string) error
	// Enabled reports whether the debugger is on in the project.
	Enabled(ctx context.Context) (bool, error)
	// SetEnabled turns the debugger on or off in the project.
	SetEnabled(ctx context.Context, enabled bool) error
	// Close releases the connection.
	Close() error
}

// ValidateName checks that the name points to a file inside the logs directory.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return ErrInvalidName
	}
	return nil
}
