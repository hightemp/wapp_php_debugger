package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/app"
	"github.com/hightemp/wapp_php_debugger/server/web"
)

func newServeFlags(cfg *app.Config, output io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprint(output, "Usage: wapp-debugger serve [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}
	flags.StringVar(&cfg.Addr, "addr", "127.0.0.1:8820", "HTTP address; other than loopback it needs a token")
	flags.StringVar(&cfg.Token, "token", os.Getenv("WAPP_DEBUGGER_TOKEN"), "access token clients must present (default: $WAPP_DEBUGGER_TOKEN, which keeps it out of the process list)")
	flags.StringVar(&cfg.DBPath, "db", "", "database file (default: in the user config directory)")
	flags.StringVar(&cfg.LogsDir, "logs-dir", "", "logs directory of a local project")
	flags.StringVar(&cfg.RootPath, "root", "", "project root (default: the nearest parent of --logs-dir with dws_debugger.json, else its parent)")
	flags.StringVar(&cfg.ProjectName, "project", "", "project name (default: name of the --root directory)")
	flags.StringVar(&cfg.ConfigPath, "config", "", "library dws_debugger.json (default: inside --root)")
	flags.DurationVar(&cfg.ScanInterval, "scan-interval", 5*time.Second, "fallback rescan period; changes are picked up right away where the file system reports them")
	return flags
}

// parseServe parses serve flags and fills in defaults.
func parseServe(args []string, userConfigDir func() (string, error)) (app.Config, error) {
	var cfg app.Config
	flags := newServeFlags(&cfg, io.Discard)
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if flags.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if cfg.ScanInterval <= 0 {
		return cfg, errors.New("--scan-interval must be positive")
	}
	if err := app.CheckExposure(cfg.Addr, cfg.Token); err != nil {
		return cfg, err
	}
	if cfg.DBPath == "" {
		if userConfigDir == nil {
			userConfigDir = os.UserConfigDir
		}
		dir, err := userConfigDir()
		if err != nil {
			return cfg, fmt.Errorf("default --db: %w", err)
		}
		cfg.DBPath = filepath.Join(dir, "wapp-debugger", "wapp-debugger.db")
	}
	if cfg.LogsDir != "" {
		if cfg.RootPath == "" {
			cfg.RootPath = app.GuessRoot(cfg.LogsDir)
		}
		if cfg.ProjectName == "" {
			cfg.ProjectName = filepath.Base(cfg.RootPath)
		}
		if cfg.ConfigPath == "" {
			cfg.ConfigPath = filepath.Join(cfg.RootPath, "dws_debugger.json")
		}
	}
	return cfg, nil
}

func serve(args []string, stderr io.Writer) int {
	cfg, err := parseServe(args, os.UserConfigDir)
	if errors.Is(err, flag.ErrHelp) {
		newServeFlags(&app.Config{}, stderr).Usage()
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n\n", err)
		newServeFlags(&app.Config{}, stderr).Usage()
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	cfg.UI = web.UI()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, func(string) {}); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}
