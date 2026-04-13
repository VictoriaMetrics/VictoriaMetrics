package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleFutureTimestamps(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	opts := testFutureTimestampsOpts{
		start: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartDefaultVmsingle()
		},
		stop: func(sut apptest.PrometheusWriteQuerier) {
			vmsingle := sut.(*apptest.Vmsingle)
			tc.StopApp(vmsingle.Name())
		},
	}

	testFutureTimestamps(tc, opts)
}

func TestClusterFutureTimestamps(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	opts := testFutureTimestampsOpts{
		start: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartDefaultCluster()
		},
		stop: func(sut apptest.PrometheusWriteQuerier) {
			vmcluster := sut.(*apptest.Vmcluster)
			tc.StopApp(vmcluster.Vminsert.Name())
			tc.StopApp(vmcluster.Vmselect.Name())
			for _, vmstorage := range vmcluster.Vmstorages {
				tc.StopApp(vmstorage.Name())
			}
		},
	}

	testFutureTimestamps(tc, opts)
}

type testFutureTimestampsOpts struct {
	start func() apptest.PrometheusWriteQuerier
	stop  func(sut apptest.PrometheusWriteQuerier)
}

func testFutureTimestamps(tc *apptest.TestCase, opts testFutureTimestampsOpts) {
	t := tc.T()

	// assertSeries retrieves set of all metric names from the storage and
	// compares it with the expected set.
	assertSeries := func(app apptest.PrometheusQuerier, start, end int64, want []map[string]string) {
		t.Helper()

		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, apptest.QueryOpts{
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

	// assertSeries retrieves all data from the storage and compares it with the
	// expected result.
	assertQueryResults := func(app apptest.PrometheusQuerier, start, end, step int64, want []*apptest.QueryResult) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, `{__name__=~".*"}`, apptest.QueryOpts{
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

	f := func(prefix string, startTime, endTime time.Time) {
		const numMetrics = 1000
		start := startTime.UnixMilli()
		end := endTime.UnixMilli()
		step := (end - start) / numMetrics
		data := genFutureTimestampsData(prefix, numMetrics, start, step)

		// Ingest data and check query results.
		sut := opts.start()
		sut.PrometheusAPIV1ImportPrometheus(t, data.samples, apptest.QueryOpts{})
		sut.ForceFlush(t)
		assertSeries(sut, start, end, data.wantSeries)
		assertQueryResults(sut, start, end, step, data.wantQueryResults)

		// Ensure the queries work after restrart.
		opts.stop(sut)
		sut = opts.start()
		assertSeries(sut, start, end, data.wantSeries)
		assertQueryResults(sut, start, end, step, data.wantQueryResults)

		opts.stop(sut)
	}

	now := time.Now().UTC()
	var start, end time.Time

	start = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	end = time.Date(now.Year(), now.Month(), now.Day()+2, 0, 0, 0, 0, time.UTC)
	f("future_1d", start, end)

	start = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, 0, time.UTC)
	f("future_1m", start, end)

	start = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(now.Year()+2, 1, 1, 0, 0, 0, 0, time.UTC)
	f("future_1y", start, end)

	start = time.Date(2262, 3, 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(2262, 4, 1, 0, 0, 0, 0, time.UTC)
	f("future_last_month", start, end)
}

type futureTimestampsData struct {
	samples          []string
	wantSeries       []map[string]string
	wantQueryResults []*apptest.QueryResult
}

func genFutureTimestampsData(prefix string, numMetrics, start, step int64) futureTimestampsData {
	samples := make([]string, numMetrics)
	wantSeries := make([]map[string]string, numMetrics)
	wantQueryResults := make([]*apptest.QueryResult, numMetrics)
	for i := range numMetrics {
		metricName := fmt.Sprintf("metric_%s_%04d", prefix, i)
		labelName := fmt.Sprintf("label_%s_%04d", prefix, i)
		labelValue := fmt.Sprintf("value_%s_%04d", prefix, i)
		value := i
		timestamp := start + i*step
		samples[i] = fmt.Sprintf("%s{%s=\"value\", label=\"%s\"} %d %d", metricName, labelName, labelValue, value, timestamp)
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
	return futureTimestampsData{samples, wantSeries, wantQueryResults}
}
