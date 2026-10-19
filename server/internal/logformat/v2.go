package logformat

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RequestEnd is the data of a v2 request_end event.
type RequestEnd struct {
	EndedAt         int64
	DurationMs      *float64
	StatusCode      *int
	ExitCode        *int
	MemoryPeakBytes *int64
}

// V2Chunk is the result of parsing a run of complete v2 lines.
type V2Chunk struct {
	Start       *Request    // set when the chunk contains request_start
	Events      []Event     // message, timer and error events in file order
	End         *RequestEnd // set when the chunk contains request_end
	Consumed    int64       // bytes up to and including the last complete line
	LastSeq     int         // seq of the last parsed event, to continue the next chunk
	BrokenLines int         // lines that are not v2 events
	Problems    []string    // stream rule violations (spec/logformat/README.md); parsing goes on
}

type v2Frame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Class    string `json:"class"`
	CallType string `json:"call_type"`
}

// v2Event holds the fields of every event type; the reader ignores unknown fields.
type v2Event struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Seq       int    `json:"seq"`
	TS        int64  `json:"ts"`

	Sapi            string   `json:"sapi"`
	Lang            string   `json:"lang"`
	LibraryVersion  string   `json:"library_version"`
	PHPVersion      string   `json:"php_version"`
	ParentRequestID string   `json:"parent_request_id"`
	Method          string   `json:"method"`
	URL             string   `json:"url"`
	IsAjax          bool     `json:"is_ajax"`
	Argv            []string `json:"argv"`
	IP              string   `json:"ip"`
	ServerIP        string   `json:"server_ip"`
	ServerName      string   `json:"server_name"`
	Hostname        string   `json:"hostname"`
	PID             *int     `json:"pid"`
	ProcessUser     string   `json:"process_user"`
	AppUser         string   `json:"app_user"`
	SessionID       string   `json:"session_id"`

	Level          string          `json:"level"`
	Message        string          `json:"message"`
	Tags           []string        `json:"tags"`
	Context        json.RawMessage `json:"context"`
	Source         *v2Frame        `json:"source"`
	Trace          []v2Frame       `json:"trace"`
	Name           string          `json:"name"`
	DurationMs     *float64        `json:"duration_ms"`
	Kind           string          `json:"kind"`
	ErrorType      string          `json:"error_type"`
	ExceptionClass string          `json:"exception_class"`
	Code           json.RawMessage `json:"code"`
	Previous       json.RawMessage `json:"previous"`

	StatusCode      *int   `json:"status_code"`
	ExitCode        *int   `json:"exit_code"`
	MemoryPeakBytes *int64 `json:"memory_peak_bytes"`
}

// ParseV2Chunk parses the complete lines of data. lastSeq is the seq of the last event already
// stored for this file (0 at the start of the file); an incomplete last line is left for the next call.
func ParseV2Chunk(data []byte, lastSeq int) (V2Chunk, error) {
	chunk := V2Chunk{LastSeq: lastSeq}
	complete := bytes.LastIndexByte(data, '\n') + 1
	chunk.Consumed = int64(complete)

	var requestID string
	var lastTS int64
	ended := false
	for _, line := range bytes.SplitAfter(data[:complete], []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\n"))
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e v2Event
		if err := json.Unmarshal(line, &e); err != nil || e.V != 2 || e.Type == "" {
			chunk.BrokenLines++
			continue
		}

		if requestID == "" {
			requestID = e.RequestID
		} else if e.RequestID != requestID {
			chunk.problem("seq %d: request_id %q differs from %q", e.Seq, e.RequestID, requestID)
		}
		if e.Seq != chunk.LastSeq+1 {
			chunk.problem("seq %d after %d: lines are missing or out of order", e.Seq, chunk.LastSeq)
		}
		if e.TS < lastTS {
			chunk.problem("seq %d: ts goes backwards", e.Seq)
		}
		if ended {
			chunk.problem("seq %d: event after request_end", e.Seq)
		}
		if e.Type != "request_start" && chunk.LastSeq == 0 && chunk.Start == nil {
			chunk.problem("seq %d: the file does not begin with request_start", e.Seq)
		}
		chunk.LastSeq, lastTS = e.Seq, e.TS

		switch e.Type {
		case "request_start":
			if chunk.Start != nil || lastSeq > 0 {
				chunk.problem("seq %d: request_start repeated", e.Seq)
				continue
			}
			chunk.Start = v2Request(e)
		case "request_end":
			ended = true
			chunk.End = &RequestEnd{EndedAt: e.TS, DurationMs: e.DurationMs, StatusCode: e.StatusCode, ExitCode: e.ExitCode, MemoryPeakBytes: e.MemoryPeakBytes}
		case TypeMessage, TypeTimer, TypeError:
			chunk.Events = append(chunk.Events, v2ToEvent(e, len(line)))
		default:
			chunk.BrokenLines++
		}
	}
	return chunk, nil
}

func (c *V2Chunk) problem(format string, args ...any) {
	c.Problems = append(c.Problems, fmt.Sprintf(format, args...))
}

func v2Request(e v2Event) *Request {
	return &Request{
		Key: e.RequestID, ParentKey: e.ParentRequestID, FormatVersion: 2, Sapi: e.Sapi, Lang: e.Lang,
		Method: e.Method, URL: e.URL, IsAjax: e.IsAjax, Argv: e.Argv, IP: e.IP, ServerIP: e.ServerIP,
		ServerName: e.ServerName, Hostname: e.Hostname, PID: e.PID, ProcessUser: e.ProcessUser, AppUser: e.AppUser,
		SessionID: e.SessionID, LibraryVersion: e.LibraryVersion, PHPVersion: e.PHPVersion, StartedAt: e.TS,
	}
}

func v2ToEvent(e v2Event, size int) Event {
	event := Event{
		Seq: e.Seq, TS: e.TS, Type: e.Type, Level: e.Level, Message: e.Message, Tags: e.Tags,
		DurationMs: e.DurationMs, ErrorKind: e.Kind, ErrorType: e.ErrorType, ExceptionClass: e.ExceptionClass, Size: size,
	}
	if e.Type == TypeTimer {
		event.Message = e.Name
	}
	if trimmed := bytes.TrimSpace(e.Context); len(trimmed) > 0 && trimmed[0] == '{' {
		event.Context = append(json.RawMessage(nil), trimmed...)
	}
	if trimmed := bytes.TrimSpace(e.Previous); len(trimmed) > 0 && trimmed[0] == '[' {
		event.Previous = append(json.RawMessage(nil), trimmed...)
	}
	if e.Source != nil {
		source := Frame(*e.Source)
		event.Source = &source
	}
	for _, f := range e.Trace {
		event.Trace = append(event.Trace, Frame(f))
	}
	event.Code = codeText(e.Code)
	return event
}

// codeText turns a JSON integer or string error code into text.
func codeText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

// DetectFormat tells v2 (1st line is an event with "v": 2) from v1 (1st line is the header) by content.
// 0 means it cannot tell yet: the first line is incomplete.
func DetectFormat(data []byte) int {
	end := bytes.IndexByte(data, '\n')
	if end < 0 {
		return 0
	}
	var first struct {
		V        *int   `json:"v"`
		Finished *bool  `json:"bFinished"`
		Start    *int64 `json:"iRequestTimestamp"`
	}
	if json.Unmarshal(bytes.TrimRight(data[:end], " "), &first) != nil {
		return 1 // the v1 header can be corrupted (pre-hotfix library); v2 lines are always valid JSON
	}
	if first.V != nil && *first.V == 2 {
		return 2
	}
	return 1
}
