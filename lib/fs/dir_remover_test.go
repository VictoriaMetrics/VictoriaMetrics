package fs

import (
	"fmt"
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

func TestTryRemoveDir(t *testing.T) {
	f := func(setup func(t *testing.T, wd string), want bool) {
		t.Helper()
		d := t.TempDir()
		setup(t, d)
		got := tryRemoveDir(d)
		if got != want {
			t.Fatalf("unexpected error: (-%v;+%v)", want, got)
		}
	}

	writeEmptyFile := func(t *testing.T, filePath string) {
		t.Helper()
		err := os.WriteFile(filePath, []byte("empty"), os.ModePerm)
		if err != nil {
			t.Fatalf("cannot write file: %q: %s", filePath, err)
		}
	}
	// regular delete
	setup := func(t *testing.T, wd string) {
		writeEmptyFile(t, filepath.Join(wd, "metadata.bin"))
		writeEmptyFile(t, filepath.Join(wd, deleteDirFilename))
	}
	f(setup, true)

	// has stale nfs file
	setup = func(t *testing.T, wd string) {
		writeEmptyFile(t, filepath.Join(wd, ".nfs0000"))
		writeEmptyFile(t, filepath.Join(wd, deleteDirFilename))
	}
	f(setup, false)

	// empty dir
	f(func(_ *testing.T, _ string) {}, true)

	// delete many files concurrent
	setup = func(t *testing.T, wd string) {
		for i := range 60 {
			writeEmptyFile(t, filepath.Join(wd, fmt.Sprintf("metadata_%d.bin", i)))
		}
		writeEmptyFile(t, filepath.Join(wd, deleteDirFilename))
	}
	f(setup, true)
}
