package source

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Memory is an in-memory source for importer and API tests.
type Memory struct {
	mu       sync.Mutex
	files    map[string][]byte
	modTimes map[string]time.Time
	enabled  bool
}

var _ Source = (*Memory)(nil)

// NewMemory creates a source with a copy of files; the debugger is off.
func NewMemory(files map[string][]byte) *Memory {
	m := &Memory{files: make(map[string][]byte, len(files)), modTimes: make(map[string]time.Time, len(files))}
	created := time.Now()
	for name, data := range files {
		m.files[name] = append([]byte(nil), data...)
		m.modTimes[name] = created
	}
	return m
}

// Append appends data to a file (creating it if needed), the way the PHP library does.
func (m *Memory) Append(name string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[name] = append(m.files[name], data...)
	m.modTimes[name] = time.Now()
}

// List returns files sorted by name.
func (m *Memory) List(ctx context.Context) ([]FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]FileInfo, 0, len(m.files))
	for name, data := range m.files {
		list = append(list, FileInfo{Name: name, Size: int64(len(data)), ModTime: m.modTimes[name]})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// ReadAt returns a copy of up to limit bytes from offset.
func (m *Memory) ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[name]
	if !ok {
		return nil, ErrNotFound
	}
	if offset > int64(len(data)) {
		return nil, ErrOffsetBeyondEnd
	}
	end := min(offset+int64(limit), int64(len(data)))
	return append([]byte(nil), data[offset:end]...), nil
}

// Remove deletes a file.
func (m *Memory) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateName(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[name]; !ok {
		return ErrNotFound
	}
	delete(m.files, name)
	delete(m.modTimes, name)
	return nil
}

// Enabled reports whether the debugger is on.
func (m *Memory) Enabled(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled, nil
}

// SetEnabled turns the debugger on or off.
func (m *Memory) SetEnabled(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
	return nil
}

// Close does nothing.
func (m *Memory) Close() error { return nil }
