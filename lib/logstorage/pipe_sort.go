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

	"github.com/valyala/quicktemplate"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

// pipeSort processes '| sort ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#sort-pipe
type pipeSort struct {
	// byFields contains field names for sorting from 'by(...)' clause.
	byFields []*bySortField

	// whether to apply descending order
	isDesc bool

	// how many results to skip
	offset uint64

	// how many results to return
	//
	// if zero, then all the results are returned
	limit uint64

	// The name of the field to store the row rank.
	rankName string
}

func (ps *pipeSort) String() string {
	s := "sort"
	if len(ps.byFields) > 0 {
		a := make([]string, len(ps.byFields))
		for i, bf := range ps.byFields {
			a[i] = bf.String()
		}
		s += " by (" + strings.Join(a, ", ") + ")"
	}
	if ps.isDesc {
		s += " desc"
	}
	if ps.offset > 0 {
		s += fmt.Sprintf(" offset %d", ps.offset)
	}
	if ps.limit > 0 {
		s += fmt.Sprintf(" limit %d", ps.limit)
	}
	if ps.rankName != "" {
		s += " rank as " + quoteTokenIfNeeded(ps.rankName)
	}
	return s
}

func (ps *pipeSort) canLiveTail() bool {
	return false
}

func (ps *pipeSort) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		return
	}

	if ps.rankName != "" {
		neededFields.remove(ps.rankName)
		if neededFields.contains("*") {
			unneededFields.add(ps.rankName)
		}
	}

	if len(ps.byFields) == 0 {
		neededFields.add("*")
		unneededFields.reset()
	} else {
		for _, bf := range ps.byFields {
			neededFields.add(bf.name)
			unneededFields.remove(bf.name)
		}
	}
}

func (ps *pipeSort) optimize() {
	// nothing to do
}

func (ps *pipeSort) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeSort) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return ps, nil
}

func (ps *pipeSort) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	if ps.limit > 0 {
		return newPipeTopkProcessor(ps, workersCount, stopCh, cancel, ppNext)
	}
	return newPipeSortProcessor(ps, workersCount, stopCh, cancel, ppNext)
}

func newPipeSortProcessor(ps *pipeSort, workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeSortProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeSortProcessorShard{
			pipeSortProcessorShardNopad: pipeSortProcessorShardNopad{
				ps: ps,
			},
		}
	}

	psp := &pipeSortProcessor{
		ps:     ps,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

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
	ppNext pipeProcessor

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
	// ps points to the parent pipeSort.
	ps *pipeSort

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

	// columnValues is used as temporary buffer at pipeSortProcessorShard.writeBlock
	columnValues [][]string
}

// sortBlock represents a block of logs for sorting.
type sortBlock struct {
	// br is a result block to sort
	br *blockResult

	// byColumns refers block data for 'by(...)' columns
	byColumns []sortBlockByColumn

	// otherColumns refers block data for other than 'by(...)' columns
	otherColumns []*blockResultColumn
}

// sortBlockByColumn represents data for a single column from 'sort by(...)' clause.
type sortBlockByColumn struct {
	// c contains column data
	c *blockResultColumn

	// i64Values contains int64 numbers parsed from values
	i64Values []int64

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

func (c *sortBlockByColumn) getI64ValueAtRow(rowIdx int) int64 {
	if c.c.isConst {
		return c.i64Values[0]
	}
	return c.i64Values[rowIdx]
}

func (c *sortBlockByColumn) getF64ValueAtRow(rowIdx int) float64 {
	if c.c.isConst {
		return c.f64Values[0]
	}
	return c.f64Values[rowIdx]
}

// writeBlock writes br to shard.
func (shard *pipeSortProcessorShard) writeBlock(br *blockResult) {
	// clone br, so it could be owned by shard
	br = br.clone()
	cs := br.getColumns()

	byFields := shard.ps.byFields
	if len(byFields) == 0 {
		// Sort by all the columns

		columnValues := shard.columnValues[:0]
		for _, c := range cs {
			values := c.getValues(br)
			columnValues = append(columnValues, values)
		}
		shard.columnValues = columnValues

		// Generate byColumns
		valuesEncoded := make([]string, br.rowsLen)
		shard.stateSizeBudget -= len(valuesEncoded) * int(unsafe.Sizeof(valuesEncoded[0]))

		bb := bbPool.Get()
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			// Marshal all the columns per each row into a single string
			// and sort rows by the resulting string.
			bb.B = bb.B[:0]
			for i, values := range columnValues {
				v := values[rowIdx]
				bb.B = marshalJSONKeyValue(bb.B, cs[i].name, v)
				bb.B = append(bb.B, ',')
			}
			if rowIdx > 0 && valuesEncoded[rowIdx-1] == string(bb.B) {
				valuesEncoded[rowIdx] = valuesEncoded[rowIdx-1]
			} else {
				valuesEncoded[rowIdx] = string(bb.B)
				shard.stateSizeBudget -= len(bb.B)
			}
		}
		bbPool.Put(bb)

		i64Values := make([]int64, br.rowsLen)
		f64Values := make([]float64, br.rowsLen)
		for i := range f64Values {
			f64Values[i] = nan
		}
		byColumns := []sortBlockByColumn{
			{
				c: &blockResultColumn{
					valueType:     valueTypeString,
					valuesEncoded: valuesEncoded,
				},
				i64Values: i64Values,
				f64Values: f64Values,
			},
		}
		shard.stateSizeBudget -= int(unsafe.Sizeof(byColumns[0]) + unsafe.Sizeof(*byColumns[0].c))

		// Append br to shard.blocks.
		shard.blocks = append(shard.blocks, sortBlock{
			br:           br,
			byColumns:    byColumns,
			otherColumns: cs,
		})
	} else {
		// Collect values for columns from byFields.
		byColumns := make([]sortBlockByColumn, len(byFields))
		for i, bf := range byFields {
			c := br.getColumnByName(bf.name)
			bc := &byColumns[i]
			bc.c = c

			if c.isTime {
				// Do not initialize bc.i64Values and bc.f64Values, since they aren't used.
				// This saves some memory.
				continue
			}
			if c.isConst {
				bc.i64Values = shard.createInt64Values(c.valuesEncoded)
				bc.f64Values = shard.createFloat64Values(c.valuesEncoded)
				continue
			}

			// pre-populate values in order to track better br memory usage
			values := c.getValues(br)
			bc.i64Values = shard.createInt64Values(values)
			bc.f64Values = shard.createFloat64Values(values)
		}
		shard.stateSizeBudget -= len(byColumns) * int(unsafe.Sizeof(byColumns[0]))

		// Collect values for other columns.
		otherColumns := make([]*blockResultColumn, 0, len(cs))
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

		// Append br to shard.blocks.
		shard.blocks = append(shard.blocks, sortBlock{
			br:           br,
			byColumns:    byColumns,
			otherColumns: otherColumns,
		})
	}

	shard.stateSizeBudget -= br.sizeBytes()
	shard.stateSizeBudget -= int(unsafe.Sizeof(shard.blocks[0]))

	// Add row references to rowRefs.
	blockIdx := len(shard.blocks) - 1
	rowRefs := shard.rowRefs
	rowRefsLen := len(rowRefs)
	for i := 0; i < br.rowsLen; i++ {
		rowRefs = append(rowRefs, sortRowRef{
			blockIdx: blockIdx,
			rowIdx:   i,
		})
	}
	shard.rowRefs = rowRefs
	shard.stateSizeBudget -= (len(rowRefs) - rowRefsLen) * int(unsafe.Sizeof(rowRefs[0]))
}

func (shard *pipeSortProcessorShard) createInt64Values(values []string) []int64 {
	a := make([]int64, len(values))
	for i, v := range values {
		i64, ok := tryParseInt64(v)
		if ok {
			a[i] = i64
			continue
		}
		u32, _ := tryParseIPv4(v)
		a[i] = int64(u32)
		// Do not try parsing timestamp and duration, since they may be negative.
		// This breaks sorting.
	}

	shard.stateSizeBudget -= len(a) * int(unsafe.Sizeof(a[0]))

	return a
}

func (shard *pipeSortProcessorShard) createFloat64Values(values []string) []float64 {
	a := make([]float64, len(values))
	for i, v := range values {
		f, ok := tryParseFloat64(v)
		if !ok {
			f = nan
		}
		a[i] = f
	}

	shard.stateSizeBudget -= len(a) * int(unsafe.Sizeof(a[0]))

	return a
}

func (shard *pipeSortProcessorShard) Len() int {
	return len(shard.rowRefs)
}

func (shard *pipeSortProcessorShard) Swap(i, j int) {
	rowRefs := shard.rowRefs
	rowRefs[i], rowRefs[j] = rowRefs[j], rowRefs[i]
}

func (shard *pipeSortProcessorShard) Less(i, j int) bool {
	return sortBlockLess(shard, i, shard, j)
}

func (psp *pipeSortProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
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

	if needStop(psp.stopCh) {
		return nil
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

	if needStop(psp.stopCh) {
		return nil
	}

	// Merge sorted results across shards
	sh := pipeSortProcessorShardsHeap(make([]*pipeSortProcessorShard, 0, len(shards)))
	for i := range shards {
		shard := &shards[i]
		if len(shard.rowRefs) > 0 {
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
	shardNextIdx := 0

	for len(sh) > 1 {
		shard := sh[0]
		wctx.writeNextRow(shard)

		if shard.rowRefNext >= len(shard.rowRefs) {
			_ = heap.Pop(&sh)
			shardNextIdx = 0

			if needStop(psp.stopCh) {
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

			if needStop(psp.stopCh) {
				return nil
			}
		}
	}
	if len(sh) == 1 {
		shard := sh[0]
		for shard.rowRefNext < len(shard.rowRefs) {
			wctx.writeNextRow(shard)
		}
	}
	wctx.flush()

	return nil
}

type pipeSortWriteContext struct {
	psp *pipeSortProcessor
	rcs []resultColumn
	br  blockResult

	// buf is a temporary buffer for non-flushed block.
	buf []byte

	// rowsWritten is the total number of rows passed to writeNextRow.
	rowsWritten uint64

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the length of all the values in the current block
	valuesLen int
}

func (wctx *pipeSortWriteContext) writeNextRow(shard *pipeSortProcessorShard) {
	ps := shard.ps
	rankName := ps.rankName
	rankFields := 0
	if rankName != "" {
		rankFields = 1
	}

	rowIdx := shard.rowRefNext
	shard.rowRefNext++

	wctx.rowsWritten++
	if wctx.rowsWritten <= ps.offset {
		return
	}

	rr := shard.rowRefs[rowIdx]
	b := &shard.blocks[rr.blockIdx]

	byFields := ps.byFields
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == rankFields+len(byFields)+len(b.otherColumns)
	if areEqualColumns {
		for i, c := range b.otherColumns {
			if rcs[rankFields+len(byFields)+i].name != c.name {
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
		for _, c := range b.otherColumns {
			rcs = appendResultColumnWithName(rcs, c.name)
		}
		wctx.rcs = rcs
	}

	if rankName != "" {
		bufLen := len(wctx.buf)
		wctx.buf = marshalUint64String(wctx.buf, wctx.rowsWritten)
		v := bytesutil.ToUnsafeString(wctx.buf[bufLen:])
		rcs[0].addValue(v)
	}

	br := b.br
	byColumns := b.byColumns
	for i := range byFields {
		v := byColumns[i].c.getValueAtRow(br, rr.rowIdx)
		rcs[rankFields+i].addValue(v)
		wctx.valuesLen += len(v)
	}

	for i, c := range b.otherColumns {
		v := c.getValueAtRow(br, rr.rowIdx)
		rcs[rankFields+len(byFields)+i].addValue(v)
		wctx.valuesLen += len(v)
	}

	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeSortWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.psp.ppNext.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
	wctx.buf = wctx.buf[:0]
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
		isDesc := len(byFields) > 0 && byFields[idx].isDesc
		if shardA.ps.isDesc {
			isDesc = !isDesc
		}

		if cA.c.isConst && cB.c.isConst {
			// Fast path - compare const values
			ccA := cA.c.valuesEncoded[0]
			ccB := cB.c.valuesEncoded[0]
			if ccA == ccB {
				continue
			}
			if isDesc {
				return ccB < ccA
			}
			return ccA < ccB
		}

		if cA.c.isTime && cB.c.isTime {
			// Fast path - sort by _time
			timestampsA := bA.br.getTimestamps()
			timestampsB := bB.br.getTimestamps()
			tA := timestampsA[rrA.rowIdx]
			tB := timestampsB[rrB.rowIdx]
			if tA == tB {
				continue
			}
			if isDesc {
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

		// Try sorting by int64 values at first
		uA := cA.getI64ValueAtRow(rrA.rowIdx)
		uB := cB.getI64ValueAtRow(rrB.rowIdx)
		if uA != 0 && uB != 0 {
			if uA == uB {
				continue
			}
			if isDesc {
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
			if isDesc {
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
		if isDesc {
			return stringsutil.LessNatural(sB, sA)
		}
		return stringsutil.LessNatural(sA, sB)
	}
	return false
}

func parsePipeSort(lex *lexer) (*pipeSort, error) {
	if !lex.isKeyword("sort") && !lex.isKeyword("order") {
		return nil, fmt.Errorf("expecting 'sort' or 'order'; got %q", lex.token)
	}
	lex.nextToken()

	var ps pipeSort
	if lex.isKeyword("by", "(") {
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		bfs, err := parseBySortFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by' clause: %w", err)
		}
		ps.byFields = bfs
	}

	switch {
	case lex.isKeyword("desc"):
		lex.nextToken()
		ps.isDesc = true
	case lex.isKeyword("asc"):
		lex.nextToken()
	}

	for {
		switch {
		case lex.isKeyword("offset"):
			lex.nextToken()
			s := lex.token
			n, ok := tryParseUint64(s)
			lex.nextToken()
			if !ok {
				return nil, fmt.Errorf("cannot parse 'offset %s'", s)
			}
			if ps.offset > 0 {
				return nil, fmt.Errorf("duplicate 'offset'; the previous one is %d; the new one is %s", ps.offset, s)
			}
			ps.offset = n
		case lex.isKeyword("limit"):
			lex.nextToken()
			s := lex.token
			n, ok := tryParseUint64(s)
			lex.nextToken()
			if !ok {
				return nil, fmt.Errorf("cannot parse 'limit %s'", s)
			}
			if ps.limit > 0 {
				return nil, fmt.Errorf("duplicate 'limit'; the previous one is %d; the new one is %s", ps.limit, s)
			}
			ps.limit = n
		case lex.isKeyword("rank"):
			lex.nextToken()
			if lex.isKeyword("as") {
				lex.nextToken()
			}
			rankName, err := getCompoundToken(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot read rank field name: %s", err)
			}
			ps.rankName = rankName
		default:
			return &ps, nil
		}
	}
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
			return bfs, nil
		}
		fieldName, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name: %w", err)
		}
		bf := &bySortField{
			name: fieldName,
		}
		switch {
		case lex.isKeyword("desc"):
			lex.nextToken()
			bf.isDesc = true
		case lex.isKeyword("asc"):
			lex.nextToken()
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

func tryParseInt64(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}

	isMinus := s[0] == '-'
	if isMinus {
		s = s[1:]
	}
	u64, ok := tryParseUint64(s)
	if !ok {
		return 0, false
	}
	if !isMinus {
		if u64 > math.MaxInt64 {
			return 0, false
		}
		return int64(u64), true
	}
	if u64 > -math.MinInt64 {
		return 0, false
	}
	return -int64(u64), true
}

func marshalJSONKeyValue(dst []byte, k, v string) []byte {
	dst = quicktemplate.AppendJSONString(dst, k, true)
	dst = append(dst, ':')
	dst = quicktemplate.AppendJSONString(dst, v, true)
	return dst
}
