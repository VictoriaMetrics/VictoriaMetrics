package logstorage

import (
	"container/heap"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

func newPipeTopkProcessor(ps *pipeSort, workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeTopkProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeTopkProcessorShard{
			pipeTopkProcessorShardNopad: pipeTopkProcessorShardNopad{
				ps: ps,
			},
		}
	}

	ptp := &pipeTopkProcessor{
		ps:     ps,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	ptp.stateSizeBudget.Store(maxStateSize)

	return ptp
}

type pipeTopkProcessor struct {
	ps     *pipeSort
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards []pipeTopkProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeTopkProcessorShard struct {
	pipeTopkProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeTopkProcessorShardNopad{})%128]byte
}

type pipeTopkProcessorShardNopad struct {
	// ps points to the parent pipeSort.
	ps *pipeSort

	// rows contains rows tracked by the given shard.
	rows []*pipeTopkRow

	// rowNext points to the next index at rows during merge shards phase
	rowNext int

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

type pipeTopkRow struct {
	byColumns       []string
	byColumnsIsTime []bool
	otherColumns    []Field
	timestamp       int64
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

func (shard *pipeTopkProcessorShard) Len() int {
	return len(shard.rows)
}

func (shard *pipeTopkProcessorShard) Swap(i, j int) {
	rows := shard.rows
	rows[i], rows[j] = rows[j], rows[i]
}

func (shard *pipeTopkProcessorShard) Less(i, j int) bool {
	rows := shard.rows

	// This is max heap
	return topkLess(shard.ps, rows[j], rows[i])
}

func (shard *pipeTopkProcessorShard) Push(x any) {
	r := x.(*pipeTopkRow)
	shard.rows = append(shard.rows, r)
}

func (shard *pipeTopkProcessorShard) Pop() any {
	rows := shard.rows
	x := rows[len(rows)-1]
	rows[len(rows)-1] = nil
	shard.rows = rows[:len(rows)-1]
	return x
}

// writeBlock writes br to shard.
func (shard *pipeTopkProcessorShard) writeBlock(br *blockResult) {
	cs := br.getColumns()

	byFields := shard.ps.byFields
	if len(byFields) == 0 {
		// Sort by all the fields

		byColumnValues := shard.byColumnValues[:0]
		for _, c := range cs {
			values := c.getValues(br)
			byColumnValues = append(byColumnValues, values)
		}
		shard.byColumnValues = byColumnValues

		byColumns := shard.byColumns[:0]
		byColumnsIsTime := shard.byColumnsIsTime[:0]
		bb := bbPool.Get()
		timestamps := br.getTimestamps()
		for rowIdx, timestamp := range timestamps {
			byColumns = byColumns[:0]
			bb.B = bb.B[:0]
			for i, values := range byColumnValues {
				v := values[rowIdx]
				bb.B = marshalJSONKeyValue(bb.B, cs[i].name, v)
				bb.B = append(bb.B, ',')
			}
			byColumns = append(byColumns, bytesutil.ToUnsafeString(bb.B))
			byColumnsIsTime = append(byColumnsIsTime, false)

			shard.addRow(br, byColumns, byColumnsIsTime, cs, rowIdx, timestamp)
		}
		bbPool.Put(bb)
		shard.byColumns = byColumns
		shard.byColumnsIsTime = byColumnsIsTime
	} else {
		// Sort by byFields

		byColumnValues := shard.byColumnValues[:0]
		byColumnsIsTime := shard.byColumnsIsTime[:0]
		for _, bf := range byFields {
			c := br.getColumnByName(bf.name)

			byColumnsIsTime = append(byColumnsIsTime, c.isTime)

			var values []string
			if !c.isTime {
				values = c.getValues(br)
			}
			byColumnValues = append(byColumnValues, values)
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
		byColumns := shard.byColumns[:0]
		timestamps := br.getTimestamps()
		for rowIdx, timestamp := range timestamps {
			byColumns = byColumns[:0]

			for i, values := range byColumnValues {
				v := ""
				if !byColumnsIsTime[i] {
					v = values[rowIdx]
				}
				byColumns = append(byColumns, v)
			}

			shard.addRow(br, byColumns, byColumnsIsTime, csOther, rowIdx, timestamp)
		}
		shard.byColumns = byColumns
	}
}

func (shard *pipeTopkProcessorShard) addRow(br *blockResult, byColumns []string, byColumnsIsTime []bool, csOther []*blockResultColumn, rowIdx int, timestamp int64) {
	r := &shard.tmpRow
	r.byColumns = byColumns
	r.byColumnsIsTime = byColumnsIsTime
	r.timestamp = timestamp

	rows := shard.rows
	maxRows := shard.ps.offset + shard.ps.limit
	if uint64(len(rows)) >= maxRows && !topkLess(shard.ps, r, rows[0]) {
		// Fast path - nothing to add.
		return
	}

	// Slow path - add r to shard.rows.

	// Populate r.otherColumns
	otherColumns := shard.otherColumns[:0]
	for _, c := range csOther {
		v := c.getValueAtRow(br, rowIdx)
		otherColumns = append(otherColumns, Field{
			Name:  c.name,
			Value: v,
		})
	}
	shard.otherColumns = otherColumns
	r.otherColumns = otherColumns

	// Clone r, so it doesn't refer the original data.
	r = r.clone()
	shard.stateSizeBudget -= r.sizeBytes()

	// Push r to shard.rows.
	if uint64(len(rows)) < maxRows {
		heap.Push(shard, r)
		shard.stateSizeBudget -= int(unsafe.Sizeof(r))
	} else {
		shard.stateSizeBudget += rows[0].sizeBytes()
		rows[0] = r
		heap.Fix(shard, 0)
	}
}

func (shard *pipeTopkProcessorShard) sortRows(stopCh <-chan struct{}) {
	rows := shard.rows
	for i := len(rows) - 1; i > 0; i-- {
		x := heap.Pop(shard)
		rows[i] = x.(*pipeTopkRow)

		if needStop(stopCh) {
			return
		}
	}
	shard.rows = rows
}

func (ptp *pipeTopkProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &ptp.shards[workerID]

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
	var wg sync.WaitGroup
	shards := ptp.shards
	for i := range shards {
		wg.Add(1)
		go func(shard *pipeTopkProcessorShard) {
			shard.sortRows(ptp.stopCh)
			wg.Done()
		}(&shards[i])
	}
	wg.Wait()

	if needStop(ptp.stopCh) {
		return nil
	}

	// Merge sorted results across shards
	sh := pipeTopkProcessorShardsHeap(make([]*pipeTopkProcessorShard, 0, len(shards)))
	for i := range shards {
		shard := &shards[i]
		if len(shard.rows) > 0 {
			sh = append(sh, shard)
		}
	}
	if len(sh) == 0 {
		return nil
	}

	heap.Init(&sh)

	wctx := &pipeTopkWriteContext{
		ptp: ptp,
	}
	shardNextIdx := 0

	for len(sh) > 1 {
		shard := sh[0]
		if !wctx.writeNextRow(shard) {
			break
		}

		if shard.rowNext >= len(shard.rows) {
			_ = heap.Pop(&sh)
			shardNextIdx = 0

			if needStop(ptp.stopCh) {
				return nil
			}

			continue
		}

		if shardNextIdx == 0 {
			shardNextIdx = 1
			if len(sh) > 2 && sh.Less(2, 1) {
				shardNextIdx = 2
			}
		}

		if sh.Less(shardNextIdx, 0) {
			heap.Fix(&sh, 0)
			shardNextIdx = 0

			if needStop(ptp.stopCh) {
				return nil
			}
		}
	}
	if len(sh) == 1 {
		shard := sh[0]
		for shard.rowNext < len(shard.rows) {
			if !wctx.writeNextRow(shard) {
				break
			}
		}
	}
	wctx.flush()

	return nil
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

func (wctx *pipeTopkWriteContext) writeNextRow(shard *pipeTopkProcessorShard) bool {
	ps := shard.ps
	rankName := ps.rankName
	rankFields := 0
	if rankName != "" {
		rankFields = 1
	}

	rowIdx := shard.rowNext
	shard.rowNext++

	wctx.rowsWritten++
	if wctx.rowsWritten <= ps.offset {
		return true
	}
	if wctx.rowsWritten > ps.offset+ps.limit {
		return false
	}

	r := shard.rows[rowIdx]

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
		if rankName != "" {
			rcs = appendResultColumnWithName(rcs, rankName)
		}
		for _, bf := range byFields {
			rcs = appendResultColumnWithName(rcs, bf.name)
		}
		for _, c := range r.otherColumns {
			rcs = appendResultColumnWithName(rcs, c.Name)
		}
		wctx.rcs = rcs
	}

	if rankName != "" {
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

type pipeTopkProcessorShardsHeap []*pipeTopkProcessorShard

func (sh *pipeTopkProcessorShardsHeap) Len() int {
	return len(*sh)
}

func (sh *pipeTopkProcessorShardsHeap) Swap(i, j int) {
	a := *sh
	a[i], a[j] = a[j], a[i]
}

func (sh *pipeTopkProcessorShardsHeap) Less(i, j int) bool {
	a := *sh
	shardA := a[i]
	shardB := a[j]
	return topkLess(shardA.ps, shardA.rows[shardA.rowNext], shardB.rows[shardB.rowNext])
}

func (sh *pipeTopkProcessorShardsHeap) Push(x any) {
	shard := x.(*pipeTopkProcessorShard)
	*sh = append(*sh, shard)
}

func (sh *pipeTopkProcessorShardsHeap) Pop() any {
	a := *sh
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*sh = a[:len(a)-1]
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

	nA, okA := tryParseUint64(a)
	nB, okB := tryParseUint64(b)
	if okA && okB {
		return nA < nB
	}

	fA, okA := tryParseNumber(a)
	fB, okB := tryParseNumber(b)
	if okA && okB {
		return fA < fB
	}

	return stringsutil.LessNatural(a, b)
}
