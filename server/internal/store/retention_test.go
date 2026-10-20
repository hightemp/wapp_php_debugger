package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

func TestPruneDropsOldAndOverflowingRequests(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	other, _ := s.EnsureLocalProject(ctx, "blog", "/b", "/b/logs")
	now := time.UnixMilli(1792400000000)
	day := int64(24 * time.Hour / time.Millisecond)
	save := func(project int64, name string, startedAt int64, messages int) {
		t.Helper()
		events := make([]logformat.Event, 0, messages)
		for i := 1; i <= messages; i++ {
			events = append(events, event(i, "info")) // 10 bytes each
		}
		file := FileState{Name: name, Size: 100, ModTime: time.UnixMilli(startedAt)}
		if _, err := s.SaveFile(ctx, project, file, parsedRequest(name, startedAt, events...)); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 5 {
		// days ago: 40, 30, 20, 10, 0
		save(ids.ProjectID, fmt.Sprintf("r%d", i), now.UnixMilli()-int64(40-10*i)*day, 3)
	}
	save(other.ProjectID, "old-blog", now.UnixMilli()-100*day, 3)

	days := 25
	removed, err := s.PruneRequests(ctx, ids.ProjectID, now, &days, nil)
	if err != nil || removed != 2 {
		t.Fatalf("by age: removed %d, %v; want 2", removed, err)
	}
	maxBytes := int64(60) // two requests of 30 bytes
	removed, err = s.PruneRequests(ctx, ids.ProjectID, now, nil, &maxBytes)
	if err != nil || removed != 1 {
		t.Fatalf("by size: removed %d, %v; want 1", removed, err)
	}
	page, _ := s.ListRequests(ctx, RequestFilter{ProjectID: ids.ProjectID, Limit: 10})
	if len(page.Items) != 2 || page.Items[0].Key != "r4" || page.Items[1].Key != "r3" {
		t.Fatalf("left %+v; want the newest two", page.Items)
	}
	if blog, _ := s.ListRequests(ctx, RequestFilter{ProjectID: other.ProjectID, Limit: 10}); len(blog.Items) != 1 {
		t.Fatal("another project was pruned")
	}
	// The files stay known: a pruned request is not imported again
	if state, found, _ := s.FileState(ctx, ids.ProjectID, "r0"); !found || state.Size != 100 {
		t.Fatalf("file state of a pruned request: %+v %v", state, found)
	}
	if removed, err := s.PruneRequests(ctx, ids.ProjectID, now, nil, nil); err != nil || removed != 0 {
		t.Fatalf("no limits: removed %d, %v", removed, err)
	}
}
