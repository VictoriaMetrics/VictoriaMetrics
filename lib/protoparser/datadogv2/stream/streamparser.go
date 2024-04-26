package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// Parse parses DataDog POST request for /api/v2/series from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func Parse(r io.Reader, contentEncoding, contentType string, callback func(series []datadogv2.Series) error) error {
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

	var err error
	switch contentType {
	case "application/x-protobuf":
		err = datadogv2.UnmarshalProtobuf(req, ctx.reqBuf.B)
	default:
		err = datadogv2.UnmarshalJSON(req, ctx.reqBuf.B)
	}
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog %s request with size %d bytes: %w", contentType, len(ctx.reqBuf.B), err)
	}

	rows := 0
	series := req.Series
	for i := range series {
		rows += len(series[i].Points)
		if *datadogutils.SanitizeMetricName {
			series[i].Metric = datadogutils.SanitizeName(series[i].Metric)
		}
	}
	rowsRead.Add(rows)

	if err := callback(series); err != nil {
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
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadogv2"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadogv2"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadogv2"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadogv2"}`)
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

func getRequest() *datadogv2.Request {
	v := requestPool.Get()
	if v == nil {
		return &datadogv2.Request{}
	}
	return v.(*datadogv2.Request)
}

func putRequest(req *datadogv2.Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool
