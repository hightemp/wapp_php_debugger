package source

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Memory — источник в памяти для тестов импорта и API.
type Memory struct {
	mu      sync.Mutex
	files   map[string][]byte
	enabled bool
	now     func() time.Time
}

var _ Source = (*Memory)(nil)

// NewMemory создаёт источник с копией files; отладчик выключен.
func NewMemory(files map[string][]byte) *Memory {
	m := &Memory{files: make(map[string][]byte, len(files)), now: time.Now}
	for name, data := range files {
		m.files[name] = append([]byte(nil), data...)
	}
	return m
}

// Append дописывает данные в файл (создаёт его при необходимости), как это делает PHP-библиотека.
func (m *Memory) Append(name string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[name] = append(m.files[name], data...)
}

// List возвращает файлы, отсортированные по имени.
func (m *Memory) List(ctx context.Context) ([]FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]FileInfo, 0, len(m.files))
	for name, data := range m.files {
		list = append(list, FileInfo{Name: name, Size: int64(len(data)), ModTime: m.now()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// ReadAt возвращает копию до limit байт с offset.
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

// Remove удаляет файл.
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
	return nil
}

// Enabled сообщает, включён ли отладчик.
func (m *Memory) Enabled(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled, nil
}

// SetEnabled включает или выключает отладчик.
func (m *Memory) SetEnabled(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
	return nil
}

// Close ничего не делает.
func (m *Memory) Close() error { return nil }
