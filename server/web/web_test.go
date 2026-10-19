package web

import (
	"io/fs"
	"testing"
)

// Без тега embedui (тесты, go vet) вместо собранного UI отдаётся заглушка с подсказкой.
func TestUIHasIndex(t *testing.T) {
	data, err := fs.ReadFile(UI(), "index.html")
	if err != nil || len(data) == 0 {
		t.Fatalf("index.html: %v", err)
	}
}
