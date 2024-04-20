package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses DataDog POST request for /api/beta/sketches from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(r io.Reader, contentEncoding string, callback func(series []*datadogsketches.Sketch) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	switch contentEncoding {
	case "gzip":
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped DataDog data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	case "deflate":
		zlr, err := common.GetZlibReader(r)
		if err != nil {
			return fmt.Errorf("cannot read deflated DataDog data: %w", err)
		}
		defer common.PutZlibReader(zlr)
		r = zlr
	}

	ctx := getPushCtx(r)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}
	req := getRequest()
	defer putRequest(req)

	if err := req.UnmarshalProtobuf(ctx.reqBuf.B); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog Sketches request with size %d bytes: %w", len(ctx.reqBuf.B), err)
	}

	rows := 0
	sketches := req.Sketches
	for _, sketch := range sketches {
		rows += sketch.RowsCount()
		if *datadogutils.SanitizeMetricName {
			sketch.Metric = datadogutils.SanitizeName(sketch.Metric)
		}
	}
	rowsRead.Add(rows)

	if err := callback(sketches); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

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
	lr := io.LimitReader(ctx.br, int64(datadogutils.MaxInsertRequestSize.N)+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(datadogutils.MaxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big request; mustn't exceed -datadog.maxInsertRequestSize=%d bytes", datadogutils.MaxInsertRequestSize.N)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadogsketches"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadogsketches"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadogsketches"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadogsketches"}`)
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

func getRequest() *datadogsketches.SketchPayload {
	v := requestPool.Get()
	if v == nil {
		return &datadogsketches.SketchPayload{}
	}
	return v.(*datadogsketches.SketchPayload)
}

func putRequest(req *datadogsketches.SketchPayload) {
	requestPool.Put(req)
}

var requestPool sync.Pool
