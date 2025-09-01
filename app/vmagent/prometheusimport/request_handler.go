package prometheusimport

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted           = metrics.NewCounter(`vmagent_rows_inserted_total{type="prometheus"}`)
	metadataInserted       = metrics.NewCounter(`vmagent_metadata_inserted_total{type="prometheus"}`)
	rowsTenantInserted     = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="prometheus"}`)
	metadataTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_metadata_total{type="prometheus"}`)

	rowsPerInsert = metrics.NewHistogram(`vmagent_rows_per_insert{type="prometheus"}`)
)

// InsertHandler processes `/api/v1/import/prometheus` request.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	defaultTimestamp, err := protoparserutil.GetTimestamp(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, defaultTimestamp, encoding, true, prommetadata.IsEnabled(), func(rows []prometheus.Row, mms []prometheus.Metadata) error {
		return insertRows(at, rows, mms, extraLabels)
	}, func(s string) {
		httpserver.LogError(req, s)
	})
}

func insertRows(at *auth.Token, rows []prometheus.Row, mms []prometheus.Metadata, extraLabels []prompb.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	tssDst := ctx.WriteRequest.Timeseries[:0]
	mmsDst := ctx.WriteRequest.Metadata[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		labelsLen := len(labels)
		labels = append(labels, prompb.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompb.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		labels = append(labels, extraLabels...)
		samples = append(samples, prompb.Sample{
			Value:     r.Value,
			Timestamp: r.Timestamp,
		})
		tssDst = append(tssDst, prompb.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[len(samples)-1:],
		})
	}
	var accountID, projectID uint32
	if at != nil {
		accountID = at.AccountID
		projectID = at.ProjectID
	}
	for i := range mms {
		mm := &mms[i]
		mmsDst = append(mmsDst, prompb.MetricMetadata{
			MetricFamilyName: mm.Metric,
			Help:             mm.Help,
			Type:             mm.Type,
			// there is no unit in Prometheus exposition formats

			AccountID: accountID,
			ProjectID: projectID,
		})
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.WriteRequest.Metadata = mmsDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(len(rows))
	metadataInserted.Add(len(mms))
	if at != nil {
		rowsTenantInserted.Get(at).Add(len(rows))
		metadataTenantInserted.Get(at).Add(len(mms))
	}
	rowsPerInsert.Update(float64(len(rows)))
	return nil
}
