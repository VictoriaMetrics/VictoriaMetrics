package datadogv1

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="datadogv1"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="datadogv1"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series request.
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, func(series []datadogv1.Series) error {
		return insertRows(series, extraLabels)
	})
}

func insertRows(series []datadogv1.Series, extraLabels []prompb.Label) error {
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
		if ss.Host != "" {
			ctx.AddLabel("host", ss.Host)
		}
		if ss.Device != "" {
			ctx.AddLabel("device", ss.Device)
		}
		for _, tag := range ss.Tags {
			name, value := datadogutil.SplitTag(tag)
			if name == "host" {
				name = "exported_host"
			}
			ctx.AddLabel(name, value)
		}
		for j := range extraLabels {
			label := &extraLabels[j]
			ctx.AddLabel(label.Name, label.Value)
		}
		if !ctx.TryPrepareLabels(hasRelabeling) {
			continue
		}
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
