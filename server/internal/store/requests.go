package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// ErrInvalidCursor means the page cursor is corrupted or did not come from ListRequests.
var ErrInvalidCursor = errors.New("invalid cursor")

// Store runs queries against the viewer database.
type Store struct {
	db *sql.DB
}

// New wraps an open database (see Open).
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// LocalProject holds the ids of the local server and project.
type LocalProject struct {
	ServerID  int64
	ProjectID int64
}

// EnsureLocalProject creates (or finds) the "local" server and a project on it; an existing project
// with that name gets the given paths.
func (s *Store) EnsureLocalProject(ctx context.Context, name, rootPath, logsDir string) (LocalProject, error) {
	serverID, err := s.EnsureLocalServer(ctx)
	if err != nil {
		return LocalProject{}, err
	}
	now := time.Now().UnixMilli()
	ids := LocalProject{ServerID: serverID}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO projects (server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (server_id, name) DO UPDATE SET root_path = excluded.root_path, logs_dir = excluded.logs_dir, updated_at = excluded.updated_at
		RETURNING id`, serverID, name, rootPath, logsDir, now, now).Scan(&ids.ProjectID)
	return ids, err
}

// FileState is what is known about a log file: size and mtime tell whether it must be re-read,
// ReadOffset and LastSeq tell where a v2 file continues.
type FileState struct {
	Name          string
	Size          int64
	ModTime       time.Time
	ReadOffset    int64
	FormatVersion int
	LastSeq       int
	Status        string // active, complete or broken
	Error         string
}

// FileState returns the stored state of a project log file.
func (s *Store) FileState(ctx context.Context, projectID int64, name string) (FileState, bool, error) {
	state := FileState{Name: name}
	var mtime int64
	var fileError sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT size, mtime, read_offset, format_version, last_seq, status, error FROM log_files WHERE project_id = ? AND path = ?`,
		projectID, name).Scan(&state.Size, &mtime, &state.ReadOffset, &state.FormatVersion, &state.LastSeq, &state.Status, &fileError)
	if errors.Is(err, sql.ErrNoRows) {
		return state, false, nil
	}
	state.ModTime = time.UnixMilli(mtime)
	state.Error = fileError.String
	return state, err == nil, err
}

// ForgetFile removes a log file and everything imported from it (the file was truncated or replaced).
func (s *Store) ForgetFile(ctx context.Context, projectID int64, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM log_files WHERE project_id = ? AND path = ?`, projectID, name)
	return err
}

// SaveResult is the outcome of saving a file.
type SaveResult struct {
	RequestID int64
	Created   bool
}

// SaveFile saves a parsed log file: file state, the request and its messages.
// Saving the same request again replaces its messages (a v1 file is re-read as a whole).
func (s *Store) SaveFile(ctx context.Context, projectID int64, file FileState, parsed logformat.Parsed) (SaveResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SaveResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	status := "active"
	if parsed.Request.Finished {
		status = "complete"
	}
	var logFileID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO log_files (project_id, path, format_version, size, mtime, read_offset, status) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project_id, path) DO UPDATE SET format_version = excluded.format_version, size = excluded.size,
			mtime = excluded.mtime, read_offset = excluded.read_offset, status = excluded.status, error = NULL
		RETURNING id`,
		projectID, file.Name, parsed.Request.FormatVersion, file.Size, file.ModTime.UnixMilli(), file.Size, status).Scan(&logFileID)
	if err != nil {
		return SaveResult{}, fmt.Errorf("log file: %w", err)
	}

	result, err := upsertRequest(ctx, tx, projectID, logFileID, parsed)
	if err != nil {
		return SaveResult{}, fmt.Errorf("request: %w", err)
	}
	if !result.Created {
		if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE request_id = ?`, result.RequestID); err != nil {
			return SaveResult{}, err
		}
	}
	for _, e := range parsed.Events {
		if err := insertEvent(ctx, tx, result.RequestID, e); err != nil {
			return SaveResult{}, fmt.Errorf("message %d: %w", e.Seq, err)
		}
	}
	return result, tx.Commit()
}

// SaveV2Chunk appends a parsed v2 chunk read from offset: the request from request_start, new events,
// and the end data from request_end. Saving the same chunk again changes nothing.
func (s *Store) SaveV2Chunk(ctx context.Context, projectID int64, file FileState, offset int64, chunk logformat.V2Chunk) (SaveResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SaveResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var logFileID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO log_files (project_id, path, format_version, size, mtime, read_offset) VALUES (?, ?, 2, ?, ?, 0)
		ON CONFLICT (project_id, path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime
		RETURNING id`, projectID, file.Name, max(file.Size, offset+chunk.Consumed), file.ModTime.UnixMilli()).Scan(&logFileID)
	if err != nil {
		return SaveResult{}, fmt.Errorf("log file: %w", err)
	}

	var result SaveResult
	err = tx.QueryRowContext(ctx, `SELECT id FROM requests WHERE log_file_id = ?`, logFileID).Scan(&result.RequestID)
	switch {
	case errors.Is(err, sql.ErrNoRows) && chunk.Start != nil:
		created, err := upsertRequest(ctx, tx, projectID, logFileID, logformat.Parsed{Request: *chunk.Start})
		if err != nil {
			return SaveResult{}, fmt.Errorf("request: %w", err)
		}
		result = created
	case errors.Is(err, sql.ErrNoRows) && chunk.Consumed == 0:
		// Not even the first line is complete yet: just remember the size
		return SaveResult{}, tx.Commit()
	case errors.Is(err, sql.ErrNoRows):
		// Without request_start there is nothing to attach events to; skip the file until it changes
		problem := "the file does not begin with request_start"
		if len(chunk.Problems) > 0 {
			problem = strings.Join(chunk.Problems, "; ")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE log_files SET read_offset = ?, last_seq = ?, status = 'broken', error = ? WHERE id = ?`,
			offset+chunk.Consumed, chunk.LastSeq, problem, logFileID); err != nil {
			return SaveResult{}, err
		}
		return SaveResult{}, tx.Commit()
	case err != nil:
		return SaveResult{}, err
	}

	for _, e := range chunk.Events {
		if err := insertEvent(ctx, tx, result.RequestID, e); err != nil {
			return SaveResult{}, fmt.Errorf("message %d: %w", e.Seq, err)
		}
	}
	if end := chunk.End; end != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE requests SET finished = 1, ended_at = ?, duration_ms = ?, status_code = ?, exit_code = ?, memory_peak_bytes = ? WHERE id = ?`,
			end.EndedAt, end.DurationMs, end.StatusCode, end.ExitCode, end.MemoryPeakBytes, result.RequestID); err != nil {
			return SaveResult{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE requests SET
			message_count = (SELECT count(*) FROM messages WHERE request_id = requests.id),
			error_count = (SELECT count(*) FROM messages WHERE request_id = requests.id AND level = 'error'),
			warning_count = (SELECT count(*) FROM messages WHERE request_id = requests.id AND level = 'warning')
		WHERE id = ?`, result.RequestID); err != nil {
		return SaveResult{}, err
	}

	status, problems := "active", sql.NullString{}
	if chunk.End != nil {
		status = "complete"
	}
	if len(chunk.Problems) > 0 {
		problems = sql.NullString{String: strings.Join(chunk.Problems, "; "), Valid: true}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE log_files SET read_offset = ?, last_seq = ?, status = CASE WHEN status = 'complete' THEN status ELSE ? END,
			error = COALESCE(?, error) WHERE id = ?`,
		offset+chunk.Consumed, chunk.LastSeq, status, problems, logFileID); err != nil {
		return SaveResult{}, err
	}
	return result, tx.Commit()
}

func upsertRequest(ctx context.Context, tx *sql.Tx, projectID, logFileID int64, parsed logformat.Parsed) (SaveResult, error) {
	r := parsed.Request
	var messages, errorsCount, warnings int
	for _, e := range parsed.Events {
		messages++
		switch e.Level {
		case "error":
			errorsCount++
		case "warning":
			warnings++
		}
	}
	var argv any
	if r.Argv != nil {
		encoded, err := json.Marshal(r.Argv)
		if err != nil {
			return SaveResult{}, err
		}
		argv = string(encoded)
	}
	values := []any{
		logFileID, nullString(r.ParentKey), r.Sapi, r.Lang, nullString(r.Method), nullString(r.URL), r.IsAjax, argv,
		nullString(r.IP), nullString(r.ServerIP), nullString(r.ServerName), nullString(r.Hostname), r.PID,
		nullString(r.ProcessUser), nullString(r.AppUser), nullString(r.SessionID), nullString(r.LibraryVersion), nullString(r.PHPVersion),
		r.StartedAt, r.EndedAt, r.DurationMs, r.StatusCode, r.ExitCode, r.MemoryPeakBytes, r.Finished,
		messages, errorsCount, warnings,
	}

	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM requests WHERE project_id = ? AND request_key = ?`, projectID, r.Key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO requests (project_id, request_key, log_file_id, parent_request_key, sapi, lang, method, url, is_ajax, argv,
				ip, server_ip, server_name, hostname, pid, process_user, app_user, session_id, library_version, php_version,
				started_at, ended_at, duration_ms, status_code, exit_code, memory_peak_bytes, finished,
				message_count, error_count, warning_count, received_at)
			VALUES (?, ?, `+placeholders(len(values))+`, ?) RETURNING id`,
			append(append([]any{projectID, r.Key}, values...), time.Now().UnixMilli())...).Scan(&id)
		return SaveResult{RequestID: id, Created: true}, err
	}
	if err != nil {
		return SaveResult{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE requests SET log_file_id = ?, parent_request_key = ?, sapi = ?, lang = ?, method = ?, url = ?, is_ajax = ?, argv = ?,
			ip = ?, server_ip = ?, server_name = ?, hostname = ?, pid = ?, process_user = ?, app_user = ?, session_id = ?,
			library_version = ?, php_version = ?, started_at = ?, ended_at = ?, duration_ms = ?, status_code = ?, exit_code = ?,
			memory_peak_bytes = ?, finished = ?, message_count = ?, error_count = ?, warning_count = ?
		WHERE id = ?`, append(values, id)...)
	return SaveResult{RequestID: id}, err
}

func insertEvent(ctx context.Context, tx *sql.Tx, requestID int64, e logformat.Event) error {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	encodedTags, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	var trace any
	if e.Trace != nil {
		encoded, err := json.Marshal(e.Trace)
		if err != nil {
			return err
		}
		trace = string(encoded)
	}
	var source logformat.Frame
	if e.Source != nil {
		source = *e.Source
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO messages (request_id, seq, ts, type, level, message, tags, context, source_file, source_line, source_function,
			source_class, trace, duration_ms, error_kind, error_type, exception_class, code, previous, size_bytes)
		VALUES (`+placeholders(20)+`)
		ON CONFLICT (request_id, seq) DO NOTHING`,
		requestID, e.Seq, e.TS, e.Type, e.Level, e.Message, string(encodedTags), nullJSON(e.Context),
		nullString(source.File), nullInt(source.Line), nullString(source.Function), nullString(source.Class), trace,
		e.DurationMs, nullString(e.ErrorKind), nullString(e.ErrorType), nullString(e.ExceptionClass), nullString(e.Code),
		nullJSON(e.Previous), e.Size)
	return err
}

// RequestRow is a request in the feed.
type RequestRow struct {
	ID            int64
	ProjectID     int64
	ServerID      int64
	Key           string
	ParentKey     string
	FormatVersion int
	Sapi          string
	Method        string
	URL           string
	IsAjax        bool
	StatusCode    *int
	ExitCode      *int
	StartedAt     int64
	DurationMs    *float64
	Finished      bool
	MessageCount  int
	ErrorCount    int
	WarningCount  int
	ReceivedAt    int64
}

// RequestFilter holds feed parameters; zero values do not filter.
type RequestFilter struct {
	ProjectID   int64
	ServerID    int64
	Environment string
	Sapi        string
	HasErrors   bool
	Query       string // words searched in messages and context, or a URL substring
	Since       int64  // started_at at or after, ms
	Until       int64  // started_at before, ms
	Method      string // HTTP method, any case
	StatusMin   int    // status code range, inclusive; 0 means no bound
	StatusMax   int
	// MinDurationMs keeps requests at least this long
	MinDurationMs float64
	Ajax          *bool
	Tag           string // a message of the request has this tag
	Level         string // error: requests with errors; warning: with errors or warnings
	Limit         int
	Cursor        string
}

// RequestPage is a feed page; NextCursor is empty on the last page.
type RequestPage struct {
	Items      []RequestRow
	NextCursor string
}

// ListRequests returns requests newest first; equal times are ordered by id.
func (s *Store) ListRequests(ctx context.Context, filter RequestFilter) (RequestPage, error) {
	where, args := []string{"1 = 1"}, []any{}
	if filter.ProjectID != 0 {
		where, args = append(where, "r.project_id = ?"), append(args, filter.ProjectID)
	}
	if filter.ServerID != 0 {
		where, args = append(where, "p.server_id = ?"), append(args, filter.ServerID)
	}
	if filter.Environment != "" {
		where, args = append(where, "p.server_id IN (SELECT id FROM servers WHERE environment = ?)"), append(args, filter.Environment)
	}
	if filter.Sapi != "" {
		where, args = append(where, "r.sapi = ?"), append(args, filter.Sapi)
	}
	if filter.HasErrors {
		where = append(where, "r.error_count > 0")
	}
	if filter.Since != 0 {
		where, args = append(where, "r.started_at >= ?"), append(args, filter.Since)
	}
	if filter.Until != 0 {
		where, args = append(where, "r.started_at < ?"), append(args, filter.Until)
	}
	if filter.Method != "" {
		where, args = append(where, "r.method = ?"), append(args, strings.ToUpper(filter.Method))
	}
	if filter.StatusMin != 0 {
		where, args = append(where, "r.status_code >= ?"), append(args, filter.StatusMin)
	}
	if filter.StatusMax != 0 {
		where, args = append(where, "r.status_code <= ?"), append(args, filter.StatusMax)
	}
	if filter.MinDurationMs > 0 {
		where, args = append(where, "r.duration_ms >= ?"), append(args, filter.MinDurationMs)
	}
	if filter.Ajax != nil {
		where, args = append(where, "r.is_ajax = ?"), append(args, *filter.Ajax)
	}
	if filter.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM messages m, json_each(m.tags) t WHERE m.request_id = r.id AND t.value = ?)")
		args = append(args, filter.Tag)
	}
	switch filter.Level {
	case "error":
		where = append(where, "r.error_count > 0")
	case "warning":
		where = append(where, "(r.error_count > 0 OR r.warning_count > 0)")
	}
	if clause, searchArgs := requestSearch(filter.Query); clause != "" {
		where, args = append(where, clause), append(args, searchArgs...)
	}
	if filter.Cursor != "" {
		startedAt, id, err := decodeCursor(filter.Cursor)
		if err != nil {
			return RequestPage{}, err
		}
		where = append(where, "(r.started_at < ? OR (r.started_at = ? AND r.id < ?))")
		args = append(args, startedAt, startedAt, id)
	}
	args = append(args, filter.Limit+1)

	rows, err := s.db.QueryContext(ctx, selectRequestRows+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT ?`, args...)
	if err != nil {
		return RequestPage{}, err
	}
	defer func() { _ = rows.Close() }()

	var page RequestPage
	for rows.Next() {
		r, err := scanRequestRow(rows)
		if err != nil {
			return RequestPage{}, err
		}
		page.Items = append(page.Items, r)
	}
	if err := rows.Err(); err != nil {
		return RequestPage{}, err
	}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(last.StartedAt, last.ID)
	}
	return page, nil
}

const selectRequestRows = `
		SELECT r.id, r.project_id, p.server_id, r.request_key, COALESCE(r.parent_request_key, ''), COALESCE(f.format_version, 2),
			r.sapi, COALESCE(r.method, ''), COALESCE(r.url, ''), r.is_ajax, r.status_code, r.exit_code, r.started_at, r.duration_ms,
			r.finished, r.message_count, r.error_count, r.warning_count, r.received_at
		FROM requests r
		JOIN projects p ON p.id = r.project_id
		LEFT JOIN log_files f ON f.id = r.log_file_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequestRow(row rowScanner) (RequestRow, error) {
	var r RequestRow
	err := row.Scan(&r.ID, &r.ProjectID, &r.ServerID, &r.Key, &r.ParentKey, &r.FormatVersion, &r.Sapi, &r.Method, &r.URL,
		&r.IsAjax, &r.StatusCode, &r.ExitCode, &r.StartedAt, &r.DurationMs, &r.Finished, &r.MessageCount, &r.ErrorCount,
		&r.WarningCount, &r.ReceivedAt)
	return r, err
}

// GetRequest returns a feed request by id.
func (s *Store) GetRequest(ctx context.Context, id int64) (RequestRow, bool, error) {
	r, err := scanRequestRow(s.db.QueryRowContext(ctx, selectRequestRows+` WHERE r.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return RequestRow{}, false, nil
	}
	return r, err == nil, err
}

func encodeCursor(startedAt, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(startedAt, 10) + ":" + strconv.FormatInt(id, 10)))
}

func decodeCursor(cursor string) (startedAt, id int64, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	left, right, found := strings.Cut(string(raw), ":")
	startedAt, err1 := strconv.ParseInt(left, 10, 64)
	id, err2 := strconv.ParseInt(right, 10, 64)
	if !found || err1 != nil || err2 != nil {
		return 0, 0, ErrInvalidCursor
	}
	return startedAt, id, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
