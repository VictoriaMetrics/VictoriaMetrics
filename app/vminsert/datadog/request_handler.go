package datadog

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="datadog"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="datadog"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series, /api/v2/series, /api/v1/sketches, /api/beta/sketches request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return stream.Parse(
		req, func(series prompbmarshal.TimeSeries) error {
			series.Labels = append(series.Labels, extraLabels...)
			return insertRows(series)
		},
	)
}

func insertRows(series prompbmarshal.TimeSeries) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	hasRelabeling := relabel.HasRelabeling()
	rowsTotal := len(series.Samples)

	ctx.Reset(rowsTotal)
	ctx.Labels = ctx.Labels[:0]
	for l := range series.Labels {
		ctx.AddLabel(series.Labels[l].Name, series.Labels[l].Value)
	}
	if hasRelabeling {
		ctx.ApplyRelabeling()
	}
	if len(ctx.Labels) == 0 {
		return nil
	}
	ctx.SortLabelsIfNeeded()

	for _, sample := range series.Samples {
		_, err := ctx.WriteDataPointExt(nil, ctx.Labels, sample.Timestamp, sample.Value)
		if err != nil {
			return err
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
