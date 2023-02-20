package opentsdb

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="opentsdb"}`)
)

// InsertHandler processes remote write for OpenTSDB put protocol.
//
// See http://opentsdb.net/docs/build/html/api_telnet/put.html
func InsertHandler(r io.Reader) error {
	return stream.Parse(r, insertRows)
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
		if err := ctx.WriteDataPoint(nil, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
