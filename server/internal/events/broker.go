// Package events — рассылка изменений подписчикам SSE (п. 4.3 плана, /api/v1/stream).
package events

import (
	"context"
	"encoding/json"
	"sync"
)

// ResyncName — событие «данные пропущены, перечитайте всё».
const ResyncName = "resync"

const subscriberBuffer = 64

// Event — событие потока: монотонный ID (для Last-Event-ID), имя и JSON-данные.
type Event struct {
	ID   int64
	Name string
	Data json.RawMessage
}

type subscriber struct {
	ch chan Event
}

// Broker хранит последние события для переподключившихся клиентов и рассылает новые.
// Публикация никогда не ждёт подписчиков: отставшему вместо пропущенных событий приходит resync.
type Broker struct {
	mu          sync.Mutex
	lastID      int64
	buffer      []Event // последние события по возрастанию ID, не больше capacity
	capacity    int
	subscribers map[*subscriber]struct{}
}

// NewBroker создаёт брокер, который помнит capacity последних событий.
func NewBroker(capacity int) *Broker {
	return &Broker{capacity: capacity, subscribers: map[*subscriber]struct{}{}}
}

// Publish рассылает событие; data кодируется в JSON.
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

// deliver отправляет событие без ожидания; если буфер подписчика полон, заменяет его содержимое на resync.
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

// Subscribe подписывает до отмены ctx. lastEventID > 0 — продолжение после переподключения:
// сначала приходят пропущенные события из буфера, а если их уже нет — resync.
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

// missedSince возвращает события с ID > lastEventID или nil, если часть из них уже вытеснена из буфера.
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
