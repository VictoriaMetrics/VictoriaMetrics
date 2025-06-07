package persistentqueue

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// FastQueue is fast persistent queue, which prefers sending data via memory.
//
// It falls back to sending data via file when readers don't catch up with writers.
type FastQueue struct {
	// mu protects the state of FastQueue.
	mu sync.Mutex

	// cond is used for notifying blocked readers when new data has been added
	// or when MustClose is called.
	cond sync.Cond

	// isPQDisabled is set to true when pq is disabled.
	isPQDisabled bool

	// pq is file-based queue
	pq *queue

	// ch is in-memory queue
	ch chan *bytesutil.ByteBuffer

	pendingInmemoryBytes uint64

	lastInmemoryBlockReadTime uint64

	stopDeadline uint64
}

// MustOpenFastQueue opens persistent queue at the given path.
//
// It holds up to maxInmemoryBlocks in memory before falling back to file-based persistence.
//
// if maxPendingBytes is 0, then the queue size is unlimited.
// Otherwise its size is limited by maxPendingBytes. The oldest data is dropped when the queue
// reaches maxPendingSize.
// if isPQDisabled is set to true, then write requests that exceed in-memory buffer capacity are rejected.
// in-memory queue part can be stored on disk during graceful shutdown.
func MustOpenFastQueue(path, name string, maxInmemoryBlocks int, maxPendingBytes int64, isPQDisabled bool) *FastQueue {
	pq := mustOpen(path, name, maxPendingBytes)
	fq := &FastQueue{
		pq:           pq,
		isPQDisabled: isPQDisabled,
		ch:           make(chan *bytesutil.ByteBuffer, maxInmemoryBlocks),
	}
	fq.cond.L = &fq.mu
	fq.lastInmemoryBlockReadTime = fasttime.UnixTimestamp()
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_persistentqueue_bytes_pending{path=%q}`, path), func() float64 {
		fq.mu.Lock()
		n := fq.pq.GetPendingBytes()
		fq.mu.Unlock()
		return float64(n)
	})

	pendingBytes := fq.GetPendingBytes()
	persistenceStatus := "enabled"
	if isPQDisabled {
		persistenceStatus = "disabled"
	}
	logger.Infof("opened fast queue at %q with maxInmemoryBlocks=%d, it contains %d pending bytes, persistence is %s", path, maxInmemoryBlocks, pendingBytes, persistenceStatus)
	return fq
}

// IsPersistentQueueDisabled returns true if persistend queue at fq is disabled.
func (fq *FastQueue) IsPersistentQueueDisabled() bool {
	return fq.isPQDisabled
}

// IsWriteBlocked checks if data can be pushed into fq
func (fq *FastQueue) IsWriteBlocked() bool {
	if !fq.isPQDisabled {
		return false
	}
	fq.mu.Lock()
	defer fq.mu.Unlock()
	return len(fq.ch) == cap(fq.ch) || fq.pq.GetPendingBytes() > 0
}

// UnblockAllReaders unblocks all the readers.
func (fq *FastQueue) UnblockAllReaders() {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	// Unblock blocked readers
	// Allow for up to 5 seconds for sending Prometheus stale markers.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1526
	fq.stopDeadline = fasttime.UnixTimestamp() + 5
	fq.cond.Broadcast()
}

// MustClose unblocks all the readers.
//
// It is expected no new writers during and after the call.
func (fq *FastQueue) MustClose() {
	fq.UnblockAllReaders()

	fq.mu.Lock()
	defer fq.mu.Unlock()

	// flush blocks from fq.ch to fq.pq, so they can be persisted
	fq.flushInmemoryBlocksToFileLocked()

	// Close fq.pq
	fq.pq.MustClose()

	logger.Infof("closed fast persistent queue at %q", fq.pq.dir)
}

func (fq *FastQueue) flushInmemoryBlocksToFileIfNeededLocked() {
	if len(fq.ch) == 0 || fq.isPQDisabled {
		return
	}
	if fasttime.UnixTimestamp() < fq.lastInmemoryBlockReadTime+5 {
		return
	}
	fq.flushInmemoryBlocksToFileLocked()
}

func (fq *FastQueue) flushInmemoryBlocksToFileLocked() {
	// fq.mu must be locked by the caller.
	for len(fq.ch) > 0 {
		bb := <-fq.ch
		fq.pq.MustWriteBlock(bb.B)
		fq.pendingInmemoryBytes -= uint64(len(bb.B))
		fq.lastInmemoryBlockReadTime = fasttime.UnixTimestamp()
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

// MustWriteBlockIgnoreDisabledPQ unconditionally writes block to fq.
//
// This method allows persisting in-memory blocks during graceful shutdown, even if persistence is disabled.
func (fq *FastQueue) MustWriteBlockIgnoreDisabledPQ(block []byte) {
	if !fq.tryWriteBlock(block, true) {
		logger.Panicf("BUG: tryWriteBlock must always write data even if persistence is disabled")
	}
}

// TryWriteBlock tries writing block to fq.
//
// false is returned if the block couldn't be written to fq when the in-memory queue is full
// and the persistent queue is disabled.
func (fq *FastQueue) TryWriteBlock(block []byte) bool {
	return fq.tryWriteBlock(block, false)
}

// WriteBlock writes block to fq.
func (fq *FastQueue) tryWriteBlock(block []byte, ignoreDisabledPQ bool) bool {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	isPQWriteAllowed := !fq.isPQDisabled || ignoreDisabledPQ

	fq.flushInmemoryBlocksToFileIfNeededLocked()
	if n := fq.pq.GetPendingBytes(); n > 0 {
		// The file-based queue isn't drained yet. This means that in-memory queue cannot be used yet.
		// So put the block to file-based queue.
		if len(fq.ch) > 0 {
			logger.Panicf("BUG: the in-memory queue must be empty when the file-based queue is non-empty; it contains %d pending bytes", n)
		}
		if !isPQWriteAllowed {
			return false
		}
		fq.pq.MustWriteBlock(block)
		return true
	}
	if len(fq.ch) == cap(fq.ch) {
		// There is no space left in the in-memory queue. Put the data to file-based queue.
		if !isPQWriteAllowed {
			return false
		}
		fq.flushInmemoryBlocksToFileLocked()
		fq.pq.MustWriteBlock(block)
		return true
	}
	// Fast path - put the block to in-memory queue.
	bb := blockBufPool.Get()
	bb.B = append(bb.B[:0], block...)
	fq.ch <- bb
	fq.pendingInmemoryBytes += uint64(len(block))

	// Notify potentially blocked reader.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/484 for the context.
	fq.cond.Signal()
	return true
}

// MustReadBlock reads the next block from fq to dst and returns it.
func (fq *FastQueue) MustReadBlock(dst []byte) ([]byte, bool) {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	for {
		if fq.stopDeadline > 0 && fasttime.UnixTimestamp() > fq.stopDeadline {
			return dst, false
		}
		if len(fq.ch) > 0 {
			if n := fq.pq.GetPendingBytes(); n > 0 {
				logger.Panicf("BUG: the file-based queue must be empty when the inmemory queue is non-empty; it contains %d pending bytes", n)
			}
			bb := <-fq.ch
			fq.pendingInmemoryBytes -= uint64(len(bb.B))
			fq.lastInmemoryBlockReadTime = fasttime.UnixTimestamp()
			dst = append(dst, bb.B...)
			blockBufPool.Put(bb)
			return dst, true
		}
		if n := fq.pq.GetPendingBytes(); n > 0 {
			data, ok := fq.pq.MustReadBlockNonblocking(dst)
			if ok {
				return data, true
			}
			dst = data
			continue
		}
		if fq.stopDeadline > 0 {
			return dst, false
		}
		// There are no blocks. Wait for new block.
		fq.pq.ResetIfEmpty()
		fq.cond.Wait()
	}
}

// Dirname returns the directory name for persistent queue.
func (fq *FastQueue) Dirname() string {
	return filepath.Base(fq.pq.dir)
}
