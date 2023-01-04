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
	pushFunc := func(tss []prompbmarshal.TimeSeries) {
		panic(fmt.Errorf("unexpected pushFunc call"))
	}
	a, err := NewAggregatorsFromData([]byte(config), pushFunc)
	if err != nil {
		b.Fatalf("unexpected error when initializing aggregators: %s", err)
	}
	defer a.MustStop()

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			a.Push(benchSeries)
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
