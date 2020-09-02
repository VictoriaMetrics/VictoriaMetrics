package vmimport

import (
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
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
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, func(rows []parser.Row) error {
			return insertRows(rows, extraLabels)
		})
	})
}

func insertRows(rows []parser.Row, extraLabels []prompbmarshal.Label) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	rowsLen := 0
	for i := range rows {
		rowsLen += len(rows[i].Values)
	}
	ic := &ctx.Common
	ic.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		for j := range r.Tags {
			tag := &r.Tags[j]
			ic.AddLabelBytes(tag.Key, tag.Value)
		}
		for j := range extraLabels {
			label := &extraLabels[j]
			ic.AddLabel(label.Name, label.Value)
		}
		if hasRelabeling {
			ic.ApplyRelabeling()
		}
		if len(ic.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf[:0], ic.Labels)
		values := r.Values
		timestamps := r.Timestamps
		_ = timestamps[len(values)-1]
		for j, value := range values {
			timestamp := timestamps[j]
			if err := ic.WriteDataPoint(ctx.metricNameBuf, nil, timestamp, value); err != nil {
				return err
			}
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
