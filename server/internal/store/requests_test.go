package store

import (
	"context"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(openTestDB(t))
}

func parsedRequest(key string, startedAt int64, events ...logformat.Event) logformat.Parsed {
	return logformat.Parsed{
		Request: logformat.Request{Key: key, FormatVersion: 1, Sapi: logformat.SapiWeb, Lang: "php", URL: "https://shop.example.test/" + key, StartedAt: startedAt, Finished: true},
		Events:  events,
	}
}

func event(seq int, level string) logformat.Event {
	return logformat.Event{Seq: seq, TS: int64(seq), Type: logformat.TypeMessage, Level: level, Message: "m", Context: []byte(`{"a":1}`), Size: 10}
}

func TestEnsureLocalProjectIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	first, err := s.EnsureLocalProject(ctx, "shop", "/var/www/shop", "/var/www/shop/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.EnsureLocalProject(ctx, "shop", "/var/www/shop", "/var/www/shop/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.ServerID == 0 || first.ProjectID == 0 {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestSaveFileCreatesRequestWithCounters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")

	file := FileState{Name: "1792400000000", Size: 100, ModTime: time.UnixMilli(5)}
	saved, err := s.SaveFile(ctx, ids.ProjectID, file, parsedRequest("1792400000000", 1792400000000, event(1, "info"), event(2, "error"), event(3, "warning"), event(4, "error")))
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Created || saved.RequestID == 0 {
		t.Fatalf("saved = %+v", saved)
	}

	page, err := s.ListRequests(ctx, RequestFilter{ProjectID: ids.ProjectID, Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListRequests = %+v, %v", page, err)
	}
	r := page.Items[0]
	if r.MessageCount != 4 || r.ErrorCount != 2 || r.WarningCount != 1 || r.ServerID != ids.ServerID || r.URL != "https://shop.example.test/1792400000000" {
		t.Fatalf("request = %+v", r)
	}
}

// v1 перезаписывает заголовок и дописывает строки, поэтому файл перечитывается целиком,
// а сообщения запроса заменяются, а не дублируются.
func TestSaveFileAgainReplacesMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	file := FileState{Name: "1792400000000", Size: 100, ModTime: time.UnixMilli(5)}

	first, _ := s.SaveFile(ctx, ids.ProjectID, file, parsedRequest("1792400000000", 1, event(1, "info")))
	file.Size = 200
	second, err := s.SaveFile(ctx, ids.ProjectID, file, parsedRequest("1792400000000", 1, event(1, "info"), event(2, "error")))
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.RequestID != first.RequestID {
		t.Fatalf("second save must update the same request: %+v vs %+v", second, first)
	}
	if n := count(t, s.db, "messages"); n != 2 {
		t.Fatalf("messages = %d, want 2 (replaced, not appended)", n)
	}
}

func TestFileStateDetectsChanges(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	file := FileState{Name: "1792400000000", Size: 100, ModTime: time.UnixMilli(5)}

	if _, found, err := s.FileState(ctx, ids.ProjectID, file.Name); err != nil || found {
		t.Fatalf("unknown file: found=%v err=%v", found, err)
	}
	if _, err := s.SaveFile(ctx, ids.ProjectID, file, parsedRequest("k", 1)); err != nil {
		t.Fatal(err)
	}
	state, found, err := s.FileState(ctx, ids.ProjectID, file.Name)
	if err != nil || !found || state.Size != 100 || !state.ModTime.Equal(file.ModTime) {
		t.Fatalf("state = %+v found=%v err=%v", state, found, err)
	}
}

func TestListRequestsNewestFirstWithCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	for i, started := range []int64{300, 100, 200, 200} {
		name := []string{"a", "b", "c", "d"}[i]
		if _, err := s.SaveFile(ctx, ids.ProjectID, FileState{Name: name, Size: 1}, parsedRequest(name, started)); err != nil {
			t.Fatal(err)
		}
	}

	var keys []string
	cursor := ""
	for range 10 {
		page, err := s.ListRequests(ctx, RequestFilter{ProjectID: ids.ProjectID, Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page.Items {
			keys = append(keys, r.Key)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	// Одинаковое started_at упорядочивается по id, чтобы страницы не теряли и не повторяли запросы
	if got := len(keys); got != 4 || keys[0] != "a" || keys[3] != "b" || keys[1] == keys[2] {
		t.Fatalf("keys = %v", keys)
	}
}

func TestListRequestsRejectsGarbageCursor(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ListRequests(context.Background(), RequestFilter{Cursor: "not-a-cursor", Limit: 10}); err == nil {
		t.Fatal("garbage cursor must be an error")
	}
}

func TestGetRequest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	saved, err := s.SaveFile(ctx, ids.ProjectID, FileState{Name: "a", Size: 1}, parsedRequest("a", 5, event(1, "error")))
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := s.GetRequest(ctx, saved.RequestID)
	if err != nil || !found || got.ID != saved.RequestID || got.Key != "a" || got.ErrorCount != 1 || got.ServerID != ids.ServerID {
		t.Fatalf("GetRequest = %+v, %v, %v", got, found, err)
	}
	if _, found, err := s.GetRequest(ctx, saved.RequestID+100); err != nil || found {
		t.Fatalf("missing request: found=%v err=%v", found, err)
	}
}
