package opentelemetry

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="opentelemetry"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="opentelemetry"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="opentelemetry"}`)
	metadataInserted   = metrics.NewCounter(`vm_metadata_rows_inserted_total{type="opentelemetry"}`)
)

// InsertHandler processes opentelemetry metrics.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	var processBody func([]byte) ([]byte, error)
	if req.Header.Get("Content-Type") == "application/json" {
		if req.Header.Get("X-Amz-Firehose-Protocol-Version") != "" {
			processBody = firehose.ProcessRequestBody
		} else {
			return fmt.Errorf("json encoding isn't supported for opentelemetry format. Use protobuf encoding")
		}
	}
	return stream.ParseStream(req.Body, encoding, processBody, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(at, tss, mms, extraLabels)
	})
}

func insertRows(at *auth.Token, tss []prompb.TimeSeries, mms []prompb.MetricMetadata, extraLabels []prompb.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	perTenantRows := make(map[auth.Token]int)
	hasRelabeling := relabel.HasRelabeling()
	for i := range tss {
		ts := &tss[i]
		rowsTotal += len(ts.Samples)
		ctx.Labels = ctx.Labels[:0]
		for _, label := range ts.Labels {
			ctx.AddLabel(label.Name, label.Value)
		}
		for _, label := range extraLabels {
			ctx.AddLabel(label.Name, label.Value)
		}
		if !ctx.TryPrepareLabels(hasRelabeling) {
			continue
		}
		atLocal := ctx.GetLocalAuthToken(at)
		storageNodeIdx := ctx.GetStorageNodeIdx(atLocal, ctx.Labels)
		ctx.Buf = ctx.Buf[:0]
		samples := ts.Samples
		for i := range samples {
			r := &samples[i]
			if len(ctx.Buf) == 0 {
				ctx.Buf = storage.MarshalMetricNameRaw(ctx.Buf[:0], atLocal.AccountID, atLocal.ProjectID, ctx.Labels)
			}
			if err := ctx.WriteDataPointExt(storageNodeIdx, ctx.Buf, r.Timestamp, r.Value); err != nil {
				return err
			}
		}
		perTenantRows[*atLocal] += len(ts.Samples)
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.MultiAdd(perTenantRows)
	rowsPerInsert.Update(float64(rowsTotal))
	if err := ctx.FlushBufs(); err != nil {
		return fmt.Errorf("cannot flush metric bufs: %w", err)
	}
	if prommetadata.IsEnabled() {
		ctx.ResetForMetricsMetadata()
		for i := range mms {
			m := &mms[i]
			atLocal := ctx.GetLocalAuthTokenForMetadata(at, m)
			if err := ctx.WriteMetadata(atLocal, m); err != nil {
				return err
			}
		}
		metadataInserted.Add(len(mms))
		return ctx.FlushBufs()
	}
	return nil
}
