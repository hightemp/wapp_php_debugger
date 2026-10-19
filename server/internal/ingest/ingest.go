// Package ingest импортирует файлы логов из источника в базу просмотрщика.
package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// Importer перечитывает изменившиеся файлы логов проекта.
type Importer struct {
	store *store.Store
}

// New создаёт импортёр.
func New(s *store.Store) *Importer {
	return &Importer{store: s}
}

// ScanResult — итог одного прохода по папке логов.
type ScanResult struct {
	Created     []int64          // id новых запросов
	Updated     []int64          // id запросов, файл которых изменился
	Unchanged   int              // файлы без изменений с прошлого прохода
	Unsupported int              // файлы v2: парсер появится в фазе 5
	Failed      map[string]error // файлы, которые не удалось разобрать
}

// Scan проходит по файлам источника и сохраняет новые и изменившиеся.
// Ошибка разбора одного файла не останавливает проход: она попадает в Failed.
func (i *Importer) Scan(ctx context.Context, projectID int64, src source.Source) (ScanResult, error) {
	result := ScanResult{Failed: map[string]error{}}
	files, err := src.List(ctx)
	if err != nil {
		return result, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name, ".jsonl") {
			result.Unsupported++
			continue
		}
		known, found, err := i.store.FileState(ctx, projectID, file.Name)
		if err != nil {
			return result, err
		}
		if found && known.Size == file.Size && known.ModTime.Equal(file.ModTime.Truncate(1e6)) {
			result.Unchanged++
			continue
		}

		// Файл v1 перечитывается целиком: заголовок перезаписывается на месте
		data, err := src.ReadAt(ctx, file.Name, 0, int(file.Size))
		if err != nil {
			result.Failed[file.Name] = err
			continue
		}
		parsed, err := logformat.ParseV1(file.Name, data)
		if err != nil {
			result.Failed[file.Name] = err
			continue
		}
		saved, err := i.store.SaveFile(ctx, projectID, store.FileState{Name: file.Name, Size: int64(len(data)), ModTime: file.ModTime}, parsed)
		if err != nil {
			return result, fmt.Errorf("%s: %w", file.Name, err)
		}
		if saved.Created {
			result.Created = append(result.Created, saved.RequestID)
		} else {
			result.Updated = append(result.Updated, saved.RequestID)
		}
	}
	return result, nil
}
