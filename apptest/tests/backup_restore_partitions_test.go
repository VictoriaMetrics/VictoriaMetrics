package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestSingleBackupRestorePartitions verifies that vmrestore -restorePartitions
// restores only the selected monthly partitions instead of the whole backup.
//
// The test ingests one sample per month into three distinct monthly partitions
// (2025_01, 2025_02, 2025_03), creates a backup and then restores subsets of
// partitions into a fresh storage, asserting that only the selected partitions'
// data is queryable.
func TestSingleBackupRestorePartitions(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")
	backupBaseDir, err := filepath.Abs(filepath.Join(tc.Dir(), "backups"))
	if err != nil {
		t.Fatalf("could not get absolute path for the backup base dir: %v", err)
	}
	backupPath := "fs://" + filepath.Join(backupBaseDir, "full")

	startSUT := func() *apptest.Vmsingle {
		return tc.MustStartVmsingle("vmsingle", []string{
			"-storageDataPath=" + storageDataPath,
			"-retentionPeriod=100y",
			"-futureRetention=2y",
		})
	}

	// One sample per month, each landing in its own monthly partition.
	jan := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	feb := time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	mar := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC).UnixMilli()
	samples := []string{
		fmt.Sprintf("m_jan 1 %d", jan),
		fmt.Sprintf("m_feb 2 %d", feb),
		fmt.Sprintf("m_mar 3 %d", mar),
	}

	rangeStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	rangeEnd := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	assertSeries := func(sut *apptest.Vmsingle, msg string, want []map[string]string) {
		t.Helper()
		tc.Assert(&apptest.AssertOptions{
			Msg: msg,
			Got: func() any {
				return sut.PrometheusAPIV1Series(t, `{__name__=~"m_.*"}`, apptest.QueryOpts{
					Start: fmt.Sprintf("%d", rangeStart),
					End:   fmt.Sprintf("%d", rangeEnd),
				}).Sort()
			},
			Want: &apptest.PrometheusAPIV1SeriesResponse{
				Status: "success",
				Data:   want,
			},
			FailNow: true,
		})
	}

	// Ingest the data and verify all three series are queryable.
	sut := startSUT()
	sut.PrometheusAPIV1ImportPrometheus(t, samples, apptest.QueryOpts{})
	sut.ForceFlush(t)
	assertSeries(sut, "unexpected /api/v1/series response before backup", []map[string]string{
		{"__name__": "m_feb"},
		{"__name__": "m_jan"},
		{"__name__": "m_mar"},
	})

	// Create a full backup of the storage and stop the instance.
	tc.MustStartVmbackup("vmbackup", storageDataPath, sut.SnapshotCreateURL(), backupPath)
	tc.StopApp("vmsingle")

	// restoreAndAssert restores the given partitions into a fresh storage, starts
	// vmsingle and asserts that only the expected series are present.
	restoreAndAssert := func(instance, restorePartitions, msg string, want []map[string]string) {
		t.Helper()
		if err := os.RemoveAll(storageDataPath); err != nil {
			t.Fatalf("cannot clear storage data path %q: %v", storageDataPath, err)
		}
		tc.MustStartVmrestore(instance, backupPath, storageDataPath, "-restorePartitions="+restorePartitions)
		sut := startSUT()
		assertSeries(sut, msg, want)
		tc.StopApp("vmsingle")
	}

	// Restore only the February partition.
	restoreAndAssert("vmrestore-feb", "2025_02", "unexpected series after restoring 2025_02", []map[string]string{
		{"__name__": "m_feb"},
	})

	// Restore January and March, skipping February.
	restoreAndAssert("vmrestore-jan-mar", "2025_(01|03)", "unexpected series after restoring 2025_(01|03)", []map[string]string{
		{"__name__": "m_jan"},
		{"__name__": "m_mar"},
	})

	// Restore all the partitions matching the year via a wildcard regexp.
	restoreAndAssert("vmrestore-all", "2025_.*", "unexpected series after restoring 2025_.*", []map[string]string{
		{"__name__": "m_feb"},
		{"__name__": "m_jan"},
		{"__name__": "m_mar"},
	})
}
