package prompush

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

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
		push(ctx, tssBlock)
	}
}

func push(ctx *common.InsertCtx, tss []prompbmarshal.TimeSeries) {
	rowsLen := 0
	for i := range tss {
		rowsLen += len(tss[i].Samples)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	for i := range tss {
		ts := &tss[i]
		ctx.Labels = ctx.Labels[:0]
		for j := range ts.Labels {
			label := &ts.Labels[j]
			ctx.AddLabel(label.Name, label.Value)
		}
		ctx.ApplyRelabeling()
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		var metricNameRaw []byte
		var err error
		for i := range ts.Samples {
			r := &ts.Samples[i]
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, r.Timestamp, r.Value)
			if err != nil {
				logger.Errorf("cannot write promscape data to storage: %s", err)
				return
			}
		}
		rowsTotal += len(ts.Samples)
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	if err := ctx.FlushBufs(); err != nil {
		logger.Errorf("cannot flush promscrape data to storage: %s", err)
	}
}
