package prometheusimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="prometheus"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="prometheus"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="prometheus"}`)
)

// InsertHandler processes `/api/v1/import/prometheus` request.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	defaultTimestamp, err := parserCommon.GetTimestamp(req)
	if err != nil {
		return err
	}
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"
	return stream.Parse(req.Body, defaultTimestamp, isGzipped, true, func(rows []parser.Row) error {
		return insertRows(at, rows, extraLabels)
	}, func(s string) {
		httpserver.LogError(req, s)
	})
}

func insertRows(at *auth.Token, rows []parser.Row, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		labelsLen := len(labels)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		labels = append(labels, extraLabels...)
		samples = append(samples, prompbmarshal.Sample{
			Value:     r.Value,
			Timestamp: r.Timestamp,
		})
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[len(samples)-1:],
		})
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(len(rows))
	if at != nil {
		rowsTenantInserted.Get(at).Add(len(rows))
	}
	rowsPerInsert.Update(float64(len(rows)))
	return nil
}
