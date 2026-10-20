package desktop

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// settingsKey is where the desktop settings live in the viewer's database.
const settingsKey = "desktop"

// DefaultSettings notify about errors in every project, keep running in the tray, check for
// updates; no autostart until the user asks.
func DefaultSettings() api.DesktopSettings {
	settings := api.DesktopSettings{CloseToTray: true, UpdateCheck: true}
	settings.Notifications.Enabled = true
	settings.Notifications.MinLevel = api.DesktopSettingsNotificationsMinLevelError
	settings.Notifications.ProjectIds = []int64{}
	return settings
}

// Settings is api.DesktopRuntime: the settings are kept in the database and applied through onChange.
type Settings struct {
	store    *store.Store
	onChange func(api.DesktopSettings)
	mu       sync.Mutex
}

var _ api.DesktopRuntime = (*Settings)(nil)

// NewSettings keeps the settings in s; onChange (may be nil) applies saved ones.
func NewSettings(s *store.Store, onChange func(api.DesktopSettings)) *Settings {
	return &Settings{store: s, onChange: onChange}
}

// Settings returns the saved settings over the defaults.
func (s *Settings) Settings(ctx context.Context) (api.DesktopSettings, error) {
	settings := DefaultSettings()
	data, found, err := s.store.Setting(ctx, settingsKey)
	if err != nil || !found {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), nil // a broken value is not worth refusing to start for
	}
	return normalize(settings), nil
}

// UpdateSettings saves the settings and applies them.
func (s *Settings) UpdateSettings(ctx context.Context, settings api.DesktopSettings) (api.DesktopSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings = normalize(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return settings, err
	}
	if err := s.store.SetSetting(ctx, settingsKey, data); err != nil {
		return settings, err
	}
	if s.onChange != nil {
		s.onChange(settings)
	}
	return settings, nil
}

func normalize(settings api.DesktopSettings) api.DesktopSettings {
	if settings.Notifications.ProjectIds == nil {
		settings.Notifications.ProjectIds = []int64{}
	}
	if !settings.Notifications.MinLevel.Valid() {
		settings.Notifications.MinLevel = api.DesktopSettingsNotificationsMinLevelError
	}
	return settings
}
