package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// RequestDetail is a request with everything known about it, plus child requests.
type RequestDetail struct {
	RequestRow
	Lang            string
	Argv            []string
	IP              string
	ServerIP        string
	ServerName      string
	Hostname        string
	PID             *int
	ProcessUser     string
	AppUser         string
	SessionID       string
	LibraryVersion  string
	PHPVersion      string
	EndedAt         *int64
	MemoryPeakBytes *int64
	Children        []RequestRow // requests whose parent_request_key is this request (AJAX, subprocesses)
}

// GetRequestDetail returns a request by id with its child requests.
func (s *Store) GetRequestDetail(ctx context.Context, id int64) (RequestDetail, bool, error) {
	row, found, err := s.GetRequest(ctx, id)
	if err != nil || !found {
		return RequestDetail{}, found, err
	}
	detail := RequestDetail{RequestRow: row}
	var argv, ip, serverIP, serverName, hostname, processUser, appUser, sessionID, libraryVersion, phpVersion sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT lang, argv, ip, server_ip, server_name, hostname, pid, process_user, app_user, session_id,
			library_version, php_version, ended_at, memory_peak_bytes
		FROM requests WHERE id = ?`, id).Scan(&detail.Lang, &argv, &ip, &serverIP, &serverName, &hostname, &detail.PID,
		&processUser, &appUser, &sessionID, &libraryVersion, &phpVersion, &detail.EndedAt, &detail.MemoryPeakBytes)
	if err != nil {
		return RequestDetail{}, false, err
	}
	if argv.Valid {
		if err := json.Unmarshal([]byte(argv.String), &detail.Argv); err != nil {
			return RequestDetail{}, false, err
		}
	}
	detail.IP, detail.ServerIP, detail.ServerName, detail.Hostname = ip.String, serverIP.String, serverName.String, hostname.String
	detail.ProcessUser, detail.AppUser, detail.SessionID = processUser.String, appUser.String, sessionID.String
	detail.LibraryVersion, detail.PHPVersion = libraryVersion.String, phpVersion.String

	rows, err := s.db.QueryContext(ctx, selectRequestRows+` WHERE r.project_id = ? AND r.parent_request_key = ? ORDER BY r.started_at, r.id`,
		row.ProjectID, row.Key)
	if err != nil {
		return RequestDetail{}, false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		child, err := scanRequestRow(rows)
		if err != nil {
			return RequestDetail{}, false, err
		}
		detail.Children = append(detail.Children, child)
	}
	return detail, true, rows.Err()
}

// MessageRow is a message in a list, without context and backtrace.
type MessageRow struct {
	ID         int64
	RequestID  int64
	Seq        int
	TS         int64
	Type       string
	Level      string
	Message    string
	Tags       []string
	Source     *logformat.Frame
	DurationMs *float64
	ErrorKind  string
	HasContext bool
	SizeBytes  int
}

// MessageFilter holds message list parameters. Empty fields do not filter.
type MessageFilter struct {
	RequestID int64
	Levels    []string
	Type      string
	Tag       string
	Cursor    string
	Limit     int
}

// MessagePage is a page of messages; NextCursor is empty on the last page.
type MessagePage struct {
	Items      []MessageRow
	NextCursor string
}

const selectMessageRows = `
	SELECT m.id, m.request_id, m.seq, m.ts, m.type, m.level, m.message, m.tags, m.source_file, m.source_line,
		m.source_function, m.source_class, m.duration_ms, m.error_kind, m.context IS NOT NULL, m.size_bytes
	FROM messages m`

func scanMessageRow(row rowScanner) (MessageRow, error) {
	var m MessageRow
	var tags string
	var file, function, class, errorKind sql.NullString
	var line sql.NullInt64
	err := row.Scan(&m.ID, &m.RequestID, &m.Seq, &m.TS, &m.Type, &m.Level, &m.Message, &tags, &file, &line,
		&function, &class, &m.DurationMs, &errorKind, &m.HasContext, &m.SizeBytes)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(tags), &m.Tags); err != nil {
		return m, err
	}
	if file.Valid {
		m.Source = &logformat.Frame{File: file.String, Line: int(line.Int64), Function: function.String, Class: class.String}
	}
	m.ErrorKind = errorKind.String
	return m, nil
}

// ListMessages returns request messages in seq order.
func (s *Store) ListMessages(ctx context.Context, filter MessageFilter) (MessagePage, error) {
	where, args := []string{"m.request_id = ?"}, []any{filter.RequestID}
	if len(filter.Levels) > 0 {
		where = append(where, "m.level IN ("+placeholders(len(filter.Levels))+")")
		for _, level := range filter.Levels {
			args = append(args, level)
		}
	}
	if filter.Type != "" {
		where, args = append(where, "m.type = ?"), append(args, filter.Type)
	}
	if filter.Tag != "" {
		where, args = append(where, "EXISTS (SELECT 1 FROM json_each(m.tags) WHERE value = ?)"), append(args, filter.Tag)
	}
	if filter.Cursor != "" {
		afterSeq, err := decodeSeqCursor(filter.Cursor)
		if err != nil {
			return MessagePage{}, err
		}
		where, args = append(where, "m.seq > ?"), append(args, afterSeq)
	}
	args = append(args, filter.Limit+1)

	rows, err := s.db.QueryContext(ctx, selectMessageRows+` WHERE `+strings.Join(where, " AND ")+` ORDER BY m.seq LIMIT ?`, args...)
	if err != nil {
		return MessagePage{}, err
	}
	defer func() { _ = rows.Close() }()
	var page MessagePage
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return MessagePage{}, err
		}
		page.Items = append(page.Items, m)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, err
	}
	if len(page.Items) > filter.Limit {
		page.Items = page.Items[:filter.Limit]
		page.NextCursor = encodeSeqCursor(page.Items[len(page.Items)-1].Seq)
	}
	return page, nil
}

// MessageDetail is a message with context, backtrace and error details.
type MessageDetail struct {
	MessageRow
	ProjectID      int64
	Context        json.RawMessage
	Trace          []logformat.Frame
	ErrorType      string
	ExceptionClass string
	Code           string
	Previous       json.RawMessage
}

// GetMessage returns a message by id.
func (s *Store) GetMessage(ctx context.Context, id int64) (MessageDetail, bool, error) {
	row, err := scanMessageRow(s.db.QueryRowContext(ctx, selectMessageRows+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MessageDetail{}, false, nil
	}
	if err != nil {
		return MessageDetail{}, false, err
	}
	detail := MessageDetail{MessageRow: row}
	var context, trace, errorType, exceptionClass, code, previous sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT r.project_id, m.context, m.trace, m.error_type, m.exception_class, m.code, m.previous
		FROM messages m JOIN requests r ON r.id = m.request_id WHERE m.id = ?`, id).Scan(
		&detail.ProjectID, &context, &trace, &errorType, &exceptionClass, &code, &previous)
	if err != nil {
		return MessageDetail{}, false, err
	}
	if context.Valid {
		detail.Context = json.RawMessage(context.String)
	}
	if previous.Valid {
		detail.Previous = json.RawMessage(previous.String)
	}
	if trace.Valid {
		if err := json.Unmarshal([]byte(trace.String), &detail.Trace); err != nil {
			return MessageDetail{}, false, err
		}
	}
	if detail.Source != nil {
		// call_type is only kept in the trace; the source frame is its first entry when present
		if len(detail.Trace) > 0 && detail.Trace[0].File == detail.Source.File && detail.Trace[0].Line == detail.Source.Line {
			detail.Source.CallType = detail.Trace[0].CallType
		}
	}
	detail.ErrorType, detail.ExceptionClass, detail.Code = errorType.String, exceptionClass.String, code.String
	return detail, true, nil
}

// ProjectLinks is what IDE links need from a project.
type ProjectLinks struct {
	Editor    string
	RootPath  string
	LocalPath string
}

// ProjectLinks returns the editor and paths of a project.
func (s *Store) ProjectLinks(ctx context.Context, projectID int64) (ProjectLinks, bool, error) {
	var links ProjectLinks
	var local sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT editor, root_path, local_path FROM projects WHERE id = ?`, projectID).Scan(&links.Editor, &links.RootPath, &local)
	if errors.Is(err, sql.ErrNoRows) {
		return links, false, nil
	}
	links.LocalPath = local.String
	return links, err == nil, err
}

func encodeSeqCursor(seq int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("s:" + strconv.Itoa(seq)))
}

func decodeSeqCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || !strings.HasPrefix(string(raw), "s:") {
		return 0, ErrInvalidCursor
	}
	seq, err := strconv.Atoi(strings.TrimPrefix(string(raw), "s:"))
	if err != nil {
		return 0, ErrInvalidCursor
	}
	return seq, nil
}
