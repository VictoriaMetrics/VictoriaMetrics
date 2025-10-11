package promremotewrite

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted           = metrics.NewCounter(`vmagent_rows_inserted_total{type="promremotewrite"}`)
	metadataInserted       = metrics.NewCounter(`vmagent_metadata_inserted_total{type="promremotewrite"}`)
	rowsTenantInserted     = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="promremotewrite"}`)
	metadataTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_metadata_total{type="promremotewrite"}`)
	rowsPerInsert          = metrics.NewHistogram(`vmagent_rows_per_insert{type="promremotewrite"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isVMRemoteWrite := req.Header.Get("Content-Encoding") == "zstd"
	return stream.Parse(req.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(at, tss, mms, extraLabels)
	})
}

func insertRows(at *auth.Token, timeseries []prompb.TimeSeries, mms []prompb.MetricMetadata, extraLabels []prompb.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	mmsDst := ctx.WriteRequest.Metadata[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range timeseries {
		ts := &timeseries[i]
		rowsTotal += len(ts.Samples)
		labelsLen := len(labels)
		for i := range ts.Labels {
			label := &ts.Labels[i]
			labels = append(labels, prompb.Label{
				Name:  label.Name,
				Value: label.Value,
			})
		}
		labels = append(labels, extraLabels...)
		samplesLen := len(samples)
		for i := range ts.Samples {
			sample := &ts.Samples[i]
			samples = append(samples, prompb.Sample{
				Value:     sample.Value,
				Timestamp: sample.Timestamp,
			})
		}
		tssDst = append(tssDst, prompb.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[samplesLen:],
		})
	}
	ctx.WriteRequest.Timeseries = tssDst

	var metadataTotal int
	if prommetadata.IsEnabled() {
		var accountID, projectID uint32
		if at != nil {
			accountID = at.AccountID
			projectID = at.ProjectID
		}
		for i := range mms {
			mm := &mms[i]
			mmsDst = append(mmsDst, prompb.MetricMetadata{
				MetricFamilyName: mm.MetricFamilyName,
				Help:             mm.Help,
				Type:             mm.Type,
				Unit:             mm.Unit,

				AccountID: accountID,
				ProjectID: projectID,
			})
		}
		ctx.WriteRequest.Metadata = mmsDst
		metadataTotal = len(mms)
	}

	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(rowsTotal)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsTotal)
		metadataTenantInserted.Get(at).Add(metadataTotal)
	}
	metadataInserted.Add(metadataTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return nil
}
