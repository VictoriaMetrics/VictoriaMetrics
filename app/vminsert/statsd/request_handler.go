package statsd

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/statsd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/statsd/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="statsd"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="statsd"}`)
)

// InsertHandler processes remote write for statsd protocol with tags.
//
// https://github.com/statsd/statsd/blob/master/docs/metric_types.md
func InsertHandler(r io.Reader) error {
	return stream.Parse(r, false, insertRows)
}

func insertRows(rows []parser.Row) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	ctx.Reset(len(rows))
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ctx.AddLabel(tag.Key, tag.Value)
		}
		if hasRelabeling {
			ctx.ApplyRelabeling()
		}
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.SortLabelsIfNeeded()
		var metricName []byte
		var err error
		for _, v := range r.Values {
			metricName, err = ctx.WriteDataPointExt(metricName, ctx.Labels, r.Timestamp, v)
			if err != nil {
				return err
			}
		}

	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
