// Package secrets keeps tokens of servers outside the database, which is never meant to hold them.
// Until the OS secret store (plan 10.19) it is a JSON file readable by the user only.
package secrets

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// File is a secret store in one JSON file with mode 0600.
type File struct {
	path string
	mu   sync.Mutex
}

// NewFile uses the file at path; it and its directory are made on the first Set.
func NewFile(path string) *File { return &File{path: path} }

func (f *File) read() (map[string]string, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	return values, json.Unmarshal(data, &values)
}

// write replaces the file at once: a temporary file with mode 0600, then rename.
func (f *File) write(values map[string]string) error {
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".secrets-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), f.path)
}

// Get returns the secret under key.
func (f *File) Get(key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return "", false, err
	}
	value, found := values[key]
	return value, found, nil
}

// Set stores the secret under key.
func (f *File) Set(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	values[key] = value
	return f.write(values)
}

// Delete removes the secret under key; a missing one is fine.
func (f *File) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, err := f.read()
	if err != nil {
		return err
	}
	if _, found := values[key]; !found {
		return nil
	}
	delete(values, key)
	return f.write(values)
}
