package promremotewrite

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

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

		if !ctx.TryPrepareLabels(hasRelabeling) {
			continue
		}
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
			if len(r.PositiveSpans) == 0 && len(r.NegativeSpans) == 0 && r.ZeroCount == 0 {
				leIndex := len(ctx.Labels)
				ctx.AddLabel("le", "0")
				for _, b := range r.Buckets {
					cumulativeCount := float64(b.CumulativeCount)
					if cumulativeCount == 0 {
						cumulativeCount = b.CumulativeCountFloat
					}
					ctx.Labels[leIndex].Value = strconv.FormatFloat(b.UpperBound, 'g', 3, 64)
					if _, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, cumulativeCount); err != nil {
						return err
					}
				}
			} else {
				vmRangeIndex := len(ctx.Labels)
				ctx.AddLabel("vmrange", "...")

				if r.ZeroCount > 0 {
					ctx.Labels[vmRangeIndex].Value = fmt.Sprintf("%0.3e...%0.3e", 0.0, r.ZeroThreshold)
					if _, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, float64(r.ZeroCount)); err != nil {
						return err
					}
				}

				ratio := math.Pow(2, -float64(r.Schema))
				base := math.Pow(2, ratio)

				var (
					value  float64
					idx    int
					offset float64
				)

				deltas := r.PositiveDeltas
				for _, span := range r.PositiveSpans {
					offset += float64(span.Offset)
					bound := math.Pow(2, offset*ratio)
					for l := 0; l < int(span.Length); l++ {
						value += float64(deltas[l+idx])
						if value > 0 {
							lowerBound := bound * math.Pow(base, float64(l+idx-1))
							upperBound := lowerBound * base
							ctx.Labels[vmRangeIndex].Value = fmt.Sprintf("%0.3e...%0.3e", lowerBound, upperBound)
							if _, err = ctx.WriteDataPointExt(nil, ctx.Labels, r.Timestamp, value); err != nil {
								return err
							}
						}
					}
					idx += int(span.Length)
				}
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
