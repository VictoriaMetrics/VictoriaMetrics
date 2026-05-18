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
				"-futureRetention=2y",
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
					"-futureRetention=2y",
				},
				Vmstorage2Instance: "vmstorage2",
				Vmstorage2Flags: []string{
					"-storageDataPath=" + storage2DataPath,
					"-retentionPeriod=100y",
					"-futureRetention=2y",
				},
				VminsertInstance: "vminsert",
				VminsertFlags:    []string{},
				VmselectInstance: "vmselect",
				VmselectFlags:    []string{},
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

	type data struct {
		samples          []string
		wantSeries       []map[string]string
		wantQueryResults []*apptest.QueryResult
	}

	genData := func(count int, prefix string, start, step int64) data {
		recs := make([]string, count)
		wantSeries := make([]map[string]string, count)
		wantQueryResults := make([]*apptest.QueryResult, count)
		for i := range count {
			name := fmt.Sprintf("%s_%03d", prefix, i)
			value := float64(i)
			timestamp := start + int64(i)*step

			recs[i] = fmt.Sprintf("%s %f %d", name, value, timestamp)
			wantSeries[i] = map[string]string{"__name__": name}
			wantQueryResults[i] = &apptest.QueryResult{
				Metric:  map[string]string{"__name__": name},
				Samples: []*apptest.Sample{{Timestamp: timestamp, Value: value}},
			}
		}
		return data{recs, wantSeries, wantQueryResults}
	}

	concatData := func(d1, d2 data) data {
		var d data
		d.samples = slices.Concat(d1.samples, d2.samples)
		d.wantSeries = slices.Concat(d1.wantSeries, d2.wantSeries)
		d.wantQueryResults = slices.Concat(d1.wantQueryResults, d2.wantQueryResults)
		return d
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
	assertQueryResults := func(app apptest.PrometheusQuerier, query string, start, end, step int64, want []*apptest.QueryResult) {
		t.Helper()
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
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	step := (end - start) / numMetrics
	batch1 := genData(numMetrics, "batch1", start, step)
	batch2 := genData(numMetrics, "batch2", start, step)
	batches12 := concatData(batch1, batch2)

	now := time.Now().UTC()
	startFuture := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	endFuture := time.Date(now.Year()+1, 3, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	stepFuture := (endFuture - startFuture) / numMetrics
	batch1Future := genData(numMetrics, "batch1", startFuture, stepFuture)
	batch2Future := genData(numMetrics, "batch2", startFuture, stepFuture)
	batches12Future := concatData(batch1Future, batch2Future)

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

	sut := opts.startSUT()

	sut.PrometheusAPIV1ImportPrometheus(t, batch1.samples, apptest.QueryOpts{})
	sut.PrometheusAPIV1ImportPrometheus(t, batch1Future.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, `{__name__=~"batch1.*"}`, start, end, batch1.wantSeries)
	assertSeries(sut, `{__name__=~"batch1.*"}`, startFuture, endFuture, batch1Future.wantSeries)
	assertQueryResults(sut, `{__name__=~"batch1.*"}`, start, end, step, batch1.wantQueryResults)
	assertQueryResults(sut, `{__name__=~"batch1.*"}`, startFuture, endFuture, stepFuture, batch1Future.wantQueryResults)

	createBackup(sut, "batch1")

	sut.PrometheusAPIV1ImportPrometheus(t, batch2.samples, apptest.QueryOpts{})
	sut.PrometheusAPIV1ImportPrometheus(t, batch2Future.samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, `{__name__=~"batch(1|2).*"}`, start, end, batches12.wantSeries)
	assertSeries(sut, `{__name__=~"batch(1|2).*"}`, startFuture, endFuture, batches12Future.wantSeries)
	assertQueryResults(sut, `{__name__=~"batch(1|2).*"}`, start, end, step, batches12.wantQueryResults)
	assertQueryResults(sut, `{__name__=~"batch(1|2).*"}`, startFuture, endFuture, stepFuture, batches12Future.wantQueryResults)
	createBackup(sut, "batch12")

	opts.stopSUT()

	restoreFromBackup("batch1")

	sut = opts.startSUT()

	assertSeries(sut, `{__name__=~"batch(1|2).*"}`, start, end, batch1.wantSeries)
	assertSeries(sut, `{__name__=~"batch(1|2).*"}`, startFuture, endFuture, batch1Future.wantSeries)
	assertQueryResults(sut, `{__name__=~"batch(1|2).*"}`, start, end, step, batch1.wantQueryResults)
	assertQueryResults(sut, `{__name__=~"batch(1|2).*"}`, startFuture, endFuture, stepFuture, batch1Future.wantQueryResults)
}
