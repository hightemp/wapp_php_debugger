package thirdparty

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
)

// Logs of others are whatever an application wrote: the converter may refuse them, never crash, and
// what it accepts is a valid v2 stream.
func FuzzConvert(f *testing.F) {
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"php_errors.log", "laravel.log", "nginx-error.log"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../spec/fixtures/thirdparty", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(name, data)
	}
	f.Fuzz(func(t *testing.T, name string, data []byte) {
		converted, ok, err := Convert(name, data)
		if err != nil || !ok {
			return
		}
		chunk, err := logformat.ParseV2Chunk(converted, 0)
		if err != nil || chunk.BrokenLines > 0 || len(chunk.Problems) > 0 || chunk.Start == nil || chunk.End == nil {
			t.Fatalf("not a valid v2 stream: err=%v broken=%d problems=%v\n%s", err, chunk.BrokenLines, chunk.Problems, converted)
		}
	})
}
