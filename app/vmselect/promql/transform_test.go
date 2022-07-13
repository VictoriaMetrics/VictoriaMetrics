package promql

import (
	"reflect"
	"testing"
)

func TestFixBrokenBuckets(t *testing.T) {
	f := func(values, expectedResult []float64) {
		t.Helper()
		xss := make([]leTimeseries, len(values))
		for i, v := range values {
			xss[i].ts = &timeseries{
				Values: []float64{v},
			}
		}
		fixBrokenBuckets(0, xss)
		result := make([]float64, len(values))
		for i, xs := range xss {
			result[i] = xs.ts.Values[0]
		}
		if !reflect.DeepEqual(result, expectedResult) {
			t.Fatalf("unexpected result for values=%v\ngot\n%v\nwant\n%v", values, result, expectedResult)
		}
	}
	f(nil, []float64{})
	f([]float64{1}, []float64{1})
	f([]float64{1, 2}, []float64{1, 2})
	f([]float64{2, 1}, []float64{1, 1})
	f([]float64{1, 2, 3, nan, nan}, []float64{1, 2, 3, 3, 3})
	f([]float64{5, 1, 2, 3, nan}, []float64{1, 1, 2, 3, 3})
	f([]float64{1, 5, 2, nan, 6, 3}, []float64{1, 2, 2, 3, 3, 3})
	f([]float64{5, 10, 4, 3}, []float64{3, 3, 3, 3})
}
