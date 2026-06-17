package protoparser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkWriteRequest_UnmarshalProtobuf(b *testing.B) {
	var data = make([]byte, 0, 21_000_000)

	f := func(rows, labels, labelSize, groupBy int) {
		bName := fmt.Sprintf("Rows=%d/Labels=%d/LabelSize=%d/GroupBy=%d", rows, labels, labelSize, groupBy)
		b.Run(bName, func(b *testing.B) {
			data := buildEncodedWriteRequest(data, rows, labels, labelSize, groupBy)
			groupLabels := []string{
				"foo",
				"bar",
				"baz",
				"__name__",
				"job",
				"groupLabel",
			}

			wru := getWriteRequestUnmarshaler()
			cnt := 0

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				wru.Reset()
				if err := wru.UnmarshalProtobuf(data, groupLabels, func(tss []TimeSerie) {
					cnt += len(tss)
				}); err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
			}
		})
	}

	f(5_000, 0, 0, 3)
	f(5_000, 1, 20, 3)

	f(1_000, 20, 20, 3)
	f(5_000, 20, 20, 3)
	f(10_000, 20, 20, 3)
	f(20_000, 20, 20, 3)

	// long label values
	f(1_000, 20, 2000, 3)

	// many labels
	f(1_000, 2000, 100, 3)
}

// buildEncodedWriteRequest builds a snappy-encoded protobuf WriteRequest
// with numSeries time series, each having numLabels labels of labelSize bytes each.
func buildEncodedWriteRequest(dst []byte, numSeries, numLabels, labelSize, groupsNum int) []byte {
	labelValue := strings.Repeat("x", labelSize)

	tss := make([]prompb.TimeSeries, numSeries)
	for i := range tss {
		labels := make([]prompb.Label, numLabels)
		for j := range labels {
			labels[j] = prompb.Label{
				Name:  fmt.Sprintf("label%02d", j),
				Value: fmt.Sprintf("val%05d_%s", i, labelValue),
			}
		}
		labels = append(labels, prompb.Label{
			Name:  "groupLabel",
			Value: fmt.Sprintf("%d", i%groupsNum),
		})

		tss[i] = prompb.TimeSeries{
			Labels:  labels,
			Samples: []prompb.Sample{{Value: 1, Timestamp: 1000}},
		}
	}

	wr := &prompb.WriteRequest{Timeseries: tss}
	return wr.MarshalProtobuf(dst[:0])
}
