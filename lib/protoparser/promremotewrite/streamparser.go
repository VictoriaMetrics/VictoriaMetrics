package promremotewrite

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var maxInsertRequestSize = flagutil.NewBytes("maxInsertRequestSize", 32*1024*1024, "The maximum size in bytes of a single Prometheus remote_write API request")

// ParseStream parses Prometheus remote_write message req and calls callback for the parsed timeseries.
//
// The callback can be called concurrently multiple times for streamed data from req.
// The callback can be called after ParseStream returns.
//
// callback shouldn't hold tss after returning.
func ParseStream(req *http.Request, callback func(tss []prompb.TimeSeries) error) error {
	ctx := getPushCtx(req.Body)
	defer putPushCtx(ctx)
	if err := ctx.Read(); err != nil {
		return err
	}
	uw := getUnmarshalWork()
	uw.callback = callback
	uw.reqBuf, ctx.reqBuf.B = ctx.reqBuf.B, uw.reqBuf
	common.ScheduleUnmarshalWork(uw)
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
	reqLen, err := ctx.reqBuf.ReadFrom(lr)
	if err != nil {
		readErrors.Inc()
		return fmt.Errorf("cannot read compressed request: %w", err)
	}
	if reqLen > int64(maxInsertRequestSize.N) {
		readErrors.Inc()
		return fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", maxInsertRequestSize.N)
	}
	return nil
}

var (
	readCalls       = metrics.NewCounter(`vm_protoparser_read_calls_total{type="promremotewrite"}`)
	readErrors      = metrics.NewCounter(`vm_protoparser_read_errors_total{type="promremotewrite"}`)
	rowsRead        = metrics.NewCounter(`vm_protoparser_rows_read_total{type="promremotewrite"}`)
	unmarshalErrors = metrics.NewCounter(`vm_protoparser_unmarshal_errors_total{type="promremotewrite"}`)
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
var pushCtxPoolCh = make(chan *pushCtx, runtime.GOMAXPROCS(-1))

type unmarshalWork struct {
	wr       prompb.WriteRequest
	callback func(tss []prompb.TimeSeries) error
	reqBuf   []byte
}

func (uw *unmarshalWork) reset() {
	uw.wr.Reset()
	uw.callback = nil
	uw.reqBuf = uw.reqBuf[:0]
}

// Unmarshal implements common.UnmarshalWork
func (uw *unmarshalWork) Unmarshal() {
	bb := bodyBufferPool.Get()
	defer bodyBufferPool.Put(bb)
	var err error
	bb.B, err = snappy.Decode(bb.B[:cap(bb.B)], uw.reqBuf)
	if err != nil {
		logger.Errorf("cannot decompress request with length %d: %s", len(uw.reqBuf), err)
		return
	}
	if len(bb.B) > maxInsertRequestSize.N {
		logger.Errorf("too big unpacked request; mustn't exceed `-maxInsertRequestSize=%d` bytes; got %d bytes", maxInsertRequestSize.N, len(bb.B))
		return
	}
	if err := uw.wr.Unmarshal(bb.B); err != nil {
		unmarshalErrors.Inc()
		logger.Errorf("cannot unmarshal prompb.WriteRequest with size %d bytes: %s", len(bb.B), err)
		return
	}

	rows := 0
	tss := uw.wr.Timeseries
	for i := range tss {
		rows += len(tss[i].Samples)
	}
	rowsRead.Add(rows)

	if err := uw.callback(tss); err != nil {
		logger.Errorf("error when processing imported data: %s", err)
		putUnmarshalWork(uw)
		return
	}
	putUnmarshalWork(uw)
}

var bodyBufferPool bytesutil.ByteBufferPool

func getUnmarshalWork() *unmarshalWork {
	v := unmarshalWorkPool.Get()
	if v == nil {
		return &unmarshalWork{}
	}
	return v.(*unmarshalWork)
}

func putUnmarshalWork(uw *unmarshalWork) {
	uw.reset()
	unmarshalWorkPool.Put(uw)
}

var unmarshalWorkPool sync.Pool
