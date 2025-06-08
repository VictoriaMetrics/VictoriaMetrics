package tests

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

type testBackupRestoreOpts struct {
	startSUT           func() at.PrometheusWriteQuerier
	stopSUT            func()
	storageDataPaths   []string
	snapshotCreateURLs func(at.PrometheusWriteQuerier) []string
}

func TestSingleBackupRestore(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")

	opts := testBackupRestoreOpts{
		startSUT: func() at.PrometheusWriteQuerier {
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
		snapshotCreateURLs: func(sut at.PrometheusWriteQuerier) []string {
			return []string{
				sut.(*at.Vmsingle).SnapshotCreateURL(),
			}
		},
	}

	testBackupRestore(tc, opts)
}

func TestClusterBackupRestore(t *testing.T) {
	tc := at.NewTestCase(t)
	defer tc.Stop()

	storage1DataPath := filepath.Join(tc.Dir(), "vmstorage1")
	storage2DataPath := filepath.Join(tc.Dir(), "vmstorage2")

	opts := testBackupRestoreOpts{
		startSUT: func() at.PrometheusWriteQuerier {
			return tc.MustStartCluster(&at.ClusterOptions{
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
		snapshotCreateURLs: func(sut at.PrometheusWriteQuerier) []string {
			c := sut.(*at.Vmcluster)
			return []string{
				c.Vmstorages[0].SnapshotCreateURL(),
				c.Vmstorages[1].SnapshotCreateURL(),
			}
		},
	}

	testBackupRestore(tc, opts)
}

func testBackupRestore(tc *at.TestCase, opts testBackupRestoreOpts) {
	t := tc.T()

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

	backupBaseDir, err := filepath.Abs(filepath.Join(tc.Dir(), "backups"))
	if err != nil {
		t.Fatalf("could not get absolute path for the backup base dir")
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
			Retries: 300,
		})
	}

	createBackup := func(sut at.PrometheusWriteQuerier, name string) {
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
	const numMetrics = 10
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

	sut.PrometheusAPIV1ImportPrometheus(t, batch1Data, at.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, start, end, wantBatch1Series)
	assertQueryResults(sut, start, end, wantBatch1QueryResults)
	createBackup(sut, "batch1")

	sut.PrometheusAPIV1ImportPrometheus(t, batch2Data, at.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, start, end, wantBatch12Series)
	assertQueryResults(sut, start, end, wantBatch12QueryResults)
	createBackup(sut, "batch2")

	opts.stopSUT()

	restoreFromBackup("batch1")

	sut = opts.startSUT()

	assertSeries(sut, start, end, wantBatch1Series)
	assertQueryResults(sut, start, end, wantBatch1QueryResults)
}
