package storage

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// The number of shards for rawRow entries.
//
// Higher number of shards reduces CPU contention and increases the max bandwidth on multi-core systems.
var numRawRowsShards = cgroup.AvailableCPUs()

// The interval for flushing buffered rows into parts, so they become visible to search.
const pendingRowsFlushInterval = 2 * time.Second

// The maximum number of rawRow items in rawRowsShard.
//
// Limit the maximum shard size to 8Mb, since this gives the lowest CPU usage under high ingestion rate.
const maxRawRowsPerShard = (8 << 20) / int(unsafe.Sizeof(rawRow{}))

// rawRow represents raw timeseries row.
type rawRow struct {
	// TSID is time series id.
	TSID TSID

	// Timestamp is unix timestamp in milliseconds.
	Timestamp int64

	// Value is time series value for the given timestamp.
	Value float64

	// PrecisionBits is the number of the significant bits in the Value
	// to store. Possible values are [1..64].
	// 1 means max. 50% error, 2 - 25%, 3 - 12.5%, 64 means no error, i.e.
	// Value stored without information loss.
	PrecisionBits uint8
}

type rawRowsMarshaler struct {
	bsw blockStreamWriter

	auxTimestamps  []int64
	auxValues      []int64
	auxFloatValues []float64
}

func (rrm *rawRowsMarshaler) reset() {
	rrm.bsw.reset()

	rrm.auxTimestamps = rrm.auxTimestamps[:0]
	rrm.auxValues = rrm.auxValues[:0]
	rrm.auxFloatValues = rrm.auxFloatValues[:0]
}

// Use sort.Interface instead of sort.Slice in order to optimize rows swap.
type rawRowsSort []rawRow

func (rrs *rawRowsSort) Len() int { return len(*rrs) }
func (rrs *rawRowsSort) Less(i, j int) bool {
	x := *rrs
	if i < 0 || j < 0 || i >= len(x) || j >= len(x) {
		// This is no-op for compiler, so it doesn't generate panic code
		// for out of range access on x[i], x[j] below
		return false
	}
	a := &x[i]
	b := &x[j]
	ta := &a.TSID
	tb := &b.TSID

	// Manually inline TSID.Less here, since the compiler doesn't inline it yet :(
	if ta.MetricGroupID != tb.MetricGroupID {
		return ta.MetricGroupID < tb.MetricGroupID
	}
	if ta.JobID != tb.JobID {
		return ta.JobID < tb.JobID
	}
	if ta.InstanceID != tb.InstanceID {
		return ta.InstanceID < tb.InstanceID
	}
	if ta.MetricID != tb.MetricID {
		return ta.MetricID < tb.MetricID
	}
	return a.Timestamp < b.Timestamp
}
func (rrs *rawRowsSort) Swap(i, j int) {
	x := *rrs
	x[i], x[j] = x[j], x[i]
}

func (rrm *rawRowsMarshaler) marshalToInmemoryPart(mp *inmemoryPart, rows []rawRow) {
	if len(rows) == 0 {
		return
	}
	if uint64(len(rows)) >= 1<<32 {
		logger.Panicf("BUG: rows count must be smaller than 2^32; got %d", len(rows))
	}

	// Use the minimum compression level for first-level in-memory blocks,
	// since they are going to be re-compressed during subsequent merges.
	const compressLevel = -5 // See https://github.com/facebook/zstd/releases/tag/v1.3.4
	rrm.bsw.MustInitFromInmemoryPart(mp, compressLevel)

	ph := &mp.ph
	ph.Reset()

	// Sort rows by (TSID, Timestamp) if they aren't sorted yet.
	rrs := rawRowsSort(rows)
	if !sort.IsSorted(&rrs) {
		sort.Sort(&rrs)
	}

	// Group rows into blocks.
	var scale int16
	var rowsMerged uint64
	r := &rows[0]
	tsid := &r.TSID
	precisionBits := r.PrecisionBits
	tmpBlock := getBlock()
	defer putBlock(tmpBlock)
	for i := range rows {
		r = &rows[i]
		if r.TSID.MetricID == tsid.MetricID && len(rrm.auxTimestamps) < maxRowsPerBlock {
			rrm.auxTimestamps = append(rrm.auxTimestamps, r.Timestamp)
			rrm.auxFloatValues = append(rrm.auxFloatValues, r.Value)
			continue
		}

		rrm.auxValues, scale = decimal.AppendFloatToDecimal(rrm.auxValues[:0], rrm.auxFloatValues)
		tmpBlock.Init(tsid, rrm.auxTimestamps, rrm.auxValues, scale, precisionBits)
		rrm.bsw.WriteExternalBlock(tmpBlock, ph, &rowsMerged)

		tsid = &r.TSID
		precisionBits = r.PrecisionBits
		rrm.auxTimestamps = append(rrm.auxTimestamps[:0], r.Timestamp)
		rrm.auxFloatValues = append(rrm.auxFloatValues[:0], r.Value)
	}

	rrm.auxValues, scale = decimal.AppendFloatToDecimal(rrm.auxValues[:0], rrm.auxFloatValues)
	tmpBlock.Init(tsid, rrm.auxTimestamps, rrm.auxValues, scale, precisionBits)
	rrm.bsw.WriteExternalBlock(tmpBlock, ph, &rowsMerged)
	if rowsMerged != uint64(len(rows)) {
		logger.Panicf("BUG: unexpected rowsMerged; got %d; want %d", rowsMerged, len(rows))
	}
	rrm.bsw.MustClose()
}

func getRawRowsMarshaler() *rawRowsMarshaler {
	v := rrmPool.Get()
	if v == nil {
		return &rawRowsMarshaler{}
	}
	return v.(*rawRowsMarshaler)
}

func putRawRowsMarshaler(rrm *rawRowsMarshaler) {
	rrm.reset()
	rrmPool.Put(rrm)
}

var rrmPool sync.Pool

type rawRowsShards struct {
	flushDeadlineMs atomic.Int64

	shardIdx atomic.Uint32

	// Shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawRowsShard

	rowssToFlushLock sync.Mutex
	rowssToFlush     [][]rawRow
}

func (rrss *rawRowsShards) init() {
	rrss.shards = make([]rawRowsShard, numRawRowsShards)
}

func (rrss *rawRowsShards) Len() int {
	n := 0
	for i := range rrss.shards[:] {
		n += rrss.shards[i].Len()
	}

	rrss.rowssToFlushLock.Lock()
	for _, rows := range rrss.rowssToFlush {
		n += len(rows)
	}
	rrss.rowssToFlushLock.Unlock()

	return n
}

func (rrss *rawRowsShards) addRows(flush func([][]rawRow), rows []rawRow) {
	shards := rrss.shards
	shardsLen := uint32(len(shards))
	for len(rows) > 0 {
		n := rrss.shardIdx.Add(1)
		idx := n % shardsLen
		tailRows, rowsToFlush := shards[idx].addRows(rows)
		rrss.addRowsToFlush(flush, rowsToFlush)
		rows = tailRows
	}
}

func (rrss *rawRowsShards) addRowsToFlush(flush func([][]rawRow), rowsToFlush []rawRow) {
	if len(rowsToFlush) == 0 {
		return
	}

	var rowssToMerge [][]rawRow

	rrss.rowssToFlushLock.Lock()
	if len(rrss.rowssToFlush) == 0 {
		rrss.updateFlushDeadline()
	}
	rrss.rowssToFlush = append(rrss.rowssToFlush, rowsToFlush)
	if len(rrss.rowssToFlush) >= defaultPartsToMerge {
		rowssToMerge = rrss.rowssToFlush
		rrss.rowssToFlush = nil
	}
	rrss.rowssToFlushLock.Unlock()

	flush(rowssToMerge)
}

func (rrss *rawRowsShards) updateFlushDeadline() {
	rrss.flushDeadlineMs.Store(time.Now().Add(pendingRowsFlushInterval).UnixMilli())
}

func (rrss *rawRowsShards) flush(flush func(rrs [][]rawRow), isFinal bool) {
	var dst [][]rawRow

	currentTimeMs := time.Now().UnixMilli()
	flushDeadlineMs := rrss.flushDeadlineMs.Load()
	if isFinal || currentTimeMs >= flushDeadlineMs {
		rrss.rowssToFlushLock.Lock()
		dst = rrss.rowssToFlush
		rrss.rowssToFlush = nil
		rrss.rowssToFlushLock.Unlock()
	}

	for i := range rrss.shards {
		dst = rrss.shards[i].appendRawRowsToFlush(dst, currentTimeMs, isFinal)
	}

	flush(dst)
}

type rawRowsShardNopad struct {
	flushDeadlineMs atomic.Int64

	mu   sync.Mutex
	rows []rawRow
}

type rawRowsShard struct {
	rawRowsShardNopad

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(rawRowsShardNopad{})%atomicutil.CacheLineSize]byte
}

func (rrs *rawRowsShard) Len() int {
	rrs.mu.Lock()
	n := len(rrs.rows)
	rrs.mu.Unlock()
	return n
}

func (rrs *rawRowsShard) addRows(rows []rawRow) ([]rawRow, []rawRow) {
	var rowsToFlush []rawRow

	rrs.mu.Lock()
	if cap(rrs.rows) == 0 {
		rrs.rows = newRawRows()
	}
	if len(rrs.rows) == 0 {
		rrs.updateFlushDeadline()
	}
	n := copy(rrs.rows[len(rrs.rows):cap(rrs.rows)], rows)
	rrs.rows = rrs.rows[:len(rrs.rows)+n]
	rows = rows[n:]
	if len(rows) > 0 {
		rowsToFlush = rrs.rows
		rrs.rows = newRawRows()
		rrs.updateFlushDeadline()
		n = copy(rrs.rows[:cap(rrs.rows)], rows)
		rrs.rows = rrs.rows[:n]
		rows = rows[n:]
	}
	rrs.mu.Unlock()

	return rows, rowsToFlush
}

func newRawRows() []rawRow {
	return make([]rawRow, 0, maxRawRowsPerShard)
}

func (rrs *rawRowsShard) updateFlushDeadline() {
	rrs.flushDeadlineMs.Store(time.Now().Add(pendingRowsFlushInterval).UnixMilli())
}

func (rrs *rawRowsShard) appendRawRowsToFlush(dst [][]rawRow, currentTimeMs int64, isFinal bool) [][]rawRow {
	flushDeadlineMs := rrs.flushDeadlineMs.Load()
	if !isFinal && currentTimeMs < flushDeadlineMs {
		// Fast path - nothing to flush
		return dst
	}

	// Slow path - move rrs.rows to dst.
	rrs.mu.Lock()
	dst = appendRawRowss(dst, rrs.rows)
	rrs.rows = rrs.rows[:0]
	rrs.mu.Unlock()

	return dst
}

func appendRawRowss(dst [][]rawRow, src []rawRow) [][]rawRow {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		dst = append(dst, newRawRows())
	}
	prows := &dst[len(dst)-1]
	n := copy((*prows)[len(*prows):cap(*prows)], src)
	*prows = (*prows)[:len(*prows)+n]
	src = src[n:]
	for len(src) > 0 {
		rows := newRawRows()
		n := copy(rows[:cap(rows)], src)
		rows = rows[:len(rows)+n]
		src = src[n:]
		dst = append(dst, rows)
	}
	return dst
}
