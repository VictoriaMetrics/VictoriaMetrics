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
	i := 0
	pushFunc := func(tss []prompbmarshal.TimeSeries) {
		i++
		if i > 1 {
			// pushFunc is expected to be called exactly once at MustStop
			panic(fmt.Errorf("unexpected pushFunc call"))
		}
	}
	a, err := NewAggregatorsFromData([]byte(config), pushFunc, 0)
	if err != nil {
		b.Fatalf("unexpected error when initializing aggregators: %s", err)
	}
	defer a.MustStop()

	b.ReportAllocs()
	b.SetBytes(int64(len(benchSeries)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			a.Push(benchSeries, nil)
		}
	})
}

func BenchmarkAggregatorsPushWithSeriesTracker(b *testing.B) {
	config := fmt.Sprintf(`
- match: http_requests_total
  interval: 24h
  without: [job]
  outputs: [%q]
`, "total")
	i := 0
	pushFunc := func(tss []prompbmarshal.TimeSeries) {
		i++
		if i > 1 {
			// pushFunc is expected to be called exactly once at MustStop
			panic(fmt.Errorf("unexpected pushFunc call"))
		}
	}
	a, err := NewAggregatorsFromData([]byte(config), pushFunc, 0)
	if err != nil {
		b.Fatalf("unexpected error when initializing aggregators: %s", err)
	}
	defer a.MustStop()

	tests := []struct {
		name   string
		series []prompbmarshal.TimeSeries
	}{
		{
			name:   "all matches",
			series: benchSeries,
		},
		{
			name:   "no matches",
			series: benchSeriesWithRandomNames100,
		},
		{
			name:   "50% matches",
			series: benchSeriesWithRandomNames50,
		},
		{
			name:   "10% matches",
			series: benchSeriesWithRandomNames10,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tt.series)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ut := NewTssUsageTracker(len(tt.series))
					a.Push(tt.series, ut.Matched)
				}
			})
		})
	}
}

func newBenchSeries(seriesCount, samplesPerSeries int, randomNameFraction float64) []prompbmarshal.TimeSeries {
	a := make([]string, seriesCount*samplesPerSeries)
	for i := 0; i < samplesPerSeries; i++ {
		for j := 0; j < seriesCount; j++ {
			metricName := "http_requests_total"
			if randomNameFraction > 0 && j%int(1/randomNameFraction) == 0 {
				metricName = fmt.Sprintf("random_other_name_%d", j)
			}

			s := fmt.Sprintf(`%s{path="/foo/%d",job="foo",instance="bar"} %d`, metricName, j, i*10)
			a = append(a, s)
		}
	}
	metrics := strings.Join(a, "\n")
	return mustParsePromMetrics(metrics)
}

const seriesCount = 10000
const samplesPerSeries = 10

var benchSeries = newBenchSeries(seriesCount, samplesPerSeries, 0)
var benchSeriesWithRandomNames10 = newBenchSeries(seriesCount, samplesPerSeries, 0.1)
var benchSeriesWithRandomNames50 = newBenchSeries(seriesCount, samplesPerSeries, 0.5)
var benchSeriesWithRandomNames100 = newBenchSeries(seriesCount, samplesPerSeries, 1)
