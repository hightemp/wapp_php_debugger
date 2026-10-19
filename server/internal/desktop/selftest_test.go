package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSelfTestServesPageAndPassesOtherRequestsThrough(t *testing.T) {
	selfTest := NewSelfTest(func() {})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("next")) })
	handler := selfTest.Wrap(next)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, PagePath, nil))
	body := page.Body.String()
	if page.Code != 200 || !strings.Contains(body, "/api/v1/version") || !strings.Contains(body, "/wails/runtime.js") ||
		!strings.Contains(body, `JSONStream("`+StreamName+`")`) {
		t.Fatalf("self-test page: %d %s", page.Code, page.Body.String())
	}

	other := httptest.NewRecorder()
	handler.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil))
	if other.Body.String() != "next" {
		t.Fatal("other requests must reach the real handler")
	}
}

func TestSelfTestPublishesWhenStreamIsOpen(t *testing.T) {
	published := make(chan struct{}, 1)
	handler := NewSelfTest(func() { published <- struct{}{} }).Wrap(http.NotFoundHandler())

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/__selftest/stream-open", nil))
	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("stream-open must trigger the probe event")
	}
}

func TestSelfTestCollectsResult(t *testing.T) {
	selfTest := NewSelfTest(func() {})
	handler := selfTest.Wrap(http.NotFoundHandler())
	body := `{"version":"dev","stream":true,"errors":[],"elapsed_ms":42}`
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/__selftest/result", strings.NewReader(body)))

	result, err := selfTest.Wait(time.Second)
	if err != nil || result.Version != "dev" || !result.Stream || !result.OK() {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestSelfTestResultReportsFailures(t *testing.T) {
	for _, result := range []Result{
		{Version: "", Stream: true},
		{Version: "dev", Stream: false},
		{Version: "dev", Stream: true, Errors: []string{"boom"}},
	} {
		if result.OK() {
			t.Errorf("%+v must not be OK", result)
		}
	}
}

func TestSelfTestTimesOut(t *testing.T) {
	if _, err := NewSelfTest(func() {}).Wait(10 * time.Millisecond); err == nil {
		t.Fatal("no result within the timeout must be an error")
	}
}
