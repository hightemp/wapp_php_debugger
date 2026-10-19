// Package source описывает место, где лежат логи проекта: локальную папку, сервер по SSH
// или HTTP-режим PHP-библиотеки. Контракт проверяется общим набором тестов sourcetest.Run.
package source

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrNotFound — файла лога нет.
	ErrNotFound = errors.New("log file not found")
	// ErrInvalidName — имя указывает за пределы папки логов (пустое, с разделителями пути, "..").
	ErrInvalidName = errors.New("invalid log file name")
	// ErrOffsetBeyondEnd — смещение больше размера файла: файл усекли или подменили.
	ErrOffsetBeyondEnd = errors.New("offset beyond end of file")
)

// FileInfo — файл лога в папке логов проекта.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// Source — папка логов одного проекта. Имена файлов — без путей, только внутри папки логов.
type Source interface {
	// List возвращает файлы логов, отсортированные по имени.
	List(ctx context.Context) ([]FileInfo, error)
	// ReadAt возвращает до limit байт файла начиная с offset; пустой результат — конец файла.
	ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error)
	// Remove удаляет файл лога.
	Remove(ctx context.Context, name string) error
	// Enabled сообщает, включён ли отладчик в проекте.
	Enabled(ctx context.Context) (bool, error)
	// SetEnabled включает или выключает отладчик в проекте.
	SetEnabled(ctx context.Context, enabled bool) error
	// Close освобождает подключение.
	Close() error
}

// ValidateName проверяет, что имя указывает на файл внутри папки логов.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return ErrInvalidName
	}
	return nil
}
