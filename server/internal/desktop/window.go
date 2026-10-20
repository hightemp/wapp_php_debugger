package desktop

import (
	"context"
	"encoding/json"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

const windowKey = "window"

// WindowState is where the window was; Known means the position is worth restoring.
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
	Known     bool `json:"-"`
}

// Smaller windows (a minimised one reports its icon size on Windows) are not restored.
const (
	minWidth  = 640
	minHeight = 400
)

// DefaultWindowState is the first window: centred by the system.
func DefaultWindowState() WindowState { return WindowState{Width: 1200, Height: 760} }

// LoadWindowState returns the saved window, or the default one when there is none or it is broken.
func LoadWindowState(ctx context.Context, s *store.Store) WindowState {
	data, found, err := s.Setting(ctx, windowKey)
	if err != nil || !found {
		return DefaultWindowState()
	}
	var state WindowState
	if json.Unmarshal(data, &state) != nil || state.Width < minWidth || state.Height < minHeight {
		return DefaultWindowState()
	}
	state.Known = true
	return state
}

// SaveWindowState remembers the window for the next start.
func SaveWindowState(ctx context.Context, s *store.Store, state WindowState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.SetSetting(ctx, windowKey, data)
}
