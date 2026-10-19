// Команда wapp-debugger — просмотрщик логов PHP-отладчика.
package main

import (
	"os"

	"github.com/hightemp/wapp_php_debugger/server/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
