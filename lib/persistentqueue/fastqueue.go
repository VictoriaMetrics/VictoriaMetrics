package persistentqueue

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// FastQueue is a wrapper around Queue, which prefers sending data via memory.
//
// It falls back to sending data via file when readers don't catch up with writers.
type FastQueue struct {
	// my protects the state of FastQueue.
	mu sync.Mutex

	// cond is used for notifying blocked readers when new data has been added
	// or when MustClose is called.
	cond sync.Cond

	// pq is file-based queue
	pq *Queue

	// ch is in-memory queue
	ch chan *bytesutil.ByteBuffer

	pendingInmemoryBytes uint64

	mustStop bool
}

// MustOpenFastQueue opens persistent queue at the given path.
//
// It holds up to maxInmemoryBlocks in memory before falling back to file-based persistence.
//
// if maxPendingBytes is 0, then the queue size is unlimited.
// Otherwise its size is limited by maxPendingBytes. The oldest data is dropped when the queue
// reaches maxPendingSize.
func MustOpenFastQueue(path, name string, maxInmemoryBlocks, maxPendingBytes int) *FastQueue {
	pq := MustOpen(path, name, maxPendingBytes)
	fq := &FastQueue{
		pq: pq,
		ch: make(chan *bytesutil.ByteBuffer, maxInmemoryBlocks),
	}
	fq.cond.L = &fq.mu
	logger.Infof("opened fast persistent queue at %q with maxInmemoryBlocks=%d", path, maxInmemoryBlocks)
	return fq
}

// MustClose unblocks all the readers.
//
// It is expected no new writers during and after the call.
func (fq *FastQueue) MustClose() {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	// Unblock blocked readers
	fq.mustStop = true
	fq.cond.Broadcast()

	// flush blocks from fq.ch to fq.pq, so they can be persisted
	fq.flushInmemoryBlocksToFileLocked()

	// Close fq.pq
	fq.pq.MustClose()

	logger.Infof("closed fast persistent queue at %q", fq.pq.dir)
}

func (fq *FastQueue) flushInmemoryBlocksToFileLocked() {
	// fq.mu must be locked by the caller.
	for len(fq.ch) > 0 {
		bb := <-fq.ch
		fq.pq.MustWriteBlock(bb.B)
		fq.pendingInmemoryBytes -= uint64(len(bb.B))
		blockBufPool.Put(bb)
	}
	// Unblock all the potentially blocked readers, so they could proceed with reading file-based queue.
	fq.cond.Broadcast()
}

// GetPendingBytes returns the number of pending bytes in the fq.
func (fq *FastQueue) GetPendingBytes() uint64 {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	n := fq.pendingInmemoryBytes
	n += fq.pq.GetPendingBytes()
	return n
}

// GetInmemoryQueueLen returns the length of inmemory queue.
func (fq *FastQueue) GetInmemoryQueueLen() int {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	return len(fq.ch)
}

// MustWriteBlock writes block to fq.
func (fq *FastQueue) MustWriteBlock(block []byte) {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	if n := fq.pq.GetPendingBytes(); n > 0 {
		// The file-based queue isn't drained yet. This means that in-memory queue cannot be used yet.
		// So put the block to file-based queue.
		if len(fq.ch) > 0 {
			logger.Panicf("BUG: the in-memory queue must be empty when the file-based queue is non-empty; it contains %d pending bytes", n)
		}
		fq.pq.MustWriteBlock(block)
		return
	}
	if len(fq.ch) == cap(fq.ch) {
		// There is no space in the in-memory queue. Put the data to file-based queue.
		fq.flushInmemoryBlocksToFileLocked()
		fq.pq.MustWriteBlock(block)
		return
	}
	// There is enough space in the in-memory queue.
	bb := blockBufPool.Get()
	bb.B = append(bb.B[:0], block...)
	fq.ch <- bb
	fq.pendingInmemoryBytes += uint64(len(block))

	// Notify potentially blocked reader.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/484 for the context.
	fq.cond.Signal()
}

// MustReadBlock reads the next block from fq to dst and returns it.
func (fq *FastQueue) MustReadBlock(dst []byte) ([]byte, bool) {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	for {
		if fq.mustStop {
			return dst, false
		}
		if len(fq.ch) > 0 {
			if n := fq.pq.GetPendingBytes(); n > 0 {
				logger.Panicf("BUG: the file-based queue must be empty when the inmemory queue is non-empty; it contains %d pending bytes", n)
			}
			bb := <-fq.ch
			fq.pendingInmemoryBytes -= uint64(len(bb.B))
			dst = append(dst, bb.B...)
			blockBufPool.Put(bb)
			return dst, true
		}
		if n := fq.pq.GetPendingBytes(); n > 0 {
			return fq.pq.MustReadBlock(dst)
		}

		// There are no blocks. Wait for new block.
		fq.pq.ResetIfEmpty()
		fq.cond.Wait()
	}
}
