package prometheus

import (
	"math"
	"net/http"
	"reflect"
	"runtime"
	"testing"

	"encoding/json"
	"io/ioutil"
	"net/http/httptest"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
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

func TestGetLatencyOffsetMillisecondsSuccess(t *testing.T) {
	f := func(url string, expectedOffset int64) {
		t.Helper()
		r, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest(%q): %s", url, err)
		}
		offset, err := getLatencyOffsetMilliseconds(r)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if offset != expectedOffset {
			t.Fatalf("unexpected offset got %d; want %d", offset, expectedOffset)
		}
	}
	f("http://localhost", latencyOffset.Milliseconds())
	f("http://localhost?latency_offset=1.234s", 1234)
}

func TestGetLatencyOffsetMillisecondsFailure(t *testing.T) {
	f := func(url string) {
		t.Helper()
		r, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest(%q): %s", url, err)
		}
		if _, err := getLatencyOffsetMilliseconds(r); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f("http://localhost?latency_offset=foobar")
}

func TestCalculateMaxMetricsLimitByResource(t *testing.T) {
	f := func(maxConcurrentRequest, remainingMemory, expect int) {
		t.Helper()
		maxMetricsLimit := calculateMaxUniqueTimeSeriesForResource(maxConcurrentRequest, remainingMemory)
		if maxMetricsLimit != expect {
			t.Fatalf("unexpected max metrics limit: got %d, want %d", maxMetricsLimit, expect)
		}
	}

	// Skip when GOARCH=386
	if runtime.GOARCH != "386" {
		// 8 CPU & 32 GiB
		f(16, int(math.Round(32*1024*1024*1024*0.4)), 4294967)
		// 4 CPU & 32 GiB
		f(8, int(math.Round(32*1024*1024*1024*0.4)), 8589934)
	}

	// 2 CPU & 4 GiB
	f(4, int(math.Round(4*1024*1024*1024*0.4)), 2147483)

	// other edge cases
	f(0, int(math.Round(4*1024*1024*1024*0.4)), 2e9)
	f(4, 0, 0)

}

func TestMetadataHandler_Limits(t *testing.T) {
	// Save and restore the original searchMetricNamesFn
	origSearchMetricNamesFn := searchMetricNamesFn
	defer func() { searchMetricNamesFn = origSearchMetricNamesFn }()

	// Mock metric metadata series
	mockSeries := []string{
		"metric_metadata\x01help\x01foo help\x01metric\x01foo\x01type\x01gauge\x01unit\x01seconds\x01",
		"metric_metadata\x01help\x01bar help\x01metric\x01bar\x01type\x01counter\x01unit\x01bytes\x01",
		"metric_metadata\x01help\x01baz help\x01metric\x01baz\x01type\x01gauge\x01unit\x01count\x01",
	}

	searchMetricNamesFn = func(_ *querytracer.Tracer, _ *storage.SearchQuery, _ searchutil.Deadline) ([]string, error) {
		return mockSeries, nil
	}

	testCases := []struct {
		name   string
		query  string
		expect map[string][]map[string]string
	}{
		{
			name:  "no limits",
			query: "",
			expect: map[string][]map[string]string{
				"foo": {{"type": "gauge", "help": "foo help", "unit": "seconds"}},
				"bar": {{"type": "counter", "help": "bar help", "unit": "bytes"}},
				"baz": {{"type": "gauge", "help": "baz help", "unit": "count"}},
			},
		},
		{
			name:  "limit=2",
			query: "limit=2",
			expect: map[string][]map[string]string{
				"foo": {{"type": "gauge", "help": "foo help", "unit": "seconds"}},
				"bar": {{"type": "counter", "help": "bar help", "unit": "bytes"}},
			},
		},
		{
			name:  "limit_per_metric=0 (no effect)",
			query: "limit_per_metric=0",
			expect: map[string][]map[string]string{
				"foo": {{"type": "gauge", "help": "foo help", "unit": "seconds"}},
				"bar": {{"type": "counter", "help": "bar help", "unit": "bytes"}},
				"baz": {{"type": "gauge", "help": "baz help", "unit": "count"}},
			},
		},
		{
			name:  "limit=1",
			query: "limit=1",
			expect: map[string][]map[string]string{
				"foo": {{"type": "gauge", "help": "foo help", "unit": "seconds"}},
			},
		},
		{
			name:  "limit_per_metric=1",
			query: "limit_per_metric=1",
			expect: map[string][]map[string]string{
				"foo": {{"type": "gauge", "help": "foo help", "unit": "seconds"}},
				"bar": {{"type": "counter", "help": "bar help", "unit": "bytes"}},
				"baz": {{"type": "gauge", "help": "baz help", "unit": "count"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/metadata?"+tc.query, nil)
			rw := httptest.NewRecorder()
			MetadataHandler(rw, req)
			resp := rw.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("unexpected status: %d", resp.StatusCode)
			}
			body, _ := ioutil.ReadAll(resp.Body)
			var parsed struct {
				Status string `json:"status"`
				Data   map[string][]map[string]string `json:"data"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("failed to parse response: %v\nbody: %s", err, string(body))
			}
			if parsed.Status != "success" {
				t.Fatalf("unexpected status: %s", parsed.Status)
			}
			if !reflect.DeepEqual(parsed.Data, tc.expect) {
				t.Errorf("unexpected data.\nGot:  %#v\nWant: %#v", parsed.Data, tc.expect)
			}
		})
	}
}
