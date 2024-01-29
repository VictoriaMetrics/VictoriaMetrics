package streamaggr

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkAggregatorsPushByJobAvg(b *testing.B) {
	for _, output := range []string{
		"total",
		"increase",
		"count_series",
		"count_samples",
		"sum_samples",
		"last",
		"min",
		"max",
		"avg",
		"stddev",
		"stdvar",
		"histogram_bucket",
		"quantiles(0, 0.5, 1)",
	} {
		b.Run(fmt.Sprintf("output=%s", output), func(b *testing.B) {
			benchmarkAggregatorsPush(b, output)
		})
	}
}

func benchmarkAggregatorsPush(b *testing.B, output string) {
	config := fmt.Sprintf(`
- match: http_requests_total
  interval: 24h
  without: [job]
  outputs: [%q]
`, output)
	pushCalls := 0
	pushFunc := func(tss []prompbmarshal.TimeSeries) {
		pushCalls++
		if pushCalls > 1 {
			panic(fmt.Errorf("pushFunc is expected to be called exactly once at MustStop"))
		}
	}
	a, err := newAggregatorsFromData([]byte(config), pushFunc, 0)
	if err != nil {
		b.Fatalf("unexpected error when initializing aggregators: %s", err)
	}
	defer a.MustStop()

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	b.RunParallel(func(pb *testing.PB) {
		var matchIdxs []byte
		for pb.Next() {
			matchIdxs = a.Push(benchSeries, matchIdxs)
		}
	})
}

func newBenchSeries(seriesCount, samplesPerSeries int) []prompbmarshal.TimeSeries {
	a := make([]string, seriesCount*samplesPerSeries)
	for i := 0; i < samplesPerSeries; i++ {
		for j := 0; j < seriesCount; j++ {
			s := fmt.Sprintf(`http_requests_total{path="/foo/%d",job="foo",instance="bar"} %d`, j, i*10)
			a = append(a, s)
		}
	}
	metrics := strings.Join(a, "\n")
	return mustParsePromMetrics(metrics)
}

const seriesCount = 10000
const samplesPerSeries = 10

var benchSeries = newBenchSeries(seriesCount, samplesPerSeries)
