package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.3.0", "v0.4.0", true},
		{"v0.3.0", "v0.3.1", true},
		{"v0.3.0", "v1.0.0", true},
		{"v0.3.0", "v0.3.0", false},
		{"v0.4.0", "v0.3.9", false},
		{"v0.3.0-2-g82cf816-dirty", "v0.3.0", false}, // a build after the tag
		{"v0.3.0-2-g82cf816", "v0.4.0", true},
		{"v0.10.0", "v0.9.0", false}, // numbers, not strings
		{"dev", "v0.4.0", false},     // a local build does not nag
		{"v0.3.0", "v0.4.0-rc.1", false},
		{"v0.3.0", "garbage", false},
	} {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCheckerAsksGitHubAtMostOnceADay(t *testing.T) {
	var calls atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/repos/hightemp/wapp_php_debugger/releases/latest" || r.Header.Get("Accept") != "application/vnd.github+json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.4.0","html_url":"https://github.com/hightemp/wapp_php_debugger/releases/tag/v0.4.0","draft":false,"prerelease":false}`))
	}))
	t.Cleanup(github.Close)
	now := time.UnixMilli(1792400000000)
	checker := NewChecker(Options{Current: "v0.3.0", API: github.URL, Client: github.Client(), Now: func() time.Time { return now }})

	result, err := checker.Check(context.Background())
	if err != nil || !result.Newer || result.Latest != "v0.4.0" || result.URL == "" || result.Current != "v0.3.0" {
		t.Fatalf("result = %+v, %v", result, err)
	}
	if _, err := checker.Check(context.Background()); err != nil || calls.Load() != 1 {
		t.Fatalf("checked again within a day: %d calls, %v", calls.Load(), err)
	}
	now = now.Add(25 * time.Hour)
	if _, err := checker.Check(context.Background()); err != nil || calls.Load() != 2 {
		t.Fatalf("a day later: %d calls, %v", calls.Load(), err)
	}
	if last := checker.Last(); !last.Newer || last.CheckedAt != now.UnixMilli() {
		t.Fatalf("last = %+v", last)
	}
}

func TestCheckerSurvivesGitHubTrouble(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // rate limited
	}))
	t.Cleanup(github.Close)
	checker := NewChecker(Options{Current: "v0.3.0", API: github.URL, Client: github.Client()})
	if _, err := checker.Check(context.Background()); err == nil {
		t.Fatal("a failed check gave no error")
	}
	if last := checker.Last(); last.Newer || last.Latest != "" {
		t.Fatalf("a failed check left %+v", last)
	}
}
