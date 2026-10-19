// Package desktop — части десктоп-версии, не зависящие от Wails (их можно проверить обычными тестами).
package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PagePath — страница самопроверки, которую открывает окно в режиме --self-test.
const PagePath = "/__selftest"

// Result — что страница самопроверки смогла сделать через asset handler Wails.
type Result struct {
	Version   string   `json:"version"`    // ответ /api/v1/version
	Stream    bool     `json:"stream"`     // событие из потока Wails дошло до страницы
	Origin    string   `json:"origin"`     // location.origin окна: у Wails он свой на каждой ОС
	Errors    []string `json:"errors"`     // ошибки на стороне страницы
	ElapsedMs int      `json:"elapsed_ms"` // время проверки
}

// OK — REST и поток событий работают внутри окна (п. 10.27 плана).
func (r Result) OK() bool {
	return r.Version != "" && r.Stream && len(r.Errors) == 0
}

// SelfTest отдаёт страницу проверки и принимает от неё результат.
type SelfTest struct {
	onStreamOpen func()
	once         sync.Once
	result       chan Result
}

// NewSelfTest создаёт проверку; onStreamOpen публикует пробное событие в поток.
func NewSelfTest(onStreamOpen func()) *SelfTest {
	return &SelfTest{onStreamOpen: onStreamOpen, result: make(chan Result, 1)}
}

// Wrap добавляет служебные пути самопроверки перед основным обработчиком.
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

// Wait ждёт результат страницы.
func (s *SelfTest) Wait(timeout time.Duration) (Result, error) {
	select {
	case result := <-s.result:
		return result, nil
	case <-time.After(timeout):
		return Result{}, fmt.Errorf("self-test: no result within %s", timeout)
	}
}

// Страница проверяет тот же путь, что и UI в десктопе: fetch к API через asset handler
// и поток событий через Wails Stream (SSE через asset handler не доходит, см. stream.go).
const page = `<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><title>wapp-debugger self-test</title></head>
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
  // Пробное событие публикуется, когда Go уже подписан на брокер
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
