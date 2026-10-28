package netstorage

import (
	"reflect"
	"testing"
)

func TestMergeResult(t *testing.T) {
	f := func(dstTimestamps []int64, patchTimestamps []int64, expectedTimestamps []int64, expectedValues []float64) {
		t.Helper()
		dst := &Result{
			Timestamps: dstTimestamps,
			Values:     make([]float64, len(dstTimestamps)),
		}
		patch := &Result{
			Timestamps: patchTimestamps,
			Values:     make([]float64, len(patchTimestamps)),
		}
		for i := range patch.Values {
			patch.Values[i] = 1
		}
		mergeResult(dst, patch)
		if !reflect.DeepEqual(dst.Timestamps, expectedTimestamps) {
			t.Fatalf("unexpected timestamps; got %v; want %v", dst.Timestamps, expectedTimestamps)
		}
		if !reflect.DeepEqual(dst.Values, expectedValues) {
			t.Fatalf("unexpected values; got %v; want %v", dst.Values, expectedValues)
		}
	}

	// empty patch
	f([]int64{1, 2}, nil, []int64{1, 2}, []float64{0, 0})

	// empty origin
	f(nil, []int64{1, 2}, []int64{1, 2}, []float64{1, 1})

	// patch before origin
	f([]int64{5, 6}, []int64{1, 2}, []int64{1, 2, 5, 6}, []float64{1, 1, 0, 0})

	// patch after origin
	f([]int64{1, 2}, []int64{5, 6}, []int64{1, 2, 5, 6}, []float64{0, 0, 1, 1})

	// patch touches origin edges
	f([]int64{1, 2, 3}, []int64{3, 4}, []int64{1, 2, 3, 4}, []float64{0, 0, 1, 1})
	f([]int64{3, 4, 5}, []int64{1, 3}, []int64{1, 3, 4, 5}, []float64{1, 1, 0, 0})

	// patch inside origin replaces the whole covered range
	f([]int64{1, 2, 3, 4, 5, 6}, []int64{2, 5}, []int64{1, 2, 5, 6}, []float64{0, 1, 1, 0})
	f([]int64{1, 3, 5}, []int64{2, 4}, []int64{1, 2, 4, 5}, []float64{0, 1, 1, 0})

	// patch covers origin
	f([]int64{2, 3}, []int64{1, 4}, []int64{1, 4}, []float64{1, 1})
}
