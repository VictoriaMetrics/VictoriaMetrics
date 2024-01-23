package promql

import (
	"math"
	"sort"
	"testing"
)

func TestSortWithNaNs(t *testing.T) {
	f := func(a []float64, ascExpected, descExpected []float64) {
		t.Helper()

		equalSlices := func(a, b []float64) bool {
			for i := range a {
				x := a[i]
				y := b[i]
				if math.IsNaN(x) {
					return math.IsNaN(y)
				}
				if math.IsNaN(y) {
					return false
				}
				if x != y {
					return false
				}
			}
			return true
		}

		aCopy := append([]float64{}, a...)
		sort.Slice(aCopy, func(i, j int) bool {
			return lessWithNaNs(aCopy[i], aCopy[j])
		})
		if !equalSlices(aCopy, ascExpected) {
			t.Fatalf("unexpected slice after asc sorting; got\n%v\nwant\n%v", aCopy, ascExpected)
		}

		aCopy = append(aCopy[:0], a...)
		sort.Slice(aCopy, func(i, j int) bool {
			return greaterWithNaNs(aCopy[i], aCopy[j])
		})
		if !equalSlices(aCopy, descExpected) {
			t.Fatalf("unexpected slice after desc sorting; got\n%v\nwant\n%v", aCopy, descExpected)
		}
	}

	f(nil, nil, nil)
	f([]float64{1}, []float64{1}, []float64{1})
	f([]float64{1, nan, 3, 2}, []float64{nan, 1, 2, 3}, []float64{nan, 3, 2, 1})
	f([]float64{nan}, []float64{nan}, []float64{nan})
	f([]float64{nan, nan, nan}, []float64{nan, nan, nan}, []float64{nan, nan, nan})
	f([]float64{nan, 1, nan}, []float64{nan, nan, 1}, []float64{nan, nan, 1})
	f([]float64{nan, 1, 0, 2, nan}, []float64{nan, nan, 0, 1, 2}, []float64{nan, nan, 2, 1, 0})
}

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
