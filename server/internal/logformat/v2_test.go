package logformat

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readV2(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(t, "v2"), name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func lineCount(data []byte) int {
	return bytes.Count(data, []byte("\n"))
}

func TestParseV2ValidFixtures(t *testing.T) {
	cases := []struct {
		file    string
		events  int
		hasEnd  bool
		sapi    string
		hasKids bool
	}{
		{"web-request.jsonl", 6, true, SapiWeb, false},
		{"ajax-child-request.jsonl", 2, true, SapiWeb, true},
		{"cli-exception.jsonl", 4, true, SapiCLI, false},
		{"killed-without-end.jsonl", 2, false, SapiWeb, false},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data := readV2(t, c.file)
			chunk, err := ParseV2Chunk(data, 0)
			if err != nil {
				t.Fatal(err)
			}
			if chunk.Start == nil || chunk.Start.Sapi != c.sapi || chunk.Start.FormatVersion != 2 {
				t.Fatalf("start = %+v", chunk.Start)
			}
			if len(chunk.Events) != c.events || (chunk.End != nil) != c.hasEnd {
				t.Fatalf("events=%d end=%v, want %d/%v", len(chunk.Events), chunk.End != nil, c.events, c.hasEnd)
			}
			if chunk.Consumed != int64(len(data)) || chunk.BrokenLines != 0 || len(chunk.Problems) != 0 {
				t.Fatalf("consumed=%d/%d broken=%d problems=%v", chunk.Consumed, len(data), chunk.BrokenLines, chunk.Problems)
			}
			if (chunk.Start.ParentKey != "") != c.hasKids {
				t.Fatalf("parent key = %q", chunk.Start.ParentKey)
			}
			if lineCount(data) != c.events+1+map[bool]int{true: 1}[c.hasEnd] {
				t.Fatal("fixture line count does not match the expectation")
			}
		})
	}
}

func TestParseV2MapsRequestFields(t *testing.T) {
	chunk, err := ParseV2Chunk(readV2(t, "web-request.jsonl"), 0)
	if err != nil {
		t.Fatal(err)
	}
	r := chunk.Start
	if r.Key != "01JAZ7Q5X0K4M8T2V6N9P3R5S7" || r.Method != "GET" || r.URL != "https://shop.example.test/catalog/item?id=42" ||
		r.Lang != "php" || r.LibraryVersion != "2.0.0" || r.PHPVersion != "7.4.33" || r.PID == nil || *r.PID != 4182 ||
		r.ProcessUser != "www-data" || r.AppUser != "manager@example.test" || r.StartedAt != 1792400000000 {
		t.Fatalf("start = %+v", r)
	}
	end := chunk.End
	if end.StatusCode == nil || *end.StatusCode != 200 || end.DurationMs == nil || *end.DurationMs != 40.2 ||
		end.MemoryPeakBytes == nil || *end.MemoryPeakBytes != 2097152 || end.EndedAt != 1792400000040 {
		t.Fatalf("end = %+v", end)
	}
}

func TestParseV2MapsEvents(t *testing.T) {
	chunk, err := ParseV2Chunk(readV2(t, "cli-exception.jsonl"), 0)
	if err != nil {
		t.Fatal(err)
	}
	timer, exception, fatal := chunk.Events[1], chunk.Events[2], chunk.Events[3]
	if timer.Type != TypeTimer || timer.Message != "send_batch" || timer.DurationMs == nil || *timer.DurationMs != 1890.5 || timer.Level != "warning" {
		t.Fatalf("timer = %+v", timer)
	}
	if exception.Type != TypeError || exception.ErrorKind != "exception" || exception.ExceptionClass != `App\Mail\TransportException` ||
		exception.Code != "111" || len(exception.Trace) != 2 || exception.Source.CallType != "->" || !strings.Contains(string(exception.Previous), "ErrorException") {
		t.Fatalf("exception = %+v", exception)
	}
	if fatal.ErrorKind != "fatal" || fatal.ErrorType != "E_ERROR" || fatal.Seq != 5 {
		t.Fatalf("fatal = %+v", fatal)
	}
	if end := chunk.End; end.ExitCode == nil || *end.ExitCode != 255 || end.StatusCode != nil {
		t.Fatalf("cli end = %+v", end)
	}

	web, _ := ParseV2Chunk(readV2(t, "web-request.jsonl"), 0)
	message := web.Events[1]
	if message.Seq != 3 || message.Level != "info" || !strings.Contains(string(message.Context), `"api_key":"***"`) || message.Size == 0 {
		t.Fatalf("message = %+v", message)
	}
	if sql := web.Events[3]; len(sql.Tags) != 1 || sql.Tags[0] != "sql" || sql.Level != "debug" {
		t.Fatalf("sql message = %+v", sql)
	}
}

// The writer may be in the middle of the last line: it is not consumed and is read again next time.
func TestParseV2LeavesIncompleteLastLine(t *testing.T) {
	data := readV2(t, "web-request.jsonl")
	cut := data[:len(data)-10]
	chunk, err := ParseV2Chunk(cut, 0)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.End != nil || chunk.BrokenLines != 0 || chunk.Consumed != int64(bytes.LastIndexByte(cut, '\n')+1) {
		t.Fatalf("consumed=%d end=%v broken=%d", chunk.Consumed, chunk.End, chunk.BrokenLines)
	}
}

func TestParseV2ContinuesFromOffset(t *testing.T) {
	data := readV2(t, "web-request.jsonl")
	split := bytes.IndexByte(data, '\n') + 1
	split += bytes.IndexByte(data[split:], '\n') + 1 // request_start and the first message

	first, err := ParseV2Chunk(data[:split], 0)
	if err != nil || first.Start == nil || len(first.Events) != 1 || first.LastSeq != 2 {
		t.Fatalf("first = %+v, %v", first, err)
	}
	rest, err := ParseV2Chunk(data[split:], first.LastSeq)
	if err != nil || rest.Start != nil || len(rest.Events) != 5 || rest.End == nil || len(rest.Problems) != 0 {
		t.Fatalf("rest = %+v, %v", rest, err)
	}
	if rest.Events[0].Seq != 3 {
		t.Fatalf("continuation must keep seq order, got %d", rest.Events[0].Seq)
	}
}

func TestParseV2ReportsBrokenStreams(t *testing.T) {
	cases := map[string]string{
		"seq-gap.jsonl":                      "seq",
		"two-request-starts.jsonl":           "request_start",
		"events-after-end.jsonl":             "after request_end",
		"mixed-request-ids.jsonl":            "request_id",
		"time-goes-backwards.jsonl":          "ts",
		"starts-without-request-start.jsonl": "request_start",
	}
	for file, problem := range cases {
		t.Run(file, func(t *testing.T) {
			chunk, err := ParseV2Chunk(readV2(t, "invalid-streams/"+file), 0)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(chunk.Problems, "; "), problem) {
				t.Fatalf("problems = %v, want one about %q", chunk.Problems, problem)
			}
		})
	}
}

func TestParseV2SkipsGarbageLinesAndUnknownFields(t *testing.T) {
	data := readV2(t, "killed-without-end.jsonl")
	lines := bytes.SplitAfter(data, []byte("\n"))
	extra := bytes.Replace(lines[2], []byte(`"type":"message"`), []byte(`"type":"message","from_the_future":true`), 1)
	mixed := bytes.Join([][]byte{lines[0], []byte("{not json\n"), lines[1], extra}, nil)

	chunk, err := ParseV2Chunk(mixed, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The reader is tolerant: unknown fields are fine, a garbage line is counted and skipped
	if chunk.BrokenLines != 1 || len(chunk.Events) != 2 {
		t.Fatalf("broken=%d events=%d problems=%v", chunk.BrokenLines, len(chunk.Events), chunk.Problems)
	}
}

func TestDetectFormat(t *testing.T) {
	v1, _ := os.ReadFile(filepath.Join(fixturesDir(t, "v1"), "web-get.log"))
	cases := []struct {
		name string
		data []byte
		want int
	}{
		{"v2 by content", readV2(t, "web-request.jsonl"), 2},
		{"v1 by content", v1, 1},
		{"too short to tell", []byte(`{"v":`), 0},
	}
	for _, c := range cases {
		if got := DetectFormat(c.data); got != c.want {
			t.Errorf("%s: DetectFormat = %d, want %d", c.name, got, c.want)
		}
	}
}
