// Package logformat parses PHP library log files (v1 and v2) into a common model
// that follows the v2 format (spec/logformat/README.md).
package logformat

import "encoding/json"

// Values of Request.Sapi.
const (
	SapiWeb = "web"
	SapiCLI = "cli"
)

// Values of Event.Type: request events other than request_start and request_end.
const (
	TypeMessage = "message"
	TypeTimer   = "timer"
	TypeError   = "error"
)

// Request is the request data: request_start and request_end of v2 or the v1 header.
type Request struct {
	Key             string
	ParentKey       string
	FormatVersion   int
	Sapi            string
	Lang            string
	Method          string
	URL             string
	IsAjax          bool
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
	StartedAt       int64
	EndedAt         *int64
	DurationMs      *float64
	StatusCode      *int
	ExitCode        *int
	MemoryPeakBytes *int64
	Finished        bool
}

// Frame is a location in code.
type Frame struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
	Class    string `json:"class,omitempty"`
	CallType string `json:"call_type,omitempty"`
}

// Event is a message, a timer or an error inside a request.
type Event struct {
	Seq            int
	TS             int64
	Type           string
	Level          string
	Message        string // for a timer, its name
	Tags           []string
	Context        json.RawMessage // a JSON object or nil
	Source         *Frame
	Trace          []Frame
	DurationMs     *float64
	ErrorKind      string
	ErrorType      string
	ExceptionClass string
	Code           string
	Previous       json.RawMessage // a JSON array or nil
	Size           int             // size of the source line, bytes
}

// Parsed is the result of parsing one log file.
type Parsed struct {
	Request     Request
	Events      []Event
	BrokenLines int // lines that could not be parsed (truncated, overwritten)
}
