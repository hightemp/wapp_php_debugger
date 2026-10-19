// Package desktop holds the parts of the desktop app that do not depend on Wails, so plain tests cover them.
package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PagePath is the self-test page the window opens in --self-test mode.
const PagePath = "/__selftest"

// Result is what the self-test page managed to do through the Wails asset handler.
type Result struct {
	Version   string   `json:"version"`    // /api/v1/version response
	Stream    bool     `json:"stream"`     // an event from the Wails stream reached the page
	Origin    string   `json:"origin"`     // location.origin of the window: Wails uses its own on each OS
	Errors    []string `json:"errors"`     // errors on the page side
	ElapsedMs int      `json:"elapsed_ms"` // how long the check took
}

// OK reports that REST and the event stream work inside the window (plan 10.27).
func (r Result) OK() bool {
	return r.Version != "" && r.Stream && len(r.Errors) == 0
}

// SelfTest serves the check page and receives its result.
type SelfTest struct {
	onStreamOpen func()
	once         sync.Once
	result       chan Result
}

// NewSelfTest creates a check; onStreamOpen publishes a probe event into the stream.
func NewSelfTest(onStreamOpen func()) *SelfTest {
	return &SelfTest{onStreamOpen: onStreamOpen, result: make(chan Result, 1)}
}

// Wrap adds the self-test endpoints in front of the real handler.
func (s *SelfTest) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == PagePath:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
		case r.Method == http.MethodPost && r.URL.Path == PagePath+"/stream-open":
			s.onStreamOpen()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == PagePath+"/result":
			var result Result
			if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
				result.Errors = append(result.Errors, "decode result: "+err.Error())
			}
			s.once.Do(func() { s.result <- result })
			w.WriteHeader(http.StatusNoContent)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// Wait waits for the page result.
func (s *SelfTest) Wait(timeout time.Duration) (Result, error) {
	select {
	case result := <-s.result:
		return result, nil
	case <-time.After(timeout):
		return Result{}, fmt.Errorf("self-test: no result within %s", timeout)
	}
}

// The page checks the same path the desktop UI uses: fetch to the API through the asset handler
// and the event stream through Wails Stream (SSE does not get through the asset handler, see stream.go).
const page = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>wapp-debugger self-test</title></head>
<body><p id="status">self-test…</p>
<script type="module">
import { JSONStream } from "/wails/runtime.js";

const started = performance.now();
const result = { version: "", stream: false, origin: location.origin, errors: [] };
let finished = false;
let stream;

async function finish(error) {
  if (finished) return;
  finished = true;
  if (error) result.errors.push(error);
  result.elapsed_ms = Math.round(performance.now() - started);
  stream?.close();
  document.getElementById("status").textContent = JSON.stringify(result);
  await fetch("/__selftest/result", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(result) });
}

try {
  const response = await fetch("/api/v1/version");
  result.version = (await response.json()).version || "";
} catch (error) {
  result.errors.push("version: " + error);
}

const timer = setTimeout(() => finish("stream: no event within 10s"), 10000);
stream = JSONStream("` + StreamName + `");
stream.onopen = () => {
  stream.send({ last_event_id: 0 });
  // The probe event is published once Go has subscribed to the broker
  setTimeout(() => fetch("/__selftest/stream-open", { method: "POST" }), 200);
};
stream.onmessage = (event) => {
  if (event.data && event.data.name === "selftest") {
    result.stream = true;
    clearTimeout(timer);
    finish();
  }
};
</script></body></html>
`
