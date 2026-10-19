// Package sourcetest is the shared contract test suite for source.Source. Every implementation
// must pass it: memory, local directory, SSH, library HTTP mode.
package sourcetest

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
)

// Factory creates a source whose logs directory holds files (name → content), with the debugger off.
type Factory func(t *testing.T, files map[string][]byte) source.Source

// Run checks the source.Source contract.
func Run(t *testing.T, newSource Factory) {
	ctx := context.Background()
	files := map[string][]byte{
		"1792400000000": []byte("first log v1\n"),
		"1792400000500-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl": []byte("{\"v\":2}\n{\"v\":2}\n"),
	}

	open := func(t *testing.T) source.Source {
		t.Helper()
		src := newSource(t, files)
		t.Cleanup(func() { _ = src.Close() })
		return src
	}

	t.Run("List returns files sorted by name with sizes", func(t *testing.T) {
		list, err := open(t).List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || list[0].Name != "1792400000000" || list[1].Name != "1792400000500-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl" {
			t.Fatalf("List = %+v", list)
		}
		if list[0].Size != int64(len(files["1792400000000"])) {
			t.Fatalf("size = %d", list[0].Size)
		}
	})

	t.Run("ReadAt reads from offset up to limit", func(t *testing.T) {
		src := open(t)
		data, err := src.ReadAt(ctx, "1792400000000", 6, 3)
		if err != nil || string(data) != "log" {
			t.Fatalf("ReadAt = %q, %v", data, err)
		}
		data, err = src.ReadAt(ctx, "1792400000000", 6, 1000)
		if err != nil || string(data) != "log v1\n" {
			t.Fatalf("ReadAt to the end = %q, %v", data, err)
		}
	})

	t.Run("ReadAt at the end returns no data", func(t *testing.T) {
		size := int64(len(files["1792400000000"]))
		data, err := open(t).ReadAt(ctx, "1792400000000", size, 10)
		if err != nil || len(data) != 0 {
			t.Fatalf("ReadAt = %q, %v", data, err)
		}
	})

	t.Run("ReadAt beyond the end means the file was truncated", func(t *testing.T) {
		size := int64(len(files["1792400000000"]))
		_, err := open(t).ReadAt(ctx, "1792400000000", size+1, 10)
		if !errors.Is(err, source.ErrOffsetBeyondEnd) {
			t.Fatalf("err = %v, want ErrOffsetBeyondEnd", err)
		}
	})

	t.Run("missing file is ErrNotFound", func(t *testing.T) {
		src := open(t)
		if _, err := src.ReadAt(ctx, "1700000000000", 0, 10); !errors.Is(err, source.ErrNotFound) {
			t.Fatalf("ReadAt err = %v", err)
		}
		if err := src.Remove(ctx, "1700000000000"); !errors.Is(err, source.ErrNotFound) {
			t.Fatalf("Remove err = %v", err)
		}
	})

	t.Run("names outside the logs dir are rejected", func(t *testing.T) {
		src := open(t)
		for _, name := range []string{"", ".", "..", "../secret.txt", "/etc/passwd", "sub/file", `sub\file`} {
			if _, err := src.ReadAt(ctx, name, 0, 10); !errors.Is(err, source.ErrInvalidName) {
				t.Errorf("ReadAt(%q) err = %v, want ErrInvalidName", name, err)
			}
			if err := src.Remove(ctx, name); !errors.Is(err, source.ErrInvalidName) {
				t.Errorf("Remove(%q) err = %v, want ErrInvalidName", name, err)
			}
		}
	})

	t.Run("Remove deletes only the given file", func(t *testing.T) {
		src := open(t)
		if err := src.Remove(ctx, "1792400000000"); err != nil {
			t.Fatal(err)
		}
		list, err := src.List(ctx)
		if err != nil || len(list) != 1 || list[0].Name == "1792400000000" {
			t.Fatalf("List after Remove = %+v, %v", list, err)
		}
	})

	t.Run("SetEnabled switches the debugger", func(t *testing.T) {
		src := open(t)
		for _, want := range []bool{true, false, true} {
			if err := src.SetEnabled(ctx, want); err != nil {
				t.Fatal(err)
			}
			got, err := src.Enabled(ctx)
			if err != nil || got != want {
				t.Fatalf("Enabled = %v, %v; want %v", got, err, want)
			}
		}
	})

	t.Run("ReadAt does not keep a reference to internal buffers", func(t *testing.T) {
		src := open(t)
		data, err := src.ReadAt(ctx, "1792400000000", 0, 5)
		if err != nil {
			t.Fatal(err)
		}
		copy(data, "XXXXX")
		again, err := src.ReadAt(ctx, "1792400000000", 0, 5)
		if err != nil || !bytes.Equal(again, []byte("first")) {
			t.Fatalf("second read = %q, %v", again, err)
		}
	})

	t.Run("canceled context is respected", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := open(t).List(canceled); !errors.Is(err, context.Canceled) {
			t.Fatalf("List err = %v, want context.Canceled", err)
		}
	})
}
