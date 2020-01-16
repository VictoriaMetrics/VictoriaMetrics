package opentsdbhttp

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

var maxInsertRequestSize = flag.Int("opentsdbhttp.maxInsertRequestSize", 32*1024*1024, "The maximum size of OpenTSDB HTTP put request")

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb-http"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="opentsdb-http"}`)

	readCalls       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb-http"}`)
	readErrors      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb-http"}`)
	unmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb-http"}`)
)

// insertHandler processes HTTP OpenTSDB put requests.
// See http://opentsdb.net/docs/build/html/api_http/put.html
func insertHandler(req *http.Request) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(req)
	})
}

func insertHandlerInternal(req *http.Request) error {
	readCalls.Inc()

	r := req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(r)
		if err != nil {
			readErrors.Inc()
			return fmt.Errorf("cannot read gzipped http protocol data: %s", err)
		}
		defer common.PutGzipReader(zr)
		r = zr
	}

	ctx := getPushCtx()
	defer putPushCtx(ctx)

	// Read the request in ctx.reqBuf
	lr := io.LimitReader(r, int64(*maxInsertRequestSize)+1)
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read HTTP OpenTSDB request: %s", err)
	}
	if reqLen > int64(*maxInsertRequestSize) {
		readErrors.Inc()
		return fmt.Errorf("too big HTTP OpenTSDB request; mustn't exceed `-opentsdbhttp.maxInsertRequestSize=%d` bytes", *maxInsertRequestSize)
	}

	// Unmarshal the request to ctx.Rows
	p := parserPool.Get()
	defer parserPool.Put(p)
	v, err := p.ParseBytes(ctx.reqBuf.B)
	if err != nil {
		unmarshalErrors.Inc()
		return fmt.Errorf("cannot parse HTTP OpenTSDB json: %s", err)
	}
	ctx.Rows.Unmarshal(v)

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

const secondMask int64 = 0x7FFFFFFF00000000

var parserPool fastjson.ParserPool

type pushCtx struct {
	Rows   Rows
	Common common.InsertCtx

	reqBuf bytesutil.ByteBuffer
}

func (ctx *pushCtx) reset() {
	ctx.Rows.Reset()
	ctx.Common.Reset(0)
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
