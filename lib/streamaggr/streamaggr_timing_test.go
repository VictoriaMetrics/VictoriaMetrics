package streamaggr

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

var benchOutputs = []string{
	"avg",
	"count_samples",
	"count_series",
	"histogram_bucket",
	"increase",
	"increase_prometheus",
	"last",
	"max",
	"min",
	"quantiles(0, 0.5, 1)",
	"rate_avg",
	"rate_sum",
	"stddev",
	"stdvar",
	"sum_samples",
	"total",
	"total_prometheus",
	"unique_samples",
}

func BenchmarkAggregatorsPush(b *testing.B) {
	for _, output := range benchOutputs {
		b.Run(fmt.Sprintf("output=%s", output), func(b *testing.B) {
			benchmarkAggregatorsPush(b, output)
		})
	}
}

func benchmarkAggregatorsPush(b *testing.B, output string) {
	pushFunc := func(_ []prompbmarshal.TimeSeries) {}
	a := newBenchAggregators([]string{output}, pushFunc)
	defer a.MustStop()

	const loops = 100

	b.ResetTimer()
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

func newBenchAggregators(outputs []string, pushFunc PushFunc) *Aggregators {
	outputsQuoted := make([]string, len(outputs))
	for i := range outputs {
		outputsQuoted[i] = stringsutil.JSONString(outputs[i])
	}
	config := fmt.Sprintf(`
- match: http_requests_total
  interval: 24h
  by: [job]
  outputs: [%s]
`, strings.Join(outputsQuoted, ","))

	a, err := LoadFromData([]byte(config), pushFunc, nil, "some_alias")
	if err != nil {
		panic(fmt.Errorf("unexpected error when initializing aggregators: %s", err))
	}
	return a
}

func newBenchSeries(seriesCount int) []prompbmarshal.TimeSeries {
	a := make([]string, 0, seriesCount)
	for j := 0; j < seriesCount; j++ {
		s := fmt.Sprintf(`http_requests_total{path="/foo/%d",job="foo_%d",instance="bar",pod="pod-123232312",namespace="kube-foo-bar",node="node-123-3434-443",`+
			`some_other_label="foo-bar-baz",environment="prod",label1="value1",label2="value2",label3="value3"} %d`, j, j%100, j*1000)
		a = append(a, s)
	}
	metrics := strings.Join(a, "\n")
	offsetMsecs := time.Now().UnixMilli()
	return prompbmarshal.MustParsePromMetrics(metrics, offsetMsecs)
}

const seriesCount = 10_000

var benchSeries = newBenchSeries(seriesCount)
