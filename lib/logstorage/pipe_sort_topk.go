package logstorage

import (
	"container/heap"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

func newPipeTopkProcessor(ps *pipeSort, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	ptp := &pipeTopkProcessor{
		ps:     ps,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		maxStateSize: maxStateSize,
	}
	ptp.shards.Init = func(shard *pipeTopkProcessorShard) {
		shard.ps = ps
	}
	ptp.stateSizeBudget.Store(maxStateSize)

	return ptp
}

type pipeTopkProcessor struct {
	ps     *pipeSort
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeTopkProcessorShard]

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeTopkProcessorShard struct {
	// ps points to the parent pipeSort.
	ps *pipeSort

	// partitionColumns contains 'partition by' columns
	partitionColumns []*blockResultColumn

	// rowsByPartition contains per-partition rows
	rowsByPartition map[string]*pipeTopkRows

	// partitionKey is a temporary buffer for constructing partition key
	partitionKey []byte

	// tmpRow is used as a temporary row when determining whether the next ingested row must be stored in the shard.
	tmpRow pipeTopkRow

	// these are aux fields for determining whether the next row must be stored in rows.
	byColumnValues  [][]string
	csOther         []*blockResultColumn
	byColumns       []string
	byColumnsIsTime []bool
	otherColumns    []Field

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeTopkProcessor.
	stateSizeBudget int
}

type pipeTopkRows struct {
	// ps points to the parent pipeSort.
	ps *pipeSort

	// rows contains rows tracked by the given shard.
	rows []*pipeTopkRow

	// rowNext points to the next index at rows during merge shards phase
	rowNext int
}

func (rs *pipeTopkRows) Len() int {
	return len(rs.rows)
}

func (rs *pipeTopkRows) Swap(i, j int) {
	rows := rs.rows
	rows[i], rows[j] = rows[j], rows[i]
}

func (rs *pipeTopkRows) Less(i, j int) bool {
	rows := rs.rows

	// This is max heap
	return topkLess(rs.ps, rows[j], rows[i])
}

func (rs *pipeTopkRows) Push(x any) {
	r := x.(*pipeTopkRow)
	rs.rows = append(rs.rows, r)
}

func (rs *pipeTopkRows) Pop() any {
	rows := rs.rows
	x := rows[len(rows)-1]
	rows[len(rows)-1] = nil
	rs.rows = rows[:len(rows)-1]
	return x
}

type pipeTopkRow struct {
	byColumns       []string
	byColumnsIsTime []bool
	otherColumns    []Field
	timestamp       int64
}

func (r *pipeTopkRow) init(byColumns []string, byColumnsIsTime []bool, timestamp int64) {
	r.byColumns = byColumns
	r.byColumnsIsTime = byColumnsIsTime
	r.otherColumns = nil
	r.timestamp = timestamp
}

func (r *pipeTopkRow) clone() *pipeTopkRow {
	byColumnsCopy := make([]string, len(r.byColumns))
	for i := range byColumnsCopy {
		byColumnsCopy[i] = strings.Clone(r.byColumns[i])
	}

	byColumnsIsTime := append([]bool{}, r.byColumnsIsTime...)

	otherColumnsCopy := make([]Field, len(r.otherColumns))
	for i := range otherColumnsCopy {
		src := &r.otherColumns[i]
		dst := &otherColumnsCopy[i]
		dst.Name = strings.Clone(src.Name)
		dst.Value = strings.Clone(src.Value)
	}

	return &pipeTopkRow{
		byColumns:       byColumnsCopy,
		byColumnsIsTime: byColumnsIsTime,
		otherColumns:    otherColumnsCopy,
		timestamp:       r.timestamp,
	}
}

func (r *pipeTopkRow) sizeBytes() int {
	n := int(unsafe.Sizeof(*r))

	for _, v := range r.byColumns {
		n += len(v)
	}
	n += len(r.byColumns) * int(unsafe.Sizeof(r.byColumns[0]))

	n += len(r.byColumnsIsTime) * int(unsafe.Sizeof(r.byColumnsIsTime[0]))

	for _, f := range r.otherColumns {
		n += len(f.Name) + len(f.Value)
	}
	n += len(r.otherColumns) * int(unsafe.Sizeof(r.otherColumns[0]))

	return n
}

// writeBlock writes br to shard.
func (shard *pipeTopkProcessorShard) writeBlock(br *blockResult) {
	cs := br.getColumns()

	byFields := shard.ps.byFields

	partitionColumns := shard.partitionColumns[:0]
	for _, f := range shard.ps.partitionByFields {
		c := br.getColumnByName(f)
		partitionColumns = append(partitionColumns, c)
	}
	shard.partitionColumns = partitionColumns

	if len(byFields) == 0 {
		// Sort by all the fields

		byColumnValues := shard.byColumnValues[:0]
		for _, c := range cs {
			values := c.getValues(br)
			byColumnValues = append(byColumnValues, values)
		}
		shard.byColumnValues = byColumnValues

		byColumns := slicesutil.SetLength(shard.byColumns, 1)
		byColumnsIsTime := slicesutil.SetLength(shard.byColumnsIsTime, 1)
		bb := bbPool.Get()
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			bb.B = bb.B[:0]
			for i, values := range byColumnValues {
				v := values[rowIdx]
				bb.B = marshalJSONKeyValue(bb.B, cs[i].name, v)
				bb.B = append(bb.B, ',')
			}
			byColumns[0] = bytesutil.ToUnsafeString(bb.B)
			byColumnsIsTime[0] = false

			shard.addRow(br, byColumns, byColumnsIsTime, cs, rowIdx, 0)
		}
		bbPool.Put(bb)
		shard.byColumns = byColumns
		shard.byColumnsIsTime = byColumnsIsTime
	} else {
		// Sort by byFields

		byColumnValues := slicesutil.SetLength(shard.byColumnValues, len(byFields))
		byColumnsIsTime := slicesutil.SetLength(shard.byColumnsIsTime, len(byFields))
		for i, bf := range byFields {
			c := br.getColumnByName(bf.name)

			byColumnsIsTime[i] = c.isTime

			var values []string
			if !c.isTime {
				values = c.getValues(br)
			}
			byColumnValues[i] = values
		}
		shard.byColumnValues = byColumnValues
		shard.byColumnsIsTime = byColumnsIsTime

		csOther := shard.csOther[:0]
		for _, c := range cs {
			isByField := false
			for _, bf := range byFields {
				if bf.name == c.name {
					isByField = true
					break
				}
			}
			if !isByField {
				csOther = append(csOther, c)
			}
		}
		shard.csOther = csOther

		// add rows to shard
		byColumns := slicesutil.SetLength(shard.byColumns, len(byFields))
		var timestamps []int64
		if slices.Contains(byColumnsIsTime, true) {
			timestamps = br.getTimestamps()
		}
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			for i, values := range byColumnValues {
				v := ""
				if !byColumnsIsTime[i] {
					v = values[rowIdx]
				}
				byColumns[i] = v
			}

			timestamp := int64(0)
			if timestamps != nil {
				timestamp = timestamps[rowIdx]
			}

			shard.addRow(br, byColumns, byColumnsIsTime, csOther, rowIdx, timestamp)
		}
		shard.byColumns = byColumns
	}
}

func (shard *pipeTopkProcessorShard) addRow(br *blockResult, byColumns []string, byColumnsIsTime []bool, csOther []*blockResultColumn, rowIdx int, timestamp int64) {
	// Construct partition key
	b := shard.partitionKey[:0]
	for _, c := range shard.partitionColumns {
		v := c.getValueAtRow(br, rowIdx)
		b = encoding.MarshalBytes(b, bytesutil.ToUnsafeBytes(v))
	}
	shard.partitionKey = b

	// Construct a temporary row
	r := &shard.tmpRow
	r.init(byColumns, byColumnsIsTime, timestamp)

	rs := shard.getRowsByPartition(bytesutil.ToUnsafeString(shard.partitionKey))
	maxRows := shard.ps.offset + shard.ps.limit
	if uint64(len(rs.rows)) >= maxRows && !topkLess(shard.ps, r, rs.rows[0]) {
		// Fast path - nothing to add.
		return
	}

	// Slow path - add r to rs.

	// Populate r.otherColumns
	otherColumns := slicesutil.SetLength(shard.otherColumns, len(csOther))
	for i, c := range csOther {
		v := c.getValueAtRow(br, rowIdx)
		otherColumns[i] = Field{
			Name:  c.name,
			Value: v,
		}
	}
	shard.otherColumns = otherColumns
	r.otherColumns = otherColumns

	// Clone r, so it doesn't refer the original data.
	r = r.clone()
	shard.stateSizeBudget -= r.sizeBytes()

	// Push r to rs.
	if uint64(len(rs.rows)) < maxRows {
		heap.Push(rs, r)
		shard.stateSizeBudget -= int(unsafe.Sizeof(r))
	} else {
		shard.stateSizeBudget += rs.rows[0].sizeBytes()
		rs.rows[0] = r
		heap.Fix(rs, 0)
	}
}

func (shard *pipeTopkProcessorShard) getRowsByPartition(partition string) *pipeTopkRows {
	if shard.rowsByPartition == nil {
		shard.rowsByPartition = make(map[string]*pipeTopkRows)
	}
	rs, ok := shard.rowsByPartition[partition]
	if !ok {
		rs = &pipeTopkRows{
			ps: shard.ps,
		}
		partition = strings.Clone(partition)
		shard.rowsByPartition[partition] = rs
		shard.stateSizeBudget += int(unsafe.Sizeof(*rs)+unsafe.Sizeof(rs)) + len(partition)
	}
	return rs
}

func (shard *pipeTopkProcessorShard) sortRows(stopCh <-chan struct{}) {
	for _, rs := range shard.rowsByPartition {
		rows := rs.rows
		for i := len(rows) - 1; i > 0; i-- {
			x := heap.Pop(rs)
			rows[i] = x.(*pipeTopkRow)

			if needStop(stopCh) {
				return
			}
		}
		rs.rows = rows
	}
}

func (ptp *pipeTopkProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := ptp.shards.Get(workerID)

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := ptp.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				ptp.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	shard.writeBlock(br)
}

func (ptp *pipeTopkProcessor) flush() error {
	if n := ptp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", ptp.ps.String(), ptp.maxStateSize/(1<<20))
	}

	if needStop(ptp.stopCh) {
		return nil
	}

	// Sort every shard in parallel
	shards := ptp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	for _, shard := range shards {
		wg.Add(1)
		go func(shard *pipeTopkProcessorShard) {
			defer wg.Done()
			shard.sortRows(ptp.stopCh)
		}(shard)
	}
	wg.Wait()

	if needStop(ptp.stopCh) {
		return nil
	}

	// Obtain all the partition keys
	partitionKeysMap := make(map[string]struct{})
	var partitionKeys []string
	for _, shard := range shards {
		for k := range shard.rowsByPartition {
			if _, ok := partitionKeysMap[k]; !ok {
				partitionKeysMap[k] = struct{}{}
				partitionKeys = append(partitionKeys, k)
			}
		}
	}
	sort.Strings(partitionKeys)

	// Merge sorted results across shards per each partitionKey
	for _, k := range partitionKeys {
		if needStop(ptp.stopCh) {
			return nil
		}
		var rss []*pipeTopkRows
		for _, shard := range shards {
			rs, ok := shard.rowsByPartition[k]
			if ok && len(rs.rows) > 0 {
				rss = append(rss, rs)
			}
		}
		ptp.mergeAndFlushRows(rss)
	}

	return nil
}

func (ptp *pipeTopkProcessor) mergeAndFlushRows(rss []*pipeTopkRows) {
	if len(rss) == 0 {
		return
	}
	rsh := pipeTopkRowsHeap(rss)

	heap.Init(&rsh)

	wctx := &pipeTopkWriteContext{
		ptp: ptp,
	}
	shardNextIdx := 0

	for len(rsh) > 1 {
		rs := rsh[0]
		if !wctx.writeNextRow(rs) {
			break
		}

		if rs.rowNext >= len(rs.rows) {
			_ = heap.Pop(&rsh)
			shardNextIdx = 0

			if needStop(ptp.stopCh) {
				return
			}

			continue
		}

		if shardNextIdx == 0 {
			shardNextIdx = 1
			if len(rsh) > 2 && rsh.Less(2, 1) {
				shardNextIdx = 2
			}
		}

		if rsh.Less(shardNextIdx, 0) {
			heap.Fix(&rsh, 0)
			shardNextIdx = 0

			if needStop(ptp.stopCh) {
				return
			}
		}
	}
	if len(rsh) == 1 {
		rs := rsh[0]
		for rs.rowNext < len(rs.rows) {
			if !wctx.writeNextRow(rs) {
				break
			}
		}
	}
	wctx.flush()
}

type pipeTopkWriteContext struct {
	ptp *pipeTopkProcessor
	rcs []resultColumn
	br  blockResult

	// buf is a temporary buffer for non-flushed block.
	buf []byte

	// rowsWritten is the total number of rows passed to writeNextRow.
	rowsWritten uint64

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeTopkWriteContext) writeNextRow(rs *pipeTopkRows) bool {
	ps := rs.ps
	rankFieldName := ps.rankFieldName
	rankFields := 0
	if rankFieldName != "" {
		rankFields = 1
	}

	rowIdx := rs.rowNext
	rs.rowNext++

	wctx.rowsWritten++
	if wctx.rowsWritten <= ps.offset {
		return true
	}
	if wctx.rowsWritten > ps.offset+ps.limit {
		return false
	}

	r := rs.rows[rowIdx]

	byFields := ps.byFields
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == rankFields+len(byFields)+len(r.otherColumns)
	if areEqualColumns {
		for i, c := range r.otherColumns {
			if rcs[rankFields+len(byFields)+i].name != c.Name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to ppNext and construct a block with new set of columns
		wctx.flush()

		rcs = wctx.rcs[:0]
		if rankFieldName != "" {
			rcs = appendResultColumnWithName(rcs, rankFieldName)
		}
		for _, bf := range byFields {
			rcs = appendResultColumnWithName(rcs, bf.name)
		}
		for _, c := range r.otherColumns {
			rcs = appendResultColumnWithName(rcs, c.Name)
		}
		wctx.rcs = rcs
	}

	if rankFieldName != "" {
		bufLen := len(wctx.buf)
		wctx.buf = marshalUint64String(wctx.buf, wctx.rowsWritten)
		v := bytesutil.ToUnsafeString(wctx.buf[bufLen:])
		rcs[0].addValue(v)
	}

	byColumns := r.byColumns
	byColumnsIsTime := r.byColumnsIsTime
	for i := range byFields {
		v := byColumns[i]
		if byColumnsIsTime[i] {
			v = string(marshalTimestampRFC3339NanoString(nil, r.timestamp))
		}
		rcs[rankFields+i].addValue(v)
		wctx.valuesLen += len(v)
	}

	for i, c := range r.otherColumns {
		v := c.Value
		rcs[rankFields+len(byFields)+i].addValue(v)
		wctx.valuesLen += len(v)
	}

	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}

	return true
}

func (wctx *pipeTopkWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.ptp.ppNext.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
	wctx.buf = wctx.buf[:0]
}

type pipeTopkRowsHeap []*pipeTopkRows

func (rsh *pipeTopkRowsHeap) Len() int {
	return len(*rsh)
}

func (rsh *pipeTopkRowsHeap) Swap(i, j int) {
	a := *rsh
	a[i], a[j] = a[j], a[i]
}

func (rsh *pipeTopkRowsHeap) Less(i, j int) bool {
	a := *rsh
	rsA := a[i]
	rsB := a[j]
	return topkLess(rsA.ps, rsA.rows[rsA.rowNext], rsB.rows[rsB.rowNext])
}

func (rsh *pipeTopkRowsHeap) Push(x any) {
	rs := x.(*pipeTopkRows)
	*rsh = append(*rsh, rs)
}

func (rsh *pipeTopkRowsHeap) Pop() any {
	a := *rsh
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*rsh = a[:len(a)-1]
	return x
}

func topkLess(ps *pipeSort, a, b *pipeTopkRow) bool {
	byFields := ps.byFields

	csA := a.byColumns
	isTimeA := a.byColumnsIsTime

	csB := b.byColumns
	isTimeB := b.byColumnsIsTime

	for i := range csA {
		isDesc := ps.isDesc
		if len(byFields) > 0 && byFields[i].isDesc {
			isDesc = !isDesc
		}

		if isTimeA[i] && isTimeB[i] {
			// Fast path - compare timestamps
			if a.timestamp == b.timestamp {
				continue
			}
			if isDesc {
				return b.timestamp < a.timestamp
			}
			return a.timestamp < b.timestamp
		}

		vA := csA[i]
		vB := csB[i]

		var bb *bytesutil.ByteBuffer

		if isTimeA[i] || isTimeB[i] {
			bb = bbPool.Get()
		}
		if isTimeA[i] {
			bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], a.timestamp)
			vA = bytesutil.ToUnsafeString(bb.B)
		} else if isTimeB[i] {
			bb.B = marshalTimestampRFC3339NanoString(bb.B[:0], a.timestamp)
			vB = bytesutil.ToUnsafeString(bb.B)
		}

		if vA == vB {
			if bb != nil {
				bbPool.Put(bb)
			}
			continue
		}

		if isDesc {
			vA, vB = vB, vA
		}
		ok := lessString(vA, vB)
		if bb != nil {
			bbPool.Put(bb)
		}
		return ok
	}
	return false
}

func lessString(a, b string) bool {
	if a == b {
		return false
	}

	if iA, okA := tryParseInt64(a); okA {
		if iB, okB := tryParseInt64(b); okB {
			return iA < iB
		}
	}

	if uA, okA := tryParseUint64(a); okA {
		if uB, okB := tryParseUint64(b); okB {
			return uA < uB
		}
	}

	if tsA, okA := TryParseTimestampRFC3339Nano(a); okA {
		if tsB, okB := TryParseTimestampRFC3339Nano(b); okB {
			return tsA < tsB
		}
	}

	if fA, okA := tryParseNumber(a); okA {
		if fB, okB := tryParseNumber(b); okB {
			return fA < fB
		}
	}

	return stringsutil.LessNatural(a, b)
}
