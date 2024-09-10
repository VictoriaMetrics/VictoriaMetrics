package logstorage

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestStorageLifecycle(t *testing.T) {
	t.Parallel()

	path := t.Name()

	for i := 0; i < 3; i++ {
		cfg := &StorageConfig{}
		s := MustOpenStorage(path, cfg)
		s.MustClose()
	}
	fs.MustRemoveAll(path)
}

func TestStorageMustAddRows(t *testing.T) {
	t.Parallel()

	path := t.Name()

	var sStats StorageStats

	cfg := &StorageConfig{}
	s := MustOpenStorage(path, cfg)

	// Try adding the same entry multiple times.
	totalRowsCount := uint64(0)
	for i := 0; i < 100; i++ {
		lr := newTestLogRows(1, 1, 0)
		lr.timestamps[0] = time.Now().UTC().UnixNano()
		totalRowsCount += uint64(len(lr.timestamps))
		s.MustAddRows(lr)
		sStats.Reset()
		s.UpdateStats(&sStats)
		if n := sStats.RowsCount(); n != totalRowsCount {
			t.Fatalf("unexpected number of entries in storage; got %d; want %d", n, totalRowsCount)
		}
	}

	s.MustClose()

	// Re-open the storage and try writing data to it
	s = MustOpenStorage(path, cfg)

	sStats.Reset()
	s.UpdateStats(&sStats)
	if n := sStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries in storage; got %d; want %d", n, totalRowsCount)
	}

	lr := newTestLogRows(3, 10, 0)
	for i := range lr.timestamps {
		lr.timestamps[i] = time.Now().UTC().UnixNano()
	}
	totalRowsCount += uint64(len(lr.timestamps))
	s.MustAddRows(lr)
	sStats.Reset()
	s.UpdateStats(&sStats)
	if n := sStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries in storage; got %d; want %d", n, totalRowsCount)
	}

	s.MustClose()

	// Re-open the storage with big retention and try writing data
	// to different days in the past and in the future
	cfg = &StorageConfig{
		Retention:       365 * 24 * time.Hour,
		FutureRetention: 365 * 24 * time.Hour,
	}
	s = MustOpenStorage(path, cfg)

	lr = newTestLogRows(3, 10, 0)
	now := time.Now().UTC().UnixNano() - int64(len(lr.timestamps)/2)*nsecsPerDay
	for i := range lr.timestamps {
		lr.timestamps[i] = now
		now += nsecsPerDay
	}
	totalRowsCount += uint64(len(lr.timestamps))
	s.MustAddRows(lr)
	sStats.Reset()
	s.UpdateStats(&sStats)
	if n := sStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries in storage; got %d; want %d", n, totalRowsCount)
	}

	s.MustClose()

	// Make sure the stats is valid after re-opening the storage
	s = MustOpenStorage(path, cfg)
	sStats.Reset()
	s.UpdateStats(&sStats)
	if n := sStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries in storage; got %d; want %d", n, totalRowsCount)
	}
	s.MustClose()

	fs.MustRemoveAll(path)
}
