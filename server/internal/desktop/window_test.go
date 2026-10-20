package desktop

import (
	"context"
	"testing"
)

func TestWindowStateIsRememberedWithinSaneBounds(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	if state := LoadWindowState(ctx, s); state != DefaultWindowState() {
		t.Fatalf("first start: %+v", state)
	}
	if err := SaveWindowState(ctx, s, WindowState{X: 100, Y: 50, Width: 1400, Height: 900, Maximised: true}); err != nil {
		t.Fatal(err)
	}
	if state := LoadWindowState(ctx, s); state != (WindowState{X: 100, Y: 50, Width: 1400, Height: 900, Maximised: true, Known: true}) {
		t.Fatalf("restored %+v", state)
	}
	// A minimised or broken window must not come back as a speck
	_ = SaveWindowState(ctx, s, WindowState{X: -32000, Y: -32000, Width: 10, Height: 0})
	if state := LoadWindowState(ctx, s); state.Width != DefaultWindowState().Width || state.Height != DefaultWindowState().Height || state.Known {
		t.Fatalf("broken state restored: %+v", state)
	}
}
