package version

import "testing"

func TestStringWithDefaults(t *testing.T) {
	if got, want := String(), "dev (commit none, built unknown)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringUsesBuildValues(t *testing.T) {
	defer func(v, c, d string) { Version, Commit, Date = v, c, d }(Version, Commit, Date)
	Version, Commit, Date = "v0.1.0", "abc1234", "2026-10-19"

	if got, want := String(), "v0.1.0 (commit abc1234, built 2026-10-19)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
