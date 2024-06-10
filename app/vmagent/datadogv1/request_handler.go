package datadogv1

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="datadogv1"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="datadogv1"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="datadogv1"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, func(series []datadogv1.Series) error {
		return insertRows(at, series, extraLabels)
	})
}

func insertRows(at *auth.Token, series []datadogv1.Series, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range series {
		ss := &series[i]
		rowsTotal += len(ss.Points)
		labelsLen := len(labels)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: ss.Metric,
		})
		if ss.Host != "" {
			labels = append(labels, prompbmarshal.Label{
				Name:  "host",
				Value: ss.Host,
			})
		}
		if ss.Device != "" {
			labels = append(labels, prompbmarshal.Label{
				Name:  "device",
				Value: ss.Device,
			})
		}
		for _, tag := range ss.Tags {
			name, value := datadogutils.SplitTag(tag)
			if name == "host" {
				name = "exported_host"
			}
			labels = append(labels, prompbmarshal.Label{
				Name:  name,
				Value: value,
			})
		}
		labels = append(labels, extraLabels...)
		samplesLen := len(samples)
		for _, pt := range ss.Points {
			samples = append(samples, prompbmarshal.Sample{
				Timestamp: pt.Timestamp(),
				Value:     pt.Value(),
			})
		}
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
