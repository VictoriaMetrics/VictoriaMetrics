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

var (
	legacyVmsinglePath  = os.Getenv("VM_LEGACY_VMSINGLE_PATH")
	legacyVmstoragePath = os.Getenv("VM_LEGACY_VMSTORAGE_PATH")
)

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

type testLegacyBackupRestoreOpts struct {
	startLegacySUT     func() at.PrometheusWriteQuerier
	startNewSUT        func() at.PrometheusWriteQuerier
	stopLegacySUT      func()
	stopNewSUT         func()
	storageDataPaths   []string
	snapshotCreateURLs func(at.PrometheusWriteQuerier) []string
}

func TestLegacySingleBackupRestore(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")

	opts := testLegacyBackupRestoreOpts{
		startLegacySUT: func() at.PrometheusWriteQuerier {
			return tc.MustStartVmsingleAt("vmsingle-legacy", legacyVmsinglePath, []string{
				"-storageDataPath=" + storageDataPath,
				"-retentionPeriod=100y",
				"-search.disableCache=true",
				"-search.maxStalenessInterval=1m",
			})
		},
		startNewSUT: func() at.PrometheusWriteQuerier {
			return tc.MustStartVmsingle("vmsingle-new", []string{
				"-storageDataPath=" + storageDataPath,
				"-retentionPeriod=100y",
				"-search.disableCache=true",
				"-search.maxStalenessInterval=1m",
			})
		},
		stopLegacySUT: func() {
			tc.StopApp("vmsingle-legacy")
		},
		stopNewSUT: func() {
			tc.StopApp("vmsingle-new")
		},
		storageDataPaths: []string{
			storageDataPath,
		},
		snapshotCreateURLs: func(sut at.PrometheusWriteQuerier) []string {
			return []string{
				sut.(*at.Vmsingle).SnapshotCreateURL(),
			}
		},
	}

	testLegacyBackupRestore(tc, opts)
}

func TestLegacyClusterBackupRestore(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	storage1DataPath := filepath.Join(tc.Dir(), "vmstorage1")
	storage2DataPath := filepath.Join(tc.Dir(), "vmstorage2")

	opts := testLegacyBackupRestoreOpts{
		startLegacySUT: func() at.PrometheusWriteQuerier {
			return tc.MustStartCluster(&at.ClusterOptions{
				Vmstorage1Instance: "vmstorage1-legacy",
				Vmstorage1Binary:   legacyVmstoragePath,
				Vmstorage1Flags: []string{
					"-storageDataPath=" + storage1DataPath,
					"-retentionPeriod=100y",
				},
				Vmstorage2Instance: "vmstorage2-legacy",
				Vmstorage2Binary:   legacyVmstoragePath,
				Vmstorage2Flags: []string{
					"-storageDataPath=" + storage2DataPath,
					"-retentionPeriod=100y",
				},
				VminsertInstance: "vminsert",
				VminsertFlags:    []string{},
				VmselectInstance: "vmselect",
				VmselectFlags: []string{
					"-search.disableCache=true",
					"-search.maxStalenessInterval=1m",
				},
			})
		},
		startNewSUT: func() at.PrometheusWriteQuerier {
			return tc.MustStartCluster(&at.ClusterOptions{
				Vmstorage1Instance: "vmstorage1-new",
				Vmstorage1Flags: []string{
					"-storageDataPath=" + storage1DataPath,
					"-retentionPeriod=100y",
				},
				Vmstorage2Instance: "vmstorage2-new",
				Vmstorage2Flags: []string{
					"-storageDataPath=" + storage2DataPath,
					"-retentionPeriod=100y",
				},
				VminsertInstance: "vminsert",
				VminsertFlags:    []string{},
				VmselectInstance: "vmselect",
				VmselectFlags: []string{
					"-search.disableCache=true",
					"-search.maxStalenessInterval=1m",
				},
			})
		},
		stopLegacySUT: func() {
			tc.StopApp("vminsert")
			tc.StopApp("vmselect")
			tc.StopApp("vmstorage1-legacy")
			tc.StopApp("vmstorage2-legacy")
		},
		stopNewSUT: func() {
			tc.StopApp("vminsert")
			tc.StopApp("vmselect")
			tc.StopApp("vmstorage1-new")
			tc.StopApp("vmstorage2-new")
		},
		storageDataPaths: []string{
			storage1DataPath,
			storage2DataPath,
		},
		snapshotCreateURLs: func(sut at.PrometheusWriteQuerier) []string {
			c := sut.(*at.Vmcluster)
			return []string{
				c.Vmstorages[0].SnapshotCreateURL(),
				c.Vmstorages[1].SnapshotCreateURL(),
			}
		},
	}

	testLegacyBackupRestore(tc, opts)
}

func testLegacyBackupRestore(tc *at.TestCase, opts testLegacyBackupRestoreOpts) {
	t := tc.T()

	const msecPerMinute = 60 * 1000
	// Use the same number of metrics and and time range for all the data ingestions
	// below.
	const numMetrics = 1000
	start := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC).Add(-numMetrics * time.Minute).UnixMilli()
	end := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	genData := func(prefix string) (recs []string, wantSeries []map[string]string, wantQueryResults []*at.QueryResult) {
		recs = make([]string, numMetrics)
		wantSeries = make([]map[string]string, numMetrics)
		wantQueryResults = make([]*at.QueryResult, numMetrics)
		for i := range numMetrics {
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

	backupBaseDir, err := filepath.Abs(filepath.Join(tc.Dir(), "backups"))
	if err != nil {
		t.Fatalf("could not get absolute path for the backup base dir")
	}

	// assertSeries issues various queries to the app and compares the query
	// results with the expected ones.
	assertQueries := func(app at.PrometheusQuerier, query string, wantSeries []map[string]string, wantQueryResults []*at.QueryResult) {
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
				Data:   wantSeries,
			},
			FailNow: true,
		})

		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, query, at.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
					Step:  "60s",
				})
			},
			Want: &at.PrometheusAPIV1QueryResponse{
				Status: "success",
				Data: &at.QueryData{
					ResultType: "matrix",
					Result:     wantQueryResults,
				},
			},
			Retries: 300,
			FailNow: true,
		})
	}

	createBackup := func(sut at.PrometheusWriteQuerier, name string) {
		t.Helper()
		for i, storageDataPath := range opts.storageDataPaths {
			replica := fmt.Sprintf("replica-%d", i)
			instance := fmt.Sprintf("vmbackup-%s-%s", name, replica)
			snapshotCreateURL := opts.snapshotCreateURLs(sut)[i]
			backupPath := "fs://" + filepath.Join(backupBaseDir, name, replica)
			tc.MustStartVmbackup(instance, storageDataPath, snapshotCreateURL, backupPath)
		}
	}

	restoreFromBackup := func(name string) {
		t.Helper()
		for i, storageDataPath := range opts.storageDataPaths {
			replica := fmt.Sprintf("replica-%d", i)
			instance := fmt.Sprintf("vmrestore-%s-%s", name, replica)
			backupPath := "fs://" + filepath.Join(backupBaseDir, name, replica)
			tc.MustStartVmrestore(instance, backupPath, storageDataPath)
		}
	}

	legacy1Data, wantLegacy1Series, wantLegacy1QueryResults := genData("legacy1")
	legacy2Data, wantLegacy2Series, wantLegacy2QueryResults := genData("legacy2")
	new1Data, wantNew1Series, wantNew1QueryResults := genData("new1")
	new2Data, wantNew2Series, wantNew2QueryResults := genData("new2")
	wantLegacy12Series := slices.Concat(wantLegacy1Series, wantLegacy2Series)
	wantLegacy12QueryResults := slices.Concat(wantLegacy1QueryResults, wantLegacy2QueryResults)
	wantLegacy1New1Series := slices.Concat(wantLegacy1Series, wantNew1Series)
	wantLegacy1New1QueryResults := slices.Concat(wantLegacy1QueryResults, wantNew1QueryResults)
	wantLegacy1New12Series := slices.Concat(wantLegacy1New1Series, wantNew2Series)
	wantLegacy1New12QueryResults := slices.Concat(wantLegacy1New1QueryResults, wantNew2QueryResults)
	var legacySUT, newSUT at.PrometheusWriteQuerier

	// Verify backup/restore with legacy SUT.

	// Start legacy SUT with empty storage data dir.
	legacySUT = opts.startLegacySUT()

	// Ingest legacy1 records, ensure the queries return legacy1, and create
	// legacy1 backup.
	legacySUT.PrometheusAPIV1ImportPrometheus(t, legacy1Data, at.QueryOpts{})
	legacySUT.ForceFlush(t)
	assertQueries(legacySUT, `{__name__=~".*"}`, wantLegacy1Series, wantLegacy1QueryResults)
	createBackup(legacySUT, "legacy1")

	// Ingest legacy2 records, ensure the queries return legacy1+legacy2, and
	// create legacy1+legacy2 backup.
	legacySUT.PrometheusAPIV1ImportPrometheus(t, legacy2Data, at.QueryOpts{})
	legacySUT.ForceFlush(t)
	assertQueries(legacySUT, `{__name__=~"legacy.*"}`, wantLegacy12Series, wantLegacy12QueryResults)
	createBackup(legacySUT, "legacy12")

	// Stop legacy SUT and restore legacy1 data.
	// Start legacy SUT and ensure the queries return legacy1.
	opts.stopLegacySUT()
	restoreFromBackup("legacy1")
	legacySUT = opts.startLegacySUT()
	assertQueries(legacySUT, `{__name__=~".*"}`, wantLegacy1Series, wantLegacy1QueryResults)

	opts.stopLegacySUT()

	// Verify backup/restore with new SUT.

	// Start new SUT (with partition indexDBs) with storage containing legacy1
	// data and Ensure that queries return legacy1 data.
	newSUT = opts.startNewSUT()
	assertQueries(newSUT, `{__name__=~".*"}`, wantLegacy1Series, wantLegacy1QueryResults)

	// Ingest new1 records, ensure that queries now return legacy1+new1, and
	// create the legacy1+new1 backup.
	newSUT.PrometheusAPIV1ImportPrometheus(t, new1Data, at.QueryOpts{})
	newSUT.ForceFlush(t)
	assertQueries(newSUT, `{__name__=~"(legacy|new).*"}`, wantLegacy1New1Series, wantLegacy1New1QueryResults)
	createBackup(newSUT, "legacy1-new1")

	// Ingest new2 records, ensure that queries now return legacy1+new1+new2,
	// and create the legacy1+new1+new2 backup.
	newSUT.PrometheusAPIV1ImportPrometheus(t, new2Data, at.QueryOpts{})
	newSUT.ForceFlush(t)
	assertQueries(newSUT, `{__name__=~"(legacy|new1|new2).*"}`, wantLegacy1New12Series, wantLegacy1New12QueryResults)
	createBackup(newSUT, "legacy1-new12")

	// Stop new SUT and restore legacy1+new1 data.
	// Start new SUT and ensure queries return legacy1+new1 data.
	opts.stopNewSUT()
	restoreFromBackup("legacy1-new1")
	newSUT = opts.startNewSUT()
	assertQueries(newSUT, `{__name__=~".*"}`, wantLegacy1New1Series, wantLegacy1New1QueryResults)

	opts.stopNewSUT()

	// Verify backup/restore with legacy SUT again.

	// Start legacy SUT with storage containing legacy1+new1 data.
	//
	// Ensure that the /series and /query_range queries return legacy1 data only.
	// new1 data is not returned because legacy vmsingle does not know about
	// partition indexDBs.
	legacySUT = opts.startLegacySUT()
	assertQueries(legacySUT, `{__name__=~".*"}`, wantLegacy1Series, wantLegacy1QueryResults)

	// Stop legacy SUT and restore legacy1+legacy2 data.
	// Start legacy SUT and ensure that queries now return legacy1+legacy2 data.
	opts.stopLegacySUT()
	restoreFromBackup("legacy12")
	legacySUT = opts.startLegacySUT()
	assertQueries(legacySUT, `{__name__=~".*"}`, wantLegacy12Series, wantLegacy12QueryResults)

	opts.stopLegacySUT()

	// Verify backup/restore with new vmsingle again.

	// Start new vmsingle with storage containing legacy1+legacy2 data and
	// ensure that queries return legacy1+legacy2 data.
	newSUT = opts.startNewSUT()
	assertQueries(newSUT, `{__name__=~".*"}`, wantLegacy12Series, wantLegacy12QueryResults)

	// Stop new SUT and restore legacy1+new1+new2 data.
	// Start new SUT and ensure that queries return legacy1+new1+new2 data.
	opts.stopNewSUT()
	restoreFromBackup("legacy1-new12")
	newSUT = opts.startNewSUT()
	assertQueries(newSUT, `{__name__=~"(legacy|new).*"}`, wantLegacy1New12Series, wantLegacy1New12QueryResults)

	opts.stopNewSUT()
}
