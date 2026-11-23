package storage

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	const retention = 123 * retention31Days

	fs.MustRemoveDir(path)
	defer fs.MustRemoveDir(path)

	// Create a new table
	strg := newTestStorage()
	strg.retentionMsecs = retention.Milliseconds()
	tb := mustOpenTable(path, strg)

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for range 10 {
		tb := mustOpenTable(path, strg)
		tb.MustClose()
	}

	stopTestStorage(strg)
}

func TestGetPartition(t *testing.T) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()

	var ptw *partitionWrapper
	timestamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	ptw = s.tb.GetPartition(timestamp)
	if ptw != nil {
		name := ptw.pt.name
		s.tb.PutPartition(ptw)
		t.Fatalf("GetPartition() unexpectedly returned a partition that should not exist: %s", name)
	}

	ptw = s.tb.MustGetPartition(timestamp)
	if ptw == nil {
		t.Fatalf("MustGetPartition() unexpectedly did not create a new partition")
	}
	s.tb.PutPartition(ptw)

	ptw = s.tb.GetPartition(timestamp)
	if ptw == nil {
		t.Fatalf("GetPartition() unexpectedly did not find partition")
	}
	s.tb.PutPartition(ptw)
}

func TestGetPartition_concurrent(t *testing.T) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()

	begin := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	limit := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for ts := begin; ts < limit; ts += msecPerDay {
		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				ptw := s.tb.MustGetPartition(ts)
				s.tb.PutPartition(ptw)

				ptw = s.tb.GetPartition(ts)
				s.tb.PutPartition(ptw)
			})
		}
		wg.Wait()
	}
}

func TestTableFuturePartitionRetention(t *testing.T) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	futureMonth := monthStart.AddDate(0, 2, 0)
	metricName := MarshalMetricNameRaw(nil, []prompb.Label{{Name: "__name__", Value: "retention_test"}})
	currentRow := MetricRow{MetricNameRaw: metricName, Timestamp: monthStart.UnixMilli(), Value: 42}
	futureRow := MetricRow{MetricNameRaw: metricName, Timestamp: futureMonth.UnixMilli(), Value: 43}

	f := func(t *testing.T, firstPass time.Time) {
		t.Helper()

		path := t.TempDir()
		s := MustOpenStorage(path, OpenOptions{FutureRetention: 90 * 24 * time.Hour})
		s.AddRows([]MetricRow{currentRow, futureRow}, defaultPrecisionBits)
		s.MustClose()

		s = MustOpenStorage(path, OpenOptions{FutureRetention: 2 * 24 * time.Hour})
		defer s.MustClose()

		assertPartitionPaths := func(month time.Time, want bool) {
			t.Helper()
			for _, dirname := range []string{smallDirname, bigDirname, indexdbDirname} {
				partitionPath := filepath.Join(path, dataDirname, dirname, month.Format("2006_01"))
				if got := fs.IsPathExist(partitionPath); got != want {
					t.Errorf("partition directory %q exists: got %t, want %t", partitionPath, got, want)
				}
			}
		}
		assertRows := func(want []MetricRow) {
			t.Helper()
			tr := TimeRange{MinTimestamp: currentRow.Timestamp, MaxTimestamp: futureRow.Timestamp}
			tfs := NewTagFilters()
			if err := tfs.Add(nil, []byte("retention_test"), false, false); err != nil {
				t.Fatal(err)
			}
			if err := testAssertSearchResult(s, tr, tfs, want); err != nil {
				t.Error(err)
			}
		}

		assertPartitionPaths(monthStart, true)
		assertPartitionPaths(futureMonth, true)
		assertRows([]MetricRow{currentRow, futureRow})

		_, startupCutoff := s.tb.getMinMaxRetentionTimestamps()
		futurePartitions := s.tb.collectFuturePartitions(startupCutoff)
		if len(futurePartitions) != 1 {
			t.Fatalf("unexpected startup future partition count: got %d, want 1", len(futurePartitions))
		}
		s.tb.removeExpiredPartitions(firstPass.UnixMilli(), futurePartitions)

		assertPartitionPaths(monthStart, true)
		assertPartitionPaths(futureMonth, false)
		assertRows([]MetricRow{currentRow})
	}

	t.Run("clock rollback before first pass", func(t *testing.T) {
		f(t, monthStart.AddDate(0, 0, -3))
	})
	t.Run("smaller future retention after restart", func(t *testing.T) {
		f(t, now)
	})
}
