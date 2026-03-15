package opentelemetry

import (
	"fmt"
	"io"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted           = metrics.NewCounter(`vmagent_rows_inserted_total{type="opentelemetry"}`)
	metadataInserted       = metrics.NewCounter(`vmagent_metadata_inserted_total{type="opentelemetry"}`)
	rowsTenantInserted     = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="opentelemetry"}`)
	metadataTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_metadata_total{type="opentelemetry"}`)
	rowsPerInsert          = metrics.NewHistogram(`vmagent_rows_per_insert{type="opentelemetry"}`)
)

// InsertHandlerForReader processes metrics from given reader.
func InsertHandlerForReader(at *auth.Token, r io.Reader, encoding string) error {
	return stream.ParseStream(r, encoding, nil, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(at, tss, mms, nil)
	})
}

// InsertHandler processes opentelemetry metrics.
func InsertHandler(at *auth.Token, req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	var processBody func([]byte) ([]byte, error)
	if req.Header.Get("Content-Type") == "application/json" {
		if req.Header.Get("X-Amz-Firehose-Protocol-Version") != "" {
			processBody = firehose.ProcessRequestBody
		} else {
			return fmt.Errorf("json encoding isn't supported for opentelemetry format. Use protobuf encoding")
		}
	}
	return stream.ParseStream(req.Body, encoding, processBody, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(at, tss, mms, extraLabels)
	})
}

func insertRows(at *auth.Token, tss []prompb.TimeSeries, mms []prompb.MetricMetadata, extraLabels []prompb.Label) error {
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
			for i := range mms {
				mm := &mms[i]
				mm.AccountID = accountID
				mm.ProjectID = projectID
			}
		}
		ctx.WriteRequest.Metadata = mms
		metadataTotal = len(mms)
	}

	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(rowsTotal)
	metadataInserted.Add(metadataTotal)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsTotal)
		metadataTenantInserted.Get(at).Add(metadataTotal)
	}
	rowsPerInsert.Update(float64(rowsTotal))
	return nil
}
