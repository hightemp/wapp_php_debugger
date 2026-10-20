package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/hightemp/wapp_php_debugger/server/internal/thirdparty"
)

func v2Schema(t *testing.T) *jsonschema.Schema {
	return compileSchema(t, "logformat/v2/event.schema.json")
}

func globFixtures(t *testing.T, pattern string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(specDir(t), pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no files match %s", pattern)
	}
	return files
}

type v2Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Seq       int    `json:"seq"`
	TS        int64  `json:"ts"`
}

// checkV2Stream checks the per-request stream rules that JSON Schema cannot express.
func checkV2Stream(data []byte) error {
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	var first v2Event
	var previousTS int64
	for index, line := range lines {
		var event v2Event
		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if index == 0 {
			first = event
			if event.Type != "request_start" {
				return fmt.Errorf("line 1: first event must be request_start, got %q", event.Type)
			}
		}
		if event.RequestID != first.RequestID {
			return fmt.Errorf("line %d: request_id %q differs from %q", index+1, event.RequestID, first.RequestID)
		}
		if event.Seq != index+1 {
			return fmt.Errorf("line %d: seq = %d, want %d", index+1, event.Seq, index+1)
		}
		if index > 0 && event.Type == "request_start" {
			return fmt.Errorf("line %d: request_start repeated", index+1)
		}
		if event.Type == "request_end" && index != len(lines)-1 {
			return fmt.Errorf("line %d: events after request_end", index+1)
		}
		if event.TS < previousTS {
			return fmt.Errorf("line %d: ts goes backwards", index+1)
		}
		previousTS = event.TS
	}
	return nil
}

// Hand-written fixtures and logs of the real library (fixtures/v2-library) obey the same contract.
func TestV2ValidFixtures(t *testing.T) {
	schema := v2Schema(t)
	files := append(globFixtures(t, "fixtures/v2/*.jsonl"), globFixtures(t, "fixtures/v2-library/*.jsonl")...)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data := readFile(t, file)
			if !bytes.HasSuffix(data, []byte("\n")) {
				t.Fatal("file must end with a newline: every event is a complete line")
			}
			for number, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
				value, err := decodeJSON(line)
				if err != nil {
					t.Fatalf("line %d: %v", number+1, err)
				}
				if err := schema.Validate(value); err != nil {
					t.Fatalf("line %d: %v", number+1, err)
				}
			}
			if err := checkV2Stream(data); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Each invalid/*.json file is one event the schema must reject; the file name says why.
func TestV2SchemaRejectsInvalidEvents(t *testing.T) {
	schema := v2Schema(t)
	for _, file := range globFixtures(t, "fixtures/v2/invalid/*.json") {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".json"), func(t *testing.T) {
			value, err := decodeJSON(readFile(t, file))
			if err != nil {
				t.Fatalf("invalid example must still be JSON: %v", err)
			}
			if schema.Validate(value) == nil {
				t.Fatal("schema accepted invalid event")
			}
		})
	}
}

// Each invalid-streams/*.jsonl file consists of valid events but breaks the stream rules.
func TestV2StreamRulesRejectInvalidStreams(t *testing.T) {
	schema := v2Schema(t)
	for _, file := range globFixtures(t, "fixtures/v2/invalid-streams/*.jsonl") {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".jsonl"), func(t *testing.T) {
			data := readFile(t, file)
			for number, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
				value, err := decodeJSON(line)
				if err != nil {
					t.Fatalf("line %d: %v", number+1, err)
				}
				if err := schema.Validate(value); err != nil {
					t.Fatalf("line %d must be a valid event, the stream is what is broken: %v", number+1, err)
				}
			}
			if checkV2Stream(data) == nil {
				t.Fatal("stream rules accepted invalid stream")
			}
		})
	}
}

func TestV2FixturesDirectoryHasNoStrayFiles(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(specDir(t), "fixtures/v2"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && !strings.HasSuffix(name, ".jsonl") && name != "README.md" {
			t.Errorf("unexpected file %s: valid fixtures are *.jsonl", name)
		}
	}
}

// Third-party logs are imported as v2: what the converter writes obeys the same schema.
func TestThirdPartyLogsBecomeValidV2(t *testing.T) {
	schema := v2Schema(t)
	for _, file := range globFixtures(t, "fixtures/thirdparty/*.log") {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, ok, err := thirdparty.Convert(filepath.Base(file), readFile(t, file))
			if err != nil || !ok {
				t.Fatalf("ok %v, %v", ok, err)
			}
			for number, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
				value, err := decodeJSON(line)
				if err != nil {
					t.Fatalf("line %d: %v", number+1, err)
				}
				if err := schema.Validate(value); err != nil {
					t.Fatalf("line %d: %v\n%s", number+1, err, line)
				}
			}
			if err := checkV2Stream(data); err != nil {
				t.Fatal(err)
			}
		})
	}
}
