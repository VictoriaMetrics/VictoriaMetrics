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

func TestTableRetentionClockRollback(t *testing.T) {
	path := t.TempDir()
	s := MustOpenStorage(path, OpenOptions{FutureRetention: 2 * 24 * time.Hour})
	defer s.MustClose()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	timestamp := monthStart.UnixMilli()
	rows := []MetricRow{{
		MetricNameRaw: MarshalMetricNameRaw(nil, []prompb.Label{{Name: "__name__", Value: "clock_rollback"}}),
		Timestamp:     timestamp,
		Value:         42,
	}}
	s.AddRows(rows, defaultPrecisionBits)
	s.DebugFlush()

	tr := TimeRange{MinTimestamp: timestamp, MaxTimestamp: timestamp}
	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte("clock_rollback"), false, false); err != nil {
		t.Fatal(err)
	}
	if err := testAssertSearchResult(s, tr, tfs, rows); err != nil {
		t.Fatalf("cannot read sample before rollback: %s", err)
	}

	// Move the wall clock far enough back that the partition starts past futureRetention.
	s.tb.removeExpiredPartitions(monthStart.AddDate(0, 0, -3).UnixMilli(), nil)

	for _, dirname := range []string{smallDirname, bigDirname, indexdbDirname} {
		partitionPath := filepath.Join(path, dataDirname, dirname, monthStart.Format("2006_01"))
		if !fs.IsPathExist(partitionPath) {
			t.Errorf("partition directory %q was removed after a clock rollback", partitionPath)
		}
	}
	if err := testAssertSearchResult(s, tr, tfs, rows); err != nil {
		t.Errorf("cannot read sample after rollback: %s", err)
	}
}
