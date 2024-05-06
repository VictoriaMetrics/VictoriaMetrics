package logstorage

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// pipeSort processes '| sort ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe
type pipeSort struct {
	// byFields contains field names for sorting from 'by(...)' clause.
	byFields []*bySortField
}

func (ps *pipeSort) String() string {
	if len(ps.byFields) == 0 {
		logger.Panicf("BUG: pipeSort must contain at least a single byField")
	}

	a := make([]string, len(ps.byFields))
	for i := range ps.byFields {
		a[i] = ps.byFields[i].String()
	}
	s := "sort by (" + strings.Join(a, ", ") + ")"

	return s
}

func (ps *pipeSort) getNeededFields() ([]string, map[string][]string) {
	byFields := ps.byFields
	neededFields := make([]string, len(byFields))
	for i := range byFields {
		neededFields[i] = byFields[i].name
	}
	return neededFields, nil
}

func (ps *pipeSort) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.3)

	shards := make([]pipeSortProcessorShard, workersCount)
	for i := range shards {
		shard := &shards[i]
		shard.ps = ps
		shard.stateSizeBudget = stateSizeBudgetChunk
		maxStateSize -= stateSizeBudgetChunk
	}

	psp := &pipeSortProcessor{
		ps:     ps,
		stopCh: stopCh,
		cancel: cancel,
		ppBase: ppBase,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	psp.stateSizeBudget.Store(maxStateSize)

	return psp
}

type pipeSortProcessor struct {
	ps     *pipeSort
	stopCh <-chan struct{}
	cancel func()
	ppBase pipeProcessor

	shards []pipeSortProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeSortProcessorShard struct {
	pipeSortProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeSortProcessorShardNopad{})%128]byte
}

type pipeSortProcessorShardNopad struct {
	// ps point to the parent pipeSort.
	ps *pipeSort

	// buf holds all the logs data written to the given shard.
	buf []byte

	// valuesBuf holds all the string values written to the given shard
	// The actual strings are stored in buf.
	valuesBuf []string

	// u64ValuesBuf holds uint64 values parsed from valuesBuf for speeding up the sorting.
	u64ValuesBuf []uint64

	// f64ValuesBuf holds float64 values parsed from valuesBuf for speeding up the sorting.
	f64ValuesBuf []float64

	// timestampsBuf holds timestamps if _time columns are used for sorting.
	// This speeds up sorting by _time.
	timestampsBuf []int64

	// byColumnsBuf holds `by(...)` columns written to the shard.
	byColumnsBuf []sortBlockByColumn

	// otherColumnsBuf holds other than `by(...)` columns written to the shard.
	otherColumnsBuf []sortBlockOtherColumn

	// blocks holds all the blocks with logs written to the shard.
	blocks []sortBlock

	// rowRefs holds references to all the rows stored in blocks.
	//
	// Sorting sorts rowRefs, while blocks remain unchanged.
	// This should speed up sorting.
	rowRefs []sortRowRef

	// rowRefNext points to the next index at rowRefs during merge shards phase
	rowRefNext uint

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeSortProcessor.
	stateSizeBudget int
}

// sortBlock represents a block of logs for sorting.
//
// It doesn't own the data it refers - all the data belongs to pipeSortProcessorShard.
type sortBlock struct {
	// byColumns refers block data for 'by(...)' columns
	byColumns []sortBlockByColumn

	// otherColumns refers block data for other than 'by(...)' columns
	otherColumns []sortBlockOtherColumn
}

// sortBlockByColumn represents data for a single column from 'sort by(...)' clause.
//
// It doesn't own the data it refers - all the data belongs to pipeSortProcessorShard.
type sortBlockByColumn struct {
	// values contains column values
	values []string

	// u64Values contains uint6464 numbers parsed from values
	u64Values []uint64

	// f64Values contains float64 numbers parsed from values
	f64Values []float64

	// timestamps contains timestamps for blockResultColumn.isTime column
	timestamps []int64
}

// sortBlockOtherColumn represents data for a single column outside 'sort by(...)' clause.
//
// It doesn't own the data it refers - all the data belongs to pipeSortProcessorShard.
type sortBlockOtherColumn struct {
	// name is the column name
	name string

	// values contains column values
	values []string
}

// sortRowRef is the reference to a single log entry written to `sort` pipe.
type sortRowRef struct {
	// blockIdx is the index of the block at pipeSortProcessorShard.blocks.
	blockIdx uint

	// rowIdx is the index of the log entry inside the block referenced by blockIdx.
	rowIdx uint
}

// writeBlock writes br with the given byFields to shard.
func (shard *pipeSortProcessorShard) writeBlock(br *blockResult) {
	byFields := shard.ps.byFields
	cs := br.getColumns()

	// Collect values for columns from byFields.
	byColumnsBuf := shard.byColumnsBuf
	byColumnsBufLen := len(byColumnsBuf)
	for _, bf := range byFields {
		c := br.getColumnByName(bf.name)
		values := c.getValues(br)
		values = shard.copyValues(values)
		u64Values := shard.createUint64Values(values)
		f64Values := shard.createFloat64Values(values)
		timestamps := shard.createTimestampsIfNeeded(br.timestamps, c.isTime)
		byColumnsBuf = append(byColumnsBuf, sortBlockByColumn{
			values:     values,
			u64Values:  u64Values,
			f64Values:  f64Values,
			timestamps: timestamps,
		})
	}
	shard.byColumnsBuf = byColumnsBuf
	byColumns := byColumnsBuf[byColumnsBufLen:]
	shard.stateSizeBudget -= len(byColumns) * int(unsafe.Sizeof(byColumns[0]))

	// Collect values for other columns.
	otherColumnsBuf := shard.otherColumnsBuf
	otherColumnsBufLen := len(otherColumnsBuf)
	for _, c := range cs {
		isByField := false
		for _, bf := range byFields {
			if bf.name == c.name {
				isByField = true
				break
			}
		}
		if isByField {
			continue
		}

		values := c.getValues(br)
		values = shard.copyValues(values)
		otherColumnsBuf = append(otherColumnsBuf, sortBlockOtherColumn{
			name:   c.name,
			values: values,
		})
	}
	shard.otherColumnsBuf = otherColumnsBuf
	otherColumns := otherColumnsBuf[otherColumnsBufLen:]
	shard.stateSizeBudget -= len(otherColumns) * int(unsafe.Sizeof(otherColumns[0]))

	// Add row references to rowRefs.
	blockIdx := uint(len(shard.blocks))
	rowRefs := shard.rowRefs
	rowRefsLen := len(rowRefs)
	for i := range br.timestamps {
		rowRefs = append(rowRefs, sortRowRef{
			blockIdx: blockIdx,
			rowIdx:   uint(i),
		})
	}
	shard.rowRefs = rowRefs
	shard.stateSizeBudget -= (len(rowRefs) - rowRefsLen) * int(unsafe.Sizeof(rowRefs[0]))

	// Add byColumns and otherColumns to blocks.
	shard.blocks = append(shard.blocks, sortBlock{
		byColumns:    byColumns,
		otherColumns: otherColumns,
	})
	shard.stateSizeBudget -= int(unsafe.Sizeof(shard.blocks[0]))
}

// copyValues copies values to the shard and returns the copied values.
func (shard *pipeSortProcessorShard) copyValues(values []string) []string {
	buf := shard.buf
	bufLenOriginal := len(buf)

	valuesBuf := shard.valuesBuf
	valuesBufLen := len(valuesBuf)

	for _, v := range values {
		bufLen := len(buf)
		buf = append(buf, v...)
		valuesBuf = append(valuesBuf, bytesutil.ToUnsafeString(buf[bufLen:]))
	}

	shard.valuesBuf = valuesBuf
	shard.buf = buf

	shard.stateSizeBudget -= len(buf) - bufLenOriginal
	shard.stateSizeBudget -= (len(valuesBuf) - valuesBufLen) * int(unsafe.Sizeof(valuesBuf[0]))

	return valuesBuf[valuesBufLen:]
}

func (shard *pipeSortProcessorShard) createUint64Values(values []string) []uint64 {
	u64ValuesBuf := shard.u64ValuesBuf
	u64ValuesBufLen := len(u64ValuesBuf)
	for _, v := range values {
		u64, ok := tryParseUint64(v)
		if ok {
			u64ValuesBuf = append(u64ValuesBuf, u64)
		}
		u32, ok := tryParseIPv4(v)
		if ok {
			u64ValuesBuf = append(u64ValuesBuf, uint64(u32))
		}
		i64, ok := tryParseTimestampRFC3339Nano(v)
		if ok {
			u64ValuesBuf = append(u64ValuesBuf, uint64(i64))
		}
		i64, ok = tryParseDuration(v)
		u64ValuesBuf = append(u64ValuesBuf, uint64(i64))
	}
	shard.u64ValuesBuf = u64ValuesBuf

	shard.stateSizeBudget -= (len(u64ValuesBuf) - u64ValuesBufLen) * int(unsafe.Sizeof(u64ValuesBuf[0]))

	return u64ValuesBuf[u64ValuesBufLen:]
}

func (shard *pipeSortProcessorShard) createFloat64Values(values []string) []float64 {
	f64ValuesBuf := shard.f64ValuesBuf
	f64ValuesBufLen := len(f64ValuesBuf)
	for _, v := range values {
		f, ok := tryParseFloat64(v)
		if !ok {
			f = nan
		}
		f64ValuesBuf = append(f64ValuesBuf, f)
	}
	shard.f64ValuesBuf = f64ValuesBuf

	shard.stateSizeBudget -= (len(f64ValuesBuf) - f64ValuesBufLen) * int(unsafe.Sizeof(f64ValuesBuf[0]))

	return f64ValuesBuf[f64ValuesBufLen:]
}

func (shard *pipeSortProcessorShard) createTimestampsIfNeeded(timestamps []int64, isTime bool) []int64 {
	if !isTime {
		return nil
	}

	timestampsBufLen := len(shard.timestampsBuf)
	shard.timestampsBuf = append(shard.timestampsBuf, timestamps...)
	shard.stateSizeBudget -= (len(timestamps) - timestampsBufLen) * int(unsafe.Sizeof(timestamps[0]))

	return shard.timestampsBuf[timestampsBufLen:]
}

func (psp *pipeSortProcessorShard) Len() int {
	return len(psp.rowRefs)
}

func (psp *pipeSortProcessorShard) Swap(i, j int) {
	rowRefs := psp.rowRefs
	rowRefs[i], rowRefs[j] = rowRefs[j], rowRefs[i]
}

func (psp *pipeSortProcessorShard) Less(i, j int) bool {
	return sortBlockLess(psp, uint(i), psp, uint(j))
}

func (psp *pipeSortProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &psp.shards[workerID]

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := psp.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				psp.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	shard.writeBlock(br)
}

func (psp *pipeSortProcessor) flush() error {
	if n := psp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", psp.ps.String(), psp.maxStateSize/(1<<20))
	}

	select {
	case <-psp.stopCh:
		return nil
	default:
	}

	// Sort every shard in parallel
	var wg sync.WaitGroup
	shards := psp.shards
	for i := range shards {
		wg.Add(1)
		go func(shard *pipeSortProcessorShard) {
			// TODO: interrupt long sorting when psp.stopCh is closed.
			sort.Sort(shard)
			wg.Done()
		}(&shards[i])
	}
	wg.Wait()

	select {
	case <-psp.stopCh:
		return nil
	default:
	}

	// Merge sorted results across shards
	sh := pipeSortProcessorShardsHeap(make([]*pipeSortProcessorShard, 0, len(shards)))
	for i := range shards {
		shard := &shards[i]
		if shard.Len() > 0 {
			sh = append(sh, shard)
		}
	}
	heap.Init(&sh)

	wctx := &pipeSortWriteContext{
		psp: psp,
	}
	var shardNext *pipeSortProcessorShard
	for len(sh) > 1 {
		shard := sh[0]
		wctx.writeRow(shard, shard.rowRefNext)
		shard.rowRefNext++

		if shard.rowRefNext >= uint(len(shard.rowRefs)) {
			_ = heap.Pop(&sh)
			shardNext = nil
			continue
		}

		if shardNext == nil {
			shardNext = sh[1]
			if len(sh) > 2 && sortBlockLess(sh[2], sh[2].rowRefNext, shardNext, shardNext.rowRefNext) {
				shardNext = sh[2]
			}
		}

		if sortBlockLess(shardNext, shardNext.rowRefNext, shard, shard.rowRefNext) {d
			heap.Fix(&sh, 0)
			shardNext = nil

			select {
			case <-psp.stopCh:
				return nil
			default:
			}
		}
	}
	if len(sh) == 1 {
		shard := sh[0]
		for shard.rowRefNext < uint(len(shard.rowRefs)) {
			wctx.writeRow(shard, shard.rowRefNext)
			shard.rowRefNext++
		}
	}
	wctx.flush()

	return nil
}

type pipeSortWriteContext struct {
	psp       *pipeSortProcessor
	rcs       []resultColumn
	br        blockResult
	valuesLen int
}

func (wctx *pipeSortWriteContext) writeRow(shard *pipeSortProcessorShard, rowIdx uint) {
	rowRef := shard.rowRefs[rowIdx]
	block := &shard.blocks[rowRef.blockIdx]

	byFields := shard.ps.byFields
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == len(byFields)+len(block.otherColumns)
	if areEqualColumns {
		for i, c := range block.otherColumns {
			if rcs[len(byFields)+i].name != c.name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to bbBase and construct new columns
		wctx.flush()

		rcs = rcs[:0]
		for _, bf := range byFields {
			rcs = append(rcs, resultColumn{
				name: bf.name,
			})
		}
		for _, c := range block.otherColumns {
			rcs = append(rcs, resultColumn{
				name: c.name,
			})
		}
		wctx.rcs = rcs
	}

	for i, c := range block.byColumns {
		v := c.values[rowRef.rowIdx]
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}
	for i, c := range block.otherColumns {
		v := c.values[rowRef.rowIdx]
		rcs[len(byFields)+i].addValue(v)
		wctx.valuesLen += len(v)
	}

	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeSortWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	if len(rcs) == 0 {
		return
	}

	// Flush rcs to ppBase
	br.setResultColumns(rcs)
	wctx.psp.ppBase.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].reset()
	}
}

type pipeSortProcessorShardsHeap []*pipeSortProcessorShard

func (sh *pipeSortProcessorShardsHeap) Len() int {
	return len(*sh)
}

func (sh *pipeSortProcessorShardsHeap) Swap(i, j int) {
	a := *sh
	a[i], a[j] = a[j], a[i]
}

func (sh *pipeSortProcessorShardsHeap) Less(i, j int) bool {
	a := *sh
	shardA := a[i]
	shardB := a[j]
	return sortBlockLess(shardA, shardA.rowRefNext, shardB, shardB.rowRefNext)
}

func (sh *pipeSortProcessorShardsHeap) Push(x any) {
	shard := x.(*pipeSortProcessorShard)
	*sh = append(*sh, shard)
}

func (sh *pipeSortProcessorShardsHeap) Pop() any {
	a := *sh
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*sh = a[:len(a)-1]
	return x
}

func sortBlockLess(shardA *pipeSortProcessorShard, rowIdxA uint, shardB *pipeSortProcessorShard, rowIdxB uint) bool {
	byFields := shardA.ps.byFields

	rowRefA := shardA.rowRefs[rowIdxA]
	rowRefB := shardB.rowRefs[rowIdxB]
	csA := shardA.blocks[rowRefA.blockIdx].byColumns
	csB := shardB.blocks[rowRefB.blockIdx].byColumns
	for idx := range csA {
		cA := &csA[idx]
		cB := &csB[idx]
		bf := byFields[idx]

		if len(cA.timestamps) > 0 && len(cB.timestamps) > 0 {
			// Fast path - sort by _time
			tA := cA.timestamps[rowIdxA]
			tB := cB.timestamps[rowIdxB]
			if tA == tB {
				continue
			}
			if bf.isDesc {
				return tB < tA
			}
			return tA < tB
		}

		// Try sorting by uint64 values at first
		uA := cA.u64Values[rowIdxA]
		uB := cB.u64Values[rowIdxB]
		if uA != 0 && uB != 0 {
			if uA == uB {
				continue
			}
			if bf.isDesc {
				return uB < uA
			}
			return uA < uB
		}

		// Try sorting by float64 then
		fA := cA.f64Values[rowIdxA]
		fB := cB.f64Values[rowIdxB]
		if !math.IsNaN(fA) && !math.IsNaN(fB) {
			if fA == fB {
				continue
			}
			if bf.isDesc {
				return fB < fA
			}
			return fA < fB
		}

		// Fall back to string sorting
		sA := cA.values[rowIdxA]
		sB := cB.values[rowIdxB]
		if sA == sB {
			continue
		}
		if bf.isDesc {
			return sB < sA
		}
		return sA < sB
	}
	return false
}

func parsePipeSort(lex *lexer) (*pipeSort, error) {
	if !lex.isKeyword("sort") {
		return nil, fmt.Errorf("expecting 'sort'; got %q", lex.token)
	}
	lex.nextToken()
	if !lex.isKeyword("by") {
		return nil, fmt.Errorf("expecting 'by'; got %q", lex.token)
	}
	lex.nextToken()
	bfs, err := parseBySortFields(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'by' clause: %w", err)
	}

	ps := &pipeSort{
		byFields: bfs,
	}
	return ps, nil
}

// bySortField represents 'by (...)' part of the pipeSort.
type bySortField struct {
	// the name of the field to sort
	name string

	// whether the sorting for the given field in descending order
	isDesc bool
}

func (bf *bySortField) String() string {
	s := quoteTokenIfNeeded(bf.name)
	if bf.isDesc {
		s += " desc"
	}
	return s
}

func parseBySortFields(lex *lexer) ([]*bySortField, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing `(`")
	}
	var bfs []*bySortField
	for {
		lex.nextToken()
		if lex.isKeyword(")") {
			lex.nextToken()
			if len(bfs) == 0 {
				return nil, fmt.Errorf("sort fields list cannot be empty")
			}
			return bfs, nil
		}
		fieldName, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		bf := &bySortField{
			name: fieldName,
		}
		if lex.isKeyword("desc") {
			lex.nextToken()
			bf.isDesc = true
		}
		bfs = append(bfs, bf)
		switch {
		case lex.isKeyword(")"):
			lex.nextToken()
			return bfs, nil
		case lex.isKeyword(","):
		default:
			return nil, fmt.Errorf("unexpected token: %q; expecting ',' or ')'", lex.token)
		}
	}
}
