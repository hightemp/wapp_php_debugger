package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// pushedPrefix names the log_files rows of pushed requests: one per request, as if it were a file.
const pushedPrefix = "pushed/"

// pushedKey is what a request_id may be to name such a row: a ULID or the like, never a path.
var pushedKey = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

// PushResult is the outcome of a pushed batch.
type PushResult struct {
	Created  []int64
	Updated  []int64
	Rejected int // lines that are not v2 events, or of a request_id that cannot be one
}

// SavePushed saves a batch of v2 events sent over HTTP; it may hold several requests. Each request
// goes on from where its previous batch ended; events already saved (a batch sent again) are skipped.
func (s *Store) SavePushed(ctx context.Context, projectID int64, data []byte) (PushResult, error) {
	var result PushResult
	var order []string
	groups := map[string][]byte{}
	for line := range bytes.Lines(data) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var head struct {
			V         int    `json:"v"`
			RequestID string `json:"request_id"`
			Seq       int    `json:"seq"`
		}
		if json.Unmarshal(line, &head) != nil || head.V != 2 || !pushedKey.MatchString(head.RequestID) {
			result.Rejected++
			continue
		}
		if _, seen := groups[head.RequestID]; !seen {
			order = append(order, head.RequestID)
		}
		if !bytes.HasSuffix(line, []byte("\n")) {
			line = append(slices.Clone(line), '\n')
		}
		groups[head.RequestID] = append(groups[head.RequestID], line...)
	}

	for _, key := range order {
		saved, err := s.savePushedRequest(ctx, projectID, key, groups[key])
		if err != nil {
			return result, err
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

func (s *Store) savePushedRequest(ctx context.Context, projectID int64, key string, data []byte) (SaveResult, error) {
	name := pushedPrefix + key + ".jsonl"
	known, _, err := s.FileState(ctx, projectID, name)
	if err != nil {
		return SaveResult{}, err
	}
	// What was saved already comes again after a retry: only newer events go on
	var fresh []byte
	for line := range bytes.Lines(data) {
		var head struct {
			Seq int `json:"seq"`
		}
		if json.Unmarshal(line, &head) == nil && head.Seq > known.LastSeq {
			fresh = append(fresh, line...)
		}
	}
	if len(fresh) == 0 {
		return SaveResult{}, nil
	}
	chunk, err := logformat.ParseV2Chunk(fresh, known.LastSeq)
	if err != nil {
		return SaveResult{}, err
	}
	file := FileState{Name: name, Size: known.ReadOffset + int64(len(fresh)), ModTime: time.Now()}
	return s.SaveV2Chunk(ctx, projectID, file, known.ReadOffset, chunk)
}

// SetIngestToken keeps the hash of a project's token for pushed logs; nil turns pushing off.
func (s *Store) SetIngestToken(ctx context.Context, projectID int64, hash *string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE projects SET ingest_token_hash = ?, updated_at = ? WHERE id = ?`,
		hash, time.Now().UnixMilli(), projectID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ProjectByIngestToken finds the project a pushed batch belongs to by the hash of its token.
func (s *Store) ProjectByIngestToken(ctx context.Context, hash string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE ingest_token_hash = ?`, hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}
