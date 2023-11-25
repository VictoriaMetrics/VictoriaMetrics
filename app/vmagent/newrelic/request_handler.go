package newrelic

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="newrelic"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="newrelic"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="newrelic"}`)
)

// InsertHandlerForHTTP processes remote write for NewRelic POST /infra/v2/metrics/events/bulk request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	isGzip := ce == "gzip"
	return stream.Parse(req.Body, isGzip, func(rows []newrelic.Row) error {
		return insertRows(at, rows, extraLabels)
	})
}

func insertRows(at *auth.Token, rows []newrelic.Row, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	samplesCount := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		tags := r.Tags
		srcSamples := r.Samples
		for j := range srcSamples {
			s := &srcSamples[j]
			labelsLen := len(labels)
			labels = append(labels, prompbmarshal.Label{
				Name:  "__name__",
				Value: bytesutil.ToUnsafeString(s.Name),
			})
			for k := range tags {
				t := &tags[k]
				labels = append(labels, prompbmarshal.Label{
					Name:  bytesutil.ToUnsafeString(t.Key),
					Value: bytesutil.ToUnsafeString(t.Value),
				})
			}
			samples = append(samples, prompbmarshal.Sample{
				Value:     s.Value,
				Timestamp: r.Timestamp,
			})
			tssDst = append(tssDst, prompbmarshal.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: samples[len(samples)-1:],
			})
			labels = append(labels, extraLabels...)
		}
		samplesCount += len(srcSamples)
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(len(rows))
	if at != nil {
		rowsTenantInserted.Get(at).Add(samplesCount)
	}
	rowsPerInsert.Update(float64(samplesCount))
	return nil
}
