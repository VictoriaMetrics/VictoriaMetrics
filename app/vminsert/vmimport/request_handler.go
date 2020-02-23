package vmimport

import (
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="vmimport"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="vmimport"}`)
)

// InsertHandler processes `/api/v1/import` request.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6
func InsertHandler(req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, insertRows)
	})
}

func insertRows(rows []parser.Row) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	rowsLen := 0
	for i := range rows {
		rowsLen += len(rows[i].Values)
	}
	ic := &ctx.Common
	ic.Reset(rowsLen)
	rowsTotal := 0
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabelBytes(tag.Key, tag.Value)
		}
		ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf[:0], ic.Labels)
		values := r.Values
		timestamps := r.Timestamps
		_ = timestamps[len(values)-1]
		for j, value := range values {
			timestamp := timestamps[j]
			ic.WriteDataPoint(ctx.metricNameBuf, nil, timestamp, value)
		}
		rowsTotal += len(values)
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common        common.InsertCtx
	metricNameBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset(0)
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
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
