//go:build synctest

package streamaggr

import (
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

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
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
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
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
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
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
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
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
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
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
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
}
