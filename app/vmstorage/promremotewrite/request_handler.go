package promremotewrite

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="promremotewrite"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
	metadataInserted   = metrics.NewCounter(`vm_metadata_rows_inserted_total{type="promremotewrite"}`)
)

// Storage is used for writing Prometheus remote write data into vmstorage.
type Storage interface {
	WriteRows(rows []storage.MetricRow) error
	WriteMetadata(rows []metricsmetadata.Row) error
	IsReadOnly() bool
}

// InsertHandler processes Prometheus remote write requests.
func InsertHandler(at *auth.Token, req *http.Request, s Storage) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isVMRemoteWrite := req.Header.Get("Content-Encoding") == "zstd"
	return stream.Parse(req.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
		return insertRows(s, at, tss, mms, extraLabels)
	})
}

func insertRows(s Storage, at *auth.Token, tss []prompb.TimeSeries, mms []prompb.MetricMetadata, extraLabels []prompb.Label) error {
	if s.IsReadOnly() {
		return errReadOnly()
	}
	ctx := getInsertCtx()
	defer putInsertCtx(ctx)

	rowsLen := 0
	for i := range tss {
		rowsLen += len(tss[i].Samples)
	}
	if rowsLen > 0 {
		ctx.rows = ctx.rows[:0]
		if cap(ctx.rows) < rowsLen {
			ctx.rows = make([]storage.MetricRow, 0, rowsLen)
		}
		ctx.metricNameBuf = ctx.metricNameBuf[:0]
		perTenantRows := make(map[auth.Token]int)
		for i := range tss {
			ts := &tss[i]
			ctx.labels = ctx.labels[:0]
			for j := range ts.Labels {
				label := &ts.Labels[j]
				ctx.addLabel(label.Name, label.Value)
			}
			for j := range extraLabels {
				label := &extraLabels[j]
				ctx.addLabel(label.Name, label.Value)
			}
			if !ctx.tryPrepareLabels() {
				continue
			}
			atLocal := ctx.getLocalAuthToken(at)
			var metricNameRaw []byte
			for j := range ts.Samples {
				sample := &ts.Samples[j]
				if len(metricNameRaw) == 0 {
					metricNameRaw = ctx.marshalMetricNameRaw(atLocal, ctx.labels)
				}
				if err := ctx.addRow(s, metricNameRaw, sample.Timestamp, sample.Value); err != nil {
					return err
				}
				if len(ctx.metricNameBuf) == 0 {
					metricNameRaw = nil
				}
			}
			perTenantRows[*atLocal] += len(ts.Samples)
		}

		rowsInserted.Add(rowsLen)
		rowsTenantInserted.MultiAdd(perTenantRows)
		rowsPerInsert.Update(float64(rowsLen))
		if err := ctx.flushRows(s); err != nil {
			return err
		}
	}

	if prommetadata.IsEnabled() && len(mms) > 0 {
		ctx.metadataRows = ctx.metadataRows[:0]
		if cap(ctx.metadataRows) < len(mms) {
			ctx.metadataRows = make([]metricsmetadata.Row, 0, len(mms))
		}
		for i := range mms {
			mm := &mms[i]
			if timeserieslimits.IsMetricMetadataExceeding(mm) {
				continue
			}
			atLocal := ctx.getLocalAuthTokenForMetadata(at, mm)
			ctx.metadataRows = append(ctx.metadataRows, metricsmetadata.Row{
				MetricFamilyName: []byte(mm.MetricFamilyName),
				Help:             []byte(mm.Help),
				Unit:             []byte(mm.Unit),
				AccountID:        atLocal.AccountID,
				ProjectID:        atLocal.ProjectID,
				Type:             mm.Type,
			})
		}
		metadataInserted.Add(len(ctx.metadataRows))
		if err := s.WriteMetadata(ctx.metadataRows); err != nil {
			return fmt.Errorf("cannot write metadata: %w", err)
		}
	}

	return nil
}

func errReadOnly() error {
	// In Prometheus remote write protocol, 400 means the request is invalid and must not be retried.
	// vmagent follows the protocol and drops data blocks if it gets 400,
	// To avoid data loss, vmstorage in read-only mode should return 503 for retrying instead of 400.
	// See https://prometheus.io/docs/specs/prw/remote_write_spec/#retries--backoff.
	return &httpserver.ErrorWithStatusCode{
		Err:        storage.ErrReadOnly,
		StatusCode: http.StatusServiceUnavailable,
	}
}
