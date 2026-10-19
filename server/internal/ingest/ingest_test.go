package ingest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func v1Fixtures(t *testing.T) map[string][]byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/v1/*.log"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	files := map[string][]byte{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.Base(path)] = data
	}
	return files
}

type env struct {
	importer  *Importer
	store     *store.Store
	projectID int64
}

func newEnv(t *testing.T) env {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	ids, err := s.EnsureLocalProject(context.Background(), "shop", "/var/www/app", "/var/www/app/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	return env{importer: New(s), store: s, projectID: ids.ProjectID}
}

func (e env) requestCount(t *testing.T) int {
	t.Helper()
	page, err := e.store.ListRequests(context.Background(), store.RequestFilter{ProjectID: e.projectID, Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Items)
}

func TestScanImportsEveryV1File(t *testing.T) {
	e := newEnv(t)
	files := v1Fixtures(t)
	result, err := e.importer.Scan(context.Background(), e.projectID, source.NewMemory(files))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != len(files) || len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v", result)
	}
	if n := e.requestCount(t); n != len(files) {
		t.Fatalf("requests in store = %d, want %d", n, len(files))
	}
}

func TestScanSkipsUnchangedAndRereadsGrownFiles(t *testing.T) {
	e := newEnv(t)
	src := source.NewMemory(v1Fixtures(t))
	ctx := context.Background()
	if _, err := e.importer.Scan(ctx, e.projectID, src); err != nil {
		t.Fatal(err)
	}

	again, err := e.importer.Scan(ctx, e.projectID, src)
	if err != nil || len(again.Created)+len(again.Updated) != 0 || again.Unchanged != 7 {
		t.Fatalf("second scan = %+v, %v; unchanged files must not be re-read", again, err)
	}

	src.Append("killed.log", []byte(`{"sType":"info","sMessage":"дописано","oVars":{},"iTimestamp":1792419999999}`+"\n"))
	grown, err := e.importer.Scan(ctx, e.projectID, src)
	if err != nil || len(grown.Updated) != 1 || len(grown.Created) != 0 {
		t.Fatalf("scan after append = %+v, %v", grown, err)
	}
}

func TestScanReportsBrokenFilesAndKeepsGoing(t *testing.T) {
	e := newEnv(t)
	files := v1Fixtures(t)
	files["1792400000000"] = []byte("garbage header\n")
	result, err := e.importer.Scan(context.Background(), e.projectID, source.NewMemory(files))
	if err != nil {
		t.Fatal(err)
	}
	if _, failed := result.Failed["1792400000000"]; !failed || len(result.Created) != 7 {
		t.Fatalf("result = %+v", result)
	}
}

func v2Fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (e env) onlyRequest(t *testing.T) store.RequestRow {
	t.Helper()
	page, err := e.store.ListRequests(context.Background(), store.RequestFilter{ProjectID: e.projectID, Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("requests = %+v, %v", page.Items, err)
	}
	return page.Items[0]
}

func TestScanImportsV2Files(t *testing.T) {
	e := newEnv(t)
	files := map[string][]byte{}
	for _, name := range []string{"web-request.jsonl", "ajax-child-request.jsonl", "cli-exception.jsonl", "killed-without-end.jsonl"} {
		files["1792400000000-"+name] = v2Fixture(t, name)
	}
	result, err := e.importer.Scan(context.Background(), e.projectID, source.NewMemory(files))
	if err != nil || len(result.Created) != 4 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

// A v2 file grows while the request runs: each scan reads only what was appended.
func TestScanContinuesGrowingV2File(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	data := v2Fixture(t, "web-request.jsonl")
	cut := bytes.IndexByte(data, '\n') + 40
	src := source.NewMemory(map[string][]byte{"w.jsonl": data[:cut]})

	if result, err := e.importer.Scan(ctx, e.projectID, src); err != nil || len(result.Created) != 1 {
		t.Fatalf("first scan = %+v, %v", result, err)
	}
	if r := e.onlyRequest(t); r.MessageCount != 0 || r.Finished {
		t.Fatalf("after the first line only: %+v", r)
	}

	src.Append("w.jsonl", data[cut:])
	if result, err := e.importer.Scan(ctx, e.projectID, src); err != nil || len(result.Updated) != 1 {
		t.Fatalf("second scan = %+v, %v", result, err)
	}
	if r := e.onlyRequest(t); r.MessageCount != 6 || !r.Finished {
		t.Fatalf("after the rest: %+v", r)
	}
}

func TestScanReadsLargeV2FilesInChunks(t *testing.T) {
	e := newEnv(t)
	e.importer.chunkSize = 256 // smaller than one line: the reader must still make progress
	src := source.NewMemory(map[string][]byte{"w.jsonl": v2Fixture(t, "web-request.jsonl")})
	if _, err := e.importer.Scan(context.Background(), e.projectID, src); err != nil {
		t.Fatal(err)
	}
	if r := e.onlyRequest(t); r.MessageCount != 6 || !r.Finished {
		t.Fatalf("request = %+v", r)
	}
}

func TestScanReimportsTruncatedFile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	src := source.NewMemory(map[string][]byte{"w.jsonl": v2Fixture(t, "web-request.jsonl")})
	if _, err := e.importer.Scan(ctx, e.projectID, src); err != nil {
		t.Fatal(err)
	}
	// The file is replaced with a shorter one, e.g. log rotation reused the name
	if err := src.Remove(ctx, "w.jsonl"); err != nil {
		t.Fatal(err)
	}
	src.Append("w.jsonl", v2Fixture(t, "killed-without-end.jsonl"))

	if _, err := e.importer.Scan(ctx, e.projectID, src); err != nil {
		t.Fatal(err)
	}
	if r := e.onlyRequest(t); r.Key != "01JAZ7S0A1B2C3D4E5F6G7H8J9" || r.MessageCount != 2 {
		t.Fatalf("request = %+v", r)
	}
}

func TestScanDetectsFormatByContent(t *testing.T) {
	e := newEnv(t)
	src := source.NewMemory(map[string][]byte{"no-extension": v2Fixture(t, "killed-without-end.jsonl")})
	if _, err := e.importer.Scan(context.Background(), e.projectID, src); err != nil {
		t.Fatal(err)
	}
	if r := e.onlyRequest(t); r.FormatVersion != 2 {
		t.Fatalf("request = %+v", r)
	}
}

func TestScanSkipsBrokenV2StreamsWithoutFailing(t *testing.T) {
	e := newEnv(t)
	src := source.NewMemory(map[string][]byte{"x.jsonl": v2Fixture(t, "invalid-streams/starts-without-request-start.jsonl")})
	result, err := e.importer.Scan(context.Background(), e.projectID, src)
	if err != nil || len(result.Failed) != 0 || len(result.Created) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
	again, _ := e.importer.Scan(context.Background(), e.projectID, src)
	if again.Unchanged != 1 {
		t.Fatalf("a broken file is not re-read until it changes: %+v", again)
	}
}
