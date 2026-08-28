package tests

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSinglePrometheusQueryTracer(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testPrometheusQueryTracer(tc, sut)
}

func TestClusterPrometheusQueryTracer(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testPrometheusQueryTracer(tc, sut)
}

// testQueryTracer executes all search query types on various time ranges with
// query tracer enabled to ensure that query tracer used in the search code is
// set up correctly and does not cause panics.
//
// Different time ranges are considered because search code takes different
// paths depending on the time range length and as the result produces different
// query traces. Important time ranges are:
//   - 1d: search in per-day index without concurrency (the simplest case)
//   - 7w: (or any time range > 1d and < 1m) concurrent search in per-day index.
//     Query tracer is not safe to use concurrently. Therefore, an instance of
//     query tracer cannot be shared between many goroutines. Instead, a child
//     query tracer needs to be created for each goroutine. This often gets
//     overlooked and causes panics later for queries with query tracer
//     enabled.
//   - 1m: this is when search switches to global index search.
//
// Tested endpoints:
//   - /api/v1/query_range
//   - /api/v1/series
//   - /api/v1/series/count
//   - /api/v1/labels
//   - /api/v1/metadata
//   - /api/v1/status/metric_names_stats
//
// TODO(@rtm0): Add tests for:
//   - /api/v1/admin/tsdb/delete_series
//   - /api/v1/status/tsdb
//   - /api/v1/export
//   - /api/v1/export/csv
//   - /api/v1/export/native
//   - /federate
//   - /graphite/metrics/index.json
//   - /graphite/metrics/find
//   - /graphite/metrics/expand
//   - /graphite/render
//   - /graphite/tags
//   - /graphite/tags/findSeries
//   - /graphite/tags/autoComplete/tags
//   - /graphite/tags/autoComplete/values
//   - /graphite/tags/delSeries
func testPrometheusQueryTracer(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier) {
	t := tc.T()

	const (
		numMetrics = 10
		tenantID   = ""
	)

	var start, end int64
	start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	data1d := apptest.GenerateTestData("metric_1d", numMetrics, start, end)
	start = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end = time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC).UnixMilli()
	data1w := apptest.GenerateTestData("metric_1w", numMetrics, start, end)
	start = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	data1m := apptest.GenerateTestData("metric_1m", numMetrics, start, end)

	sut.PrometheusAPIV1ImportPrometheus(t, data1d.Samples, apptest.QueryOpts{})
	sut.PrometheusAPIV1ImportPrometheus(t, data1w.Samples, apptest.QueryOpts{})
	sut.PrometheusAPIV1ImportPrometheus(t, data1m.Samples, apptest.QueryOpts{})
	sut.ForceFlush(t)

	assertQueryResults := func(metricNameRE string, d apptest.TestData) {
		t.Helper()
		apptest.AssertQueryResults(tc, sut, metricNameRE, tenantID, d.Start, d.End, d.Step, d.WantQueryResults)
	}
	assertQueryResults(`metric_1d.*`, data1d)
	assertQueryResults(`metric_1w.*`, data1w)
	assertQueryResults(`metric_1m.*`, data1m)

	assertSeries := func(metricNameRE string, d apptest.TestData) {
		t.Helper()
		apptest.AssertSeries(tc, sut, metricNameRE, tenantID, d.Start, d.End, d.WantSeries)
	}
	assertSeries(`metric_1d.*`, data1d)
	assertSeries(`metric_1w.*`, data1w)
	assertSeries(`metric_1m.*`, data1m)

	assertLabels := func(metricNameRE string, d apptest.TestData) {
		t.Helper()
		apptest.AssertLabels(tc, sut, metricNameRE, tenantID, d.Start, d.End, d.WantLabels)
	}
	assertLabels(`metric_1d.*`, data1d)
	assertLabels(`metric_1w.*`, data1w)
	assertLabels(`metric_1m.*`, data1m)

	assertLabelValues := func(metricNameRE string, d apptest.TestData) {
		t.Helper()
		apptest.AssertLabelValues(tc, sut, metricNameRE, "label", tenantID, d.Start, d.End, d.WantLabelValues)
	}
	assertLabelValues(`metric_1d.*`, data1d)
	assertLabelValues(`metric_1w.*`, data1w)
	assertLabelValues(`metric_1m.*`, data1m)

	assertSeriesCount := func(d apptest.TestData) {
		t.Helper()
		apptest.AssertSeriesCount(tc, sut, tenantID, d.Start, d.End, numMetrics*3)
	}
	assertSeriesCount(data1d)
	assertSeriesCount(data1w)
	assertSeriesCount(data1m)

	allMetadata := make(map[string][]apptest.MetadataEntry)
	maps.Insert(allMetadata, maps.All(data1d.WantMetadata))
	maps.Insert(allMetadata, maps.All(data1w.WantMetadata))
	maps.Insert(allMetadata, maps.All(data1m.WantMetadata))
	apptest.AssertMetadata(tc, sut, "", tenantID, allMetadata)

	allStats := slices.Concat(data1d.WantMetricNamesStats, data1w.WantMetricNamesStats, data1m.WantMetricNamesStats)
	// Metric name usage stats depends on previous queries.
	for i := range allStats {
		allStats[i].QueryRequestsCount = 1
	}
	apptest.AssertMetricNamesStats(tc, sut, "", tenantID, allStats)
}
