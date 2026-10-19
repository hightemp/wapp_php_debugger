// Package events fans out changes to SSE subscribers (plan 4.3, /api/v1/stream).
package events

import (
	"context"
	"encoding/json"
	"sync"
)

// ResyncName is the "events were missed, reload everything" event.
const ResyncName = "resync"

const subscriberBuffer = 64

// Event is a stream event: a monotonic ID (for Last-Event-ID), a name and JSON data.
type Event struct {
	ID   int64
	Name string
	Data json.RawMessage
}

type subscriber struct {
	ch chan Event
}

// Broker keeps recent events for reconnecting clients and fans out new ones.
// Publishing never waits for subscribers: a lagging one gets resync instead of the missed events.
type Broker struct {
	mu          sync.Mutex
	lastID      int64
	buffer      []Event // recent events in ascending ID order, at most capacity
	capacity    int
	subscribers map[*subscriber]struct{}
}

// NewBroker creates a broker that remembers the last capacity events.
func NewBroker(capacity int) *Broker {
	return &Broker{capacity: capacity, subscribers: map[*subscriber]struct{}{}}
}

// Publish fans out an event; data is encoded as JSON.
func (b *Broker) Publish(name string, data any) {
	encoded, err := json.Marshal(data)
	if err != nil {
		encoded = []byte("null")
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lastID++
	event := Event{ID: b.lastID, Name: name, Data: encoded}
	b.buffer = append(b.buffer, event)
	if len(b.buffer) > b.capacity {
		b.buffer = b.buffer[len(b.buffer)-b.capacity:]
	}
	for sub := range b.subscribers {
		b.deliver(sub, event)
	}
}

// deliver sends an event without waiting; if the subscriber buffer is full, its contents are replaced with resync.
func (b *Broker) deliver(sub *subscriber, event Event) {
	select {
	case sub.ch <- event:
	default:
		for drained := false; !drained; {
			select {
			case <-sub.ch:
			default:
				drained = true
			}
		}
		sub.ch <- b.resync()
	}
}

func (b *Broker) resync() Event {
	return Event{ID: b.lastID, Name: ResyncName, Data: json.RawMessage(`{"event":"resync"}`)}
}

// Subscribe subscribes until ctx is canceled. lastEventID > 0 resumes after a reconnect:
// missed events come from the buffer first, or resync if they are gone.
func (b *Broker) Subscribe(ctx context.Context, lastEventID int64) <-chan Event {
	sub := &subscriber{ch: make(chan Event, subscriberBuffer)}

	b.mu.Lock()
	if lastEventID > 0 && lastEventID < b.lastID {
		missed := b.missedSince(lastEventID)
		if missed == nil || len(missed) > subscriberBuffer-1 {
			sub.ch <- b.resync()
		} else {
			for _, event := range missed {
				sub.ch <- event
			}
		}
	}
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, sub)
		close(sub.ch)
		b.mu.Unlock()
	}()
	return sub.ch
}

// missedSince returns events with ID > lastEventID, or nil if some of them were evicted from the buffer.
func (b *Broker) missedSince(lastEventID int64) []Event {
	if len(b.buffer) == 0 || b.buffer[0].ID > lastEventID+1 {
		return nil
	}
	for i, event := range b.buffer {
		if event.ID > lastEventID {
			return append([]Event(nil), b.buffer[i:]...)
		}
	}
	return []Event{}
}
