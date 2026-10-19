package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// libraryManifest describes spec/fixtures/v2-library: logs written by the real PHP library 2.x
// (make fixtures-v2-library), so the viewer is checked against the writer, not only the spec.
type libraryManifest struct {
	Files map[string]struct {
		RequestID       string   `json:"request_id"`
		ParentRequestID string   `json:"parent_request_id"`
		Types           []string `json:"types"`
	} `json:"files"`
}

func libraryFixtures(t *testing.T) (string, libraryManifest) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "../../../spec/fixtures/v2-library")
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest libraryManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) < 6 {
		t.Fatalf("manifest lists %d files, want every scenario", len(manifest.Files))
	}
	return dir, manifest
}

func TestImportsLogsOfTheRealLibrary(t *testing.T) {
	dir, manifest := libraryFixtures(t)
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/var/www/app", "/var/www/app/tmp/logs")

	for name, entry := range manifest.Files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		chunk := chunkOf(t, data, 0)
		if chunk.BrokenLines != 0 || len(chunk.Problems) != 0 || chunk.Consumed != int64(len(data)) {
			t.Fatalf("%s: broken %d, problems %v, consumed %d of %d", name, chunk.BrokenLines, chunk.Problems, chunk.Consumed, len(data))
		}
		total := 1 + len(chunk.Events) // request_start and the events after it
		if chunk.End != nil {
			total++
		}
		if total != len(entry.Types) {
			t.Fatalf("%s: %d events, manifest lists %d", name, total, len(entry.Types))
		}
		if _, err := s.SaveV2Chunk(ctx, ids.ProjectID, FileState{Name: name, Size: int64(len(data)), ModTime: time.UnixMilli(1)}, 0, chunk); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	page, err := s.ListRequests(ctx, RequestFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]RequestRow{}
	for _, row := range page.Items {
		byKey[row.Key] = row
	}
	request := func(name string) RequestRow {
		t.Helper()
		row, found := byKey[manifest.Files[name].RequestID]
		if !found {
			t.Fatalf("%s was not imported", name)
		}
		return row
	}
	children := func(name string) []string {
		t.Helper()
		detail, _, err := s.GetRequestDetail(ctx, request(name).ID)
		if err != nil {
			t.Fatal(err)
		}
		var keys []string
		for _, child := range detail.Children {
			keys = append(keys, child.Key)
		}
		return keys
	}

	page200, ajax := request("web-page.jsonl"), request("web-ajax.jsonl")
	if page200.Method != "GET" || page200.StatusCode == nil || *page200.StatusCode != 200 || page200.WarningCount == 0 || !page200.Finished {
		t.Errorf("web page: %+v", page200)
	}
	if !ajax.IsAjax || ajax.StatusCode == nil || *ajax.StatusCode != 201 || ajax.ParentKey != page200.Key {
		t.Errorf("ajax: %+v", ajax)
	}
	if got := children("web-page.jsonl"); len(got) != 1 || got[0] != ajax.Key {
		t.Errorf("children of the page: %v", got)
	}
	if got := children("cli-parent.jsonl"); len(got) != 1 || got[0] != request("cli-child.jsonl").Key {
		t.Errorf("children of the CLI parent: %v", got)
	}
	for _, name := range []string{"cli-exception.jsonl", "cli-fatal.jsonl"} {
		row := request(name)
		if row.Sapi != "cli" || row.ExitCode == nil || *row.ExitCode != 255 || row.ErrorCount == 0 {
			t.Errorf("%s: %+v", name, row)
		}
	}
}
