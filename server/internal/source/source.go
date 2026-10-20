// Package source describes where project logs live: a local directory, a server over SSH
// or the PHP library HTTP mode. The shared sourcetest.Run suite checks the contract.
package source

import (
	"context"
	"encoding/json"
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
	// ErrOutsideRoot means a source file resolves (through .. or symlinks) outside the project root.
	ErrOutsideRoot = errors.New("the file is outside the project root")
	// ErrTooLarge means a source file is over the size the preview reads.
	ErrTooLarge = errors.New("the file is too large to preview")
)

// ProjectFiles reads source files of the project (not its logs) for the code preview. The path is
// a server path; it must resolve, symlinks included, to a regular file inside root.
type ProjectFiles interface {
	ReadProjectFile(ctx context.Context, root, path string, limit int64) ([]byte, error)
}

// within reports whether path is root or inside it; both are clean and absolute.
func within(root, path string, separator string) bool {
	return path == root || strings.HasPrefix(path, strings.TrimSuffix(root, separator)+separator)
}

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

// enabledKey is the flag key in the config: enabled (library 2.x), or bIsOnFlag of a v1 config.
// With both keys enabled wins, as in library 2.x, which reads the v1 key only when the new one is missing.
func enabledKey(config map[string]json.RawMessage) string {
	if _, ok := config["enabled"]; ok {
		return "enabled"
	}
	if _, ok := config["bIsOnFlag"]; ok {
		return "bIsOnFlag"
	}
	return "enabled"
}

// enabledIn reads the debugger flag from the content of dws_debugger.json.
func enabledIn(data []byte) (bool, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return false, err
	}
	var enabled bool
	if raw, ok := config[enabledKey(config)]; ok {
		_ = json.Unmarshal(raw, &enabled)
	}
	return enabled, nil
}

// withEnabled returns dws_debugger.json with the flag set and every other key kept.
func withEnabled(data []byte, enabled bool) ([]byte, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	value, _ := json.Marshal(enabled)
	config[enabledKey(config)] = value
	out, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
