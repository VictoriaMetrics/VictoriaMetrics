package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
)

func TestRestoreKeepsCompletedPrefix(t *testing.T) {
	const (
		prefixStart = "PREFIX_A"
		prefixEnd   = "PREFIX_B"
		tail        = "TAILDATA"
	)
	fileSize := common.MaxPartSize + uint64(len(tail))
	part0 := common.Part{
		Path:     "data.bin",
		FileSize: fileSize,
		Offset:   0,
		Size:     common.MaxPartSize,
	}
	part1 := common.Part{
		Path:     "data.bin",
		FileSize: fileSize,
		Offset:   common.MaxPartSize,
		Size:     uint64(len(tail)),
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	markers := map[int64]string{
		0:                             prefixStart,
		int64(common.MaxPartSize) - 8: prefixEnd,
	}
	writeSizedFile(t, remotePartPath(srcDir, part0), int64(part0.Size), markers)
	writeSizedFile(t, remotePartPath(srcDir, part1), int64(part1.Size), map[int64]string{0: tail})
	writeSizedFile(t, filepath.Join(dstDir, part0.Path), int64(common.MaxPartSize), markers)
	if err := os.WriteFile(filepath.Join(srcDir, backupnames.BackupCompleteFilename), nil, 0600); err != nil {
		t.Fatal(err)
	}

	dstFS := &fslocal.FS{Dir: dstDir}
	if err := dstFS.Init(); err != nil {
		t.Fatalf("Init error: %s", err)
	}
	defer dstFS.MustStop()

	r := &Restore{
		Concurrency: 1,
		Src:         &fsremote.FS{Dir: srcDir},
		Dst:         dstFS,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(dstDir, part0.Path)
	fi, err := os.Stat(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(fileSize) {
		t.Fatalf("restored size = %d, want %d", fi.Size(), fileSize)
	}
	if got := readAt(t, dstPath, 0, len(prefixStart)); got != prefixStart {
		t.Fatalf("prefix start = %q, want %q (a zero-filled prefix means the completed part was discarded)", got, prefixStart)
	}
	if got := readAt(t, dstPath, int64(common.MaxPartSize)-8, len(prefixEnd)); got != prefixEnd {
		t.Fatalf("prefix end = %q, want %q (a zero-filled prefix means the completed part was discarded)", got, prefixEnd)
	}
	if got := readAt(t, dstPath, int64(common.MaxPartSize), len(tail)); got != tail {
		t.Fatalf("restored tail = %q, want %q", got, tail)
	}
}

func remotePartPath(dir string, p common.Part) string {
	name := fmt.Sprintf("%016X_%016X_%016X", p.FileSize, p.Offset, p.Size)
	return filepath.Join(p.LocalPath(dir), name)
}

func writeSizedFile(t *testing.T, path string, size int64, at map[int64]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	for off, s := range at {
		if _, err := f.WriteAt([]byte(s), off); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func readAt(t *testing.T, path string, off int64, n int) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, off); err != nil {
		t.Fatal(err)
	}
	return string(buf)
}
