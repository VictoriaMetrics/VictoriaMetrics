package opentsdbhttp

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="opentsdb-http"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="opentsdb-http"}`)

	opentsdbReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb-http"}`)
	opentsdbReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb-http"}`)
	opentsdbUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb-http"}`)
)

// insertHandler processes HTTP OpenTSDB put requests.
// See http://opentsdb.net/docs/build/html/api_http/put.html
func insertHandler(at *auth.Token, req *http.Request, maxSize int64) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(at, req, maxSize)
	})
}

func insertHandlerInternal(at *auth.Token, req *http.Request, maxSize int64) error {
	opentsdbReadCalls.Inc()

	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			opentsdbReadErrors.Inc()
			return fmt.Errorf("cannot read gzipped http protocol data: %s", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getPushCtx()
	defer putPushCtx(ctx)

	// Read the request in ctx.reqBuf
	lr := io.LimitReader(r, maxSize+1)
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		opentsdbReadErrors.Inc()
		return fmt.Errorf("cannot read HTTP OpenTSDB request: %s", err)
	}
	if reqLen > maxSize {
		opentsdbReadErrors.Inc()
		return fmt.Errorf("too big HTTP OpenTSDB request; mustn't exceed %d bytes", maxSize)
	}

	// Unmarshal the request to ctx.Rows
	p := parserPool.Get()
	defer parserPool.Put(p)
	v, err := p.ParseBytes(ctx.reqBuf.B)
	if err != nil {
		opentsdbUnmarshalErrors.Inc()
		return fmt.Errorf("cannot parse HTTP OpenTSDB json: %s", err)
	}
	if err := ctx.Rows.Unmarshal(v); err != nil {
		opentsdbUnmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal HTTP OpenTSDB json %s, %s", err, v)
	}

	// Fill in missing timestamps
	currentTimestamp := time.Now().Unix()
	rows := ctx.Rows.Rows
	for i := range rows {
		r := &rows[i]
		if r.Timestamp == 0 {
			r.Timestamp = currentTimestamp
		}
	}

	// Convert timestamps in seconds to milliseconds if needed.
	// See http://opentsdb.net/docs/javadoc/net/opentsdb/core/Const.html#SECOND_MASK
	for i := range rows {
		r := &rows[i]
		if r.Timestamp&secondMask == 0 {
			r.Timestamp *= 1e3
		}
	}

	// Insert ctx.Rows to db.
	ic := &ctx.Common
	ic.Reset()
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		ic.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabel(tag.Key, tag.Value)
		}
		ic.WriteDataPoint(at, ic.Labels, r.Timestamp, r.Value)
	}
	rowsInserted.Get(at).Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ic.FlushBufs()
}

const secondMask int64 = 0x7FFFFFFF00000000

var parserPool fastjson.ParserPool

type pushCtx struct {
	Rows   Rows
	Common netstorage.InsertCtx

	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) reset() {
	ctx.Rows.Reset()
	ctx.Common.Reset()
	ctx.reqBuf.Reset()
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
