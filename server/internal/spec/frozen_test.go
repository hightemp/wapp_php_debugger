package spec

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// Log format v2 is frozen at v1.0 (spec/frozen/README.md): logs written by any library 2.x stay valid
// for the schema and readable for the viewer. The copies in spec/frozen/v1.0 never change; a change of
// the format that refuses one of them is a breaking change.
func TestFrozenV2LogsStayValidAndReadable(t *testing.T) {
	schema := v2Schema(t)
	files := append(globFixtures(t, "frozen/v1.0/fixtures/*.jsonl"), globFixtures(t, "frozen/v1.0/fixtures/library/*.jsonl")...)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data := readFile(t, file)
			for number, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
				value, err := decodeJSON(line)
				if err != nil {
					t.Fatalf("line %d: %v", number+1, err)
				}
				if err := schema.Validate(value); err != nil {
					t.Fatalf("line %d: the current schema refuses a frozen event: %v", number+1, err)
				}
			}
			chunk, err := logformat.ParseV2Chunk(data, 0)
			if err != nil || chunk.BrokenLines > 0 || len(chunk.Problems) > 0 || chunk.Start == nil {
				t.Fatalf("the viewer reads it no more: err=%v broken=%d problems=%v", err, chunk.BrokenLines, chunk.Problems)
			}
		})
	}
}

// The frozen copy of the schema is still the one the frozen logs were checked with
func TestFrozenV2SchemaAcceptsItsLogs(t *testing.T) {
	schema := compileSchema(t, "frozen/v1.0/logformat/event.schema.json")
	for _, file := range globFixtures(t, "frozen/v1.0/fixtures/*.jsonl") {
		for number, line := range bytes.Split(bytes.TrimSuffix(readFile(t, file), []byte("\n")), []byte("\n")) {
			value, err := decodeJSON(line)
			if err == nil {
				err = schema.Validate(value)
			}
			if err != nil {
				t.Fatalf("%s line %d: %v", filepath.Base(file), number+1, err)
			}
		}
	}
}
