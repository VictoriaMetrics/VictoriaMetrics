package fslocal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

func TestFSTemporaryRestoreFile(t *testing.T) {
	for _, prefix := range []string{"", "head"} {
		t.Run(prefix, func(t *testing.T) {
			fs := &FS{Dir: t.TempDir(), UseTmpFiles: true}
			p := common.Part{Path: "data/file", FileSize: uint64(len(prefix) + 4), Offset: uint64(len(prefix)), Size: 4}
			if prefix != "" {
				path := p.LocalPath(fs.Dir)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := fs.PreallocateFile(p); err != nil {
				t.Fatal(err)
			}
			w, err := fs.NewDirectWriteCloser(p)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("tail")); err != nil {
				_ = w.Close()
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(p.LocalPath(fs.Dir)); !os.IsNotExist(err) {
				t.Fatalf("incomplete file is visible at final path: %v", err)
			}
			if err := fs.FinalizeFile(p); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p.LocalPath(fs.Dir))
			if err != nil || !bytes.Equal(got, []byte(prefix+"tail")) {
				t.Fatalf("unexpected restored file: %q; err=%v", got, err)
			}
			tmpPath := filepath.Join(fs.Dir, "incomplete.tmp")
			if err := os.WriteFile(tmpPath, []byte("incomplete"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := fs.CleanupTmpFiles(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
				t.Fatalf("incomplete temporary file wasn't removed: %v", err)
			}
			if _, err := os.Stat(p.LocalPath(fs.Dir)); err != nil {
				t.Fatalf("completed file was removed: %s", err)
			}
		})
	}
}

func TestFSListPartsWithTrailingSlashInDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "testfile"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	// trailing slash must not cause ListParts to panic
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10823
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
