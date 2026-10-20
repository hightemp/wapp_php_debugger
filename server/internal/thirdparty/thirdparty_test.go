package thirdparty

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/thirdparty", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// converted checks the stream is one valid v2 request and gives its parts.
func converted(t *testing.T, name string) (*logformat.Request, []logformat.Event, *logformat.RequestEnd) {
	t.Helper()
	data, ok, err := Convert(name, fixture(t, name))
	if err != nil || !ok {
		t.Fatalf("%s: ok %v, %v", name, ok, err)
	}
	chunk, err := logformat.ParseV2Chunk(data, 0)
	if err != nil || len(chunk.Problems) > 0 || chunk.BrokenLines > 0 || chunk.Start == nil || chunk.End == nil {
		t.Fatalf("%s: not a valid v2 request: %v %v broken %d\n%s", name, err, chunk.Problems, chunk.BrokenLines, data)
	}
	return chunk.Start, chunk.Events, chunk.End
}

func ts(text string) int64 {
	at, _ := time.Parse(time.RFC3339, text)
	return at.UnixMilli()
}

func TestPHPErrorLog(t *testing.T) {
	start, events, end := converted(t, "php_errors.log")
	if start.Sapi != "cli" || len(start.Argv) != 1 || start.Argv[0] != "php_errors.log" || start.StartedAt != ts("2026-10-20T10:00:00Z") {
		t.Fatalf("start: %+v", start)
	}
	if len(events) != 4 {
		t.Fatalf("%d events: %+v", len(events), events)
	}
	warning := events[0]
	if warning.Type != "error" || warning.ErrorKind != "php_error" || warning.ErrorType != "E_WARNING" || warning.Level != "warning" ||
		warning.Message != "Undefined variable $discount" || warning.Source.File != "/var/www/shop/app/Cart.php" || warning.Source.Line != 42 {
		t.Fatalf("warning: %+v %+v", warning, warning.Source)
	}
	// The time zone of the line is honoured: 13:00:01 in Moscow is 10:00:01 UTC
	if deprecated := events[1]; deprecated.ErrorType != "E_DEPRECATED" || deprecated.Level != "info" || deprecated.TS != ts("2026-10-20T10:00:01Z") {
		t.Fatalf("deprecated: %+v", deprecated)
	}
	fatal := events[2]
	if fatal.ErrorKind != "exception" || fatal.ExceptionClass != "RuntimeException" || fatal.Message != "Order 7 not found" ||
		fatal.Source.Line != 31 || len(fatal.Trace) != 1 || fatal.Trace[0].File != "/var/www/shop/public/index.php" ||
		fatal.Trace[0].Line != 12 || fatal.Trace[0].Class != `App\Orders` || fatal.Trace[0].Function != "load" {
		t.Fatalf("fatal: %+v %+v", fatal, fatal.Trace)
	}
	if custom := events[3]; custom.Type != "message" || custom.Level != "message" || custom.Message != "Custom line written by error_log()" {
		t.Fatalf("custom: %+v", custom)
	}
	if end.EndedAt != ts("2026-10-20T10:00:03Z") || *end.DurationMs != 3000 {
		t.Fatalf("end: %+v", end)
	}
}

func TestLaravelLog(t *testing.T) {
	_, events, _ := converted(t, "laravel.log")
	if len(events) != 3 {
		t.Fatalf("%d events", len(events))
	}
	var context map[string]any
	if info := events[0]; info.Level != "info" || info.Message != "User logged in" || json.Unmarshal(info.Context, &context) != nil || context["user_id"] != 7.0 || info.Tags[0] != "local" {
		t.Fatalf("info: %+v %s", info, info.Context)
	}
	failure := events[1]
	if failure.Type != "error" || failure.ExceptionClass != `Illuminate\Database\QueryException` || failure.Code != "42S02" ||
		failure.Message != "SQLSTATE[42S02]: Base table or view not found" || failure.Source.Line != 822 ||
		len(failure.Trace) != 1 || failure.Trace[0].Line != 25 || failure.Trace[0].Function != "select" {
		t.Fatalf("error: %+v %+v %+v", failure, failure.Source, failure.Trace)
	}
	if warning := events[2]; warning.Level != "warning" || warning.Message != "Slow request" || warning.Context != nil || warning.Tags[0] != "production" {
		t.Fatalf("warning: %+v %s", warning, warning.Context)
	}
}

func TestNginxErrorLog(t *testing.T) {
	_, events, _ := converted(t, "nginx-error.log")
	if len(events) != 3 {
		t.Fatalf("%d events", len(events))
	}
	var context map[string]string
	first := events[0]
	if first.Level != "error" || first.Message != `FastCGI sent in stderr: "PHP message: PHP Warning:  Undefined index: id" while reading response header from upstream` ||
		json.Unmarshal(first.Context, &context) != nil || context["request"] != "GET /item HTTP/1.1" || context["client"] != "203.0.113.10" || context["host"] != "shop.example.test" {
		t.Fatalf("error: %+v %s", first, first.Context)
	}
	if events[1].Level != "warning" || events[2].Level != "info" || events[2].Message != "signal process started" {
		t.Fatalf("levels: %+v", events[1:])
	}
}

func TestOnlyKnownFormats(t *testing.T) {
	for name, data := range map[string]string{
		"v2":     `{"v":2,"type":"request_start"}` + "\n",
		"random": "hello\nworld\n",
		"empty":  "",
	} {
		if _, ok, err := Convert(name, []byte(data)); ok || err != nil {
			t.Errorf("%s: ok %v %v", name, ok, err)
		}
	}
	// The same file gives the same request: importing it again finds it
	a, _, _ := Convert("php_errors.log", fixture(t, "php_errors.log"))
	b, _, _ := Convert("php_errors.log", fixture(t, "php_errors.log"))
	if string(a) != string(b) {
		t.Fatal("the conversion is not stable")
	}
}
