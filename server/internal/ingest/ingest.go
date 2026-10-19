// Package ingest imports log files from a source into the viewer database.
package ingest

import (
	"context"
	"fmt"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// Importer re-reads log files of a project that changed.
type Importer struct {
	store     *store.Store
	chunkSize int // how much of a v2 file is read at once; grows for lines longer than this
}

// New creates an importer.
func New(s *store.Store) *Importer {
	return &Importer{store: s, chunkSize: 4 << 20}
}

// ScanResult is the outcome of one pass over the logs directory.
type ScanResult struct {
	Created   []int64          // ids of new requests
	Updated   []int64          // ids of requests whose file changed
	Unchanged int              // files unchanged since the last pass
	Failed    map[string]error // files that could not be parsed
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
		known, found, err := i.store.FileState(ctx, projectID, file.Name)
		if err != nil {
			return result, err
		}
		if found && known.Size == file.Size && known.ModTime.Equal(file.ModTime.Truncate(1e6)) {
			result.Unchanged++
			continue
		}

		format := known.FormatVersion
		if !found {
			head, err := src.ReadAt(ctx, file.Name, 0, min(int(file.Size), 64<<10))
			if err != nil {
				result.Failed[file.Name] = err
				continue
			}
			if format = logformat.DetectFormat(head); format == 0 {
				continue // the first line is not complete yet
			}
		}

		var saved store.SaveResult
		if format == 2 {
			saved, err = i.importV2(ctx, projectID, src, file, known, found)
		} else {
			saved, err = i.importV1(ctx, projectID, src, file)
		}
		if err != nil {
			result.Failed[file.Name] = err
			continue
		}
		switch {
		case saved.Created:
			result.Created = append(result.Created, saved.RequestID)
		case saved.RequestID != 0:
			result.Updated = append(result.Updated, saved.RequestID)
		}
	}
	return result, nil
}

// importV1 re-reads a v1 file as a whole: its header is rewritten in place.
func (i *Importer) importV1(ctx context.Context, projectID int64, src source.Source, file source.FileInfo) (store.SaveResult, error) {
	data, err := src.ReadAt(ctx, file.Name, 0, int(file.Size))
	if err != nil {
		return store.SaveResult{}, err
	}
	parsed, err := logformat.ParseV1(file.Name, data)
	if err != nil {
		return store.SaveResult{}, err
	}
	saved, err := i.store.SaveFile(ctx, projectID, store.FileState{Name: file.Name, Size: int64(len(data)), ModTime: file.ModTime}, parsed)
	if err != nil {
		return saved, fmt.Errorf("%s: %w", file.Name, err)
	}
	return saved, nil
}

// importV2 continues an append-only v2 file from the saved offset, chunk by chunk.
func (i *Importer) importV2(ctx context.Context, projectID int64, src source.Source, file source.FileInfo, known store.FileState, found bool) (store.SaveResult, error) {
	offset, lastSeq := known.ReadOffset, known.LastSeq
	if found && file.Size < offset {
		// Shorter than what was read: the file was truncated or replaced, start over
		if err := i.store.ForgetFile(ctx, projectID, file.Name); err != nil {
			return store.SaveResult{}, err
		}
		offset, lastSeq = 0, 0
	}

	state := store.FileState{Name: file.Name, Size: file.Size, ModTime: file.ModTime}
	var result store.SaveResult
	limit := i.chunkSize
	for offset < file.Size {
		data, err := src.ReadAt(ctx, file.Name, offset, limit)
		if err != nil {
			return result, err
		}
		chunk, err := logformat.ParseV2Chunk(data, lastSeq)
		if err != nil {
			return result, err
		}
		if chunk.Consumed == 0 {
			if offset+int64(len(data)) >= file.Size {
				break // the last line is still being written
			}
			limit *= 2 // a line longer than the chunk
			continue
		}
		saved, err := i.store.SaveV2Chunk(ctx, projectID, state, offset, chunk)
		if err != nil {
			return result, fmt.Errorf("%s: %w", file.Name, err)
		}
		if saved.RequestID != 0 {
			result.RequestID = saved.RequestID
			result.Created = result.Created || saved.Created
		}
		offset += chunk.Consumed
		lastSeq = chunk.LastSeq
		limit = i.chunkSize
	}
	return result, nil
}
