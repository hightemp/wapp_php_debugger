// Package update tells whether a newer version of the viewer is out, from GitHub Releases (plan
// 10.29: a notice only, no self-update). It asks at most once a day.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Repository the releases come from.
const Repository = "hightemp/wapp_php_debugger"

// Result is the last answer.
type Result struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	URL       string `json:"url,omitempty"`
	Newer     bool   `json:"newer"`
	CheckedAt int64  `json:"checked_at,omitempty"` // ms
}

// Options configure a Checker; zero values take the defaults.
type Options struct {
	Current  string        // the running version
	API      string        // https://api.github.com
	Client   *http.Client  // 15 s timeout
	Interval time.Duration // 24 h
	Now      func() time.Time
}

// Checker asks GitHub for the latest published release.
type Checker struct {
	options Options
	mu      sync.Mutex
	last    Result
	asked   time.Time
}

// NewChecker makes a Checker.
func NewChecker(options Options) *Checker {
	if options.API == "" {
		options.API = "https://api.github.com"
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 15 * time.Second}
	}
	if options.Interval == 0 {
		options.Interval = 24 * time.Hour
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Checker{options: options, last: Result{Current: options.Current}}
}

// Last is the last answer (nothing newer before the first check).
func (c *Checker) Last() Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

// Check asks GitHub unless it was asked within the interval; failures keep the last answer.
// The network call runs without the lock, so Last never waits for GitHub.
func (c *Checker) Check(ctx context.Context) (Result, error) {
	c.mu.Lock()
	now := c.options.Now()
	if !c.asked.IsZero() && now.Sub(c.asked) < c.options.Interval {
		defer c.mu.Unlock()
		return c.last, nil
	}
	c.asked = now
	c.mu.Unlock()

	result, err := c.fetch(ctx, now)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		return c.last, err
	}
	c.last = result
	return c.last, nil
}

func (c *Checker) fetch(ctx context.Context, now time.Time) (Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.options.API+"/repos/"+Repository+"/releases/latest", nil)
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.options.Client.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("GitHub answered %s", response.Status)
	}
	var release struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return Result{}, err
	}
	return Result{Current: c.options.Current, Latest: release.Tag, URL: release.URL, CheckedAt: now.UnixMilli(),
		Newer: !release.Draft && !release.Prerelease && Newer(c.options.Current, release.Tag)}, nil
}

// Run checks now and then every minute while enabled says so; Check itself asks GitHub once a
// day, and a setting switched on is followed within a minute.
func (c *Checker) Run(ctx context.Context, enabled func() bool) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if enabled() {
			_, _ = c.Check(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

var semver = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-.*)?$`)

// Newer reports whether latest is a newer release than current. A build between tags
// (v0.3.0-2-gabc) counts as its tag; a local build without a version ("dev") never asks to update;
// pre-releases are not offered.
func Newer(current, latest string) bool {
	c := semver.FindStringSubmatch(current)
	l := semver.FindStringSubmatch(latest)
	if c == nil || l == nil || l[4] != "" {
		return false
	}
	for i := 1; i <= 3; i++ {
		cn, _ := strconv.Atoi(c[i])
		ln, _ := strconv.Atoi(l[i])
		if ln != cn {
			return ln > cn
		}
	}
	return false
}
