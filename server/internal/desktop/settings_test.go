package desktop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "desktop.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store.New(db)
}

func TestSettingsStartWithDefaultsAndApplyOnSave(t *testing.T) {
	ctx := context.Background()
	var applied []api.DesktopSettings
	settings := NewSettings(newStore(t), func(s api.DesktopSettings) { applied = append(applied, s) })

	got, err := settings.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Notifications.Enabled || got.Notifications.MinLevel != api.DesktopSettingsNotificationsMinLevelError ||
		got.Notifications.ProjectIds == nil || got.Autostart || !got.CloseToTray || !got.UpdateCheck {
		t.Fatalf("defaults = %+v", got)
	}

	got.Autostart = true
	got.Notifications.ProjectIds = nil // a client may send null
	saved, err := settings.UpdateSettings(ctx, got)
	if err != nil || !saved.Autostart || saved.Notifications.ProjectIds == nil {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	if len(applied) != 1 || !applied[0].Autostart {
		t.Fatalf("applied %+v", applied)
	}
	again, _ := NewSettings(settings.store, nil).Settings(ctx)
	if !again.Autostart {
		t.Fatalf("not kept: %+v", again)
	}
}
