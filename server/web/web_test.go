package web

import (
	"io/fs"
	"testing"
)

// Without the embedui tag (tests, go vet) a placeholder with a hint is served instead of the built UI.
func TestUIHasIndex(t *testing.T) {
	data, err := fs.ReadFile(UI(), "index.html")
	if err != nil || len(data) == 0 {
		t.Fatalf("index.html: %v", err)
	}
}
