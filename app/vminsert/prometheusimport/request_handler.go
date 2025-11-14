package prometheusimport

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="prometheus"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="prometheus"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="prometheus"}`)
	metadataInserted   = metrics.NewCounter(`vm_metadata_rows_inserted_total{type="prometheus"}`)
)

// InsertHandler processes `/api/v1/import/prometheus` request.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	defaultTimestamp, err := protoparserutil.GetTimestamp(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, defaultTimestamp, encoding, true, prommetadata.IsEnabled(), func(rows []prometheus.Row, mms []prometheus.Metadata) error {
		return insertRows(at, rows, mms, extraLabels)
	}, func(s string) {
		httpserver.LogError(req, s)
	})
}

func insertRows(at *auth.Token, rows []prometheus.Row, mms []prometheus.Metadata, extraLabels []prompb.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	perTenantRows := make(map[auth.Token]int)
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
		if !ctx.TryPrepareLabels(hasRelabeling) {
			continue
		}
		atLocal := ctx.GetLocalAuthToken(at)
		if err := ctx.WriteDataPoint(atLocal, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
		perTenantRows[*atLocal]++
	}
	rowsInserted.Add(len(rows))
	rowsTenantInserted.MultiAdd(perTenantRows)
	rowsPerInsert.Update(float64(len(rows)))
	if err := ctx.FlushBufs(); err != nil {
		return fmt.Errorf("cannot flush metric bufs: %w", err)
	}

	if prommetadata.IsEnabled() {
		ctx.ResetForMetricsMetadata()
		for i := range mms {
			m := &mms[i]
			mdr := metricsmetadata.Row{
				Type:             m.Type,
				MetricFamilyName: []byte(m.Metric),
				Help:             []byte(m.Help),
			}
			if at != nil {
				mdr.AccountID = at.AccountID
				mdr.ProjectID = at.ProjectID
			}
			ctx.Buf = mdr.MarshalTo(ctx.Buf[:0])
			storageNodeIdx := ctx.GetStorageNodeIdxForMeta(ctx.Buf)
			if err := ctx.WriteMetadataExt(storageNodeIdx, ctx.Buf); err != nil {
				return err
			}
		}
		metadataInserted.Add(len(mms))
		return ctx.FlushBufs()
	}
	return nil
}
