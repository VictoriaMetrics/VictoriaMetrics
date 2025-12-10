package zabbixconnector

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="zabbixconnector"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="zabbixconnector"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="zabbixconnector"}`)
)

// InsertHandlerForHTTP processes remote write for ZabbixConnector POST /zabbixconnector/v1/history request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, encoding, func(rows []zabbixconnector.Row) error {
		return insertRows(at, rows, extraLabels)
	})
}

func insertRows(at *auth.Token, rows []zabbixconnector.Row, extraLabels []prompb.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := len(rows)
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]

		labelsLen := len(labels)
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompb.Label{
				Name:  bytesutil.ToUnsafeString(tag.Key),
				Value: bytesutil.ToUnsafeString(tag.Value),
			})
		}
		labels = append(labels, extraLabels...)

		samplesLen := len(samples)
		samples = append(samples, prompb.Sample{
			Value:     r.Value,
			Timestamp: r.Timestamp,
		})

		tssDst = append(tssDst, prompb.TimeSeries{
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
