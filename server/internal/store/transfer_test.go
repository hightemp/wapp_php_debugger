package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// A request goes out as the v2 stream the library writes and comes back the same.
func TestExportImportRoundTrip(t *testing.T) {
	for _, name := range []string{"web-request.jsonl", "cli-exception.jsonl", "killed-without-end.jsonl", "sql-and-http.jsonl", "ajax-child-request.jsonl"} {
		t.Run(name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
			original := v2Fixture(t, name)

			id, err := s.ImportRequest(ctx, ids.ProjectID, original)
			if err != nil {
				t.Fatal(err)
			}
			exported, key, err := s.ExportRequest(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			want, got := chunkOf(t, original, 0), chunkOf(t, exported, 0)
			if len(got.Problems) > 0 || got.BrokenLines > 0 {
				t.Fatalf("exported stream: %v, broken %d\n%s", got.Problems, got.BrokenLines, exported)
			}
			if key != want.Start.Key {
				t.Fatalf("key %q, want %q", key, want.Start.Key)
			}
			if !reflect.DeepEqual(got.Start, want.Start) || !reflect.DeepEqual(got.End, want.End) {
				t.Fatalf("request differs:\n got %+v %+v\nwant %+v %+v", got.Start, got.End, want.Start, want.End)
			}
			if len(got.Events) != len(want.Events) {
				t.Fatalf("%d events, want %d", len(got.Events), len(want.Events))
			}
			for i := range want.Events {
				w, g := want.Events[i], got.Events[i]
				w.Size, g.Size = 0, 0
				// The database keeps no call_type of the source (the trace has it); the UI does not show it
				if w.Source != nil {
					source := *w.Source
					source.CallType = ""
					w.Source = &source
				}
				if !reflect.DeepEqual(compact(t, g), compact(t, w)) {
					t.Fatalf("event %d:\n got %+v\nwant %+v", i, g, w)
				}
			}
		})
	}
}

// compact makes raw JSON fields comparable whatever their spacing.
func compact(t *testing.T, e logformat.Event) logformat.Event {
	t.Helper()
	for _, field := range []*[]byte{(*[]byte)(&e.Context), (*[]byte)(&e.Previous)} {
		if *field != nil {
			var out bytes.Buffer
			if err := json.Compact(&out, *field); err != nil {
				t.Fatal(err)
			}
			*field = out.Bytes()
		}
	}
	return e
}

func TestImportRequestRejects(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	data := v2Fixture(t, "web-request.jsonl")
	first, err := s.ImportRequest(ctx, ids.ProjectID, data)
	if err != nil {
		t.Fatal(err)
	}

	// The same request again: it is already here
	var exists *ExistsError
	if _, err := s.ImportRequest(ctx, ids.ProjectID, data); !errors.As(err, &exists) || exists.RequestID != first {
		t.Fatalf("import twice: %v", err)
	}
	for name, bad := range map[string][]byte{
		"not v2":       []byte(`{"bFinished":true}` + "\n"),
		"no start":     bytes.SplitAfterN(data, []byte("\n"), 2)[1],
		"empty":        nil,
		"out of order": append(append([]byte{}, data...), data...),
	} {
		if _, err := s.ImportRequest(ctx, ids.ProjectID, bad); err == nil || errors.As(err, &exists) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
