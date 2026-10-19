package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// ProjectRuntime runs the import of projects; the API restarts it when a project changes.
type ProjectRuntime interface {
	Add(project store.Project)
	Remove(projectID int64)
	Source(projectID int64) (source.Source, bool)
}

// Deps are the dependencies of the HTTP layer.
type Deps struct {
	Store     *store.Store
	Broker    *events.Broker
	Projects  ProjectRuntime
	UI        fs.FS         // built Svelte UI (index.html and assets/)
	Heartbeat time.Duration // SSE heartbeat comment period; 0 means 15 seconds
}

// NewHandler builds the HTTP handler: /api/v1/* per spec/openapi.yaml, the SSE stream and the UI.
func NewHandler(deps Deps) http.Handler {
	if deps.Heartbeat == 0 {
		deps.Heartbeat = 15 * time.Second
	}

	apiMux := http.NewServeMux()
	strict := NewStrictHandlerWithOptions(&server{deps: deps}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: internalError,
	})
	HandlerWithOptions(strict, StdHTTPServerOptions{BaseRouter: apiMux, ErrorHandlerFunc: badRequest})
	apiMux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusNotFound, "Not Found", "no such API endpoint")
	})

	mux := http.NewServeMux()
	// SSE does not fit strict handlers (it needs a flush after every event)
	mux.Handle("GET /api/v1/stream", &streamHandler{broker: deps.Broker, heartbeat: deps.Heartbeat})
	mux.Handle("/api/", limitBody(apiMux))
	mux.Handle("/", spaHandler(deps.UI))
	return mux
}

func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Status: status, Title: title, Detail: &detail})
}

func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	if tooLarge := new(http.MaxBytesError); errors.As(err, &tooLarge) {
		writeProblem(w, http.StatusRequestEntityTooLarge, "Content Too Large", err.Error())
		return
	}
	writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error())
}

func internalError(w http.ResponseWriter, _ *http.Request, err error) {
	writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
}

func problem(status int, title, detail string) Problem {
	return Problem{Status: status, Title: title, Detail: &detail}
}

func notImplemented() Problem {
	return problem(http.StatusNotImplemented, "Not Implemented", "this operation is not implemented yet")
}

// spaHandler serves UI files and falls back to index.html for paths without an extension (client-side routes).
func spaHandler(ui fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if info, err := fs.Stat(ui, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite file names contain a content hash
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, ui, name)
				return
			}
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, ui, "index.html")
	})
}
