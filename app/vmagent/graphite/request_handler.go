package graphite

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vmagent_rows_inserted_total{type="graphite"}`)
	rowsPerInsert = metrics.NewHistogram(`vmagent_rows_per_insert{type="graphite"}`)
)

// InsertHandler processes remote write for graphite plaintext protocol.
//
// See https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol
func InsertHandler(r io.Reader) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(r, insertRows)
	})
}

func insertRows(rows []parser.Row) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		labelsLen := len(labels)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, prompbmarshal.Sample{
			Value:     r.Value,
			Timestamp: r.Timestamp,
		})
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[len(samples)-1:],
		})
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	remotewrite.Push(&ctx.WriteRequest)
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return nil
}
