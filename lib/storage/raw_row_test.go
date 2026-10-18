package storage

import (
	"testing"
	"unsafe"
)

func BenchmarkFlushCopy(b *testing.B) {
	for _, tc := range []struct {
		name string
		rows int
	}{
		{name: "sparse", rows: 128},
		{name: "full", rows: maxRawRowsPerShard},
	} {
		src := make([]rawRow, tc.rows)
		for i := range src {
			src[i].Timestamp = int64(i)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(src)) * int64(unsafe.Sizeof(rawRow{})))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				flushCopyBenchmarkSink = appendRawRowss(nil, src, len(src))
			}
		})
	}
}

var flushCopyBenchmarkSink [][]rawRow

func TestAppendRawRowssSizesSparseCopy(t *testing.T) {
	src := []rawRow{{Timestamp: 1}, {Timestamp: 2}}
	dst := appendRawRowss(nil, src, len(src))
	if len(dst) != 1 || cap(dst[0]) != len(src) {
		t.Fatalf("unexpected copy size: got %d slices with capacity %d; want one with capacity %d", len(dst), cap(dst[0]), len(src))
	}
	src[0].Timestamp = 3
	if dst[0][0].Timestamp != 1 {
		t.Fatal("flush copy must not alias shard rows")
	}
}

func TestRawRowsShardReleasesIdleBuffer(t *testing.T) {
	var rrs rawRowsShard

	rowsToAdd := []rawRow{
		{Timestamp: 1},
		{Timestamp: 2},
	}
	rows, rowsToFlush := rrs.addRows(rowsToAdd)
	if len(rows) != 0 || len(rowsToFlush) != 0 {
		t.Fatalf("unexpected rows left after addRows: %d, %d", len(rows), len(rowsToFlush))
	}
	if len(rrs.rows) != len(rowsToAdd) {
		t.Fatalf("unexpected rrs.rows addRows: got: %d, want: %d", len(rrs.rows), len(rowsToAdd))
	}

	// The first flush after the deadline moves the buffered rows to dst and keeps the buffer.
	currentTimeMs := rrs.flushDeadlineMs.Load()
	dst := rrs.appendRawRowsToFlush(nil, 0, currentTimeMs, false)
	if len(dst) != 1 {
		t.Fatalf("unexpected dst rows after flush: got: %d, want: 1", len(dst))
	}
	if len(dst[0]) != len(rowsToAdd) {
		t.Fatalf("unexpected dst[0] rows after flush: got: %d, want: %d", len(dst[0]), len(rowsToAdd))
	}
	if rrs.rows == nil {
		t.Fatalf("the buffer must be kept right after the flush")
	}

	// The buffer must be kept while the shard stays empty for less than maxIdleFlushes ticks.
	for i := 1; i < maxRawRowsIdleFlushes; i++ {
		if dst := rrs.appendRawRowsToFlush(nil, 0, currentTimeMs, false); len(dst) != 0 {
			t.Fatalf("unexpected rows to flush from empty shard: %d", len(dst))
		}
		if rrs.rows == nil {
			t.Fatalf("the buffer is released too early after %d idle flushes", i)
		}
	}

	// The next idle tick releases the buffer.
	rrs.appendRawRowsToFlush(nil, 0, currentTimeMs, false)
	if rrs.rows != nil {
		t.Fatalf("the buffer must be released after %d idle flushes", maxRawRowsIdleFlushes)
	}

	// The released shard must accept new rows.
	rrs.addRows(rowsToAdd)
	if len(rrs.rows) != len(rowsToAdd) {
		t.Fatalf("unexpected rrs.rows addRows: got: %d, want: %d", len(rrs.rows), len(rowsToAdd))
	}
	if rrs.idleFlushes != 0 {
		t.Fatalf("unexpected idleFlushes state after adding rows to released shard: got: %d, want: 0", rrs.idleFlushes)
	}

	// A final flush must not release the buffer.
	rrs.appendRawRowsToFlush(nil, 0, currentTimeMs, true)
	for i := 0; i < 2*maxRawRowsIdleFlushes; i++ {
		rrs.appendRawRowsToFlush(nil, 0, currentTimeMs, true)
	}
	if rrs.rows == nil {
		t.Fatalf("final flushes must not release the buffer")
	}
}
