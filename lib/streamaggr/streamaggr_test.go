package streamaggr

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func TestAggregatorsFailure(t *testing.T) {
	f := func(config string) {
		t.Helper()
		pushFunc := func(_ []prompb.TimeSeries) {
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

		pushFunc := func(_ []prompb.TimeSeries) {}
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

func timeSeriessToString(tss []prompb.TimeSeries) string {
	sorted := make([]prompb.TimeSeries, len(tss))
	copy(sorted, tss)
	sort.SliceStable(sorted, func(i, j int) bool {
		return promrelabel.LabelsToString(sorted[i].Labels) < promrelabel.LabelsToString(sorted[j].Labels)
	})
	a := make([]string, len(sorted))
	for i, ts := range sorted {
		a[i] = timeSeriesToString(ts)
	}
	return strings.Join(a, "")
}

func timeSeriesToString(ts prompb.TimeSeries) string {
	labelsString := promrelabel.LabelsToString(ts.Labels)
	if len(ts.Samples) != 1 {
		panic(fmt.Errorf("unexpected number of samples for %s: %d; want 1", labelsString, len(ts.Samples)))
	}
	return fmt.Sprintf("%s %v\n", labelsString, ts.Samples[0].Value)
}

func appendClonedTimeseries(dst, src []prompb.TimeSeries) []prompb.TimeSeries {
	for _, ts := range src {
		dst = append(dst, prompb.TimeSeries{
			Labels:  append(ts.Labels[:0:0], ts.Labels...),
			Samples: append(ts.Samples[:0:0], ts.Samples...),
		})
	}
	return dst
}

func TestFlushLag(t *testing.T) {
	type push struct {
		hasAccepted     bool
		acceptedLagMsec int64
		droppedLagMsec  int64
	}
	const interval = 10 * time.Second
	f := func(pushes []push, delayExpected time.Duration) {
		t.Helper()
		var fl flushLag
		for _, p := range pushes {
			fl.update(p.hasAccepted, p.acceptedLagMsec, p.droppedLagMsec)
		}
		if delay := fl.takeFlushDelay(interval); delay != delayExpected {
			t.Fatalf("unexpected delay; got %s; want %s", delay, delayExpected)
		}
		// The lags must be reset after takeFlushDelay.
		if delay := fl.takeFlushDelay(interval); delay != 0 {
			t.Fatalf("unexpected delay after reset; got %s; want 0s", delay)
		}
	}

	// no pushes
	f(nil, 0)

	// nothing accepted or dropped
	f([]push{{}}, 0)

	// the max accepted lag is used
	f([]push{{true, 1000, 0}, {true, 3000, 0}, {true, 2000, 0}}, 3*time.Second)

	// accepted lag wins over dropped lag
	f([]push{{true, 1000, 0}, {false, 0, 5000}}, time.Second)
	f([]push{{false, 0, 5000}, {true, 0, 0}}, 0)

	// dropped lag is used if no samples were accepted
	f([]push{{false, 0, 4000}, {false, 0, 6000}}, 6*time.Second)

	// the delay is capped at 90% of the interval
	f([]push{{true, 15000, 0}}, 9*time.Second)
	f([]push{{false, 0, 15000}}, 9*time.Second)
}
