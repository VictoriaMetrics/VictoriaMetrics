package datadog

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
)

// The maximum request size is defined at https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
var maxInsertRequestSize = flagutil.NewBytes("datadog.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single DataDog POST request to /api/v1/series")

// ParseStream parses DataDog POST request for /api/v1/series from reader and calls callback for the parsed request.
//
// callback shouldn't hold series after returning.
func ParseStream(r io.Reader, contentEncoding string, callback func(series []Series) error) error {
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
	if err := req.Unmarshal(ctx.reqBuf.B); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal DataDog POST request with size %d bytes: %s", len(ctx.reqBuf.B), err)
	}
	rows := 0
	series := req.Series
	for i := range series {
		rows += len(series[i].Points)
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
	lr := io.LimitReader(ctx.br, int64(maxInsertRequestSize.N)+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read compressed request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="datadog"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="datadog"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="datadog"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="datadog"}`)
)

func getPushCtx(r io.Reader) *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		ctx.br.Reset(r)
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			ctx := v.(*pushCtx)
			ctx.br.Reset(r)
			return ctx
		}
		return &pushCtx{
			br: bufio.NewReaderSize(r, 64*1024),
		}
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, cgroup.AvailableCPUs())

func getRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

func putRequest(req *Request) {
	requestPool.Put(req)
}

var requestPool sync.Pool
