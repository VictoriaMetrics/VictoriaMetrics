package tests

import (
	"fmt"
	"testing"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleSearchWithDisabledPerDayIndex(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	testSearchWithDisabledPerDayIndex(tc, func(name string, disablePerDayIndex bool) at.PrometheusWriteQuerier {
		return tc.MustStartVmsingle("vmsingle-"+name, []string{
			"-storageDataPath=" + tc.Dir() + "/vmsingle",
			"-retentionPeriod=100y",
			"-search.maxStalenessInterval=1m",
			fmt.Sprintf("-disablePerDayIndex=%t", disablePerDayIndex),
		})
	})
}

func TestClusterSearchWithDisabledPerDayIndex(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	testSearchWithDisabledPerDayIndex(tc, func(name string, disablePerDayIndex bool) at.PrometheusWriteQuerier {
		return tc.MustStartCluster(&at.ClusterOptions{
			Vmstorage1Instance: "vmstorage1-" + name,
			Vmstorage1Flags: []string{
				fmt.Sprintf("-disablePerDayIndex=%t", disablePerDayIndex),
			},
			Vmstorage2Instance: "vmstorage2-" + name,
			Vmstorage2Flags: []string{
				fmt.Sprintf("-disablePerDayIndex=%t", disablePerDayIndex),
			},
			VminsertInstance: "vminsert",
			VmselectInstance: "vmselect",
		})
	})
}

type startSUTFunc func(name string, disablePerDayIndex bool) at.PrometheusWriteQuerier

// testDisablePerDayIndex_Search shows what search results to expect when data
// is first inserted with per-day index enabled and then with per-day index
// disabled.
//
// The data inserted with enabled per-day index must be searchable with disabled
// per-day index.
//
// The data inserted with disabled per-day index is not searcheable with per-day
// index enabled unless the search time range is > 40 days.
func testSearchWithDisabledPerDayIndex(tc *at.TestCase, start startSUTFunc) {
	t := tc.T()

	type opts struct {
		start, end       string
		wantSeries       []map[string]string
		wantQueryResults []*at.QueryResult
	}
	assertSearchResults := func(sut at.PrometheusQuerier, opts *opts) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return sut.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{
					Start: opts.start,
					End:   opts.end,
				}).Sort()
			},
			Want: &at.PrometheusAPIV1SeriesResponse{
				Status: "success",
				Data:   opts.wantSeries,
			},
		})
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return sut.PrometheusAPIV1QueryRange(t, `{__name__=~".*"}`, at.QueryOpts{
					Start: opts.start,
					End:   opts.end,
					Step:  "1d",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result:     opts.wantQueryResults,
				},
			},
		})
	}

	// Start vmsingle with enabled per-day index, insert sample1, and confirm it
	// is searcheable.
	sut := start("with-per-day-index", false)
	sample1 := []string{"metric1 111 1704067200000"} // 2024-01-01T00:00:00Z
	sut.PrometheusAPIV1ImportPrometheus(t, sample1, at.QueryOpts{})
	sut.ForceFlush(t)
	assertSearchResults(sut, &opts{
		start:      "2024-01-01T00:00:00Z",
		end:        "2024-01-01T23:59:59Z",
		wantSeries: []map[string]string{{"__name__": "metric1"}},
		wantQueryResults: []*at.QueryResult{
			{
				Metric:  map[string]string{"__name__": "metric1"},
				Samples: []*at.Sample{{Timestamp: 1704067200000, Value: float64(111)}},
			},
		},
	})

	// Restart vmsingle with disabled per-day index, insert sample2, and confirm
	// that both sample1 and sample2 is searcheable.
	tc.StopPrometheusWriteQuerier(sut)
	sut = start("without-per-day-index", true)
	sample2 := []string{"metric2 222 1704067200000"} // 2024-01-01T00:00:00Z
	sut.PrometheusAPIV1ImportPrometheus(t, sample2, at.QueryOpts{})
	sut.ForceFlush(t)
	assertSearchResults(sut, &opts{
		start: "2024-01-01T00:00:00Z",
		end:   "2024-01-01T23:59:59Z",
		wantSeries: []map[string]string{
			{"__name__": "metric1"},
			{"__name__": "metric2"},
		},
		wantQueryResults: []*at.QueryResult{
			{
				Metric:  map[string]string{"__name__": "metric1"},
				Samples: []*at.Sample{{Timestamp: 1704067200000, Value: float64(111)}},
			},
			{
				Metric:  map[string]string{"__name__": "metric2"},
				Samples: []*at.Sample{{Timestamp: 1704067200000, Value: float64(222)}},
			},
		},
	})

	// Insert sample1 but for a different date, restart vmsingle with enabled
	// per-day index and confirm that:
	// - sample1 is searcheable within the time range of Jan 1st
	// - sample1 is not searcheable within the time range of Jan 20th
	// - sample1 is searcheable within the time range of Jan 1st-20th (because
	//   the metric1 metricID will be found in the per-day index for Jan 1st).
	// - sample2 is not searcheable when the time range is <= 40 days
	// - sample2 becomes searcheable when the time range is > 40 days
	sample3 := []string{"metric1 333 1705708800000"} // 2024-01-20T00:00:00Z
	sut.PrometheusAPIV1ImportPrometheus(t, sample3, at.QueryOpts{})
	sut.ForceFlush(t)
	tc.StopPrometheusWriteQuerier(sut)
	sut = start("with-per-day-index2", false)

	// Time range is 1 day (Jan 1st) <= 40 days
	assertSearchResults(sut, &opts{
		start: "2024-01-01T00:00:00Z",
		end:   "2024-01-01T23:59:59Z",
		wantSeries: []map[string]string{
			{"__name__": "metric1"},
		},
		wantQueryResults: []*at.QueryResult{
			{
				Metric:  map[string]string{"__name__": "metric1"},
				Samples: []*at.Sample{{Timestamp: 1704067200000, Value: float64(111)}},
			},
		},
	})

	// Time range is 1 day (Jan 20th) <= 40 days
	assertSearchResults(sut, &opts{
		start:            "2024-01-20T00:00:00Z",
		end:              "2024-01-20T23:59:59Z",
		wantSeries:       []map[string]string{},
		wantQueryResults: []*at.QueryResult{},
	})

	// Time range is 20 days (Jan 1st-20th) <= 40 days
	assertSearchResults(sut, &opts{
		start: "2024-01-01T00:00:00Z",
		end:   "2024-01-20T23:59:59Z",
		wantSeries: []map[string]string{
			{"__name__": "metric1"},
		},
		wantQueryResults: []*at.QueryResult{
			{
				Metric: map[string]string{"__name__": "metric1"},
				Samples: []*at.Sample{
					{Timestamp: 1704067200000, Value: float64(111)},
					{Timestamp: 1705708800000, Value: float64(333)},
				},
			},
		},
	})

	// Time range > 40 days
	assertSearchResults(sut, &opts{
		start: "2024-01-01T00:00:00Z",
		end:   "2024-02-29T23:59:59Z",
		wantSeries: []map[string]string{
			{"__name__": "metric1"},
			{"__name__": "metric2"},
		},
		wantQueryResults: []*at.QueryResult{
			{
				Metric: map[string]string{"__name__": "metric1"},
				Samples: []*at.Sample{
					{Timestamp: 1704067200000, Value: float64(111)},
					{Timestamp: 1705708800000, Value: float64(333)},
				},
			},
			{
				Metric: map[string]string{"__name__": "metric2"},
				Samples: []*at.Sample{
					{Timestamp: 1704067200000, Value: float64(222)},
				},
			},
		},
	})
}
