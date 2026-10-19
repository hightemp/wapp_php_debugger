// Package version хранит данные сборки; значения подставляются через -ldflags "-X ..." (см. Makefile).
package version

import "fmt"

var (
	// Version — тег релиза, например v0.1.0.
	Version = "dev"
	// Commit — короткий хеш коммита сборки.
	Commit = "none"
	// Date — дата сборки.
	Date = "unknown"
)

// String возвращает версию в виде "v0.1.0 (commit abc1234, built 2026-10-19)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
