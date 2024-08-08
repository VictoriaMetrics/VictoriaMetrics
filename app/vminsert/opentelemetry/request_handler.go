package opentelemetry

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/firehose"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="opentelemetry"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="opentelemetry"}`)
)

// InsertHandler processes opentelemetry metrics.
func InsertHandler(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"
	var processBody func([]byte) ([]byte, error)
	contentType := req.Header.Get("Content-Type")
	if contentType == "application/json" {
		if req.Header.Get("X-Amz-Firehose-Protocol-Version") != "" {
			processBody = firehose.ProcessRequestBody
			contentType = "application/x-protobuf"
		}
	}
	return stream.ParseMetricsStream(req.Body, contentType, isGzipped, processBody, func(tss []prompbmarshal.TimeSeries) error {
		return insertRows(tss, extraLabels)
	})
}

func insertRows(tss []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range tss {
		rowsLen += len(tss[i].Samples)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range tss {
		ts := &tss[i]
		rowsTotal += len(ts.Samples)
		ctx.Labels = ctx.Labels[:0]
		for _, label := range ts.Labels {
			ctx.AddLabel(label.Name, label.Value)
		}
		for _, label := range extraLabels {
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
		samples := ts.Samples
		for i := range samples {
			r := &samples[i]
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, r.Timestamp, r.Value)
			if err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
