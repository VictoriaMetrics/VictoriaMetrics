package persistentqueue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestFastQueueOpenClose(t *testing.T) {
	path := "fast-queue-open-close"
	mustDeleteDir(path)
	for i := 0; i < 10; i++ {
		fq := MustOpenFastQueue(path, "foobar", 100, 0)
		fq.MustClose()
	}
	mustDeleteDir(path)
}

func TestFastQueueWriteReadInmemory(t *testing.T) {
	path := "fast-queue-write-read-inmemory"
	mustDeleteDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0)
	if n := fq.GetInmemoryQueueLen(); n != 0 {
		t.Fatalf("unexpected non-zero inmemory queue size:  %d", n)
	}
	var blocks []string
	for i := 0; i < capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		fq.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
	}
	if n := fq.GetInmemoryQueueLen(); n != capacity {
		t.Fatalf("unexpected size of inmemory queue; got %d; want %d", n, capacity)
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	fq.MustClose()
	mustDeleteDir(path)
}

func TestFastQueueWriteReadMixed(t *testing.T) {
	path := "fast-queue-write-read-mixed"
	mustDeleteDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0)
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	var blocks []string
	for i := 0; i < 2*capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		fq.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
	}
	if n := fq.GetPendingBytes(); n == 0 {
		t.Fatalf("the number of pending bytes must be greater than 0")
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	fq.MustClose()
	mustDeleteDir(path)
}

func TestFastQueueWriteReadWithCloses(t *testing.T) {
	path := "fast-queue-write-read-with-closes"
	mustDeleteDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0)
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	var blocks []string
	for i := 0; i < 2*capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		fq.MustWriteBlock([]byte(block))
		blocks = append(blocks, block)
		fq.MustClose()
		fq = MustOpenFastQueue(path, "foobar", capacity, 0)
	}
	if n := fq.GetPendingBytes(); n == 0 {
		t.Fatalf("the number of pending bytes must be greater than 0")
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
		fq.MustClose()
		fq = MustOpenFastQueue(path, "foobar", capacity, 0)
	}
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	fq.MustClose()
	mustDeleteDir(path)
}

func TestFastQueueReadUnblockByClose(t *testing.T) {
	path := "fast-queue-read-unblock-by-close"
	mustDeleteDir(path)

	fq := MustOpenFastQueue(path, "foorbar", 123, 0)
	resultCh := make(chan error)
	go func() {
		data, ok := fq.MustReadBlock(nil)
		if ok {
			resultCh <- fmt.Errorf("unexpected ok=true")
			return
		}
		if len(data) != 0 {
			resultCh <- fmt.Errorf("unexpected non-empty data=%q", data)
			return
		}
		resultCh <- nil
	}()
	fq.MustClose()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	mustDeleteDir(path)
}

func TestFastQueueReadUnblockByWrite(t *testing.T) {
	path := "fast-queue-read-unblock-by-write"
	mustDeleteDir(path)

	fq := MustOpenFastQueue(path, "foobar", 13, 0)
	block := "foodsafdsaf sdf"
	resultCh := make(chan error)
	go func() {
		data, ok := fq.MustReadBlock(nil)
		if !ok {
			resultCh <- fmt.Errorf("unexpected ok=false")
			return
		}
		if string(data) != block {
			resultCh <- fmt.Errorf("unexpected block read; got %q; want %q", data, block)
			return
		}
		resultCh <- nil
	}()
	fq.MustWriteBlock([]byte(block))
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	fq.MustClose()
	mustDeleteDir(path)
}

func TestFastQueueReadWriteConcurrent(t *testing.T) {
	path := "fast-queue-read-write-concurrent"
	mustDeleteDir(path)

	fq := MustOpenFastQueue(path, "foobar", 5, 0)

	var blocks []string
	blocksMap := make(map[string]bool)
	var blocksMapLock sync.Mutex
	for i := 0; i < 1000; i++ {
		block := fmt.Sprintf("block %d", i)
		blocks = append(blocks, block)
		blocksMap[block] = true
	}

	// Start readers
	var readersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			for {
				data, ok := fq.MustReadBlock(nil)
				if !ok {
					return
				}
				blocksMapLock.Lock()
				if !blocksMap[string(data)] {
					panic(fmt.Errorf("unexpected data read from the queue: %q", data))
				}
				delete(blocksMap, string(data))
				blocksMapLock.Unlock()
			}
		}()
	}

	// Start writers
	blocksCh := make(chan string)
	var writersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		writersWG.Add(1)
		go func() {
			defer writersWG.Done()
			for block := range blocksCh {
				fq.MustWriteBlock([]byte(block))
			}
		}()
	}

	// feed writers
	for _, block := range blocks {
		blocksCh <- block
	}
	close(blocksCh)

	// Wait for writers to finish
	writersWG.Wait()

	// wait for a while, so readers could catch up
	time.Sleep(100 * time.Millisecond)

	// Close fq
	fq.MustClose()

	// Wait for readers to finish
	readersWG.Wait()

	// Collect the remaining data
	fq = MustOpenFastQueue(path, "foobar", 5, 0)
	resultCh := make(chan error)
	go func() {
		for len(blocksMap) > 0 {
			data, ok := fq.MustReadBlock(nil)
			if !ok {
				resultCh <- fmt.Errorf("unexpected ok=false")
				return
			}
			if !blocksMap[string(data)] {
				resultCh <- fmt.Errorf("unexpected data read from fq: %q", data)
				return
			}
			delete(blocksMap, string(data))
		}
		resultCh <- nil
	}()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout")
	}
	fq.MustClose()
	mustDeleteDir(path)
}
