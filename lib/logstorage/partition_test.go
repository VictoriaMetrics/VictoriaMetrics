package logstorage

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
)

func TestPartitionLifecycle(t *testing.T) {
	const path = "TestPartitionLifecycle"
	var ddbStats DatadbStats

	s := newTestStorage()
	for i := 0; i < 3; i++ {
		mustCreatePartition(path)
		for j := 0; j < 2; j++ {
			pt := mustOpenPartition(s, path)
			ddbStats.reset()
			pt.ddb.updateStats(&ddbStats)
			if n := ddbStats.RowsCount(); n != 0 {
				t.Fatalf("unexpected non-zero number of entries in empty partition: %d", n)
			}
			if ddbStats.InmemoryParts != 0 {
				t.Fatalf("unexpected non-zero number of in-memory parts in empty partition: %d", ddbStats.InmemoryParts)
			}
			if ddbStats.FileParts != 0 {
				t.Fatalf("unexpected non-zero number of file parts in empty partition: %d", ddbStats.FileParts)
			}
			if ddbStats.CompressedInmemorySize != 0 {
				t.Fatalf("unexpected non-zero size of inmemory parts for empty partition")
			}
			if ddbStats.CompressedFileSize != 0 {
				t.Fatalf("unexpected non-zero size of file parts for empty partition")
			}
			time.Sleep(10 * time.Millisecond)
			mustClosePartition(pt)
		}
		mustDeletePartition(path)
	}
	closeTestStorage(s)
}

func TestPartitionMustAddRowsSerial(t *testing.T) {
	const path = "TestPartitionMustAddRowsSerial"
	var ddbStats DatadbStats

	s := newTestStorage()
	mustCreatePartition(path)
	pt := mustOpenPartition(s, path)

	// Try adding the same entry at a time.
	totalRowsCount := uint64(0)
	for i := 0; i < 100; i++ {
		lr := newTestLogRows(1, 1, 0)
		totalRowsCount += uint64(len(lr.timestamps))
		pt.mustAddRows(lr)
		ddbStats.reset()
		pt.ddb.updateStats(&ddbStats)
		if n := ddbStats.RowsCount(); n != totalRowsCount {
			t.Fatalf("unexpected number of entries in partition; got %d; want %d", n, totalRowsCount)
		}
	}

	// Try adding different entry at a time.
	for i := 0; i < 100; i++ {
		lr := newTestLogRows(1, 1, int64(i))
		totalRowsCount += uint64(len(lr.timestamps))
		pt.mustAddRows(lr)
		ddbStats.reset()
		pt.ddb.updateStats(&ddbStats)
		if n := ddbStats.RowsCount(); n != totalRowsCount {
			t.Fatalf("unexpected number of entries in partition; got %d; want %d", n, totalRowsCount)
		}
	}

	// Re-open the partition and verify the number of entries remains the same
	mustClosePartition(pt)
	pt = mustOpenPartition(s, path)
	ddbStats.reset()
	pt.ddb.updateStats(&ddbStats)
	if n := ddbStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries after re-opening the partition; got %d; want %d", n, totalRowsCount)
	}
	if ddbStats.InmemoryParts != 0 {
		t.Fatalf("unexpected non-zero number of in-memory parts after re-opening the partition: %d", ddbStats.InmemoryParts)
	}
	if ddbStats.FileParts == 0 {
		t.Fatalf("the number of file parts must be greater than 0 after re-opening the partition")
	}

	// Try adding entries for multiple streams at a time
	for i := 0; i < 5; i++ {
		lr := newTestLogRows(3, 7, 0)
		totalRowsCount += uint64(len(lr.timestamps))
		pt.mustAddRows(lr)
		ddbStats.reset()
		pt.ddb.updateStats(&ddbStats)
		if n := ddbStats.RowsCount(); n != totalRowsCount {
			t.Fatalf("unexpected number of entries in partition; got %d; want %d", n, totalRowsCount)
		}
		time.Sleep(time.Millisecond)
	}

	// Re-open the partition and verify the number of entries remains the same
	mustClosePartition(pt)
	pt = mustOpenPartition(s, path)
	ddbStats.reset()
	pt.ddb.updateStats(&ddbStats)
	if n := ddbStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries after re-opening the partition; got %d; want %d", n, totalRowsCount)
	}
	if ddbStats.InmemoryParts != 0 {
		t.Fatalf("unexpected non-zero number of in-memory parts after re-opening the partition: %d", ddbStats.InmemoryParts)
	}
	if ddbStats.FileParts == 0 {
		t.Fatalf("the number of file parts must be greater than 0 after re-opening the partition")
	}

	mustClosePartition(pt)
	mustDeletePartition(path)

	closeTestStorage(s)
}

func TestPartitionMustAddRowsConcurrent(t *testing.T) {
	const path = "TestPartitionMustAddRowsConcurrent"
	s := newTestStorage()

	mustCreatePartition(path)
	pt := mustOpenPartition(s, path)

	const workersCount = 3
	totalRowsCount := uint64(0)
	doneCh := make(chan struct{}, workersCount)
	for i := 0; i < cap(doneCh); i++ {
		go func() {
			for j := 0; j < 7; j++ {
				lr := newTestLogRows(5, 10, int64(j))
				pt.mustAddRows(lr)
				atomic.AddUint64(&totalRowsCount, uint64(len(lr.timestamps)))
			}
			doneCh <- struct{}{}
		}()
	}
	timer := timerpool.Get(time.Second)
	defer timerpool.Put(timer)
	for i := 0; i < cap(doneCh); i++ {
		select {
		case <-doneCh:
		case <-timer.C:
			t.Fatalf("timeout")
		}
	}

	var ddbStats DatadbStats
	pt.ddb.updateStats(&ddbStats)
	if n := ddbStats.RowsCount(); n != totalRowsCount {
		t.Fatalf("unexpected number of entries; got %d; want %d", n, totalRowsCount)
	}

	mustClosePartition(pt)
	mustDeletePartition(path)

	closeTestStorage(s)
}

// newTestStorage creates new storage for tests.
//
// When the storage is no longer needed, closeTestStorage() must be called.
func newTestStorage() *Storage {
	streamIDCache := workingsetcache.New(1024 * 1024)
	streamFilterCache := workingsetcache.New(1024 * 1024)
	return &Storage{
		flushInterval:     time.Second,
		streamIDCache:     streamIDCache,
		streamFilterCache: streamFilterCache,
	}
}

// closeTestStorage closes storage created via newTestStorage().
func closeTestStorage(s *Storage) {
	s.streamIDCache.Stop()
	s.streamFilterCache.Stop()
}
