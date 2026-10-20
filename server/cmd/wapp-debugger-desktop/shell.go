//go:build desktop

package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/app"
	"github.com/hightemp/wapp_php_debugger/server/internal/desktop"
)

//go:embed tray.png
var trayIcon []byte

// shell is the part of the desktop app around the window: tray, notifications, deep links,
// autostart and the remembered window. The decisions live in internal/desktop; this is the wiring.
type shell struct {
	app      *application.App
	window   *application.WebviewWindow
	core     *app.Core
	notifier *notifications.NotificationService
	alerter  *desktop.Alerter
	tray     *application.SystemTray
	trayMenu *application.Menu

	settings atomic.Pointer[api.DesktopSettings]
	quitting atomic.Bool
	names    sync.Map // project id → name, for notification titles
}

func newShell(core *app.Core) *shell {
	s := &shell{core: core, notifier: notifications.New()}
	defaults := desktop.DefaultSettings()
	s.settings.Store(&defaults)
	if current, err := core.DesktopSettings().Settings(context.Background()); err == nil {
		s.settings.Store(&current)
	}
	s.alerter = desktop.NewAlerter(desktop.AlerterOptions{
		Settings:    func() api.DesktopSettings { return *s.settings.Load() },
		ProjectName: s.projectName,
	})
	return s
}

// applySettings is called when the user saves the settings.
func (s *shell) applySettings(settings api.DesktopSettings) {
	s.settings.Store(&settings)
	if s.app == nil {
		return
	}
	enabled, err := s.app.Autostart.IsEnabled()
	if err == nil && enabled != settings.Autostart {
		if settings.Autostart {
			err = s.app.Autostart.Enable()
		} else {
			err = s.app.Autostart.Disable()
		}
	}
	if err != nil {
		slog.Warn("autostart", "error", err)
	}
}

func (s *shell) projectName(id int64) string {
	if name, ok := s.names.Load(id); ok {
		return name.(string)
	}
	project, found, err := s.core.Store().GetProject(context.Background(), id)
	if err != nil || !found {
		return ""
	}
	s.names.Store(id, project.Name)
	return project.Name
}

// windowOptions restore the window where it was.
func (s *shell) windowOptions(url string) application.WebviewWindowOptions {
	state := desktop.LoadWindowState(context.Background(), s.core.Store())
	options := application.WebviewWindowOptions{
		Name:   "main",
		Title:  "wapp-debugger",
		Width:  state.Width,
		Height: state.Height,
		URL:    url,
	}
	if state.Known {
		options.InitialPosition = application.WindowXY
		options.X, options.Y = state.X, state.Y
	}
	if state.Maximised {
		options.StartState = application.WindowStateMaximised
	}
	return options
}

// attach wires the window: closing hides it into the tray, the focus resets the error counter.
func (s *shell) attach(window *application.WebviewWindow) {
	s.window = window
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		s.saveWindow()
		if !s.quitting.Load() && s.settings.Load().CloseToTray {
			event.Cancel()
			window.Hide()
		}
	})
	window.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) {
		s.alerter.Seen()
		s.showCount(0)
	})
}

func (s *shell) saveWindow() {
	if s.window == nil || s.window.IsMinimised() {
		return
	}
	state := desktop.WindowState{Maximised: s.window.IsMaximised()}
	if !state.Maximised {
		state.X, state.Y = s.window.Position()
		state.Width, state.Height = s.window.Size()
	} else {
		// Keep the size the window returns to when it is no longer maximised
		previous := desktop.LoadWindowState(context.Background(), s.core.Store())
		state.X, state.Y, state.Width, state.Height = previous.X, previous.Y, previous.Width, previous.Height
	}
	if err := desktop.SaveWindowState(context.Background(), s.core.Store(), state); err != nil {
		slog.Warn("save window", "error", err)
	}
}

// open shows the window and goes to a route of the UI (a deep link, a notification, the tray menu).
func (s *shell) open(route string) {
	if s.window == nil {
		return
	}
	s.window.Show()
	if s.window.IsMinimised() {
		s.window.UnMinimise()
	}
	s.window.Focus()
	if route != "" {
		// The route is built from numbers by desktop.ParseLink or the Alerter
		s.window.ExecJS("window.location.hash = " + strconv.Quote(route))
	}
}

// startTray puts the icon with its menu into the tray (the menu bar on macOS).
func (s *shell) startTray() {
	s.tray = s.app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		s.tray.SetTemplateIcon(trayIcon)
	} else {
		s.tray.SetIcon(trayIcon)
	}
	s.tray.SetTooltip("wapp-debugger")
	s.tray.OnClick(func() { s.open("") })
	s.trayMenu = application.NewMenu()
	s.tray.SetMenu(s.trayMenu)
	s.rebuildTrayMenu()
}

func (s *shell) rebuildTrayMenu() {
	if s.trayMenu == nil {
		return
	}
	s.trayMenu.Clear()
	s.trayMenu.Add("Open wapp-debugger").OnClick(func(*application.Context) { s.open("") })
	if recent := s.alerter.Recent(); len(recent) > 0 {
		s.trayMenu.AddSeparator()
		for _, alert := range recent {
			route := alert.Route
			s.trayMenu.Add(alert.Title + " — " + alert.Body).OnClick(func(*application.Context) { s.open(route) })
		}
	}
	s.trayMenu.AddSeparator()
	s.trayMenu.Add("Quit").OnClick(func(*application.Context) {
		s.quitting.Store(true)
		s.app.Quit()
	})
	s.trayMenu.Update()
}

// showCount puts the number of unseen requests with errors on the tray icon.
func (s *shell) showCount(unseen int) {
	if s.tray == nil {
		return
	}
	if unseen == 0 {
		s.tray.SetLabel("")
		s.tray.SetTooltip("wapp-debugger")
		return
	}
	s.tray.SetLabel(strconv.Itoa(unseen))
	s.tray.SetTooltip(fmt.Sprintf("wapp-debugger: %d new requests with errors", unseen))
}

// Notify is desktop.AlertSink: a system notification that opens the request when clicked.
func (s *shell) Notify(alert desktop.Alert) {
	err := s.notifier.SendNotification(notifications.NotificationOptions{
		ID:    "wapp-debugger-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Title: alert.Title,
		Body:  alert.Body,
		Data:  map[string]any{"route": alert.Route},
	})
	if err != nil {
		slog.Warn("notification", "error", err)
	}
}

// Count is desktop.AlertSink: the tray counter and its menu of recent errors.
func (s *shell) Count(unseen int) {
	application.InvokeAsync(func() {
		s.showCount(unseen)
		s.rebuildTrayMenu()
	})
}

// start runs everything around the window once the app is up.
func (s *shell) start(ctx context.Context) {
	s.startTray()
	s.notifier.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			return
		}
		// The route was put there by the Alerter
		if route, _ := result.Response.UserInfo["route"].(string); strings.HasPrefix(route, "#/") {
			application.InvokeAsync(func() { s.open(route) })
		}
	})
	if runtime.GOOS == "darwin" {
		if _, err := s.notifier.RequestNotificationAuthorization(); err != nil {
			slog.Warn("notifications", "error", err)
		}
	}
	s.applySettings(*s.settings.Load())
	go desktop.RunAlerts(ctx, s.core.Broker(), s.alerter, s, time.Second)
}

// macMenu is the application menu of macOS: the standard roles with their shortcuts.
func macMenu() *application.Menu {
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	return menu
}
