package storage

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

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
	rrss.shards = make([]rawRowsShard, rawRowsShardsPerPartition)
}

func (rrss *rawRowsShards) addRows(pt *partition, rows []rawRow) {
	shards := rrss.shards
	shardsLen := uint32(len(shards))
	for len(rows) > 0 {
		n := rrss.shardIdx.Add(1)
		idx := n % shardsLen
		tailRows, rowsToFlush := shards[idx].addRows(rows)
		rrss.addRowsToFlush(pt, rowsToFlush)
		rows = tailRows
	}
}

func (rrss *rawRowsShards) addRowsToFlush(pt *partition, rowsToFlush []rawRow) {
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

	pt.flushRowssToInmemoryParts(rowssToMerge)
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

func (rrss *rawRowsShards) updateFlushDeadline() {
	rrss.flushDeadlineMs.Store(time.Now().Add(pendingRowsFlushInterval).UnixMilli())
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
