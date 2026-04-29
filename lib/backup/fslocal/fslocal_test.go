package fslocal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSInitCleanDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "testfile"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	// trailing slash must not cause ListParts to panic
	fs := &FS{Dir: dir + string(filepath.Separator)}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %s", err)
	}
	defer fs.MustStop()

	parts, err := fs.ListParts()
	if err != nil {
		t.Fatalf("ListParts error: %s", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
}
