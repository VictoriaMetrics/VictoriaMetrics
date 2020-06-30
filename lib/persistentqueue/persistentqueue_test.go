package persistentqueue

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestQueueOpenClose(t *testing.T) {
	path := "queue-open-close"
	mustDeleteDir(path)
	for i := 0; i < 3; i++ {
		q := MustOpen(path, "foobar", 0)
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("pending bytes must be 0; got %d", n)
		}
		q.MustClose()
	}
	mustDeleteDir(path)
}

func TestQueueOpen(t *testing.T) {
	t.Run("invalid-metainfo", func(t *testing.T) {
		path := "queue-open-invalid-metainfo"
		mustCreateDir(path)
		mustCreateFile(path+"/metainfo.json", "foobarbaz")
		q := MustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("junk-files-and-dirs", func(t *testing.T) {
		path := "queue-open-junk-files-and-dir"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(path+"/junk-file", "foobar")
		mustCreateDir(path + "/junk-dir")
		q := MustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-chunk-offset", func(t *testing.T) {
		path := "queue-open-invalid-chunk-offset"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 1234), "qwere")
		q := MustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-new-chunk", func(t *testing.T) {
		path := "queue-open-too-new-chunk"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 100*uint64(defaultChunkFileSize)), "asdf")
		q := MustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-old-chunk", func(t *testing.T) {
		path := "queue-open-too-old-chunk"
		mustCreateDir(path)
		mi := &metainfo{
			Name:         "foobar",
			ReaderOffset: defaultChunkFileSize,
			WriterOffset: defaultChunkFileSize,
		}
		if err := mi.WriteToFile(path + "/metainfo.json"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 0), "adfsfd")
		q := MustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("too-big-reader-offset", func(t *testing.T) {
		path := "queue-open-too-big-reader-offset"
		mustCreateDir(path)
		mi := &metainfo{
			Name:         "foobar",
			ReaderOffset: defaultChunkFileSize + 123,
		}
		if err := mi.WriteToFile(path + "/metainfo.json"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		q := MustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("metainfo-dir", func(t *testing.T) {
		path := "queue-open-metainfo-dir"
		mustCreateDir(path)
		mustCreateDir(path + "/metainfo.json")
		q := MustOpen(path, "foobar", 0)
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
		if err := mi.WriteToFile(path + "/metainfo.json"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 0), "sdf")
		q := MustOpen(path, mi.Name, 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-writer-file-size", func(t *testing.T) {
		path := "too-small-reader-file"
		mustCreateDir(path)
		mustCreateEmptyMetainfo(path, "foobar")
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 0), "sdfdsf")
		q := MustOpen(path, "foobar", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
	t.Run("invalid-queue-name", func(t *testing.T) {
		path := "invalid-queue-name"
		mustCreateDir(path)
		mi := &metainfo{
			Name: "foobar",
		}
		if err := mi.WriteToFile(path + "/metainfo.json"); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		mustCreateFile(fmt.Sprintf("%s/%016X", path, 0), "sdf")
		q := MustOpen(path, "baz", 0)
		q.MustClose()
		mustDeleteDir(path)
	})
}

func TestQueueResetIfEmpty(t *testing.T) {
	path := "queue-reset-if-empty"
	mustDeleteDir(path)
	q := MustOpen(path, "foobar", 0)
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
			buf, ok = q.MustReadBlock(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=false returned from MustReadBlock")
			}
		}
		q.ResetIfEmpty()
		if n := q.GetPendingBytes(); n > 0 {
			t.Fatalf("unexpected non-zer pending bytes after queue reset: %d", n)
		}
	}
}

func TestQueueWriteRead(t *testing.T) {
	path := "queue-write-read"
	mustDeleteDir(path)
	q := MustOpen(path, "foobar", 0)
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
			buf, ok = q.MustReadBlock(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=%v returned from MustReadBlock; want true", ok)
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
	q := MustOpen(path, "foobar", 0)
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
		q = MustOpen(path, "foobar", 0)
		if n := q.GetPendingBytes(); n <= 0 {
			t.Fatalf("pending bytes must be greater than 0; got %d", n)
		}
		var buf []byte
		var ok bool
		for _, block := range blocks {
			buf, ok = q.MustReadBlock(buf[:0])
			if !ok {
				t.Fatalf("unexpected ok=%v returned from MustReadBlock; want true", ok)
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

func TestQueueReadEmpty(t *testing.T) {
	path := "queue-read-empty"
	mustDeleteDir(path)
	q := MustOpen(path, "foobar", 0)
	defer mustDeleteDir(path)

	resultCh := make(chan error)
	go func() {
		data, ok := q.MustReadBlock(nil)
		var err error
		if ok {
			err = fmt.Errorf("unexpected ok=%v returned from MustReadBlock; want false", ok)
		} else if len(data) > 0 {
			err = fmt.Errorf("unexpected non-empty data returned from MustReadBlock: %q", data)
		}
		resultCh <- err
	}()
	if n := q.GetPendingBytes(); n > 0 {
		t.Fatalf("pending bytes must be 0; got %d", n)
	}
	q.MustClose()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
}

func TestQueueReadWriteConcurrent(t *testing.T) {
	path := "queue-read-write-concurrent"
	mustDeleteDir(path)
	q := MustOpen(path, "foobar", 0)
	defer mustDeleteDir(path)

	blocksMap := make(map[string]bool, 1000)
	var blocksMapLock sync.Mutex
	blocks := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		block := fmt.Sprintf("block #%d", i)
		blocksMap[block] = true
		blocks[i] = block
	}

	// Start block readers
	var readersWG sync.WaitGroup
	for workerID := 0; workerID < 10; workerID++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			for {
				block, ok := q.MustReadBlock(nil)
				if !ok {
					return
				}
				blocksMapLock.Lock()
				if !blocksMap[string(block)] {
					panic(fmt.Errorf("unexpected block read: %q", block))
				}
				delete(blocksMap, string(block))
				blocksMapLock.Unlock()
			}
		}()
	}

	// Start block writers
	blocksCh := make(chan string)
	var writersWG sync.WaitGroup
	for workerID := 0; workerID < 10; workerID++ {
		writersWG.Add(1)
		go func(workerID int) {
			defer writersWG.Done()
			for block := range blocksCh {
				q.MustWriteBlock([]byte(block))
			}
		}(workerID)
	}
	for _, block := range blocks {
		blocksCh <- block
	}
	close(blocksCh)

	// Wait for block writers to finish
	writersWG.Wait()

	// Notify readers that the queue is closed
	q.MustClose()

	// Wait for block readers to finish
	readersWG.Wait()

	// Read the remaining blocks in q.
	q = MustOpen(path, "foobar", 0)
	defer q.MustClose()
	resultCh := make(chan error)
	go func() {
		for len(blocksMap) > 0 {
			block, ok := q.MustReadBlock(nil)
			if !ok {
				resultCh <- fmt.Errorf("unexpected ok=false returned from MustReadBlock")
				return
			}
			if !blocksMap[string(block)] {
				resultCh <- fmt.Errorf("unexpected block read from the queue: %q", block)
				return
			}
			delete(blocksMap, string(block))
		}
		resultCh <- nil
	}()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
	if n := q.GetPendingBytes(); n > 0 {
		t.Fatalf("pending bytes must be 0; got %d", n)
	}
}

func TestQueueChunkManagementSimple(t *testing.T) {
	path := "queue-chunk-management-simple"
	mustDeleteDir(path)
	const chunkFileSize = 100
	const maxBlockSize = 20
	q := mustOpen(path, "foobar", chunkFileSize, maxBlockSize, 0)
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
		data, ok := q.MustReadBlock(nil)
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
	q := mustOpen(path, "foobar", chunkFileSize, maxBlockSize, 0)
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
		q = mustOpen(path, "foobar", chunkFileSize, maxBlockSize, 0)
	}
	if n := q.GetPendingBytes(); n == 0 {
		t.Fatalf("unexpected zero number of bytes pending")
	}
	for _, block := range blocks {
		data, ok := q.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if block != string(data) {
			t.Fatalf("unexpected block read; got %q; want %q", data, block)
		}
		q.MustClose()
		q = mustOpen(path, "foobar", chunkFileSize, maxBlockSize, 0)
	}
	if n := q.GetPendingBytes(); n != 0 {
		t.Fatalf("unexpected non-zero number of pending bytes: %d", n)
	}
}

func TestQueueLimitedSize(t *testing.T) {
	const maxPendingBytes = 1000
	path := "queue-limited-size"
	mustDeleteDir(path)
	q := MustOpen(path, "foobar", maxPendingBytes)
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
		buf, ok = q.MustReadBlock(buf[:0])
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
	buf, ok = q.MustReadBlock(buf[:0])
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
	if err := ioutil.WriteFile(path, []byte(contents), 0600); err != nil {
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
	if err := mi.WriteToFile(path + "/metainfo.json"); err != nil {
		panic(fmt.Errorf("cannot create metainfo: %w", err))
	}
}
