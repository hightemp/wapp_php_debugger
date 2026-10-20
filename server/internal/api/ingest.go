package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// ingestLimit bounds a pushed batch: the library sends a request at a time.
const ingestLimit = 8 << 20

// IngestDeps is what the receiver of pushed logs needs.
type IngestDeps struct {
	Store  *store.Store
	Broker *events.Broker
	UI     fs.FS // the built UI: debugger.js, the JS client of pages, comes from it
}

// NewIngestHandler is the receiver of logs pushed over HTTP (spec/ingest.yaml): the library
// HttpWriter, the JS client of a page, the agent. It is served on its own address, apart from the
// viewer: only POST /ingest/v2, and a project token is the only key, so the viewer API never has
// to be open to the network for it.
func NewIngestHandler(deps IngestDeps) http.Handler {
	mux := http.NewServeMux()
	receiver := &ingestHandler{deps: deps}
	mux.Handle("POST /ingest/v2", receiver)
	mux.HandleFunc("GET /ingest/debugger.js", func(w http.ResponseWriter, r *http.Request) {
		script, err := fs.ReadFile(deps.UI, "debugger.js")
		if err != nil {
			writeProblem(w, http.StatusNotFound, "Not Found", "this build has no debugger.js")
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=300")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(script)
	})
	mux.HandleFunc("OPTIONS /ingest/v2", func(w http.ResponseWriter, _ *http.Request) {
		cors(w.Header())
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusNotFound, "Not Found", "this address only takes pushed logs: POST /ingest/v2")
	})
	return mux
}

// cors lets the JS client of any page send its events: the token, not the origin, is the key.
func cors(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "POST")
	header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Debugger-Token")
	header.Set("Access-Control-Max-Age", "600")
	// Private Network Access: a page on https may reach a receiver in the local network
	header.Set("Access-Control-Allow-Private-Network", "true")
}

type ingestHandler struct{ deps IngestDeps }

// ingestResult counts requests started and continued by a batch, and lines that were not taken.
type ingestResult struct {
	Created  int `json:"created"`
	Updated  int `json:"updated"`
	Rejected int `json:"rejected"`
}

func (h *ingestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cors(w.Header())
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		token = r.Header.Get("X-Debugger-Token")
	}
	if token == "" {
		// The JS client sends a beacon, which takes no headers
		token = r.URL.Query().Get("token")
	}
	projectID, known := int64(0), false
	if token != "" {
		var err error
		if projectID, known, err = h.deps.Store.ProjectByIngestToken(r.Context(), HashIngestToken(token)); err != nil {
			internalError(w, r, err)
			return
		}
	}
	if !known {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeProblem(w, http.StatusUnauthorized, "Unauthorized", "send the project's push token: Authorization: Bearer <token>")
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ingestLimit))
	if err != nil {
		badRequest(w, r, err)
		return
	}
	result, err := h.deps.Store.SavePushed(r.Context(), projectID, data)
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.publish(r.Context(), RequestCreated, result.Created)
	h.publish(r.Context(), RequestUpdated, result.Updated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ingestResult{Created: len(result.Created), Updated: len(result.Updated), Rejected: result.Rejected})
}

func (h *ingestHandler) publish(ctx context.Context, name StreamEventEvent, ids []int64) {
	if h.deps.Broker == nil {
		return
	}
	for _, id := range ids {
		row, found, err := h.deps.Store.GetRequest(ctx, id)
		if err != nil || !found {
			continue
		}
		dto := RequestDTO(row)
		h.deps.Broker.Publish(string(name), StreamEvent{Event: name, Request: &dto})
	}
}

// HashIngestToken is what the database keeps of a push token.
func HashIngestToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateIngestToken makes a new push token of a project and shows it this once.
func (s *server) CreateIngestToken(ctx context.Context, request CreateIngestTokenRequestObject) (CreateIngestTokenResponseObject, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	hash := HashIngestToken(token)
	if err := s.deps.Store.SetIngestToken(ctx, request.ProjectId, &hash); errors.Is(err, store.ErrNotFound) {
		return CreateIngestTokendefaultApplicationProblemPlusJSONResponse{Body: notFound("project"), StatusCode: http.StatusNotFound}, nil
	} else if err != nil {
		return nil, err
	}
	return CreateIngestToken200JSONResponse{Token: token}, nil
}

// DeleteIngestToken stops pushed logs of a project.
func (s *server) DeleteIngestToken(ctx context.Context, request DeleteIngestTokenRequestObject) (DeleteIngestTokenResponseObject, error) {
	if err := s.deps.Store.SetIngestToken(ctx, request.ProjectId, nil); errors.Is(err, store.ErrNotFound) {
		return DeleteIngestTokendefaultApplicationProblemPlusJSONResponse{Body: notFound("project"), StatusCode: http.StatusNotFound}, nil
	} else if err != nil {
		return nil, err
	}
	return DeleteIngestToken204Response{}, nil
}
