package zabbixconnector

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="zabbixconnector"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="zabbixconnector"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="zabbixconnector"}`)
)

// InsertHandlerForHTTP processes remote write for ZabbixConnector POST /zabbixconnector/v1/history request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, encoding, func(rows []zabbixconnector.Row) error {
		return insertRows(at, rows, extraLabels)
	})
}

func insertRows(at *auth.Token, rows []zabbixconnector.Row, extraLabels []prompb.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	perTenantRows := make(map[auth.Token]int)
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		rowsTotal++
		ctx.Labels = ctx.Labels[:0]
		for k := range r.Tags {
			t := &r.Tags[k]
			ctx.AddLabelBytes(t.Key, t.Value)
		}
		for k := range extraLabels {
			label := &extraLabels[k]
			ctx.AddLabel(label.Name, label.Value)
		}
		ctx.TryPrepareLabels(hasRelabeling)
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		atLocal := ctx.GetLocalAuthToken(at)
		if err := ctx.WriteDataPoint(atLocal, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
		perTenantRows[*atLocal]++
	}

	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	rowsTenantInserted.MultiAdd(perTenantRows)

	return ctx.FlushBufs()
}
