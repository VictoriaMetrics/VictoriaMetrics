package promremotewrite

import (
	"math"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite/stream"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isVMRemoteWrite := req.Header.Get("Content-Encoding") == "zstd"
	return stream.Parse(req.Body, isVMRemoteWrite, func(tss []prompb.TimeSeries) error {
		return insertRows(tss, extraLabels)
	})
}

func insertRows(timeseries []prompb.TimeSeries, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsLen := 0
	for i := range timeseries {
		rowsLen += len(timeseries[i].Samples)
	}
	ctx.Reset(rowsLen)
	rowsTotal := 0
	hasRelabeling := relabel.HasRelabeling()
	for i := range timeseries {
		ts := &timeseries[i]
		rowsTotal += len(ts.Samples)
		ctx.Labels = ctx.Labels[:0]
		srcLabels := ts.Labels
		var nameIndex int
		for _, srcLabel := range srcLabels {
			if srcLabel.Name == "__name__" {
				nameIndex = len(ctx.Labels)
			}
			ctx.AddLabel(srcLabel.Name, srcLabel.Value)
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
		samples := ts.Samples
		for i := range samples {
			r := &samples[i]
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, ctx.Labels, r.Timestamp, r.Value)
			if err != nil {
				return err
			}
		}
		exemplars := ts.Exemplars
		for i := range exemplars {
			r := &exemplars[i]
			labels := append(ctx.Labels, r.Labels...)
			metricNameRaw, err = ctx.WriteDataPointExt(metricNameRaw, labels, r.Timestamp, r.Value)
			if err != nil {
				return err
			}
		}
		histograms := ts.Histograms
		for i := range histograms {
			r := &histograms[i]
			count := float64(r.Count)
			if count == 0 {
				count = r.CountFloat
			}
			metricPrefix := ctx.Labels[nameIndex].Value
			ctx.Labels[nameIndex].Value = metricPrefix + "_count"
			_, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, count)
			if err != nil {
				return err
			}
			ctx.Labels[nameIndex].Value = metricPrefix + "_sum"
			_, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, r.Sum)
			if err != nil {
				return err
			}
			ctx.Labels[nameIndex].Value = metricPrefix + "_bucket"
			ratio := math.Pow(2, math.Pow(2, -float64(r.Schema)))
			var lowerBound float64 = 1
			var upperBound float64 = 1
			var value float64

			vmRangeIndex := len(ctx.Labels)
			ctx.AddLabel("vmrange", "...")
			vmRanges := make(map[string]float64)
			for s := range r.PositiveSpans {
				span := r.PositiveSpans[s]
				upperBound = upperBound * math.Pow(ratio, float64(span.Offset-1))
				for l := 0; l < int(span.Length); l++ {
					value += float64(r.PositiveDeltas[l+s])
					lowerBound = upperBound
					upperBound = lowerBound * ratio
					metrics.ConvertToVMRange(vmRanges, value, lowerBound, upperBound)
				}
			}
			for vmRange, vmValue := range vmRanges {
				ctx.Labels[vmRangeIndex].Value = vmRange
				_, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, vmValue)
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
