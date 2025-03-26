package tests

import (
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"os"
	"testing"
)

// TestSpecialQueryRegression is used to test queries that have experienced issues for specific data sets.
// These test cases were migrated from `app/victoria-metrics/main_test.go`.
// Most of these cases are based on user feedback. Refer to the corresponding GitHub issue for details on each case.
//
// To improve performance, it will handle ingestion and flushing at one location and then evaluate each query.
func TestSpecialQueryRegression(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	// prometheus
	setupPrometheusImport(t, sut)
	testCaseSensetiveRegex(t, tc, sut)
	testDuplicateLabel(t, tc, sut)
	testTooBigLookbehindWindow(t, tc, sut)
	testMatchSeries(t, tc, sut)

	// graphite
	setupGraphiteImport(t, tc, sut)
	testComparisonNotInfNotNan(t, tc, sut)
	testEmptyLabelMatch(t, tc, sut)
	testMaxLookbehind(t, tc, sut)
	testNonNanAsMissingData(t, tc, sut)
	testSubqueryAggregation(t, tc, sut)
}

// setupPrometheusImport import data for each test cases by prometheus import API.
func setupPrometheusImport(t *testing.T, sut *at.Vmsingle) {
	// case-sensitive-regex
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/161
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.sensitiveRegex{label="sensitiveRegex"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`prometheus.sensitiveRegex{label="SensitiveRegex"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, at.QueryOpts{})
	// duplicate_label
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/172
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.duplicate_label{label="duplicate", label="duplicate"} 10 1707123456700`,
	}, at.QueryOpts{})
	// too big look-behind window
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.too_big_lookbehind{label="foo"} 10 1707123456700`,
	}, at.QueryOpts{})
	// too big look-behind window - query range
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`prometheus.too_big_lookbehind_range{label="foo"} 13 1707123496700`,
		`prometheus.too_big_lookbehind_range{label="foo"} 12 1707123466700`,
		`prometheus.too_big_lookbehind_range{label="foo"} 11 1707123436700`,
		`prometheus.too_big_lookbehind_range{label="foo"} 10 1707123406700`,
	}, at.QueryOpts{})
	// too big look-behind window
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`GenBearTemp{db="TenMinute",Park="1",TurbineType="V112"} 10 1707123456700`,
		`GenBearTemp{db="TenMinute",Park="2",TurbineType="V112"} 10 1707123456700`,
		`GenBearTemp{db="TenMinute",Park="3",TurbineType="V112"} 10 1707123456700`,
		`GenBearTemp{db="TenMinute",Park="4",TurbineType="V112"} 10 1707123456700`,
	}, at.QueryOpts{})

	sut.ForceFlush(t)
}

// setupGraphiteImport import data for each test cases using Graphite TCP conn.
func setupGraphiteImport(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	rowInserted := sut.ServesMetrics.GetIntMetric(t, `vm_rows_inserted_total{type="graphite"}`)

	sut.GraphiteWrite(t, []string{
		"not_nan_not_inf;item=x 1 1707123456",
		"not_nan_not_inf;item=x 1 1707123455",
		"not_nan_not_inf;item=y 3 1707123456",
		"not_nan_not_inf;item=y 1 1707123455",
	}, at.QueryOpts{})
	sut.GraphiteWrite(t, []string{
		"empty_label_match 1 1707123456",
		"empty_label_match;foo=bar 2 1707123456",
		"empty_label_match;foo=baz 3 1707123456",
	}, at.QueryOpts{})
	sut.GraphiteWrite(t, []string{
		"max_lookback_set 1 1707123426",
		"max_lookback_set 2 1707123396",
		"max_lookback_set 3 1707123336",
		"max_lookback_set 4 1707123306",
	}, at.QueryOpts{})
	sut.GraphiteWrite(t, []string{
		"max_lookback_unset 1 1707123426",
		"max_lookback_unset 2 1707123396",
		"max_lookback_unset 3 1707123336",
		"max_lookback_unset 4 1707123306",
	}, at.QueryOpts{})
	sut.GraphiteWrite(t, []string{
		"not_nan_as_missing_data;item=x 2 1707123454",
		"not_nan_as_missing_data;item=x 1 1707123455",
		"not_nan_as_missing_data;item=y 4 1707123454",
		"not_nan_as_missing_data;item=y 3 1707123455",
	}, at.QueryOpts{})
	sut.GraphiteWrite(t, []string{
		"forms_daily_count;item=x 1 1707123396",
		"forms_daily_count;item=x 2 1707123336",
		"forms_daily_count;item=y 3 1707123396",
		"forms_daily_count;item=y 4 1707123336",
	}, at.QueryOpts{})

	tc.Assert(&at.AssertOptions{
		Msg: "unexpected row inserted metrics check",
		Got: func() any {
			return (sut.ServesMetrics.GetIntMetric(t, `vm_rows_inserted_total{type="graphite"}`) - rowInserted) >= 23
		},
		Want: true,
	})

	sut.ForceFlush(t)
}

func testCaseSensetiveRegex(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// case-sensitive-regex
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/161
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/export response",
		Got: func() any {
			return sut.PrometheusAPIV1Export(t, `{label=~'(?i)sensitiveregex'}`, at.QueryOpts{
				Start: "2024-02-05T08:50:00.700Z",
				End:   "2024-02-05T09:00:00.700Z",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric:  map[string]string{"__name__": "prometheus.sensitiveRegex", "label": "SensitiveRegex"},
						Samples: []*at.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
					{
						Metric:  map[string]string{"__name__": "prometheus.sensitiveRegex", "label": "sensitiveRegex"},
						Samples: []*at.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
				},
			},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func testDuplicateLabel(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// duplicate_label
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/172
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/export response",
		Got: func() any {
			return sut.PrometheusAPIV1Export(t, `{__name__='prometheus.duplicate_label'}`, at.QueryOpts{
				Start: "2024-02-05T08:50:00.700Z",
				End:   "2024-02-05T09:00:00.700Z",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric:  map[string]string{"__name__": "prometheus.duplicate_label", "label": "duplicate"},
						Samples: []*at.Sample{{Timestamp: 1707123456700, Value: 10}},
					},
				},
			},
		},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func testTooBigLookbehindWindow(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// too big look-behind window
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query response",
		Got: func() any {
			return sut.PrometheusAPIV1Query(t, `prometheus.too_big_lookbehind{label="foo"}[100y]`, at.QueryOpts{
				Step: "5m",
				Time: "2024-02-05T08:57:36.700Z",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "prometheus.too_big_lookbehind", "label": "foo"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:57:36.700Z", 10),
						},
					},
				},
			},
		},
	})

	// too big look-behind window - query range
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `prometheus.too_big_lookbehind_range{label="foo"}`, at.QueryOpts{
				Start: "2024-02-05T08:56:46.700Z",
				End:   "2024-02-05T08:58:16.700Z",
				Step:  "30s",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "prometheus.too_big_lookbehind_range", "label": "foo"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:56:46.700Z", 10),
							at.NewSample(t, "2024-02-05T08:57:16.700Z", 11),
							at.NewSample(t, "2024-02-05T08:57:46.700Z", 12),
							at.NewSample(t, "2024-02-05T08:58:16.700Z", 13),
						},
					},
				},
			},
		},
	})
}

func testMatchSeries(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// match_series
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/155
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/series response",
		Got: func() any {
			return sut.PrometheusAPIV1Series(t, `{__name__="GenBearTemp"}`, at.QueryOpts{
				Start: "2024-02-04T08:57:36.700Z",
				End:   "2024-02-05T08:57:36.700Z",
			}).Sort()
		},
		Want: &at.PrometheusAPIV1SeriesResponse{
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

func testComparisonNotInfNotNan(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// comparison-not-inf-not-nan
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `1/(not_nan_not_inf-1)!=inf!=nan`, at.QueryOpts{
				Start: "2024-02-05T06:50:36.000Z",
				End:   "2024-02-05T09:58:37.000Z",
				Step:  "60",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"item": "y"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:58:00.000Z", 0.5),
						},
					},
				},
			},
		},
	})
}

func testEmptyLabelMatch(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// empty-label-match
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/395
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `empty_label_match{foo=~'bar|'}`, at.QueryOpts{
				Start: "2024-02-05T08:55:36.000Z",
				End:   "2024-02-05T08:57:36.000Z",
				Step:  "60s",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "empty_label_match"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:57:36.000Z", 1),
						},
					},
					{
						Metric: map[string]string{"__name__": "empty_label_match", "foo": "bar"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:57:36.000Z", 2),
						},
					},
				},
			},
		},
	})
}

func testMaxLookbehind(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// max_lookback_set
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/209
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `max_lookback_set{foo=~'bar|'}`, at.QueryOpts{
				Start:       "2024-02-05T08:55:06.000Z",
				End:         "2024-02-05T08:57:37.000Z",
				Step:        "10s",
				MaxLookback: "1s",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "max_lookback_set"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:55:06.000Z", 4),
							at.NewSample(t, "2024-02-05T08:55:36.000Z", 3),
							at.NewSample(t, "2024-02-05T08:56:36.000Z", 2),
							at.NewSample(t, "2024-02-05T08:57:06.000Z", 1),
						},
					},
				},
			},
		},
	})

	// max_lookback_unset
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/209
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `max_lookback_unset{foo=~'bar|'}`, at.QueryOpts{
				Start: "2024-02-05T08:55:06.000Z",
				End:   "2024-02-05T08:57:37.000Z",
				Step:  "10s",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "max_lookback_unset"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:55:06.000Z", 4),
							at.NewSample(t, "2024-02-05T08:55:16.000Z", 4),
							at.NewSample(t, "2024-02-05T08:55:26.000Z", 4),
							at.NewSample(t, "2024-02-05T08:55:36.000Z", 3),
							at.NewSample(t, "2024-02-05T08:55:46.000Z", 3),
							at.NewSample(t, "2024-02-05T08:55:56.000Z", 3),
							at.NewSample(t, "2024-02-05T08:56:06.000Z", 3),
							at.NewSample(t, "2024-02-05T08:56:16.000Z", 3),
							at.NewSample(t, "2024-02-05T08:56:36.000Z", 2),
							at.NewSample(t, "2024-02-05T08:56:46.000Z", 2),
							at.NewSample(t, "2024-02-05T08:56:56.000Z", 2),
							at.NewSample(t, "2024-02-05T08:57:06.000Z", 1),
							at.NewSample(t, "2024-02-05T08:57:16.000Z", 1),
							at.NewSample(t, "2024-02-05T08:57:26.000Z", 1),
							at.NewSample(t, "2024-02-05T08:57:36.000Z", 1),
						},
					},
				},
			},
		},
	})
}

func testNonNanAsMissingData(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// not-nan-as-missing-data
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query_range response",
		Got: func() any {
			return sut.PrometheusAPIV1QueryRange(t, `not_nan_as_missing_data>1`, at.QueryOpts{
				Start: "2024-02-05T08:57:34.000Z",
				End:   "2024-02-05T08:57:36.000Z",
				Step:  "1s",
			})
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "matrix",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"__name__": "not_nan_as_missing_data", "item": "x"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:57:34.000Z", 2),
						},
					},
					{
						Metric: map[string]string{"__name__": "not_nan_as_missing_data", "item": "y"},
						Samples: []*at.Sample{
							at.NewSample(t, "2024-02-05T08:57:34.000Z", 4),
							at.NewSample(t, "2024-02-05T08:57:35.000Z", 3),
							at.NewSample(t, "2024-02-05T08:57:36.000Z", 3),
						},
					},
				},
			},
		},
	})
}

func testSubqueryAggregation(t *testing.T, tc *at.TestCase, sut *at.Vmsingle) {
	// subquery-aggregation
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/184
	tc.Assert(&at.AssertOptions{
		Msg: "unexpected /api/v1/query response",
		Got: func() any {
			got := sut.PrometheusAPIV1Query(t, `min by (item) (min_over_time(forms_daily_count[10m:1m]))`, at.QueryOpts{
				Time:          "2024-02-05T08:56:35.000Z",
				LatencyOffset: "1ms",
			})
			got.Sort()
			return got
		},
		Want: &at.PrometheusAPIV1QueryResponse{
			Status: "success",
			Data: &at.QueryData{
				ResultType: "vector",
				Result: []*at.QueryResult{
					{
						Metric: map[string]string{"item": "x"},
						Sample: at.NewSample(t, "2024-02-05T08:56:35.000Z", 2),
					},
					{
						Metric: map[string]string{"item": "y"},
						Sample: at.NewSample(t, "2024-02-05T08:56:35.000Z", 4),
					},
				},
			},
		},
	})
}
