package sourcetest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/source"
)

// FilesFactory gives a source that can read project files, for a project rooted at root.
type FilesFactory func(t *testing.T, root string) source.ProjectFiles

// RunProjectFiles checks reading project sources for the code preview: only inside the root,
// symlinks included, and only small enough files.
func RunProjectFiles(t *testing.T, newSource FilesFactory) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "shop")
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "app", "Item.php"), "<?php\nclass Item {}\n")
	write(filepath.Join(base, "secret.php"), "<?php // outside the project\n")
	write(filepath.Join(root, "big.php"), strings.Repeat("x", 2048))
	if err := os.Symlink(filepath.Join(base, "secret.php"), filepath.Join(root, "link.php")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "app"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	src := newSource(t, root)

	t.Run("a file inside the root", func(t *testing.T) {
		data, err := src.ReadProjectFile(ctx, root, filepath.Join(root, "app", "Item.php"), 1024)
		if err != nil || string(data) != "<?php\nclass Item {}\n" {
			t.Fatalf("read = %q, %v", data, err)
		}
	})
	t.Run("a symlink that stays inside", func(t *testing.T) {
		if _, err := src.ReadProjectFile(ctx, root, filepath.Join(root, "alias", "Item.php"), 1024); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("nothing outside the root", func(t *testing.T) {
		for _, path := range []string{
			filepath.Join(root, "..", "secret.php"),
			filepath.Join(base, "secret.php"),
			filepath.Join(root, "link.php"),
			"/etc/passwd",
		} {
			if _, err := src.ReadProjectFile(ctx, root, path, 1024); !errors.Is(err, source.ErrOutsideRoot) {
				t.Errorf("%s: err = %v, want ErrOutsideRoot", path, err)
			}
		}
	})
	t.Run("a missing file", func(t *testing.T) {
		if _, err := src.ReadProjectFile(ctx, root, filepath.Join(root, "nope.php"), 1024); !errors.Is(err, source.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("a file over the limit", func(t *testing.T) {
		if _, err := src.ReadProjectFile(ctx, root, filepath.Join(root, "big.php"), 1024); !errors.Is(err, source.ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
	})
}
