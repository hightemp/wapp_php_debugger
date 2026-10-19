// Package watch tells the importer when to scan a logs directory: right after file system
// notifications (debounced) and periodically as a fallback for file systems without them.
package watch

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Options tune the watcher.
type Options struct {
	PollInterval time.Duration // fallback scan period; scans also happen on file system events
	Debounce     time.Duration // events closer than this are merged into one signal
}

// Dir sends a signal right away (the initial scan), after file changes in dir and every PollInterval.
// The channel is closed when ctx is canceled. Signals never queue up: a slow reader gets one.
func Dir(ctx context.Context, dir string, options Options) <-chan struct{} {
	signals := make(chan struct{}, 1)
	notify := func() {
		select {
		case signals <- struct{}{}:
		default: // a scan is already pending
		}
	}
	notify()

	var events <-chan fsnotify.Event
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		if watcher.Add(dir) == nil {
			events = watcher.Events
		} else {
			_ = watcher.Close()
			watcher = nil
		}
	}

	go func() {
		defer close(signals)
		if watcher != nil {
			defer func() { _ = watcher.Close() }()
		}
		poll := time.NewTicker(options.PollInterval)
		defer poll.Stop()
		var debounce <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-poll.C:
				notify()
			case _, ok := <-events:
				if !ok {
					events = nil
					continue
				}
				if debounce == nil {
					debounce = time.After(options.Debounce)
				}
			case <-debounce:
				debounce = nil
				notify()
			}
		}
	}()
	return signals
}

// Every sends a signal right away and then every interval: for directories without file system
// events (SFTP, HTTP).
func Every(ctx context.Context, interval time.Duration) <-chan struct{} {
	signals := make(chan struct{}, 1)
	signals <- struct{}{}
	go func() {
		defer close(signals)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case signals <- struct{}{}:
				default: // a scan is still running: one pending signal is enough
				}
			}
		}
	}()
	return signals
}
