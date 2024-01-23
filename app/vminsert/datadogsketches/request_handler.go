package datadogsketches

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="datadogsketches"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="datadogsketches"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/beta/sketches request.
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, func(sketches []*datadogsketches.Sketch) error {
		return insertRows(sketches, extraLabels)
	})
}

func insertRows(sketches []*datadogsketches.Sketch, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range sketches {
		sketch := sketches[i]
		rowsLen += sketch.RowsCount()
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range sketches {
		sketch := sketches[i]
		metrics := sketch.ToHistogram()
		rowsTotal += sketch.RowsCount()
		for m := range metrics {
			metric := metrics[m]
			ctx.Labels = ctx.Labels[:0]
			ctx.AddLabel("", metric.Name)
			for l := range metric.Labels {
				label := metric.Labels[l]
				ctx.AddLabel(label.Name, label.Value)
			}
			for _, tag := range sketch.Tags {
				name, value := datadogutils.SplitTag(tag)
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
			for p := range metric.Points {
				value := metric.Points[p]
				timestamp := sketch.Dogsketches[p].Ts * 1000
				metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, timestamp, value)
				if err != nil {
					return err
				}
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
