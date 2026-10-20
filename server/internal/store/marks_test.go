package store

import (
	"context"
	"testing"
	"time"
)

func TestRequestsArePinnedAndNotedAndKeptByRetention(t *testing.T) {
	s, project := searchEnv(t)
	ctx := context.Background()
	page, _ := s.ListRequests(ctx, RequestFilter{Limit: 10})
	var web RequestRow
	for _, r := range page.Items {
		if r.Key == webKey {
			web = r
		}
	}
	pinned, note := true, "the checkout bug"
	row, err := s.MarkRequest(ctx, web.ID, &pinned, &note)
	if err != nil || !row.Pinned || row.Note != "the checkout bug" {
		t.Fatalf("mark = %+v, %v", row, err)
	}
	if got := searchKeys(t, s, RequestFilter{Pinned: true}); len(got) != 1 || got[0] != webKey {
		t.Fatalf("pinned feed: %v", got)
	}
	// A re-read of the file keeps the marks
	importV2(t, s, project.ProjectID, "web-request.jsonl")
	if again, _, _ := s.GetRequest(ctx, web.ID); !again.Pinned || again.Note != "the checkout bug" {
		t.Fatalf("after a re-import: %+v", again)
	}

	days := 1
	removed, err := s.PruneRequests(ctx, project.ProjectID, time.UnixMilli(1792400000000).Add(30*24*time.Hour), &days, nil)
	if err != nil || removed != 3 {
		t.Fatalf("retention removed %d, %v; want every request but the pinned one", removed, err)
	}
	if got := searchKeys(t, s, RequestFilter{}); len(got) != 1 || got[0] != webKey {
		t.Fatalf("left: %v", got)
	}

	unpinned, empty := false, ""
	if row, err := s.MarkRequest(ctx, web.ID, &unpinned, &empty); err != nil || row.Pinned || row.Note != "" {
		t.Fatalf("unmark = %+v, %v", row, err)
	}
	if _, err := s.MarkRequest(ctx, 999999, &pinned, nil); err != ErrNotFound {
		t.Fatalf("missing request: %v", err)
	}
}
