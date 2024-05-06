package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var benchHistogramOutputs = []string{
	HistogramMerge,
}

func BenchmarkAggregatorsHistogramPush(b *testing.B) {
	for _, output := range benchHistogramOutputs {
		b.Run(fmt.Sprintf("histogram_output=%s", output), func(b *testing.B) {
			benchmarkAggregatorsHistogramsPush(b, output)
		})
	}
}

func BenchmarkAggregatorsHistogramFlushSerial(b *testing.B) {
	histogramOutputs := []string{HistogramMerge}
	pushFunc := func(tss []prompbmarshal.TimeSeries) {}
	a := newHistogramAggregators(histogramOutputs, pushFunc)
	defer a.MustStop()
	_ = a.Push(histogramSeries, nil)

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(histogramSeries) * len(histogramOutputs)))
	for i := 0; i < b.N; i++ {
		for _, aggr := range a.as {
			aggr.flush(pushFunc, time.Hour, false)
		}
	}
}

func benchmarkAggregatorsHistogramsPush(b *testing.B, output string) {
	pushFunc := func(tss []prompbmarshal.TimeSeries) {}
	a := newHistogramAggregators([]string{output}, pushFunc)
	defer a.MustStop()

	const loops = 100

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(histogramSeries) * loops))
	b.RunParallel(func(pb *testing.PB) {
		var matchIdxs []byte
		for pb.Next() {
			for i := 0; i < loops; i++ {
				matchIdxs = a.Push(histogramSeries, matchIdxs)
			}
		}
	})
}

func newHistogramAggregators(outputs []string, pushFunc PushFunc) *Aggregators {
	outputsQuoted := make([]string, len(outputs))
	for i := range outputs {
		outputsQuoted[i] = strconv.Quote(outputs[i])
	}
	config := fmt.Sprintf(`
- match: http_requests_total
  interval: 24h
  by: [job]
  outputs: [total]
  histogram_outputs: [%s]
`, strings.Join(outputsQuoted, ","))

	a, err := newAggregatorsFromData([]byte(config), pushFunc, nil)
	if err != nil {
		panic(fmt.Errorf("unexpected error when initializing aggregators: %s", err))
	}
	return a
}

func newHistogramSeries(seriesCount int) []prompbmarshal.TimeSeries {
	tss := make([]prompbmarshal.TimeSeries, seriesCount)
	currentTimeMs := int64(fasttime.UnixTimestamp()) * 1000
	for j := 0; j < seriesCount; j++ {
		s := prompbmarshal.TimeSeries{
			Labels: []prompbmarshal.Label{
				{
					Name:  "__name__",
					Value: "http_requests_total",
				},
				{
					Name:  "path",
					Value: fmt.Sprintf("/foo/%d", j),
				},
				{
					Name:  "job",
					Value: fmt.Sprintf("/foo/%d", j%100),
				},
				{
					Name:  "instance",
					Value: "bar",
				},
				{
					Name:  "pod",
					Value: "pod-123232312",
				},
				{
					Name:  "namespace",
					Value: "kube-foo-bar",
				},
				{
					Name:  "node",
					Value: "node-123-3434-443",
				},
				{
					Name:  "some_other_label",
					Value: "foo-bar-baz",
				},
				{
					Name:  "environment",
					Value: "prod",
				},
				{
					Name:  "label1",
					Value: "value1",
				},
				{
					Name:  "label2",
					Value: "value2",
				},
				{
					Name:  "label3",
					Value: "value3",
				},
			},
			Samples: []prompbmarshal.Sample{},
			Histograms: []prompbmarshal.Histogram{
				{
					Count:         12 + uint64(j*9),
					ZeroCount:     2 + uint64(j),
					ZeroThreshold: 0.001,
					Sum:           18.4 * float64(j+1),
					Schema:        1,
					PositiveSpans: []prompbmarshal.BucketSpan{
						{Offset: 0, Length: 2},
						{Offset: 1, Length: 2},
					},
					PositiveDeltas: []int64{int64(j + 1), 1, -1, 0},
					NegativeSpans: []prompbmarshal.BucketSpan{
						{Offset: 0, Length: 2},
						{Offset: 1, Length: 2},
					},
					NegativeDeltas: []int64{int64(j + 1), 1, -1, 0},
					Timestamp:      currentTimeMs,
				},
			},
		}
		tss = append(tss, s)
	}
	return tss
}

const histogramsCount = 10_000

var histogramSeries = newHistogramSeries(histogramsCount)
