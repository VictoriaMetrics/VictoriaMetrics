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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var (
	flushInterval = flag.Duration("remoteWrite.flushInterval", time.Second, "Interval for flushing the data to remote storage. "+
		"This option takes effect only when less than -remoteWrite.maxRowsPerBlock data points per -remoteWrite.flushInterval are pushed to -remoteWrite.url")
	maxUnpackedBlockSize = flagutil.NewBytes("remoteWrite.maxBlockSize", 8*1024*1024, "The maximum block size to send to remote storage. Bigger blocks may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxRowsPerBlock")
	maxRowsPerBlock      = flag.Int("remoteWrite.maxRowsPerBlock", 10000, "The maximum number of samples to send in each block to remote storage. Higher number may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxBlockSize")
	maxMetadataPerBlock  = flag.Int("remoteWrite.maxMetadataPerBlock", 5000, "The maximum number of metadata to send in each block to remote storage. Higher number may improve performance at the cost of the increased memory usage. See also -remoteWrite.maxBlockSize")
	vmProtoCompressLevel = flag.Int("remoteWrite.vmProtoCompressLevel", 0, "The compression level for VictoriaMetrics remote write protocol. "+
		"Higher values reduce network traffic at the cost of higher CPU usage. Negative values reduce CPU usage at the cost of increased network traffic. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmagent/#victoriametrics-remote-write-protocol")
)

type pendingSeries struct {
	mu sync.Mutex
	wr writeRequest

	stopCh            chan struct{}
	periodicFlusherWG sync.WaitGroup
}

func newPendingSeries(fq *persistentqueue.FastQueue, isVMRemoteWrite *atomic.Bool, significantFigures, roundDigits int) *pendingSeries {
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

func (ps *pendingSeries) TryPushTimeSeries(tss []prompb.TimeSeries) bool {
	ps.mu.Lock()
	ok := ps.wr.tryPushTimeSeries(tss)
	ps.mu.Unlock()
	return ok
}

func (ps *pendingSeries) TryPushMetadata(mms []prompb.MetricMetadata) bool {
	ps.mu.Lock()
	ok := ps.wr.tryPushMetadata(mms)
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
	isVMRemoteWrite *atomic.Bool

	// How many significant figures must be left before sending the writeRequest to fq.
	significantFigures int

	// How many decimal digits after point must be left before sending the writeRequest to fq.
	roundDigits int

	wr prompb.WriteRequest

	tss     []prompb.TimeSeries
	mms     []prompb.MetricMetadata
	labels  []prompb.Label
	samples []prompb.Sample

	// buf holds labels data
	buf []byte
	// metadatabuf holds metadata data
	metadatabuf []byte
}

func (wr *writeRequest) reset() {
	// Do not reset lastFlushTime, fq, isVMRemoteWrite, significantFigures and roundDigits, since they are reused.

	wr.wr.Timeseries = nil
	wr.wr.Metadata = nil

	clear(wr.tss)
	wr.tss = wr.tss[:0]

	clear(wr.mms)
	wr.mms = wr.mms[:0]

	promrelabel.CleanLabels(wr.labels)
	wr.labels = wr.labels[:0]

	wr.samples = wr.samples[:0]
	wr.buf = wr.buf[:0]
	wr.metadatabuf = wr.metadatabuf[:0]
}

// mustFlushOnStop force pushes wr data into wr.fq
//
// This is needed in order to properly save in-memory data to persistent queue on graceful shutdown.
func (wr *writeRequest) mustFlushOnStop() {
	wr.wr.Timeseries = wr.tss
	wr.wr.Metadata = wr.mms
	if !tryPushWriteRequest(&wr.wr, wr.mustWriteBlock, wr.isVMRemoteWrite.Load()) {
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
	wr.wr.Metadata = wr.mms
	wr.lastFlushTime.Store(fasttime.UnixTimestamp())
	if !tryPushWriteRequest(&wr.wr, wr.fq.TryWriteBlock, wr.isVMRemoteWrite.Load()) {
		return false
	}
	wr.reset()
	return true
}

func adjustSampleValues(samples []prompb.Sample, significantFigures, roundDigits int) {
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

func (wr *writeRequest) tryPushMetadata(mms []prompb.MetricMetadata) bool {
	mmdDst := wr.mms
	maxMetadataPerBlock := *maxMetadataPerBlock
	for i := range mms {
		if len(wr.mms) >= maxMetadataPerBlock {
			if !wr.tryFlush() {
				return false
			}
			mmdDst = wr.mms
		}
		mmSrc := &mms[i]
		mmdDst = append(mmdDst, prompb.MetricMetadata{})
		wr.copyMetadata(&mmdDst[len(mmdDst)-1], mmSrc)
	}
	wr.mms = mmdDst
	return true
}

func (wr *writeRequest) copyMetadata(dst, src *prompb.MetricMetadata) {
	// Direct copy for non-string fields, which are safe by value.
	dst.Type = src.Type
	dst.Unit = src.Unit

	// Pre-allocate memory for all string fields.
	neededBufLen := len(src.MetricFamilyName) + len(src.Help)
	bufLen := len(wr.metadatabuf)
	wr.metadatabuf = slicesutil.SetLength(wr.metadatabuf, bufLen+neededBufLen)
	buf := wr.metadatabuf[:bufLen]

	// Copy MetricFamilyName
	bufLen = len(buf)
	buf = append(buf, src.MetricFamilyName...)
	dst.MetricFamilyName = bytesutil.ToUnsafeString(buf[bufLen:])

	// Copy Help
	bufLen = len(buf)
	buf = append(buf, src.Help...)
	dst.Help = bytesutil.ToUnsafeString(buf[bufLen:])

	wr.metadatabuf = buf
}

func (wr *writeRequest) tryPushTimeSeries(src []prompb.TimeSeries) bool {
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
		tssDst = append(tssDst, prompb.TimeSeries{})
		wr.copyTimeSeries(&tssDst[len(tssDst)-1], tsSrc)
	}

	wr.tss = tssDst
	return true
}

func (wr *writeRequest) copyTimeSeries(dst, src *prompb.TimeSeries) {
	labelsSrc := src.Labels

	// Pre-allocate memory for labels.
	labelsLen := len(wr.labels)
	wr.labels = slicesutil.SetLength(wr.labels, labelsLen+len(labelsSrc))
	labelsDst := wr.labels[labelsLen:]

	// Pre-allocate memory for byte slice needed for storing label names and values.
	neededBufLen := 0
	for i := range labelsSrc {
		label := &labelsSrc[i]
		neededBufLen += len(label.Name) + len(label.Value)
	}
	bufLen := len(wr.buf)
	wr.buf = slicesutil.SetLength(wr.buf, bufLen+neededBufLen)
	buf := wr.buf[:bufLen]

	// Copy labels
	for i := range labelsSrc {
		dstLabel := &labelsDst[i]
		srcLabel := &labelsSrc[i]

		bufLen := len(buf)
		buf = append(buf, srcLabel.Name...)
		dstLabel.Name = bytesutil.ToUnsafeString(buf[bufLen:])

		bufLen = len(buf)
		buf = append(buf, srcLabel.Value...)
		dstLabel.Value = bytesutil.ToUnsafeString(buf[bufLen:])
	}
	wr.buf = buf
	dst.Labels = labelsDst

	// Copy samples
	samplesLen := len(wr.samples)
	wr.samples = append(wr.samples, src.Samples...)
	dst.Samples = wr.samples[samplesLen:]
}

// marshalConcurrency limits the maximum number of concurrent workers, which marshal and compress WriteRequest.
var marshalConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

func tryPushWriteRequest(wr *prompb.WriteRequest, tryPushBlock func(block []byte) bool, isVMRemoteWrite bool) bool {
	if wr.IsEmpty() {
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
				blockMetadataRows.Update(float64(len(wr.Metadata)))
				blockSizeBytes.Update(float64(zbLen))
			}
			return ok
		}
		compressBufPool.Put(zb)
	} else {
		writeRequestBufPool.Put(bb)

		<-marshalConcurrencyCh
	}

	// Split timeseries or metadata into two smaller blocks
	switch len(wr.Timeseries) {
	case 0:
		if len(wr.Metadata) == 1 {
			logger.Warnf("dropping a metadata exceeding -remoteWrite.maxBlockSize=%d bytes", maxUnpackedBlockSize.N)
			return true
		}
		metadata := wr.Metadata
		n := len(metadata) / 2
		wr.Metadata = metadata[:n]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Metadata = metadata
			return false
		}
		wr.Metadata = metadata[n:]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Metadata = metadata
			return false
		}
		wr.Metadata = metadata
		return true

	case 1:
		// A single time series left. Recursively split its samples and metadata into smaller parts if possible.
		samples := wr.Timeseries[0].Samples
		metaData := wr.Metadata
		if len(samples) == 1 && len(metaData) <= 1 {
			logger.Warnf("dropping a sample for metric and %d metadata which are exceeding -remoteWrite.maxBlockSize=%d bytes", len(metaData), maxUnpackedBlockSize.N)
			return true
		}
		n := len(samples) / 2
		m := len(metaData) / 2
		wr.Timeseries[0].Samples = samples[:n]
		wr.Metadata = metaData[:m]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries[0].Samples = samples
			wr.Metadata = metaData
			return false
		}
		wr.Timeseries[0].Samples = samples[n:]
		wr.Metadata = metaData[m:]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries[0].Samples = samples
			wr.Metadata = metaData
			return false
		}
		wr.Timeseries[0].Samples = samples
		wr.Metadata = metaData
		return true

	default:
		// Split both timeseries and metadata.
		timeseries := wr.Timeseries
		metaData := wr.Metadata
		n := len(timeseries) / 2
		m := len(metaData) / 2
		wr.Timeseries = timeseries[:n]
		wr.Metadata = metaData[:m]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries = timeseries
			wr.Metadata = metaData
			return false
		}
		wr.Timeseries = timeseries[n:]
		wr.Metadata = metaData[m:]
		if !tryPushWriteRequest(wr, tryPushBlock, isVMRemoteWrite) {
			wr.Timeseries = timeseries
			wr.Metadata = metaData
			return false
		}
		wr.Timeseries = timeseries
		wr.Metadata = metaData
		return true
	}
}

var (
	blockSizeBytes    = metrics.NewHistogram(`vmagent_remotewrite_block_size_bytes`)
	blockSizeRows     = metrics.NewHistogram(`vmagent_remotewrite_block_size_rows`)
	blockMetadataRows = metrics.NewHistogram(`vmagent_remotewrite_block_metadata_rows`)
)

var (
	writeRequestBufPool bytesutil.ByteBufferPool
	compressBufPool     bytesutil.ByteBufferPool
)
