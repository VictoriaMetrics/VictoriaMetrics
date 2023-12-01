package datadog

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="datadog"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="datadog"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="datadog"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series, /api/v2/series, /api/beta/sketches request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return stream.Parse(
		req, func(series []prompbmarshal.TimeSeries) error {
			return insertRows(at, series, extraLabels)
		},
	)
}

func insertRows(at *auth.Token, series []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	var rowsTotal int
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]

	for i := range series {
		s := &series[i]
		rowsTotal += len(s.Samples)

		s.Labels = append(s.Labels, extraLabels...)
		tssDst = append(tssDst, *s)

		labels = append(labels, s.Labels...)
		samples = append(samples, s.Samples...)
	}

	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(rowsTotal)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsTotal)
	}
	rowsPerInsert.Update(float64(rowsTotal))
	return nil
}
