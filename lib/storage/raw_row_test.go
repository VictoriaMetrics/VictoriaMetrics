package storage

import (
	"testing"
)

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
	dst := rrs.appendRawRowsToFlush(nil, currentTimeMs, false)
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
		if dst := rrs.appendRawRowsToFlush(nil, currentTimeMs, false); len(dst) != 0 {
			t.Fatalf("unexpected rows to flush from empty shard: %d", len(dst))
		}
		if rrs.rows == nil {
			t.Fatalf("the buffer is released too early after %d idle flushes", i)
		}
	}

	// The next idle tick releases the buffer.
	rrs.appendRawRowsToFlush(nil, currentTimeMs, false)
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
	rrs.appendRawRowsToFlush(nil, currentTimeMs, true)
	for i := 0; i < 2*maxRawRowsIdleFlushes; i++ {
		rrs.appendRawRowsToFlush(nil, currentTimeMs, true)
	}
	if rrs.rows == nil {
		t.Fatalf("final flushes must not release the buffer")
	}
}
