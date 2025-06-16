package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

var legacyVmsinglePath = os.Getenv("VM_LEGACY_VMSINGLE_PATH")

func TestLegacySingleDeleteSeries(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	type want struct {
		series       []map[string]string
		queryResults []*at.QueryResult
	}

	genData := func(prefix string, start, end, step int64, value float64) (recs []string, w *want) {
		count := (end - start) / step
		recs = make([]string, count)
		w = &want{
			series:       make([]map[string]string, count),
			queryResults: make([]*at.QueryResult, count),
		}
		for i := range count {
			name := fmt.Sprintf("%s_%03d", prefix, i)
			timestamp := start + int64(i)*step

			recs[i] = fmt.Sprintf("%s %f %d", name, value, timestamp)
			w.series[i] = map[string]string{"__name__": name}
			w.queryResults[i] = &at.QueryResult{
				Metric:  map[string]string{"__name__": name},
				Samples: []*at.Sample{{Timestamp: timestamp, Value: value}},
			}
		}
		return recs, w
	}

	assertSearchResults := func(app at.PrometheusQuerier, query string, start, end int64, step string, want *want) {
		t.Helper()

		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, query, at.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
				}).Sort()
			},
			Want: &at.PrometheusAPIV1SeriesResponse{
				Status: "success",
				Data:   want.series,
			},
			FailNow: true,
		})

		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, query, at.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
					Step:  step,
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result:     want.queryResults,
				},
			},
			FailNow: true,
		})
	}

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")

	// startLegacyVmsingle starts and instance of vmsingle that uses legacy
	// indexDB.
	startLegacyVmsingle := func() *at.Vmsingle {
		return tc.MustStartVmsingleAt("vmsingle-legacy", legacyVmsinglePath, []string{
			"-storageDataPath=" + storageDataPath,
			"-retentionPeriod=100y",
			"-search.maxStalenessInterval=1m",
		})
	}

	// startNewVmsingle starts and instance of vmsingle that uses partition
	// indexDBs.
	startNewVmsingle := func() *at.Vmsingle {
		return tc.MustStartVmsingle("vmsingle-new", []string{
			"-storageDataPath=" + storageDataPath,
			"-retentionPeriod=100y",
			"-search.maxStalenessInterval=1m",
		})
	}

	// - start legacy vmsingle
	// - insert data1
	// - confirm that metric names and samples are searcheable
	// - stop legacy vmsingle
	const step = 24 * 3600 * 1000 // 24h
	start1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end1 := time.Date(2000, 1, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	data1, want1 := genData("metric", start1, end1, step, 1)
	legacyVmsingle := startLegacyVmsingle()
	legacyVmsingle.PrometheusAPIV1ImportPrometheus(t, data1, at.QueryOpts{})
	legacyVmsingle.ForceFlush(t)
	assertSearchResults(legacyVmsingle, `{__name__=~".*"}`, start1, end1, "1d", want1)
	tc.StopApp(legacyVmsingle.Name())

	// - start new vmsingle
	// - confirm that data1 metric names and samples are searcheable
	// - delete data1
	// - confirm that data1 metric names and samples are not searcheable anymore
	// - insert data2 (same metric names, different dates)
	// - confirm that metric names become searcheable again
	// - confirm that data1 samples are not searchable and data2 samples are searcheable

	newVmsingle := startNewVmsingle()
	assertSearchResults(newVmsingle, `{__name__=~".*"}`, start1, end1, "1d", want1)

	newVmsingle.APIV1AdminTSDBDeleteSeries(t, `{__name__=~".*"}`, at.QueryOpts{})
	wantNoResults := &want{
		series:       []map[string]string{},
		queryResults: []*at.QueryResult{},
	}
	assertSearchResults(newVmsingle, `{__name__=~".*"}`, start1, end1, "1d", wantNoResults)

	start2 := time.Date(2000, 1, 11, 0, 0, 0, 0, time.UTC).UnixMilli()
	end2 := time.Date(2000, 1, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	data2, want2 := genData("metric", start2, end2, step, 2)
	newVmsingle.PrometheusAPIV1ImportPrometheus(t, data2, at.QueryOpts{})
	newVmsingle.ForceFlush(t)
	assertSearchResults(newVmsingle, `{__name__=~".*"}`, start1, end2, "1d", want2)

	// - restart new vmsingle
	// - confirm that metric names still searchable, data1 samples are not
	//   searchable, and data2 samples are searcheable

	tc.StopApp(newVmsingle.Name())
	newVmsingle = startNewVmsingle()
	assertSearchResults(newVmsingle, `{__name__=~".*"}`, start1, end2, "1d", want2)
}

func TestLegacySingleBackupRestore(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	const msecPerMinute = 60 * 1000
	genData := func(count int, prefix string, start int64) (recs []string, wantSeries []map[string]string, wantQueryResults []*at.QueryResult) {
		recs = make([]string, count)
		wantSeries = make([]map[string]string, count)
		wantQueryResults = make([]*at.QueryResult, count)
		for i := range count {
			name := fmt.Sprintf("%s_%03d", prefix, i)
			value := float64(i)
			timestamp := start + int64(i)*msecPerMinute

			recs[i] = fmt.Sprintf("%s %f %d", name, value, timestamp)
			wantSeries[i] = map[string]string{"__name__": name}
			wantQueryResults[i] = &at.QueryResult{
				Metric:  map[string]string{"__name__": name},
				Samples: []*at.Sample{{Timestamp: timestamp, Value: value}},
			}
		}
		return recs, wantSeries, wantQueryResults
	}

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")
	backupBaseDir, err := filepath.Abs(filepath.Join(tc.Dir(), "backups"))
	if err != nil {
		t.Fatalf("could not get absolute path for the backup base dir")
	}

	// startLegacyVmsingle starts and instance of vmsingle that uses legacy
	// indexDB.
	startLegacyVmsingle := func() *at.Vmsingle {
		return tc.MustStartVmsingleAt("vmsingle-legacy", legacyVmsinglePath, []string{
			"-storageDataPath=" + storageDataPath,
			"-retentionPeriod=100y",
			"-search.maxStalenessInterval=1m",
		})
	}

	// startNewVmsingle starts and instance of vmsingle that uses partition
	// indexDBs.
	startNewVmsingle := func() *at.Vmsingle {
		return tc.MustStartVmsingle("vmsingle-new", []string{
			"-storageDataPath=" + storageDataPath,
			"-retentionPeriod=100y",
			"-search.maxStalenessInterval=1m",
		})
	}

	// assertSeries retrieves set of all metric names from the storage and
	// compares it with the expected set.
	assertSeries := func(app at.PrometheusQuerier, start, end int64, want []map[string]string) {
		t.Helper()

		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/series response",
			Got: func() any {
				return app.PrometheusAPIV1Series(t, `{__name__=~".*"}`, at.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
				}).Sort()
			},
			Want: &at.PrometheusAPIV1SeriesResponse{
				Status: "success",
				Data:   want,
			},
			FailNow: true,
		})
	}

	// assertSeries retrieves all data from the storage and compares it with the
	// expected result.
	assertQueryResults := func(app at.PrometheusQuerier, start, end int64, want []*at.QueryResult) {
		t.Helper()
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, `{__name__=~".*"}`, at.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
					Step:  "60s",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result:     want,
				},
			},
			FailNow: true,
			// The vmsingle with pt index seems to require more retries before
			// the ingested data becomes available for querying.
			Retries: 100,
		})
	}

	// Use the same number of metrics and time range for all the data ingestions
	// below.
	const numMetrics = 1000
	// With 1000 metrics (one per minute), the time range spans 2 months.
	end := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	start := end - numMetrics*msecPerMinute

	// Verify backup/restore with legacy vmsingle:
	//
	// - Start legacy vmsingle with empty storage data dir.
	// - Ingest first batch or records (legacy1) and ensure they can be queried.
	// - Create legacy1 backup
	// - Ingest second batch of records (legacy2) and ensure the queries return
	//   (legacy1 + legacy2) data.
	// - Create legacy2 backup
	// - Stop legacy vmsingle
	// - Restore legacy1 from backup
	// - Start legacy vmsingle
	// - Ensure that the queries return legacy1 data only.
	// - Stop legacy vmsingle

	legacyVmsingle := startLegacyVmsingle()

	legacy1Data, wantLegacy1Series, wantLegacy1QueryResults := genData(numMetrics, "legacy1", start)
	legacyVmsingle.PrometheusAPIV1ImportPrometheus(t, legacy1Data, at.QueryOpts{})
	legacyVmsingle.ForceFlush(t)
	assertSeries(legacyVmsingle, start, end, wantLegacy1Series)
	assertQueryResults(legacyVmsingle, start, end, wantLegacy1QueryResults)

	legacy1Backup := "fs://" + filepath.Join(backupBaseDir, "legacy1")
	tc.MustStartVmbackup("vmbackup-legacy1", storageDataPath, legacyVmsingle.SnapshotCreateURL(), legacy1Backup)

	legacy2Data, wantLegacy2Series, wantLegacy2QueryResults := genData(numMetrics, "legacy2", start)
	legacyVmsingle.PrometheusAPIV1ImportPrometheus(t, legacy2Data, at.QueryOpts{})
	legacyVmsingle.ForceFlush(t)
	wantAllSeries := slices.Concat(wantLegacy1Series, wantLegacy2Series)
	assertSeries(legacyVmsingle, start, end, wantAllSeries)
	wantAllQueryResults := slices.Concat(wantLegacy1QueryResults, wantLegacy2QueryResults)
	assertQueryResults(legacyVmsingle, start, end, wantAllQueryResults)

	legacy2Backup := "fs://" + filepath.Join(backupBaseDir, "legacy2")
	tc.MustStartVmbackup("vmbackup-legacy2", storageDataPath, legacyVmsingle.SnapshotCreateURL(), legacy2Backup)

	tc.StopApp(legacyVmsingle.Name())

	tc.MustStartVmrestore("vmrestore-legacy1", legacy1Backup, storageDataPath)

	legacyVmsingle = startLegacyVmsingle()
	assertSeries(legacyVmsingle, start, end, wantLegacy1Series)
	assertQueryResults(legacyVmsingle, start, end, wantLegacy1QueryResults)

	tc.StopApp(legacyVmsingle.Name())

	// Verify backup/restore with new vmsingle:
	//
	// - Start new vmsingle (with partition indexDBs) with storage containing
	//   legacy1 data.
	// - Ensure that legacy1 data can still be queried.
	// - Ingest first batch or records (new1). Ensure that queries now return
	//   (legacy1+new1) data.
	// - Create a backup
	// - Ingest second batch of records (new2). Ensure that queries now return
	//   (legacy1+new1+new2) data.
	// - Create a backup
	// - Stop new vmsingle
	// - Restore legacy1 from backup, start new vmsingle, and ensure the
	//   storage does not have legacy2 data
	// - Stop new vmsingle
	// - Restore from (legacy1+new1) backup
	// - Start new vmsingle
	// - Ensure that queries now return (legacy1+new1) data only
	// - Stop new vmsingle

	newVmsingle := startNewVmsingle()

	assertSeries(newVmsingle, start, end, wantLegacy1Series)
	assertQueryResults(newVmsingle, start, end, wantLegacy1QueryResults)

	new1Data, wantNew1Series, wantNew1QueryResults := genData(numMetrics, "new1", start)
	newVmsingle.PrometheusAPIV1ImportPrometheus(t, new1Data, at.QueryOpts{})
	newVmsingle.ForceFlush(t)
	wantAllSeries = slices.Concat(wantLegacy1Series, wantNew1Series)
	assertSeries(newVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults)
	assertQueryResults(newVmsingle, start, end, wantAllQueryResults)

	new1Backup := "fs://" + filepath.Join(backupBaseDir, "new1")
	tc.MustStartVmbackup("vmbackup-new1", storageDataPath, newVmsingle.SnapshotCreateURL(), new1Backup)

	new2Data, wantNew2Series, wantNew2QueryResults := genData(numMetrics, "new2", start)
	newVmsingle.PrometheusAPIV1ImportPrometheus(t, new2Data, at.QueryOpts{})
	newVmsingle.ForceFlush(t)
	wantAllSeries = slices.Concat(wantLegacy1Series, wantNew1Series, wantNew2Series)
	assertSeries(newVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults, wantNew2QueryResults)
	assertQueryResults(newVmsingle, start, end, wantAllQueryResults)

	new2Backup := "fs://" + filepath.Join(backupBaseDir, "new2")
	tc.MustStartVmbackup("vmbackup-new2", storageDataPath, newVmsingle.SnapshotCreateURL(), new2Backup)

	tc.StopApp(newVmsingle.Name())

	tc.MustStartVmrestore("vmrestore-new1", new1Backup, storageDataPath)

	newVmsingle = startNewVmsingle()
	wantAllSeries = slices.Concat(wantLegacy1Series, wantNew1Series)
	assertSeries(newVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults)
	assertQueryResults(newVmsingle, start, end, wantAllQueryResults)

	tc.StopApp(newVmsingle.Name())

	// Verify backup/restore with legacy vmsingle again:
	//
	// - Start legacy vmsingle with storage containing (legacy1 + new1) data
	// - Ensure that the SearchMetricNames() queries return legacy1 data only.
	//   new1 data is not returned because legacy vmsingle does not know about
	//   partition indexDBs.
	// - Ensure that query_range queries return both legacy1 and new1 data. This
	//   is because the samples are stored the same way in both legacy and new
	//   storage and the corresponding metric names are retrieved from the
	//   metricID -> metricName cache, not from indexDB.
	// - Stop legacy vmsingle
	// - Restore from legacy2 backup
	// - Start legacy vmsingle
	// - Ensure that queries now return (legacy1 + legacy2) data.
	// - Stop legacy vmsingle

	legacyVmsingle = startLegacyVmsingle()

	assertSeries(legacyVmsingle, start, end, wantLegacy1Series)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults)
	assertQueryResults(legacyVmsingle, start, end, wantAllQueryResults)

	tc.StopApp(legacyVmsingle.Name())

	tc.MustStartVmrestore("vmrestore-legacy2", legacy2Backup, storageDataPath)

	legacyVmsingle = startLegacyVmsingle()

	wantAllSeries = slices.Concat(wantLegacy1Series, wantLegacy2Series)
	assertSeries(legacyVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantLegacy2QueryResults)
	assertQueryResults(legacyVmsingle, start, end, wantAllQueryResults)

	tc.StopApp(legacyVmsingle.Name())

	// Verify backup/restore with new vmsingle again:
	//
	// - Start new vmsingle with storage containing (legacy1 + legacy2) data
	// - Ensure that queries return (legacy1 + legacy2) data
	// - Stop new vmsingle
	// - Restore from new2 backup
	// - Start new vmsingle
	// - Ensure that queries return (legacy1 + new1, new2) data
	// - Stop new vmsingle

	newVmsingle = startNewVmsingle()

	wantAllSeries = slices.Concat(wantLegacy1Series, wantLegacy2Series)
	assertSeries(newVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantLegacy2QueryResults)
	assertQueryResults(newVmsingle, start, end, wantAllQueryResults)

	tc.StopApp(newVmsingle.Name())

	tc.MustStartVmrestore("vmrestore-new2", new2Backup, storageDataPath)

	newVmsingle = startNewVmsingle()

	wantAllSeries = slices.Concat(wantLegacy1Series, wantNew1Series, wantNew2Series)
	assertSeries(newVmsingle, start, end, wantAllSeries)
	wantAllQueryResults = slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults, wantNew2QueryResults)
	assertQueryResults(newVmsingle, start, end, wantAllQueryResults)

	tc.StopApp(legacyVmsingle.Name())
}
