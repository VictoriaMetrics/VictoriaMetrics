package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/snappy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ioutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var maxInsertRequestSize = flagutil.NewBytes("maxInsertRequestSize", 32*1024*1024, "The maximum size in bytes of a single Prometheus remote_write API request")

// Parse parses Prometheus remote_write message from reader and calls callback for the parsed timeseries.
//
// callback shouldn't hold tss after returning.
func Parse(r io.Reader, isVMRemoteWrite bool, callback func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	wcr, err := writeconcurrencylimiter.GetReader(r)
	if err != nil {
		return err
	}
	defer writeconcurrencylimiter.PutReader(wcr)

	ctx := getPushCtx(wcr)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}

	// Synchronously process the request in order to properly return errors to Parse caller,
	// so it could properly return HTTP 503 status code in response.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/896
	bb := bodyBufferPool.Get()
	defer bodyBufferPool.Put(bb)
	if isVMRemoteWrite {
		bb.B, err = encoding.DecompressZSTDLimited(bb.B[:0], ctx.reqBuf.B, maxInsertRequestSize.IntN())
		if err != nil {
			// Fall back to Snappy decompression, since vmagent may send snappy-encoded messages
			// with 'Content-Encoding: zstd' header if they were put into persistent queue before vmagent restart.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301
			//
			// Newer vmagent sends proper 'Content-Encoding' header.
			// The logic is preserved for backwards compatibility.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8650
			zstdErr := err
			bb.B, err = snappy.Decode(bb.B, ctx.reqBuf.B, maxInsertRequestSize.IntN())
			if err != nil {
				return fmt.Errorf("cannot decompress zstd-encoded request with length %d: %w", len(ctx.reqBuf.B), zstdErr)
			}
		}
	} else {
		bb.B, err = snappy.Decode(bb.B, ctx.reqBuf.B, maxInsertRequestSize.IntN())
		if err != nil {
			// Fall back to zstd decompression, since vmagent may send zstd-encoded messages
			// without 'Content-Encoding: zstd' header if they were put into persistent queue before vmagent restart.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5301#issuecomment-1815871992
			//
			// Newer vmagent sends proper 'Content-Encoding' header.
			// The logic is preserved for backwards compatibility.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8650
			snappyErr := err
			bb.B, err = encoding.DecompressZSTDLimited(bb.B[:0], ctx.reqBuf.B, maxInsertRequestSize.IntN())
			if err != nil {
				return fmt.Errorf("cannot decompress snappy-encoded request with length %d: %w", len(ctx.reqBuf.B), snappyErr)
			}
		}
	}
	if int64(len(bb.B)) > maxInsertRequestSize.N {
		return fmt.Errorf("too big unpacked request; mustn't exceed `-maxInsertRequestSize=%d` bytes; got %d bytes", maxInsertRequestSize.N, len(bb.B))
	}
	wru := prompb.GetWriteRequestUnmarshaler()
	defer prompb.PutWriteRequestUnmarshaler(wru)
	wr, err := wru.UnmarshalProtobuf(bb.B)
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal prompb.WriteRequest with size %d bytes: %w", len(bb.B), err)
	}

	rows := 0
	tss := wr.Timeseries
	for i := range tss {
		rows += len(tss[i].Samples)
	}
	rowsRead.Add(rows)
	mms := wr.Metadata
	metadataRead.Add(len(mms))

	if err := callback(tss, mms); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

var bodyBufferPool bytesutil.ByteBufferPool

type pushCtx struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

func (ctx *pushCtx) Read() error {
	readCalls.Inc()
	lr := ioutil.GetLimitedReader(ctx.br, int64(maxInsertRequestSize.N)+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	ioutil.PutLimitedReader(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read compressed request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed -maxInsertRequestSize=%d bytes; got %d bytes", maxInsertRequestSize.N, reqLen)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="promremotewrite"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="promremotewrite"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="promremotewrite"}`)
	metadataRead    = metrics.NewCounter(`vm_protoparser_metadata_read_total{type="promremotewrite"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="promremotewrite"}`)
)

func getPushCtx(r io.Reader) *pushCtx {
	if v := pushCtxPool.Get(); v != nil {
		ctx := v.(*pushCtx)
		ctx.br.Reset(r)
		return ctx
	}
	return &pushCtx{
		br: bufio.NewReaderSize(r, 64*1024),
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	pushCtxPool.Put(ctx)
}

var pushCtxPool sync.Pool
