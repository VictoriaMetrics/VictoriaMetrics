package netstorage

import (
	"reflect"
	"testing"
)

func TestMergeSortBlocks(t *testing.T) {
	f := func(blocks []*sortBlock, dedupInterval int64, expectedResult *Result) {
		t.Helper()
		var result Result
		sbh := getSortBlocksHeap()
		sbh.sbs = append(sbh.sbs[:0], blocks...)
		mergeSortBlocks(&result, sbh, dedupInterval)
		putSortBlocksHeap(sbh)
		if !reflect.DeepEqual(result.Values, expectedResult.Values) {
			t.Fatalf("unexpected values;\ngot\n%v\nwant\n%v", result.Values, expectedResult.Values)
		}
		if !reflect.DeepEqual(result.Timestamps, expectedResult.Timestamps) {
			t.Fatalf("unexpected timestamps;\ngot\n%v\nwant\n%v", result.Timestamps, expectedResult.Timestamps)
		}
	}

	// Zero blocks
	f(nil, 1, &Result{})

	// Single block without samples
	f([]*sortBlock{{}}, 1, &Result{})

	// Single block with a single samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1},
			Values:     []float64{4.2},
		},
	}, 1, &Result{
		Timestamps: []int64{1},
		Values:     []float64{4.2},
	})

	// Single block with multiple samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3},
		Values:     []float64{4.2, 2.1, 10},
	})

	// Single block with multiple samples with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 2, &Result{
		Timestamps: []int64{2, 3},
		Values:     []float64{2.1, 10},
	})

	// Multiple blocks without time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2},
			Values:     []float64{4.2, 2.1},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 5},
		Values:     []float64{4.2, 2.1, 5.2, 6.1},
	})

	// Multiple blocks with time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 4, 5},
		Values:     []float64{4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with time range inclusion.
	f([]*sortBlock{
		{
			Timestamps: []int64{0, 3, 5},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{0, 1, 2, 3, 4, 5},
		Values:     []float64{9, 4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with identical timestamps and identical values.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4, 5},
			Values:     []float64{9, 5.2, 6.1, 9},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5},
		Values:     []float64{9, 5.2, 6.1, 9},
	})

	// Multiple blocks with identical timestamps.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4, 5},
			Values:     []float64{9, 5.2, 6.1, 9},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5},
		Values:     []float64{9, 5.2, 42, 9},
	})
	// Multiple blocks with identical timestamps, disabled deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 4},
		Values:     []float64{9, 4.2, 2.1, 5.2, 6.1, 42},
	})

	// Multiple blocks with identical timestamp ranges.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5, 10, 11, 12},
		Values:     []float64{21, 22, 23, 7, 24, 25, 26},
	})

	// Multiple blocks with identical timestamp ranges, no deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 5, 10, 10, 11, 11, 12},
		Values:     []float64{9, 21, 22, 8, 23, 7, 6, 24, 25, 5, 26},
	})

	// Multiple blocks with identical timestamp ranges with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 5, &Result{
		Timestamps: []int64{5, 10, 12},
		Values:     []float64{7, 24, 26},
	})
}
