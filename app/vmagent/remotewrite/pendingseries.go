package remotewrite

import (
	"flag"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var (
	flushInterval = flag.Duration("remoteWrite.flushInterval", time.Second, "Interval for flushing the data to remote storage. "+
		"This option takes effect only when less than 10K data points per second are pushed to -remoteWrite.url")
	maxUnpackedBlockSize = flagutil.NewBytes("remoteWrite.maxBlockSize", 8*1024*1024, "The maximum block size to send to remote storage. Bigger blocks may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxRowsPerBlock")
	maxRowsPerBlock      = flag.Int("remoteWrite.maxRowsPerBlock", 10000, "The maximum number of samples to send in each block to remote storage. Higher number may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxBlockSize")
	vmProtoCompressLevel = flag.Int("remoteWrite.vmProtoCompressLevel", 0, "The compression level for VictoriaMetrics remote write protocol. "+
		"Higher values reduce network traffic at the cost of higher CPU usage. Negative values reduce CPU usage at the cost of increased network traffic. "+
		"See https://docs.victoriametrics.com/vmagent/#victoriametrics-remote-write-protocol")
)

type pendingSeries struct {
	mu sync.Mutex
	wr writeRequest

	stopCh            chan struct{}
	periodicFlusherWG sync.WaitGroup
}

func newPendingSeries(fq *persistentqueue.FastQueue, isVMRemoteWrite bool, significantFigures, roundDigits int) *pendingSeries {
	var ps pendingSeries
	ps.wr.fq = fq
	ps.wr.isVMRemoteWrite = isVMRemoteWrite
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

func (ps *pendingSeries) TryPush(tss []prompbmarshal.TimeSeries) bool {
	ps.mu.Lock()
	ok := ps.wr.tryPush(tss)
	ps.mu.Unlock()
	return ok
}

func (ps *pendingSeries) periodicFlusher() {
	flushSeconds := int64(flushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	d := timeutil.AddJitterToDuration(*flushInterval)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ps.stopCh:
			ps.mu.Lock()
			ps.wr.mustFlushOnStop()
			ps.mu.Unlock()
			return
		case <-ticker.C:
			if fasttime.UnixTimestamp()-ps.wr.lastFlushTime.Load() < uint64(flushSeconds) {
				continue
			}
		}
		ps.mu.Lock()
		_ = ps.wr.tryFlush()
		ps.mu.Unlock()
	}
}

type writeRequest struct {
	lastFlushTime atomic.Uint64

	// The queue to send blocks to.
	fq *persistentqueue.FastQueue

	// Whether to encode the write request with VictoriaMetrics remote write protocol.
	isVMRemoteWrite bool

	// How many significant figures must be left before sending the writeRequest to fq.
	significantFigures int

	// How many decimal digits after point must be left before sending the writeRequest to fq.
	roundDigits int

	wr prompbmarshal.WriteRequest

	tss     []prompbmarshal.TimeSeries
	labels  []prompbmarshal.Label
	samples []prompbmarshal.Sample

	// buf holds labels data
	buf []byte
}

func (wr *writeRequest) reset() {
	// Do not reset lastFlushTime, fq, isVMRemoteWrite, significantFigures and roundDigits, since they are re-used.

	wr.wr.Timeseries = nil

	clear(wr.tss)
	wr.tss = wr.tss[:0]

	promrelabel.CleanLabels(wr.labels)
	wr.labels = wr.labels[:0]

	wr.samples = wr.samples[:0]
	wr.buf = wr.buf[:0]
}

// mustFlushOnStop force pushes wr data into wr.fq
//
// This is needed in order to properly save in-memory data to persistent queue on graceful shutdown.
func (wr *writeRequest) mustFlushOnStop() {
	wr.wr.Timeseries = wr.tss
	if !tryPushWriteRequest(&wr.wr, wr.mustWriteBlock, wr.isVMRemoteWrite) {
		logger.Panicf("BUG: final flush must always return true")
	}
	wr.reset()
}

func (wr *writeRequest) mustWriteBlock(block []byte) bool {
	wr.fq.MustWriteBlockIgnoreDisabledPQ(block)
	return true
}

func (wr *writeRequest) tryFlush() bool {
	wr.wr.Timeseries = wr.tss
	wr.lastFlushTime.Store(fasttime.UnixTimestamp())
	if !tryPushWriteRequest(&wr.wr, wr.fq.TryWriteBlock, wr.isVMRemoteWrite) {
		return false
	}
	wr.reset()
	return true
}

func adjustSampleValues(samples []prompbmarshal.Sample, significantFigures, roundDigits int) {
	if n := significantFigures; n > 0 {
		for i := range samples {
			s := &samples[i]
			s.Value = decimal.RoundToSignificantFigures(s.Value, n)
		}
	}
	if n := roundDigits; n < 100 {
		for i := range samples {
			s := &samples[i]
			s.Value = decimal.RoundToDecimalDigits(s.Value, n)
		}
	}
}

func (wr *writeRequest) tryPush(src []prompbmarshal.TimeSeries) bool {
	tssDst := wr.tss
	maxSamplesPerBlock := *maxRowsPerBlock
	// Allow up to 10x of labels per each block on average.
	maxLabelsPerBlock := 10 * maxSamplesPerBlock
	for i := range src {
		if len(wr.samples) >= maxSamplesPerBlock || len(wr.labels) >= maxLabelsPerBlock {
			wr.tss = tssDst
			if !wr.tryFlush() {
				return false
			}
			tssDst = wr.tss
		}
		tsSrc := &src[i]
		adjustSampleValues(tsSrc.Samples, wr.significantFigures, wr.roundDigits)
		tssDst = append(tssDst, prompbmarshal.TimeSeries{})
		wr.copyTimeSeries(&tssDst[len(tssDst)-1], tsSrc)
	}

	wr.tss = tssDst
	return true
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

// marshalConcurrency limits the maximum number of concurrent workers, which marshal and compress WriteRequest.
var marshalConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

func tryPushWriteRequest(wr *prompbmarshal.WriteRequest, tryPushBlock func(block []byte) bool, isVMRemoteWrite bool) bool {
	if len(wr.Timeseries) == 0 {
		// Nothing to push
		return true
	}

	marshalConcurrencyCh <- struct{}{}

	bb := writeRequestBufPool.Get()
	bb.B = wr.MarshalProtobuf(bb.B[:0])
	if len(bb.B) <= maxUnpackedBlockSize.IntN() {
		zb := compressBufPool.Get()
		if isVMRemoteWrite {
			zb.B = zstd.CompressLevel(zb.B[:0], bb.B, *vmProtoCompressLevel)
		} else {
			zb.B = snappy.Encode(zb.B[:cap(zb.B)], bb.B)
		}
		writeRequestBufPool.Put(bb)

		<-marshalConcurrencyCh

		if len(zb.B) <= persistentqueue.MaxBlockSize {
			zbLen := len(zb.B)
			ok := tryPushBlock(zb.B)
			compressBufPool.Put(zb)
			if ok {
				blockSizeRows.Update(float64(len(wr.Timeseries)))
				blockSizeBytes.Update(float64(zbLen))
			}
			return ok
		}
		compressBufPool.Put(zb)
	} else {
		writeRequestBufPool.Put(bb)

		<-marshalConcurrencyCh
	}

	// Too big block. Recursively split it into smaller parts if possible.
	if len(wr.Timeseries) == 1 {
		// A single time series left. Recursively split its samples into smaller parts if possible.
		samples := wr.Timeseries[0].Samples
		if len(samples) == 1 {
			logger.Warnf("dropping a sample for metric with too long labels exceeding -remoteWrite.maxBlockSize=%d bytes", maxUnpackedBlockSize.N)
			return true
		}
		n := len(samples) / 2
		wr.Timeseries[0].Samples = samples[:n]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries[0].Samples = samples
			return false
		}
		wr.Timeseries[0].Samples = samples[n:]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries[0].Samples = samples
			return false
		}
		wr.Timeseries[0].Samples = samples
		return true
	}
	timeseries := wr.Timeseries
	n := len(timeseries) / 2
	wr.Timeseries = timeseries[:n]
	if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
		wr.Timeseries = timeseries
		return false
	}
	wr.Timeseries = timeseries[n:]
	if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
		wr.Timeseries = timeseries
		return false
	}
	wr.Timeseries = timeseries
	return true
}

var (
	blockSizeBytes = metrics.NewHistogram(`vmagent_remotewrite_block_size_bytes`)
	blockSizeRows  = metrics.NewHistogram(`vmagent_remotewrite_block_size_rows`)
)

var (
	writeRequestBufPool bytesutil.ByteBufferPool
	compressBufPool     bytesutil.ByteBufferPool
)
