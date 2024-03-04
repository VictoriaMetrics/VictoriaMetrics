package streamaggr

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var benchOutputs = []string{
	"total",
	"total_prometheus",
	"increase",
	"increase_prometheus",
	"count_series",
	"count_samples",
	"unique_samples",
	"sum_samples",
	"last",
	"min",
	"max",
	"avg",
	"stddev",
	"stdvar",
	"histogram_bucket",
	"quantiles(0, 0.5, 1)",
}

func BenchmarkAggregatorsPush(b *testing.B) {
	for _, output := range benchOutputs {
		b.Run(fmt.Sprintf("output=%s", output), func(b *testing.B) {
			benchmarkAggregatorsPush(b, output)
		})
	}
}

func BenchmarkAggregatorsFlushSerial(b *testing.B) {
	for _, output := range benchOutputs {
		b.Run(fmt.Sprintf("output=%s", output), func(b *testing.B) {
			benchmarkAggregatorsFlushSerial(b, output)
		})
	}
}

func benchmarkAggregatorsFlushSerial(b *testing.B, output string) {
	pushFunc := func(tss []prompbmarshal.TimeSeries) {}
	a := newBenchAggregators(output, pushFunc)
	defer a.MustStop()

	var matchIdxs []byte

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	for i := 0; i < b.N; i++ {
		matchIdxs = a.Push(benchSeries, matchIdxs)
		for _, aggr := range a.as {
			aggr.flush(pushFunc)
		}
	}
}

func benchmarkAggregatorsPush(b *testing.B, output string) {
	pushFunc := func(tss []prompbmarshal.TimeSeries) {}
	a := newBenchAggregators(output, pushFunc)
	defer a.MustStop()

	const loops = 100

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries) * loops))
	b.RunParallel(func(pb *testing.PB) {
		var matchIdxs []byte
		for pb.Next() {
			for i := 0; i < loops; i++ {
				matchIdxs = a.Push(benchSeries, matchIdxs)
			}
		}
	})
}

func newBenchAggregators(output string, pushFunc PushFunc) *Aggregators {
	config := fmt.Sprintf(`
- match: http_requests_total
  interval: 24h
  without: [job]
  outputs: [%q]
`, output)
	a, err := newAggregatorsFromData([]byte(config), pushFunc, nil)
	if err != nil {
		panic(fmt.Errorf("unexpected error when initializing aggregators: %s", err))
	}
	return a
}

func newBenchSeries(seriesCount int) []prompbmarshal.TimeSeries {
	a := make([]string, seriesCount)
	for j := 0; j < seriesCount; j++ {
		s := fmt.Sprintf(`http_requests_total{path="/foo/%d",job="foo",instance="bar",pod="pod-123232312",namespace="kube-foo-bar",node="node-123-3434-443",`+
			`some_other_label="foo-bar-baz",environment="prod",label1="value1",label2="value2",label3="value3"} %d`, j, j*1000)
		a = append(a, s)
	}
	metrics := strings.Join(a, "\n")
	return mustParsePromMetrics(metrics)
}

const seriesCount = 10_000

var benchSeries = newBenchSeries(seriesCount)
