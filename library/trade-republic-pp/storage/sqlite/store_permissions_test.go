package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDoesNotRepermissionExistingParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared-parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), filepath.Join(parent, "private.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("existing parent permissions changed to %o", got)
	}
}
