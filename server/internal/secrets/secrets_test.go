package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileKeepsSecretsForTheUserOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "secrets.json")
	store := NewFile(path)
	if _, found, err := store.Get("server/2/token"); err != nil || found {
		t.Fatalf("empty store: found=%v err=%v", found, err)
	}
	if err := store.Set("server/2/token", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("server/3/token", "other"); err != nil {
		t.Fatal(err)
	}
	if value, found, err := NewFile(path).Get("server/2/token"); err != nil || !found || value != "s3cret" {
		t.Fatalf("Get = %q, %v, %v", value, found, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
	if err := store.Delete("server/2/token"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Get("server/2/token"); found {
		t.Fatal("deleted secret is still there")
	}
	if value, _, _ := store.Get("server/3/token"); value != "other" {
		t.Fatalf("other secret = %q", value)
	}
	if err := store.Delete("server/9/token"); err != nil {
		t.Fatalf("deleting a missing secret: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("temporary files left: %v", entries)
	}
}
