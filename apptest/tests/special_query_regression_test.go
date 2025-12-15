package tests

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// TestSingleSpecialQueryRegression is used to test queries that have experienced issues for specific data sets.
// These test cases were migrated from `app/victoria-metrics/main_test.go`.
// Most of these cases are based on user feedback. Refer to the corresponding GitHub issue for details on each case.
func TestSingleSpecialQueryRegression(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()
	testSpecialQueryRegression(tc, sut)
}

func TestClusterSpecialQueryRegression(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorage1",
		Vmstorage1Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage1"),
			"-retentionPeriod=100y",
		},
		Vmstorage2Instance: "vmstorage2",
		Vmstorage2Flags: []string{
			"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage2"),
			"-retentionPeriod=100y",
		},
		VminsertInstance: "vminsert",
		VminsertFlags:    []string{"-graphiteListenAddr=:0", "-opentsdbListenAddr=127.0.0.1:0"},
		VmselectInstance: "vmselect",
	})
	testSpecialQueryRegression(tc, sut)
}

func testSpecialQueryRegression(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	// prometheus
	testCaseSensitiveRegex(tc, sut)
	testDuplicateLabel(tc, sut)
	testTooBigLookbehindWindow(tc, sut)
	testMatchSeries(tc, sut)
	testNegativeIncrease(tc, sut)
	testInstantQueryWithOffsetUsingCache(tc, sut)
	testQueryRangeEndAtFirstMillisecondOfDate(tc, sut)

	// graphite
	testComparisonNotInfNotNan(tc, sut)
	testEmptyLabelMatch(tc, sut)
	testMaxLookbehind(tc, sut)
	testNonNanAsMissingData(tc, sut)
	testSubqueryAggregation(tc, sut)
}

func testCaseSensitiveRegex(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// case-sensitive-regex
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/161
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.sensitiveRegex{label="sensitiveRegex"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`prometheus.sensitiveRegex{label="SensitiveRegex"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/export response",
		Got: func() any {
			resp := sut.PrometheusAPIV1Export(t, `{label=~'(?i)sensitiveregex'}`, apptest.QueryOpts{
				Start: "2024-02-05T08:50:00.700Z",
				End:   "2024-02-05T09:00:00.700Z",
			})
			resp.Sort()
			return resp
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric:  map[string]string{"__name__": "prometheus.sensitiveRegex", "label": "SensitiveRegex"},
						Samples: []*apptest.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
					{
						Metric:  map[string]string{"__name__": "prometheus.sensitiveRegex", "label": "sensitiveRegex"},
						Samples: []*apptest.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
				},
			},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func testDuplicateLabel(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// duplicate_label
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/172
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.duplicate_label{label="duplicate", label="duplicate"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/export response",
		Got: func() any {
			return sut.PrometheusAPIV1Export(t, `{__name__='prometheus.duplicate_label'}`, apptest.QueryOpts{
				Start: "2024-02-05T08:50:00.700Z",
				End:   "2024-02-05T09:00:00.700Z",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric:  map[string]string{"__name__": "prometheus.duplicate_label", "label": "duplicate"},
						Samples: []*apptest.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
				},
			},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func testTooBigLookbehindWindow(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// too big look-behind window
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.too_big_lookbehind{label="foo"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query response",
		Got: func() any {
			return sut.PrometheusAPIV1Query(t, `prometheus.too_big_lookbehind{label="foo"}[100y]`, apptest.QueryOpts{
				Step: "5m",
				Time: "2024-02-05T08:57:36.700Z",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "prometheus.too_big_lookbehind", "label": "foo"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:57:36.700Z", 10),
						},
					},
				},
			},
		},
	})

	// too big look-behind window - query range
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.too_big_lookbehind_range{label="foo"} 13 1707123496700`, // 2024-02-05T08:58:16.700Z
		`prometheus.too_big_lookbehind_range{label="foo"} 12 1707123466700`, // 2024-02-05T08:57:46.700Z
		`prometheus.too_big_lookbehind_range{label="foo"} 11 1707123436700`, // 2024-02-05T08:57:16.700Z
		`prometheus.too_big_lookbehind_range{label="foo"} 10 1707123406700`, // 2024-02-05T08:56:46.700Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `prometheus.too_big_lookbehind_range{label="foo"}`, apptest.QueryOpts{
				Start: "2024-02-05T08:56:46.700Z",
				End:   "2024-02-05T08:58:16.700Z",
				Step:  "30s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "prometheus.too_big_lookbehind_range", "label": "foo"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:56:46.700Z", 10),
							apptest.NewSample(t, "2024-02-05T08:57:16.700Z", 11),
							apptest.NewSample(t, "2024-02-05T08:57:46.700Z", 12),
							apptest.NewSample(t, "2024-02-05T08:58:16.700Z", 13),
						},
					},
				},
			},
		},
	})
}

func testMatchSeries(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// match_series
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/155
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`GenBearTemp{db="TenMinute",Park="1",TurbineType="V112"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`GenBearTemp{db="TenMinute",Park="2",TurbineType="V112"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`GenBearTemp{db="TenMinute",Park="3",TurbineType="V112"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`GenBearTemp{db="TenMinute",Park="4",TurbineType="V112"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/series response",
		Got: func() any {
			return sut.PrometheusAPIV1Series(t, `{__name__="GenBearTemp"}`, apptest.QueryOpts{
				Start: "2024-02-04T08:57:36.700Z",
				End:   "2024-02-05T08:57:36.700Z",
			}).Sort()
		},
		Want: &apptest.PrometheusAPIV1SeriesResponse{
			Status:    "success",
			IsPartial: false,
			Data: []map[string]string{
				{"__name__": "GenBearTemp", "db": "TenMinute", "Park": "1", "TurbineType": "V112"},
				{"__name__": "GenBearTemp", "db": "TenMinute", "Park": "2", "TurbineType": "V112"},
				{"__name__": "GenBearTemp", "db": "TenMinute", "Park": "3", "TurbineType": "V112"},
				{"__name__": "GenBearTemp", "db": "TenMinute", "Park": "4", "TurbineType": "V112"},
			},
		},
	})
}

func testNegativeIncrease(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// negative increase when user overrides staleness interval
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8935#issuecomment-2978728661
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`foo 108 1750109243514`, // 2025-06-16 21:27:23:514
		`foo 108 1750109258514`, // 2025-06-16 21:27:38:514
		// gap 75s
		`foo 1 1750109333514`, // 2025-06-16 21:28:53:514
		`foo 1 1750109348514`, // 2025-06-16 21:29:08:514
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg:        "regression for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8935#issuecomment-2978728661",
		DoNotRetry: true,
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `increase(foo[1m])`, apptest.QueryOpts{
				Start:       "2025-06-16T21:28:40.700Z",
				End:         "2025-06-16T21:29:30.700Z",
				Step:        "9s",
				MaxLookback: "65s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2025-06-16T21:28:40.700Z", 0),
							apptest.NewSample(t, "2025-06-16T21:28:49.700Z", 0),
							apptest.NewSample(t, "2025-06-16T21:28:58.700Z", 1),
							apptest.NewSample(t, "2025-06-16T21:29:07.700Z", 1),
							apptest.NewSample(t, "2025-06-16T21:29:16.700Z", 0),
							apptest.NewSample(t, "2025-06-16T21:29:25.700Z", 0),
						},
					},
				},
			},
		},
	})
}

func testInstantQueryWithOffsetUsingCache(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// unexpected /api/v1/query response due to wrong applied offset to request range
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9762
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`vm_http_requests_total 1 1758196800000`, // 2025-09-18 12:00:00
		`vm_http_requests_total 2 1758218400000`, // 2025-09-18 18:00:00
		`vm_http_requests_total 3 1758240000000`, // 2025-09-19 00:00:00
		`vm_http_requests_total 4 1758261600000`, // 2025-09-19 06:00:00
		`vm_http_requests_total 5 1758283200000`, // 2025-09-19 12:00:00
		`vm_http_requests_total 6 1758304800000`, // 2025-09-19 18:00:00
		`vm_http_requests_total 7 1758326400000`, // 2025-09-20 00:00:00
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg:        "unexpected /api/v1/query response",
		DoNotRetry: true,
		Got: func() any {
			return sut.PrometheusAPIV1Query(t, `avg_over_time(vm_http_requests_total[1d] offset 12h)`, apptest.QueryOpts{
				Time: "2025-09-20T12:00:01.000Z",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "vector",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{},
						Sample: &apptest.Sample{Timestamp: 1758369601000, Value: 5.5},
					},
				},
			},
		},
	})
}

func testQueryRangeEndAtFirstMillisecondOfDate(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// unexpected /api/v1/query_range response
	// when the sample is at the last millisecond of a day, e.g. `2025-12-12 00:00:00`
	// query_range with `End` at the last millisecond of that day may cause the time point to be missed.
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9804

	// `End` should be inclusive according to https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries

	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`foo_bar 7 1765497600000`, // 2025-12-12 00:00:00
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg:        "unexpected /api/v1/query response",
		DoNotRetry: true,
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `foo_bar`, apptest.QueryOpts{
				Start: "2025-12-11T20:00:00.000Z",
				End:   "2025-12-12T00:00:00.000Z",
				Step:  "1h",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "foo_bar"},
						Samples: []*apptest.Sample{
							{Timestamp: 1765497600000, Value: 7},
						},
					},
				},
			},
		},
	})
}

func testComparisonNotInfNotNan(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// comparison-not-inf-not-nan
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150
	rowInserted := getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"not_nan_not_inf;item=x 1 1707123456", // 2024-02-05T08:57:36.000Z
		"not_nan_not_inf;item=x 1 1707123455", // 2024-02-05T08:57:35.000Z
		"not_nan_not_inf;item=y 3 1707123456", // 2024-02-05T08:57:36.000Z
		"not_nan_not_inf;item=y 1 1707123455", // 2024-02-05T08:57:35.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 4
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `1/(not_nan_not_inf-1)!=inf!=nan`, apptest.QueryOpts{
				Start: "2024-02-05T06:50:36.000Z",
				End:   "2024-02-05T09:58:37.000Z",
				Step:  "60",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"item": "y"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:58:00.000Z", 0.5),
						},
					},
				},
			},
		},
	})
}

func testEmptyLabelMatch(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// empty-label-match
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/395
	rowInserted := getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"empty_label_match 1 1707123456",         // 2024-02-05T08:57:36.000Z
		"empty_label_match;foo=bar 2 1707123456", // 2024-02-05T08:57:36.000Z
		"empty_label_match;foo=baz 3 1707123456", // 2024-02-05T08:57:36.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 3
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `empty_label_match{foo=~'bar|'}`, apptest.QueryOpts{
				Start: "2024-02-05T08:55:36.000Z",
				End:   "2024-02-05T08:57:36.000Z",
				Step:  "60s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "empty_label_match"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:57:36.000Z", 1),
						},
					},
					{
						Metric: map[string]string{"__name__": "empty_label_match", "foo": "bar"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:57:36.000Z", 2),
						},
					},
				},
			},
		},
	})
}

func testMaxLookbehind(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// max_lookback_set
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/209
	rowInserted := getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"max_lookback_set 1 1707123426", // 2024-02-05T08:57:06.000Z
		"max_lookback_set 2 1707123396", // 2024-02-05T08:56:36.000Z
		"max_lookback_set 3 1707123336", // 2024-02-05T08:55:36.000Z",
		"max_lookback_set 4 1707123306", // 2024-02-05T08:55:06.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 4
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `max_lookback_set{foo=~'bar|'}`, apptest.QueryOpts{
				Start:       "2024-02-05T08:55:06.000Z",
				End:         "2024-02-05T08:57:37.000Z",
				Step:        "10s",
				MaxLookback: "1s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "max_lookback_set"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:55:06.000Z", 4),
							apptest.NewSample(t, "2024-02-05T08:55:36.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:56:36.000Z", 2),
							apptest.NewSample(t, "2024-02-05T08:57:06.000Z", 1),
						},
					},
				},
			},
		},
	})

	// max_lookback_unset
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/209
	rowInserted = getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"max_lookback_unset 1 1707123426", // 2024-02-05T08:57:06.000Z
		"max_lookback_unset 2 1707123396", // 2024-02-05T08:56:36.000Z
		"max_lookback_unset 3 1707123336", // 2024-02-05T08:55:36.000Z
		"max_lookback_unset 4 1707123306", // 2024-02-05T08:55:06.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 4
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `max_lookback_unset{foo=~'bar|'}`, apptest.QueryOpts{
				Start: "2024-02-05T08:55:06.000Z",
				End:   "2024-02-05T08:57:37.000Z",
				Step:  "10s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "max_lookback_unset"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:55:06.000Z", 4),
							apptest.NewSample(t, "2024-02-05T08:55:16.000Z", 4),
							apptest.NewSample(t, "2024-02-05T08:55:26.000Z", 4),
							apptest.NewSample(t, "2024-02-05T08:55:36.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:55:46.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:55:56.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:56:06.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:56:16.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:56:36.000Z", 2),
							apptest.NewSample(t, "2024-02-05T08:56:46.000Z", 2),
							apptest.NewSample(t, "2024-02-05T08:56:56.000Z", 2),
							apptest.NewSample(t, "2024-02-05T08:57:06.000Z", 1),
							apptest.NewSample(t, "2024-02-05T08:57:16.000Z", 1),
							apptest.NewSample(t, "2024-02-05T08:57:26.000Z", 1),
							apptest.NewSample(t, "2024-02-05T08:57:36.000Z", 1),
						},
					},
				},
			},
		},
	})
}

func testNonNanAsMissingData(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// not-nan-as-missing-data
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	rowInserted := getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"not_nan_as_missing_data;item=x 2 1707123454", // 2024-02-05T08:57:34.000Z
		"not_nan_as_missing_data;item=x 1 1707123455", // 2024-02-05T08:57:35.000Z
		"not_nan_as_missing_data;item=y 4 1707123454", // 2024-02-05T08:57:34.000Z
		"not_nan_as_missing_data;item=y 3 1707123455", // 2024-02-05T08:57:35.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 4
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `not_nan_as_missing_data>1`, apptest.QueryOpts{
				Start: "2024-02-05T08:57:34.000Z",
				End:   "2024-02-05T08:57:36.000Z",
				Step:  "1s",
			})
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "matrix",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"__name__": "not_nan_as_missing_data", "item": "x"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:57:34.000Z", 2),
						},
					},
					{
						Metric: map[string]string{"__name__": "not_nan_as_missing_data", "item": "y"},
						Samples: []*apptest.Sample{
							apptest.NewSample(t, "2024-02-05T08:57:34.000Z", 4),
							apptest.NewSample(t, "2024-02-05T08:57:35.000Z", 3),
							apptest.NewSample(t, "2024-02-05T08:57:36.000Z", 3),
						},
					},
				},
			},
		},
	})
}

func testSubqueryAggregation(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	// subquery-aggregation
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/184
	rowInserted := getRowsInsertedTotal(t, sut)
	sut.GraphiteWrite(t, []string{
		"forms_daily_count;item=x 1 1707123396", // 2024-02-05T08:56:36.000Z
		"forms_daily_count;item=x 2 1707123336", // 2024-02-05T08:55:36.000Z
		"forms_daily_count;item=y 3 1707123396", // 2024-02-05T08:56:36.000Z
		"forms_daily_count;item=y 4 1707123336", // 2024-02-05T08:55:36.000Z
	}, apptest.QueryOpts{})
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (getRowsInsertedTotal(t, sut) - rowInserted) >= 4
		},
		Want: true,
	})
	sut.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/query response",
		Got: func() any {
			got := sut.PrometheusAPIV1Query(t, `min by (item) (min_over_time(forms_daily_count[10m:1m]))`, apptest.QueryOpts{
				Time:          "2024-02-05T08:56:35.000Z",
				LatencyOffset: "1ms",
			})
			got.Sort()
			return got
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &apptest.QueryData{
				ResultType: "vector",
				Result: []*apptest.QueryResult{
					{
						Metric: map[string]string{"item": "x"},
						Sample: apptest.NewSample(t, "2024-02-05T08:56:35.000Z", 2),
					},
					{
						Metric: map[string]string{"item": "y"},
						Sample: apptest.NewSample(t, "2024-02-05T08:56:35.000Z", 4),
					},
				},
			},
		},
	})
}

func getRowsInsertedTotal(t *testing.T, sut apptest.PrometheusWriteQuerier) int {
	t.Helper()

	selector := `vm_rows_inserted_total{type="graphite"}`
	switch tt := sut.(type) {
	case *apptest.Vmsingle:
		return tt.GetIntMetric(t, selector)
	case *apptest.Vmcluster:
		return tt.Vminsert.GetIntMetric(t, selector)
	default:
		t.Fatalf("unexpected type: got %T, want *Vmsingle or *Vminsert", sut)
	}
	return 0
}
