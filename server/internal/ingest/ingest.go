// Package ingest imports log files from a source into the viewer database.
package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// Importer re-reads log files of a project that changed.
type Importer struct {
	store *store.Store
}

// New creates an importer.
func New(s *store.Store) *Importer {
	return &Importer{store: s}
}

// ScanResult is the outcome of one pass over the logs directory.
type ScanResult struct {
	Created     []int64          // ids of new requests
	Updated     []int64          // ids of requests whose file changed
	Unchanged   int              // files unchanged since the last pass
	Unsupported int              // v2 files: the parser arrives in phase 5
	Failed      map[string]error // files that could not be parsed
}

// Scan walks the source files and saves new and changed ones.
// A parse error in one file does not stop the pass: it goes to Failed.
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

		// A v1 file is re-read as a whole: its header is rewritten in place
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
