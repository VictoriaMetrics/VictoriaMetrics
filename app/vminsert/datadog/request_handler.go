package datadog

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="datadog"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="datadog"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="datadog"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series, /api/v2/series, /api/v1/sketches, /api/beta/sketches request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return stream.Parse(
		req, func(series prompbmarshal.TimeSeries) error {
			series.Labels = append(series.Labels, extraLabels...)
			return insertRows(at, series)
		},
	)
}

func insertRows(at *auth.Token, series prompbmarshal.TimeSeries) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset()
	perTenantRows := make(map[auth.Token]int)
	hasRelabeling := relabel.HasRelabeling()
	rowsTotal := len(series.Samples)

	ctx.Labels = ctx.Labels[:0]
	for l := range series.Labels {
		ctx.AddLabel(series.Labels[l].Name, series.Labels[l].Value)
	}
	if hasRelabeling {
		ctx.ApplyRelabeling()
	}
	if len(ctx.Labels) == 0 {
		return nil
	}
	ctx.SortLabelsIfNeeded()
	atLocal := ctx.GetLocalAuthToken(at)
	ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], atLocal.AccountID, atLocal.ProjectID, ctx.Labels)
	storageNodeIdx := ctx.GetStorageNodeIdx(atLocal, ctx.Labels)
	for _, sample := range series.Samples {
		err := ctx.WriteDataPointExt(storageNodeIdx, ctx.MetricNameBuf, sample.Timestamp, sample.Value)
		if err != nil {
			return err
		}
	}
	perTenantRows[*atLocal] = rowsTotal
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.MultiAdd(perTenantRows)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
