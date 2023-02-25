package csvimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/csvimport/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="csvimport"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="csvimport"}`)
)

// InsertHandler processes /api/v1/import/csv requests.
func InsertHandler(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return stream.Parse(req, func(rows []parser.Row) error {
		return insertRows(rows, extraLabels)
	})
}

func insertRows(rows []parser.Row, extraLabels []prompbmarshal.Label) error {
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
		if err := ctx.WriteDataPoint(nil, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
