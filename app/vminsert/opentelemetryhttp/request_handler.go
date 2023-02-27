package opentelemetryhttp

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="opentelemetry"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="opentelemetry"}`)
)

// InsertHandler processes opentelemetry metrics.
func InsertHandler(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isJSON := req.Header.Get("Content-Type") == "application/json"
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req.Body, isJSON, isGzipped, func(tss []prompb.TimeSeries) error {
			return insertRows(tss, extraLabels)
		})
	})
}

func insertRows(timeseries []prompb.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range timeseries {
		rowsLen += len(timeseries[i].Samples)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range timeseries {
		ts := &timeseries[i]
		rowsTotal += len(ts.Samples)
		ctx.Labels = ctx.Labels[:0]
		srcLabels := ts.Labels
		for _, srcLabel := range srcLabels {
			ctx.AddLabelBytes(srcLabel.Name, srcLabel.Value)
		}
		for j := range extraLabels {
			label := &extraLabels[j]
			ctx.AddLabel(label.Name, label.Value)
		}
		if hasRelabeling {
			ctx.ApplyRelabeling()
		}
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.SortLabelsIfNeeded()
		var metricNameRaw []byte
		var err error
		samples := ts.Samples
		for i := range samples {
			r := &samples[i]
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, r.Timestamp, r.Value)
			if err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
