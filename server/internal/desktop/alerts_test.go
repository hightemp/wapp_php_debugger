package desktop

import (
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func request(id, project int64, errors, warnings int) api.StreamEvent {
	method, url := "GET", "https://shop.test/item?id=42"
	r := api.Request{Id: id, ProjectId: project, ErrorCount: errors, WarningCount: warnings, Method: &method, Url: &url, Sapi: api.Web}
	return api.StreamEvent{Event: api.RequestCreated, Request: &r}
}

func newAlerter(settings api.DesktopSettings) (*Alerter, *clock) {
	c := &clock{now: time.UnixMilli(1792400000000)}
	names := map[int64]string{1: "shop", 2: "blog"}
	a := NewAlerter(AlerterOptions{
		Settings:    func() api.DesktopSettings { return settings },
		ProjectName: func(id int64) string { return names[id] },
		Window:      5 * time.Second,
		Now:         c.Now,
	})
	return a, c
}

func TestAnErrorIsAnnouncedOnce(t *testing.T) {
	a, _ := newAlerter(DefaultSettings())
	alerts := a.Handle(request(42, 1, 2, 0))
	if len(alerts) != 1 {
		t.Fatalf("alerts = %+v", alerts)
	}
	got := alerts[0]
	if got.Title != "shop: 2 errors" || got.Body != "GET /item?id=42" || got.Route != "#/projects/1/requests/42" {
		t.Fatalf("alert = %+v", got)
	}
	updated := request(42, 1, 3, 0)
	updated.Event = api.RequestUpdated
	if alerts := a.Handle(updated); len(alerts) != 0 {
		t.Fatalf("the same request again: %+v", alerts)
	}
	if a.Unseen() != 1 {
		t.Fatalf("unseen = %d", a.Unseen())
	}
}

func TestWarningsOnlyWhenAskedFor(t *testing.T) {
	a, _ := newAlerter(DefaultSettings())
	if alerts := a.Handle(request(1, 1, 0, 2)); len(alerts) != 0 {
		t.Fatalf("a warning with min_level error: %+v", alerts)
	}
	settings := DefaultSettings()
	settings.Notifications.MinLevel = api.DesktopSettingsNotificationsMinLevelWarning
	b, c := newAlerter(settings)
	alerts := b.Handle(request(1, 1, 0, 2))
	if len(alerts) != 1 || alerts[0].Title != "shop: 2 warnings" {
		t.Fatalf("alerts = %+v", alerts)
	}
	// It turns into an error later: that is news
	c.now = c.now.Add(time.Minute)
	if alerts := b.Handle(request(1, 1, 1, 2)); len(alerts) != 1 {
		t.Fatalf("a warning that became an error: %+v", alerts)
	}
}

func TestOnlyChosenProjects(t *testing.T) {
	settings := DefaultSettings()
	settings.Notifications.ProjectIds = []int64{2}
	a, _ := newAlerter(settings)
	if alerts := a.Handle(request(1, 1, 1, 0)); len(alerts) != 0 || a.Unseen() != 0 {
		t.Fatalf("another project: %+v, unseen %d", alerts, a.Unseen())
	}
	if alerts := a.Handle(request(2, 2, 1, 0)); len(alerts) != 1 {
		t.Fatalf("the chosen one: %+v", alerts)
	}
}

func TestNotificationsOffStillCount(t *testing.T) {
	settings := DefaultSettings()
	settings.Notifications.Enabled = false
	a, _ := newAlerter(settings)
	if alerts := a.Handle(request(1, 1, 1, 0)); len(alerts) != 0 {
		t.Fatalf("alerts while off: %+v", alerts)
	}
	if a.Unseen() != 1 || len(a.Recent()) != 1 {
		t.Fatalf("the tray still counts: %d %+v", a.Unseen(), a.Recent())
	}
	a.Seen()
	if a.Unseen() != 0 {
		t.Fatal("Seen does not reset the counter")
	}
}

func TestABurstBecomesOneSummary(t *testing.T) {
	a, c := newAlerter(DefaultSettings())
	if alerts := a.Handle(request(1, 1, 1, 0)); len(alerts) != 1 {
		t.Fatalf("first: %+v", alerts)
	}
	for id := int64(2); id <= 5; id++ {
		c.now = c.now.Add(time.Second)
		if alerts := a.Handle(request(id, 1+id%2, 1, 0)); len(alerts) != 0 {
			t.Fatalf("within the window: %+v", alerts)
		}
	}
	if alerts := a.Tick(); len(alerts) != 0 {
		t.Fatalf("before the window ends: %+v", alerts)
	}
	c.now = c.now.Add(2 * time.Second)
	alerts := a.Tick()
	if len(alerts) != 1 || alerts[0].Title != "4 more requests with errors" || alerts[0].Route != "#/" {
		t.Fatalf("summary = %+v", alerts)
	}
	if alerts := a.Tick(); len(alerts) != 0 {
		t.Fatalf("summary twice: %+v", alerts)
	}
	if a.Unseen() != 5 {
		t.Fatalf("unseen = %d", a.Unseen())
	}
	recent := a.Recent()
	if len(recent) != 5 || recent[0].Route != "#/projects/2/requests/5" {
		t.Fatalf("recent, newest first: %+v", recent)
	}
}

func TestRecentKeepsTheLastFive(t *testing.T) {
	a, c := newAlerter(DefaultSettings())
	for id := int64(1); id <= 8; id++ {
		c.now = c.now.Add(time.Minute)
		a.Handle(request(id, 1, 1, 0))
	}
	if recent := a.Recent(); len(recent) != 5 || recent[4].Route != "#/projects/1/requests/4" {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestAlertsSpeakTheLanguageOfTheUI(t *testing.T) {
	settings := DefaultSettings()
	ru := api.DesktopSettingsLocaleRu
	settings.Locale = &ru
	a, _ := newAlerter(settings)
	for errors, want := range map[int]string{1: "shop: 1 ошибка", 3: "shop: 3 ошибки", 5: "shop: 5 ошибок", 21: "shop: 21 ошибка", 12: "shop: 12 ошибок"} {
		got := a.Handle(request(int64(100+errors), 1, errors, 0))
		a.lastSent = time.Time{}
		if len(got) != 1 || got[0].Title != want {
			t.Errorf("%d errors: %+v, want %q", errors, got, want)
		}
	}
}
