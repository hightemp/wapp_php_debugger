//go:build desktop

// Command wapp-debugger-desktop is the desktop viewer built on Wails v3 (plan 7.8).
// It runs the same core and UI as the server; the HTTP handler is mounted into the Wails asset handler, no network port.
//
// Build: make desktop-build. Self-test (REST and the event stream inside the window, plan 10.27): wapp-debugger-desktop --self-test
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/app"
	"github.com/hightemp/wapp_php_debugger/server/internal/desktop"
	"github.com/hightemp/wapp_php_debugger/server/internal/version"
	"github.com/hightemp/wapp_php_debugger/server/web"
)

func main() {
	os.Exit(run())
}

func run() int {
	selfTest := flag.Bool("self-test", false, "check REST and the event stream inside the window, then exit")
	logsDir := flag.String("logs-dir", "", "logs directory of a local project")
	dbPath := flag.String("db", "", "database file (default: in the user config directory)")
	flag.Parse()

	cfg := app.Config{LogsDir: *logsDir, DBPath: *dbPath, UI: web.UI()}
	if cfg.DBPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cfg.DBPath = filepath.Join(dir, "wapp-debugger", "desktop.db")
	}
	if *selfTest {
		// The self-test never touches the user's real database
		dir, err := os.MkdirTemp("", "wapp-debugger-self-test-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer func() { _ = os.RemoveAll(dir) }()
		cfg.DBPath = filepath.Join(dir, "self-test.db")
	}
	if cfg.LogsDir != "" {
		cfg.RootPath = app.GuessRoot(cfg.LogsDir)
		cfg.ProjectName = filepath.Base(cfg.RootPath)
		cfg.ConfigPath = filepath.Join(cfg.RootPath, "dws_debugger.json")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// The shell around the window applies saved settings; it exists once the core does, and the
	// core's goroutines read it, hence the atomic pointer
	var current atomic.Pointer[shell]
	cfg.Desktop = &app.DesktopConfig{OnSettings: func(settings api.DesktopSettings) {
		if sh := current.Load(); sh != nil {
			sh.applySettings(settings)
		}
	}}
	cfg.UpdateCheck = func() bool {
		sh := current.Load()
		return sh != nil && sh.settings.Load().UpdateCheck
	}
	core, err := app.Start(context.Background(), cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer core.Close()
	sh := newShell(core)
	current.Store(sh)

	var handler http.Handler = core.Handler()
	var check *desktop.SelfTest
	startURL := "/"
	// Started by the system for a wapp-debugger:// link (Windows, Linux)
	if route, ok := desktop.LinkInArgs(flag.Args()); ok {
		startURL = "/" + route
	}
	if *selfTest {
		check = desktop.NewSelfTest(func() {
			core.Broker().Publish("selftest", map[string]any{"event": "selftest", "ok": true})
		})
		handler = check.Wrap(handler)
		startURL = desktop.PagePath
	}

	options := application.Options{
		Name:        "wapp-debugger",
		Description: "Log viewer for the PHP debugger " + version.String(),
		Assets:      application.AssetOptions{Handler: handler, DisableLogging: true},
		LogLevel:    slog.LevelWarn,
		Services:    sh.services(),
		// The app lives on in the tray; Quit in its menu ends it
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		OnShutdown: func() { sh.quitting.Store(true) },
	}
	if !*selfTest {
		// A second start (a deep link, the launcher) brings this window forward instead
		options.SingleInstance = &application.SingleInstanceOptions{
			UniqueID: "io.github.hightemp.wapp-debugger",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				route, _ := desktop.LinkInArgs(data.Args)
				application.InvokeAsync(func() { sh.open(route) })
			},
		}
	}
	wails := application.New(options)
	sh.app = wails
	if runtime.GOOS == "darwin" {
		wails.Menu.SetApplicationMenu(macMenu())
	}
	// macOS hands a wapp-debugger:// link over as an event, not as an argument
	wails.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
		if route, ok := desktop.ParseLink(event.Context().URL()); ok {
			sh.open(route)
		}
	})
	// Event stream for the UI: SSE through the asset handler never reaches the window (it is buffered), so Wails Stream is used
	wails.HandleStream(desktop.StreamName, func(conn *application.StreamConn) {
		defer func() { _ = conn.Close() }()
		_ = desktop.ServeEvents(conn, core.Broker())
	})
	window := wails.Window.NewWithOptions(sh.windowOptions(startURL))
	sh.attach(window)
	shellCtx, stopShell := context.WithCancel(context.Background())
	defer stopShell()
	wails.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		sh.start(shellCtx)
	})

	exitCode := 0
	if check != nil {
		go func() {
			// GTK4 under Xvfb (no GPU) takes up to half a minute to show the window in CI
			result, err := check.Wait(2 * time.Minute)
			report, _ := json.Marshal(map[string]any{"ok": err == nil && result.OK(), "result": result, "error": errorText(err)})
			fmt.Println(string(report))
			if err != nil || !result.OK() {
				exitCode = 1
			}
			sh.quitting.Store(true)
			wails.Quit()
		}()
	}
	if err := wails.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return exitCode
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
