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
	fields := make([]string, len(ps.byFields))
	for i, bf := range ps.byFields {
		fields[i] = bf.name
	}
	m := map[string][]string{
		"*": fields,
	}
	return []string{"*"}, m
}

func (ps *pipeSort) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

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

	// u64ValuesBuf holds uint64 values parsed from values for speeding up the sorting.
	u64ValuesBuf []uint64

	// f64ValuesBuf holds float64 values parsed from values for speeding up the sorting.
	f64ValuesBuf []float64

	// blocks holds all the blocks with logs written to the shard.
	blocks []sortBlock

	// rowRefs holds references to all the rows stored in blocks.
	//
	// Sorting sorts rowRefs, while blocks remain unchanged. This should speed up sorting.
	rowRefs []sortRowRef

	// rowRefNext points to the next index at rowRefs during merge shards phase
	rowRefNext int

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeSortProcessor.
	stateSizeBudget int
}

// sortBlock represents a block of logs for sorting.
type sortBlock struct {
	// br is a result block to sort
	br *blockResult

	// byColumns refers block data for 'by(...)' columns
	byColumns []sortBlockByColumn

	// otherColumns refers block data for other than 'by(...)' columns
	otherColumns []blockResultColumn
}

// sortBlockByColumn represents data for a single column from 'sort by(...)' clause.
type sortBlockByColumn struct {
	// c contains column data
	c blockResultColumn

	// u64Values contains uint64 numbers parsed from values
	u64Values []uint64

	// f64Values contains float64 numbers parsed from values
	f64Values []float64
}

// sortRowRef is the reference to a single log entry written to `sort` pipe.
type sortRowRef struct {
	// blockIdx is the index of the block at pipeSortProcessorShard.blocks.
	blockIdx int

	// rowIdx is the index of the log entry inside the block referenced by blockIdx.
	rowIdx int
}

func (c *sortBlockByColumn) getU64ValueAtRow(rowIdx int) uint64 {
	if c.c.isConst {
		return c.u64Values[0]
	}
	return c.u64Values[rowIdx]
}

func (c *sortBlockByColumn) getF64ValueAtRow(rowIdx int) float64 {
	if c.c.isConst {
		return c.f64Values[0]
	}
	return c.f64Values[rowIdx]
}

// writeBlock writes br with the given byFields to shard.
func (shard *pipeSortProcessorShard) writeBlock(br *blockResult) {
	// clone br, so it could be owned by shard
	br = br.clone()

	byFields := shard.ps.byFields

	// Collect values for columns from byFields.
	byColumns := make([]sortBlockByColumn, len(byFields))
	for i, bf := range byFields {
		c := br.getColumnByName(bf.name)
		bc := &byColumns[i]
		bc.c = c

		if c.isTime {
			// Do not initialize bc.values, bc.u64Values and bc.f64Values, since they aren't used.
			// This saves some memory.
			continue
		}
		if c.isConst {
			// Do not initialize bc.values in order to save some memory.
			bc.u64Values = shard.createUint64Values(c.encodedValues)
			bc.f64Values = shard.createFloat64Values(c.encodedValues)
			continue
		}

		// pre-populate values in order to track better br memory usage
		values := c.getValues(br)
		bc.u64Values = shard.createUint64Values(values)
		bc.f64Values = shard.createFloat64Values(values)
	}
	shard.stateSizeBudget -= len(byColumns) * int(unsafe.Sizeof(byColumns[0]))

	// Collect values for other columns.
	cs := br.getColumns()
	otherColumns := make([]blockResultColumn, 0, len(cs))
	for _, c := range cs {
		isByField := false
		for _, bf := range byFields {
			if bf.name == c.name {
				isByField = true
				break
			}
		}
		if !isByField {
			otherColumns = append(otherColumns, c)
		}
	}
	shard.stateSizeBudget -= len(otherColumns) * int(unsafe.Sizeof(otherColumns[0]))

	// Add row references to rowRefs.
	blockIdx := len(shard.blocks)
	rowRefs := shard.rowRefs
	rowRefsLen := len(rowRefs)
	for i := range br.timestamps {
		rowRefs = append(rowRefs, sortRowRef{
			blockIdx: blockIdx,
			rowIdx:   i,
		})
	}
	shard.rowRefs = rowRefs
	shard.stateSizeBudget -= (len(rowRefs) - rowRefsLen) * int(unsafe.Sizeof(rowRefs[0]))

	// Append br to shard.blocks.
	shard.blocks = append(shard.blocks, sortBlock{
		br:           br,
		byColumns:    byColumns,
		otherColumns: otherColumns,
	})
	shard.stateSizeBudget -= br.sizeBytes()
	shard.stateSizeBudget -= int(unsafe.Sizeof(shard.blocks[0]))
}

func (shard *pipeSortProcessorShard) createUint64Values(values []string) []uint64 {
	u64ValuesBuf := shard.u64ValuesBuf
	u64ValuesBufLen := len(u64ValuesBuf)
	for _, v := range values {
		u64, ok := tryParseUint64(v)
		if ok {
			u64ValuesBuf = append(u64ValuesBuf, u64)
			continue
		}
		u32, _ := tryParseIPv4(v)
		u64ValuesBuf = append(u64ValuesBuf, uint64(u32))
		// Do not try parsing timestamp and duration, since they may be negative.
		// This breaks sorting.
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

func (psp *pipeSortProcessorShard) Len() int {
	return len(psp.rowRefs)
}

func (psp *pipeSortProcessorShard) Swap(i, j int) {
	rowRefs := psp.rowRefs
	rowRefs[i], rowRefs[j] = rowRefs[j], rowRefs[i]
}

func (psp *pipeSortProcessorShard) Less(i, j int) bool {
	return sortBlockLess(psp, i, psp, j)
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
	if len(sh) == 0 {
		return nil
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

		if shard.rowRefNext >= len(shard.rowRefs) {
			_ = heap.Pop(&sh)
			shardNext = nil

			select {
			case <-psp.stopCh:
				return nil
			default:
			}

			continue
		}

		if shardNext == nil {
			shardNext = sh[1]
			if len(sh) > 2 && sortBlockLess(sh[2], sh[2].rowRefNext, shardNext, shardNext.rowRefNext) {
				shardNext = sh[2]
			}
		}

		if sortBlockLess(shardNext, shardNext.rowRefNext, shard, shard.rowRefNext) {
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
		for shard.rowRefNext < len(shard.rowRefs) {
			wctx.writeRow(shard, shard.rowRefNext)
			shard.rowRefNext++
		}
	}
	wctx.flush()

	return nil
}

type pipeSortWriteContext struct {
	psp *pipeSortProcessor
	rcs []resultColumn
	br  blockResult

	valuesLen int
}

func (wctx *pipeSortWriteContext) writeRow(shard *pipeSortProcessorShard, rowIdx int) {
	rr := shard.rowRefs[rowIdx]
	b := &shard.blocks[rr.blockIdx]

	byFields := shard.ps.byFields
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == len(byFields)+len(b.otherColumns)
	if areEqualColumns {
		for i, c := range b.otherColumns {
			if rcs[len(byFields)+i].name != c.name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to bbBase and construct new columns
		wctx.flush()

		rcs = wctx.rcs[:0]
		for _, bf := range byFields {
			rcs = append(rcs, resultColumn{
				name: bf.name,
			})
		}
		for _, c := range b.otherColumns {
			rcs = append(rcs, resultColumn{
				name: c.name,
			})
		}
		wctx.rcs = rcs
	}

	br := b.br
	byColumns := b.byColumns
	for i := range byColumns {
		v := byColumns[i].c.getValueAtRow(br, rr.rowIdx)
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}

	otherColumns := b.otherColumns
	for i := range otherColumns {
		v := otherColumns[i].getValueAtRow(br, rr.rowIdx)
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
		rcs[i].resetKeepName()
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

func sortBlockLess(shardA *pipeSortProcessorShard, rowIdxA int, shardB *pipeSortProcessorShard, rowIdxB int) bool {
	byFields := shardA.ps.byFields

	rrA := shardA.rowRefs[rowIdxA]
	rrB := shardB.rowRefs[rowIdxB]
	bA := &shardA.blocks[rrA.blockIdx]
	bB := &shardB.blocks[rrB.blockIdx]
	for idx := range bA.byColumns {
		cA := &bA.byColumns[idx]
		cB := &bB.byColumns[idx]
		bf := byFields[idx]

		if cA.c.isConst && cB.c.isConst {
			// Fast path - compare const values
			return cA.c.encodedValues[0] < cB.c.encodedValues[0]
		}

		if cA.c.isTime && cB.c.isTime {
			// Fast path - sort by _time
			tA := bA.br.timestamps[rrA.rowIdx]
			tB := bB.br.timestamps[rrB.rowIdx]
			if tA == tB {
				continue
			}
			if bf.isDesc {
				return tB < tA
			}
			return tA < tB
		}
		if cA.c.isTime {
			// treat timestamps as smaller than other values
			return true
		}
		if cB.c.isTime {
			// treat timestamps as smaller than other values
			return false
		}

		// Try sorting by uint64 values at first
		uA := cA.getU64ValueAtRow(rrA.rowIdx)
		uB := cB.getU64ValueAtRow(rrB.rowIdx)
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
		fA := cA.getF64ValueAtRow(rrA.rowIdx)
		fB := cB.getF64ValueAtRow(rrB.rowIdx)
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
		sA := cA.c.getValueAtRow(bA.br, rrA.rowIdx)
		sB := cB.c.getValueAtRow(bB.br, rrB.rowIdx)
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
