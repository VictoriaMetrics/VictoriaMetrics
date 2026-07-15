package persistentqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestQueueOpenClose(t *testing.T) {
	path := "queue-open-close"
	fs.MustRemoveDir(path)
	for range 3 {
		q := mustOpen(path, "foobar", 0)
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("pending bytes must be 0; got %d", n)
		}
		q.MustClose()
	}
	fs.MustRemoveDir(path)
}

func TestQueueOpen(t *testing.T) {
	t.Run("invalid-metainfo", func(_ *testing.T) {
		path := "queue-open-invalid-metainfo"
		mustCreateDir(path)
		mustCreateFile(filepath.Join(path, metainfoFilename), "foobarbaz")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
	})
	t.Run("junk-files-and-dirs", func(_ *testing.T) {
		path := "queue-open-junk-files-and-dir"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, "junk-file"), "foobar")
		mustCreateDir(filepath.Join(path, "junk-dir"))
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
	})
	t.Run("invalid-chunk-offset", func(_ *testing.T) {
		path := "queue-open-invalid-chunk-offset"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 1234)), "qwere")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
	})
	t.Run("too-new-chunk", func(_ *testing.T) {
		path := "queue-open-too-new-chunk"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 100*uint64(DefaultChunkFileSize))), "asdf")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
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
		fs.MustRemoveDir(path)
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
		fs.MustRemoveDir(path)
	})
	t.Run("metainfo-dir", func(_ *testing.T) {
		path := "queue-open-metainfo-dir"
		mustCreateDir(path)
		mustCreateDir(filepath.Join(path, metainfoFilename))
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
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
		fs.MustRemoveDir(path)
	})
	t.Run("invalid-writer-file-size", func(_ *testing.T) {
		path := "too-small-reader-file"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(filepath.Join(path, fmt.Sprintf("%016X", 0)), "sdfdsf")
		q := mustOpen(path, "foobar", 0)
		q.MustClose()
		fs.MustRemoveDir(path)
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
		fs.MustRemoveDir(path)
	})
	t.Run("damaged-writer-file-tail", func(t *testing.T) {
		path := "damaged-writer-file-tail"
		fs.MustRemoveDir(path)

		q := mustOpen(path, "foobar", 0)
		block1 := []byte("valid block 1")
		block2 := []byte("valid block 2")
		q.MustWriteBlock(block1)
		q.MustWriteBlock(block2)
		q.MustClose()

		chunkPath := filepath.Join(path, fmt.Sprintf("%016X", 0))
		chunkFileSize := fs.MustFileSize(chunkPath)

		if err := os.Truncate(chunkPath, int64(chunkFileSize)-1); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		q = mustOpen(path, "foobar", 0)

		var buf []byte
		var ok bool
		buf, ok = q.MustReadBlockNonblocking(buf[:0])
		if !ok {
			t.Fatalf("expected block to be readable")
		}
		if string(buf) != string(block1) {
			t.Fatalf("unexpected block got: %q, want: %q", buf, block1)
		}

		_, ok = q.MustReadBlockNonblocking(buf[:0])
		if ok {
			t.Fatalf("expected second block to be dropped")
		}
		q.MustClose()

		expectedChunkFileSize := uint64(blockHeaderSize + len(block1))
		chunkFileSize = fs.MustFileSize(chunkPath)
		if chunkFileSize != expectedChunkFileSize {
			t.Fatalf("unexpected chunk file size: got %d; want %d", chunkFileSize, expectedChunkFileSize)
		}

		fs.MustRemoveDir(path)
	})
	t.Run("damaged-writer-file-extra-tail", func(t *testing.T) {
		path := "damaged-writer-file-extra-tail"
		fs.MustRemoveDir(path)

		q := mustOpen(path, "foobar", 0)
		blocks := [][]byte{
			[]byte("valid block 1"),
			[]byte("valid block 2"),
		}
		for _, block := range blocks {
			q.MustWriteBlock(block)
		}
		q.MustClose()

		chunkPath := filepath.Join(path, fmt.Sprintf("%016X", 0))
		originChunkFileSize := fs.MustFileSize(chunkPath)

		f, err := os.OpenFile(chunkPath, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if _, err := f.Write([]byte("corrupted block")); err != nil {
			t.Fatalf("cannot append corrupted tail: %s", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("cannot close chunk file: %s", err)
		}

		q = mustOpen(path, "foobar", 0)

		var buf []byte
		var ok bool
		for _, block := range blocks {
			buf, ok = q.MustReadBlockNonblocking(buf[:0])
			if !ok {
				t.Fatalf("expected block to be readable")
			}
			if string(buf) != string(block) {
				t.Fatalf("unexpected block got: %q, want: %q", buf, block)
			}
		}
		q.MustClose()
		chunkFileSize := fs.MustFileSize(chunkPath)
		if chunkFileSize != originChunkFileSize {
			t.Fatalf("unexpected chunk file size: got %d; want %d", chunkFileSize, originChunkFileSize)
		}
		fs.MustRemoveDir(path)
	})
	t.Run("damaged-writer-file-partial-header", func(t *testing.T) {
		path := "damaged-writer-file-partial-header"
		fs.MustRemoveDir(path)

		q := mustOpen(path, "foobar", 0)
		block1 := []byte("valid block 1")
		block2 := []byte("valid block 2")
		q.MustWriteBlock(block1)
		q.MustWriteBlock(block2)
		q.MustClose()

		chunkPath := filepath.Join(path, fmt.Sprintf("%016X", 0))
		// Keep block1 intact plus only 3 bytes of the second block's header.
		newSize := int64(blockHeaderSize + len(block1) + 3)
		if err := os.Truncate(chunkPath, newSize); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		q = mustOpen(path, "foobar", 0)
		buf, ok := q.MustReadBlockNonblocking(nil)
		if !ok {
			t.Fatalf("expected block to be readable")
		}
		if string(buf) != string(block1) {
			t.Fatalf("unexpected block got: %q, want: %q", buf, block1)
		}
		if _, ok := q.MustReadBlockNonblocking(buf[:0]); ok {
			t.Fatalf("expected second block to be dropped")
		}
		q.MustClose()

		expectedChunkFileSize := uint64(blockHeaderSize + len(block1))
		if chunkFileSize := fs.MustFileSize(chunkPath); chunkFileSize != expectedChunkFileSize {
			t.Fatalf("unexpected chunk file size: got %d; want %d", chunkFileSize, expectedChunkFileSize)
		}
		fs.MustRemoveDir(path)
	})
	t.Run("damaged-writer-file-corrupted-header", func(t *testing.T) {
		path := "damaged-writer-file-corrupted-header"
		fs.MustRemoveDir(path)

		q := mustOpen(path, "foobar", 0)
		block1 := []byte("valid block 1")
		block2 := []byte("valid block 2")
		block3 := []byte("valid block 3")
		q.MustWriteBlock(block1)
		q.MustWriteBlock(block2)
		q.MustWriteBlock(block3)
		q.MustClose()

		chunkPath := filepath.Join(path, fmt.Sprintf("%016X", 0))
		chunkFileSize := fs.MustFileSize(chunkPath)

		// Overwrite the second block's header with an impossible blockLen
		// and drop the last byte so the recovery scan is triggered.
		f, err := os.OpenFile(chunkPath, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		garbage := [blockHeaderSize]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
		if _, err := f.WriteAt(garbage[:], int64(blockHeaderSize+len(block1))); err != nil {
			t.Fatalf("cannot corrupt block header: %s", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("cannot close chunk file: %s", err)
		}
		if err := os.Truncate(chunkPath, int64(chunkFileSize)-1); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		q = mustOpen(path, "foobar", 0)
		buf, ok := q.MustReadBlockNonblocking(nil)
		if !ok {
			t.Fatalf("expected block to be readable")
		}
		if string(buf) != string(block1) {
			t.Fatalf("unexpected block got: %q, want: %q", buf, block1)
		}
		if _, ok := q.MustReadBlockNonblocking(buf[:0]); ok {
			t.Fatalf("expected remaining blocks to be dropped")
		}
		q.MustClose()

		expectedChunkFileSize := uint64(blockHeaderSize + len(block1))
		if chunkFileSize := fs.MustFileSize(chunkPath); chunkFileSize != expectedChunkFileSize {
			t.Fatalf("unexpected chunk file size: got %d; want %d", chunkFileSize, expectedChunkFileSize)
		}
		fs.MustRemoveDir(path)
	})
	t.Run("damaged-writer-file-second-chunk", func(t *testing.T) {
		path := "damaged-writer-file-second-chunk"
		fs.MustRemoveDir(path)

		const maxBlockSize = 64
		const chunkFileSize = (maxBlockSize + blockHeaderSize) * 2

		newBlock := func(fill byte) []byte {
			b := make([]byte, 32)
			for i := range b {
				b[i] = fill
			}
			return b
		}
		block1 := newBlock('1')
		block2 := newBlock('2')
		block3 := newBlock('3')
		block4 := newBlock('4')

		q := mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
		q.MustWriteBlock(block1)
		q.MustWriteBlock(block2)
		// blocks 3 and 4 land in the second chunk file
		q.MustWriteBlock(block3)
		q.MustWriteBlock(block4)
		q.MustClose()

		secondChunkPath := filepath.Join(path, fmt.Sprintf("%016X", chunkFileSize))
		secondChunkSize := fs.MustFileSize(secondChunkPath)
		if err := os.Truncate(secondChunkPath, int64(secondChunkSize)-1); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		q = mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
		var buf []byte
		var ok bool
		for _, block := range [][]byte{block1, block2, block3} {
			buf, ok = q.MustReadBlockNonblocking(buf[:0])
			if !ok {
				t.Fatalf("expected block to be readable")
			}
			if string(buf) != string(block) {
				t.Fatalf("unexpected block got: %q, want: %q", buf, block)
			}
		}
		if _, ok := q.MustReadBlockNonblocking(buf[:0]); ok {
			t.Fatalf("expected last block to be dropped")
		}
		q.MustClose()

		expectedSize := uint64(blockHeaderSize + len(block3))
		if gotSize := fs.MustFileSize(secondChunkPath); gotSize != expectedSize {
			t.Fatalf("unexpected second chunk file size: got %d; want %d", gotSize, expectedSize)
		}
		fs.MustRemoveDir(path)
	})
}

func TestQueueResetIfEmpty(t *testing.T) {
	path := "queue-reset-if-empty"
	fs.MustRemoveDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		fs.MustRemoveDir(path)
	}()

	block := make([]byte, 1024*1024)
	var buf []byte
	for range 10 {
		for range 10 {
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
	fs.MustRemoveDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		fs.MustRemoveDir(path)
	}()

	for j := range 5 {
		var blocks [][]byte
		for i := range 10 {
			block := fmt.Appendf(nil, "block %d+%d", j, i)
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
	fs.MustRemoveDir(path)
	q := mustOpen(path, "foobar", 0)
	defer func() {
		q.MustClose()
		fs.MustRemoveDir(path)
	}()

	for j := range 5 {
		var blocks [][]byte
		for i := range 10 {
			block := fmt.Appendf(nil, "block %d+%d", j, i)
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
	fs.MustRemoveDir(path)
	const chunkFileSize = 100
	const maxBlockSize = 20
	q := mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	defer fs.MustRemoveDir(path)
	defer q.MustClose()
	var blocks []string
	for i := range 100 {
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
	fs.MustRemoveDir(path)
	const chunkFileSize = 100
	const maxBlockSize = 20
	q := mustOpenInternal(path, "foobar", chunkFileSize, maxBlockSize, 0)
	defer func() {
		q.MustClose()
		fs.MustRemoveDir(path)
	}()
	var blocks []string
	for i := range 100 {
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
	fs.MustRemoveDir(path)
	q := mustOpen(path, "foobar", maxPendingBytes)
	defer func() {
		q.MustClose()
		fs.MustRemoveDir(path)
	}()

	// Check that small blocks are successfully buffered and read
	var blocks []string
	for i := range 10 {
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
	for i := range maxPendingBytes {
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
	fs.MustWriteSync(path, []byte(contents))
}

func mustCreateDir(path string) {
	fs.MustRemoveDir(path)
	if err := os.MkdirAll(path, 0700); err != nil {
		panic(fmt.Errorf("cannot create dir %q: %w", path, err))
	}
}

func mustCreateEmptyMetainfo(path, name string) {
	var mi metainfo
	mi.Name = name
	if err := mi.WriteToFile(filepath.Join(path, metainfoFilename)); err != nil {
		panic(fmt.Errorf("cannot create metainfo: %w", err))
	}
}
