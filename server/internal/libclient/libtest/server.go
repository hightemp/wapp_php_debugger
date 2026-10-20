// Package libtest is the PHP library HTTP mode (spec/library-http.yaml) over a directory, for tests:
// a TLS server that answers only with the right token, as the library does.
package libtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const prefix = "/__wapp_debugger/v1/"

// Server serves Dir as the logs directory of a project with the library 2.x in HTTP mode.
type Server struct {
	*httptest.Server
	Token   string
	Dir     string
	enabled atomic.Bool
	// Limited makes the next answers 429 with Retry-After.
	Limited  atomic.Int32
	requests atomic.Int64
	mu       sync.Mutex
}

// Start serves files (name → content) over HTTPS; Client() trusts its certificate.
func Start(t *testing.T, token string, files map[string][]byte) *Server {
	t.Helper()
	s := &Server{Token: token, Dir: t.TempDir()}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(s.Dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Requests counts the requests that reached the library (with the right token).
func (s *Server) Requests() int64 { return s.requests.Load() }

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	route, found := strings.CutPrefix(r.URL.Path, prefix)
	if !found || r.Header.Get("X-Debugger-Token") != s.Token {
		http.NotFound(w, r) // the application answers: the library stays silent
		return
	}
	s.requests.Add(1)
	if s.Limited.Load() > 0 {
		s.Limited.Add(-1)
		w.Header().Set("Retry-After", "1")
		problem(w, http.StatusTooManyRequests, "the HTTP mode rate limit is reached")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case route == "status" && r.Method == http.MethodGet:
		s.status(w)
	case route == "enabled" && r.Method == http.MethodPut:
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Enabled == nil {
			problem(w, http.StatusBadRequest, `expected {"enabled": true|false}`)
			return
		}
		s.enabled.Store(*body.Enabled)
		s.status(w)
	case route == "logs" && r.Method == http.MethodGet:
		s.list(w)
	case strings.HasPrefix(route, "logs/"):
		name := strings.TrimPrefix(route, "logs/")
		file := filepath.Join(s.Dir, name)
		if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
			problem(w, http.StatusNotFound, "no such log file")
			return
		}
		data, err := os.ReadFile(file)
		if err != nil {
			problem(w, http.StatusNotFound, "no such log file")
			return
		}
		switch r.Method {
		case http.MethodGet:
			read(w, r, data)
		case http.MethodDelete:
			_ = os.Remove(file)
			w.WriteHeader(http.StatusNoContent)
		default:
			problem(w, http.StatusMethodNotAllowed, "use GET, DELETE")
		}
	default:
		problem(w, http.StatusNotFound, "no such endpoint")
	}
}

func (s *Server) status(w http.ResponseWriter) {
	writeJSON(w, map[string]any{
		"enabled": s.enabled.Load(), "library_version": "2.0.0", "format_version": 2,
		"php_version": "7.4.33", "logs_dir_writable": true, "disk_free_bytes": nil,
	})
}

func (s *Server) list(w http.ResponseWriter) {
	entries, _ := os.ReadDir(s.Dir)
	files := []map[string]any{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, map[string]any{"name": entry.Name(), "size": info.Size(), "modified_at": info.ModTime().UnixMilli()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i]["name"].(string) < files[j]["name"].(string) })
	writeJSON(w, map[string]any{"files": files})
}

func read(w http.ResponseWriter, r *http.Request, data []byte) {
	offset, limit := int64(0), int64(1<<20)
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, _ = strconv.ParseInt(value, 10, 64)
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, _ = strconv.ParseInt(value, 10, 64)
	}
	size := int64(len(data))
	if offset > size {
		problem(w, http.StatusRequestedRangeNotSatisfiable, "offset is past the end of the file")
		return
	}
	end := min(offset+limit, size)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-File-Size", strconv.FormatInt(size, 10))
	_, _ = w.Write(data[offset:end])
}

func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"title": http.StatusText(status), "status": status, "detail": detail})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(fmt.Sprint("libtest: ", err))
	}
}
