package promql

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
)

func TestRollupResultCache(t *testing.T) {
	ResetRollupResultCache()
	window := int64(456)
	ec := &EvalConfig{
		Start: 1000,
		End:   2000,
		Step:  200,

		MayCache: true,
	}
	me := &metricsql.MetricExpr{
		LabelFilters: []metricsql.LabelFilter{{
			Label: "aaa",
			Value: "xxx",
		}},
	}
	fe := &metricsql.FuncExpr{
		Name: "foo",
		Args: []metricsql.Expr{me},
	}
	ae := &metricsql.AggrFuncExpr{
		Name: "foobar",
		Args: []metricsql.Expr{fe},
	}

	// Try obtaining an empty value.
	t.Run("empty", func(t *testing.T) {
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != ec.Start {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, ec.Start)
		}
		if len(tss) != 0 {
			t.Fatalf("got %d timeseries, while expecting zero", len(tss))
		}
	})

	// Store timeseries overlapping with start
	t.Run("start-overlap-no-ae", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{800, 1000, 1200},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1400 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1400)
		}
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{1, 2},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
	t.Run("start-overlap-with-ae", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{800, 1000, 1200},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, ae, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, ae, window)
		if newStart != 1400 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1400)
		}
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{1, 2},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})

	// Store timeseries overlapping with end
	t.Run("end-overlap", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{1800, 2000, 2200, 2400},
				Values:     []float64{333, 0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1000 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1000)
		}
		if len(tss) != 0 {
			t.Fatalf("got %d timeseries, while expecting zero", len(tss))
		}
	})

	// Store timeseries covered by [start ... end]
	t.Run("full-cover", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{1200, 1400, 1600},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1000 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1000)
		}
		if len(tss) != 0 {
			t.Fatalf("got %d timeseries, while expecting zero", len(tss))
		}
	})

	// Store timeseries below start
	t.Run("before-start", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{200, 400, 600},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1000 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1000)
		}
		if len(tss) != 0 {
			t.Fatalf("got %d timeseries, while expecting zero", len(tss))
		}
	})

	// Store timeseries after end
	t.Run("after-end", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{2200, 2400, 2600},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1000 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1000)
		}
		if len(tss) != 0 {
			t.Fatalf("got %d timeseries, while expecting zero", len(tss))
		}
	})

	// Store timeseries bigger than the interval [start ... end]
	t.Run("bigger-than-start-end", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{800, 1000, 1200, 1400, 1600, 1800, 2000, 2200},
				Values:     []float64{0, 1, 2, 3, 4, 5, 6, 7},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 2200 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 2200)
		}
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})

	// Store timeseries matching the interval [start ... end]
	t.Run("start-end-match", func(t *testing.T) {
		ResetRollupResultCache()
		tss := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 2200 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 2200)
		}
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})

	// Store big timeseries, so their marshaled size exceeds 64Kb.
	t.Run("big-timeseries", func(t *testing.T) {
		ResetRollupResultCache()
		var tss []*timeseries
		for i := 0; i < 1000; i++ {
			ts := &timeseries{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			}
			tss = append(tss, ts)
		}
		rollupResultCacheV.Put(ec, fe, window, tss)
		tssResult, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 2200 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 2200)
		}
		testTimeseriesEqual(t, tssResult, tss)
	})

	// Store multiple time series
	t.Run("multi-timeseries", func(t *testing.T) {
		ResetRollupResultCache()
		tss1 := []*timeseries{
			{
				Timestamps: []int64{800, 1000, 1200},
				Values:     []float64{0, 1, 2},
			},
		}
		tss2 := []*timeseries{
			{
				Timestamps: []int64{1800, 2000, 2200, 2400},
				Values:     []float64{333, 0, 1, 2},
			},
		}
		tss3 := []*timeseries{
			{
				Timestamps: []int64{1200, 1400, 1600},
				Values:     []float64{0, 1, 2},
			},
		}
		rollupResultCacheV.Put(ec, fe, window, tss1)
		rollupResultCacheV.Put(ec, fe, window, tss2)
		rollupResultCacheV.Put(ec, fe, window, tss3)
		tss, newStart := rollupResultCacheV.Get(ec, fe, window)
		if newStart != 1400 {
			t.Fatalf("unexpected newStart; got %d; want %d", newStart, 1400)
		}
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{1, 2},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})

}

func TestMergeTimeseries(t *testing.T) {
	ec := &EvalConfig{
		Start: 1000,
		End:   2000,
		Step:  200,
	}
	bStart := int64(1400)

	t.Run("bStart=ec.Start", func(t *testing.T) {
		a := []*timeseries{}
		b := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			},
		}
		tss := mergeTimeseries(a, b, 1000, ec)
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{1, 2, 3, 4, 5, 6},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
	t.Run("a-empty", func(t *testing.T) {
		a := []*timeseries{}
		b := []*timeseries{
			{
				Timestamps: []int64{1400, 1600, 1800, 2000},
				Values:     []float64{3, 4, 5, 6},
			},
		}
		tss := mergeTimeseries(a, b, bStart, ec)
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{nan, nan, 3, 4, 5, 6},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
	t.Run("b-empty", func(t *testing.T) {
		a := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{2, 1},
			},
		}
		b := []*timeseries{}
		tss := mergeTimeseries(a, b, bStart, ec)
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{2, 1, nan, nan, nan, nan},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
	t.Run("non-empty", func(t *testing.T) {
		a := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{2, 1},
			},
		}
		b := []*timeseries{
			{
				Timestamps: []int64{1400, 1600, 1800, 2000},
				Values:     []float64{3, 4, 5, 6},
			},
		}
		tss := mergeTimeseries(a, b, bStart, ec)
		tssExpected := []*timeseries{
			{
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{2, 1, 3, 4, 5, 6},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
	t.Run("non-empty-distinct-metric-names", func(t *testing.T) {
		a := []*timeseries{
			{
				Timestamps: []int64{1000, 1200},
				Values:     []float64{2, 1},
			},
		}
		a[0].MetricName.MetricGroup = []byte("bar")
		b := []*timeseries{
			{
				Timestamps: []int64{1400, 1600, 1800, 2000},
				Values:     []float64{3, 4, 5, 6},
			},
		}
		b[0].MetricName.MetricGroup = []byte("foo")
		tss := mergeTimeseries(a, b, bStart, ec)
		tssExpected := []*timeseries{
			{
				MetricName: storage.MetricName{
					MetricGroup: []byte("foo"),
				},
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{nan, nan, 3, 4, 5, 6},
			},
			{
				MetricName: storage.MetricName{
					MetricGroup: []byte("bar"),
				},
				Timestamps: []int64{1000, 1200, 1400, 1600, 1800, 2000},
				Values:     []float64{2, 1, nan, nan, nan, nan},
			},
		}
		testTimeseriesEqual(t, tss, tssExpected)
	})
}

func testTimeseriesEqual(t *testing.T, tss, tssExpected []*timeseries) {
	t.Helper()
	if len(tss) != len(tssExpected) {
		t.Fatalf(`unexpected timeseries count; got %d; want %d`, len(tss), len(tssExpected))
	}
	for i, ts := range tss {
		tsExpected := tssExpected[i]
		testMetricNamesEqual(t, &ts.MetricName, &tsExpected.MetricName, i)
		testRowsEqual(t, ts.Values, ts.Timestamps, tsExpected.Values, tsExpected.Timestamps)
	}
}
