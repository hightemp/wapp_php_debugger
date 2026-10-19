package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// Security configures Protect.
type Security struct {
	// Token, when set, is required from every client (Bearer header or the cookie set by ?token=);
	// without it only loopback host names are served.
	Token string
}

const (
	tokenCookie  = "wapp_debugger_token"
	maxBodyBytes = 1 << 20
	// The UI has no inline scripts; Svelte sets style attributes, hence 'unsafe-inline' for styles only
	contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
		"connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
)

// Protect guards the viewer served over the network: security headers, a loopback Host check
// against DNS rebinding (without a token), an Origin check on writes against CSRF, and the token.
func Protect(next http.Handler, sec Security) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")

		if sec.Token == "" && !loopbackHost(r.Host) {
			writeProblem(w, http.StatusForbidden, "Forbidden", "without a token the viewer answers only on localhost; set a token to serve other host names")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			writeProblem(w, http.StatusForbidden, "Forbidden", "cross-origin request")
			return
		}
		if sec.Token != "" {
			if login := r.URL.Query().Get("token"); login != "" && !strings.HasPrefix(r.URL.Path, "/api/") {
				if !tokenEqual(login, sec.Token) {
					writeProblem(w, http.StatusUnauthorized, "Unauthorized", "wrong token")
					return
				}
				http.SetCookie(w, &http.Cookie{Name: tokenCookie, Value: sec.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
				query := r.URL.Query()
				query.Del("token")
				target := r.URL.Path
				if encoded := query.Encode(); encoded != "" {
					target += "?" + encoded
				}
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
			if !authorized(r, sec.Token) {
				header.Set("WWW-Authenticate", "Bearer")
				writeProblem(w, http.StatusUnauthorized, "Unauthorized", "open the link with ?token= printed at start, or send Authorization: Bearer <token>")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameOrigin is true when the browser's Origin (if any) names this host.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	_, rest, found := strings.Cut(origin, "://")
	return found && strings.EqualFold(rest, r.Host)
}

func authorized(r *http.Request, token string) bool {
	if value, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); found && tokenEqual(value, token) {
		return true
	}
	cookie, err := r.Cookie(tokenCookie)
	return err == nil && tokenEqual(cookie.Value, token)
}

func tokenEqual(given, token string) bool {
	return subtle.ConstantTimeCompare([]byte(given), []byte(token)) == 1
}

// limitBody caps API request bodies; the decoder then fails and badRequest answers 413.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
