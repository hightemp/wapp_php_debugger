// Package agent pushes the logs directory of a PHP project to the viewer's receiver
// (spec/ingest.yaml), for places SSH does not reach: containers, pods (plan 7.6).
package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/watch"
)

// batchLimit keeps a push under the receiver's 8 MiB.
const batchLimit = 4 << 20

// RefusedError is the receiver saying no to the token: it will not get better by itself.
type RefusedError struct{ Status string }

func (e *RefusedError) Error() string {
	return "the receiver refused the push token (" + e.Status + "): make a new one in the project settings"
}

// Agent sends the complete new lines of v2 log files. Offsets live in memory: after a restart it
// sends the files again, and the receiver keeps one copy of every event (it skips seq it has).
type Agent struct {
	Source source.Source
	URL    string // the receiver: http://viewer:8821/ingest/v2
	Token  string // the project's push token
	Client *http.Client
	Logger *slog.Logger

	offsets map[string]int64
	skipped map[string]bool // v1 files: the library 2.x writes v2, older ones are read over SSH or a share
}

// Sync pushes what is new in every file; it stops at the first failed push, which the next call retries.
func (a *Agent) Sync(ctx context.Context) error {
	if a.offsets == nil {
		a.offsets, a.skipped = map[string]int64{}, map[string]bool{}
	}
	files, err := a.Source.List(ctx)
	if err != nil {
		return err
	}
	for _, file := range files {
		if a.skipped[file.Name] {
			continue
		}
		offset := a.offsets[file.Name]
		if file.Size < offset {
			offset = 0 // truncated or replaced
		}
		for offset < file.Size {
			data, err := a.Source.ReadAt(ctx, file.Name, offset, batchLimit)
			if err != nil {
				return err
			}
			if offset == 0 {
				switch logformat.DetectFormat(data) {
				case 0:
					break // the first line is not complete yet
				case 1:
					a.skipped[file.Name] = true
					a.logger().Info("not pushing a v1 log file", "file", file.Name)
				}
				if a.skipped[file.Name] {
					break
				}
			}
			complete := bytes.LastIndexByte(data, '\n') + 1
			if complete == 0 {
				if len(data) >= batchLimit {
					a.skipped[file.Name] = true
					a.logger().Warn("a line longer than 4 MiB: the file is not pushed", "file", file.Name)
				}
				break // the last line is still being written
			}
			if err := a.push(ctx, data[:complete]); err != nil {
				a.offsets[file.Name] = offset
				return fmt.Errorf("%s: %w", file.Name, err)
			}
			offset += int64(complete)
		}
		a.offsets[file.Name] = offset
	}
	return nil
}

func (a *Agent) push(ctx context.Context, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-ndjson")
	request.Header.Set("Authorization", "Bearer "+a.Token)
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return &RefusedError{Status: response.Status}
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the receiver answered %s: %s", response.Status, strings.TrimSpace(string(answer)))
	}
	return nil
}

func (a *Agent) logger() *slog.Logger {
	if a.Logger == nil {
		return slog.Default()
	}
	return a.Logger
}

// Run pushes on every change of dir and every interval until ctx ends; a failure is logged once
// and retried. A refused token ends it.
func (a *Agent) Run(ctx context.Context, dir string, interval time.Duration) error {
	lastErr := ""
	for range watch.Dir(ctx, dir, watch.Options{PollInterval: interval, Debounce: 200 * time.Millisecond}) {
		err := a.Sync(ctx)
		var refused *RefusedError
		switch {
		case ctx.Err() != nil:
			return nil
		case errors.As(err, &refused):
			return err
		case err != nil && err.Error() != lastErr:
			a.logger().Warn("push", "error", err)
			lastErr = err.Error()
		case err == nil && lastErr != "":
			a.logger().Info("pushing again")
			lastErr = ""
		}
	}
	return nil
}
