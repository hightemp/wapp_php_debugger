// Package version holds build information; values are injected with -ldflags "-X ..." (see the Makefile).
package version

import "fmt"

var (
	// Version is the release tag, for example v0.1.0.
	Version = "dev"
	// Commit is the short hash of the build commit.
	Commit = "none"
	// Date is the build date.
	Date = "unknown"
)

// String returns the version as "v0.1.0 (commit abc1234, built 2026-10-19)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
