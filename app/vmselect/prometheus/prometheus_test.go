package prometheus

import (
	"math"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestRemoveEmptyValuesAndTimeseries(t *testing.T) {
	f := func(tss []netstorage.Result, tssExpected []netstorage.Result) {
		t.Helper()
		tss = removeEmptyValuesAndTimeseries(tss)
		if !reflect.DeepEqual(tss, tssExpected) {
			t.Fatalf("unexpected result; got %v; want %v", tss, tssExpected)
		}
	}

	f(nil, nil)
	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, 3},
		},
		{
			Timestamps: []int64{100, 200, 300, 400},
			Values:     []float64{nan, nan, 3, nan},
		},
		{
			Timestamps: []int64{1, 2},
			Values:     []float64{nan, nan},
		},
		{
			Timestamps: nil,
			Values:     nil,
		},
	}, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, 3},
		},
		{
			Timestamps: []int64{300},
			Values:     []float64{3},
		},
	})
}

func TestAdjustLastPoints(t *testing.T) {
	f := func(tss []netstorage.Result, start, end int64, tssExpected []netstorage.Result) {
		t.Helper()
		tss = adjustLastPoints(tss, start, end)
		for i, ts := range tss {
			for j, value := range ts.Values {
				expectedValue := tssExpected[i].Values[j]
				if math.IsNaN(expectedValue) {
					if !math.IsNaN(value) {
						t.Fatalf("unexpected value for time series #%d at position %d; got %v; want nan", i, j, value)
					}
				} else if expectedValue != value {
					t.Fatalf("unexpected value for time series #%d at position %d; got %v; want %v", i, j, value, expectedValue)
				}
			}
			if !reflect.DeepEqual(ts.Timestamps, tssExpected[i].Timestamps) {
				t.Fatalf("unexpected timestamps for time series #%d; got %v; want %v", i, tss, tssExpected)
			}
		}

	}

	nan := math.NaN()

	f(nil, 300, 500, nil)

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 4, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
	}, 400, 500, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 4, 4},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
	})

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, nan, nan, nan},
		},
	}, 300, 500, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 3, 3},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, nan, nan, nan},
		},
	})

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, nan, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, nan, nan, nan, nan},
		},
	}, 500, 300, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, nan, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, nan, nan, nan, nan},
		},
	})

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 4, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400},
			Values:     []float64{1, 2, 3, 4},
		},
	}, 400, 500, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 4, 4},
		},
		{
			Timestamps: []int64{100, 200, 300, 400},
			Values:     []float64{1, 2, 3, 4},
		},
	})

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, nan},
		},
	}, 300, 600, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 3, 3},
		},
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, nan},
		},
	})

	// Check for timestamps outside the configured time range.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/625
	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, 45},
		},
	}, 250, 400, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, 2},
		},
	})
}

// helper for tests
func tfFromKV(k, v string) storage.TagFilter {
	return storage.TagFilter{
		Key:   []byte(k),
		Value: []byte(v),
	}
}

func Test_addEnforcedFiltersToTagFilterss(t *testing.T) {
	f := func(t *testing.T, dstTfss [][]storage.TagFilter, enforcedFilters []storage.TagFilter, want [][]storage.TagFilter) {
		t.Helper()
		got := addEnforcedFiltersToTagFilterss(dstTfss, enforcedFilters)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unxpected result for addEnforcedFiltersToTagFilterss, \ngot: %v,\n want: %v", want, got)
		}
	}
	f(t, [][]storage.TagFilter{{tfFromKV("label", "value")}},
		nil,
		[][]storage.TagFilter{{tfFromKV("label", "value")}})

	f(t, nil,
		[]storage.TagFilter{tfFromKV("ext-label", "ext-value")},
		[][]storage.TagFilter{{tfFromKV("ext-label", "ext-value")}})

	f(t, [][]storage.TagFilter{
		{tfFromKV("l1", "v1")},
		{tfFromKV("l2", "v2")},
	},
		[]storage.TagFilter{tfFromKV("ext-l1", "v2")},
		[][]storage.TagFilter{
			{tfFromKV("l1", "v1"), tfFromKV("ext-l1", "v2")},
			{tfFromKV("l2", "v2"), tfFromKV("ext-l1", "v2")},
		})
}
