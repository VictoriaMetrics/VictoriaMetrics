package stream

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

var (
	maxInsertRequestSize = flagutil.NewBytes("newrelic.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single NewRelic request "+
		"to /newrelic/infra/v2/metrics/events/bulk")
)

// Parse parses NewRelic POST request for /newrelic/infra/v2/metrics/events/bulk from r and calls callback for the parsed request.
//
// callback shouldn't hold rows after returning.
func Parse(r io.Reader, isGzip bool, callback func(rows []newrelic.Row) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)
	r = wcr

	if isGzip {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped NewRelic agent data: %w", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getPushCtx(r)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return fmt.Errorf("cannot read NewRelic request: %w", err)
	}

	rows := getRows()
	defer putRows(rows)

	if err := rows.Unmarshal(ctx.reqBuf.B); err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal NewRelic request: %w", err)
	}

	// Fill in missing timestamps
	currentTimestamp := int64(fasttime.UnixTimestamp())
	for i := range rows.Rows {
		r := &rows.Rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp * 1e3
		}
	}

	if err := callback(rows.Rows); err != nil {
		return fmt.Errorf("error when processing imported data: %w", err)
	}
	return nil
}

func getRows() *newrelic.Rows {
	v := rowsPool.Get()
	if v == nil {
		return &newrelic.Rows{}
	}
	return v.(*newrelic.Rows)
}

func putRows(rows *newrelic.Rows) {
	rows.Reset()
	rowsPool.Put(rows)
}

var rowsPool sync.Pool

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="newrelic"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="newrelic"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="newrelic"}`)
)

type pushCtx struct {
	br     *bufio.Reader
	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) Read() error {
	readCalls.Inc()
	lr := io.LimitReader(ctx.br, maxInsertRequestSize.N+1)
	startTime := fasttime.UnixTimestamp()
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read request in %d seconds: %w", fasttime.UnixTimestamp()-startTime, err)
	}
	if reqLen > maxInsertRequestSize.N {
		readErrors.Inc()
		return fmt.Errorf("too big request; mustn't exceed -newrelic.maxInsertRequestSize=%d bytes", maxInsertRequestSize.N)
	}
	return nil
}

func (ctx *pushCtx) reset() {
	ctx.br.Reset(nil)
	ctx.reqBuf.Reset()
}

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
