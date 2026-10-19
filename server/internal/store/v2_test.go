package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

func v2Fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func chunkOf(t *testing.T, data []byte, lastSeq int) logformat.V2Chunk {
	t.Helper()
	chunk, err := logformat.ParseV2Chunk(data, lastSeq)
	if err != nil {
		t.Fatal(err)
	}
	return chunk
}

func TestSaveV2ChunksAppendAndFinish(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	data := v2Fixture(t, "web-request.jsonl")
	name := "1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl"
	half := bytes.IndexByte(data, '\n') + 40 // in the middle of the second line

	first := chunkOf(t, data[:half], 0)
	saved, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: name, Size: int64(half), ModTime: time.UnixMilli(1)}, 0, first)
	if err != nil || !saved.Created {
		t.Fatalf("first chunk: %+v, %v", saved, err)
	}
	state, _, _ := s.FileState(ctx, ids.ProjectID, name)
	if state.ReadOffset != first.Consumed || state.FormatVersion != 2 || state.LastSeq != first.LastSeq {
		t.Fatalf("state after first chunk = %+v", state)
	}

	rest := chunkOf(t, data[state.ReadOffset:], state.LastSeq)
	again, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: name, Size: int64(len(data)), ModTime: time.UnixMilli(2)}, state.ReadOffset, rest)
	if err != nil || again.Created || again.RequestID != saved.RequestID {
		t.Fatalf("second chunk: %+v, %v", again, err)
	}

	page, _ := s.ListRequests(ctx, RequestFilter{ProjectID: ids.ProjectID, Limit: 10})
	r := page.Items[0]
	if r.MessageCount != 6 || r.WarningCount != 2 || r.ErrorCount != 0 || !r.Finished || r.StatusCode == nil || *r.StatusCode != 200 ||
		r.FormatVersion != 2 || r.Key != "01JAZ7Q5X0K4M8T2V6N9P3R5S7" || r.Method != "GET" {
		t.Fatalf("request = %+v", r)
	}
	state, _, _ = s.FileState(ctx, ids.ProjectID, name)
	if state.ReadOffset != int64(len(data)) || state.Status != "complete" {
		t.Fatalf("final state = %+v", state)
	}
}

func TestSaveV2ChunkTwiceDoesNotDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	chunk := chunkOf(t, v2Fixture(t, "killed-without-end.jsonl"), 0)
	file := FileState{Name: "k.jsonl", Size: chunk.Consumed}
	for range 2 {
		if _, err := s.SaveV2Chunk(ctx, ids.ProjectID, file, 0, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, s.db, "messages"); n != 2 {
		t.Fatalf("messages = %d, want 2", n)
	}
}

func TestSaveV2WithoutRequestStartMarksFileBroken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	chunk := chunkOf(t, v2Fixture(t, "invalid-streams/starts-without-request-start.jsonl"), 0)
	if _, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: "x.jsonl", Size: chunk.Consumed}, 0, chunk); err != nil {
		t.Fatal(err)
	}
	state, found, _ := s.FileState(ctx, ids.ProjectID, "x.jsonl")
	if !found || state.Status != "broken" || state.Error == "" || state.ReadOffset != chunk.Consumed {
		t.Fatalf("state = %+v: the file must be skipped until it changes", state)
	}
	if n := count(t, s.db, "requests"); n != 0 {
		t.Fatalf("requests = %d", n)
	}
}

func TestForgetFileRemovesItsRequest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	chunk := chunkOf(t, v2Fixture(t, "web-request.jsonl"), 0)
	if _, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: "w.jsonl", Size: chunk.Consumed}, 0, chunk); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetFile(ctx, ids.ProjectID, "w.jsonl"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s.db, "requests") + count(t, s.db, "log_files"); n != 0 {
		t.Fatalf("%d rows left: a truncated file is imported again from scratch", n)
	}
}

// The writer has not finished the first line yet: nothing is complete, nothing is broken.
func TestSaveV2EmptyChunkIsNotBroken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	data := v2Fixture(t, "web-request.jsonl")
	chunk := chunkOf(t, data[:100], 0)
	if _, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: "w.jsonl", Size: 100}, 0, chunk); err != nil {
		t.Fatal(err)
	}
	state, _, _ := s.FileState(ctx, ids.ProjectID, "w.jsonl")
	if state.Status == "broken" || state.ReadOffset != 0 {
		t.Fatalf("state = %+v", state)
	}
}
