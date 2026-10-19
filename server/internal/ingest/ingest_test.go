package ingest

import (
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

func TestScanLeavesV2FilesForLater(t *testing.T) {
	e := newEnv(t)
	files := map[string][]byte{"1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl": []byte(`{"v":2}` + "\n")}
	result, err := e.importer.Scan(context.Background(), e.projectID, source.NewMemory(files))
	if err != nil || result.Unsupported != 1 || len(result.Failed) != 0 {
		t.Fatalf("v2 parser arrives in phase 5: result = %+v, %v", result, err)
	}
}
