package prompush

import (
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="promscrape"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="promscrape"}`)
)

const maxRowsPerBlock = 10000

// Push pushes wr to storage.
func Push(wr *prompbmarshal.WriteRequest) {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	tss := wr.Timeseries
	for len(tss) > 0 {
		// Process big tss in smaller blocks in order to reduce maxmimum memory usage
		tssBlock := tss
		if len(tssBlock) > maxRowsPerBlock {
			tssBlock = tss[:maxRowsPerBlock]
			tss = tss[maxRowsPerBlock:]
		} else {
			tss = nil
		}
		ctx.push(tssBlock)
	}
}

func (ctx *pushCtx) push(tss []prompbmarshal.TimeSeries) {
	rowsLen := 0
	for i := range tss {
		rowsLen += len(tss[i].Samples)
	}
	ic := &ctx.Common
	ic.Reset(rowsLen)
	rowsTotal := 0
	labels := ctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labels = labels[:0]
		for j := range ts.Labels {
			label := &ts.Labels[j]
			labels = append(labels, prompb.Label{
				Name:  bytesutil.ToUnsafeBytes(label.Name),
				Value: bytesutil.ToUnsafeBytes(label.Value),
			})
		}
		var metricNameRaw []byte
		for i := range ts.Samples {
			r := &ts.Samples[i]
			metricNameRaw = ic.WriteDataPointExt(metricNameRaw, labels, r.Timestamp, r.Value)
		}
		rowsTotal += len(ts.Samples)
	}
	ctx.labels = labels
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	if err := ic.FlushBufs(); err != nil {
		logger.Errorf("cannot flush promscrape data to storage: %s", err)
	}
}

type pushCtx struct {
	Common common.InsertCtx
	labels []prompb.Label
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset(0)

	for i := range ctx.labels {
		label := &ctx.labels[i]
		label.Name = nil
		label.Value = nil
	}
	ctx.labels = ctx.labels[:0]
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
