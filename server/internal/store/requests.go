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

// ErrInvalidCursor — курсор страницы повреждён или получен не от ListRequests.
var ErrInvalidCursor = errors.New("invalid cursor")

// Store — запросы к базе просмотрщика.
type Store struct {
	db *sql.DB
}

// New оборачивает открытую базу (см. Open).
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// LocalProject — идентификаторы локального сервера и проекта.
type LocalProject struct {
	ServerID  int64
	ProjectID int64
}

// EnsureLocalProject создаёт (или находит) сервер "local" и проект на нём.
func (s *Store) EnsureLocalProject(ctx context.Context, name, rootPath, logsDir string) (LocalProject, error) {
	now := time.Now().UnixMilli()
	var ids LocalProject
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO servers (name, kind, environment, created_at, updated_at) VALUES ('local', 'local', 'local', ?, ?)
		ON CONFLICT (name) DO UPDATE SET name = excluded.name
		RETURNING id`, now, now).Scan(&ids.ServerID)
	if err != nil {
		return ids, err
	}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO projects (server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (server_id, name) DO UPDATE SET root_path = excluded.root_path, logs_dir = excluded.logs_dir, updated_at = excluded.updated_at
		RETURNING id`, ids.ServerID, name, rootPath, logsDir, now, now).Scan(&ids.ProjectID)
	return ids, err
}

// FileState — что известно о файле лога: по размеру и времени изменения видно, надо ли его перечитать.
type FileState struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// FileState возвращает сохранённое состояние файла лога проекта.
func (s *Store) FileState(ctx context.Context, projectID int64, name string) (FileState, bool, error) {
	state := FileState{Name: name}
	var mtime int64
	err := s.db.QueryRowContext(ctx, `SELECT size, mtime FROM log_files WHERE project_id = ? AND path = ?`, projectID, name).Scan(&state.Size, &mtime)
	if errors.Is(err, sql.ErrNoRows) {
		return state, false, nil
	}
	state.ModTime = time.UnixMilli(mtime)
	return state, err == nil, err
}

// SaveResult — итог сохранения файла.
type SaveResult struct {
	RequestID int64
	Created   bool
}

// SaveFile сохраняет разобранный файл лога: состояние файла, запрос и его сообщения.
// Повторное сохранение того же запроса заменяет сообщения целиком (файл v1 перечитывается полностью).
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
		VALUES (`+placeholders(20)+`)`,
		requestID, e.Seq, e.TS, e.Type, e.Level, e.Message, string(encodedTags), nullJSON(e.Context),
		nullString(source.File), nullInt(source.Line), nullString(source.Function), nullString(source.Class), trace,
		e.DurationMs, nullString(e.ErrorKind), nullString(e.ErrorType), nullString(e.ExceptionClass), nullString(e.Code),
		nullJSON(e.Previous), e.Size)
	return err
}

// RequestRow — запрос в ленте.
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

// RequestFilter — параметры ленты. ProjectID = 0 — все проекты.
type RequestFilter struct {
	ProjectID int64
	Limit     int
	Cursor    string
}

// RequestPage — страница ленты; NextCursor пустой на последней странице.
type RequestPage struct {
	Items      []RequestRow
	NextCursor string
}

// ListRequests возвращает запросы, новые сверху; одинаковое время упорядочивается по id.
func (s *Store) ListRequests(ctx context.Context, filter RequestFilter) (RequestPage, error) {
	where, args := []string{"1 = 1"}, []any{}
	if filter.ProjectID != 0 {
		where, args = append(where, "r.project_id = ?"), append(args, filter.ProjectID)
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

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.project_id, p.server_id, r.request_key, COALESCE(r.parent_request_key, ''), COALESCE(f.format_version, 2),
			r.sapi, COALESCE(r.method, ''), COALESCE(r.url, ''), r.is_ajax, r.status_code, r.exit_code, r.started_at, r.duration_ms,
			r.finished, r.message_count, r.error_count, r.warning_count, r.received_at
		FROM requests r
		JOIN projects p ON p.id = r.project_id
		LEFT JOIN log_files f ON f.id = r.log_file_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT ?`, args...)
	if err != nil {
		return RequestPage{}, err
	}
	defer func() { _ = rows.Close() }()

	var page RequestPage
	for rows.Next() {
		var r RequestRow
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.ServerID, &r.Key, &r.ParentKey, &r.FormatVersion, &r.Sapi, &r.Method, &r.URL,
			&r.IsAjax, &r.StatusCode, &r.ExitCode, &r.StartedAt, &r.DurationMs, &r.Finished, &r.MessageCount, &r.ErrorCount,
			&r.WarningCount, &r.ReceivedAt); err != nil {
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
