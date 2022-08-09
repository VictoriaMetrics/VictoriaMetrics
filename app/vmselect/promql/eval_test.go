package promql

import (
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
		me := &metricsql.MetricExpr{
			LabelFilters: lfs,
		}
		lfsMarshaled := me.AppendString(nil)
		if string(lfsMarshaled) != lfsExpected {
			t.Fatalf("unexpected common label filters;\ngot\n%s\nwant\n%s", lfsMarshaled, lfsExpected)
		}
	}
	f(``, `{}`)
	f(`m 1`, `{}`)
	f(`m{a="b"} 1`, `{a="b"}`)
	f(`m{c="d",a="b"} 1`, `{a="b", c="d"}`)
	f(`m1{a="foo"} 1
m2{a="bar"} 1`, `{a=~"bar|foo"}`)
	f(`m1{a="foo"} 1
m2{b="bar"} 1`, `{}`)
	f(`m1{a="foo",b="bar"} 1
m2{b="bar",c="x"} 1`, `{b="bar"}`)
}

func Test_validateMaxPointsPerTimeseriesFailed(t *testing.T) {
	f := func(name string, start, end, step int64, limiter int) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			err := validateMaxPointsPerTimeseries(start, end, step, limiter)
			if err == nil {
				t.Fatal("should be non-nil error")
			}
		})
	}
	f("all zeroes", 0, 0, 0, 0)
	f("calculated points more than expected limiter", 0, 1, 1, 0)
	f("calculated points more than expected limiter but limiter not zero", 0, 1, 1, 1)
	f("calculated points equal to 782 (higher than limiter)", 1659962171908, 1659966077742, 5000, 700)
}

func Test_validateMaxPointsPerTimeseriesSuccess(t *testing.T) {
	f := func(name string, start, end, step int64, limiter int) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			err := validateMaxPointsPerTimeseries(start, end, step, limiter)
			if err != nil {
				t.Fatal("should be nil error")
			}
		})
	}
	f("limiter bigger than calculated points", 1, 1, 1, 2)
	f("calculated point equal to 782 (lower than limiter)", 1659962171908, 1659966077742, 5000, 800)
	f("calculated point equal to 10000 (equal to limiter)", 1659962150000, 1659966070000, 10000, 393)
}
