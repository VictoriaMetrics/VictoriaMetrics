package promql

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestFixBrokenBuckets(t *testing.T) {
	f := func(values, expectedResult []float64) {
		t.Helper()
		xss := make([]leTimeseries, len(values))
		for i, v := range values {
			xss[i].ts = &timeseries{
				Values: []float64{v},
			}
		}
		fixBrokenBuckets(0, xss)
		result := make([]float64, len(values))
		for i, xs := range xss {
			result[i] = xs.ts.Values[0]
		}
		if !reflect.DeepEqual(result, expectedResult) {
			t.Fatalf("unexpected result for values=%v\ngot\n%v\nwant\n%v", values, result, expectedResult)
		}
	}
	f(nil, []float64{})
	f([]float64{1}, []float64{1})
	f([]float64{1, 2}, []float64{1, 2})
	f([]float64{2, 1}, []float64{1, 1})
	f([]float64{1, 2, 3, nan, nan}, []float64{1, 2, 3, 3, 3})
	f([]float64{5, 1, 2, 3, nan}, []float64{1, 1, 2, 3, 3})
	f([]float64{1, 5, 2, nan, 6, 3}, []float64{1, 2, 2, 3, 3, 3})
	f([]float64{5, 10, 4, 3}, []float64{3, 3, 3, 3})
}

func TestVmrangeBucketsToLE(t *testing.T) {
	f := func(buckets, bucketsExpected string) {
		t.Helper()
		tss := promMetricsToTimeseries(buckets)
		result := vmrangeBucketsToLE(tss)
		resultBuckets := timeseriesToPromMetrics(result)
		if !reflect.DeepEqual(resultBuckets, bucketsExpected) {
			t.Errorf("unexpected vmrangeBucketsToLE(); got\n%v\nwant\n%v", resultBuckets, bucketsExpected)
		}
	}

	// A single non-empty vmrange bucket
	f(
		`foo{vmrange="4.084e+02...4.642e+02"} 2 123`,
		`foo{le="4.084e+02"} 0 123
foo{le="4.642e+02"} 2 123
foo{le="+Inf"} 2 123`,
	)
	f(
		`foo{vmrange="0...+Inf"} 5 123`,
		`foo{le="+Inf"} 5 123`,
	)
	f(
		`foo{vmrange="-Inf...0"} 4 123`,
		`foo{le="-Inf"} 0 123
foo{le="0"} 4 123
foo{le="+Inf"} 4 123`,
	)
	f(
		`foo{vmrange="-Inf...+Inf"} 1.23 456`,
		`foo{le="-Inf"} 0 456
foo{le="+Inf"} 1.23 456`,
	)
	f(
		`foo{vmrange="0...0"} 5.3 0`,
		`foo{le="0"} 5.3 0
foo{le="+Inf"} 5.3 0`,
	)

	// Multiple non-empty vmrange buckets
	f(
		`foo{vmrange="4.084e+02...4.642e+02"} 2 123
foo{vmrange="1.234e+02...4.084e+02"} 3 123
`,
		`foo{le="1.234e+02"} 0 123
foo{le="4.084e+02"} 3 123
foo{le="4.642e+02"} 5 123
foo{le="+Inf"} 5 123`,
	)

	// Multiple disjoint vmrange buckets
	f(
		`foo{vmrange="1...2"} 2 123
foo{vmrange="4...6"} 3 123
`,
		`foo{le="1"} 0 123
foo{le="2"} 2 123
foo{le="4"} 2 123
foo{le="6"} 5 123
foo{le="+Inf"} 5 123`,
	)

	// Multiple intersected vmrange buckets
	f(
		`foo{vmrange="1...5"} 2 123
foo{vmrange="4...6"} 3 123
`,
		`foo{le="1"} 0 123
foo{le="5"} 2 123
foo{le="4"} 2 123
foo{le="6"} 5 123
foo{le="+Inf"} 5 123`,
	)

	// Multiple vmrange buckets with the same end range
	f(
		`foo{vmrange="1...5"} 2 123
foo{vmrange="0...5"} 3 123
`,
		`foo{le="1"} 0 123
foo{le="5"} 2 123
foo{le="0"} 2 123
foo{le="+Inf"} 2 123`,
	)

	// A single empty vmrange bucket
	f(
		`foo{vmrange="0...1"} 0 123`,
		``,
	)
	f(
		`foo{vmrange="0...+Inf"} 0 123`,
		``,
	)
	f(
		`foo{vmrange="-Inf...0"} 0 123`,
		``,
	)
	f(
		`foo{vmrange="0...0"} 0 0`,
		``,
	)
	f(
		`foo{vmrange="-Inf...+Inf"} 0 456`,
		``,
	)

	// Multiple empty vmrange buckets
	f(
		`foo{vmrange="2...3"} 0 123
foo{vmrange="1...2"} 0 123`,
		``,
	)

	// The bucket with negative value
	f(
		`foo{vmrange="4.084e+02...4.642e+02"} -5 1`,
		``,
	)

	// Missing vmrange in the original metric
	f(
		`foo 3 6`,
		``,
	)

	// Missing le label in the original metric
	f(
		`foo{le="456"} 3 6`,
		`foo{le="456"} 3 6`,
	)

	// Invalid vmrange label value
	f(
		`foo{vmrange="foo...bar"} 1 1`,
		``,
	)
	f(
		`foo{vmrange="4.084e+02"} 1 1`,
		``,
	)
	f(
		`foo{vmrange="4.084e+02...foo"} 1 1`,
		``,
	)
}

func promMetricsToTimeseries(s string) []*timeseries {
	var rows prometheus.Rows
	rows.UnmarshalWithErrLogger(s, func(errStr string) {
		panic(fmt.Errorf("cannot parse %q: %s", s, errStr))
	})
	var tss []*timeseries
	for _, row := range rows.Rows {
		var tags []storage.Tag
		for _, tag := range row.Tags {
			tags = append(tags, storage.Tag{
				Key:   []byte(tag.Key),
				Value: []byte(tag.Value),
			})
		}
		var ts timeseries
		ts.MetricName.MetricGroup = []byte(row.Metric)
		ts.MetricName.Tags = tags
		ts.Timestamps = append(ts.Timestamps, row.Timestamp/1000)
		ts.Values = append(ts.Values, row.Value)
		tss = append(tss, &ts)
	}
	return tss
}

func timeseriesToPromMetrics(tss []*timeseries) string {
	var a []string
	for _, ts := range tss {
		metricName := ts.MetricName.String()
		for i := range ts.Timestamps {
			line := fmt.Sprintf("%s %v %d", metricName, ts.Values[i], ts.Timestamps[i])
			a = append(a, line)
		}
	}
	return strings.Join(a, "\n")
}
