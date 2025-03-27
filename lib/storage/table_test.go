package storage

import (
	"os"
	"sync"
	"testing"
	"time"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	const retention = 123 * retention31Days

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

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

func TestMustGetIndexDB(t *testing.T) {
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
				idb := s.tb.MustGetIndexDB(ts)
				s.tb.PutIndexDB(idb)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}
