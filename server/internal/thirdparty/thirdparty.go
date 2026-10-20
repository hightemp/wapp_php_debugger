// Package thirdparty turns logs the library did not write - the PHP error log, a Laravel log, the
// nginx error log - into a request of log format v2, so they can be imported next to the others.
// A file becomes one request, like a long CLI run named after it; every entry is a message.
package thirdparty

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // PHP writes the zone name of date.timezone; Windows has no zone database

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// entriesMax bounds one import: bigger logs are better split (or tailed by the agent in v2).
const entriesMax = 100_000

type entry struct {
	ts    int64
	event map[string]any // the fields of the v2 event without v, request_id, seq and ts
}

type format struct {
	name   string
	header *regexp.Regexp // the first line of an entry; other lines continue it
	parse  func(match []string, continued []string) (entry, bool)
}

var formats = []format{
	{name: "php", header: regexp.MustCompile(`^\[(\d{2}-[A-Za-z]{3}-\d{4} \d{2}:\d{2}:\d{2}) ([^\]]+)\] (.*)$`), parse: parsePHP},
	{name: "laravel", header: regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:?\d{2}|Z)?)\] ([\w-]+)\.([A-Za-z]+): (.*)$`), parse: parseLaravel},
	{name: "nginx", header: regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[([a-z]+)\] \d+#\d+: (?:\*\d+ )?(.*)$`), parse: parseNginx},
}

// Convert gives the v2 stream of a third-party log; ok is false when the format is not one of
// these (a v2 file, say). The same file always gives the same request id: an import twice finds it.
func Convert(name string, data []byte) ([]byte, bool, error) {
	first := firstLine(data)
	for _, f := range formats {
		if f.header.MatchString(first) {
			stream, err := convert(f, name, data)
			return stream, true, err
		}
	}
	return nil, false, nil
}

func firstLine(data []byte) string {
	for line := range bytes.Lines(data) {
		if text := strings.TrimRight(string(line), "\r\n"); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func convert(f format, name string, data []byte) ([]byte, error) {
	var entries []entry
	var match []string
	var continued []string
	flush := func() {
		if match != nil {
			if e, ok := f.parse(match, continued); ok {
				entries = append(entries, e)
			}
		}
		match, continued = nil, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if m := f.header.FindStringSubmatch(line); m != nil {
			flush()
			match = m
			continue
		}
		if match != nil {
			continued = append(continued, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()
	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries of a %s log", f.name)
	}
	if len(entries) > entriesMax {
		return nil, fmt.Errorf("%d entries: more than %d in one import, split the file", len(entries), entriesMax)
	}

	sum := sha256.Sum256(data)
	key := ulid(entries[0].ts, sum[:10])
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	write := func(seq int, ts int64, kind string, fields map[string]any) error {
		event := map[string]any{"v": 2, "type": kind, "request_id": key, "seq": seq, "ts": ts}
		for k, v := range fields {
			event[k] = v
		}
		return encoder.Encode(event)
	}
	start := entries[0].ts
	if err := write(1, start, "request_start", map[string]any{"sapi": "cli", "lang": "php", "library_version": "import:" + f.name, "argv": []string{name}}); err != nil {
		return nil, err
	}
	last := start
	for i, e := range entries {
		last = max(last, e.ts) // the stream wants time that never goes back; a log may have it go back
		kind := e.event["type"].(string)
		delete(e.event, "type")
		if err := write(i+2, last, kind, e.event); err != nil {
			return nil, err
		}
	}
	if err := write(len(entries)+2, last, "request_end", map[string]any{"duration_ms": float64(last - start)}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ulid makes a ULID of the first entry's time and of the file content instead of random bits.
func ulid(ms int64, entropy []byte) string {
	var id [26]byte
	t := uint64(max(ms, 0))
	for i := 9; i >= 0; i-- {
		id[i] = crockford[t%32]
		t /= 32
	}
	var bits uint64
	var have uint
	pos := 10
	for _, b := range entropy {
		bits = bits<<8 | uint64(b)
		have += 8
		for have >= 5 && pos < 26 {
			have -= 5
			id[pos] = crockford[(bits>>have)&31]
			pos++
		}
	}
	return string(id[:])
}

// A frame of a PHP stack trace: #0 /var/www/app/Orders.php(31): App\Orders->load(7)
var phpFrame = regexp.MustCompile(`^#\d+ (.+)\((\d+)\): (?:([\w\\]+)(->|::))?(\w+)\(`)

func frames(lines []string, unescape bool) []logformat.Frame {
	var trace []logformat.Frame
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if unescape {
			line = strings.ReplaceAll(line, `\\`, `\`)
		}
		if m := phpFrame.FindStringSubmatch(line); m != nil {
			number, _ := strconv.Atoi(m[2])
			trace = append(trace, logformat.Frame{File: m[1], Line: number, Class: m[3], CallType: m[4], Function: m[5]})
		}
	}
	return trace
}

func frameJSON(f logformat.Frame) map[string]any {
	frame := map[string]any{"file": f.File, "line": f.Line}
	if f.Function != "" {
		frame["function"] = f.Function
	}
	if f.Class != "" {
		frame["class"] = f.Class
		frame["call_type"] = f.CallType
	}
	return frame
}

func traceJSON(trace []logformat.Frame) []map[string]any {
	out := make([]map[string]any, 0, len(trace))
	for _, f := range trace {
		out = append(out, frameJSON(f))
	}
	return out
}

// PHP error log ---------------------------------------------------------------------------------

var (
	phpError    = regexp.MustCompile(`^PHP ([A-Za-z ]+?):\s+(.*)$`)
	phpAt       = regexp.MustCompile(`^(.*) in (.+) on line (\d+)$`)
	phpUncaught = regexp.MustCompile(`^Uncaught ([\w\\]+): (.*) in (.+):(\d+)$`)
	phpTypes    = map[string][3]string{ // error type, kind, level
		"Fatal error":             {"E_ERROR", "fatal", "error"},
		"Parse error":             {"E_PARSE", "fatal", "error"},
		"Recoverable fatal error": {"E_RECOVERABLE_ERROR", "php_error", "error"},
		"Warning":                 {"E_WARNING", "php_error", "warning"},
		"Notice":                  {"E_NOTICE", "php_error", "info"},
		"Deprecated":              {"E_DEPRECATED", "php_error", "info"},
		"Strict Standards":        {"E_STRICT", "php_error", "info"},
	}
)

func parsePHP(match []string, continued []string) (entry, bool) {
	location, err := time.LoadLocation(match[2])
	if err != nil {
		location = time.UTC
	}
	at, err := time.ParseInLocation("02-Jan-2006 15:04:05", match[1], location)
	if err != nil {
		return entry{}, false
	}
	text := match[3]
	m := phpError.FindStringSubmatch(text)
	known, isError := phpTypes[strings.TrimSpace(firstOr(m, 1))]
	if m == nil || !isError {
		body := strings.Join(append([]string{text}, continued...), "\n")
		return entry{ts: at.UnixMilli(), event: map[string]any{"type": "message", "level": "message", "message": body}}, true
	}
	message := m[2]
	event := map[string]any{"type": "error", "kind": known[1], "level": known[2], "error_type": known[0]}
	if u := phpUncaught.FindStringSubmatch(message); u != nil {
		line, _ := strconv.Atoi(u[4])
		event["kind"], event["exception_class"], event["message"] = "exception", u[1], u[2]
		delete(event, "error_type")
		event["source"] = map[string]any{"file": u[3], "line": line}
		if trace := frames(continued, false); len(trace) > 0 {
			event["trace"] = traceJSON(trace)
		}
		return entry{ts: at.UnixMilli(), event: event}, true
	}
	event["message"] = message
	if a := phpAt.FindStringSubmatch(message); a != nil {
		line, _ := strconv.Atoi(a[3])
		event["message"], event["source"] = a[1], map[string]any{"file": a[2], "line": line}
	}
	return entry{ts: at.UnixMilli(), event: event}, true
}

func firstOr(m []string, i int) string {
	if len(m) > i {
		return m[i]
	}
	return ""
}

// Laravel ---------------------------------------------------------------------------------------

var (
	laravelLevels = map[string]string{"debug": "debug", "info": "info", "notice": "info", "warning": "warning",
		"error": "error", "critical": "error", "alert": "error", "emergency": "error"}
	laravelException = regexp.MustCompile(`(?s)"exception":"\[object\] \(([\w\\]+)\(code: ([^)]*)\): .*? at (.+?):(\d+)\)`)
	tagPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)
)

func parseLaravel(match []string, continued []string) (entry, bool) {
	at, ok := parseTime(match[1], []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04:05.999999", "2006-01-02T15:04:05.999999Z07:00", "2006-01-02 15:04:05Z07:00", "2006-01-02T15:04:05Z07:00"})
	if !ok {
		return entry{}, false
	}
	text := strings.Join(append([]string{match[4]}, continued...), "\n")
	level := laravelLevels[strings.ToLower(match[3])]
	if level == "" {
		level = "message"
	}
	event := map[string]any{"type": "message", "level": level}
	if env := strings.ToLower(match[2]); tagPattern.MatchString(env) && len(env) <= 64 {
		event["tags"] = []string{env}
	}

	// "message {context} [extra]": the first place the rest reads as JSON values splits them
	message, context, found := splitJSONTail(text)
	if !found {
		message = text
		if i := strings.Index(text, ` {"exception":`); i >= 0 {
			message = text[:i]
		}
		if e := laravelException.FindStringSubmatch(text); e != nil {
			line, _ := strconv.Atoi(e[4])
			event = map[string]any{"type": "error", "kind": "exception", "level": "error", "exception_class": strings.ReplaceAll(e[1], `\\`, `\`),
				"code": e[2], "source": map[string]any{"file": e[3], "line": line}}
			if trace := frames(continued, true); len(trace) > 0 {
				event["trace"] = traceJSON(trace)
			}
		}
	}
	event["message"] = strings.TrimSpace(message)
	if len(context) > 0 {
		event["context"] = context
	}
	return entry{ts: at.UnixMilli(), event: event}, true
}

// splitJSONTail finds where "message {context} [extra]" turns into JSON; the context is the first
// non-empty object of the tail.
func splitJSONTail(text string) (string, map[string]any, bool) {
	for i := 0; i < len(text); i++ {
		if text[i] != ' ' || i+1 >= len(text) || (text[i+1] != '{' && text[i+1] != '[') {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(text[i+1:]))
		var context map[string]any
		valid := true
		for decoder.More() {
			var value any
			if err := decoder.Decode(&value); err != nil {
				valid = false
				break
			}
			if object, ok := value.(map[string]any); ok && context == nil && len(object) > 0 {
				context = object
			}
		}
		if valid {
			return text[:i], context, true
		}
	}
	return text, nil, false
}

func parseTime(text string, layouts []string) (time.Time, bool) {
	for _, layout := range layouts {
		if at, err := time.Parse(layout, text); err == nil {
			return at, true
		}
	}
	return time.Time{}, false
}

// nginx -----------------------------------------------------------------------------------------

var (
	nginxLevels = map[string]string{"emerg": "error", "alert": "error", "crit": "error", "error": "error",
		"warn": "warning", "notice": "info", "info": "info", "debug": "debug"}
	nginxField = regexp.MustCompile(`, (client|server|request|upstream|host|referrer): ("(?:[^"\\]|\\.)*"|[^,]*)`)
)

func parseNginx(match []string, continued []string) (entry, bool) {
	at, err := time.ParseInLocation("2006/01/02 15:04:05", match[1], time.UTC)
	if err != nil {
		return entry{}, false
	}
	text := strings.Join(append([]string{match[3]}, continued...), "\n")
	level := nginxLevels[match[2]]
	if level == "" {
		level = "message"
	}
	event := map[string]any{"type": "message", "level": level}
	if i := strings.Index(text, ", client: "); i >= 0 {
		context := map[string]string{}
		for _, field := range nginxField.FindAllStringSubmatch(text[i:], -1) {
			context[field[1]] = strings.Trim(field[2], `"`)
		}
		event["context"], text = context, text[:i]
	}
	event["message"] = text
	return entry{ts: at.UnixMilli(), event: event}, true
}
