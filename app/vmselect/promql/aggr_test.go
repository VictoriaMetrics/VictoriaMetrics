package promql

import (
	"log"
	"math"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func TestModeNoNaNs(t *testing.T) {
	f := func(prevValue float64, a []float64, expectedResult float64) {
		t.Helper()
		result := modeNoNaNs(prevValue, a)
		if math.IsNaN(result) {
			if !math.IsNaN(expectedResult) {
				t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
			}
			return
		}
		if result != expectedResult {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
		}
	}
	f(nan, nil, nan)
	f(nan, []float64{123}, 123)
	f(nan, []float64{1, 2, 3}, 1)
	f(nan, []float64{1, 2, 2}, 2)
	f(nan, []float64{1, 1, 2}, 1)
	f(nan, []float64{1, 1, 1}, 1)
	f(nan, []float64{1, 2, 2, 3}, 2)
	f(nan, []float64{1, 1, 2, 2, 3, 3, 3}, 3)
	f(1, []float64{2, 3, 4, 5}, 1)
	f(1, []float64{2, 2}, 2)
	f(1, []float64{2, 3, 3}, 3)
	f(1, []float64{2, 4, 3, 4, 3, 4}, 4)
	f(1, []float64{2, 3, 3, 4, 4}, 3)
	f(1, []float64{4, 3, 2, 3, 4}, 3)
}

func TestLessWithNaNs(t *testing.T) {
	f := func(a, b float64, expectedResult bool) {
		t.Helper()
		result := lessWithNaNs(a, b)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
		}
	}
	f(nan, nan, false)
	f(nan, 1, true)
	f(1, nan, false)
	f(1, 2, true)
	f(2, 1, false)
	f(1, 1, false)
}

func TestLessWithNaNsReversed(t *testing.T) {
	f := func(a, b float64, expectedResult bool) {
		t.Helper()
		result := lessWithNaNsReversed(a, b)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedResult)
		}
	}
	f(nan, nan, true)
	f(nan, 1, true)
	f(1, nan, false)
	f(1, 2, false)
	f(2, 1, true)
	f(1, 1, false)
}

func TestTopK(t *testing.T) {
	f := func(all [][]*timeseries, expected []*timeseries, k int, reversed bool) {
		t.Helper()
		topKFunc := newAggrFuncTopK(reversed)
		actual, err := topKFunc(&aggrFuncArg{
			args: all,
			ae: &metricsql.AggrFuncExpr{
				Limit:    1,
				Modifier: metricsql.ModifierExpr{},
			},
			ec: nil,
		})
		if err != nil {
			log.Fatalf("failed to call topK, err=%v", err)
		}
		for i := range actual {
			if !eq(expected[i], actual[i]) {
				t.Fatalf("unexpected result: i:%v got:\n%v; want:\t%v", i, actual[i], expected[i])
			}
		}
	}

	f(newTestSeries(), []*timeseries{
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{nan, nan, 3, 2, 1},
		},
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{1, 2, 3, 4, 5},
		},
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{2, 3, nan, nan, nan},
		},
	}, 2, true)
	f(newTestSeries(), []*timeseries{
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{3, 4, 5, 6, 7},
		},
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{nan, nan, 4, 5, 6},
		},
		{
			Timestamps: []int64{1, 2, 3, 4, 5},
			Values:     []float64{5, 4, nan, nan, nan},
		},
	}, 2, false)
	f(newTestSeriesWithNaNsWithoutOverlap(), []*timeseries{
		{
			Values:     []float64{nan, nan, nan, 2, 1},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{nan, nan, 5, 6, 7},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{2, 3, 4, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{1, 2, nan, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
	}, 2, true)
	f(newTestSeriesWithNaNsWithoutOverlap(), []*timeseries{
		{
			Values:     []float64{nan, nan, 5, 6, 7},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{nan, nan, 6, 2, 1},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{2, 3, nan, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{1, 2, nan, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
	}, 2, false)
	f(newTestSeriesWithNaNsWithOverlap(), []*timeseries{
		{
			Values:     []float64{nan, nan, nan, 2, 1},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{nan, nan, nan, 6, 7},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{1, 2, 3, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{2, 3, 4, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
	}, 2, true)
	f(newTestSeriesWithNaNsWithOverlap(), []*timeseries{
		{
			Values:     []float64{nan, nan, 5, 6, 7},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{nan, nan, 6, 2, 1},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{2, 3, nan, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
		{
			Values:     []float64{1, 2, nan, nan, nan},
			Timestamps: []int64{1, 2, 3, 4, 5},
		},
	}, 2, false)
}

func newTestSeries() [][]*timeseries {
	return [][]*timeseries{
		{
			{
				Values:     []float64{2, 2, 2, 2, 2},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
		{
			{
				Values:     []float64{1, 2, 3, 4, 5},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{2, 3, 4, 5, 6},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{5, 4, 3, 2, 1},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{3, 4, 5, 6, 7},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
	}
}

func newTestSeriesWithNaNsWithoutOverlap() [][]*timeseries {
	return [][]*timeseries{
		{
			{
				Values:     []float64{2, 2, 2, 2, 2},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
		{
			{
				Values:     []float64{1, 2, nan, nan, nan},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{2, 3, 4, nan, nan},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{nan, nan, 6, 2, 1},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{nan, nan, 5, 6, 7},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
	}
}

func newTestSeriesWithNaNsWithOverlap() [][]*timeseries {
	return [][]*timeseries{
		{
			{
				Values:     []float64{2, 2, 2, 2, 2},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
		{
			{
				Values:     []float64{1, 2, 3, nan, nan},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{2, 3, 4, nan, nan},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{nan, nan, 6, 2, 1},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
			{
				Values:     []float64{nan, nan, 5, 6, 7},
				Timestamps: []int64{1, 2, 3, 4, 5},
			},
		},
	}
}

func eq(a, b *timeseries) bool {
	if !reflect.DeepEqual(a.Timestamps, b.Timestamps) {
		return false
	}
	for i := range a.Values {
		if !eqWithNan(a.Values[i], b.Values[i]) {
			return false
		}
	}
	return true
}

func eqWithNan(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	return a == b
}
