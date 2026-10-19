package logformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type v1Header struct {
	URL              string   `json:"sURL"`
	IP               string   `json:"sIP"`
	ServerIP         string   `json:"sServerIP"`
	ServerName       string   `json:"sServerName"`
	IsCli            bool     `json:"bIsCli"`
	IsAjax           bool     `json:"bIsAjax"`
	Argv             []string `json:"aArgv"`
	User             string   `json:"sUser"`
	SysUser          string   `json:"sSysUser"`
	Session          string   `json:"sSession"`
	RequestTimestamp int64    `json:"iRequestTimestamp"`
	Lang             string   `json:"sLang"`
	Finished         bool     `json:"bFinished"`
	Code             int      `json:"iCode"`
	StartTime        int64    `json:"iStartTime"`
	StopTime         int64    `json:"iStopTime"`
	TotalTime        int64    `json:"iTotalTime"`
}

type v1Message struct {
	Type      *string         `json:"sType"`
	Message   json.RawMessage `json:"sMessage"`
	Vars      json.RawMessage `json:"oVars"`
	BackTrace *struct {
		File     *string `json:"file"`
		Line     *int    `json:"line"`
		Function *string `json:"function"`
	} `json:"oBackTrace"`
	Trace []struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Function string `json:"function"`
		Class    string `json:"class"`
		Type     string `json:"type"`
	} `json:"aBackTrace"`
	Timestamp int64 `json:"iTimestamp"`
}

// ParseV1 разбирает файл лога v1: заголовок фиксированной длины и строки сообщений.
// Битые строки (оборванные, затёртые длинным заголовком) пропускаются и считаются в BrokenLines.
func ParseV1(name string, data []byte) (Parsed, error) {
	headerLine, rest, _ := bytes.Cut(data, []byte("\n"))

	var header v1Header
	if err := json.Unmarshal(bytes.TrimRight(headerLine, " "), &header); err != nil {
		return Parsed{}, fmt.Errorf("%s: invalid v1 header: %w", name, err)
	}

	parsed := Parsed{Request: v1Request(name, header)}
	for _, line := range bytes.Split(rest, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var raw v1Message
		if err := json.Unmarshal(line, &raw); err != nil || raw.Type == nil {
			parsed.BrokenLines++
			continue
		}
		parsed.Events = append(parsed.Events, v1Event(raw, len(parsed.Events)+1, len(line)))
	}
	return parsed, nil
}

func v1Request(name string, h v1Header) Request {
	r := Request{
		Key:           strconv.FormatInt(h.RequestTimestamp, 10),
		FormatVersion: 1,
		Sapi:          SapiWeb,
		Lang:          h.Lang,
		URL:           h.URL,
		IsAjax:        h.IsAjax,
		IP:            h.IP,
		ServerIP:      h.ServerIP,
		ServerName:    h.ServerName,
		ProcessUser:   h.User,
		AppUser:       h.SysUser,
		SessionID:     h.Session,
		StartedAt:     h.RequestTimestamp,
		Finished:      h.Finished,
	}
	if h.RequestTimestamp == 0 {
		r.Key = name
	}
	if r.Lang == "" {
		r.Lang = "php"
	}
	if h.IsCli {
		r.Sapi = SapiCLI
		r.Argv = h.Argv
	}
	if h.StartTime != 0 {
		r.StartedAt = h.StartTime
	}
	if h.Finished {
		ended, duration := h.StopTime, float64(h.TotalTime)
		r.EndedAt, r.DurationMs = &ended, &duration
		if r.Sapi == SapiWeb && h.Code > 0 {
			code := h.Code
			r.StatusCode = &code
		}
	}
	return r
}

// v1 пишет время таймера в тексте: "#timediff #name 12 мс"
var v1TimerPattern = regexp.MustCompile(`^#timediff #(\S+) (\d+(?:\.\d+)?) мс$`)

func v1Event(m v1Message, seq, size int) Event {
	e := Event{
		Seq:     seq,
		TS:      m.Timestamp,
		Type:    TypeMessage,
		Level:   v1Level(*m.Type),
		Message: messageText(m.Message),
		Size:    size,
	}
	e.Tags = extractTags(e.Message)
	if trimmed := bytes.TrimSpace(m.Vars); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("{}")) && trimmed[0] == '{' {
		e.Context = append(json.RawMessage(nil), trimmed...)
	}
	if bt := m.BackTrace; bt != nil && bt.File != nil {
		e.Source = &Frame{File: *bt.File}
		if bt.Line != nil {
			e.Source.Line = *bt.Line
		}
		if bt.Function != nil {
			e.Source.Function = *bt.Function
		}
	}
	for _, f := range m.Trace {
		e.Trace = append(e.Trace, Frame{File: f.File, Line: f.Line, Function: f.Function, Class: f.Class, CallType: f.Type})
	}
	if match := v1TimerPattern.FindStringSubmatch(e.Message); match != nil {
		duration, _ := strconv.ParseFloat(match[2], 64)
		e.Type, e.Message, e.DurationMs = TypeTimer, match[1], &duration
	}
	return e
}

// v1Level сводит sType v1 к уровням v2: неизвестные типы (functions_debug и любые из __d()) — message.
func v1Level(sType string) string {
	switch sType {
	case "error", "warning", "info", "message":
		return sType
	default:
		return "message"
	}
}

// messageText приводит sMessage (любой скаляр в v1) к строке.
func messageText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return ""
	}
	return trimmed
}

var v1TagPattern = regexp.MustCompile(`(?:^|\s)#(\S+)`)

// v2TagPattern — допустимый тег v2 (spec/logformat/v2/event.schema.json).
var v2TagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)

// extractTags достаёт теги "#tag" из текста v1. Теги, которые нельзя записать в v2
// (например, кириллические), остаются только в тексте.
func extractTags(text string) []string {
	var tags []string
	seen := map[string]bool{}
	for _, match := range v1TagPattern.FindAllStringSubmatch(text, -1) {
		tag := strings.ToLower(match[1])
		if v2TagPattern.MatchString(tag) && len(tag) <= 64 && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
