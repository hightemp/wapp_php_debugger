package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

// fakeConn is an in-memory Wails Stream connection: Receive returns client messages queued in advance.
type fakeConn struct {
	ctx      context.Context
	incoming chan []byte
	sent     chan StreamFrame
}

func (c *fakeConn) Context() context.Context { return c.ctx }

func (c *fakeConn) ReceiveJSON(v any) error {
	select {
	case data := <-c.incoming:
		return json.Unmarshal(data, v)
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *fakeConn) SendJSON(v any) error {
	frame, ok := v.(StreamFrame)
	if !ok {
		return errors.New("unexpected frame type")
	}
	c.sent <- frame
	return nil
}

func newFakeConn(t *testing.T) (*fakeConn, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &fakeConn{ctx: ctx, incoming: make(chan []byte, 4), sent: make(chan StreamFrame, 16)}, cancel
}

func nextFrame(t *testing.T, conn *fakeConn) StreamFrame {
	t.Helper()
	select {
	case frame := <-conn.sent:
		return frame
	case <-time.After(time.Second):
		t.Fatal("no frame within a second")
	}
	return StreamFrame{}
}

func TestServeEventsForwardsBrokerEventsAsFrames(t *testing.T) {
	broker := events.NewBroker(16)
	conn, cancel := newFakeConn(t)
	conn.incoming <- []byte(`{"last_event_id":0}`)
	done := make(chan error, 1)
	go func() { done <- ServeEvents(conn, broker) }()

	// The subscription appears after the client hello; keep publishing until a frame arrives
	deadline := time.After(time.Second)
	for {
		broker.Publish("request_created", map[string]int{"request_id": 7})
		select {
		case frame := <-conn.sent:
			if frame.Name != "request_created" || string(frame.Data) != `{"request_id":7}` || frame.ID == 0 {
				t.Fatalf("frame = %+v", frame)
			}
			cancel()
			if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("ServeEvents: %v", err)
			}
			return
		case <-deadline:
			t.Fatal("no frame")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestServeEventsReplaysAfterLastEventID(t *testing.T) {
	broker := events.NewBroker(16)
	for i := range 3 {
		broker.Publish("request_created", i)
	}
	conn, _ := newFakeConn(t)
	conn.incoming <- []byte(`{"last_event_id":2}`)
	go func() { _ = ServeEvents(conn, broker) }()

	if frame := nextFrame(t, conn); frame.ID != 3 {
		t.Fatalf("after last_event_id 2 expected event 3, got %+v", frame)
	}
}
