package datadogsketches

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="datadogsketches"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="datadogsketches"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="datadogsketches"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/beta/sketches request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, func(sketches []*datadogsketches.Sketch) error {
		return insertRows(at, sketches, extraLabels)
	})
}

func insertRows(at *auth.Token, sketches []*datadogsketches.Sketch, extraLabels []prompbmarshal.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset()
	rowsTotal := 0
	perTenantRows := make(map[auth.Token]int)
	hasRelabeling := relabel.HasRelabeling()
	for _, sketch := range sketches {
		ms := sketch.ToSummary()
		for _, m := range ms {
			ctx.Labels = ctx.Labels[:0]
			ctx.AddLabel("", m.Name)
			for _, label := range m.Labels {
				ctx.AddLabel(label.Name, label.Value)
			}
			for _, tag := range sketch.Tags {
				name, value := datadogutils.SplitTag(tag)
				if name == "host" {
					name = "exported_host"
				}
				ctx.AddLabel(name, value)
			}
			for j := range extraLabels {
				label := &extraLabels[j]
				ctx.AddLabel(label.Name, label.Value)
			}
			if !ctx.TryPrepareLabels(hasRelabeling) {
				continue
			}
			atLocal := ctx.GetLocalAuthToken(at)
			ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], atLocal.AccountID, atLocal.ProjectID, ctx.Labels)
			storageNodeIdx := ctx.GetStorageNodeIdx(atLocal, ctx.Labels)
			for _, p := range m.Points {
				if err := ctx.WriteDataPointExt(storageNodeIdx, ctx.MetricNameBuf, p.Timestamp, p.Value); err != nil {
					return err
				}
			}
			rowsTotal += len(m.Points)
			perTenantRows[*atLocal] += len(m.Points)
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.MultiAdd(perTenantRows)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
