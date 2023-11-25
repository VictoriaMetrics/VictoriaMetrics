package native

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="native"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="native"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="native"}`)
)

// InsertHandler processes `/api/v1/import` request.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isGzip := req.Header.Get("Content-Encoding") == "gzip"
	return stream.Parse(req.Body, isGzip, func(block *stream.Block) error {
		return insertRows(at, block, extraLabels)
	})
}

func insertRows(at *auth.Token, block *stream.Block, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	// Update rowsInserted and rowsPerInsert before actual inserting,
	// since relabeling can prevent from inserting the rows.
	rowsLen := len(block.Values)
	rowsInserted.Add(rowsLen)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsLen)
	}
	rowsPerInsert.Update(float64(rowsLen))

	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	mn := &block.MetricName
	labelsLen := len(labels)
	labels = append(labels, prompbmarshal.Label{
		Name:  "__name__",
		Value: bytesutil.ToUnsafeString(mn.MetricGroup),
	})
	for j := range mn.Tags {
		tag := &mn.Tags[j]
		labels = append(labels, prompbmarshal.Label{
			Name:  bytesutil.ToUnsafeString(tag.Key),
			Value: bytesutil.ToUnsafeString(tag.Value),
		})
	}
	labels = append(labels, extraLabels...)
	values := block.Values
	timestamps := block.Timestamps
	if len(timestamps) != len(values) {
		logger.Panicf("BUG: len(timestamps)=%d must match len(values)=%d", len(timestamps), len(values))
	}
	samplesLen := len(samples)
	for j, value := range values {
		samples = append(samples, prompbmarshal.Sample{
			Value:     value,
			Timestamp: timestamps[j],
		})
	}
	tssDst = append(tssDst, prompbmarshal.TimeSeries{
		Labels:  labels[labelsLen:],
		Samples: samples[samplesLen:],
	})
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	return nil
}
