package source_test

import (
	"context"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/source/sourcetest"
)

func TestMemorySourceContract(t *testing.T) {
	sourcetest.Run(t, func(_ *testing.T, files map[string][]byte) source.Source {
		return source.NewMemory(files)
	})
}

func TestMemoryModTimeChangesOnlyOnWrite(t *testing.T) {
	src := source.NewMemory(map[string][]byte{"a": []byte("1"), "b": []byte("2")})
	before, _ := src.List(context.Background())
	time.Sleep(2 * time.Millisecond)
	again, _ := src.List(context.Background())
	if !before[0].ModTime.Equal(again[0].ModTime) {
		t.Fatal("ModTime must not change without writes")
	}
	src.Append("a", []byte("more"))
	after, _ := src.List(context.Background())
	if !after[0].ModTime.After(before[0].ModTime) || !after[1].ModTime.Equal(before[1].ModTime) {
		t.Fatalf("only the appended file changes: before=%v after=%v", before, after)
	}
}
