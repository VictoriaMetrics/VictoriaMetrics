package streamaggr

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
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

func TestAggregatorsReload(t *testing.T) {
	f := func(prevConfig, config, reusedExpected string) {
		t.Helper()

		pushFunc := func(_ []prompb.TimeSeries) {}
		prev, err := loadFromData([]byte(prevConfig), "inmemory", pushFunc, nil, "some_alias", nil)
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		prevAggrs := make(map[*aggregator]struct{})
		for _, aggr := range prev.as {
			prevAggrs[aggr] = struct{}{}
		}

		a, err := loadFromData([]byte(config), "inmemory", pushFunc, nil, "some_alias", prev)
		if err != nil {
			t.Fatalf("cannot reload aggregators: %s", err)
		}
		if a != prev {
			prev.MustStop()
		}

		reused := ""
		for _, aggr := range a.as {
			if _, ok := prevAggrs[aggr]; ok {
				reused += "1"
			} else {
				reused += "0"
			}
			select {
			case <-aggr.stopCh:
				t.Fatalf("unexpected stopped aggregator")
			default:
			}
		}
		if reused != reusedExpected {
			t.Fatalf("unexpected reused aggregators; got %s; want %s", reused, reusedExpected)
		}

		// The previous aggregators that weren't reused, must be stopped
		if a != prev {
			for aggr := range prevAggrs {
				if slices.Contains(a.as, aggr) {
					continue
				}
				select {
				case <-aggr.stopCh:
				default:
					t.Fatalf("expecting stopped aggregator")
				}
			}
		}
		a.MustStop()
	}

	f("", "", "")

	// identical configs
	f(`
- interval: 1m
  outputs: [last]
`, `
- interval: 1m
  outputs: [last]
`, "1")

	// changed config
	f(`
- interval: 1m
  outputs: [last]
`, `
- interval: 5m
  outputs: [last]
`, "0")

	// added config
	f(`
- interval: 1m
  outputs: [last]
`, `
- interval: 1m
  outputs: [last]
- interval: 1m
  outputs: [max]
`, "10")

	// removed config
	f(`
- interval: 1m
  outputs: [last]
- interval: 1m
  outputs: [max]
`, `
- interval: 1m
  outputs: [last]
`, "1")

	// changed the second config
	f(`
- interval: 1m
  outputs: [max]
- interval: 2m
  outputs: [max]
- interval: 3m
  outputs: [max]
`, `
- interval: 1m
  outputs: [max]
- interval: 2m
  outputs: [min]
- interval: 3m
  outputs: [max]
`, "101")

	// changed positions, aggregators aren't reused
	f(`
- interval: 1m
  outputs: [max]
- interval: 2m
  outputs: [max]
`, `
- interval: 2m
  outputs: [max]
- interval: 1m
  outputs: [max]
`, "00")
}

func TestAggregatorsReloadFailure(t *testing.T) {
	pushFunc := func(_ []prompb.TimeSeries) {}
	prev, err := loadFromData([]byte(`
- interval: 1m
  outputs: [max]
`), "inmemory", pushFunc, nil, "some_alias", nil)
	if err != nil {
		t.Fatalf("cannot initialize aggregators: %s", err)
	}
	a, err := loadFromData([]byte(`
- interval: 1m
  outputs: [max]
- interval: 1m
  outputs: [unknown]
`), "inmemory", pushFunc, nil, "some_alias", prev)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if a != nil {
		t.Fatalf("expecting nil a")
	}

	aggr := prev.as[0]
	select {
	case <-aggr.stopCh:
		t.Fatalf("unexpected stopped aggregator")
	default:
	}
	prev.MustStop()
	select {
	case <-aggr.stopCh:
	default:
		t.Fatalf("expecting stopped aggregator")
	}
}

func TestAggregatorsReloadKeepsState(t *testing.T) {
	var tssOutput []prompb.TimeSeries
	pushFunc := func(tss []prompb.TimeSeries) {
		tssOutput = appendClonedTimeseries(tssOutput, tss)
	}
	prevConfig := `
- interval: 1h
  flush_on_shutdown: true
  no_align_flush_to_interval: true
  outputs: [sum_samples]
`
	config := prevConfig + `
- interval: 1h
  flush_on_shutdown: true
  no_align_flush_to_interval: true
  outputs: [count_samples]
`
	prev, err := loadFromData([]byte(prevConfig), "inmemory", pushFunc, nil, "some_alias", nil)
	if err != nil {
		t.Fatalf("cannot initialize aggregators: %s", err)
	}
	prev.Push(prometheus.MustParsePromMetrics("foo 1\nfoo 2", 0), nil)

	a, err := loadFromData([]byte(config), "inmemory", pushFunc, nil, "some_alias", prev)
	if err != nil {
		t.Fatalf("cannot reload aggregators: %s", err)
	}
	prev.MustStop()
	if len(tssOutput) > 0 {
		t.Fatalf("unexpected output after stopping the previous aggregators: %s", timeSeriessToString(tssOutput))
	}

	a.Push(prometheus.MustParsePromMetrics("foo 3", 0), nil)
	a.MustStop()

	outputExpected := `foo:1h_count_samples 1
foo:1h_sum_samples 6
`
	if output := timeSeriessToString(tssOutput); output != outputExpected {
		t.Fatalf("unexpected output;\ngot\n%s\nwant\n%s", output, outputExpected)
	}
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
