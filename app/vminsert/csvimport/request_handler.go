package csvimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="csvimport"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="csvimport"}`)
)

// InsertHandler processes /api/v1/import/csv requests.
func InsertHandler(req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, func(rows []parser.Row) error {
			return insertRows(rows)
		})
	})
}

func insertRows(rows []parser.Row) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	ctx.Reset(len(rows))
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			ctx.AddLabel(tag.Key, tag.Value)
		}
		ctx.WriteDataPoint(nil, ctx.Labels, r.Timestamp, r.Value)
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
