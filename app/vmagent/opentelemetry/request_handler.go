package opentelemetry

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="opentelemetry"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="opentelemetry"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="opentelemetry"}`)
)

// InsertHandler processes opentelemetry metrics.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"
	var processBody func([]byte) ([]byte, error)
	contentType := req.Header.Get("Content-Type")
	if contentType == "application/json" {
		if req.Header.Get("X-Amz-Firehose-Protocol-Version") != "" {
			processBody = firehose.ProcessRequestBody
			contentType = "application/x-protobuf"
		}
	}
	return stream.ParseMetricsStream(req.Body, contentType, isGzipped, processBody, func(tss []prompbmarshal.TimeSeries) error {
		return insertRows(at, tss, extraLabels)
	})
}

func insertRows(at *auth.Token, tss []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range tss {
		ts := &tss[i]
		rowsTotal += len(ts.Samples)
		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		labels = append(labels, extraLabels...)
		samplesLen := len(samples)
		samples = append(samples, ts.Samples...)
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[samplesLen:],
		})
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
