package desktop

import (
	"fmt"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
)

// Alert is a system notification about requests with errors; Route opens them in the UI.
type Alert struct {
	Title string
	Body  string
	Route string
}

// AlerterOptions configure an Alerter.
type AlerterOptions struct {
	Settings    func() api.DesktopSettings
	ProjectName func(projectID int64) string
	// Window: after an alert, the next ones within it are summed up in one; 5 s
	Window time.Duration
	Now    func() time.Time
}

// recentMax is how many requests with errors the tray menu lists.
const recentMax = 5

// Alerter turns the event stream into notifications and the tray counter: a request is announced
// once, when it first reaches the level asked for (an error, or a warning too), in the chosen
// projects; a burst is summed up instead of flooding the screen.
type Alerter struct {
	options AlerterOptions

	mu       sync.Mutex
	notified map[int64]severity // the level a request was counted at
	order    []int64            // notified requests, oldest first, to forget old ones
	pending  int                // requests held back within the window
	lastSent time.Time
	unseen   int
	recent   []Alert
}

type severity int

const (
	none severity = iota
	warning
	failure
)

// notifiedMax bounds the memory of announced requests.
const notifiedMax = 10000

// NewAlerter makes an Alerter.
func NewAlerter(options AlerterOptions) *Alerter {
	if options.Window == 0 {
		options.Window = 5 * time.Second
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Alerter{options: options, notified: map[int64]severity{}}
}

// Handle takes an event of the stream and returns the notifications to show now.
func (a *Alerter) Handle(event api.StreamEvent) []Alert {
	if event.Request == nil || (event.Event != api.RequestCreated && event.Event != api.RequestUpdated) {
		return nil
	}
	settings := a.options.Settings()
	r := *event.Request
	level := none
	switch {
	case r.ErrorCount > 0:
		level = failure
	case r.WarningCount > 0 && settings.Notifications.MinLevel == api.DesktopSettingsNotificationsMinLevelWarning:
		level = warning
	}
	if level == none || !chosen(settings, r.ProjectId) {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.notified[r.Id] >= level {
		return nil
	}
	if _, known := a.notified[r.Id]; !known {
		a.order = append(a.order, r.Id)
		if len(a.order) > notifiedMax {
			delete(a.notified, a.order[0])
			a.order = a.order[1:]
		}
		a.unseen++
	}
	a.notified[r.Id] = level

	alert := a.describe(settings, r, level)
	a.recent = append([]Alert{alert}, a.recent...)
	if len(a.recent) > recentMax {
		a.recent = a.recent[:recentMax]
	}
	if !settings.Notifications.Enabled {
		return nil
	}
	now := a.options.Now()
	if now.Sub(a.lastSent) < a.options.Window {
		a.pending++
		return nil
	}
	a.lastSent = now
	return []Alert{alert}
}

// Tick sends the summary of requests held back once the window has passed; call it every second.
func (a *Alerter) Tick() []Alert {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.options.Now()
	if a.pending == 0 || now.Sub(a.lastSent) < a.options.Window {
		return nil
	}
	text := texts(a.options.Settings())
	alert := Alert{Title: fmt.Sprintf(text.more, a.pending), Route: "#/"}
	a.pending = 0
	a.lastSent = now
	return []Alert{alert}
}

// Unseen is the tray counter: requests with errors since the window was last looked at.
func (a *Alerter) Unseen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.unseen
}

// Seen resets the counter: the window got the focus.
func (a *Alerter) Seen() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.unseen = 0
}

// Recent are the last requests with errors, newest first, for the tray menu.
func (a *Alerter) Recent() []Alert {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.recent)
}

func chosen(settings api.DesktopSettings, projectID int64) bool {
	ids := settings.Notifications.ProjectIds
	return len(ids) == 0 || slices.Contains(ids, projectID)
}

func (a *Alerter) describe(settings api.DesktopSettings, r api.Request, level severity) Alert {
	text := texts(settings)
	project := a.options.ProjectName(r.ProjectId)
	title := fmt.Sprintf(text.errors(r.ErrorCount), r.ErrorCount)
	if level == warning {
		title = fmt.Sprintf(text.warnings(r.WarningCount), r.WarningCount)
	}
	if project != "" {
		title = project + ": " + title
	}
	return Alert{Title: title, Body: requestLine(r), Route: fmt.Sprintf("#/projects/%d/requests/%d", r.ProjectId, r.Id)}
}

// requestLine is "GET /path?query" for a web request and "CLI" for a script.
func requestLine(r api.Request) string {
	if r.Sapi == api.Cli || r.Url == nil {
		return "CLI"
	}
	line := *r.Url
	if parsed, err := url.Parse(line); err == nil && parsed.Path != "" {
		line = parsed.RequestURI()
	}
	if r.Method != nil {
		line = *r.Method + " " + line
	}
	return line
}
