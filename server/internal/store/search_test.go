package store

import (
	"context"
	"strings"
	"testing"
)

func searchKeys(t *testing.T, s *Store, filter RequestFilter) []string {
	t.Helper()
	filter.Limit = 100
	page, err := s.ListRequests(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, r := range page.Items {
		keys = append(keys, r.Key)
	}
	return keys
}

func searchEnv(t *testing.T) (*Store, LocalProject) {
	t.Helper()
	s := newTestStore(t)
	project, _ := s.EnsureLocalProject(context.Background(), "shop", "/r", "/r/logs")
	importV2(t, s, project.ProjectID, "web-request.jsonl", "ajax-child-request.jsonl", "cli-exception.jsonl", "killed-without-end.jsonl")
	return s, project
}

const (
	webKey    = "01JAZ7Q5X0K4M8T2V6N9P3R5S7"
	ajaxKey   = "01JAZ7Q5X0K4M8T2V6N9P3R5S8"
	cliKey    = "01JAZ7R2B3C4D5E6F7G8H9J0K1"
	killedKey = "01JAZ7S0A1B2C3D4E5F6G7H8J9"
)

func TestSearchRequestsByMessagesContextAndURL(t *testing.T) {
	s, _ := searchEnv(t)
	cases := map[string][]string{
		"кеша":            {webKey},    // message text, Cyrillic
		"кеш":             {webKey},    // prefix
		"Connection":      {cliKey},    // exception message
		"c-981":           {ajaxKey},   // context value
		"report/export":   {killedKey}, // URL substring
		"SELECT * FROM":   {webKey},    // FTS syntax characters are just text
		`"unbalanced`:     {},          // a stray quote does not break the query
		"items Медленный": {webKey},    // every word must match
		"nothing-like-it": {},
	}
	for q, want := range cases {
		t.Run(q, func(t *testing.T) {
			got := searchKeys(t, s, RequestFilter{Query: q})
			if len(got) != len(want) || (len(want) > 0 && got[0] != want[0]) {
				t.Fatalf("q=%q: %v, want %v", q, got, want)
			}
		})
	}
}

func TestFilterRequests(t *testing.T) {
	s, project := searchEnv(t)
	if got := searchKeys(t, s, RequestFilter{HasErrors: true}); len(got) != 1 || got[0] != cliKey {
		t.Fatalf("has errors: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{Sapi: "cli"}); len(got) != 1 || got[0] != cliKey {
		t.Fatalf("cli: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{Since: 1792400100000}); len(got) != 2 {
		t.Fatalf("since: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{Until: 1792400100000}); len(got) != 2 {
		t.Fatalf("until: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{ServerID: project.ServerID}); len(got) != 4 {
		t.Fatalf("server: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{ServerID: project.ServerID + 1}); len(got) != 0 {
		t.Fatalf("other server: %v", got)
	}
	if got := searchKeys(t, s, RequestFilter{Environment: "local"}); len(got) != 4 {
		t.Fatalf("environment: %v", got)
	}
}

func TestSearchMessagesWithinARequest(t *testing.T) {
	s, project := searchEnv(t)
	page, _ := s.ListRequests(context.Background(), RequestFilter{ProjectID: project.ProjectID, Query: "кеша", Limit: 10})
	messages, err := s.ListMessages(context.Background(), MessageFilter{RequestID: page.Items[0].ID, Query: "кеша", Limit: 10})
	if err != nil || len(messages.Items) != 1 || messages.Items[0].Message != "Медленный ответ кеша" {
		t.Fatalf("messages = %+v, %v", messages.Items, err)
	}
}

func TestFTSQueryQuotesEveryWord(t *testing.T) {
	cases := map[string]string{
		"кеш":           `"кеш"*`,
		"SELECT * FROM": `"SELECT"* "FROM"*`,
		`say "hi"`:      `"say"* "hi"*`,
		"a:b c-d":       `"a"* "b"* "c"* "d"*`,
		"   ":           ``,
	}
	for in, want := range cases {
		if got := ftsQuery(in); got != want {
			t.Errorf("ftsQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterRequestsByWhatTheyWere(t *testing.T) {
	s, _ := searchEnv(t)
	yes, no := true, false
	cases := []struct {
		name   string
		filter RequestFilter
		want   []string
	}{
		{"method", RequestFilter{Method: "post"}, []string{ajaxKey}},
		{"status class", RequestFilter{StatusMin: 200, StatusMax: 299}, []string{ajaxKey, webKey}},
		{"exact status", RequestFilter{StatusMin: 201, StatusMax: 201}, []string{ajaxKey}},
		{"slow ones", RequestFilter{MinDurationMs: 1000}, []string{cliKey}},
		{"ajax", RequestFilter{Ajax: &yes}, []string{ajaxKey}},
		{"not ajax", RequestFilter{Ajax: &no, Sapi: "web"}, []string{killedKey, webKey}},
		{"tag", RequestFilter{Tag: "sql"}, []string{webKey}},
		{"warnings or worse", RequestFilter{Level: "warning"}, []string{killedKey, cliKey, webKey}},
		{"errors", RequestFilter{Level: "error"}, []string{cliKey}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := searchKeys(t, s, c.filter)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("%s: %v, want %v", c.name, got, c.want)
			}
		})
	}
}
