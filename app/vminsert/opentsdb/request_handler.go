package opentsdb

import (
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="opentsdb"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="opentsdb"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="opentsdb"}`)
)

// InsertHandler processes remote write for OpenTSDB put protocol.
//
// See http://opentsdb.net/docs/build/html/api_telnet/put.html
func InsertHandler(at *auth.Token, r io.Reader) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(r, func(rows []parser.Row) error {
			return insertRows(at, rows)
		})
	})
}

func insertRows(at *auth.Token, rows []parser.Row) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	atCopy := *at
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", r.Metric)
		for j := range r.Tags {
			tag := &r.Tags[j]
			if atCopy.AccountID == 0 {
				// Multi-tenancy support via custom tags.
				// Do not allow overriding AccountID and ProjectID from atCopy for security reasons.
				if tag.Key == "VictoriaMetrics_AccountID" {
					atCopy.AccountID = uint32(fastfloat.ParseUint64BestEffort(tag.Value))
				}
				if atCopy.ProjectID == 0 && tag.Key == "VictoriaMetrics_ProjectID" {
					atCopy.ProjectID = uint32(fastfloat.ParseUint64BestEffort(tag.Value))
				}
			}
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
		if err := ctx.WriteDataPoint(&atCopy, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}
	// Assume that all the rows for a single connection belong to the same (AccountID, ProjectID).
	rowsInserted.Add(len(rows))
	rowsTenantInserted.Get(&atCopy).Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
