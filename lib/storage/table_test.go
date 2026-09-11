package storage

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// newBucketTestTable uses real partitions and shard writes without background workers.
func newBucketTestTable(partitions, capacity int) *table {
	tb := &table{}
	for i := range partitions {
		start := time.Date(2025, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		pt := &partition{tr: TimeRange{MinTimestamp: start.UnixMilli(), MaxTimestamp: start.AddDate(0, 1, 0).UnixMilli() - 1}}
		pt.rawRows.shards = make([]rawRowsShard, 1)
		pt.rawRows.shards[0].rows = make([]rawRow, 0, capacity)
		tb.addPartitionLocked(pt)
	}
	return tb
}

func TestTableMustAddRowsBuckets(t *testing.T) {
	for _, indices := range [][]int{{}, {0, 0}, {0, 0, 1, 1}, {0, 1, 0, 1, 0}, {0, 0, 1, 1, 0, 0, 2, 0}} {
		tb := newBucketTestTable(3, 100)
		backing := make([]rawRow, len(indices)+8)
		for i := range backing {
			backing[i] = rawRow{Timestamp: -999, Value: -123, PrecisionBits: defaultPrecisionBits}
		}
		rows := backing[:len(indices)]
		want := make([][]rawRow, 3)
		for i, idx := range indices {
			pt := tb.ptws[idx].pt
			ts := pt.tr.MinTimestamp
			if i%2 == 0 {
				ts = pt.tr.MaxTimestamp
			}
			rows[i] = rawRow{TSID: TSID{MetricID: uint64(i + 1)}, Timestamp: ts, Value: float64(i + 1), PrecisionBits: defaultPrecisionBits}
			want[idx] = append(want[idx], rows[i])
		}
		original := slices.Clone(backing)
		partitions := slices.Clone(tb.ptws)
		tb.MustAddRows(rows)
		if !slices.Equal(backing, original) {
			t.Fatalf("input or spare capacity modified for %v", indices)
		}
		clear(backing)
		for i, ptw := range partitions {
			got := ptw.pt.rawRows.shards[0].rows
			if !slices.Equal(got, want[i]) {
				t.Fatalf("partition %d for %v: got %+v; want %+v", i, indices, got, want[i])
			}
			calls := uint32(0)
			if len(want[i]) > 0 {
				calls = 1
			}
			if got := ptw.pt.rawRows.shardIdx.Load(); got != calls {
				t.Fatalf("partition %d: got %d shard writes; want %d", i, got, calls)
			}
		}
	}
}

func TestTableMustAddRowsMissingMonth(t *testing.T) {
	s := newTestStorage()
	defer stopTestStorage(s)
	tb := mustOpenTable(t.TempDir(), s)
	defer tb.MustClose()
	now := time.Now().UTC()
	boundary := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	ptw := tb.MustGetPartition(boundary - 1)
	tb.PutPartition(ptw)
	rows := []rawRow{
		{Timestamp: boundary - 1, Value: 1, PrecisionBits: defaultPrecisionBits},
		{Timestamp: boundary, Value: 2, PrecisionBits: defaultPrecisionBits},
		{Timestamp: boundary - 2, Value: 3, PrecisionBits: defaultPrecisionBits},
		{Timestamp: boundary + 1, Value: 4, PrecisionBits: defaultPrecisionBits},
	}
	want := slices.Clone(rows)
	tb.MustAddRows(rows)
	if !slices.Equal(rows, want) {
		t.Fatal("input modified")
	}
	clear(rows)
	tb.DebugFlush()
	for _, ts := range []int64{boundary - 1, boundary} {
		ptw := tb.GetPartition(ts)
		if ptw == nil {
			t.Fatalf("missing partition for %d", ts)
		}
		tr := ptw.pt.tr
		tb.PutPartition(ptw)
		var search tableSearch
		search.Init(tb, []TSID{{}}, tr)
		var got []rawRow
		for search.NextBlock() {
			var block Block
			search.BlockRef.MustReadBlock(&block)
			rb := newTestRawBlock(&block, tr)
			for i, timestamp := range rb.Timestamps {
				got = append(got, rawRow{TSID: rb.TSID, Timestamp: timestamp, Value: rb.Values[i], PrecisionBits: defaultPrecisionBits})
			}
		}
		err := search.Error()
		search.MustClose()
		if err != nil {
			t.Fatal(err)
		}
		slices.SortFunc(got, func(a, b rawRow) int { return int(a.Value - b.Value) })
		expected := []rawRow{want[0], want[2]}
		if ts == boundary {
			expected = []rawRow{want[1], want[3]}
		}
		if !slices.Equal(got, expected) {
			t.Fatalf("timestamp %d: got %+v; want %+v", ts, got, expected)
		}
	}
}

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
