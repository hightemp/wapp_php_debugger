package desktop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/events"
)

// AlertSink shows what the Alerter decided: system notifications and the tray counter.
type AlertSink interface {
	Notify(alert Alert)
	Count(unseen int)
}

// RunAlerts feeds the event stream to the Alerter until ctx ends; every tick it sends held-back
// summaries and reports the counter when it changed (Seen resets it from outside).
func RunAlerts(ctx context.Context, broker *events.Broker, alerter *Alerter, sink AlertSink, tick time.Duration) {
	stream := broker.Subscribe(ctx, 0)
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	reported := -1
	report := func(alerts []Alert) {
		for _, alert := range alerts {
			sink.Notify(alert)
		}
		if unseen := alerter.Unseen(); unseen != reported {
			reported = unseen
			sink.Count(unseen)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			var payload api.StreamEvent
			if json.Unmarshal(event.Data, &payload) != nil {
				continue
			}
			report(alerter.Handle(payload))
		case <-ticker.C:
			report(alerter.Tick())
		}
	}
}
