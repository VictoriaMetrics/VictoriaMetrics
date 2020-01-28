package prometheus

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

var maxInsertRequestSize = flag.Int("maxInsertRequestSize", 32*1024*1024, "The maximum size in bytes of a single Prometheus remote_write API request")

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="prometheus"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="prometheus"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(r *http.Request) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(r)
	})
}

func insertHandlerInternal(r *http.Request) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)
	if err := ctx.Read(r); err != nil {
		return err
	}
	timeseries := ctx.req.Timeseries
	rowsLen := 0
	for i := range timeseries {
		rowsLen += len(timeseries[i].Samples)
	}
	ic := &ctx.Common
	ic.Reset(rowsLen)
	rowsTotal := 0
	for i := range timeseries {
		ts := &timeseries[i]
		var metricNameRaw []byte
		for i := range ts.Samples {
			r := &ts.Samples[i]
			metricNameRaw = ic.WriteDataPointExt(metricNameRaw, ts.Labels, r.Timestamp, r.Value)
		}
		rowsTotal += len(ts.Samples)
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common common.InsertCtx

	req    prompb.WriteRequest
	reqBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset(0)
	ctx.req.Reset()
	ctx.reqBuf = ctx.reqBuf[:0]
}

func (ctx *pushCtx) Read(r *http.Request) error {
	prometheusReadCalls.Inc()

	var err error
	ctx.reqBuf, err = readSnappy(ctx.reqBuf[:0], r.Body)
	if err != nil {
		prometheusReadErrors.Inc()
		return fmt.Errorf("cannot read prompb.WriteRequest: %s", err)
	}
	if err = ctx.req.Unmarshal(ctx.reqBuf); err != nil {
		prometheusUnmarshalErrors.Inc()
		return fmt.Errorf("cannot unmarshal prompb.WriteRequest with size %d bytes: %s", len(ctx.reqBuf), err)
	}
	return nil
}

var (
	prometheusReadCalls       = metrics.NewCounter(`vm_read_calls_total{name="prometheus"}`)
	prometheusReadErrors      = metrics.NewCounter(`vm_read_errors_total{name="prometheus"}`)
	prometheusUnmarshalErrors = metrics.NewCounter(`vm_unmarshal_errors_total{name="prometheus"}`)
)

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

func readSnappy(dst []byte, r io.Reader) ([]byte, error) {
	lr := io.LimitReader(r, int64(*maxInsertRequestSize)+1)
	bb := bodyBufferPool.Get()
	reqLen, err := bb.ReadFrom(lr)
	if err != nil {
		bodyBufferPool.Put(bb)
		return dst, fmt.Errorf("cannot read compressed request: %s", err)
	}
	if reqLen > int64(*maxInsertRequestSize) {
		return dst, fmt.Errorf("too big packed request; mustn't exceed `-maxInsertRequestSize=%d` bytes", *maxInsertRequestSize)
	}

	buf := dst[len(dst):cap(dst)]
	buf, err = snappy.Decode(buf, bb.B)
	bodyBufferPool.Put(bb)
	if err != nil {
		err = fmt.Errorf("cannot decompress request with length %d: %s", reqLen, err)
		return dst, err
	}
	if len(buf) > *maxInsertRequestSize {
		return dst, fmt.Errorf("too big unpacked request; mustn't exceed `-maxInsertRequestSize=%d` bytes", *maxInsertRequestSize)
	}
	if len(buf) > 0 && len(dst) < cap(dst) && &buf[0] == &dst[len(dst):cap(dst)][0] {
		dst = dst[:len(dst)+len(buf)]
	} else {
		dst = append(dst, buf...)
	}
	return dst, nil
}

var bodyBufferPool bytesutil.ByteBufferPool
