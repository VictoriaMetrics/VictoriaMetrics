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
		req, func(series []prompbmarshal.TimeSeries) error {
			return insertRows(series, extraLabels)
		},
	)
}

func insertRows(series []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsTotal := 0
	for i := range series {
		rowsTotal += len(series[i].Samples)
	}

	hasRelabeling := relabel.HasRelabeling()
	ctx.Reset(rowsTotal)
	for i := range series {
		s := &series[i]

		ctx.Labels = ctx.Labels[:0]
		for k := range s.Labels {
			ctx.AddLabel(s.Labels[k].Name, s.Labels[k].Value)
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

		for _, sample := range s.Samples {
			_, err := ctx.WriteDataPointExt(nil, ctx.Labels, sample.Timestamp, sample.Value)
			if err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
