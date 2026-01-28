package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPartiallyRemovedDir(t *testing.T) {
	f := func(dirName, filename string, want bool) {
		t.Helper()
		dirPath := filepath.Join(t.TempDir(), dirName)
		if err := os.Mkdir(dirPath, os.ModePerm); err != nil {
			t.Fatalf("cannot create directory=%q: %s", dirPath, err)
		}
		if len(filename) > 0 {
			f, err := os.Create(filepath.Join(dirPath, filename))
			if err != nil {
				t.Fatalf("cannot create filename=%q at %q: %s", filename, dirPath, err)
			}
			if err := f.Close(); err != nil {
				t.Fatalf("cannot Close() file=%q: %s", filename, err)
			}
		}
		got := IsPartiallyRemovedDir(dirPath)
		if got != want {
			t.Errorf("unexpected result: got %v, want %v", got, want)

		}
	}
	f("partially_deleted", deleteDirFilename, true)
	f("empty_dir", "", true)
	f("regular_dir", "index.bin", false)
}
