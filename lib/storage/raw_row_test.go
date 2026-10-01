package storage

import (
	"testing"
)

func TestRawRowsShardReleasesIdleBuffer(t *testing.T) {
	var rrs rawRowsShard

	rows, rowsToFlush := rrs.addRows([]rawRow{{Timestamp: 1}, {Timestamp: 2}})
	if len(rows) != 0 || len(rowsToFlush) != 0 {
		t.Fatalf("unexpected rows left after addRows: %d, %d", len(rows), len(rowsToFlush))
	}
	if cap(rrs.rows) == 0 {
		t.Fatalf("expecting allocated buffer after addRows")
	}

	// The first flush after the deadline moves the buffered rows to dst and keeps the buffer.
	currentTimeMs := rrs.flushDeadlineMs.Load()
	dst := rrs.appendRawRowsToFlush(nil, currentTimeMs, false)
	if n := len(dst); n != 1 || len(dst[0]) != 2 {
		t.Fatalf("unexpected rows to flush: %d", n)
	}
	if cap(rrs.rows) == 0 {
		t.Fatalf("the buffer must be kept right after the flush")
	}

	// The buffer must be kept while the shard stays empty for less than maxIdleFlushes ticks.
	for i := 1; i < maxIdleFlushes; i++ {
		if dst := rrs.appendRawRowsToFlush(nil, currentTimeMs, false); len(dst) != 0 {
			t.Fatalf("unexpected rows to flush from empty shard: %d", len(dst))
		}
		if cap(rrs.rows) == 0 {
			t.Fatalf("the buffer is released too early after %d idle flushes", i)
		}
	}

	// The next idle tick releases the buffer.
	rrs.appendRawRowsToFlush(nil, currentTimeMs, false)
	if cap(rrs.rows) != 0 {
		t.Fatalf("the buffer must be released after %d idle flushes", maxIdleFlushes)
	}

	// The released shard must accept new rows.
	rrs.addRows([]rawRow{{Timestamp: 3}})
	if len(rrs.rows) != 1 || rrs.idleFlushes != 0 {
		t.Fatalf("unexpected state after adding rows to released shard: len=%d, idleFlushes=%d", len(rrs.rows), rrs.idleFlushes)
	}

	// A final flush must not release the buffer.
	rrs.appendRawRowsToFlush(nil, currentTimeMs, true)
	for i := 0; i < 2*maxIdleFlushes; i++ {
		rrs.appendRawRowsToFlush(nil, currentTimeMs, true)
	}
	if cap(rrs.rows) == 0 {
		t.Fatalf("final flushes must not release the buffer")
	}
}
