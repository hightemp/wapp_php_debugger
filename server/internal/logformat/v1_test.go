package logformat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixturesDir(t *testing.T, version string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "../../../spec/fixtures", version)
}

type manifestEntry struct {
	File           string         `json:"file"`
	Finished       bool           `json:"finished"`
	IsCli          bool           `json:"is_cli"`
	IsAjax         bool           `json:"is_ajax"`
	URL            string         `json:"url"`
	MessagesValid  int            `json:"messages_valid"`
	MessagesBroken int            `json:"messages_broken"`
	Levels         map[string]int `json:"levels"`
}

func loadV1(t *testing.T, name string) Parsed {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(t, "v1"), name))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseV1(name, data)
	if err != nil {
		t.Fatalf("ParseV1(%s): %v", name, err)
	}
	return parsed
}

func TestParseV1MatchesManifest(t *testing.T) {
	var manifest struct {
		Fixtures []manifestEntry `json:"fixtures"`
	}
	data, err := os.ReadFile(filepath.Join(fixturesDir(t, "v1"), "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	for _, want := range manifest.Fixtures {
		t.Run(want.File, func(t *testing.T) {
			got := loadV1(t, want.File)
			if len(got.Events) != want.MessagesValid || got.BrokenLines != want.MessagesBroken {
				t.Fatalf("events/broken = %d/%d, want %d/%d", len(got.Events), got.BrokenLines, want.MessagesValid, want.MessagesBroken)
			}
			r := got.Request
			if r.Finished != want.Finished || (r.Sapi == SapiCLI) != want.IsCli || r.IsAjax != want.IsAjax || r.URL != want.URL {
				t.Fatalf("request = %+v, manifest = %+v", r, want)
			}
			levels := map[string]int{}
			for _, e := range got.Events {
				levels[e.Level]++
			}
			for level, n := range want.Levels {
				if levels[level] != n {
					t.Errorf("level %s: %d events, want %d", level, levels[level], n)
				}
			}
			for i, e := range got.Events {
				if e.Seq != i+1 {
					t.Fatalf("event %d has seq %d: seq must follow file order from 1", i, e.Seq)
				}
			}
		})
	}
}

func TestParseV1Request(t *testing.T) {
	web := loadV1(t, "web-get.log").Request
	if web.FormatVersion != 1 || web.Sapi != SapiWeb || web.StatusCode == nil || *web.StatusCode != 200 {
		t.Fatalf("web request = %+v", web)
	}
	if web.Key == "" || web.StartedAt == 0 || web.EndedAt == nil || web.DurationMs == nil {
		t.Fatalf("web request timing = %+v", web)
	}

	cli := loadV1(t, "cli.log").Request
	if cli.Sapi != SapiCLI || cli.StatusCode != nil || len(cli.Argv) != 4 || cli.Argv[2] != "--queue=emails" {
		t.Fatalf("cli request = %+v", cli)
	}

	killed := loadV1(t, "killed.log").Request
	if killed.Finished || killed.EndedAt != nil || killed.DurationMs != nil || killed.StatusCode != nil {
		t.Fatalf("killed request must have no end data: %+v", killed)
	}
}

func TestParseV1Events(t *testing.T) {
	events := loadV1(t, "web-get.log").Events

	first := events[0]
	if first.Type != TypeMessage || first.Level != "message" || first.Message != "$_SERVER" || len(first.Context) == 0 {
		t.Fatalf("first event = %+v", first)
	}

	var timer, inline, scenario *Event
	for i := range events {
		switch e := &events[i]; {
		case e.Type == TypeTimer:
			timer = e
		case e.Message == "#inlinelog #string inline-value":
			inline = e
		case e.Message == "Сценарий web-get":
			scenario = e
		}
	}
	if timer == nil || timer.Message != "db" || timer.DurationMs == nil || !hasTag(timer.Tags, "timediff") {
		t.Fatalf("v1 '#timediff #db N мс' must become a timer named db: %+v", timer)
	}
	if inline == nil || !hasTag(inline.Tags, "inlinelog") || !hasTag(inline.Tags, "string") {
		t.Fatalf("inline log tags: %+v", inline)
	}
	if scenario == nil || scenario.Source == nil || scenario.Source.File != "/scenario.php" || scenario.Source.Line == 0 {
		t.Fatalf("source from oBackTrace: %+v", scenario)
	}
	if len(scenario.Trace) == 0 || scenario.Trace[0].File == "" {
		t.Fatalf("trace from aBackTrace: %+v", scenario.Trace)
	}
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func TestV1MessageText(t *testing.T) {
	cases := map[string]string{`"text"`: "text", `42`: "42", `1.5`: "1.5", `true`: "true", `null`: ""}
	for raw, want := range cases {
		if got := messageText(json.RawMessage(raw)); got != want {
			t.Errorf("messageText(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestV1Level(t *testing.T) {
	cases := map[string]string{"error": "error", "warning": "warning", "info": "info", "message": "message", "functions_debug": "message", "custom": "message"}
	for in, want := range cases {
		if got := v1Level(in); got != want {
			t.Errorf("v1Level(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestV1Tags(t *testing.T) {
	got := extractTags("#timediff #DB 12 мс #кириллица #a.b:c-d")
	want := []string{"timediff", "db", "a.b:c-d"}
	if len(got) != len(want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tags = %v, want %v", got, want)
		}
	}
}

func TestParseV1RejectsNonJSONHeader(t *testing.T) {
	if _, err := ParseV1("1792400000000", []byte("not json\n{}\n")); err == nil {
		t.Fatal("garbage header must be an error")
	}
}

// The v1 request key is the file name: it is unique in the logs directory, while copies of a file share iRequestTimestamp.
func TestParseV1KeyIsFileName(t *testing.T) {
	web, truncated := loadV1(t, "web-get.log").Request, loadV1(t, "truncated-last-line.log").Request
	if web.Key != "web-get.log" || truncated.Key != "truncated-last-line.log" {
		t.Fatalf("keys = %q, %q", web.Key, truncated.Key)
	}
}
