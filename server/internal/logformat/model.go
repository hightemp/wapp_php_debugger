// Package logformat разбирает файлы логов PHP-библиотеки (v1 и v2) в общую модель,
// соответствующую формату v2 (spec/logformat/README.md).
package logformat

import "encoding/json"

// Значения Request.Sapi.
const (
	SapiWeb = "web"
	SapiCLI = "cli"
)

// Значения Event.Type: события запроса, кроме request_start и request_end.
const (
	TypeMessage = "message"
	TypeTimer   = "timer"
	TypeError   = "error"
)

// Request — данные запроса: request_start и request_end формата v2 или заголовок v1.
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

// Frame — место в коде.
type Frame struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
	Class    string `json:"class,omitempty"`
	CallType string `json:"call_type,omitempty"`
}

// Event — сообщение, таймер или ошибка внутри запроса.
type Event struct {
	Seq            int
	TS             int64
	Type           string
	Level          string
	Message        string // для таймера — его имя
	Tags           []string
	Context        json.RawMessage // JSON-объект или nil
	Source         *Frame
	Trace          []Frame
	DurationMs     *float64
	ErrorKind      string
	ErrorType      string
	ExceptionClass string
	Code           string
	Previous       json.RawMessage // JSON-массив или nil
	Size           int             // размер исходной строки, байт
}

// Parsed — результат разбора одного файла лога.
type Parsed struct {
	Request     Request
	Events      []Event
	BrokenLines int // строки, которые не удалось разобрать (оборванные, затёртые)
}
