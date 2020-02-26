package vmimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="vmimport"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="vmimport"}`)
)

// InsertHandler processes `/api/v1/import` request.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6
func InsertHandler(at *auth.Token, req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, func(rows []parser.Row) error {
			return insertRows(at, rows)
		})
	})
}

func insertRows(at *auth.Token, rows []parser.Row) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		for j := range r.Tags {
			tag := &r.Tags[j]
			ctx.AddLabelBytes(tag.Key, tag.Value)
		}
		ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, ctx.Labels)
		storageNodeIdx := ctx.GetStorageNodeIdx(at, ctx.Labels)
		values := r.Values
		timestamps := r.Timestamps
		_ = timestamps[len(values)-1]
		for j, value := range values {
			timestamp := timestamps[j]
			if err := ctx.WriteDataPointExt(at, storageNodeIdx, ctx.MetricNameBuf, timestamp, value); err != nil {
				return err
			}
		}
		rowsTotal += len(values)
	}
	rowsInserted.Get(at).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
