package opentsdb

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vmagent_rows_inserted_total{type="opentsdb"}`)
	rowsPerInsert = metrics.NewHistogram(`vmagent_rows_per_insert{type="opentsdb"}`)
)

// InsertHandler processes remote write for OpenTSDB put protocol.
//
// See http://opentsdb.net/docs/build/html/api_telnet/put.html
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
