package desktop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

type sink struct {
	mu     sync.Mutex
	alerts []Alert
	counts []int
}

func (s *sink) Notify(alert Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, alert)
}

func (s *sink) Count(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts = append(s.counts, n)
}

func (s *sink) snapshot() ([]Alert, []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Alert(nil), s.alerts...), append([]int(nil), s.counts...)
}

func TestRunAlertsFollowsTheStream(t *testing.T) {
	broker := events.NewBroker(16)
	a := NewAlerter(AlerterOptions{
		Settings:    DefaultSettings,
		ProjectName: func(int64) string { return "shop" },
		Window:      50 * time.Millisecond,
	})
	out := &sink{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunAlerts(ctx, broker, a, out, 10*time.Millisecond); close(done) }()
	time.Sleep(20 * time.Millisecond) // subscribed

	for id := int64(1); id <= 3; id++ {
		event := request(id, 1, 1, 0)
		broker.Publish(string(event.Event), event)
	}
	broker.Publish("server_status", api.StreamEvent{Event: api.ServerStatus})

	deadline := time.Now().Add(2 * time.Second)
	for {
		alerts, counts := out.snapshot()
		if len(alerts) == 2 && len(counts) > 0 && counts[len(counts)-1] == 3 {
			if alerts[1].Title != "2 more requests with errors" {
				t.Fatalf("alerts = %+v", alerts)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("alerts %+v counts %v", alerts, counts)
		}
		time.Sleep(10 * time.Millisecond)
	}
	a.Seen()
	deadline = time.Now().Add(time.Second)
	for {
		if _, counts := out.snapshot(); counts[len(counts)-1] == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the counter after Seen is not reported")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}
