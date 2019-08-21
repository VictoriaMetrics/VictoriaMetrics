package opentsdbhttp

import (
	"compress/gzip"
	"fmt"
	"github.com/valyala/fasthttp"
	"io"
	"net/http"
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

	opentsdbReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb-http"}`)
	opentsdbReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb-http"}`)
	opentsdbUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb-http"}`)


	rowsInsertedFastHttp  = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb-fasthttp"}`)
	rowsPerInsertFastHttp = metrics.NewSummary(`vm_rows_per_insert{type="opentsdb-fasthttp"}`)

	opentsdbReadCallsFastHttp       = metrics.NewCounter(`vm_read_calls_total{name="opentsdb-fasthttp"}`)
	opentsdbReadErrorsFastHttp      = metrics.NewCounter(`vm_read_errors_total{name="opentsdb-fasthttp"}`)
	opentsdbUnmarshalErrorsFastHttp = metrics.NewCounter(`vm_unmarshal_errors_total{name="opentsdb-fasthttp"}`)



)

// InsertHandlerFastHttp processes remote write for openTSDB http protocol via fasthttp.
//
func InsertHandlerFastHttp(req *fasthttp.RequestCtx) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternalFastHttp(req)
	})
}

func insertHandlerInternalFastHttp(req *fasthttp.RequestCtx) error {
	opentsdbReadCallsFastHttp.Inc()

	var err error
	r := req.Request.Body()

	if ob2s(req.Request.Header.Peek("Content-Encoding")) == "gzip" {
		r, err = req.Request.BodyGunzip()
		if err != nil {
			opentsdbReadErrorsFastHttp.Inc()
			return fmt.Errorf("cannot read gzipped http protocol data: %s", err)
		}
	}

	ctx := getPushCtx()
	defer putPushCtx(ctx)

	if err = ctx.ReadBytes(r, true); err != nil {
		return err
	}

	if err = ctx.InsertRows(true); err != nil {
		return err
	}

	return nil
}

func getGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func putGzipReader(zr *gzip.Reader) {
	_ = zr.Close()
	gzipReaderPool.Put(zr)
}

var gzipReaderPool sync.Pool


func InsertHandler(req *http.Request, maxSize int64) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(req, maxSize)
	})
}

func insertHandlerInternal(req *http.Request, maxSize int64) error {
	opentsdbReadCalls.Inc()
	var err error

	r := req.Body

	if req.Header.Get("Content-Encoding") == "gzip" {
		zr, err := getGzipReader(r)
		if err != nil {
			return fmt.Errorf("cannot read gzipped http protocol data: %s", err)
		}
		defer putGzipReader(zr)
		r = zr
	}

	ctx := getPushCtx()
	defer putPushCtx(ctx)

	if err = ctx.Read(r, maxSize); err != nil {
		return err
	}

	if err = ctx.InsertRows(false); err != nil {
		return err
	}

	return nil
}

func (ctx *pushCtx) InsertRows(isFastHttp bool) error {
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

	if isFastHttp {
		rowsInsertedFastHttp.Add(len(rows))
		rowsPerInsertFastHttp.Update(float64(len(rows)))
	} else {
		rowsInserted.Add(len(rows))
		rowsPerInsert.Update(float64(len(rows)))
	}

	return ic.FlushBufs()
}

func (ctx *pushCtx) ReadBytes(r []byte, isFastHttp bool) error {
	var err error

	v, err := ctx.parser.ParseBytes(r)

	if err != nil {
		if isFastHttp {
			opentsdbUnmarshalErrorsFastHttp.Inc()
		} else {
			opentsdbUnmarshalErrors.Inc()
		}

		ctx.err = fmt.Errorf("error parsing json: %s, length: %d", err, len(r))
		return ctx.err
	}

	if err := ctx.Rows.Unmarshal(v); err != nil {
		if isFastHttp {
			opentsdbUnmarshalErrorsFastHttp.Inc()
		} else {
			opentsdbUnmarshalErrors.Inc()
		}

		ctx.err = fmt.Errorf("cannot unmarshal opentsdb http protocol json %s, %s", err, v)
		return ctx.err
	}

	return nil
}

func (ctx *pushCtx) Read(r io.Reader, maxSize int64) error {

	var err error
	lr := io.LimitReader(r, maxSize+1)
	reqLen, err := ctx.reqBuf.ReadFrom(lr)

	if err != nil {
		opentsdbReadErrors.Inc()
		ctx.err = fmt.Errorf("cannot read request: %s", err)
		return ctx.err
	}
	if reqLen > maxSize {
		opentsdbReadErrors.Inc()
		ctx.err = fmt.Errorf("too big packed request; mustn't exceed %d bytes", maxSize)
		return ctx.err
	}
	return ctx.ReadBytes(ctx.reqBuf.B, false)
}


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

