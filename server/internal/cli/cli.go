// Package cli parses wapp-debugger command line arguments.
package cli

import (
	"fmt"
	"io"

	"github.com/hightemp/wapp_php_debugger/server/internal/version"
)

const usage = `Usage: wapp-debugger <command>

Commands:
  serve     run the viewer (serve --help for flags)
  agent     push a logs directory to the viewer (agent --help)
  version   print version
  help      print this help
`

// Run executes a command and returns the exit code: 0 on success, 2 on invalid arguments.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "serve":
		return serve(args[1:], stderr)
	case "agent":
		return runAgent(args[1:], stderr)
	case "version", "--version":
		fmt.Fprintf(stdout, "wapp-debugger %s\n", version.String())
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
