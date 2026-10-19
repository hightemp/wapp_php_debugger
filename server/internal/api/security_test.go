package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func protected(token string) http.Handler {
	return Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), Security{Token: token})
}

func call(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:8820"
	for name, value := range headers {
		if name == "Host" {
			r.Host = value
			continue
		}
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestProtectSetsSecurityHeaders(t *testing.T) {
	w := call(protected(""), "GET", "/", nil)
	csp := w.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "base-uri 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP %q lacks %q", csp, directive)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP allows inline scripts: %q", csp)
	}
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := w.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// Without a token only loopback host names are served: a DNS rebinding page gets nothing.
func TestProtectWithoutTokenAcceptsOnlyLoopbackHosts(t *testing.T) {
	handler := protected("")
	for host, want := range map[string]int{
		"127.0.0.1:8820":      200,
		"localhost:8820":      200,
		"[::1]:8820":          200,
		"wails.localhost":     200,
		"evil.example:8820":   403,
		"192.168.1.10:8820":   403,
		"localhost.evil.test": 403,
	} {
		if got := call(handler, "GET", "/api/v1/requests", map[string]string{"Host": host}).Code; got != want {
			t.Errorf("Host %s: %d, want %d", host, got, want)
		}
	}
}

func TestProtectRejectsCrossOriginWrites(t *testing.T) {
	handler := protected("")
	cases := []struct {
		method, origin string
		want           int
	}{
		{"DELETE", "http://evil.example", 403},
		{"PUT", "http://127.0.0.1:8820", 200},
		{"POST", "", 200},                   // not a browser, or a same-origin form without Origin
		{"GET", "http://evil.example", 200}, // reads are guarded by the Host check and CORS
	}
	for _, c := range cases {
		headers := map[string]string{}
		if c.origin != "" {
			headers["Origin"] = c.origin
		}
		if got := call(handler, c.method, "/api/v1/projects/1", headers).Code; got != c.want {
			t.Errorf("%s from %q: %d, want %d", c.method, c.origin, got, c.want)
		}
	}
}

func TestProtectWithTokenRequiresIt(t *testing.T) {
	handler := protected("s3cret")
	external := map[string]string{"Host": "debugger.lan:8820"}

	if got := call(handler, "GET", "/api/v1/requests", external); got.Code != 401 || got.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("no token: %d %v", got.Code, got.Header())
	}
	if got := call(handler, "GET", "/api/v1/requests", map[string]string{"Host": "debugger.lan:8820", "Authorization": "Bearer nope"}).Code; got != 401 {
		t.Fatalf("wrong token: %d", got)
	}
	if got := call(handler, "GET", "/api/v1/requests", map[string]string{"Host": "debugger.lan:8820", "Authorization": "Bearer s3cret"}).Code; got != 200 {
		t.Fatalf("bearer: %d", got)
	}

	// The link with ?token= sets a cookie (EventSource cannot send headers) and drops the token from the URL
	login := call(handler, "GET", "/projects/1?token=s3cret", external)
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/projects/1" {
		t.Fatalf("login: %d %v", login.Code, login.Header())
	}
	cookie := login.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value != "s3cret" {
		t.Fatalf("cookie: %+v", cookie)
	}
	if got := call(handler, "GET", "/api/v1/stream", map[string]string{"Host": "debugger.lan:8820", "Cookie": cookie.Name + "=s3cret"}).Code; got != 200 {
		t.Fatalf("cookie auth: %d", got)
	}
	if got := call(handler, "GET", "/?token=wrong", external).Code; got != 401 {
		t.Fatalf("wrong login: %d", got)
	}
}

func TestAPIBodyIsLimited(t *testing.T) {
	e := newTestEnv(t)
	body := `{"name":"` + strings.Repeat("x", 2<<20) + `"}`
	r, _ := http.NewRequest("POST", e.server.URL+"/api/v1/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}
