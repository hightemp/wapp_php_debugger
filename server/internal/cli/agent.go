package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/agent"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
)

type agentConfig struct {
	logsDir  string
	url      string
	token    string
	interval time.Duration
}

func newAgentFlags(cfg *agentConfig, output io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprint(output, "Usage: wapp-debugger agent --logs-dir DIR --push URL\n\n"+
			"Pushes the logs the PHP library writes in DIR to the viewer's receiver (serve --ingest-addr),\n"+
			"for containers and servers the viewer cannot reach over SSH.\n\nFlags:\n")
		flags.PrintDefaults()
	}
	flags.StringVar(&cfg.logsDir, "logs-dir", "", "logs directory of the project (required)")
	flags.StringVar(&cfg.url, "push", "", "the receiver, e.g. http://viewer.lan:8821/ingest/v2 (required)")
	flags.StringVar(&cfg.token, "token", os.Getenv("WAPP_DEBUGGER_PUSH_TOKEN"), "the project's push token (default: $WAPP_DEBUGGER_PUSH_TOKEN, which keeps it out of the process list)")
	flags.DurationVar(&cfg.interval, "interval", 2*time.Second, "fallback rescan period; changes are pushed right away where the file system reports them")
	return flags
}

func parseAgent(args []string) (agentConfig, error) {
	var cfg agentConfig
	flags := newAgentFlags(&cfg, io.Discard)
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	switch {
	case flags.NArg() > 0:
		return cfg, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	case cfg.logsDir == "":
		return cfg, errors.New("--logs-dir is required")
	case !strings.HasPrefix(cfg.url, "http://") && !strings.HasPrefix(cfg.url, "https://"):
		return cfg, errors.New("--push must be the http(s) address of the receiver")
	case cfg.token == "":
		return cfg, errors.New("the push token is required: WAPP_DEBUGGER_PUSH_TOKEN or --token")
	case cfg.interval <= 0:
		return cfg, errors.New("--interval must be positive")
	}
	return cfg, nil
}

func runAgent(args []string, stderr io.Writer) int {
	cfg, err := parseAgent(args)
	if errors.Is(err, flag.ErrHelp) {
		newAgentFlags(&agentConfig{}, stderr).Usage()
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "agent: %v\n\n", err)
		newAgentFlags(&agentConfig{}, stderr).Usage()
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a := &agent.Agent{Source: source.NewLocal(cfg.logsDir, ""), URL: cfg.url, Token: cfg.token}
	slog.Info("wapp-debugger agent started", "logs_dir", cfg.logsDir, "push", cfg.url)
	if err := a.Run(ctx, cfg.logsDir, cfg.interval); err != nil {
		fmt.Fprintf(stderr, "agent: %v\n", err)
		return 1
	}
	return 0
}
