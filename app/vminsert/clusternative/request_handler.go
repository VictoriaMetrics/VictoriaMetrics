package clusternative

import (
	"fmt"
	"net"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/clusternative"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="clusternative"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="clusternative"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="clusternative"}`)
)

// InsertHandler processes data from vminsert nodes.
func InsertHandler(c net.Conn) error {
	bc, err := handshake.VMInsertServer(c, 0)
	if err != nil {
		return fmt.Errorf("cannot perform vminsert handshake with client %q: %w", c.RemoteAddr(), err)
	}
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(bc, func(rows []storage.MetricRow) error {
			return insertRows(rows)
		}, nil)
	})
}

func insertRows(rows []storage.MetricRow) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	hasRelabeling := relabel.HasRelabeling()
	var at auth.Token
	var rowsPerTenant *metrics.Counter
	var mn storage.MetricName
	for i := range rows {
		mr := &rows[i]
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			return fmt.Errorf("cannot unmarshal MetricNameRaw: %w", err)
		}
		if rowsPerTenant == nil || mn.AccountID != at.AccountID || mn.ProjectID != at.ProjectID {
			at.AccountID = mn.AccountID
			at.ProjectID = mn.ProjectID
			rowsPerTenant = rowsTenantInserted.Get(&at)
		}
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabelBytes(nil, mn.MetricGroup)
		for j := range mn.Tags {
			tag := &mn.Tags[j]
			ctx.AddLabelBytes(tag.Key, tag.Value)
		}
		if hasRelabeling {
			ctx.ApplyRelabeling()
		}
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.SortLabelsIfNeeded()
		if err := ctx.WriteDataPoint(&at, ctx.Labels, mr.Timestamp, mr.Value); err != nil {
			return err
		}
		rowsPerTenant.Inc()
	}
	rowsInserted.Add(len(rows))
	rowsPerInsert.Update(float64(len(rows)))
	return ctx.FlushBufs()
}
