package vmimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vmagent_rows_inserted_total{type="vmimport"}`)
	rowsPerInsert = metrics.NewHistogram(`vmagent_rows_per_insert{type="vmimport"}`)
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
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		labelsLen := len(labels)
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompbmarshal.Label{
				Name:  bytesutil.ToUnsafeString(tag.Key),
				Value: bytesutil.ToUnsafeString(tag.Value),
			})
		}
		labels = append(labels, extraLabels...)
		values := r.Values
		timestamps := r.Timestamps
		if len(timestamps) != len(values) {
			logger.Panicf("BUG: len(timestamps)=%d must match len(values)=%d", len(timestamps), len(values))
		}
		samplesLen := len(samples)
		for j, value := range values {
			samples = append(samples, prompbmarshal.Sample{
				Value:     value,
				Timestamp: timestamps[j],
			})
		}
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[samplesLen:],
		})
		rowsTotal += len(values)
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	remotewrite.Push(&ctx.WriteRequest)
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return nil
}
