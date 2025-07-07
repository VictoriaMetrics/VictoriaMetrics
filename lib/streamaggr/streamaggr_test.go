package streamaggr

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAggregatorsFailure(t *testing.T) {
	f := func(config string) {
		t.Helper()
		pushFunc := func(_ []prompbmarshal.TimeSeries) {
			panic(fmt.Errorf("pushFunc shouldn't be called"))
		}
		a, err := LoadFromData([]byte(config), pushFunc, nil, "some_alias")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if a != nil {
			t.Fatalf("expecting nil a")
		}
	}

	// Invalid config
	f(`foobar`)

	// Unknown option
	f(`
- interval: 1m
  outputs: [total]
  foobar: baz
`)

	// missing interval
	f(`
- outputs: [total]
`)

	// missing outputs
	f(`
- interval: 1m
`)

	// Bad interval
	f(`
- interval: 1foo
  outputs: [total]
`)

	// Invalid output
	f(`
- interval: 1m
  outputs: [foobar]
`)

	// Negative interval
	f(`
- outputs: [total]
  interval: -5m
`)
	// Too small interval
	f(`
- outputs: [total]
  interval: 10ms
`)

	// bad dedup_interval
	f(`
- interval: 1m
  dedup_interval: 1foo
  outputs: ["quantiles"]
`)

	// interval isn't multiple of dedup_interval
	f(`
- interval: 1m
  dedup_interval: 35s
  outputs: ["quantiles"]
`)

	// dedup_interval is bigger than dedup_interval
	f(`
- interval: 1m
  dedup_interval: 1h
  outputs: ["quantiles"]
`)

	// bad staleness_interval
	f(`
- interval: 1m
  staleness_interval: 1foo
  outputs: ["quantiles"]
`)

	// staleness_interval should be > interval
	f(`
- interval: 1m
  staleness_interval: 30s
  outputs: ["quantiles"]
`)

	// staleness_interval should be multiple of interval
	f(`
- interval: 1m
  staleness_interval: 100s
  outputs: ["quantiles"]
`)

	// keep_metric_names is set for multiple inputs
	f(`
- interval: 1m
  keep_metric_names: true
  outputs: ["total", "increase"]
`)

	// keep_metric_names is set for unsupported input
	f(`
- interval: 1m
  keep_metric_names: true
  outputs: ["histogram_bucket"]
`)

	// Invalid input_relabel_configs
	f(`
- interval: 1m
  outputs: [total]
  input_relabel_configs:
  - foo: bar
`)
	f(`
- interval: 1m
  outputs: [total]
  input_relabel_configs:
  - action: replace
`)

	// Invalid output_relabel_configs
	f(`
- interval: 1m
  outputs: [total]
  output_relabel_configs:
  - foo: bar
`)
	f(`
- interval: 1m
  outputs: [total]
  output_relabel_configs:
  - action: replace
`)

	// Both by and without are non-empty
	f(`
- interval: 1m
  outputs: [total]
  by: [foo]
  without: [bar]
`)

	// Invalid quantiles()
	f(`
- interval: 1m
  outputs: ["quantiles("]
`)
	f(`
- interval: 1m
  outputs: ["quantiles()"]
`)
	f(`
- interval: 1m
  outputs: ["quantiles(foo)"]
`)
	f(`
- interval: 1m
  outputs: ["quantiles(-0.5)"]
`)
	f(`
- interval: 1m
  outputs: ["quantiles(1.5)"]
`)
	f(`
- interval: 1m
  outputs: [total, total]
`)
	// "quantiles(0.5)", "quantiles(0.9)" should be set as "quantiles(0.5, 0.9)"
	f(`
- interval: 1m
  outputs: ["quantiles(0.5)", "quantiles(0.9)"]
`)
}

func TestAggregatorsEqual(t *testing.T) {
	f := func(a, b string, expectedResult bool) {
		t.Helper()

		pushFunc := func(_ []prompbmarshal.TimeSeries) {}
		opts := Options{
			EnableWindows: true,
		}
		aa, err := LoadFromData([]byte(a), pushFunc, &opts, "some_alias")
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		ab, err := LoadFromData([]byte(b), pushFunc, &opts, "some_alias")
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		result := aa.Equal(ab)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
		}
	}
	f("", "", true)
	f(`
- outputs: [total]
  interval: 5m
`, ``, false)
	f(`
- outputs: [total]
  interval: 5m
`, `
- outputs: [total]
  interval: 5m
`, true)
	f(`
- outputs: [total]
  interval: 3m
`, `
- outputs: [total]
  interval: 5m
`, false)
	f(`
- outputs: [total]
  interval: 5m
  flush_on_shutdown: true  
`, `
- outputs: [total]
  interval: 5m
  flush_on_shutdown: false
`, false)
	f(`
- outputs: [total]
  interval: 5m
  ignore_first_intervals: 2
`, `
- outputs: [total]
  interval: 5m
  ignore_first_intervals: 4`, false)
}

func timeSeriessToString(tss []prompbmarshal.TimeSeries) string {
	a := make([]string, len(tss))
	for i, ts := range tss {
		a[i] = timeSeriesToString(ts)
	}
	sort.Strings(a)
	return strings.Join(a, "")
}

func timeSeriesToString(ts prompbmarshal.TimeSeries) string {
	labelsString := promrelabel.LabelsToString(ts.Labels)
	if len(ts.Samples) != 1 {
		panic(fmt.Errorf("unexpected number of samples for %s: %d; want 1", labelsString, len(ts.Samples)))
	}
	return fmt.Sprintf("%s %v\n", labelsString, ts.Samples[0].Value)
}

func appendClonedTimeseries(dst, src []prompbmarshal.TimeSeries) []prompbmarshal.TimeSeries {
	for _, ts := range src {
		dst = append(dst, prompbmarshal.TimeSeries{
			Labels:  append(ts.Labels[:0:0], ts.Labels...),
			Samples: append(ts.Samples[:0:0], ts.Samples...),
		})
	}
	return dst
}
