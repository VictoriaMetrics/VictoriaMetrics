package remotewrite

import (
	"flag"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var (
	flushInterval = flag.Duration("remoteWrite.flushInterval", time.Second, "Interval for flushing the data to remote storage. "+
		"This option takes effect only when less than 10K data points per second are pushed to -remoteWrite.url")
	maxUnpackedBlockSize = flagutil.NewBytes("remoteWrite.maxBlockSize", 8*1024*1024, "The maximum block size to send to remote storage. Bigger blocks may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxRowsPerBlock")
	maxRowsPerBlock      = flag.Int("remoteWrite.maxRowsPerBlock", 10000, "The maximum number of samples to send in each block to remote storage. Higher number may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxBlockSize")
)

type pendingSeries struct {
	mu sync.Mutex
	wr writeRequest

	stopCh            chan struct{}
	periodicFlusherWG sync.WaitGroup
}

func newPendingSeries(pushBlock func(block []byte), significantFigures, roundDigits int) *pendingSeries {
	var ps pendingSeries
	ps.wr.pushBlock = pushBlock
	ps.wr.significantFigures = significantFigures
	ps.wr.roundDigits = roundDigits
	ps.stopCh = make(chan struct{})
	ps.periodicFlusherWG.Add(1)
	go func() {
		defer ps.periodicFlusherWG.Done()
		ps.periodicFlusher()
	}()
	return &ps
}

func (ps *pendingSeries) MustStop() {
	close(ps.stopCh)
	ps.periodicFlusherWG.Wait()
}

func (ps *pendingSeries) Push(tss []prompbmarshal.TimeSeries) {
	ps.mu.Lock()
	ps.wr.push(tss)
	ps.mu.Unlock()
}

func (ps *pendingSeries) periodicFlusher() {
	flushSeconds := int64(flushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	ticker := time.NewTicker(*flushInterval)
	defer ticker.Stop()
	mustStop := false
	for !mustStop {
		select {
		case <-ps.stopCh:
			mustStop = true
		case <-ticker.C:
			if fasttime.UnixTimestamp()-atomic.LoadUint64(&ps.wr.lastFlushTime) < uint64(flushSeconds) {
				continue
			}
		}
		ps.mu.Lock()
		ps.wr.flush()
		ps.mu.Unlock()
	}
}

type writeRequest struct {
	// Move lastFlushTime to the top of the struct in order to guarantee atomic access on 32-bit architectures.
	lastFlushTime uint64

	// pushBlock is called when whe write request is ready to be sent.
	pushBlock func(block []byte)

	// How many significant figures must be left before sending the writeRequest to pushBlock.
	significantFigures int

	// How many decimal digits after point must be left before sending the writeRequest to pushBlock.
	roundDigits int

	wr prompbmarshal.WriteRequest

	tss []prompbmarshal.TimeSeries

	labels  []prompbmarshal.Label
	samples []prompbmarshal.Sample
	buf     []byte
}

func (wr *writeRequest) reset() {
	// Do not reset pushBlock, significantFigures and roundDigits, since they are re-used.

	wr.wr.Timeseries = nil

	for i := range wr.tss {
		ts := &wr.tss[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	wr.tss = wr.tss[:0]

	promrelabel.CleanLabels(wr.labels)
	wr.labels = wr.labels[:0]

	wr.samples = wr.samples[:0]
	wr.buf = wr.buf[:0]
}

func (wr *writeRequest) flush() {
	wr.wr.Timeseries = wr.tss
	wr.adjustSampleValues()
	atomic.StoreUint64(&wr.lastFlushTime, fasttime.UnixTimestamp())
	pushWriteRequest(&wr.wr, wr.pushBlock)
	wr.reset()
}

func (wr *writeRequest) adjustSampleValues() {
	samples := wr.samples
	if n := wr.significantFigures; n > 0 {
		for i := range samples {
			s := &samples[i]
			s.Value = decimal.RoundToSignificantFigures(s.Value, n)
		}
	}
	if n := wr.roundDigits; n < 100 {
		for i := range samples {
			s := &samples[i]
			s.Value = decimal.RoundToDecimalDigits(s.Value, n)
		}
	}
}

func (wr *writeRequest) push(src []prompbmarshal.TimeSeries) {
	tssDst := wr.tss
	maxSamplesPerBlock := *maxRowsPerBlock
	// Allow up to 10x of labels per each block on average.
	maxLabelsPerBlock := 10 * maxSamplesPerBlock
	for i := range src {
		tssDst = append(tssDst, prompbmarshal.TimeSeries{})
		wr.copyTimeSeries(&tssDst[len(tssDst)-1], &src[i])
		if len(wr.samples) >= maxSamplesPerBlock || len(wr.labels) >= maxLabelsPerBlock {
			wr.tss = tssDst
			wr.flush()
			tssDst = wr.tss
		}
	}
	wr.tss = tssDst
}

func (wr *writeRequest) copyTimeSeries(dst, src *prompbmarshal.TimeSeries) {
	labelsDst := wr.labels
	labelsLen := len(wr.labels)
	samplesDst := wr.samples
	buf := wr.buf
	for i := range src.Labels {
		labelsDst = append(labelsDst, prompbmarshal.Label{})
		dstLabel := &labelsDst[len(labelsDst)-1]
		srcLabel := &src.Labels[i]

		buf = append(buf, srcLabel.Name...)
		dstLabel.Name = bytesutil.ToUnsafeString(buf[len(buf)-len(srcLabel.Name):])
		buf = append(buf, srcLabel.Value...)
		dstLabel.Value = bytesutil.ToUnsafeString(buf[len(buf)-len(srcLabel.Value):])
	}
	dst.Labels = labelsDst[labelsLen:]

	samplesDst = append(samplesDst, src.Samples...)
	dst.Samples = samplesDst[len(samplesDst)-len(src.Samples):]

	wr.samples = samplesDst
	wr.labels = labelsDst
	wr.buf = buf
}

func pushWriteRequest(wr *prompbmarshal.WriteRequest, pushBlock func(block []byte)) {
	if len(wr.Timeseries) == 0 {
		// Nothing to push
		return
	}
	bb := writeRequestBufPool.Get()
	bb.B = prompbmarshal.MarshalWriteRequest(bb.B[:0], wr)
	if len(bb.B) <= maxUnpackedBlockSize.N {
		zb := snappyBufPool.Get()
		zb.B = snappy.Encode(zb.B[:cap(zb.B)], bb.B)
		writeRequestBufPool.Put(bb)
		if len(zb.B) <= persistentqueue.MaxBlockSize {
			pushBlock(zb.B)
			blockSizeRows.Update(float64(len(wr.Timeseries)))
			blockSizeBytes.Update(float64(len(zb.B)))
			snappyBufPool.Put(zb)
			return
		}
		snappyBufPool.Put(zb)
	} else {
		writeRequestBufPool.Put(bb)
	}

	// Too big block. Recursively split it into smaller parts.
	timeseries := wr.Timeseries
	n := len(timeseries) / 2
	wr.Timeseries = timeseries[:n]
	pushWriteRequest(wr, pushBlock)
	wr.Timeseries = timeseries[n:]
	pushWriteRequest(wr, pushBlock)
	wr.Timeseries = timeseries
}

var (
	blockSizeBytes = metrics.NewHistogram(`vmagent_remotewrite_block_size_bytes`)
	blockSizeRows  = metrics.NewHistogram(`vmagent_remotewrite_block_size_rows`)
)

var writeRequestBufPool bytesutil.ByteBufferPool
var snappyBufPool bytesutil.ByteBufferPool
