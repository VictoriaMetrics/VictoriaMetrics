package prometheus

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/concurrencylimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="prometheus"}`)
	rowsPerInsert = metrics.NewSummary(`vm_rows_per_insert{type="prometheus"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(at *auth.Token, r *http.Request, maxSize int64) error {
	return concurrencylimiter.Do(func() error {
		return insertHandlerInternal(at, r, maxSize)
	})
}

func insertHandlerInternal(at *auth.Token, r *http.Request, maxSize int64) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)
	if err := ctx.Read(r, maxSize); err != nil {
		return err
	}

	ic := &ctx.Common
	ic.Reset()
	timeseries := ctx.req.Timeseries
	rowsTotal := 0
	for i := range timeseries {
		ts := &timeseries[i]
		storageNodeIdx := ic.GetStorageNodeIdx(at, ts.Labels)
		ic.MetricNameBuf = ic.MetricNameBuf[:0]
		for i := range ts.Samples {
			r := &ts.Samples[i]
			if len(ic.MetricNameBuf) == 0 {
				ic.MetricNameBuf = storage.MarshalMetricNameRaw(ic.MetricNameBuf[:0], at.AccountID, at.ProjectID, ts.Labels)
			}
			if err := ic.WriteDataPointExt(at, storageNodeIdx, ic.MetricNameBuf, r.Timestamp, r.Value); err != nil {
				return err
			}
		}
		rowsTotal += len(ts.Samples)
	}
	rowsInserted.Get(at).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common netstorage.InsertCtx

	req    prompb.WriteRequest
	reqBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset()
	ctx.req.Reset()
	ctx.reqBuf = ctx.reqBuf[:0]
}

func (ctx *pushCtx) Read(r *http.Request, maxSize int64) error {
	prometheusReadCalls.Inc()

	var err error
	ctx.reqBuf, err = prompb.ReadSnappy(ctx.reqBuf[:0], r.Body, maxSize)
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
