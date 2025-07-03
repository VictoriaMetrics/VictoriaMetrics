package remotewrite

import (
	"flag"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	maxUnpackedBlockSize = flagutil.NewBytes("remoteWrite.maxBlockSize", 8*1024*1024, "The maximum block size to send to remote storage. Bigger blocks may improve performance at the cost of the increased memory usage.")
	flushInterval        = flag.Duration("remoteWrite.flushInterval", time.Second, "Interval for flushing the data to remote storage. "+
		"This option takes effect only when less than 2MB of data per second are pushed to -remoteWrite.url")
)

type pendingLogs struct {
	lastFlushTime atomic.Uint64

	// The queue to send blocks to.
	fq *persistentqueue.FastQueue

	// mu protects wr
	mu sync.Mutex
	wr writeRequest

	stopCh            chan struct{}
	periodicFlusherWG sync.WaitGroup
}

func newPendingLogs(fq *persistentqueue.FastQueue) *pendingLogs {
	pl := &pendingLogs{
		fq:     fq,
		stopCh: make(chan struct{}),
	}

	pl.periodicFlusherWG.Add(1)
	go func() {
		defer pl.periodicFlusherWG.Done()
		pl.periodicFlusher()
	}()

	return pl
}

func (pl *pendingLogs) add(lr *logstorage.LogRows) {
	lr.ForEachRow(func(_ uint64, r *logstorage.InsertRow) {
		pl.addLogRow(r)
	})
}

func (pl *pendingLogs) addLogRow(r *logstorage.InsertRow) {
	bb := bbPool.Get()
	bb.B = r.Marshal(bb.B)

	pl.mu.Lock()
	_, _ = pl.wr.pendingData.Write(bb.B)
	pl.wr.pendingLogRowsCount++
	if len(pl.wr.pendingData.B) > maxUnpackedBlockSize.IntN() {
		pl.mustFlushLocked()
	}
	pl.mu.Unlock()
	bbPool.Put(bb)
}

func (pl *pendingLogs) mustFlushLocked() {
	pl.lastFlushTime.Store(fasttime.UnixTimestamp())
	pl.wr.push(func(b []byte) {
		if !pl.fq.TryWriteBlock(b) {
			logger.Fatalf("BUG: TryWriteBlock cannot return false")
		}
	})
	pl.wr.reset()
}

func (pl *pendingLogs) periodicFlusher() {
	flushSeconds := int64(flushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	d := timeutil.AddJitterToDuration(*flushInterval)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-pl.stopCh:
			pl.mu.Lock()
			pl.mustFlushOnStop()
			pl.mu.Unlock()
			return
		case <-ticker.C:
			if fasttime.UnixTimestamp()-pl.lastFlushTime.Load() < uint64(flushSeconds) {
				continue
			}
		}
		pl.mu.Lock()
		pl.mustFlushLocked()
		pl.mu.Unlock()
	}
}

// mustFlushOnStop force pushes wr data
//
// This is needed in order to properly save in-memory data to persistent queue on graceful shutdown.
func (pl *pendingLogs) mustFlushOnStop() {
	pl.wr.push(pl.fq.MustWriteBlockIgnoreDisabledPQ)
	pl.wr.reset()
}

func (pl *pendingLogs) mustStop() {
	close(pl.stopCh)
	pl.periodicFlusherWG.Wait()
}

type writeRequest struct {
	pendingData         bytesutil.ByteBuffer
	pendingLogRowsCount int64
}

func (wr *writeRequest) push(pushBlock func([]byte)) {
	if len(wr.pendingData.B) == 0 {
		return
	}
	b := wr.pendingData.B

	zb := compressBufPool.Get()
	zb.B = zstd.CompressLevel(zb.B[:0], b, 1)
	zbLen := len(zb.B)
	pushBlock(zb.B)
	compressBufPool.Put(zb)
	blockSizeBytes.Update(float64(zbLen))
	blockSizeLogRows.Update(float64(wr.pendingLogRowsCount))
}

func (wr *writeRequest) reset() {
	wr.pendingData.Reset()
	wr.pendingLogRowsCount = 0
}

var (
	blockSizeBytes   = metrics.NewHistogram(`vlagent_remotewrite_block_size_bytes`)
	blockSizeLogRows = metrics.NewHistogram(`vlagent_remotewrite_block_size_rows`)
)

var (
	compressBufPool bytesutil.ByteBufferPool
	bbPool          bytesutil.ByteBufferPool
)
