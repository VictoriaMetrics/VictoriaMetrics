package datadog

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadog"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="datadog"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="datadog"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	return writeconcurrencylimiter.Do(func() error {
		ce := req.Header.Get("Content-Encoding")
		err := parser.ParseStream(req.Body, ce, func(series []parser.Series) error {
			return insertRows(series, extraLabels)
		})
		if err != nil {
			return fmt.Errorf("headers: %q; err: %w", req.Header, err)
		}
		return nil
	})
}

func insertRows(series []parser.Series, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range series {
		rowsLen += len(series[i].Points)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range series {
		ss := &series[i]
		rowsTotal += len(ss.Points)
		ctx.Labels = ctx.Labels[:0]
		ctx.AddLabel("", ss.Metric)
		ctx.AddLabel("host", ss.Host)
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
		var metricNameRaw []byte
		var err error
		for _, pt := range ss.Points {
			timestamp := pt.Timestamp()
			value := pt.Value()
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, timestamp, value)
			if err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
