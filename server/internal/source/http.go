package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/libclient"
)

var (
	// ErrNoLibrary means the address answered, but not the library: a wrong URL or token, or the HTTP
	// mode is off in the library config. The library stays silent without the right token.
	ErrNoLibrary = errors.New("the debugger library did not answer: check the URL, the token and http_token in the library config")
	// ErrRateLimited means the library asked to wait (429); no requests go out until Retry-After passes.
	ErrRateLimited = errors.New("the library rate limit is reached")
)

// readLimitMax is the largest read the library serves at once
const readLimitMax = 8 << 20

// HTTPOptions configure a source over the library HTTP mode.
type HTTPOptions struct {
	BaseURL string // address of the application with the library, https:// (http:// only with a warning in the UI)
	Token   string // http_token from the library config
	Client  *http.Client
	// MinInterval spaces the requests to one project out, so polling never floods the application; 50 ms
	MinInterval time.Duration
	// OnResult hears how each request went: nil, or the error of the request (connection state in the UI)
	OnResult func(err error)
	// OnClock hears how far the server's clock is ahead, from the Date header (good to about a second)
	OnClock func(offset time.Duration)
}

// HTTP is the logs directory of a project read through the PHP library HTTP mode (spec/library-http.yaml),
// for servers without SSH access.
type HTTP struct {
	client      *libclient.Client
	minInterval time.Duration
	onResult    func(error)
	onClock     func(time.Duration)

	mu          sync.Mutex
	last        time.Time // when the last request went out
	pausedUntil time.Time // Retry-After of the last 429
}

var _ Source = (*HTTP)(nil)

// NewHTTP makes a source; the base URL must be an absolute http(s) address.
func NewHTTP(options HTTPOptions) (*HTTP, error) {
	base, err := url.Parse(options.BaseURL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
		return nil, fmt.Errorf("base URL must be an absolute https:// address, got %q", options.BaseURL)
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if options.MinInterval == 0 {
		options.MinInterval = 50 * time.Millisecond
	}
	token := options.Token
	client, err := libclient.NewClient(base.String(), libclient.WithHTTPClient(options.Client),
		libclient.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("X-Debugger-Token", token)
			return nil
		}))
	if err != nil {
		return nil, err
	}
	return &HTTP{client: client, minInterval: options.MinInterval, onResult: options.OnResult, onClock: options.OnClock}, nil
}

// wait holds a request back for the rate limit and the spacing between requests.
func (h *HTTP) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	now := time.Now()
	if now.Before(h.pausedUntil) {
		until := h.pausedUntil
		h.mu.Unlock()
		return fmt.Errorf("%w: retry after %s", ErrRateLimited, until.Format(time.TimeOnly))
	}
	next := h.last.Add(h.minInterval)
	if next.Before(now) {
		next = now
	}
	h.last = next
	h.mu.Unlock()
	select {
	case <-time.After(time.Until(next)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// problemOf turns an answer that is not 2xx into an error. Only an application/problem+json answer
// comes from the library; anything else is the application answering instead of it.
func (h *HTTP) problemOf(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentType != "application/problem+json" {
		return fmt.Errorf("%w (HTTP %d)", ErrNoLibrary, response.StatusCode)
	}
	var problem libclient.Problem
	_ = json.Unmarshal(body, &problem)
	detail := problem.Title
	if problem.Detail != nil {
		detail = *problem.Detail
	}
	switch response.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, detail)
	case http.StatusRequestedRangeNotSatisfiable:
		return ErrOffsetBeyondEnd
	case http.StatusTooManyRequests:
		seconds, err := strconv.Atoi(response.Header.Get("Retry-After"))
		if err != nil || seconds <= 0 {
			seconds = 60
		}
		h.mu.Lock()
		h.pausedUntil = time.Now().Add(time.Duration(seconds) * time.Second)
		h.mu.Unlock()
		return fmt.Errorf("%w: retry after %d s", ErrRateLimited, seconds)
	default:
		return fmt.Errorf("library: HTTP %d: %s", response.StatusCode, detail)
	}
}

// do sends a request and checks its status; the caller closes the body of a successful answer.
func (h *HTTP) do(ctx context.Context, send func() (*http.Response, error), want int) (*http.Response, error) {
	if err := h.wait(ctx); err != nil {
		return nil, err
	}
	sent := time.Now()
	response, err := send()
	if err == nil && h.onClock != nil {
		if date, dateErr := http.ParseTime(response.Header.Get("Date")); dateErr == nil {
			// Date has whole seconds: the true time is anywhere in that second
			middle := sent.Add(time.Since(sent) / 2)
			h.onClock(date.Add(500 * time.Millisecond).Sub(middle))
		}
	}
	if err == nil && response.StatusCode != want {
		defer func() { _ = response.Body.Close() }()
		err = h.problemOf(response)
	}
	if h.onResult != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		h.onResult(err)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func decodeJSON(response *http.Response, value any) error {
	defer func() { _ = response.Body.Close() }()
	contentType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentType != "application/json" {
		return fmt.Errorf("%w (an answer of type %q)", ErrNoLibrary, contentType)
	}
	return json.NewDecoder(response.Body).Decode(value)
}

// Status is the state of the library in the project.
func (h *HTTP) Status(ctx context.Context) (libclient.Status, error) {
	var status libclient.Status
	response, err := h.do(ctx, func() (*http.Response, error) { return h.client.GetStatus(ctx) }, http.StatusOK)
	if err != nil {
		return status, err
	}
	return status, decodeJSON(response, &status)
}

// List returns the log files the library lists; names it could not read back are skipped.
func (h *HTTP) List(ctx context.Context) ([]FileInfo, error) {
	response, err := h.do(ctx, func() (*http.Response, error) { return h.client.ListLogs(ctx) }, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var list struct {
		Files []libclient.LogFile `json:"files"`
	}
	if err := decodeJSON(response, &list); err != nil {
		return nil, err
	}
	files := make([]FileInfo, 0, len(list.Files))
	for _, file := range list.Files {
		if ValidateName(file.Name) != nil {
			continue // a file name the source could not read back
		}
		files = append(files, FileInfo{Name: file.Name, Size: file.Size, ModTime: time.UnixMilli(file.ModifiedAt)})
	}
	return files, nil
}

// ReadAt reads up to limit bytes from offset, at most 8 MiB at once.
func (h *HTTP) ReadAt(ctx context.Context, name string, offset int64, limit int) ([]byte, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	limit = min(limit, readLimitMax)
	params := &libclient.ReadLogParams{Offset: &offset, Limit: &limit}
	response, err := h.do(ctx, func() (*http.Response, error) { return h.client.ReadLog(ctx, name, params) }, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	return io.ReadAll(io.LimitReader(response.Body, int64(limit)))
}

// Remove deletes a log file.
func (h *HTTP) Remove(ctx context.Context, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	response, err := h.do(ctx, func() (*http.Response, error) { return h.client.RemoveLog(ctx, name) }, http.StatusNoContent)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

// Enabled is the flag from the library status.
func (h *HTTP) Enabled(ctx context.Context) (bool, error) {
	status, err := h.Status(ctx)
	return status.Enabled, err
}

// SetEnabled asks the library to switch itself; it rewrites dws_debugger.json on its side.
func (h *HTTP) SetEnabled(ctx context.Context, enabled bool) error {
	body := libclient.SetEnabledJSONRequestBody{Enabled: enabled}
	response, err := h.do(ctx, func() (*http.Response, error) { return h.client.SetEnabled(ctx, body) }, http.StatusOK)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

// Close does nothing: the HTTP client keeps no connection of its own.
func (h *HTTP) Close() error { return nil }
