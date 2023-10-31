package newrelic

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic/stream"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="newrelic"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="newrelic"}`)
)

// InsertHandlerForHTTP processes remote write for request to /newrelic/infra/v2/metrics/events/bulk request.
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	isGzip := ce == "gzip"
	return stream.Parse(req.Body, isGzip, func(rows []newrelic.Row) error {
		return insertRows(rows, extraLabels)
	})
}

func insertRows(rows []newrelic.Row, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	samplesCount := 0
	for i := range rows {
		samplesCount += len(rows[i].Samples)
	}
	ctx.Reset(samplesCount)

	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		samples := r.Samples
		for j := range samples {
			s := &samples[j]

			ctx.Labels = ctx.Labels[:0]
			ctx.AddLabelBytes(nil, s.Name)
			for k := range r.Tags {
				t := &r.Tags[k]
				ctx.AddLabelBytes(t.Key, t.Value)
			}
			for k := range extraLabels {
				label := &extraLabels[k]
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
			if err := ctx.WriteDataPoint(nil, ctx.Labels, r.Timestamp, s.Value); err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(samplesCount)
	rowsPerInsert.Update(float64(samplesCount))
	return ctx.FlushBufs()
}
