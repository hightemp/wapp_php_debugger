package events

import (
	"context"
	"testing"
	"time"
)

func receive(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return e
	case <-time.After(time.Second):
		t.Fatal("no event within a second")
	}
	return Event{}
}

func TestSubscribersReceivePublishedEventsInOrder(t *testing.T) {
	b := NewBroker(16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, second := b.Subscribe(ctx, 0), b.Subscribe(ctx, 0)

	b.Publish("request_created", map[string]int{"id": 1})
	b.Publish("messages_added", map[string]int{"count": 3})

	for _, ch := range []<-chan Event{first, second} {
		if e := receive(t, ch); e.ID != 1 || e.Name != "request_created" || string(e.Data) != `{"id":1}` {
			t.Fatalf("first event = %+v", e)
		}
		if e := receive(t, ch); e.ID != 2 || e.Name != "messages_added" {
			t.Fatalf("second event = %+v", e)
		}
	}
}

func TestReconnectReplaysEventsAfterLastEventID(t *testing.T) {
	b := NewBroker(16)
	for i := range 5 {
		b.Publish("request_created", i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx, 3)
	if e := receive(t, ch); e.ID != 4 {
		t.Fatalf("replay must start after Last-Event-ID 3, got %+v", e)
	}
	if e := receive(t, ch); e.ID != 5 {
		t.Fatalf("got %+v", e)
	}
}

func TestReconnectAfterBufferOverflowGetsResync(t *testing.T) {
	b := NewBroker(2)
	for i := range 5 {
		b.Publish("request_created", i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if e := receive(t, b.Subscribe(ctx, 1)); e.Name != "resync" {
		t.Fatalf("events 2..3 are gone, client must reload everything: %+v", e)
	}
}

func TestSlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	b := NewBroker(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx, 0) // nobody reads

	done := make(chan struct{})
	go func() {
		for i := range 1000 {
			b.Publish("request_created", i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked by a slow subscriber")
	}
	// The lagging subscriber gets resync and then live events again
	for e := range ch {
		if e.Name == "resync" {
			return
		}
	}
	t.Fatal("slow subscriber never got resync")
}

func TestSubscriptionEndsWithContext(t *testing.T) {
	b := NewBroker(4)
	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx, 0)
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unexpected event")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after context cancel")
	}
}
