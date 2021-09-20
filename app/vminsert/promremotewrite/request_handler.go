package promremotewrite

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="promremotewrite"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req.Body, func(tss []prompb.TimeSeries) error {
			return insertRows(at, tss, extraLabels)
		})
	})
}

func insertRows(at *auth.Token, timeseries []prompb.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range timeseries {
		ts := &timeseries[i]
		rowsTotal += len(ts.Samples)
		ctx.Labels = ctx.Labels[:0]
		srcLabels := ts.Labels
		for _, srcLabel := range srcLabels {
			ctx.AddLabelBytes(srcLabel.Name, srcLabel.Value)
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
		storageNodeIdx := ctx.GetStorageNodeIdx(at, ctx.Labels)
		ctx.MetricNameBuf = ctx.MetricNameBuf[:0]
		samples := ts.Samples
		for i := range samples {
			r := &samples[i]
			if len(ctx.MetricNameBuf) == 0 {
				ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, ctx.Labels)
			}
			if err := ctx.WriteDataPointExt(at, storageNodeIdx, ctx.MetricNameBuf, r.Timestamp, r.Value); err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.Get(at).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
