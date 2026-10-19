package desktop

import (
	"context"
	"encoding/json"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

// StreamName is the Wails stream (app.HandleStream) the desktop receives events through.
// The Wails asset handler buffers whole responses, so SSE does not work inside the window (found by the self-test, plan 10.27).
const StreamName = "events"

// StreamFrame is a stream event: the same id, name and data as SSE.
type StreamFrame struct {
	ID   int64           `json:"id"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// StreamConn is what is needed from the Wails application.StreamConn.
type StreamConn interface {
	Context() context.Context
	ReceiveJSON(v any) error
	SendJSON(v any) error
}

type hello struct {
	LastEventID int64 `json:"last_event_id"`
}

// ServeEvents forwards broker events to the Wails stream until the client disconnects.
// The first client message is {"last_event_id": N}, like Last-Event-ID in SSE.
func ServeEvents(conn StreamConn, broker *events.Broker) error {
	var start hello
	if err := conn.ReceiveJSON(&start); err != nil {
		return err
	}
	for event := range broker.Subscribe(conn.Context(), start.LastEventID) {
		if err := conn.SendJSON(StreamFrame{ID: event.ID, Name: event.Name, Data: event.Data}); err != nil {
			return err
		}
	}
	return conn.Context().Err()
}
