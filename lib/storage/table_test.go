package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	for i := 0; i < 10; i++ {
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
			wg.Add(1)
			go func() {
				ptw := s.tb.MustGetPartition(ts)
				s.tb.PutPartition(ptw)

				ptw = s.tb.GetPartition(ts)
				s.tb.PutPartition(ptw)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}
