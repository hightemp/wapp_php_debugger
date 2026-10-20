package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// ExistsError says the imported request is already in the project.
type ExistsError struct{ RequestID int64 }

func (e *ExistsError) Error() string {
	return fmt.Sprintf("the request is already here (id %d)", e.RequestID)
}

// importedPrefix marks log_files rows of imported requests: no source has such a file.
const importedPrefix = "imported/"

// ImportRequest saves a request given as a v2 stream (an export or a library log file).
// The stream must be whole and valid: one request, from request_start on, seq without gaps.
func (s *Store) ImportRequest(ctx context.Context, projectID int64, data []byte) (int64, error) {
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	chunk, err := logformat.ParseV2Chunk(data, 0)
	switch {
	case err != nil:
		return 0, err
	case chunk.BrokenLines > 0:
		return 0, fmt.Errorf("%d lines are not events of log format v2", chunk.BrokenLines)
	case len(chunk.Problems) > 0:
		return 0, errors.New(strings.Join(chunk.Problems, "; "))
	case chunk.Start == nil:
		return 0, errors.New("no request_start: not a request of log format v2")
	}

	var existing int64
	err = s.db.QueryRowContext(ctx, `SELECT id FROM requests WHERE project_id = ? AND request_key = ?`, projectID, chunk.Start.Key).Scan(&existing)
	if err == nil {
		return 0, &ExistsError{RequestID: existing}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	file := FileState{Name: importedPrefix + chunk.Start.Key + ".jsonl", Size: int64(len(data)), ModTime: time.Now()}
	saved, err := s.SaveV2Chunk(ctx, projectID, file, 0, chunk)
	return saved.RequestID, err
}

// v2Line is one event of the exported stream, in the field names of the format.
type v2Line struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Seq       int    `json:"seq"`
	TS        int64  `json:"ts"`

	Sapi            string   `json:"sapi,omitempty"`
	Lang            string   `json:"lang,omitempty"`
	LibraryVersion  string   `json:"library_version,omitempty"`
	PHPVersion      string   `json:"php_version,omitempty"`
	ParentRequestID string   `json:"parent_request_id,omitempty"`
	Method          string   `json:"method,omitempty"`
	URL             string   `json:"url,omitempty"`
	IsAjax          *bool    `json:"is_ajax,omitempty"`
	Argv            []string `json:"argv,omitempty"`
	IP              string   `json:"ip,omitempty"`
	ServerIP        string   `json:"server_ip,omitempty"`
	ServerName      string   `json:"server_name,omitempty"`
	Hostname        string   `json:"hostname,omitempty"`
	PID             *int     `json:"pid,omitempty"`
	ProcessUser     string   `json:"process_user,omitempty"`
	AppUser         string   `json:"app_user,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`

	Level          string            `json:"level,omitempty"`
	Message        *string           `json:"message,omitempty"`
	Name           string            `json:"name,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Context        json.RawMessage   `json:"context,omitempty"`
	Source         *logformat.Frame  `json:"source,omitempty"`
	Trace          []logformat.Frame `json:"trace,omitempty"`
	DurationMs     *float64          `json:"duration_ms,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	ErrorType      string            `json:"error_type,omitempty"`
	ExceptionClass string            `json:"exception_class,omitempty"`
	Code           string            `json:"code,omitempty"`
	Previous       json.RawMessage   `json:"previous,omitempty"`

	StatusCode      *int   `json:"status_code,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	MemoryPeakBytes *int64 `json:"memory_peak_bytes,omitempty"`
}

// ExportRequest writes a request as a v2 stream, the way the library would have logged it, and
// returns its key. A request read from a v1 file comes out in v2 too; seq is numbered anew.
func (s *Store) ExportRequest(ctx context.Context, requestID int64) ([]byte, string, error) {
	var (
		start                                                       v2Line
		parent, method, url, argv, ip, serverIP, serverName, host   sql.NullString
		processUser, appUser, sessionID, libraryVersion, phpVersion sql.NullString
		endedAt                                                     sql.NullInt64
		finished                                                    bool
		isAjax                                                      bool
		durationMs                                                  *float64
		statusCode, exitCode                                        *int
		memoryPeak                                                  *int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT request_key, parent_request_key, sapi, lang, method, url, is_ajax, argv, ip, server_ip, server_name, hostname, pid,
			process_user, app_user, session_id, library_version, php_version, started_at, ended_at, duration_ms, status_code,
			exit_code, memory_peak_bytes, finished
		FROM requests WHERE id = ?`, requestID).Scan(
		&start.RequestID, &parent, &start.Sapi, &start.Lang, &method, &url, &isAjax, &argv, &ip, &serverIP, &serverName, &host, &start.PID,
		&processUser, &appUser, &sessionID, &libraryVersion, &phpVersion, &start.TS, &endedAt, &durationMs, &statusCode,
		&exitCode, &memoryPeak, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	start.V, start.Type, start.Seq = 2, "request_start", 1
	start.ParentRequestID, start.Method, start.URL = parent.String, method.String, url.String
	start.IP, start.ServerIP, start.ServerName, start.Hostname = ip.String, serverIP.String, serverName.String, host.String
	start.ProcessUser, start.AppUser, start.SessionID = processUser.String, appUser.String, sessionID.String
	start.LibraryVersion, start.PHPVersion = libraryVersion.String, phpVersion.String
	if start.Sapi == logformat.SapiWeb {
		start.IsAjax = &isAjax
	}
	if argv.Valid {
		if err := json.Unmarshal([]byte(argv.String), &start.Argv); err != nil {
			return nil, "", err
		}
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(start); err != nil {
		return nil, "", err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, type, level, message, tags, context, source_file, source_line, source_function, source_class, trace,
			duration_ms, error_kind, error_type, exception_class, code, previous
		FROM messages WHERE request_id = ? ORDER BY seq`, requestID)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()
	seq, lastTS := 1, start.TS
	for rows.Next() {
		var (
			line                                            v2Line
			text, tags                                      string
			context, file, function, class, trace           sql.NullString
			kind, errorType, exceptionClass, code, previous sql.NullString
			sourceLine                                      sql.NullInt64
		)
		if err := rows.Scan(&line.TS, &line.Type, &line.Level, &text, &tags, &context, &file, &sourceLine, &function, &class, &trace,
			&line.DurationMs, &kind, &errorType, &exceptionClass, &code, &previous); err != nil {
			return nil, "", err
		}
		seq++
		line.V, line.RequestID, line.Seq = 2, start.RequestID, seq
		if line.Type == logformat.TypeTimer {
			line.Name = text
		} else {
			line.Message = &text
		}
		if err := json.Unmarshal([]byte(tags), &line.Tags); err != nil {
			return nil, "", err
		}
		if context.Valid {
			line.Context = json.RawMessage(context.String)
		}
		if file.Valid || sourceLine.Valid || function.Valid || class.Valid {
			line.Source = &logformat.Frame{File: file.String, Line: int(sourceLine.Int64), Function: function.String, Class: class.String}
		}
		if trace.Valid {
			if err := json.Unmarshal([]byte(trace.String), &line.Trace); err != nil {
				return nil, "", err
			}
		}
		line.Kind, line.ErrorType, line.ExceptionClass, line.Code = kind.String, errorType.String, exceptionClass.String, code.String
		if previous.Valid {
			line.Previous = json.RawMessage(previous.String)
		}
		if err := encoder.Encode(line); err != nil {
			return nil, "", err
		}
		lastTS = line.TS
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if finished {
		end := v2Line{V: 2, Type: "request_end", RequestID: start.RequestID, Seq: seq + 1, TS: max(lastTS, endedAt.Int64),
			DurationMs: durationMs, StatusCode: statusCode, ExitCode: exitCode, MemoryPeakBytes: memoryPeak}
		if err := encoder.Encode(end); err != nil {
			return nil, "", err
		}
	}
	return out.Bytes(), start.RequestID, nil
}
