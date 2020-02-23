package promremotewrite

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, insertRows)
	})
}

func insertRows(timeseries []prompb.TimeSeries) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range timeseries {
		rowsLen += len(timeseries[i].Samples)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	for i := range timeseries {
		ts := &timeseries[i]
		var metricNameRaw []byte
		for i := range ts.Samples {
			r := &ts.Samples[i]
			metricNameRaw = ctx.WriteDataPointExt(metricNameRaw, ts.Labels, r.Timestamp, r.Value)
		}
		rowsTotal += len(ts.Samples)
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
