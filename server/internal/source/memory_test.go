package source_test

import (
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/source/sourcetest"
)

func TestMemorySourceContract(t *testing.T) {
	sourcetest.Run(t, func(_ *testing.T, files map[string][]byte) source.Source {
		return source.NewMemory(files)
	})
}
