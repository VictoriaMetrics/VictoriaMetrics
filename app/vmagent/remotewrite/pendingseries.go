package remotewrite

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
	vmProtoCompressLevel = flag.Int("remoteWrite.vmProtoCompressLevel", 0, "The compression level for VictoriaMetrics remote write protocol. "+
		"Higher values reduce network traffic at the cost of higher CPU usage. Negative values reduce CPU usage at the cost of increased network traffic. "+
		"See https://docs.victoriametrics.com/vmagent.html#victoriametrics-remote-write-protocol")
)

type pendingSeries struct {
	mu sync.Mutex
	wr writeRequest

	stopCh            chan struct{}
	periodicFlusherWG sync.WaitGroup
}

func newPendingSeries(pushBlock func(block []byte) error, isVMRemoteWrite bool, significantFigures, roundDigits int) *pendingSeries {
	var ps pendingSeries
	ps.wr.pushBlock = pushBlock
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

func (ps *pendingSeries) Push(tss []prompbmarshal.TimeSeries) error {
	ps.mu.Lock()
	err := ps.wr.push(tss)
	ps.mu.Unlock()
	return err
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
		// no-op error
		_ = ps.wr.flush()
		ps.mu.Unlock()
	}
}

type writeRequest struct {
	// Move lastFlushTime to the top of the struct in order to guarantee atomic access on 32-bit architectures.
	lastFlushTime uint64

	// pushBlock is called when whe write request is ready to be sent.
	pushBlock func(block []byte) error

	// Whether to encode the write request with VictoriaMetrics remote write protocol.
	isVMRemoteWrite bool

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
	// Do not reset lastFlushTime, pushBlock, isVMRemoteWrite, significantFigures and roundDigits, since they are re-used.

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

func (wr *writeRequest) flush() error {
	wr.wr.Timeseries = wr.tss
	wr.adjustSampleValues()
	atomic.StoreUint64(&wr.lastFlushTime, fasttime.UnixTimestamp())
	err := pushWriteRequest(&wr.wr, wr.pushBlock, wr.isVMRemoteWrite)
	if err != nil {
		return err
	}
	wr.reset()
	return nil
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

func (wr *writeRequest) push(src []prompbmarshal.TimeSeries) error {
	tssDst := wr.tss
	maxSamplesPerBlock := *maxRowsPerBlock
	// Allow up to 10x of labels per each block on average.
	maxLabelsPerBlock := 10 * maxSamplesPerBlock
	for i := range src {
		tssDst = append(tssDst, prompbmarshal.TimeSeries{})
		wr.copyTimeSeries(&tssDst[len(tssDst)-1], &src[i])
		if len(wr.samples) >= maxSamplesPerBlock || len(wr.labels) >= maxLabelsPerBlock {
			wr.tss = tssDst
			if err := wr.flush(); err != nil {
				return err
			}
			tssDst = wr.tss
		}
	}

	wr.tss = tssDst
	return nil
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

func pushWriteRequest(wr *prompbmarshal.WriteRequest, pushBlock func(block []byte) error, isVMRemoteWrite bool) error {
	if len(wr.Timeseries) == 0 {
		// Nothing to push
		return nil
	}
	bb := writeRequestBufPool.Get()
	bb.B = prompbmarshal.MarshalWriteRequest(bb.B[:0], wr)
	if len(bb.B) <= maxUnpackedBlockSize.IntN() {
		zb := snappyBufPool.Get()
		if isVMRemoteWrite {
			zb.B = zstd.CompressLevel(zb.B[:0], bb.B, *vmProtoCompressLevel)
		} else {
			zb.B = snappy.Encode(zb.B[:cap(zb.B)], bb.B)
		}
		writeRequestBufPool.Put(bb)
		if len(zb.B) <= persistentqueue.MaxBlockSize {
			if err := pushBlock(zb.B); err != nil {
				return fmt.Errorf("cannot pushBlock  block into queue: %w", err)
			}
			blockSizeRows.Update(float64(len(wr.Timeseries)))
			blockSizeBytes.Update(float64(len(zb.B)))
			snappyBufPool.Put(zb)
			return nil
		}
		snappyBufPool.Put(zb)
	} else {
		writeRequestBufPool.Put(bb)
	}

	// Too big block. Recursively split it into smaller parts if possible.
	if len(wr.Timeseries) == 1 {
		// A single time series left. Recursively split its samples into smaller parts if possible.
		samples := wr.Timeseries[0].Samples
		if len(samples) == 1 {
			logger.Warnf("dropping a sample for metric with too long labels exceeding -remoteWrite.maxBlockSize=%d bytes", maxUnpackedBlockSize.N)
			return nil
		}
		n := len(samples) / 2
		wr.Timeseries[0].Samples = samples[:n]
		if err := pushWriteRequest(wr, pushBlock, isVMRemoteWrite); err != nil {
			return err
		}
		wr.Timeseries[0].Samples = samples[n:]
		if err := pushWriteRequest(wr, pushBlock, isVMRemoteWrite); err != nil {
			return err
		}
		wr.Timeseries[0].Samples = samples
		return nil
	}
	timeseries := wr.Timeseries
	n := len(timeseries) / 2
	wr.Timeseries = timeseries[:n]
	if err := pushWriteRequest(wr, pushBlock, isVMRemoteWrite); err != nil {
		return err
	}
	wr.Timeseries = timeseries[n:]
	if err := pushWriteRequest(wr, pushBlock, isVMRemoteWrite); err != nil {
		return err
	}
	wr.Timeseries = timeseries
	return nil
}

var (
	blockSizeBytes = metrics.NewHistogram(`vmagent_remotewrite_block_size_bytes`)
	blockSizeRows  = metrics.NewHistogram(`vmagent_remotewrite_block_size_rows`)
)

var writeRequestBufPool bytesutil.ByteBufferPool
var snappyBufPool bytesutil.ByteBufferPool
