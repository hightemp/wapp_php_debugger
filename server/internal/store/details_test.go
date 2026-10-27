package store

import (
	"context"
	"strings"
	"testing"
)

// importV2 saves whole v2 fixtures; returns request ids by fixture name.
func importV2(t *testing.T, s *Store, projectID int64, names ...string) map[string]int64 {
	t.Helper()
	ids := map[string]int64{}
	for _, name := range names {
		chunk := chunkOf(t, v2Fixture(t, name), 0)
		saved, err := s.SaveV2Chunk(context.Background(), projectID, FileState{Name: name, Size: chunk.Consumed}, 0, chunk)
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = saved.RequestID
	}
	return ids
}

func TestGetRequestDetailWithChildren(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	project, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	ids := importV2(t, s, project.ProjectID, "web-request.jsonl", "ajax-child-request.jsonl", "cli-exception.jsonl")

	detail, found, err := s.GetRequestDetail(ctx, ids["web-request.jsonl"])
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if detail.Lang != "php" || detail.PHPVersion != "7.4.33" || detail.PID == nil || *detail.PID != 4182 || detail.Hostname != "web-1" ||
		detail.AppUser != "manager@example.test" || detail.EndedAt == nil || detail.MemoryPeakBytes == nil {
		t.Fatalf("detail = %+v", detail)
	}
	if len(detail.Children) != 1 || detail.Children[0].ID != ids["ajax-child-request.jsonl"] {
		t.Fatalf("children = %+v: the AJAX request points to this one via parent_request_id", detail.Children)
	}

	cli, _, _ := s.GetRequestDetail(ctx, ids["cli-exception.jsonl"])
	if strings.Join(cli.Argv, " ") != "bin/console queue:work --queue=emails" || len(cli.Children) != 0 {
		t.Fatalf("cli detail = %+v", cli)
	}
	if _, found, _ := s.GetRequestDetail(ctx, 999); found {
		t.Fatal("missing request")
	}
}

func TestListMessagesInSeqOrderWithFilters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	project, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	id := importV2(t, s, project.ProjectID, "web-request.jsonl")["web-request.jsonl"]

	seqs := func(filter MessageFilter) []int {
		filter.RequestID = id
		if filter.Limit == 0 {
			filter.Limit = 100
		}
		page, err := s.ListMessages(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		var result []int
		for _, m := range page.Items {
			result = append(result, m.Seq)
		}
		return result
	}
	if got := seqs(MessageFilter{}); len(got) != 6 || got[0] != 2 || got[5] != 7 {
		t.Fatalf("all = %v", got)
	}
	if got := seqs(MessageFilter{Levels: []string{"warning"}}); len(got) != 2 {
		t.Fatalf("warnings = %v", got)
	}
	if got := seqs(MessageFilter{Type: "timer"}); len(got) != 1 || got[0] != 4 {
		t.Fatalf("timers = %v", got)
	}
	if got := seqs(MessageFilter{Tag: "sql"}); len(got) != 1 || got[0] != 5 {
		t.Fatalf("sql = %v", got)
	}

	var all []int
	cursor := ""
	for range 10 {
		page, err := s.ListMessages(ctx, MessageFilter{RequestID: id, Limit: 4, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Items {
			all = append(all, m.Seq)
		}
		if cursor = page.NextCursor; cursor == "" {
			break
		}
	}
	if len(all) != 6 || all[4] != 6 {
		t.Fatalf("paged = %v", all)
	}
	if _, err := s.ListMessages(ctx, MessageFilter{RequestID: id, Limit: 4, Cursor: "garbage"}); err == nil {
		t.Fatal("garbage cursor must be an error")
	}
}

func TestGetMessageDetail(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	project, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	id := importV2(t, s, project.ProjectID, "cli-exception.jsonl")["cli-exception.jsonl"]
	page, _ := s.ListMessages(ctx, MessageFilter{RequestID: id, Type: "error", Limit: 10})
	exception := page.Items[0]

	detail, found, err := s.GetMessage(ctx, exception.ID)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if detail.ExceptionClass != `App\Mail\TransportException` || detail.Code != "111" || len(detail.Trace) != 2 ||
		detail.Source.File != "app/Mail/SmtpTransport.php" || detail.Source.CallType != "->" ||
		!strings.Contains(string(detail.Previous), "ErrorException") || detail.ProjectID != project.ProjectID {
		t.Fatalf("detail = %+v", detail)
	}
	if !exception.HasContext == (detail.Context != nil) {
		t.Fatalf("has_context %v vs context %s", exception.HasContext, detail.Context)
	}
}

func TestProjectForLinks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	project, _ := s.EnsureLocalProject(ctx, "shop", "/var/www/shop", "/var/www/shop/tmp/logs")
	links, found, err := s.ProjectLinks(ctx, project.ProjectID)
	if err != nil || !found || links.Editor != "vscode" || links.RootPath != "/var/www/shop" || links.LocalPath != "" {
		t.Fatalf("links = %+v, %v, %v", links, found, err)
	}
}

func TestProjectLinksOnAnSSHServerCarryTheHost(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for config, want := range map[string]string{
		`{"host":"stage-web","user":"deploy"}`: "deploy@stage-web",
		`{"host":"stage-web"}`:                 "stage-web",
	} {
		server, err := s.CreateServer(ctx, ServerInput{Name: want, Kind: "ssh", Config: config})
		if err != nil {
			t.Fatal(err)
		}
		project, err := s.CreateProject(ctx, ProjectInput{ServerID: server.ID, Name: want, RootPath: "/var/www/shop", LogsDir: "/var/www/shop/logs"})
		if err != nil {
			t.Fatal(err)
		}
		links, _, err := s.ProjectLinks(ctx, project.ID)
		if err != nil || links.RemoteHost != want {
			t.Fatalf("links = %+v, %v; want host %q", links, err, want)
		}
	}
	local, _ := s.EnsureLocalProject(ctx, "local", "/srv/app", "/srv/app/logs")
	if links, _, _ := s.ProjectLinks(ctx, local.ProjectID); links.RemoteHost != "" {
		t.Fatalf("local project has a remote host: %+v", links)
	}
}

// The feed names a CLI run by its command line: the row carries argv, a web request has none
func TestRequestRowsCarryTheCommandLine(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	project, _ := s.EnsureLocalProject(ctx, "shop", "/r", "/r/logs")
	ids := importV2(t, s, project.ProjectID, "web-request.jsonl", "cli-exception.jsonl")

	page, err := s.ListRequests(ctx, RequestFilter{ProjectID: project.ProjectID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	argv := map[int64][]string{}
	for _, row := range page.Items {
		argv[row.ID] = row.Argv
	}
	if got := strings.Join(argv[ids["cli-exception.jsonl"]], " "); got != "bin/console queue:work --queue=emails" {
		t.Fatalf("cli argv = %q", got)
	}
	if argv[ids["web-request.jsonl"]] != nil {
		t.Fatalf("web argv = %q", argv[ids["web-request.jsonl"]])
	}
	row, _, _ := s.GetRequest(ctx, ids["cli-exception.jsonl"])
	if len(row.Argv) != 3 {
		t.Fatalf("GetRequest argv = %q", row.Argv)
	}
}
