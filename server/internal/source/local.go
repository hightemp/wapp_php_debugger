package source

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Local — папка логов на этой машине (п. 7.3 плана).
type Local struct {
	logsDir    string
	configPath string
}

var _ Source = (*Local)(nil)

// NewLocal создаёт источник: logsDir — папка с файлами логов, configPath — dws_debugger.json библиотеки.
func NewLocal(logsDir, configPath string) *Local {
	return &Local{logsDir: logsDir, configPath: configPath}
}

// List возвращает обычные файлы папки логов; каталоги и символические ссылки пропускаются.
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
			continue // файл удалили между ReadDir и Info
		}
		list = append(list, FileInfo{Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

// path проверяет имя и возвращает путь к обычному файлу внутри папки логов.
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

// ReadAt читает до limit байт с offset.
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

// Remove удаляет файл лога.
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

// enabledKey — ключ флага в конфиге: bIsOnFlag у библиотеки v1, enabled у v2.
func enabledKey(config map[string]json.RawMessage) string {
	if _, ok := config["bIsOnFlag"]; ok {
		return "bIsOnFlag"
	}
	return "enabled"
}

func (l *Local) readConfig() (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(l.configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoConfig
	}
	if err != nil {
		return nil, err
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// Enabled читает флаг из конфига; нет конфига — отладчик выключен.
func (l *Local) Enabled(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	config, err := l.readConfig()
	if errors.Is(err, ErrNoConfig) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var enabled bool
	if raw, ok := config[enabledKey(config)]; ok {
		_ = json.Unmarshal(raw, &enabled)
	}
	return enabled, nil
}

// SetEnabled меняет только флаг, остальные ключи и права файла сохраняются; запись атомарная.
func (l *Local) SetEnabled(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	config, err := l.readConfig()
	if err != nil {
		return err
	}
	value, _ := json.Marshal(enabled)
	config[enabledKey(config)] = value

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
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
	if _, err := temp.Write(append(data, '\n')); err != nil {
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

// Close ничего не делает.
func (l *Local) Close() error { return nil }
