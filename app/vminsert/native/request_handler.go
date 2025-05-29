package native

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="native"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="native"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="native"}`)
)

// InsertHandler processes `/api/v1/import/native` request.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, encoding, func(block *stream.Block) error {
		return insertRows(at, block, extraLabels)
	})
}

func insertRows(at *auth.Token, block *stream.Block, extraLabels []prompbmarshal.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	// Update rowsInserted and rowsPerInsert before actual inserting,
	// since relabeling can prevent from inserting the rows.
	rowsLen := len(block.Values)
	rowsInserted.Add(rowsLen)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsLen)
	}
	rowsPerInsert.Update(float64(rowsLen))

	ctx.Reset() // This line is required for initializing ctx internals.
	hasRelabeling := relabel.HasRelabeling()
	mn := &block.MetricName
	ctx.Labels = ctx.Labels[:0]
	ctx.AddLabelBytes(nil, mn.MetricGroup)
	for j := range mn.Tags {
		tag := &mn.Tags[j]
		ctx.AddLabelBytes(tag.Key, tag.Value)
	}
	for j := range extraLabels {
		label := &extraLabels[j]
		ctx.AddLabel(label.Name, label.Value)
	}
	if !ctx.TryPrepareLabels(hasRelabeling) {
		return nil
	}
	// use tenant info from data if it's a multi-tenant import.
	atLocal := ctx.GetLocalAuthToken(at)
	ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], atLocal.AccountID, atLocal.ProjectID, ctx.Labels)
	storageNodeIdx := ctx.GetStorageNodeIdx(atLocal, ctx.Labels)
	values := block.Values
	timestamps := block.Timestamps
	if len(timestamps) != len(values) {
		logger.Panicf("BUG: len(timestamps)=%d must match len(values)=%d", len(timestamps), len(values))
	}
	for j, value := range values {
		timestamp := timestamps[j]
		if err := ctx.WriteDataPointExt(storageNodeIdx, ctx.MetricNameBuf, timestamp, value); err != nil {
			return err
		}
	}
	if err := ctx.FlushBufs(); err != nil {
		return err
	}
	if at == nil {
		rowsTenantInserted.Get(atLocal).Add(rowsLen)
	}
	return nil
}
