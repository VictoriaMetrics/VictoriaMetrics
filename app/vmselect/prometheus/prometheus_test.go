package prometheus

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
)

func TestRemoveNaNValuesInplace(t *testing.T) {
	f := func(tss []netstorage.Result, tssExpected []netstorage.Result) {
		t.Helper()
		removeNaNValuesInplace(tss)
		if !reflect.DeepEqual(tss, tssExpected) {
			t.Fatalf("unexpected result; got %v; want %v", tss, tssExpected)
		}
	}

	nan := math.NaN()

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

func TestGetTimeSuccess(t *testing.T) {
	f := func(s string, timestampExpected int64) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := getTime(r, "foo", 123)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from getTime(%q): %s", s, err)
		}
		if ts != 123 {
			t.Fatalf("unexpected default value for getTime(%q); got %d; want %d", s, ts, 123)
		}

		// Verify timestampExpected
		ts, err = getTime(r, "s", 123)
		if err != nil {
			t.Fatalf("unexpected error in getTime(%q): %s", s, err)
		}
		if ts != timestampExpected {
			t.Fatalf("unexpected timestamp for getTime(%q); got %d; want %d", s, ts, timestampExpected)
		}
	}

	f("2019-07-07T20:01:02Z", 1562529662000)
	f("2019-07-07T20:47:40+03:00", 1562521660000)
	f("-292273086-05-16T16:47:06Z", minTimeMsecs)
	f("292277025-08-18T07:12:54.999999999Z", maxTimeMsecs)
	f("1562529662.324", 1562529662324)
	f("-9223372036.854", minTimeMsecs)
	f("-9223372036.855", minTimeMsecs)
	f("9223372036.855", maxTimeMsecs)
}

func TestGetTimeError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := getTime(r, "foo", 123)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from getTime(%q): %s", s, err)
		}
		if ts != 123 {
			t.Fatalf("unexpected default value for getTime(%q); got %d; want %d", s, ts, 123)
		}

		// Verify timestampExpected
		_, err = getTime(r, "s", 123)
		if err == nil {
			t.Fatalf("expecting non-nil error in getTime(%q)", s)
		}
	}

	f("foo")
	f("2019-07-07T20:01:02Zisdf")
	f("2019-07-07T20:47:40+03:00123")
	f("-292273086-05-16T16:47:07Z")
	f("292277025-08-18T07:12:54.999999998Z")
}

func TestAdjustLastPoints(t *testing.T) {
	f := func(tss []netstorage.Result, ct, queryOffset int64, tssExpected []netstorage.Result) {
		t.Helper()
		tss = adjustLastPoints(tss, ct, queryOffset)
		for i, ts := range tss {
			for j, value := range ts.Values {
				expectedValue := tssExpected[i].Values[j]
				if math.IsNaN(expectedValue) {
					if !math.IsNaN(value) {
						t.Fatalf("unexpected result; got %v; want nan", value)
					}
				} else if expectedValue != value {
					t.Fatalf("unexpected result; got %v; want %v", value, expectedValue)
				}
			}
			if !reflect.DeepEqual(ts.Timestamps, tssExpected[i].Timestamps) {
				t.Fatalf("unexpected result; got %v; want %v", tss, tssExpected)
			}
		}

	}

	nan := math.NaN()

	f(nil, 500, 300, nil)

	f([]netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 4, nan},
		},
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, nan, nan},
		},
	}, 500, 300, []netstorage.Result{
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
	}, 500, 300, []netstorage.Result{
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
	}, 500, 300, []netstorage.Result{
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
	}, 500, 300, []netstorage.Result{
		{
			Timestamps: []int64{100, 200, 300, 400, 500},
			Values:     []float64{1, 2, 3, 3, 3},
		},
		{
			Timestamps: []int64{100, 200, 300},
			Values:     []float64{1, 2, nan},
		},
	})
}
