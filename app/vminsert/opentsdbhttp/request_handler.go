package opentsdbhttp

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"runtime"
	"sync"

	"github.com/valyala/fastjson"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb-http"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="opentsdb-http"}`)
)

// InsertHandler processes remote write for openTSDB http protocol.
//
func InsertHandler(req *fasthttp.RequestCtx) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(req)
	})
}

func insertHandlerInternal(req *fasthttp.RequestCtx) error {
	opentsdbReadCalls.Inc()

	var err error
	r := req.Request.Body()

	if ob2s(req.Request.Header.Peek("Content-Encoding")) == "gzip" {
		r, err = req.Request.BodyGunzip()
		if err != nil {
			opentsdbReadErrors.Inc()
			return fmt.Errorf("cannot read gzipped http protocol data: %s", err)
		}
	}

	ctx := getPushCtx()
	defer putPushCtx(ctx)

	if err = ctx.Read(r); err != nil {
		return err
	}

	if err = ctx.InsertRows(); err != nil {
		return err
	}

	return nil
}

func (ctx *pushCtx) InsertRows() error {
	rows := ctx.Rows.Rows
	ic := &ctx.Common
	ic.Reset(len(rows))
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabel(tag.Key, tag.Value)
		}
		ic.WriteDataPoint(nil, ic.Labels, r.Timestamp, r.Value)
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ic.FlushBufs()
}

func (ctx *pushCtx) Read(r []byte) error {
	var err error

	v, err := ctx.parser.ParseBytes(r)

	if err != nil {
		opentsdbUnmarshalErrors.Inc()
		ctx.err = fmt.Errorf("error parsing json: %s, length: %d", err, len(r))
		return ctx.err
	}

	if err := ctx.Rows.Unmarshal(v); err != nil {
		opentsdbUnmarshalErrors.Inc()
		ctx.err = fmt.Errorf("cannot unmarshal opentsdb http protocol json %s, %s", err, v)
		return ctx.err
	}

	return nil
}

var (
	opentsdbReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb-http"}`)
	opentsdbReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb-http"}`)
	opentsdbUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb-http"}`)
)

type pushCtx struct {
	Rows   Rows
	Common common.InsertCtx

	reqBuf         bytesutil.ByteBuffer
	parser 		   fastjson.Parser

	err error
}

func (ctx *pushCtx) reset() {
	ctx.Rows.Reset()
	ctx.Common.Reset(0)

	ctx.reqBuf.Reset()

	ctx.err = nil
}

func getPushCtx() *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			return v.(*pushCtx)
		}
		return &pushCtx{}
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
var pushCtxPoolCh = make(chan *pushCtx, runtime.GOMAXPROCS(-1))

