package fslocal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

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

func TestCopyDestPrefixIntoTmp(t *testing.T) {
	dir := t.TempDir()
	const prefix = "hello-prefix"
	if err := os.WriteFile(filepath.Join(dir, "obj"), []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}
	fs := &FS{Dir: dir, UseTmpFiles: true}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %s", err)
	}
	defer fs.MustStop()

	missing := common.Part{Path: "absent", Offset: 3, FileSize: 6, Size: 3}
	err := fs.CopyDestPrefixIntoTmp(context.Background(), missing)
	if canPreallocate {
		if err == nil {
			t.Fatal("missing dest with offset > 0: expected error")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing dest: %v; want os.ErrNotExist in chain", err)
		}
	} else if err != nil {
		t.Fatalf("missing dest without preallocation: %s", err)
	}

	p := common.Part{
		Path:     "obj",
		FileSize: uint64(len(prefix) + 4),
		Offset:   uint64(len(prefix)),
		Size:     4,
	}
	if !canPreallocate {
		if err := fs.CopyDestPrefixIntoTmp(context.Background(), p); err != nil {
			t.Fatalf("CopyDestPrefixIntoTmp: %s", err)
		}
		return
	}
	if err := fs.PreallocateFile(p); err != nil {
		t.Fatalf("PreallocateFile: %s", err)
	}
	if err := fs.CopyDestPrefixIntoTmp(context.Background(), p); err != nil {
		t.Fatalf("CopyDestPrefixIntoTmp: %s", err)
	}
	got, err := os.ReadFile(fs.tmpPath(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < len(prefix) || string(got[:len(prefix)]) != prefix {
		t.Fatalf("tmp prefix = %q, want prefix %q", got[:len(prefix)], prefix)
	}
}

func TestCopyDestPrefixIntoTmpCanceled(t *testing.T) {
	if !canPreallocate {
		t.Skip("prefix copy is a no-op without preallocation")
	}
	dir := t.TempDir()
	const prefix = "hello-prefix-that-must-not-be-copied"
	if err := os.WriteFile(filepath.Join(dir, "obj"), []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}
	fs := &FS{Dir: dir, UseTmpFiles: true}
	if err := fs.Init(); err != nil {
		t.Fatalf("Init error: %s", err)
	}
	defer fs.MustStop()

	p := common.Part{
		Path:     "obj",
		FileSize: uint64(len(prefix) + 4),
		Offset:   uint64(len(prefix)),
		Size:     4,
	}
	if err := fs.PreallocateFile(p); err != nil {
		t.Fatalf("PreallocateFile: %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := fs.CopyDestPrefixIntoTmp(ctx, p)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyDestPrefixIntoTmp() error = %v; want context.Canceled", err)
	}
	got, readErr := os.ReadFile(fs.tmpPath(p))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if bytes.Contains(got, []byte(prefix)) {
		t.Fatal("canceled prefix copy wrote the destination prefix")
	}
}

func TestCopyPrefixStopsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := &cancelOnRead{cancel: cancel, payload: bytes.Repeat([]byte("x"), prefixCopyChunk*2)}
	var dst bytes.Buffer
	err := copyPrefix(ctx, &dst, src, int64(len(src.payload)), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copyPrefix() error = %v; want context.Canceled", err)
	}
	if src.calls != 1 {
		t.Fatalf("reads after cancel = %d; want 1", src.calls)
	}
	if dst.Len() != prefixCopyChunk {
		t.Fatalf("copied %d bytes; want one chunk (%d)", dst.Len(), prefixCopyChunk)
	}
}

type cancelOnRead struct {
	cancel  context.CancelFunc
	payload []byte
	calls   int
}

func (r *cancelOnRead) Read(p []byte) (int, error) {
	r.calls++
	r.cancel()
	n := copy(p, r.payload)
	r.payload = r.payload[n:]
	return n, nil
}
