package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"strconv"
	"sync"
	"testing"
)

func TestHistogramAggregatorsFailure(t *testing.T) {
	f := func(config string) {
		t.Helper()
		pushFunc := func(tss []prompbmarshal.TimeSeries) {
			panic(fmt.Errorf("pushFunc shouldn't be called"))
		}
		a, err := newAggregatorsFromData([]byte(config), pushFunc, nil)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if a != nil {
			t.Fatalf("expecting nil a")
		}
	}

	// missing outputs
	f(`
- interval: 1m
- histogram_outputs: [histogram_merge]
`)

	// Invalid histogram_output
	f(`
- interval: 1m
- outputs: [total]
- histogram_outputs: [foo]
`)
}

func TestHistogramAggregatorsEqual(t *testing.T) {
	f := func(a, b string, expectedResult bool) {
		t.Helper()

		pushFunc := func(tss []prompbmarshal.TimeSeries) {}
		aa, err := newAggregatorsFromData([]byte(a), pushFunc, nil)
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		ab, err := newAggregatorsFromData([]byte(b), pushFunc, nil)
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		result := aa.Equal(ab)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
		}
	}
	f(`
- outputs: [total]
  histogram_outputs: [histogram_merge]
  interval: 5m
`, `
- histogram_outputs: [histogram_merge]
  outputs: [total]
  interval: 5m
`, true)
}

func TestHistogramAggregatorsSuccess(t *testing.T) {
	f := func(config, inputMetrics, outputMetricsExpected, matchIdxsStrExpected string) {
		t.Helper()

		// Initialize Aggregators
		var tssOutput []prompbmarshal.TimeSeries
		var tssOutputLock sync.Mutex
		pushFunc := func(tss []prompbmarshal.TimeSeries) {
			tssOutputLock.Lock()
			tssOutput = appendClonedTimeseries(tssOutput, tss)
			tssOutputLock.Unlock()
		}
		opts := &Options{
			FlushOnShutdown:        true,
			NoAlignFlushToInterval: true,
		}
		a, err := newAggregatorsFromData([]byte(config), pushFunc, opts)
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}

		// Push the inputMetrics to Aggregators
		tssInput := mustParsePromMetrics(inputMetrics)
		matchIdxs := a.Push(tssInput, nil)
		a.MustStop()

		// Verify matchIdxs equals to matchIdxsExpected
		matchIdxsStr := ""
		for _, v := range matchIdxs {
			matchIdxsStr += strconv.Itoa(int(v))
		}
		if matchIdxsStr != matchIdxsStrExpected {
			t.Fatalf("unexpected matchIdxs;\ngot\n%s\nwant\n%s", matchIdxsStr, matchIdxsStrExpected)
		}

		// Verify the tssOutput contains the expected metrics
		outputMetrics := timeSeriessToString(tssOutput, false)
		if outputMetrics != outputMetricsExpected {
			t.Fatalf("unexpected output metrics;\ngot\n%s\nwant\n%s", outputMetrics, outputMetricsExpected)
		}
	}

	// Empty config
	f(``, ``, ``, "")
	f(``, `foo{bar="baz"} 1`, ``, "0")
	f(``, "foo 1\nbaz 2", ``, "00")

	// Empty by list - aggregate only by time
	f(`
- interval: 1m
  outputs: [count_samples, sum_samples, count_series, last]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_count_samples 1
bar:1m_count_series 1
bar:1m_last 5
bar:1m_sum_samples 5
foo:1m_count_samples{abc="123"} 2
foo:1m_count_samples{abc="456",de="fg"} 1
foo:1m_count_series{abc="123"} 1
foo:1m_count_series{abc="456",de="fg"} 1
foo:1m_last{abc="123"} 8.5
foo:1m_last{abc="456",de="fg"} 8
foo:1m_sum_samples{abc="123"} 12.5
foo:1m_sum_samples{abc="456",de="fg"} 8
`, "1111")

	// Special case: __name__ in `by` list - this is the same as empty `by` list
	f(`
- interval: 1m
  by: [__name__]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_count_samples 1
bar:1m_count_series 1
bar:1m_sum_samples 5
foo:1m_count_samples 3
foo:1m_count_series 2
foo:1m_sum_samples 20.5
`, "1111")

	// Non-empty `by` list with non-existing labels
	f(`
- interval: 1m
  by: [foo, bar]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_by_bar_foo_count_samples 1
bar:1m_by_bar_foo_count_series 1
bar:1m_by_bar_foo_sum_samples 5
foo:1m_by_bar_foo_count_samples 3
foo:1m_by_bar_foo_count_series 2
foo:1m_by_bar_foo_sum_samples 20.5
`, "1111")

	// Non-empty `by` list with existing label
	f(`
- interval: 1m
  by: [abc]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_by_abc_count_samples 1
bar:1m_by_abc_count_series 1
bar:1m_by_abc_sum_samples 5
foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, "1111")

	// Non-empty `by` list with duplicate existing label
	f(`
- interval: 1m
  by: [abc, abc]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_by_abc_count_samples 1
bar:1m_by_abc_count_series 1
bar:1m_by_abc_sum_samples 5
foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, "1111")

	// Non-empty `without` list with non-existing labels
	f(`
- interval: 1m
  without: [foo]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_without_foo_count_samples 1
bar:1m_without_foo_count_series 1
bar:1m_without_foo_sum_samples 5
foo:1m_without_foo_count_samples{abc="123"} 2
foo:1m_without_foo_count_samples{abc="456",de="fg"} 1
foo:1m_without_foo_count_series{abc="123"} 1
foo:1m_without_foo_count_series{abc="456",de="fg"} 1
foo:1m_without_foo_sum_samples{abc="123"} 12.5
foo:1m_without_foo_sum_samples{abc="456",de="fg"} 8
`, "1111")

	// Non-empty `without` list with existing labels
	f(`
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_without_abc_count_samples 1
bar:1m_without_abc_count_series 1
bar:1m_without_abc_sum_samples 5
foo:1m_without_abc_count_samples 2
foo:1m_without_abc_count_samples{de="fg"} 1
foo:1m_without_abc_count_series 1
foo:1m_without_abc_count_series{de="fg"} 1
foo:1m_without_abc_sum_samples 12.5
foo:1m_without_abc_sum_samples{de="fg"} 8
`, "1111")

	// Special case: __name__ in `without` list
	f(`
- interval: 1m
  without: [__name__]
  outputs: [count_samples, sum_samples, count_series]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `:1m_count_samples 1
:1m_count_samples{abc="123"} 2
:1m_count_samples{abc="456",de="fg"} 1
:1m_count_series 1
:1m_count_series{abc="123"} 1
:1m_count_series{abc="456",de="fg"} 1
:1m_sum_samples 5
:1m_sum_samples{abc="123"} 12.5
:1m_sum_samples{abc="456",de="fg"} 8
`, "1111")

	// drop some input metrics
	f(`
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  input_relabel_configs:
  - if: 'foo'
    action: drop
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_without_abc_count_samples 1
bar:1m_without_abc_count_series 1
bar:1m_without_abc_sum_samples 5
`, "1111")

	// rename output metrics
	f(`
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
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar-1m-without-abc-count-samples 1
bar-1m-without-abc-count-series 1
bar-1m-without-abc-sum-samples 5
foo-1m-without-abc-count-samples 2
foo-1m-without-abc-count-series 1
foo-1m-without-abc-sum-samples 12.5
`, "1111")

	// match doesn't match anything
	f(`
- interval: 1m
  without: [abc]
  outputs: [count_samples, sum_samples, count_series]
  match: '{non_existing_label!=""}'
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, ``, "0000")

	// match matches foo series with non-empty abc label
	f(`
- interval: 1m
  by: [abc]
  outputs: [count_samples, sum_samples, count_series]
  match:
  - foo{abc=~".+"}
  - '{non_existing_label!=""}'
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `foo:1m_by_abc_count_samples{abc="123"} 2
foo:1m_by_abc_count_samples{abc="456"} 1
foo:1m_by_abc_count_series{abc="123"} 1
foo:1m_by_abc_count_series{abc="456"} 1
foo:1m_by_abc_sum_samples{abc="123"} 12.5
foo:1m_by_abc_sum_samples{abc="456"} 8
`, "1011")

	// total output for non-repeated series
	f(`
- interval: 1m
  outputs: [total]
`, `
foo 123
bar{baz="qwe"} 4.34
`, `bar:1m_total{baz="qwe"} 0
foo:1m_total 0
`, "11")

	// total_prometheus output for non-repeated series
	f(`
- interval: 1m
  outputs: [total_prometheus]
`, `
foo 123
bar{baz="qwe"} 4.34
`, `bar:1m_total{baz="qwe"} 0
foo:1m_total 0
`, "11")

	// total output for repeated series
	f(`
- interval: 1m
  outputs: [total]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_total{baz="qwe"} 5.02
bar:1m_total{baz="qwer"} 1
foo:1m_total 0
foo:1m_total{baz="qwe"} 15
`, "11111111")

	// total_prometheus output for repeated series
	f(`
- interval: 1m
  outputs: [total_prometheus]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_total{baz="qwe"} 5.02
bar:1m_total{baz="qwer"} 1
foo:1m_total 0
foo:1m_total{baz="qwe"} 15
`, "11111111")

	// total output for repeated series with group by __name__
	f(`
- interval: 1m
  by: [__name__]
  outputs: [total]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_total 6.02
foo:1m_total 15
`, "11111111")

	// total_prometheus output for repeated series with group by __name__
	f(`
- interval: 1m
  by: [__name__]
  outputs: [total_prometheus]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_total 6.02
foo:1m_total 15
`, "11111111")

	// increase output for non-repeated series
	f(`
- interval: 1m
  outputs: [increase]
`, `
foo 123
bar{baz="qwe"} 4.34
`, `bar:1m_increase{baz="qwe"} 0
foo:1m_increase 0
`, "11")

	// increase_prometheus output for non-repeated series
	f(`
- interval: 1m
  outputs: [increase_prometheus]
`, `
foo 123
bar{baz="qwe"} 4.34
`, `bar:1m_increase{baz="qwe"} 0
foo:1m_increase 0
`, "11")

	// increase output for repeated series
	f(`
- interval: 1m
  outputs: [increase]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_increase{baz="qwe"} 5.02
bar:1m_increase{baz="qwer"} 1
foo:1m_increase 0
foo:1m_increase{baz="qwe"} 15
`, "11111111")

	// increase_prometheus output for repeated series
	f(`
- interval: 1m
  outputs: [increase_prometheus]
`, `
foo 123
bar{baz="qwe"} 1.32
bar{baz="qwe"} 4.34
bar{baz="qwe"} 2
foo{baz="qwe"} -5
bar{baz="qwer"} 343
bar{baz="qwer"} 344
foo{baz="qwe"} 10
`, `bar:1m_increase{baz="qwe"} 5.02
bar:1m_increase{baz="qwer"} 1
foo:1m_increase 0
foo:1m_increase{baz="qwe"} 15
`, "11111111")

	// multiple aggregate configs
	f(`
- interval: 1m
  outputs: [count_series, sum_samples]
- interval: 5m
  by: [bar]
  outputs: [sum_samples]
`, `
foo 1
foo{bar="baz"} 2
foo 3.3
`, `foo:1m_count_series 1
foo:1m_count_series{bar="baz"} 1
foo:1m_sum_samples 4.3
foo:1m_sum_samples{bar="baz"} 2
foo:5m_by_bar_sum_samples 4.3
foo:5m_by_bar_sum_samples{bar="baz"} 2
`, "111")

	// min and max outputs
	f(`
- interval: 1m
  outputs: [min, max]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_max 5
bar:1m_min 5
foo:1m_max{abc="123"} 8.5
foo:1m_max{abc="456",de="fg"} 8
foo:1m_min{abc="123"} 4
foo:1m_min{abc="456",de="fg"} 8
`, "1111")

	// avg output
	f(`
- interval: 1m
  outputs: [avg]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_avg 5
foo:1m_avg{abc="123"} 6.25
foo:1m_avg{abc="456",de="fg"} 8
`, "1111")

	// stddev output
	f(`
- interval: 1m
  outputs: [stddev]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_stddev 0
foo:1m_stddev{abc="123"} 2.25
foo:1m_stddev{abc="456",de="fg"} 0
`, "1111")

	// stdvar output
	f(`
- interval: 1m
  outputs: [stdvar]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar:1m_stdvar 0
foo:1m_stdvar{abc="123"} 5.0625
foo:1m_stdvar{abc="456",de="fg"} 0
`, "1111")

	// histogram_bucket output
	f(`
- interval: 1m
  outputs: [histogram_bucket]
`, `
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`, `cpu_usage:1m_histogram_bucket{cpu="1",vmrange="1.136e+01...1.292e+01"} 2
cpu_usage:1m_histogram_bucket{cpu="1",vmrange="1.292e+01...1.468e+01"} 3
cpu_usage:1m_histogram_bucket{cpu="1",vmrange="2.448e+01...2.783e+01"} 1
cpu_usage:1m_histogram_bucket{cpu="2",vmrange="8.799e+01...1.000e+02"} 1
`, "1111111")

	// histogram_bucket output without cpu
	f(`
- interval: 1m
  without: [cpu]
  outputs: [histogram_bucket]
`, `
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`, `cpu_usage:1m_without_cpu_histogram_bucket{vmrange="1.136e+01...1.292e+01"} 2
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="1.292e+01...1.468e+01"} 3
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="2.448e+01...2.783e+01"} 1
cpu_usage:1m_without_cpu_histogram_bucket{vmrange="8.799e+01...1.000e+02"} 1
`, "1111111")

	// quantiles output
	f(`
- interval: 1m
  outputs: ["quantiles(0, 0.5, 1)"]
`, `
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`, `cpu_usage:1m_quantiles{cpu="1",quantile="0"} 12
cpu_usage:1m_quantiles{cpu="1",quantile="0.5"} 13.3
cpu_usage:1m_quantiles{cpu="1",quantile="1"} 25
cpu_usage:1m_quantiles{cpu="2",quantile="0"} 90
cpu_usage:1m_quantiles{cpu="2",quantile="0.5"} 90
cpu_usage:1m_quantiles{cpu="2",quantile="1"} 90
`, "1111111")

	// quantiles output without cpu
	f(`
- interval: 1m
  without: [cpu]
  outputs: ["quantiles(0, 0.5, 1)"]
`, `
cpu_usage{cpu="1"} 12.5
cpu_usage{cpu="1"} 13.3
cpu_usage{cpu="1"} 13
cpu_usage{cpu="1"} 12
cpu_usage{cpu="1"} 14
cpu_usage{cpu="1"} 25
cpu_usage{cpu="2"} 90
`, `cpu_usage:1m_without_cpu_quantiles{quantile="0"} 12
cpu_usage:1m_without_cpu_quantiles{quantile="0.5"} 13.3
cpu_usage:1m_without_cpu_quantiles{quantile="1"} 90
`, "1111111")

	// append additional label
	f(`
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
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
foo{abc="456",de="fg"} 8
`, `bar-1m-without-abc-count-samples{new_label="must_keep_metric_name"} 1
bar-1m-without-abc-count-series{new_label="must_keep_metric_name"} 1
bar-1m-without-abc-sum-samples{new_label="must_keep_metric_name"} 5
foo-1m-without-abc-count-samples{new_label="must_keep_metric_name"} 2
foo-1m-without-abc-count-series{new_label="must_keep_metric_name"} 1
foo-1m-without-abc-sum-samples{new_label="must_keep_metric_name"} 12.5
`, "1111")

	// keep_metric_names
	f(`
- interval: 1m
  keep_metric_names: true
  outputs: [count_samples]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
bar -34.3
foo{abc="456",de="fg"} 8
`, `bar 2
foo{abc="123"} 2
foo{abc="456",de="fg"} 1
`, "11111")

	// drop_input_labels
	f(`
- interval: 1m
  drop_input_labels: [abc]
  keep_metric_names: true
  outputs: [count_samples]
`, `
foo{abc="123"} 4
bar 5
foo{abc="123"} 8.5
bar -34.3
foo{abc="456",de="fg"} 8
`, `bar 2
foo 2
foo{de="fg"} 1
`, "11111")
}
