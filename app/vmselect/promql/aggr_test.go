package promql

import (
	"math"
	"testing"
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
