package promql

import (
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
)

func TestGetCommonLabelFilters(t *testing.T) {
	f := func(metrics string, lfsExpected string) {
		t.Helper()
		var tss []*timeseries
		var rows prometheus.Rows
		rows.UnmarshalWithErrLogger(metrics, func(errStr string) {
			t.Fatalf("unexpected error when parsing %s: %s", metrics, errStr)
		})
		for _, row := range rows.Rows {
			var tags []storage.Tag
			for _, tag := range row.Tags {
				tags = append(tags, storage.Tag{
					Key:   []byte(tag.Key),
					Value: []byte(tag.Value),
				})
			}
			var ts timeseries
			ts.MetricName.Tags = tags
			tss = append(tss, &ts)
		}
		lfs := getCommonLabelFilters(tss)
		var me metricsql.MetricExpr
		if len(lfs) > 0 {
			me.LabelFilterss = [][]metricsql.LabelFilter{lfs}
		}
		lfsMarshaled := me.AppendString(nil)
		if string(lfsMarshaled) != lfsExpected {
			t.Fatalf("unexpected common label filters;\ngot\n%s\nwant\n%s", lfsMarshaled, lfsExpected)
		}
	}
	f(``, `{}`)
	f(`m 1`, `{}`)
	f(`m{a="b"} 1`, `{a="b"}`)
	f(`m{c="d",a="b"} 1`, `{a="b",c="d"}`)
	f(`m1{a="foo"} 1
m2{a="bar"} 1`, `{a=~"bar|foo"}`)
	f(`m1{a="foo"} 1
m2{b="bar"} 1`, `{}`)
	f(`m1{a="foo",b="bar"} 1
m2{b="bar",c="x"} 1`, `{b="bar"}`)
}

func TestValidateMaxPointsPerSeriesFailure(t *testing.T) {
	f := func(start, end, step int64, maxPoints int) {
		t.Helper()
		if err := ValidateMaxPointsPerSeries(start, end, step, maxPoints); err == nil {
			t.Fatalf("expecting non-nil error for ValidateMaxPointsPerSeries(start=%d, end=%d, step=%d, maxPoints=%d)", start, end, step, maxPoints)
		}
	}
	// zero step
	f(0, 0, 0, 0)
	f(0, 0, 0, 1)
	// the maxPoints is smaller than the generated points
	f(0, 1, 1, 0)
	f(0, 1, 1, 1)
	f(1659962171908, 1659966077742, 5000, 700)
}

func TestValidateMaxPointsPerSeriesSuccess(t *testing.T) {
	f := func(start, end, step int64, maxPoints int) {
		t.Helper()
		if err := ValidateMaxPointsPerSeries(start, end, step, maxPoints); err != nil {
			t.Fatalf("unexpected error in ValidateMaxPointsPerSeries(start=%d, end=%d, step=%d, maxPoints=%d): %s", start, end, step, maxPoints, err)
		}
	}
	f(1, 1, 1, 2)
	f(1659962171908, 1659966077742, 5000, 800)
	f(1659962150000, 1659966070000, 10000, 393)
}

func TestQueryStats_addSeriesFetched(t *testing.T) {
	qs := &QueryStats{}
	ec := &EvalConfig{
		QueryStats: qs,
	}
	ec.QueryStats.addSeriesFetched(1)

	if n := qs.SeriesFetched.Load(); n != 1 {
		t.Fatalf("expected to get 1; got %d instead", n)
	}

	ecNew := copyEvalConfig(ec)
	ecNew.QueryStats.addSeriesFetched(3)
	if n := qs.SeriesFetched.Load(); n != 4 {
		t.Fatalf("expected to get 4; got %d instead", n)
	}
}

func TestQueryStats_DataFetchDuration(t *testing.T) {
	qs := &QueryStats{}
	// nil safety
	var qsNil *QueryStats
	qsNil.addDataFetchDuration(5 * 1000000)
	if qsNil.getDataFetchDuration() != 0 {
		t.Fatalf("expected 0 for nil QueryStats; got %d", qsNil.getDataFetchDuration())
	}
	if n := qs.DataFetchDuration.Load(); n != 0 {
		t.Fatalf("expected initial 0; got %d", n)
	}
	qs.addDataFetchDuration(10 * 1000000)
	if d := qs.getDataFetchDuration(); d != 10*1000000 {
		t.Fatalf("expected 10ms; got %v", d)
	}
	qs.addDataFetchDuration(5 * 1000000)
	if d := qs.getDataFetchDuration(); d != 15*1000000 {
		t.Fatalf("expected 15ms after second add; got %v", d)
	}
	if ms := qs.getDataFetchDuration().Milliseconds(); ms != 15 {
		t.Fatalf("expected 15 milliseconds; got %d", ms)
	}
	// Verify DataFetchDuration does not interfere with ExecutionDuration
	qs2 := &QueryStats{}
	qs2.addDataFetchDuration(20 * 1000000)
	qs2.addExecutionTimeMsec(time.Now().Add(-10 * 1000000))
	if ed := qs2.ExecutionDuration.Load(); ed == nil {
		t.Fatalf("expected ExecutionDuration to be set")
	}
	if d := qs2.getDataFetchDuration(); d != 20*1000000 {
		t.Fatalf("expected data fetch 20ms; got %v", d)
	}
	// Concurrent adds
	qs3 := &QueryStats{}
	var wg = make(chan struct{})
	go func() {
		for range 100 {
			qs3.addDataFetchDuration(1000000)
		}
		close(wg)
	}()
	for range 100 {
		qs3.addDataFetchDuration(1000000)
	}
	<-wg
	if d := qs3.getDataFetchDuration(); d != 200*1000000 {
		t.Fatalf("expected 200ms after concurrent adds; got %v", d)
	}
}

func TestQueryStats_ExecutionDurationPreserved(t *testing.T) {
	qs := &QueryStats{}
	start := time.Now().Add(-50 * 1000000)
	qs.addExecutionTimeMsec(start)
	if ed := qs.ExecutionDuration.Load(); ed == nil {
		t.Fatalf("expected ExecutionDuration to be set")
	} else {
		ms := ed.Milliseconds()
		if ms < 50 || ms > 100 {
			t.Fatalf("expected execution duration ~50ms; got %d", ms)
		}
	}
	// Ensure DataFetchDuration is independent and initially 0
	if d := qs.getDataFetchDuration(); d != 0 {
		t.Fatalf("expected data fetch 0; got %v", d)
	}
	// Ensure adding data fetch doesn't affect execution duration
	qs.addDataFetchDuration(10 * 1000000)
	if ed := qs.ExecutionDuration.Load(); ed == nil {
		t.Fatalf("expected ExecutionDuration still set")
	}
}

func TestQueryStats_NilSafety(t *testing.T) {
	var qs *QueryStats
	// Should not panic
	qs.addSeriesFetched(1)
	qs.addExecutionTimeMsec(time.Now())
	qs.addDataFetchDuration(1000000)
	qs.addMemoryUsage(100)
	if qs.memoryUsage() != 0 {
		t.Fatalf("expected 0 for nil memoryUsage")
	}
	if qs.getDataFetchDuration() != 0 {
		t.Fatalf("expected 0 for nil dataFetchDuration")
	}
	qs.maybeLogQueryStats(time.Now())
}

func TestGetSumInstantValues(t *testing.T) {
	f := func(cached, start, end []*timeseries, timestamp int64, expectedResult []*timeseries) {
		t.Helper()

		result := getSumInstantValues(nil, cached, start, end, timestamp)
		if !reflect.DeepEqual(result, expectedResult) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, expectedResult)
		}
	}
	ts := func(name string, timestamp int64, value float64) *timeseries {
		return &timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte(name),
			},
			Timestamps: []int64{timestamp},
			Values:     []float64{value},
		}
	}

	// start - end + cached = 1
	f(
		nil,
		[]*timeseries{ts("foo", 42, 1)},
		nil,
		100,
		[]*timeseries{ts("foo", 100, 1)},
	)

	// start - end + cached = 0
	f(
		nil,
		[]*timeseries{ts("foo", 100, 1)},
		[]*timeseries{ts("foo", 10, 1)},
		100,
		[]*timeseries{ts("foo", 100, 0)},
	)

	// start - end + cached = 2
	f(
		[]*timeseries{ts("foo", 10, 1)},
		[]*timeseries{ts("foo", 100, 1)},
		nil,
		100,
		[]*timeseries{ts("foo", 100, 2)},
	)

	// start - end + cached = 1
	f(
		[]*timeseries{ts("foo", 50, 1)},
		[]*timeseries{ts("foo", 100, 1)},
		[]*timeseries{ts("foo", 10, 1)},
		100,
		[]*timeseries{ts("foo", 100, 1)},
	)

	// start - end + cached = 0
	f(
		[]*timeseries{ts("foo", 50, 1)},
		nil,
		[]*timeseries{ts("foo", 10, 1)},
		100,
		[]*timeseries{ts("foo", 100, 0)},
	)

	// start - end + cached = 1
	f(
		[]*timeseries{ts("foo", 50, 1)},
		nil,
		nil,
		100,
		[]*timeseries{ts("foo", 100, 1)},
	)
}

func TestShouldOptimizeRepeatedBinaryOpSubexprsGate(t *testing.T) {
	e, err := metricsql.Parse(`count(count(vm_requests_total) by (action,addr,cluster,endpoint)) by (action,addr,cluster) / count(count(vm_requests_total) by (action,addr,cluster,endpoint))`)
	if err != nil {
		t.Fatalf("unexpected error in metricsql.Parse(): %s", err)
	}
	be, ok := e.(*metricsql.BinaryOpExpr)
	if !ok {
		t.Fatalf("unexpected expr type; got %T; want *metricsql.BinaryOpExpr", e)
	}

	f := func(name string, ec *EvalConfig, resultExpected bool) {
		t.Helper()
		result := shouldOptimizeRepeatedBinaryOpSubexprs(ec, be.Left, be.Right)
		if result != resultExpected {
			t.Fatalf("unexpected result for %q; got %v; want %v", name, result, resultExpected)
		}
	}

	f("disabled optimization", &EvalConfig{
		Start: 1000,
		End:   2000,
		Step:  1000,
	}, false)
	f("disabled cache", &EvalConfig{
		Start:                            1000,
		End:                              2000,
		Step:                             1000,
		OptimizeRepeatedBinaryOpSubexprs: true,
	}, false)
	f("instant query", &EvalConfig{
		Start:                            1000,
		End:                              1000,
		Step:                             1000,
		MayCache:                         true,
		OptimizeRepeatedBinaryOpSubexprs: true,
	}, false)
	f("repeated cacheable aggregate subexpression", &EvalConfig{
		Start:                            1000,
		End:                              2000,
		Step:                             1000,
		MayCache:                         true,
		OptimizeRepeatedBinaryOpSubexprs: true,
	}, true)
	f("unaligned range query", &EvalConfig{
		Start:                            1001,
		End:                              2000,
		Step:                             1000,
		MayCache:                         true,
		OptimizeRepeatedBinaryOpSubexprs: true,
	}, false)
}

func TestShouldOptimizeRepeatedBinaryOpSubexprsExpressions(t *testing.T) {
	f := func(name, q string, resultExpected bool) {
		t.Helper()
		e, err := metricsql.Parse(q)
		if err != nil {
			t.Fatalf("unexpected error in metricsql.Parse(%q) for %q: %s", q, name, err)
		}
		be, ok := e.(*metricsql.BinaryOpExpr)
		if !ok {
			t.Fatalf("unexpected expr type for %q; got %T; want *metricsql.BinaryOpExpr", name, e)
		}
		ec := &EvalConfig{Start: 1000, End: 2000, Step: 1000, MayCache: true, OptimizeRepeatedBinaryOpSubexprs: true}
		result := shouldOptimizeRepeatedBinaryOpSubexprs(ec, be.Left, be.Right)
		if result != resultExpected {
			t.Fatalf("unexpected result for %q; got %v; want %v; query: %q", name, result, resultExpected, q)
		}
	}

	f("original issue query", `count(count(vm_requests_total) by (action,addr,cluster,endpoint)) by (action,addr,cluster) / count(count(vm_requests_total) by (action,addr,cluster,endpoint))`, true)
	f("right side contains repeated count aggregate", `count(foo) by (job) / (count(foo) by (job) + 1)`, true)
	f("same sum aggregate", `sum(rate(foo[5m])) by (job) / sum(rate(foo[5m])) by (job)`, true)
	f("same inner rollup but different aggregates", `sum(rate(foo[5m])) by (job) / count(rate(foo[5m])) by (job)`, false)
	f("different count aggregates", `count(foo) by (job) / count(bar) by (job)`, false)
	f("bare metric selector", `foo / foo`, false)
	f("bare rollup function", `rate(a[5m]) / rate(a[5m])`, false)
	f("now at modifier", `sum(rate(foo[5m] @ now())) by (job) / sum(rate(foo[5m] @ now())) by (job)`, false)
	f("unseeded rand at modifier", `sum(rate(foo[5m] @ rand())) by (job) / sum(rate(foo[5m] @ rand())) by (job)`, false)
	f("unseeded rand_normal at modifier", `sum(rate(foo[5m] @ rand_normal())) by (job) / sum(rate(foo[5m] @ rand_normal())) by (job)`, false)
	f("unseeded rand_exponential at modifier", `sum(rate(foo[5m] @ rand_exponential())) by (job) / sum(rate(foo[5m] @ rand_exponential())) by (job)`, false)
	f("seeded rand at modifier", `sum(rate(foo[5m] @ rand(1))) by (job) / sum(rate(foo[5m] @ rand(1))) by (job)`, true)
}
