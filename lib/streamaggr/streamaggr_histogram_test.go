package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"slices"
	"strconv"
	"strings"
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

	// Invalid histogram_output
	f(`
- interval: 1m
  outputs: [total]
  histogram_outputs: [foo]
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
- histogram_outputs: [histogram_merge]
  interval: 5m
`, `
- interval: 5m
  histogram_outputs: [histogram_merge]
`, true)
}

func TestHistogramAggregatorsSuccess(t *testing.T) {
	// Sort []TimeSeries by labels to make test less flaky
	sortTimeSeries := func(a, b prompbmarshal.TimeSeries) int {
		return strings.Compare(promrelabel.LabelsToString(a.Labels), promrelabel.LabelsToString(b.Labels))
	}
	f := func(config string,
		inputMetrics []prompbmarshal.TimeSeries,
		outputMetricsExpected []prompbmarshal.TimeSeries,
		matchIdxsStrExpected string) {

		t.Helper()

		// Initialize Aggregators
		var tssOutput []prompbmarshal.TimeSeries
		var tssOutputLock sync.Mutex
		pushFunc := func(tss []prompbmarshal.TimeSeries) {
			for _, ts := range tss {
				// sort labels to make tests less flaky
				promrelabel.SortLabels(ts.Labels)
			}
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
		matchIdxs := a.Push(inputMetrics, nil)
		a.MustStop()

		// Verify matchIdxs equals to matchIdxsExpected
		matchIdxsStr := ""
		for _, v := range matchIdxs {
			matchIdxsStr += strconv.Itoa(int(v))
		}
		if matchIdxsStr != matchIdxsStrExpected {
			t.Fatalf("unexpected matchIdxs;\ngot\n%s\nwant\n%s", matchIdxsStr, matchIdxsStrExpected)
		}

		slices.SortFunc(outputMetricsExpected, sortTimeSeries)
		slices.SortFunc(tssOutput, sortTimeSeries)

		if diff := cmp.Diff(outputMetricsExpected, tssOutput); diff != "" {
			t.Fatalf("unexpected output metrics;\ngot\n%v\nwant\n%v\n%s", tssOutput, outputMetricsExpected, diff)
		}
	}

	emptyTs := []prompbmarshal.TimeSeries{}
	histo0 := tsdbutil.GenerateTestFloatHistogram(0)
	histo1 := tsdbutil.GenerateTestFloatHistogram(1)
	histo2 := tsdbutil.GenerateTestFloatHistogram(2)

	histo1s0 := histo1.Copy().Sub(histo0)
	histo2s0 := histo2.Copy().Sub(histo0)

	zeroHisto := &histogram.FloatHistogram{
		CounterResetHint: histo0.CounterResetHint,
		Schema:           histo0.Schema,
		ZeroThreshold:    histo0.ZeroThreshold,
	}

	// Empty config
	f(``, emptyTs, nil, "")
	f(``, []prompbmarshal.TimeSeries{
		generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo0}),
	},
		nil, "0")
	f(``, []prompbmarshal.TimeSeries{
		generateHistogramTimeSeries("foo", nil, []*histogram.FloatHistogram{histo0}),
		generateHistogramTimeSeries("bar", nil, []*histogram.FloatHistogram{histo0}),
	},
		nil, "00")

	// Empty by list - aggregate only by time
	f(`
- interval: 1m
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo1s0}),
		}, "11")

	//Special case: __name__ in `by` list - this is the same as empty `by` list
	f(`
- interval: 1m
  by: [__name__]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
			generateHistogramTimeSeries("bar:1m_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo1s0}),
		}, "111111")

	// Non-empty `by` list with non-existing labels
	f(`
- interval: 1m
  by: [foo, bar]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_by_bar_foo_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
		}, "1111")

	// Non-empty `by` list with existing label
	f(`
- interval: 1m
  by: [abc]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "456"}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	//Non-empty `by` list with duplicate existing label
	f(`
- interval: 1m
  by: [abc, abc]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "456"}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	//	// Non-empty `without` list with non-existing labels
	f(`
- interval: 1m
  without: [foo]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_without_foo_histogram_merge", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo:1m_without_foo_histogram_merge", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	// Non-empty `without` list with existing labels
	f(`
- interval: 1m
  without: [abc]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_without_abc_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo:1m_without_abc_histogram_merge", map[string]string{"de": "fg"}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	// Special case: __name__ in `without` list
	f(`
- interval: 1m
  without: [__name__]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries(":1m_histogram_merge", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries(":1m_histogram_merge", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	// drop some input metrics
	f(`
- interval: 1m
  without: [abc]
  histogram_outputs: [histogram_merge]
  input_relabel_configs:
  - if: 'foo'
    action: drop
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("bar:1m_without_abc_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
		}, "111111")

	// rename output metrics
	f(`
- interval: 1m
  without: [abc]
  histogram_outputs: [histogram_merge]
  output_relabel_configs:
  - action: replace_all
    source_labels: [__name__]
    regex: ":|_"
    replacement: "-"
    target_label: __name__
  - action: drop
    source_labels: [de]
    regex: fg
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo-1m-without-abc-histogram-merge", map[string]string{}, []*histogram.FloatHistogram{histo1s0}),
		}, "1111")

	// match doesn't match anything
	f(`
- interval: 1m
  without: [abc]
  histogram_outputs: [histogram_merge]
  match: '{non_existing_label!=""}'
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		}, nil, "0000")

	// match matches foo series with non-empty abc label
	f(`
- interval: 1m
  by: [abc]
  histogram_outputs: [histogram_merge]
  match:
  - foo{abc=~".+"}
  - '{non_existing_label!=""}'
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo:1m_by_abc_histogram_merge", map[string]string{"abc": "456"}, []*histogram.FloatHistogram{histo1s0}),
		}, "001111")

	// histogram_merge output for non-repeated series
	f(`
- interval: 1m
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo0}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{zeroHisto}),
			generateHistogramTimeSeries("bar:1m_histogram_merge", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{zeroHisto}),
		}, "11")

	// histogram_merge output for repeated series
	f(`
- interval: 1m
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{zeroHisto}),
			generateHistogramTimeSeries("bar:1m_histogram_merge", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{"baz": "qwe"}, []*histogram.FloatHistogram{histo1s0}),
		}, "111111")

	// multiple aggregate configs
	f(`
- interval: 1m
  histogram_outputs: [histogram_merge]
- interval: 5m
  by: [bar]
  outputs: [sum_samples]
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("foo", map[string]string{}, []*histogram.FloatHistogram{histo2}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
			generateHistogramTimeSeries("foo:1m_histogram_merge", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{zeroHisto}),
			generateHistogramTimeSeries("foo:5m_by_bar_histogram_merge", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
			generateHistogramTimeSeries("foo:5m_by_bar_histogram_merge", map[string]string{"bar": "baz"}, []*histogram.FloatHistogram{zeroHisto}),
		}, "111")

	// append additional label
	f(`
- interval: 1m
  without: [abc]
  histogram_outputs: [histogram_merge]
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
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo-1m-without-abc-histogram-merge", map[string]string{"new_label": "must_keep_metric_name"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("bar-1m-without-abc-histogram-merge", map[string]string{"new_label": "must_keep_metric_name"}, []*histogram.FloatHistogram{histo2s0}),
		}, "111111")

	// keep_metric_names
	f(`
- interval: 1m
  keep_metric_names: true
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1s0}),
		}, "111111")

	// drop_input_labels
	f(`
- interval: 1m
  drop_input_labels: [abc]
  keep_metric_names: true
  histogram_outputs: [histogram_merge]
`,
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "123"}, []*histogram.FloatHistogram{histo1}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo0}),
			generateHistogramTimeSeries("foo", map[string]string{"abc": "456", "de": "fg"}, []*histogram.FloatHistogram{histo1}),
		},
		[]prompbmarshal.TimeSeries{
			generateHistogramTimeSeries("bar", map[string]string{}, []*histogram.FloatHistogram{histo2s0}),
			generateHistogramTimeSeries("foo", map[string]string{}, []*histogram.FloatHistogram{histo1s0}),
			generateHistogramTimeSeries("foo", map[string]string{"de": "fg"}, []*histogram.FloatHistogram{histo1s0}),
		}, "111111")
}

// the generated histogram will have 2+i elements in the zero bucket
func generateHistogramTimeSeries(metricName string, labels map[string]string, floatHistograms []*histogram.FloatHistogram) prompbmarshal.TimeSeries {
	promLabels := make([]prompbmarshal.Label, len(labels)+1)
	promLabels[0] = prompbmarshal.Label{
		Name:  "__name__",
		Value: metricName,
	}

	var i = 1
	for k, v := range labels {
		promLabels[i] = prompbmarshal.Label{
			Name:  k,
			Value: v,
		}
		i++
	}

	histograms := make([]prompbmarshal.Histogram, len(floatHistograms))
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	for i, fh := range floatHistograms {
		histograms[i] = *prompbmarshal.FromFloatHistogram(fh)
		histograms[i].Timestamp = currentTimeMsec
	}

	return prompbmarshal.TimeSeries{
		Labels:     promLabels,
		Histograms: histograms,
	}
}
