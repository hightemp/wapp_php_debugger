// Command wapp-debugger is the log viewer for the PHP debugger.
package main

import (
	"os"

	"github.com/hightemp/wapp_php_debugger/server/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
