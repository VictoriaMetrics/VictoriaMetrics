package datadogv2

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv2/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="datadogv2"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="datadogv2"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v2/series request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ct := req.Header.Get("Content-Type")
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, ct, func(series []datadogv2.Series) error {
		return insertRows(series, extraLabels)
	})
}

func insertRows(series []datadogv2.Series, extraLabels []prompbmarshal.Label) error {
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
		for _, rs := range ss.Resources {
			ctx.AddLabel(rs.Type, rs.Name)
		}
		for _, tag := range ss.Tags {
			name, value := datadogutils.SplitTag(tag)
			if name == "host" {
				name = "exported_host"
			}
			ctx.AddLabel(name, value)
		}
		if ss.SourceTypeName != "" {
			ctx.AddLabel("source_type_name", ss.SourceTypeName)
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
			timestamp := pt.Timestamp * 1000
			value := pt.Value
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
