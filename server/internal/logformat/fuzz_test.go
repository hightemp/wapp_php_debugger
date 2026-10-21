package logformat

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Fuzz tests: logs come from applications and remote servers, so no input may crash the parser.
// make server-fuzz runs each for a short while; the seeds are the fixtures of spec/fixtures.

func addFixtures(f *testing.F, dir, pattern string) {
	f.Helper()
	paths, err := filepath.Glob(filepath.Join(fixturesDir(f, dir), pattern))
	if err != nil || len(paths) == 0 {
		f.Fatalf("no seeds in %s/%s: %v", dir, pattern, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
}

func FuzzParseV2Chunk(f *testing.F) {
	addFixtures(f, "v2", "*.jsonl")
	addFixtures(f, "v2", "invalid-streams/*.jsonl")
	f.Fuzz(func(t *testing.T, data []byte) {
		chunk, err := ParseV2Chunk(data, 0)
		if err != nil {
			return
		}
		// Only complete lines are taken: the rest is read with the next chunk
		if chunk.Consumed < 0 || chunk.Consumed > int64(len(data)) {
			t.Fatalf("consumed %d of %d bytes", chunk.Consumed, len(data))
		}
		if chunk.Consumed > 0 && data[chunk.Consumed-1] != '\n' {
			t.Fatalf("consumed %d bytes, not up to a line end", chunk.Consumed)
		}
		if bytes.IndexByte(data[chunk.Consumed:], '\n') >= 0 {
			t.Fatalf("a complete line is left after %d bytes", chunk.Consumed)
		}
	})
}

func FuzzParseV1(f *testing.F) {
	addFixtures(f, "v1", "*.log")
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseV1("fuzz.log", data)
	})
}

func FuzzDetectFormat(f *testing.F) {
	addFixtures(f, "v1", "*.log")
	addFixtures(f, "v2", "*.jsonl")
	f.Fuzz(func(t *testing.T, data []byte) {
		if format := DetectFormat(data); format < 0 || format > 2 {
			t.Fatalf("format %d", format)
		}
	})
}
