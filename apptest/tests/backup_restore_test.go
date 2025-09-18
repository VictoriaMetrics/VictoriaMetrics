package tests

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

type testBackupRestoreOpts struct {
	startSUT           func() apptest.PrometheusWriteQuerier
	stopSUT            func()
	storageDataPaths   []string
	snapshotCreateURLs func(apptest.PrometheusWriteQuerier) []string
}

func TestSingleBackupRestore(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")

	opts := testBackupRestoreOpts{
		startSUT: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartVmsingle("vmsingle", []string{
				"-storageDataPath=" + storageDataPath,
				"-retentionPeriod=100y",
				"-search.maxStalenessInterval=1m",
			})
		},
		stopSUT: func() {
			tc.StopApp("vmsingle")
		},
		storageDataPaths: []string{
			storageDataPath,
		},
		snapshotCreateURLs: func(sut apptest.PrometheusWriteQuerier) []string {
			return []string{
				sut.(*apptest.Vmsingle).SnapshotCreateURL(),
			}
		},
	}

	testBackupRestore(tc, opts)
}

func TestClusterBackupRestore(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	storage1DataPath := filepath.Join(tc.Dir(), "vmstorage1")
	storage2DataPath := filepath.Join(tc.Dir(), "vmstorage2")

	opts := testBackupRestoreOpts{
		startSUT: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartCluster(&apptest.ClusterOptions{
				Vmstorage1Instance: "vmstorage1",
				Vmstorage1Flags: []string{
					"-storageDataPath=" + storage1DataPath,
					"-retentionPeriod=100y",
				},
				Vmstorage2Instance: "vmstorage2",
				Vmstorage2Flags: []string{
					"-storageDataPath=" + storage2DataPath,
					"-retentionPeriod=100y",
				},
				VminsertInstance: "vminsert",
				VminsertFlags:    []string{},
				VmselectInstance: "vmselect",
				VmselectFlags: []string{
					"-search.maxStalenessInterval=1m",
				},
			})
		},
		stopSUT: func() {
			tc.StopApp("vminsert")
			tc.StopApp("vmselect")
			tc.StopApp("vmstorage1")
			tc.StopApp("vmstorage2")
		},
		storageDataPaths: []string{
			storage1DataPath,
			storage2DataPath,
		},
		snapshotCreateURLs: func(sut apptest.PrometheusWriteQuerier) []string {
			c := sut.(*apptest.Vmcluster)
			return []string{
				c.Vmstorages[0].SnapshotCreateURL(),
				c.Vmstorages[1].SnapshotCreateURL(),
			}
		},
	}

	testBackupRestore(tc, opts)
}

func testBackupRestore(tc *apptest.TestCase, opts testBackupRestoreOpts) {
	t := tc.T()

	const msecPerMinute = 60 * 1000
	genData := func(count int, prefix string, start int64) (recs []string, wantSeries []map[string]string, wantQueryResults []*apptest.QueryResult) {
		recs = make([]string, count)
		wantSeries = make([]map[string]string, count)
		wantQueryResults = make([]*apptest.QueryResult, count)
		for i := range count {
			name := fmt.Sprintf("%s_%03d", prefix, i)
			value := float64(i)
			timestamp := start + int64(i)*msecPerMinute

			recs[i] = fmt.Sprintf("%s %f %d", name, value, timestamp)
			wantSeries[i] = map[string]string{"__name__": name}
			wantQueryResults[i] = &apptest.QueryResult{
				Metric:  map[string]string{"__name__": name},
				Samples: []*apptest.Sample{{Timestamp: timestamp, Value: value}},
			}
		}
		return recs, wantSeries, wantQueryResults
	}

	backupBaseDir, err := filepath.Abs(filepath.Join(tc.Dir(), "backups"))
	if err != nil {
		t.Fatalf("could not get absolute path for the backup base dir")
	}

	// assertSeries retrieves set of all metric names from the storage and
	// compares it with the expected set.
	assertSeries := func(app apptest.PrometheusQuerier, query string, start, end int64, want []map[string]string) {
		t.Helper()

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

	// assertSeries retrieves all data from the storage and compares it with the
	// expected result.
	assertQueryResults := func(app apptest.PrometheusQuerier, query string, start, end int64, want []*apptest.QueryResult) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query_range response",
			Got: func() any {
				return app.PrometheusAPIV1QueryRange(t, query, apptest.QueryOpts{
					Start: fmt.Sprintf("%d", start),
					End:   fmt.Sprintf("%d", end),
					Step:  "60s",
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
			Retries: 300,
		})
	}

	createBackup := func(sut apptest.PrometheusWriteQuerier, name string) {
		for i, storageDataPath := range opts.storageDataPaths {
			replica := fmt.Sprintf("replica-%d", i)
			instance := fmt.Sprintf("vmbackup-%s-%s", name, replica)
			snapshotCreateURL := opts.snapshotCreateURLs(sut)[i]
			backupPath := "fs://" + filepath.Join(backupBaseDir, name, replica)
			tc.MustStartVmbackup(instance, storageDataPath, snapshotCreateURL, backupPath)
		}
	}

	restoreFromBackup := func(name string) {
		for i, storageDataPath := range opts.storageDataPaths {
			replica := fmt.Sprintf("replica-%d", i)
			instance := fmt.Sprintf("vmrestore-%s-%s", name, replica)
			backupPath := "fs://" + filepath.Join(backupBaseDir, name, replica)
			tc.MustStartVmrestore(instance, backupPath, storageDataPath)
		}
	}

	// Use the same number of metrics and time range for all the data ingestions
	// below.
	const numMetrics = 1000
	// With 1000 metrics (one per minute), the time range spans 2 months.
	end := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	start := end - numMetrics*msecPerMinute

	// Verify backup/restore:
	//
	// - Start vmsingle with empty storage data dir.
	// - Ingest first batch or records (batch1) and ensure they can be queried.
	// - Create batch1 backup
	// - Ingest second batch of records (batch2) and ensure the queries return
	//   (batch1 + batch2) data.
	// - Stop vmsingle
	// - Restore batch1 from backup
	// - Start vmsingle
	// - Ensure that the queries return batch1 data only.

	batch1Data, wantBatch1Series, wantBatch1QueryResults := genData(numMetrics, "batch1", start)
	batch2Data, wantBatch2Series, wantBatch2QueryResults := genData(numMetrics, "batch2", start)
	wantBatch12Series := slices.Concat(wantBatch1Series, wantBatch2Series)
	wantBatch12QueryResults := slices.Concat(wantBatch1QueryResults, wantBatch2QueryResults)

	sut := opts.startSUT()

	sut.PrometheusAPIV1ImportPrometheus(t, batch1Data, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, `{__name__=~"batch1.*"}`, start, end, wantBatch1Series)
	assertQueryResults(sut, `{__name__=~"batch1.*"}`, start, end, wantBatch1QueryResults)
	createBackup(sut, "batch1")

	sut.PrometheusAPIV1ImportPrometheus(t, batch2Data, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, `{__name__=~"batch(1|2).*"}`, start, end, wantBatch12Series)
	assertQueryResults(sut, `{__name__=~"batch(1|2).*"}`, start, end, wantBatch12QueryResults)
	createBackup(sut, "batch12")

	opts.stopSUT()

	restoreFromBackup("batch1")

	sut = opts.startSUT()

	assertSeries(sut, `{__name__=~"batch1.*"}`, start, end, wantBatch1Series)
	assertQueryResults(sut, `{__name__=~"batch1.*"}`, start, end, wantBatch1QueryResults)
}
