package persistentqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestQueueOpenClose(t *testing.T) {
	path := "queue-open-close"
	mustDeleteDir(path)
	for i := 0; i < 3; i++ {
		q := mustOpen(path, "foobar", 0)
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("pending bytes must be 0; got %d", n)
		}
		q.MustClose()
	}
	mustDeleteDir(path)
}

func TestQueueOpen(t *testing.T) {
	t.Run("invalid-metainfo", func(_ *testing.T) {
		path := "queue-open-invalid-metainfo"
		mustCreateDir(path)
		mustCreateFile(filepath.Join(path, metainfoFilename), "foobarbaz")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("junk-files-and-dirs", func(_ *testing.T) {
		path := "queue-open-junk-files-and-dir"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, "junk-file"), "foobar")
		mustCreateDir(filepath.Join(path, "junk-dir"))
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-chunk-offset", func(_ *testing.T) {
		path := "queue-open-invalid-chunk-offset"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 1234)), "qwere")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-new-chunk", func(_ *testing.T) {
		path := "queue-open-too-new-chunk"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 100*uint64(DefaultChunkFileSize))), "asdf")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-old-chunk", func(t *testing.T) {
		path := "queue-open-too-old-chunk"
		mustCreateDir(path)
		mi := &metainfo{
			Name:         "foobar",
			ReaderOffset: DefaultChunkFileSize,
			WriterOffset: DefaultChunkFileSize,
		}
		if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 0)), "adfsfd")
		q := mustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-big-reader-offset", func(t *testing.T) {
		path := "queue-open-too-big-reader-offset"
		mustCreateDir(path)
		mi := &metainfo{
			Name:         "foobar",
			ReaderOffset: DefaultChunkFileSize + 123,
		}
		if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		q := mustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("metainfo-dir", func(_ *testing.T) {
		path := "queue-open-metainfo-dir"
		mustCreateDir(path)
		mustCreateDir(filepath.Join(path, metainfoFilename))
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-small-reader-file", func(t *testing.T) {
		path := "too-small-reader-file"
		mustCreateDir(path)
		mi := &metainfo{
			Name:         "foobar",
			ReaderOffset: 123,
			WriterOffset: 123,
		}
		if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 0)), "sdf")
		q := mustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-writer-file-size", func(_ *testing.T) {
		path := "too-small-reader-file"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 0)), "sdfdsf")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-queue-name", func(t *testing.T) {
		path := "invalid-queue-name"
		mustCreateDir(path)
		mi := &metainfo{
			Name: "foobar",
		}
		if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 0)), "sdf")
		q := mustOpen(path, "baz", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
}

func TestQueueResetIfEmpty(t *testing.T) {
	path := "queue-reset-if-empty"
	mustDeleteDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		mustDeleteDir(path)
	}()

	block := make([]byte, 1024*1024)
	var buf []byte
	for j := 0; j < 10; j++ {
		for i := 0; i < 10; i++ {
			q.MustWriteBlock(block)
			var ok bool
			buf, ok = q.MustReadBlockNonblocking(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=false returned from MustReadBlockNonblocking")
			}
		}
		q.ResetIfEmpty()
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("unexpected non-zero pending bytes after queue reset: %d", n)
		}
		q.ResetIfEmpty()
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("unexpected non-zero pending bytes after queue reset: %d", n)
		}
	}
}

func TestQueueWriteRead(t *testing.T) {
	path := "queue-write-read"
	mustDeleteDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		mustDeleteDir(path)
	}()

	for j := 0; j < 5; j++ {
		var blocks [][]byte
		for i := 0; i < 10; i++ {
			block := []byte(fmt.Sprintf("block %d+%d", j, i))
			q.MustWriteBlock(block)
			blocks = append(blocks, block)
		}
		if n := q.GetPendingBytes(); n <= 0 {
			t.Fatalf("pending bytes must be greater than 0; got %d", n)
		}
		var buf []byte
		var ok bool
		for _, block := range blocks {
			buf, ok = q.MustReadBlockNonblocking(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=%v returned from MustReadBlockNonblocking; want true", ok)
			}
			if string(buf) != string(block) {
				t.Fatalf("unexpected block read; got %q; want %q", buf, block)
			}
		}
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("pending bytes must be 0; got %d", n)
		}
	}
}

func TestQueueWriteCloseRead(t *testing.T) {
	path := "queue-write-close-read"
	mustDeleteDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		mustDeleteDir(path)
	}()

	for j := 0; j < 5; j++ {
		var blocks [][]byte
		for i := 0; i < 10; i++ {
			block := []byte(fmt.Sprintf("block %d+%d", j, i))
			q.MustWriteBlock(block)
			blocks = append(blocks, block)
		}
		if n := q.GetPendingBytes(); n <= 0 {
			t.Fatalf("pending bytes must be greater than 0; got %d", n)
		}
		q.MustClose()
		q = mustOpen(path, "foobar", 0)
		if n := q.GetPendingBytes(); n <= 0 {
			t.Fatalf("pending bytes must be greater than 0; got %d", n)
		}
		var buf []byte
		var ok bool
		for _, block := range blocks {
			buf, ok = q.MustReadBlockNonblocking(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=%v returned from MustReadBlockNonblocking; want true", ok)
			}
			if string(buf) != string(block) {
				t.Fatalf("unexpected block read; got %q; want %q", buf, block)
			}
		}
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("pending bytes must be 0; got %d", n)
		}
	}
}

func TestQueueChunkManagementSimple(t *testing.T) {
	path := "queue-chunk-management-simple"
	mustDeleteDir(path)
	const chunkFileSize = 100
	const maxBlockSize = 20
	q := mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	defer mustDeleteDir(path)
	defer q.MustClose()
	var blocks []string
	for i := 0; i < 100; i++ {
		block := fmt.Sprintf("block %d", i)
		q.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
	}
	if n := q.GetPendingBytes(); n == 0 {
		t.Fatalf("unexpected zero number of bytes pending")
	}
	for _, block := range blocks {
		data, ok := q.MustReadBlockNonblocking(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if block != string(data) {
			t.Fatalf("unexpected block read; got %q; want %q", data, block)
		}
	}
	if n := q.GetPendingBytes(); n != 0 {
		t.Fatalf("unexpected non-zero number of pending bytes: %d", n)
	}
}

func TestQueueChunkManagementPeriodicClose(t *testing.T) {
	path := "queue-chunk-management-periodic-close"
	mustDeleteDir(path)
	const chunkFileSize = 100
	const maxBlockSize = 20
	q := mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	defer func() {
		q.MustClose()
		mustDeleteDir(path)
	}()
	var blocks []string
	for i := 0; i < 100; i++ {
		block := fmt.Sprintf("block %d", i)
		q.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
		q.MustClose()
		q = mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	}
	if n := q.GetPendingBytes(); n == 0 {
		t.Fatalf("unexpected zero number of bytes pending")
	}
	for _, block := range blocks {
		data, ok := q.MustReadBlockNonblocking(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if block != string(data) {
			t.Fatalf("unexpected block read; got %q; want %q", data, block)
		}
		q.MustClose()
		q = mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	}
	if n := q.GetPendingBytes(); n != 0 {
		t.Fatalf("unexpected non-zero number of pending bytes: %d", n)
	}
}

func TestQueueLimitedSize(t *testing.T) {
	const maxPendingBytes = 1000
	path := "queue-limited-size"
	mustDeleteDir(path)
	q := mustOpen(path, "foobar", maxPendingBytes)
	defer func() {
		q.MustClose()
		mustDeleteDir(path)
	}()

	// Check that small blocks are successfully buffered and read
	var blocks []string
	for i := 0; i < 10; i++ {
		block := fmt.Sprintf("block_%d", i)
		q.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
	}
	var buf []byte
	var ok bool
	for _, block := range blocks {
		buf, ok = q.MustReadBlockNonblocking(buf[:0])
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}

	// Make sure that old blocks are dropped on queue size overflow
	for i := 0; i < maxPendingBytes; i++ {
		block := fmt.Sprintf("%d", i)
		q.MustWriteBlock([]byte(block))
	}
	if n := q.GetPendingBytes(); n > maxPendingBytes {
		t.Fatalf("too many pending bytes; got %d; mustn't exceed %d", n, maxPendingBytes)
	}
	buf, ok = q.MustReadBlockNonblocking(buf[:0])
	if !ok {
		t.Fatalf("unexpected ok=false")
	}
	blockNum, err := strconv.Atoi(string(buf))
	if err != nil {
		t.Fatalf("cannot parse block contents: %s", err)
	}
	if blockNum < 20 {
		t.Fatalf("too small block number: %d; it looks like it wasn't dropped", blockNum)
	}

	// Try writing a block with too big size
	block := make([]byte, maxPendingBytes+1)
	q.MustWriteBlock(block)
	if n := q.GetPendingBytes(); n != 0 {
		t.Fatalf("unexpected non-empty queue after writing a block with too big size; queue size: %d bytes", n)
	}
}

func mustCreateFile(path, contents string) {
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		panic(fmt.Errorf("cannot create file %q with %d bytes contents: %w", path, len(contents), err))
	}
}

func mustCreateDir(path string) {
	mustDeleteDir(path)
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(fmt.Errorf("cannot create dir %q: %w", path, err))
	}
}

func mustDeleteDir(path string) {
	if err := os.RemoveAll(path); err != nil {
		panic(fmt.Errorf("cannot remove dir %q: %w", path, err))
	}
}

func mustCreateEmptyMetainfo(path, name string) {
	var mi metainfo
	mi.Name = name
	if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
		panic(fmt.Errorf("cannot create metainfo: %w", err))
	}
}
