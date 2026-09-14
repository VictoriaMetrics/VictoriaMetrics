package tests

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleMaxBackfillAge(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	opts := maxBackfillAgeOpts{
		start: func(retentionPeriod, maxBackfillAge string) apptest.PrometheusWriteQuerier {
			return tc.MustStartVmsingle("vmsingle", []string{
				"-storageDataPath=" + filepath.Join(tc.Dir(), "vmsingle"),
				"-retentionPeriod=" + retentionPeriod,
				"-maxBackfillAge=" + maxBackfillAge,
			})
		},
		stop: func() {
			tc.StopApp("vmsingle")
		},
	}

	testMaxBackfillAge(tc, opts)
}

func TestClusterMaxBackfillAge(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	opts := maxBackfillAgeOpts{
		start: func(retentionPeriod, maxBackfillAge string) apptest.PrometheusWriteQuerier {
			return tc.MustStartCluster(&apptest.ClusterOptions{
				Vmstorage1Instance: "vmstorage1",
				Vmstorage1Flags: []string{
					"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage1"),
					"-retentionPeriod=" + retentionPeriod,
					"-maxBackfillAge=" + maxBackfillAge,
				},
				Vmstorage2Instance: "vmstorage2",
				Vmstorage2Flags: []string{
					"-storageDataPath=" + filepath.Join(tc.Dir(), "vmstorage2"),
					"-retentionPeriod=" + retentionPeriod,
					"-maxBackfillAge=" + maxBackfillAge,
				},
				VminsertInstance: "vminsert",
				VminsertFlags:    []string{},
				VmselectInstance: "vmselect",
				VmselectFlags:    []string{},
			})
		},
		stop: func() {
			tc.StopApp("vminsert")
			tc.StopApp("vmselect")
			tc.StopApp("vmstorage1")
			tc.StopApp("vmstorage2")
		},
	}

	testMaxBackfillAge(tc, opts)
}

type maxBackfillAgeOpts struct {
	start func(retentionPeriod, maxBackfillAge string) apptest.PrometheusWriteQuerier
	stop  func()
}

func testMaxBackfillAge(tc *apptest.TestCase, opts maxBackfillAgeOpts) {
	t := tc.T()

	assertSeries := func(app apptest.PrometheusQuerier, prefix string, start, end int64, want []map[string]string) {
		t.Helper()

		query := fmt.Sprintf(`{__name__=~"metric_%s.*"}`, prefix)
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, query, apptest.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
				}).Sort()
			},
			Want: &apptest.PrometheusAPIV1SeriesResponse{
				Status: "success",
				Data:   want,
			},
			FailNow: true,
		})
	}

	assertQueryResults := func(app apptest.PrometheusQuerier, prefix string, start, end, step int64, want []*apptest.QueryResult) {
		t.Helper()

		query := fmt.Sprintf(`{__name__=~"metric_%s.*"}`, prefix)
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, query, apptest.QueryOpts{
					Start:       fmt.Sprintf("%d", start),
					End:         fmt.Sprintf("%d", end),
					Step:        fmt.Sprintf("%dms", step),
					MaxLookback: fmt.Sprintf("%dms", step-1),
					NoCache:     "1",
				})
			},
			Want: &apptest.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &apptest.QueryData{
					ResultType: "matrix",
					Result:     want,
				},
			},
			FailNow: true,
		})
	}

	const numMetrics = 1000
	now := time.Now().UTC()
	var start, end, step int64
	emptySeries := []map[string]string{}
	emptyQueryResults := []*apptest.QueryResult{}

	// Start sut with the same -retentionPeriod and -maxBackfillAge.
	sut := opts.start("1y", "1y")

	// Verify that samples older than the retention period are rejected.
	start = now.Add(-365 * 24 * time.Hour).Add(-time.Hour).UnixMilli()
	end = now.Add(-365 * 24 * time.Hour).UnixMilli()
	step = (end - start) / numMetrics
	outsideRetention := genMaxBackfillAgeData("outside_retention", numMetrics, start, step)
	sut.PrometheusAPIV1ImportPrometheus(t, outsideRetention.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "outside_retention", start, end, emptySeries)
	assertQueryResults(sut, "outside_retention", start, end, step, emptyQueryResults)

	// Verify that samples within the retention period are accepted and
	// searcheable.
	start = now.Add(-365 * 24 * time.Hour).Add(time.Hour).UnixMilli()
	end = now.Add(-365 * 24 * time.Hour).Add(2 * time.Hour).UnixMilli()
	step = (end - start) / numMetrics
	insideRetention := genMaxBackfillAgeData("inside_retention", numMetrics, start, step)
	sut.PrometheusAPIV1ImportPrometheus(t, insideRetention.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "inside_retention", start, end, insideRetention.wantSeries)
	assertQueryResults(sut, "inside_retention", start, end, step, insideRetention.wantQueryResults)

	// Restart sut with -maxBackfillAge shorter than the -retentionPeriod.
	opts.stop()
	sut = opts.start("1y", "6M")

	// Verify that new samples older than max backfill age but still within the
	// retention period are rejected but existing samples are still searcheable.
	start = now.Add(-365 * 24 * time.Hour).Add(time.Hour).UnixMilli()
	end = now.Add(-365 * 24 * time.Hour).Add(2 * time.Hour).UnixMilli()
	step = (end - start) / numMetrics
	insideRetention2 := genMaxBackfillAgeData("inside_retention2", numMetrics, start, step)
	sut.PrometheusAPIV1ImportPrometheus(t, insideRetention2.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "inside_retention2", start, end, emptySeries)
	assertQueryResults(sut, "inside_retention2", start, end, step, emptyQueryResults)
	assertSeries(sut, "inside_retention", start, end, insideRetention.wantSeries)
	assertQueryResults(sut, "inside_retention", start, end, step, insideRetention.wantQueryResults)

	// Verify that the metrics that are outside the backfill window can still
	// be deleted.
	sut.PrometheusAPIV1AdminTSDBDeleteSeries(t, `{__name__=~".*inside_retention.*"}`, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "inside_retention", start, end, emptySeries)
	assertQueryResults(sut, "inside_retention", start, end, step, emptyQueryResults)

	// Verify that the samples that are within the backfill window are accepted
	// and searchable.
	start = now.Add(-180 * 24 * time.Hour).UnixMilli()
	end = now.Add(-180 * 24 * time.Hour).Add(1 * time.Hour).UnixMilli()
	step = (end - start) / numMetrics
	insideMaxBackfillAge := genMaxBackfillAgeData("inside_max_backfill_age", numMetrics, start, step)
	sut.PrometheusAPIV1ImportPrometheus(t, insideMaxBackfillAge.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "inside_max_backfill_age", start, end, insideMaxBackfillAge.wantSeries)
	assertQueryResults(sut, "inside_max_backfill_age", start, end, step, insideMaxBackfillAge.wantQueryResults)

	opts.stop()
}

type maxBackfillAgeData struct {
	samples          []string
	wantSeries       []map[string]string
	wantQueryResults []*apptest.QueryResult
}

func genMaxBackfillAgeData(prefix string, numMetrics, start, step int64) maxBackfillAgeData {
	samples := make([]string, numMetrics)
	wantSeries := make([]map[string]string, numMetrics)
	wantQueryResults := make([]*apptest.QueryResult, numMetrics)
	for i := range numMetrics {
		metricName := fmt.Sprintf("metric_%s_%04d", prefix, i)
		labelName := fmt.Sprintf("label_%s_%04d", prefix, i)
		labelValue := fmt.Sprintf("value_%s_%04d", prefix, i)
		value := i
		timestamp := start + i*step
		samples[i] = fmt.Sprintf(`%s{%s="value", label="%s"} %d %d`, metricName, labelName, labelValue, value, timestamp)
		wantSeries[i] = map[string]string{
			"__name__": metricName,
			labelName:  "value",
			"label":    labelValue,
		}
		wantQueryResults[i] = &apptest.QueryResult{
			Metric: map[string]string{
				"__name__": metricName,
				labelName:  "value",
				"label":    labelValue,
			},
			Samples: []*apptest.Sample{{Timestamp: timestamp, Value: float64(value)}},
		}
	}
	return maxBackfillAgeData{samples, wantSeries, wantQueryResults}
}
