package datadog

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="datadog"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="datadog"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="datadog"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return writeconcurrencylimiter.Do(func() error {
		ce := req.Header.Get("Content-Encoding")
		err := parser.ParseStream(req.Body, ce, func(series []parser.Series) error {
			return insertRows(at, series, extraLabels)
		})
		if err != nil {
			return fmt.Errorf("headers: %q; err: %w", req.Header, err)
		}
		return nil
	})
}

func insertRows(at *auth.Token, series []parser.Series, extraLabels []prompbmarshal.Label) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset()
	rowsTotal := 0
	perTenantRows := make(map[auth.Token]int)
	hasRelabeling := relabel.HasRelabeling()
	for i := range series {
		ss := &series[i]
		rowsTotal += len(ss.Points)
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", ss.Metric)
		if ss.Host != "" {
			ctx.AddLabel("host", ss.Host)
		}
		if ss.Device != "" {
			ctx.AddLabel("device", ss.Device)
		}
		for _, tag := range ss.Tags {
			name, value := parser.SplitTag(tag)
			if name == "host" {
				name = "exported_host"
			}
			ctx.AddLabel(name, value)
		}
		for j := range extraLabels {
			label := &extraLabels[j]
			ctx.AddLabel(label.Name, label.Value)
		}
		if hasRelabeling {
			ctx.ApplyRelabeling()
		}
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.SortLabelsIfNeeded()
		atLocal := ctx.GetLocalAuthToken(at)
		ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], atLocal.AccountID, atLocal.ProjectID, ctx.Labels)
		storageNodeIdx := ctx.GetStorageNodeIdx(atLocal, ctx.Labels)
		for _, pt := range ss.Points {
			timestamp := pt.Timestamp()
			value := pt.Value()
			if err := ctx.WriteDataPointExt(storageNodeIdx, ctx.MetricNameBuf, timestamp, value); err != nil {
				return err
			}
		}
		perTenantRows[*atLocal] += len(ss.Points)
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.MultiAdd(perTenantRows)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
