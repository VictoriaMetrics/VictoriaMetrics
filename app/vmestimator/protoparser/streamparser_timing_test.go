package protoparser

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/golang/snappy"
)

func BenchmarkParse(b *testing.B) {
	data := buildSnappyEncodedWriteRequest(5000, 20, 20, 3)
	groupLabels := []string{
		"foo",
		"bar",
		"baz",
		"__name__",
		"job",
		"groupLabel",
	}

	var cnt int

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		err := Parse(bytes.NewReader(data), groupLabels, func(tss []TimeSerie) {
			cnt += len(tss)
		})
		if err != nil {
			b.Fatalf("stream.Parse: %v", err)
		}
	}
}

// buildSnappyEncodedWriteRequest builds a snappy-encoded protobuf WriteRequest
// with numSeries time series, each having numLabels labels of labelSize bytes each.
func buildSnappyEncodedWriteRequest(numSeries, numLabels, labelSize, groupsNum int) []byte {
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
	pbData := wr.MarshalProtobuf(nil)
	return snappy.Encode(nil, pbData)
}
