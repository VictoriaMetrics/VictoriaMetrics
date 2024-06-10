package promql

import (
	"reflect"
	"testing"

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
			t.Fatalf("expecint non-nil error for ValidateMaxPointsPerSeries(start=%d, end=%d, step=%d, maxPoints=%d)", start, end, step, maxPoints)
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

func TestGetSumInstantValues(t *testing.T) {
	f := func(cached, start, end []*timeseries, timestamp int64, expectedResult []*timeseries) {
		t.Helper()

		result := getSumInstantValues(nil, cached, start, end, timestamp)
		if !reflect.DeepEqual(result, expectedResult) {
			t.Errorf("unexpected result; got\n%v\nwant\n%v", result, expectedResult)
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
