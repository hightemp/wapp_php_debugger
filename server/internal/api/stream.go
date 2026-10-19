package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

// streamHandler — GET /api/v1/stream: Server-Sent Events с повтором пропущенного по Last-Event-ID.
type streamHandler struct {
	broker    *events.Broker
	heartbeat time.Duration
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "streaming is not supported")
		return
	}
	lastEventID, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx перед просмотрщиком не должен буферизовать поток
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	stream := h.broker.Subscribe(r.Context(), lastEventID)
	ticker := time.NewTicker(h.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case event, open := <-stream:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Name, event.Data); err != nil {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
		flusher.Flush()
	}
}
