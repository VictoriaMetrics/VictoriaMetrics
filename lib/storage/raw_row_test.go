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
				flushCopyBenchmarkSink = appendRawRowss(flushCopyBenchmarkSink[:0], src, len(src))
				for _, rows := range flushCopyBenchmarkSink {
					putRawRows(rows)
				}
			}
		})
	}
}

var flushCopyBenchmarkSink [][]rawRow

func TestAppendRawRowssSizesSparseCopy(t *testing.T) {
	drainRawRowsPool()

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

func TestRawRowsShardsFlushCounters(t *testing.T) {
	var rrss rawRowsShards
	rrss.init()
	nopFlush := func([][]rawRow) {}

	// Overflow a shard to trigger a full flush.
	rows := make([]rawRow, maxRawRowsPerShard*len(rrss.shards)+1)
	rrss.addRows(nopFlush, rows)
	if n := rrss.fullFlushes.Load(); n == 0 {
		t.Fatalf("expecting at least one full flush")
	}
	if n := rrss.partialFlushes.Load(); n != 0 {
		t.Fatalf("unexpected partial flushes after addRows: %d", n)
	}

	// The final flush moves the remaining shard rows as a single partial flush.
	rrss.flush(nopFlush, true)
	if n := rrss.partialFlushes.Load(); n != 1 {
		t.Fatalf("unexpected partial flushes; got %d; want 1", n)
	}
	flushed := rrss.fullFlushedRows.Load() + rrss.partialFlushedRows.Load()
	if flushed != uint64(len(rows)) {
		t.Fatalf("unexpected flushed rows; got %d; want %d", flushed, len(rows))
	}

	// An empty flush must not be counted.
	rrss.flush(nopFlush, true)
	if n := rrss.partialFlushes.Load(); n != 1 {
		t.Fatalf("empty flush must not be counted; got %d partial flushes", n)
	}
}

func drainRawRowsPool() {
	rawRowsFreeLock.Lock()
	rawRowsFree = nil
	rawRowsFreeLock.Unlock()
}

func TestRawRowsPool(t *testing.T) {
	drainRawRowsPool()

	rows := getRawRows(16)
	rows = append(rows, rawRow{Timestamp: 1})
	putRawRows(rows)

	// A free buffer with enough capacity is reused and returned empty.
	reused := getRawRows(8)
	if len(reused) != 0 || cap(reused) != cap(rows) || &reused[:1][0] != &rows[0] {
		t.Fatalf("expecting the free buffer to be reused; got len=%d, cap=%d", len(reused), cap(reused))
	}

	// A too small free buffer stays in the list and a new one is allocated.
	putRawRows(reused)
	if bigger := getRawRows(32); cap(bigger) != 32 {
		t.Fatalf("unexpected capacity; got %d; want 32", cap(bigger))
	}
	if n := len(rawRowsFree); n != 1 {
		t.Fatalf("too small buffer must stay in the free list; got %d buffers", n)
	}

	// The smallest fitting buffer is picked.
	putRawRows(make([]rawRow, 0, 64))
	if got := getRawRows(10); cap(got) != 16 {
		t.Fatalf("expecting the smallest fitting buffer; got capacity %d", cap(got))
	}
}

func TestRawRowsPoolLimit(t *testing.T) {
	drainRawRowsPool()
	defer drainRawRowsPool()

	half := maxRawRowsPerShard / 2
	putRawRows(make([]rawRow, 0, half))
	putRawRows(make([]rawRow, 0, half))

	// A larger buffer evicts smaller ones until the total fits 8 MiB.
	putRawRows(make([]rawRow, 0, half+1))
	total := 0
	for _, rows := range rawRowsFree {
		total += cap(rows)
	}
	if total > maxRawRowsPerShard {
		t.Fatalf("free list exceeds 8 MiB: %d rows", total)
	}
	if got := getRawRows(half + 1); cap(got) != half+1 {
		t.Fatalf("expecting larger buffer to be kept; got capacity %d", cap(got))
	}
}

func TestRawRowsPoolCountLimit(t *testing.T) {
	drainRawRowsPool()
	defer drainRawRowsPool()

	for i := 0; i <= numRawRowsShards; i++ {
		putRawRows(make([]rawRow, 0, 1))
	}
	if n := len(rawRowsFree); n > numRawRowsShards {
		t.Fatalf("too many free buffers: got %d; want at most %d", n, numRawRowsShards)
	}
}

func TestRawRowsShardsCapBytes(t *testing.T) {
	var rrss rawRowsShards
	rrss.init()
	if n := rrss.CapBytes(); n != 0 {
		t.Fatalf("unexpected capacity of empty shards: %d", n)
	}

	drainRawRowsPool()

	// A single row allocates the minimum shard buffer.
	rrss.addRows(func([][]rawRow) {}, []rawRow{{Timestamp: 1}})
	want := minRawRowsPerShard * int(unsafe.Sizeof(rawRow{}))
	if n := rrss.CapBytes(); n != want {
		t.Fatalf("unexpected capacity; got %d; want %d", n, want)
	}
}

func TestRawRowsShardGrows(t *testing.T) {
	drainRawRowsPool()
	defer drainRawRowsPool()

	var rrs rawRowsShard
	rrs.addRows([]rawRow{{Timestamp: 1}})
	if n := cap(rrs.rows); n != minRawRowsPerShard {
		t.Fatalf("unexpected initial capacity; got %d; want %d", n, minRawRowsPerShard)
	}

	// Growing keeps already added rows and doubles the capacity.
	tail, rowsToFlush := rrs.addRows(make([]rawRow, minRawRowsPerShard))
	if len(tail) != 0 || len(rowsToFlush) != 0 {
		t.Fatalf("growing shard mustn't flush; got %d tail rows, %d rows to flush", len(tail), len(rowsToFlush))
	}
	if n := cap(rrs.rows); n != 2*minRawRowsPerShard {
		t.Fatalf("unexpected capacity after grow; got %d; want %d", n, 2*minRawRowsPerShard)
	}
	if len(rrs.rows) != minRawRowsPerShard+1 || rrs.rows[0].Timestamp != 1 {
		t.Fatalf("rows must be preserved on grow; got %d rows, first timestamp %d", len(rrs.rows), rrs.rows[0].Timestamp)
	}

	// The shard flushes only after reaching maxRawRowsPerShard.
	_, rowsToFlush = rrs.addRows(make([]rawRow, maxRawRowsPerShard))
	if len(rowsToFlush) != maxRawRowsPerShard {
		t.Fatalf("unexpected rows to flush; got %d; want %d", len(rowsToFlush), maxRawRowsPerShard)
	}
}
