package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func lines(data []byte) [][]byte {
	return bytes.SplitAfter(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
}

func join(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

// Events pushed in batches build the request as a file read bit by bit would.
func TestSavePushedBatches(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	all := lines(v2Fixture(t, "web-request.jsonl"))

	first, err := s.SavePushed(ctx, ids.ProjectID, join(all[:3]...))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Created) != 1 || first.Rejected != 0 {
		t.Fatalf("first batch: %+v", first)
	}
	// A batch sent again (a retry) adds nothing; the rest finishes the request
	if _, err := s.SavePushed(ctx, ids.ProjectID, join(all[:3]...)); err != nil {
		t.Fatal(err)
	}
	rest, err := s.SavePushed(ctx, ids.ProjectID, join(all[2:]...))
	if err != nil {
		t.Fatal(err)
	}
	if len(rest.Updated) != 1 || rest.Updated[0] != first.Created[0] {
		t.Fatalf("rest: %+v", rest)
	}
	request, _, _ := s.GetRequest(ctx, first.Created[0])
	if !request.Finished || request.MessageCount != len(all)-2 {
		t.Fatalf("request: finished %v, %d messages of %d lines", request.Finished, request.MessageCount, len(all))
	}
	if state, _, _ := s.FileState(ctx, ids.ProjectID, "pushed/"+request.Key+".jsonl"); state.Error != "" {
		t.Fatalf("a retry is not a problem: %q", state.Error)
	}
}

func TestSavePushedSeveralRequestsAndBadLines(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	batch := join(v2Fixture(t, "web-request.jsonl"), []byte("not json\n"), []byte(`{"v":2,"type":"message","request_id":"../x","seq":1,"ts":1}`+"\n"), v2Fixture(t, "cli-exception.jsonl"))
	result, err := s.SavePushed(ctx, ids.ProjectID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 2 || result.Rejected != 2 {
		t.Fatalf("result: %+v", result)
	}
}

func TestIngestTokens(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	sum := sha256.Sum256([]byte("secret"))
	hash := hex.EncodeToString(sum[:])
	if err := s.SetIngestToken(ctx, ids.ProjectID, &hash); err != nil {
		t.Fatal(err)
	}
	if id, found, err := s.ProjectByIngestToken(ctx, hash); err != nil || !found || id != ids.ProjectID {
		t.Fatalf("by token: %d %v %v", id, found, err)
	}
	if project, _, _ := s.GetProject(ctx, ids.ProjectID); !project.IngestEnabled {
		t.Fatal("the project does not say it takes pushed logs")
	}
	if err := s.SetIngestToken(ctx, ids.ProjectID, nil); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.ProjectByIngestToken(ctx, hash); found {
		t.Fatal("a removed token still works")
	}
	if err := s.SetIngestToken(ctx, 999, &hash); err != ErrNotFound {
		t.Fatalf("no such project: %v", err)
	}
}
