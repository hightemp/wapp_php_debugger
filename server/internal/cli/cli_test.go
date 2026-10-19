package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/version"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionCommand(t *testing.T) {
	for _, arg := range []string{"version", "--version"} {
		code, stdout, _ := run(arg)
		if code != 0 {
			t.Fatalf("%s: exit code = %d, want 0", arg, code)
		}
		if want := "wapp-debugger " + version.String() + "\n"; stdout != want {
			t.Fatalf("%s: stdout = %q, want %q", arg, stdout, want)
		}
	}
}

func TestHelpPrintsUsageToStdout(t *testing.T) {
	code, stdout, _ := run("help")
	if code != 0 || !strings.Contains(stdout, "Usage: wapp-debugger") {
		t.Fatalf("help: code=%d stdout=%q", code, stdout)
	}
}

func TestNoArgumentsIsUsageError(t *testing.T) {
	code, _, stderr := run()
	if code != 2 || !strings.Contains(stderr, "Usage: wapp-debugger") {
		t.Fatalf("no args: code=%d stderr=%q", code, stderr)
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	code, _, stderr := run("frobnicate")
	if code != 2 || !strings.Contains(stderr, `unknown command "frobnicate"`) || !strings.Contains(stderr, "Usage:") {
		t.Fatalf("unknown: code=%d stderr=%q", code, stderr)
	}
}
