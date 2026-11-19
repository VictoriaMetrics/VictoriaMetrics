//go:build synctest

package streamaggr

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/metrics"
)

func init() {
	// Create the first summary outside of synctest so the global goroutine it starts doesn't belong to any synctest.
	// Otherwise, the test would panic with "blocked goroutines remain" when it ends.
	_ = metrics.NewSet().NewSummary("streamaggr_synctest_seconds")
}

func TestAggregatorsSuccess(t *testing.T) {
	f := func(inputMetrics []string, interval time.Duration, outputMetricsExpected, config, matchIdxsStrExpected string) {
		t.Helper()

		var matchIdxs []uint32
		var tssOutput []prompb.TimeSeries
		synctest.Test(t, func(t *testing.T) {
			var tssOutputLock sync.Mutex
			// Initialize Aggregators
			pushFunc := func(tss []prompb.TimeSeries) {
				tssOutputLock.Lock()
				tssOutput = appendClonedTimeseries(tssOutput, tss)
				tssOutputLock.Unlock()
			}
			a, err := LoadFromData([]byte(config), pushFunc, nil, "some_alias")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			offsetMsecs := time.Now().UnixMilli()
			for _, metrics := range inputMetrics {
				// Push the inputMetrics to Aggregators
				tssInput := prometheus.MustParsePromMetrics(metrics, offsetMsecs)
				matchIdxs = append(matchIdxs, a.Push(tssInput, nil)...)
				time.Sleep(interval + time.Millisecond) // shift by 1ms from flush border to avoid flaky tests
				offsetMsecs += interval.Milliseconds()
			}
			a.MustStop()
		})

		// Verify matchIdxs equals to matchIdxsExpected
		matchIdxsStr := ""
		for _, v := range matchIdxs {
			matchIdxsStr += strconv.Itoa(int(v))
		}
		if matchIdxsStr != matchIdxsStrExpected {
			t.Fatalf("unexpected matchIdxs;\ngot\n%s\nwant\n%s", matchIdxsStr, matchIdxsStrExpected)
		}

		// Verify the tssOutput contains the expected metrics
		outputMetrics := timeSeriessToString(tssOutput)
		if outputMetrics != outputMetricsExpected {
			t.Fatalf("unexpected output metrics;\ngot\n%s\nwant\n%s", outputMetrics, outputMetricsExpected)
		}
	}

	// Empty config
	f([]string{}, time.Second, ``, ``, "")
	f([]string{`foo{bar="baz"} 1`}, time.Second, ``, ``, "0")
	f([]string{"foo 1\nbaz 2"}, time.Second, ``, ``, "00")

	// Empty by list - aggregate only by time
	f([]string{`
foo{abc="123"} 4
bar 5 11
bar 34 10
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_count_samples 2
bar:1m_count_series 1
bar:1m_last 5
bar:1m_sum_samples 39
foo:1m_count_samples{abc="123"} 2
foo:1m_count_samples{abc="456",de="fg"} 1
foo:1m_count_series{abc="123"} 1
foo:1m_count_series{abc="456",de="fg"} 1
foo:1m_last{abc="123"} 8.5
foo:1m_last{abc="456",de="fg"} 8
foo:1m_sum_samples{abc="123"} 12.5
foo:1m_sum_samples{abc="456",de="fg"} 8
`, `
- interval: 1m
  outputs: [count_samples, sum_samples, count_series, last]
`, "11111")

	// Special case: __name__ in `by` list - this is the same as empty `by` list
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_count_samples 1
bar:1m_count_series 1
bar:1m_sum_samples 5
foo:1m_count_samples 3
foo:1m_count_series 2
foo:1m_sum_samples 20.5
`, `
- interval: 1m
  by: [__name__]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Non-empty `by` list with non-existing labels
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_by_bar_foo_count_samples 1
bar:1m_by_bar_foo_count_series 1
bar:1m_by_bar_foo_sum_samples 5
foo:1m_by_bar_foo_count_samples 3
foo:1m_by_bar_foo_count_series 2
foo:1m_by_bar_foo_sum_samples 20.5
`, `
- interval: 1m
  by: [foo, bar]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Non-empty `by` list with existing label
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_by_abc_count_samples 1
bar:1m_by_abc_count_series 1
bar:1m_by_abc_sum_samples 5
foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, `
- interval: 1m
  by: [abc]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Non-empty `by` list with duplicate existing label
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_by_abc_count_samples 1
bar:1m_by_abc_count_series 1
bar:1m_by_abc_sum_samples 5
foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, `
- interval: 1m
  by: [abc, abc]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Non-empty `without` list with non-existing labels
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_without_foo_count_samples 1
bar:1m_without_foo_count_series 1
bar:1m_without_foo_sum_samples 5
foo:1m_without_foo_count_samples{abc="123"} 2
foo:1m_without_foo_count_samples{abc="456",de="fg"} 1
foo:1m_without_foo_count_series{abc="123"} 1
foo:1m_without_foo_count_series{abc="456",de="fg"} 1
foo:1m_without_foo_sum_samples{abc="123"} 12.5
foo:1m_without_foo_sum_samples{abc="456",de="fg"} 8
`, `
- interval: 1m
  without: [foo]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Non-empty `without` list with existing labels
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_without_abc_count_samples 1
bar:1m_without_abc_count_series 1
bar:1m_without_abc_sum_samples 5
foo:1m_without_abc_count_samples 2
foo:1m_without_abc_count_samples{de="fg"} 1
foo:1m_without_abc_count_series 1
foo:1m_without_abc_count_series{de="fg"} 1
foo:1m_without_abc_sum_samples 12.5
foo:1m_without_abc_sum_samples{de="fg"} 8
`, `
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// Special case: __name__ in `without` list
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `:1m_count_samples 1
:1m_count_samples{abc="123"} 2
:1m_count_samples{abc="456",de="fg"} 1
:1m_count_series 1
:1m_count_series{abc="123"} 1
:1m_count_series{abc="456",de="fg"} 1
:1m_sum_samples 5
:1m_sum_samples{abc="123"} 12.5
:1m_sum_samples{abc="456",de="fg"} 8
`, `
- interval: 1m
  without: [__name__]
  outputs: [count_samples, sum_samples, count_series]
`, "1111")

	// drop some input metrics
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_without_abc_count_samples 1
bar:1m_without_abc_count_series 1
bar:1m_without_abc_sum_samples 5
`, `
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  input_relabel_configs:
  - if: 'foo'
    action: drop
`, "1111")

	// rename output metrics
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar-1m-without-abc-count-samples 1
bar-1m-without-abc-count-series 1
bar-1m-without-abc-sum-samples 5
foo-1m-without-abc-count-samples 2
foo-1m-without-abc-count-series 1
foo-1m-without-abc-sum-samples 12.5
`, `
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  output_relabel_configs:
  - action: replace_all
    source_labels: [__name__]
    regex: ":|_"
    replacement: "-"
    target_label: __name__
  - action: drop
    source_labels: [de]
    regex: fg
`, "1111")

	// match doesn't match anything
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, ``, `
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  match: '{non_existing_label!=""}'
  name: foobar
`, "0000")

	// match matches foo series with non-empty abc label
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, `
- interval: 1m
  by: [abc]
  outputs: [count_samples, sum_samples, count_series]
  name: abcdef
  match:
  - foo{abc=~".+"}
  - '{non_existing_label!=""}'
`, "1011")

	// total output for non-repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 4.34
`}, time.Minute, `bar:1m_total{baz="qwe"} 0
foo:1m_total 0
`, `
- interval: 1m
  outputs: [total]
`, "11")

	// total output for non-repeated series, ignore first sample 0s
	f([]string{`
foo 123
bar{baz="qwe"} 4.34
`}, time.Minute, `bar:1m_total{baz="qwe"} 4.34
foo:1m_total 123
`, `
- interval: 1m
  outputs: [total]
  ignore_first_sample_interval: 0s
`, "11")

	// total_prometheus output for non-repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 4.34
`}, time.Minute, `bar:1m_total_prometheus{baz="qwe"} 0
foo:1m_total_prometheus 0
`, `
- interval: 1m
  outputs: [total_prometheus]
`, "11")

	// total output for repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 1.31
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_total{baz="qwe"} 3.03
bar:1m_total{baz="qwer"} 1
foo:1m_total 0
foo:1m_total{baz="qwe"} 15
`, `
- interval: 1m
  outputs: [total]
`, "11111111")

	// total_prometheus output for repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_total_prometheus{baz="qwe"} 5.02
bar:1m_total_prometheus{baz="qwer"} 1
foo:1m_total_prometheus 0
foo:1m_total_prometheus{baz="qwe"} 15
`, `
- interval: 1m
  outputs: [total_prometheus]
`, "11111111")

	// total output for repeated series with group by __name__
	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_total 6.02
foo:1m_total 15
`, `
- interval: 1m
  by: [__name__]
  outputs: [total]
`, "11111111")

	// total_prometheus output for repeated series with group by __name__
	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_total_prometheus 6.02
foo:1m_total_prometheus 15
`, `
- interval: 1m
  by: [__name__]
  outputs: [total_prometheus]
`, "11111111")

	// total output must skip a duplicate sample with the same timestamp and a lower value instead of treating it as a counter reset
	f([]string{`
foo 100
foo 105 10
foo 103 10
foo 105 10
foo 110 20
`}, time.Minute, `foo:1m_total 10
`, `
- interval: 1m
  outputs: [total]
`, "11111")

	// increase output for non-repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 4.34
`}, time.Minute, `bar:1m_increase{baz="qwe"} 0
foo:1m_increase 0
`, `
- interval: 1m
  outputs: [increase]
`, "11")

	// increase_prometheus output for non-repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 4.34
`}, time.Minute, `bar:1m_increase_prometheus{baz="qwe"} 0
foo:1m_increase_prometheus 0
`, `
- interval: 1m
  outputs: [increase_prometheus]
`, "11")

	// increase output for repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_increase{baz="qwe"} 5.02
bar:1m_increase{baz="qwer"} 1
foo:1m_increase 0
foo:1m_increase{baz="qwe"} 15
`, `
- interval: 1m
  outputs: [increase]
`, "11111111")

	// increase_prometheus output for repeated series
	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34 1
bar{baz="qwe"} 2 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`}, time.Minute, `bar:1m_increase_prometheus{baz="qwe"} 5.02
bar:1m_increase_prometheus{baz="qwer"} 1
foo:1m_increase_prometheus 0
foo:1m_increase_prometheus{baz="qwe"} 15
`, `
- interval: 1m
  outputs: [increase_prometheus]
`, "11111111")

	// increase, increase_prometheus, total, total_prometheus outputs with different staleness intervals
	f([]string{`
foo 5
bar 200
`, `
foo 10
bar 201
`, ``, `
foo 7
bar 205
`}, time.Minute, `bar:1m_increase 200
bar:1m_increase 1
bar:1m_increase 205
bar:1m_increase_prometheus 0
bar:1m_increase_prometheus 1
bar:1m_increase_prometheus 0
bar:1m_total 200
bar:1m_total 201
bar:1m_total 205
bar:1m_total_prometheus 0
bar:1m_total_prometheus 1
bar:1m_total_prometheus 0
bar:1m_without_non_existing_label_increase 0
bar:1m_without_non_existing_label_increase 1
bar:1m_without_non_existing_label_increase 4
bar:1m_without_non_existing_label_increase_prometheus 0
bar:1m_without_non_existing_label_increase_prometheus 1
bar:1m_without_non_existing_label_increase_prometheus 4
bar:1m_without_non_existing_label_total 0
bar:1m_without_non_existing_label_total 1
bar:1m_without_non_existing_label_total 1
bar:1m_without_non_existing_label_total 5
bar:1m_without_non_existing_label_total_prometheus 0
bar:1m_without_non_existing_label_total_prometheus 1
bar:1m_without_non_existing_label_total_prometheus 1
bar:1m_without_non_existing_label_total_prometheus 5
foo:1m_increase 5
foo:1m_increase 5
foo:1m_increase 7
foo:1m_increase_prometheus 0
foo:1m_increase_prometheus 5
foo:1m_increase_prometheus 0
foo:1m_total 5
foo:1m_total 10
foo:1m_total 7
foo:1m_total_prometheus 0
foo:1m_total_prometheus 5
foo:1m_total_prometheus 0
foo:1m_without_non_existing_label_increase 0
foo:1m_without_non_existing_label_increase 5
foo:1m_without_non_existing_label_increase 7
foo:1m_without_non_existing_label_increase_prometheus 0
foo:1m_without_non_existing_label_increase_prometheus 5
foo:1m_without_non_existing_label_increase_prometheus 7
foo:1m_without_non_existing_label_total 0
foo:1m_without_non_existing_label_total 5
foo:1m_without_non_existing_label_total 5
foo:1m_without_non_existing_label_total 12
foo:1m_without_non_existing_label_total_prometheus 0
foo:1m_without_non_existing_label_total_prometheus 5
foo:1m_without_non_existing_label_total_prometheus 5
foo:1m_without_non_existing_label_total_prometheus 12
`, `
- interval: 1m
  ignore_first_sample_interval: 0s
  outputs: [increase, increase_prometheus, total, total_prometheus]
- interval: 1m
  staleness_interval: 2m
  without: [non_existing_label]
  outputs: [increase, increase_prometheus, total, total_prometheus]
`, "111111")

	// sum_sample and sum_samples_total outputs with different staleness intervals
	f([]string{`
foo 1
foo 2 1
foo{bar="baz"} 2
`, `
foo 4
`, ``, ``, `
foo 6
`, ``, ``}, time.Minute, `foo:1m_sum_samples 3
foo:1m_sum_samples 4
foo:1m_sum_samples 6
foo:1m_sum_samples_total 3
foo:1m_sum_samples_total 7
foo:1m_sum_samples_total 6
foo:1m_sum_samples_total{bar="baz"} 2
foo:1m_sum_samples{bar="baz"} 2
foo:1m_without_non-existing-label_sum_samples 3
foo:1m_without_non-existing-label_sum_samples 4
foo:1m_without_non-existing-label_sum_samples 0
foo:1m_without_non-existing-label_sum_samples 6
foo:1m_without_non-existing-label_sum_samples 0
foo:1m_without_non-existing-label_sum_samples_total 3
foo:1m_without_non-existing-label_sum_samples_total 7
foo:1m_without_non-existing-label_sum_samples_total 7
foo:1m_without_non-existing-label_sum_samples_total 6
foo:1m_without_non-existing-label_sum_samples_total 6
foo:1m_without_non-existing-label_sum_samples_total{bar="baz"} 2
foo:1m_without_non-existing-label_sum_samples_total{bar="baz"} 2
foo:1m_without_non-existing-label_sum_samples{bar="baz"} 2
foo:1m_without_non-existing-label_sum_samples{bar="baz"} 0
foo:5m_by_bar_sum_samples 13
foo:5m_by_bar_sum_samples_total 13
foo:5m_by_bar_sum_samples_total{bar="baz"} 2
foo:5m_by_bar_sum_samples{bar="baz"} 2
`, `
- interval: 1m
  staleness_interval: 1m
  outputs: [ sum_samples, sum_samples_total]
- interval: 1m
  staleness_interval: 2m
  without: [non-existing-label]
  outputs: [ sum_samples, sum_samples_total]
- interval: 5m
  by: [bar]
  outputs: [sum_samples, sum_samples_total]
`, "11111")

	// min and max outputs
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_max 5
bar:1m_min 5
foo:1m_max{abc="123"} 8.5
foo:1m_max{abc="456",de="fg"} 8
foo:1m_min{abc="123"} 4
foo:1m_min{abc="456",de="fg"} 8
`, `
- interval: 1m
  outputs: [min, max]
`, "1111")

	// avg output
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_avg 5
foo:1m_avg{abc="123"} 6.25
foo:1m_avg{abc="456",de="fg"} 8
`, `
- interval: 1m
  outputs: [avg]
`, "1111")

	// stddev output
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_stddev 0
foo:1m_stddev{abc="123"} 2.25
foo:1m_stddev{abc="456",de="fg"} 0
`, `
- interval: 1m
  outputs: [stddev]
`, "1111")

	// stdvar output
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar:1m_stdvar 0
foo:1m_stdvar{abc="123"} 5.0625
foo:1m_stdvar{abc="456",de="fg"} 0
`, `
- interval: 1m
  outputs: [stdvar]
`, "1111")

	// histogram_bucket output
	f([]string{`
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`}, time.Minute, `cpu_usage:1m_histogram_bucket{cpu="1",vmrange="1.136e+01...1.292e+01"} 2
cpu_usage:1m_histogram_bucket{cpu="1",vmrange="1.292e+01...1.468e+01"} 3
cpu_usage:1m_histogram_bucket{cpu="1",vmrange="2.448e+01...2.783e+01"} 1
cpu_usage:1m_histogram_bucket{cpu="2",vmrange="8.799e+01...1.000e+02"} 1
`, `
- interval: 1m
  outputs: [histogram_bucket]
`, "1111111")

	// histogram_bucket output without cpu
	f([]string{`
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`}, time.Minute, `cpu_usage:1m_without_cpu_histogram_bucket{vmrange="1.136e+01...1.292e+01"} 2
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="1.292e+01...1.468e+01"} 3
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="2.448e+01...2.783e+01"} 1
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="8.799e+01...1.000e+02"} 1
`, `
- interval: 1m
  without: [cpu]
  outputs: [histogram_bucket]
`, "1111111")

	// quantiles output
	f([]string{`
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`}, time.Minute, `cpu_usage:1m_quantiles{cpu="1",quantile="0"} 12
cpu_usage:1m_quantiles{cpu="1",quantile="0.5"} 13.3
cpu_usage:1m_quantiles{cpu="1",quantile="1"} 25
cpu_usage:1m_quantiles{cpu="2",quantile="0"} 90
cpu_usage:1m_quantiles{cpu="2",quantile="0.5"} 90
cpu_usage:1m_quantiles{cpu="2",quantile="1"} 90
`, `
- interval: 1m
  outputs: ["quantiles(0, 0.5, 1)"]
`, "1111111")

	// quantiles output without cpu
	f([]string{`
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`}, time.Minute, `cpu_usage:1m_without_cpu_quantiles{quantile="0"} 12
cpu_usage:1m_without_cpu_quantiles{quantile="0.5"} 13.3
cpu_usage:1m_without_cpu_quantiles{quantile="1"} 90
`, `
- interval: 1m
  without: [cpu]
  outputs: ["quantiles(0, 0.5, 1)"]
`, "1111111")

	// no stale quantiles should be produced
	f([]string{`
cpu_usage{cpu="1"} 3
cpu_usage{cpu="2"} 3`,
		`cpu_usage{cpu="2"} 4`,
	}, time.Minute, `cpu_usage:1m_quantiles{cpu="1",quantile="1"} 3
cpu_usage:1m_quantiles{cpu="2",quantile="1"} 3
cpu_usage:1m_quantiles{cpu="2",quantile="1"} 4
`, `
- interval: 1m
  outputs: ["quantiles(1)"]
`, "111")

	// append additional label
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5 10
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar-1m-without-abc-count-samples{new_label="must_keep_metric_name"} 1
bar-1m-without-abc-count-series{new_label="must_keep_metric_name"} 1
bar-1m-without-abc-sum-samples{new_label="must_keep_metric_name"} 5
foo-1m-without-abc-count-samples{new_label="must_keep_metric_name"} 2
foo-1m-without-abc-count-series{new_label="must_keep_metric_name"} 1
foo-1m-without-abc-sum-samples{new_label="must_keep_metric_name"} 12.5
`, `
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  output_relabel_configs:
  - action: replace_all
    source_labels: [__name__]
    regex: ":|_"
    replacement: "-"
    target_label: __name__
  - action: drop
    source_labels: [de]
    regex: fg
  - target_label: new_label
    replacement: must_keep_metric_name
`, "1111")

	// test rate_sum and rate_avg
	f([]string{`
foo{abc="123", cde="1"} 3
foo{abc="456", cde="1"} 8.5
foo 12 34
`, `
foo{abc="123", cde="1"} 8
foo{abc="456", cde="1"} 11
`}, time.Minute, `foo:1m_by_cde_rate_avg{cde="1"} 0.0625
foo:1m_by_cde_rate_sum{cde="1"} 0.125
`, `
- interval: 1m
  by: [cde]
  outputs: [rate_sum, rate_avg]
`, "11111")

	// test rate_sum with out of order samples
	f([]string{`
foo 1
`, `
foo 61
`, `
foo 31 -70
foo 91
`, `
foo 121
`}, time.Minute, `foo:1m_rate_sum 1
foo:1m_rate_sum 0.5
foo:1m_rate_sum 0.5
`, `
- interval: 1m
  outputs: [rate_sum]
`, "11111")

	// test rate_sum and rate_avg with different staleness intervals
	f([]string{`
foo{abc="123", cde="1"} 1
foo{abc="123", cde="1"} 2 1
foo{abc="456", cde="1"} 3
foo{abc="456", cde="1"} 4 1
foo{abc="777", cde="1"} 5
foo{abc="777", cde="1"} 6 1
`, ``, `
foo{abc="123", cde="1"} 121
foo{abc="123", cde="1"} 122 1
foo{abc="456", cde="1"} 123
foo{abc="456", cde="1"} 124 1
foo{abc="777", cde="1"} 125
foo{abc="777", cde="1"} 126 1
`}, time.Minute, `foo:1m_by_cde_rate_avg{cde="1"} 1
foo:1m_by_cde_rate_avg{cde="1"} 1
foo:1m_by_cde_rate_sum{cde="1"} 3
foo:1m_by_cde_rate_sum{cde="1"} 3
foo:1m_without_abc_rate_avg{cde="1"} 1
foo:1m_without_abc_rate_avg{cde="1"} 1
foo:1m_without_abc_rate_sum{cde="1"} 3
foo:1m_without_abc_rate_sum{cde="1"} 3
`, `            
- interval: 1m
  by: [cde]
  outputs: [rate_sum, rate_avg]
  enable_windows: true
- interval: 1m
  staleness_interval: 2m
  without: [abc]
  outputs: [rate_sum, rate_avg]
  enable_windows: true
`, "111111111111")

	// rate_sum and rate_avg with duplicated events
	f([]string{`
foo{abc="123", cde="1"} 4  10
foo{abc="123", cde="1"} 4  10
`}, time.Minute, ``, `
- interval: 1m
  outputs: [rate_sum, rate_avg]
`, "11")

	// rate_sum and rate_avg for a single sample
	f([]string{`
foo 4  10
bar 5  10
`}, time.Minute, ``, `
- interval: 1m
  outputs: [rate_sum, rate_avg]
`, "11")

	// unique_samples output
	f([]string{`
foo 1  10
foo 2  20
foo 1  10
foo 2  20
foo 3  20
`}, time.Minute, `foo:1m_unique_samples 3
`, `
- interval: 1m
  outputs: [unique_samples]
`, "11111")

	// keep_metric_names
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
bar -34.3
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar 2
foo{abc="123"} 2
foo{abc="456",de="fg"} 1
`, `
- interval: 1m
  keep_metric_names: true
  outputs: [count_samples]
`, "11111")

	// drop_input_labels
	f([]string{`
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
bar -34.3
foo{abc="456",de="fg"} 8
`}, time.Minute, `bar 2
foo 2
foo{de="fg"} 1
`, `
- interval: 1m
  drop_input_labels: [abc]
  keep_metric_names: true
  outputs: [count_samples]
`, "11111")

	f([]string{`
foo 123
bar 567
`, ``, ``}, 30*time.Second, `bar:1m_sum_samples 567
foo:1m_sum_samples 123
`, `
- interval: 1m
  outputs: [sum_samples]
  dedup_interval: 30s
`, "11")

	f([]string{`
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, ``, ``}, 30*time.Second, `bar:1m_sum_samples{baz="qwe"} 4.34
bar:1m_sum_samples{baz="qwer"} 344
foo:1m_sum_samples 123
foo:1m_sum_samples{baz="qwe"} 10
`, `
- interval: 1m
  dedup_interval: 30s
  outputs: [sum_samples]
`, "11111111")

	// Reproduce issue #11261: sum_samples_total must be monotonic with enable_windows: true
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/11262
	f([]string{`
test_delta 1
`, `
test_delta 1
`, `
test_delta 1
`, `
test_delta 1
`}, time.Minute, `test_delta 1
test_delta 2
test_delta 3
test_delta 4
`, `
- interval: 1m
  keep_metric_names: true
  outputs: [sum_samples_total]
  enable_windows: true
`, "1111")

	// Reproduce issue #11707: samples from both windows in one push must not move the increase into the next window
	f([]string{`
foo 0
`, `
foo 0 -25
`, `
foo 1 -2
foo 1 1
`, `
foo 1 -25
`, `
foo 2 -2
foo 2 1
`, `
foo 2 -25
`, `
foo 3 -2
foo 3 1
`}, 30*time.Second, `foo:1m_increase 1
foo:1m_increase 1
foo:1m_increase 1
foo:1m_total 1
foo:1m_total 2
foo:1m_total 3
`, `
- interval: 1m
  outputs: [increase, total]
  enable_windows: true
`, "1111111111")

	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11810
	// rate_sum with enable_windows must skip out-of-order samples across windows instead of treating them as counter resets.
	f([]string{`
foo 1000000
foo 1000005 5
`, `
foo 1000065 35
bar 1 -30
`, `
foo 1000045 -10
`, `
foo 1000090
`, ``, ``}, 30*time.Second, `foo:1m_rate_sum 1
foo:1m_rate_sum 1
`, `
- interval: 1m
  outputs: [rate_sum]
  enable_windows: true
`, "111111")
}

func TestAggregatorSlowFlush(t *testing.T) {
	type flushResult struct {
		// timestamp is the time of the day of the output sample
		timestamp string
		value     float64
	}

	// f creates an aggregator for the given config, makes its first non-empty flush take slowFlushDuration,
	// pushes a sample foo{i="N"} with value N+1 at every time of the day from pushTimes, where N is the index of the time,
	// and stops the aggregator at stopTime.
	// Then it verifies the flushed outputs, the number of skipped flushes and that no samples were dropped as too old.
	f := func(config string, alignFlush bool, slowFlushDuration time.Duration, pushTimes []string, stopTime string, resultsExpected []flushResult, flushSkippedExpected uint64) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			// Start the aggregator with aligned flushes on the interval boundary, so it has no partial first interval.
			// Start the aggregator with non-aligned flushes off the interval grid like in real usage.
			startTime := "00:01:00"
			if !alignFlush {
				startTime = "00:00:10"
			}
			time.Sleep(time.Until(at(startTime)))

			var results []prompb.TimeSeries
			var resultsLock sync.Mutex
			slowFlushDone := false
			pushFunc := func(tss []prompb.TimeSeries) {
				resultsLock.Lock()
				defer resultsLock.Unlock()
				results = appendClonedTimeseries(results, tss)
				// Make only the first non-empty flush slow.
				if !slowFlushDone {
					slowFlushDone = true
					time.Sleep(slowFlushDuration)
				}
			}
			a, err := LoadFromData([]byte(config), pushFunc, &Options{
				NoAlignFlushToInterval: !alignFlush,
			}, "some_alias")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			ag := a.as[0]

			for i, pushTime := range pushTimes {
				time.Sleep(time.Until(at(pushTime)))
				tss := prometheus.MustParsePromMetrics(fmt.Sprintf(`foo{i="%d"} %d`, i, i+1), time.Now().UnixMilli())
				a.Push(tss, nil)
			}
			time.Sleep(time.Until(at(stopTime)))
			synctest.Wait()

			flushSkipped := ag.flushSkipped.Get()
			if ag.da != nil {
				flushSkipped += ag.da.flushSkipped.Get()
			}
			ignoredOldSamples := ag.ignoredOldSamples.Get()
			a.MustStop()

			if ignoredOldSamples != 0 {
				t.Fatalf("config=%s, alignFlush=%v, slowFlushDuration=%s: unexpected number of samples dropped as too old; got %d; want 0", config, alignFlush, slowFlushDuration, ignoredOldSamples)
			}
			if flushSkipped != flushSkippedExpected {
				t.Fatalf("config=%s, alignFlush=%v, slowFlushDuration=%s: unexpected number of skipped flushes; got %d; want %d", config, alignFlush, slowFlushDuration, flushSkipped, flushSkippedExpected)
			}
			var got []flushResult
			for _, ts := range results {
				for _, s := range ts.Samples {
					got = append(got, flushResult{
						timestamp: timeOfDay(s.Timestamp),
						value:     s.Value,
					})
				}
			}
			if !reflect.DeepEqual(got, resultsExpected) {
				t.Fatalf("config=%s, alignFlush=%v, slowFlushDuration=%s: unexpected results;\ngot\n%v\nwant\n%v", config, alignFlush, slowFlushDuration, got, resultsExpected)
			}
		})
	}

	s := time.Second

	// Both ignore_old_samples and enable_windows drop samples older than the current aggregation interval,
	// so the slow flush must not make them drop the samples pushed meanwhile.
	for _, oldSamplesOption := range []string{"ignore_old_samples: true", "enable_windows: true"} {
		config := fmt.Sprintf(`
- interval: 1m
  %s
  without: [i]
  outputs: [count_samples]
`, oldSamplesOption)
		dedupConfig := fmt.Sprintf(`
- interval: 1m
  dedup_interval: 30s
  %s
  without: [i]
  outputs: [count_samples]
`, oldSamplesOption)
		sameSeriesConfig := fmt.Sprintf(`
- interval: 1m
  %s
  drop_input_labels: [i]
  outputs: [count_samples]
`, oldSamplesOption)
		sameSeriesDedupConfig := `
- interval: 1m
  dedup_interval: 30s
  ` + oldSamplesOption + `
  drop_input_labels: [i]
  outputs: [%s]
`

		// The aggregator with aligned flushes starts at 00:01:00, so the flushes happen at 00:02:00, 00:03:00, 00:04:00, etc.

		// The first flush at 00:02:00 takes 1.5 intervals, so the flush at 00:03:00 must be skipped.
		// The output timestamps must stay on the flush interval grid,
		// and the samples of the skipped interval must be flushed at 00:04:00.
		f(config, true, 90*s, []string{"00:01:30", "00:02:30", "00:03:36", "00:04:30", "00:05:30"}, "00:06:30", []flushResult{
			{"00:02:00", 1},
			{"00:04:00", 2},
			{"00:05:00", 1},
			{"00:06:00", 1},
		}, 1)

		// The first flush at 00:02:00 takes 2.5 intervals, so the flushes at 00:03:00 and 00:04:00 must be skipped.
		f(config, true, 150*s, []string{"00:01:30", "00:03:00", "00:04:00", "00:04:40", "00:05:30"}, "00:06:30", []flushResult{
			{"00:02:00", 1},
			{"00:05:00", 3},
			{"00:06:00", 1},
		}, 2)

		// The first flush at 00:02:00 takes 100s with dedup_interval=30s,
		// so the flush at 00:03:00 and the dedup flushes at 00:02:30, 00:03:00 and 00:03:30 must be skipped.
		// The samples pushed meanwhile must be flushed at 00:04:00 instead of being dropped as stale.
		// The samples are pushed between dedup ticks, so the result doesn't depend on the order of simultaneous events.
		f(dedupConfig, true, 100*s, []string{"00:01:40", "00:02:20", "00:03:36", "00:04:20", "00:05:20"}, "00:06:20", []flushResult{
			{"00:02:00", 1},
			{"00:04:00", 2},
			{"00:05:00", 1},
			{"00:06:00", 1},
		}, 4)

		// The same series is pushed every 30s while the first flush at 00:02:00 takes 100s.
		// Without dedup_interval, all the samples of the skipped interval must be aggregated at 00:04:00.
		sameSeriesPushTimes := []string{"00:01:40", "00:02:10", "00:02:40", "00:03:10", "00:03:36", "00:04:20", "00:05:20"}
		f(sameSeriesConfig, true, 100*s, sameSeriesPushTimes, "00:06:20", []flushResult{
			{"00:02:00", 1},
			{"00:04:00", 4},
			{"00:05:00", 1},
			{"00:06:00", 1},
		}, 1)

		// With dedup_interval, the samples of the skipped dedup intervals must be deduplicated together,
		// so only the last sample of the series must be aggregated at 00:04:00.
		f(fmt.Sprintf(sameSeriesDedupConfig, "count_samples"), true, 100*s, sameSeriesPushTimes, "00:06:20", []flushResult{
			{"00:02:00", 1},
			{"00:04:00", 1},
			{"00:05:00", 1},
			{"00:06:00", 1},
		}, 4)
		f(fmt.Sprintf(sameSeriesDedupConfig, "last"), true, 100*s, sameSeriesPushTimes, "00:06:20", []flushResult{
			{"00:02:00", 1},
			{"00:04:00", 5},
			{"00:05:00", 6},
			{"00:06:00", 7},
		}, 4)

		// The aggregator with non-aligned flushes starts at 00:00:10, so the flushes happen at 00:01:10, 00:02:10, 00:03:10, etc.
		// The cases are the same as above, shifted to the non-aligned flush times.

		f(config, false, 90*s, []string{"00:00:40", "00:01:40", "00:02:46", "00:03:40", "00:04:40"}, "00:05:40", []flushResult{
			{"00:01:10", 1},
			{"00:03:10", 2},
			{"00:04:10", 1},
			{"00:05:10", 1},
		}, 1)

		f(config, false, 150*s, []string{"00:00:40", "00:02:10", "00:03:10", "00:03:50", "00:04:40"}, "00:05:40", []flushResult{
			{"00:01:10", 1},
			{"00:04:10", 3},
			{"00:05:10", 1},
		}, 2)

		f(dedupConfig, false, 100*s, []string{"00:00:50", "00:01:30", "00:02:46", "00:03:30", "00:04:30"}, "00:05:30", []flushResult{
			{"00:01:10", 1},
			{"00:03:10", 2},
			{"00:04:10", 1},
			{"00:05:10", 1},
		}, 4)

		sameSeriesPushTimes = []string{"00:00:50", "00:01:20", "00:01:50", "00:02:20", "00:02:46", "00:03:30", "00:04:30"}
		f(sameSeriesConfig, false, 100*s, sameSeriesPushTimes, "00:05:30", []flushResult{
			{"00:01:10", 1},
			{"00:03:10", 4},
			{"00:04:10", 1},
			{"00:05:10", 1},
		}, 1)

		f(fmt.Sprintf(sameSeriesDedupConfig, "count_samples"), false, 100*s, sameSeriesPushTimes, "00:05:30", []flushResult{
			{"00:01:10", 1},
			{"00:03:10", 1},
			{"00:04:10", 1},
			{"00:05:10", 1},
		}, 4)
		f(fmt.Sprintf(sameSeriesDedupConfig, "last"), false, 100*s, sameSeriesPushTimes, "00:05:30", []flushResult{
			{"00:01:10", 1},
			{"00:03:10", 5},
			{"00:04:10", 6},
			{"00:05:10", 7},
		}, 4)
	}
}

// at returns the given time of the day hh:mm:ss on the synctest fake clock, which starts at 2000-01-01 00:00:00 UTC.
func at(timeOfDay string) time.Time {
	t, err := time.Parse(time.DateTime, "2000-01-01 "+timeOfDay)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot parse time of the day %q: %w", timeOfDay, err))
	}
	return t
}

// timeOfDay returns the time of the day hh:mm:ss for the given timestamp in milliseconds on the synctest fake clock.
func timeOfDay(timestampMsec int64) string {
	return time.UnixMilli(timestampMsec).UTC().Format(time.TimeOnly)
}

func TestAggregatorIgnoreFirstIntervals(t *testing.T) {
	type flushResult struct {
		// timestamp is the time of the day of the output sample
		timestamp string
		value     float64
	}

	// f creates an aggregator for the given config at 00:01:00, pushes a sample foo{i="N"}
	// at every time of the day from pushTimes, where N is the index of the time, and stops the aggregator at stopTime.
	// Then it verifies the flushed outputs, including the output of the flush on shutdown.
	f := func(config string, pushTimes []string, stopTime string, resultsExpected []flushResult) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			// Start the aggregator on the interval boundary, so it has no partial first interval.
			time.Sleep(time.Until(at("00:01:00")))

			var results []prompb.TimeSeries
			var resultsLock sync.Mutex
			pushFunc := func(tss []prompb.TimeSeries) {
				resultsLock.Lock()
				results = appendClonedTimeseries(results, tss)
				resultsLock.Unlock()
			}
			a, err := LoadFromData([]byte(config), pushFunc, nil, "some_alias")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}

			for i, pushTime := range pushTimes {
				time.Sleep(time.Until(at(pushTime)))
				tss := prometheus.MustParsePromMetrics(fmt.Sprintf(`foo{i="%d"} 1`, i), time.Now().UnixMilli())
				a.Push(tss, nil)
			}
			time.Sleep(time.Until(at(stopTime)))
			synctest.Wait()
			a.MustStop()

			var got []flushResult
			for _, ts := range results {
				for _, s := range ts.Samples {
					got = append(got, flushResult{
						timestamp: timeOfDay(s.Timestamp),
						value:     s.Value,
					})
				}
			}
			if !reflect.DeepEqual(got, resultsExpected) {
				t.Fatalf("unexpected results;\ngot\n%v\nwant\n%v", got, resultsExpected)
			}
		})
	}

	// The aggregator starts at 00:01:00, so the flushes happen at 00:02:00, 00:03:00, 00:04:00, etc.
	pushTimes := []string{"00:01:30", "00:02:30", "00:03:30", "00:04:30"}

	// The flushes at 00:02:00 and 00:03:00 must be dropped, while the next ones must be pushed.
	f(`
- interval: 1m
  ignore_first_intervals: 2
  without: [i]
  outputs: [count_samples]
`, pushTimes, "00:04:40", []flushResult{
		{"00:04:00", 1},
	})

	// With flush_on_shutdown, the stop at 00:04:40 must flush the latest incomplete interval
	// once the ignored intervals have passed. Since flushes are aligned to the interval,
	// its timestamp is 00:05:00, the end of that interval.
	f(`
- interval: 1m
  ignore_first_intervals: 2
  flush_on_shutdown: true
  without: [i]
  outputs: [count_samples]
`, pushTimes, "00:04:40", []flushResult{
		{"00:04:00", 1},
		{"00:05:00", 1},
	})

	// The flush on shutdown must be dropped while ignored intervals remain, even with flush_on_shutdown.
	f(`
- interval: 1m
  ignore_first_intervals: 5
  flush_on_shutdown: true
  without: [i]
  outputs: [count_samples]
`, pushTimes, "00:04:40", nil)

	// ignore_first_intervals counts the flushes at 00:02:00 and 00:03:00, not the dedup flushes every 30s.
	f(`
- interval: 1m
  dedup_interval: 30s
  ignore_first_intervals: 2
  without: [i]
  outputs: [count_samples]
`, pushTimes, "00:04:40", []flushResult{
		{"00:04:00", 1},
	})
}

func TestAggregatorWindowsLateSamples(t *testing.T) {
	type flushResult struct {
		// timestamp is the time of the day of the output sample
		timestamp string
		value     float64
	}
	type push struct {
		// pushTime is the time of the day of the push
		pushTime string
		// sampleTime is the time of the day of the sample timestamp
		sampleTime string
	}

	// f creates an aggregator for the given config at 00:01:00, makes every push from pushes
	// with a sample foo{i="N"} with value 2^N, where N is the index of the push,
	// so the sum_samples output shows which samples got into every flushed window,
	// and stops the aggregator at stopTime.
	// Then it verifies the flushed outputs and the number of samples dropped as too old.
	f := func(config string, pushes []push, stopTime string, resultsExpected []flushResult, ignoredOldSamplesExpected uint64) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			// Start the aggregator on the interval boundary, so it has no partial first interval.
			time.Sleep(time.Until(at("00:01:00")))

			var results []prompb.TimeSeries
			var resultsLock sync.Mutex
			pushFunc := func(tss []prompb.TimeSeries) {
				resultsLock.Lock()
				results = appendClonedTimeseries(results, tss)
				resultsLock.Unlock()
			}
			a, err := LoadFromData([]byte(config), pushFunc, nil, "some_alias")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			ag := a.as[0]

			for i, p := range pushes {
				time.Sleep(time.Until(at(p.pushTime)))
				tss := prometheus.MustParsePromMetrics(fmt.Sprintf(`foo{i="%d"} %d`, i, 1<<i), at(p.sampleTime).UnixMilli())
				a.Push(tss, nil)
			}
			time.Sleep(time.Until(at(stopTime)))
			synctest.Wait()
			ignoredOldSamples := ag.ignoredOldSamples.Get()
			a.MustStop()

			if ignoredOldSamples != ignoredOldSamplesExpected {
				t.Fatalf("unexpected number of samples dropped as too old; got %d; want %d", ignoredOldSamples, ignoredOldSamplesExpected)
			}
			var got []flushResult
			for _, ts := range results {
				for _, s := range ts.Samples {
					got = append(got, flushResult{
						timestamp: timeOfDay(s.Timestamp),
						value:     s.Value,
					})
				}
			}
			if !reflect.DeepEqual(got, resultsExpected) {
				t.Fatalf("unexpected results;\ngot\n%v\nwant\n%v", got, resultsExpected)
			}
		})
	}

	const config = `
- interval: 1m
  enable_windows: true
  without: [i]
  outputs: [sum_samples]
`

	// The aggregator starts at 00:01:00, so the flushes happen at 00:02:00, 00:03:00, etc.

	// Samples arrive with a steady 20s lag, so every flush waits 20s for the late samples of the due window.
	// The sample pushed at 00:02:15 belongs to the window ending at 00:02:00 and must be flushed with it at 00:02:20.
	// The sample pushed at 00:02:25 arrives after that flush, so it belongs to the next window.
	f(config, []push{
		{"00:01:30", "00:01:10"},
		{"00:02:15", "00:01:55"},
		{"00:02:25", "00:02:05"},
		{"00:03:10", "00:02:55"},
	}, "00:03:40", []flushResult{
		{"00:02:00", 1 + 2},
		{"00:03:00", 4 + 8},
	}, 0)

	// Samples without lag don't delay the flush at 00:02:00,
	// so the late sample arriving at 00:02:10 after the flush must be dropped as too old.
	// If no samples are accepted since the previous flush, the lag of dropped samples is used for the flush delay,
	// so the late sample arriving at 00:03:10 must be flushed with the window ending at 00:03:00.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/11866
	f(config, []push{
		{"00:01:30", "00:01:30"},
		{"00:02:10", "00:01:55"},
		{"00:03:10", "00:02:55"},
	}, "00:03:40", []flushResult{
		{"00:02:00", 1},
		{"00:03:00", 4},
	}, 1)

	// The flush delay is capped at 90% of the interval, so with a 59s lag the flush happens at 00:02:54,
	// and a sample arriving at 00:02:56 must be dropped as too old.
	// Stop before the next flush at 00:03:00, since the stop must not depend on the flush delay.
	f(config, []push{
		{"00:01:59", "00:01:00"},
		{"00:02:56", "00:01:59"},
	}, "00:02:58", []flushResult{
		{"00:02:00", 1},
	}, 1)
}
