package desktop

import (
	"context"
	"encoding/json"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

// StreamName — имя потока Wails (app.HandleStream), по которому десктоп получает события.
// Asset handler Wails буферизует ответ целиком, поэтому SSE в окне не работает (проверено самопроверкой, п. 10.27).
const StreamName = "events"

// StreamFrame — событие потока: те же id, имя и данные, что у SSE.
type StreamFrame struct {
	ID   int64           `json:"id"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// StreamConn — то, что нужно от application.StreamConn Wails.
type StreamConn interface {
	Context() context.Context
	ReceiveJSON(v any) error
	SendJSON(v any) error
}

type hello struct {
	LastEventID int64 `json:"last_event_id"`
}

// ServeEvents пересылает события брокера в поток Wails, пока клиент не отключится.
// Первое сообщение клиента — {"last_event_id": N}: как Last-Event-ID у SSE.
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
