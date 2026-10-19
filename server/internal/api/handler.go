package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// Deps — зависимости HTTP-слоя.
type Deps struct {
	Store     *store.Store
	Broker    *events.Broker
	UI        fs.FS         // собранный Svelte-UI (index.html и assets/)
	Heartbeat time.Duration // период комментариев-heartbeat в SSE; 0 — 15 секунд
}

// NewHandler собирает HTTP-обработчик: /api/v1/* по spec/openapi.yaml, SSE-поток и UI.
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
	// SSE не укладывается в strict-обработчики (нужен flush после каждого события)
	mux.Handle("GET /api/v1/stream", &streamHandler{broker: deps.Broker, heartbeat: deps.Heartbeat})
	mux.Handle("/api/", apiMux)
	mux.Handle("/", spaHandler(deps.UI))
	return mux
}

func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Status: status, Title: title, Detail: &detail})
}

func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
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

// spaHandler отдаёт файлы UI, а для путей без расширения — index.html (маршруты клиентского роутера).
func spaHandler(ui fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if info, err := fs.Stat(ui, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Имена файлов Vite содержат хеш содержимого
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
