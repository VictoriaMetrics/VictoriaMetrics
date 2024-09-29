package logstorage

import (
	"container/heap"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// pipeStreamContext processes '| stream_context ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#stream_context-pipe
type pipeStreamContext struct {
	// linesBefore is the number of lines to return before the matching line
	linesBefore int

	// linesAfter is the number of lines to return after the matching line
	linesAfter int
}

func (pc *pipeStreamContext) String() string {
	s := "stream_context"
	if pc.linesBefore > 0 {
		s += fmt.Sprintf(" before %d", pc.linesBefore)
	}
	if pc.linesAfter > 0 {
		s += fmt.Sprintf(" after %d", pc.linesAfter)
	}
	if pc.linesBefore <= 0 && pc.linesAfter <= 0 {
		s += " after 0"
	}
	return s
}

func (pc *pipeStreamContext) canLiveTail() bool {
	return false
}

var neededFieldsForStreamContext = []string{
	"_time",
	"_stream_id",
}

func (pc *pipeStreamContext) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.addFields(neededFieldsForStreamContext)
	unneededFields.removeFields(neededFieldsForStreamContext)
}

func (pc *pipeStreamContext) optimize() {
	// nothing to do
}

func (pc *pipeStreamContext) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeStreamContext) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pc, nil
}

func (pc *pipeStreamContext) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeStreamContextProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeStreamContextProcessorShard{
			pipeStreamContextProcessorShardNopad: pipeStreamContextProcessorShardNopad{
				pc: pc,
			},
		}
	}

	pcp := &pipeStreamContextProcessor{
		pc:     pc,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	pcp.stateSizeBudget.Store(maxStateSize)

	return pcp
}

type pipeStreamContextProcessor struct {
	pc     *pipeStreamContext
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	s                   *Storage
	neededColumnNames   []string
	unneededColumnNames []string

	shards []pipeStreamContextProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

func (pcp *pipeStreamContextProcessor) init(s *Storage, neededColumnNames, unneededColumnNames []string) {
	pcp.s = s
	pcp.neededColumnNames = neededColumnNames
	pcp.unneededColumnNames = unneededColumnNames
}

func (pcp *pipeStreamContextProcessor) getStreamRowss(streamID string, neededRows []streamContextRow, stateSizeBudget int) ([][]*streamContextRow, error) {
	tenantID, ok := getTenantIDFromStreamIDString(streamID)
	if !ok {
		logger.Panicf("BUG: cannot obtain tenantID from streamID %q", streamID)
	}

	// construct the query for selecting all the rows for the given streamID
	qStr := "_stream_id:" + streamID
	if slices.Contains(pcp.neededColumnNames, "*") {
		if len(pcp.unneededColumnNames) > 0 {
			qStr += " | delete " + fieldNamesString(pcp.unneededColumnNames)
		}
	} else {
		if len(pcp.neededColumnNames) > 0 {
			qStr += " | fields " + fieldNamesString(pcp.neededColumnNames)
		}
	}
	q, err := ParseQuery(qStr)
	if err != nil {
		logger.Panicf("BUG: cannot parse query [%s]: %s", qStr, err)
	}

	// mu protects contextRows and stateSize inside writeBlock callback.
	var mu sync.Mutex

	contextRows := make([]streamContextRows, len(neededRows))
	for i := range neededRows {
		contextRows[i] = streamContextRows{
			neededTimestamp: neededRows[i].timestamp,
			linesBefore:     pcp.pc.linesBefore,
			linesAfter:      pcp.pc.linesAfter,
		}
	}
	sort.Slice(contextRows, func(i, j int) bool {
		return contextRows[i].neededTimestamp < contextRows[j].neededTimestamp
	})

	stateSize := 0

	ctxWithCancel, cancel := contextutil.NewStopChanContext(pcp.stopCh)
	defer cancel()

	writeBlock := func(_ uint, br *blockResult) {
		mu.Lock()
		defer mu.Unlock()

		if stateSize > stateSizeBudget {
			cancel()
			return
		}

		for i := range contextRows {
			if needStop(pcp.stopCh) {
				break
			}

			if !contextRows[i].canUpdate(br) {
				// Fast path - skip reading block timestamps for the given ctx.
				continue
			}

			timestamps := br.getTimestamps()
			for j, timestamp := range timestamps {
				if i > 0 && timestamp <= contextRows[i-1].neededTimestamp {
					continue
				}
				if i+1 < len(contextRows) && timestamp >= contextRows[i+1].neededTimestamp {
					continue
				}
				stateSize += contextRows[i].update(br, j, timestamp)
			}
		}
	}

	if err := pcp.s.runQuery(ctxWithCancel, []TenantID{tenantID}, q, writeBlock); err != nil {
		return nil, err
	}
	if stateSize > stateSizeBudget {
		return nil, fmt.Errorf("more than %dMB of memory is needed for fetching the surrounding logs for %d matching logs", stateSizeBudget/(1<<20), len(neededRows))
	}

	// return sorted results from contextRows
	rowss := make([][]*streamContextRow, len(contextRows))
	for i, ctx := range contextRows {
		rowss[i] = ctx.getSortedRows()
	}
	rowss = deduplicateStreamRowss(rowss)
	return rowss, nil
}

func deduplicateStreamRowss(streamRowss [][]*streamContextRow) [][]*streamContextRow {
	var lastSeenRow *streamContextRow
	for _, streamRows := range streamRowss {
		if len(streamRows) > 0 {
			lastSeenRow = streamRows[len(streamRows)-1]
			break
		}
	}
	if lastSeenRow == nil {
		return nil
	}

	resultRowss := streamRowss[:1]
	for _, streamRows := range streamRowss[1:] {
		i := 0
		for i < len(streamRows) && !lastSeenRow.less(streamRows[i]) {
			i++
		}
		streamRows = streamRows[i:]
		if len(streamRows) == 0 {
			continue
		}
		resultRowss = append(resultRowss, streamRows)
		lastSeenRow = streamRows[len(streamRows)-1]
	}
	return resultRowss
}

type streamContextRows struct {
	neededTimestamp int64
	linesBefore     int
	linesAfter      int

	rowsBefore  streamContextRowsHeapMin
	rowsAfter   streamContextRowsHeapMax
	rowsMatched []*streamContextRow
}

func (ctx *streamContextRows) getSortedRows() []*streamContextRow {
	var rows []*streamContextRow
	rows = append(rows, ctx.rowsBefore...)
	rows = append(rows, ctx.rowsMatched...)
	rows = append(rows, ctx.rowsAfter...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].less(rows[j])
	})
	return rows
}

func (ctx *streamContextRows) canUpdate(br *blockResult) bool {
	if ctx.linesBefore > 0 {
		if len(ctx.rowsBefore) < ctx.linesBefore {
			return true
		}
		minTimestamp := ctx.rowsBefore[0].timestamp - 1
		maxTimestamp := ctx.neededTimestamp
		if br.intersectsTimeRange(minTimestamp, maxTimestamp) {
			return true
		}
	}

	if ctx.linesAfter > 0 {
		if len(ctx.rowsAfter) < ctx.linesAfter {
			return true
		}
		minTimestamp := ctx.neededTimestamp
		maxTimestamp := ctx.rowsAfter[0].timestamp + 1
		if br.intersectsTimeRange(minTimestamp, maxTimestamp) {
			return true
		}
	}

	if ctx.linesBefore <= 0 && ctx.linesAfter <= 0 {
		if len(ctx.rowsMatched) == 0 {
			return true
		}
		timestamp := ctx.rowsMatched[0].timestamp
		if br.intersectsTimeRange(timestamp-1, timestamp+1) {
			return true
		}
	}

	return false
}

func (ctx *streamContextRows) update(br *blockResult, rowIdx int, rowTimestamp int64) int {
	if rowTimestamp < ctx.neededTimestamp {
		if ctx.linesBefore <= 0 {
			return 0
		}
		if len(ctx.rowsBefore) < ctx.linesBefore {
			r := ctx.copyRowAtIdx(br, rowIdx, rowTimestamp)
			heap.Push(&ctx.rowsBefore, r)
			return r.sizeBytes()
		}
		if rowTimestamp <= ctx.rowsBefore[0].timestamp {
			return 0
		}
		r := ctx.copyRowAtIdx(br, rowIdx, rowTimestamp)
		stateSizeChange := r.sizeBytes() - ctx.rowsBefore[0].sizeBytes()
		ctx.rowsBefore[0] = r
		heap.Fix(&ctx.rowsBefore, 0)
		return stateSizeChange
	}

	if rowTimestamp > ctx.neededTimestamp {
		if ctx.linesAfter <= 0 {
			return 0
		}
		if len(ctx.rowsAfter) < ctx.linesAfter {
			r := ctx.copyRowAtIdx(br, rowIdx, rowTimestamp)
			heap.Push(&ctx.rowsAfter, r)
			return r.sizeBytes()
		}
		if rowTimestamp >= ctx.rowsAfter[0].timestamp {
			return 0
		}
		r := ctx.copyRowAtIdx(br, rowIdx, rowTimestamp)
		stateSizeChange := r.sizeBytes() - ctx.rowsAfter[0].sizeBytes()
		ctx.rowsAfter[0] = r
		heap.Fix(&ctx.rowsAfter, 0)
		return stateSizeChange
	}

	// rowTimestamp == ctx.neededTimestamp
	r := ctx.copyRowAtIdx(br, rowIdx, rowTimestamp)
	ctx.rowsMatched = append(ctx.rowsMatched, r)
	return r.sizeBytes()
}

func (ctx *streamContextRows) copyRowAtIdx(br *blockResult, rowIdx int, rowTimestamp int64) *streamContextRow {
	cs := br.getColumns()

	fields := make([]Field, len(cs))
	for i, c := range cs {
		v := c.getValueAtRow(br, rowIdx)
		fields[i] = Field{
			Name:  strings.Clone(c.name),
			Value: strings.Clone(v),
		}
	}
	return &streamContextRow{
		timestamp: rowTimestamp,
		fields:    fields,
	}
}

func getTenantIDFromStreamIDString(s string) (TenantID, bool) {
	var sid streamID
	if !sid.tryUnmarshalFromString(s) {
		return TenantID{}, false
	}
	return sid.tenantID, true
}

type pipeStreamContextProcessorShard struct {
	pipeStreamContextProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeStreamContextProcessorShardNopad{})%128]byte
}

type streamContextRow struct {
	timestamp int64
	fields    []Field
}

func (r *streamContextRow) sizeBytes() int {
	n := 0
	fields := r.fields
	for _, f := range fields {
		n += len(f.Name) + len(f.Value) + int(unsafe.Sizeof(f))
	}
	n += int(unsafe.Sizeof(*r) + unsafe.Sizeof(r))
	return n
}

func (r *streamContextRow) less(other *streamContextRow) bool {
	// compare timestamps at first
	if r.timestamp != other.timestamp {
		return r.timestamp < other.timestamp
	}

	// compare fields then
	i := 0
	aFields := r.fields
	bFields := other.fields
	for i < len(aFields) && i < len(bFields) {
		af := &aFields[i]
		bf := &bFields[i]
		if af.Name != bf.Name {
			return af.Name < bf.Name
		}
		if af.Value != bf.Value {
			return af.Value < bf.Value
		}
		i++
	}
	if len(aFields) != len(bFields) {
		return len(aFields) < len(bFields)
	}

	return false
}

type pipeStreamContextProcessorShardNopad struct {
	// pc points to the parent pipeStreamContext.
	pc *pipeStreamContext

	// m holds per-stream matching rows
	m map[string][]streamContextRow

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeStreamContextProcessor.
	stateSizeBudget int
}

// writeBlock writes br to shard.
func (shard *pipeStreamContextProcessorShard) writeBlock(br *blockResult) {
	m := shard.getM()

	cs := br.getColumns()
	cStreamID := br.getColumnByName("_stream_id")
	stateSize := 0
	timestamps := br.getTimestamps()
	for i, timestamp := range timestamps {
		fields := make([]Field, len(cs))
		stateSize += int(unsafe.Sizeof(fields[0])) * len(fields)

		for j, c := range cs {
			v := c.getValueAtRow(br, i)
			fields[j] = Field{
				Name:  strings.Clone(c.name),
				Value: strings.Clone(v),
			}
			stateSize += len(c.name) + len(v)
		}

		row := streamContextRow{
			timestamp: timestamp,
			fields:    fields,
		}
		stateSize += int(unsafe.Sizeof(row))

		streamID := cStreamID.getValueAtRow(br, i)
		rows, ok := m[streamID]
		if !ok {
			stateSize += len(streamID)
		}
		rows = append(rows, row)
		streamID = strings.Clone(streamID)
		m[streamID] = rows
	}

	shard.stateSizeBudget -= stateSize
}

func (shard *pipeStreamContextProcessorShard) getM() map[string][]streamContextRow {
	if shard.m == nil {
		shard.m = make(map[string][]streamContextRow)
	}
	return shard.m
}

func (pcp *pipeStreamContextProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &pcp.shards[workerID]

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := pcp.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				pcp.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	shard.writeBlock(br)
}

func (pcp *pipeStreamContextProcessor) flush() error {
	n := pcp.stateSizeBudget.Load()
	if n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", pcp.pc.String(), pcp.maxStateSize/(1<<20))
	}
	if n > math.MaxInt {
		logger.Panicf("BUG: stateSizeBudget shouldn't exceed math.MaxInt=%v; got %d", math.MaxInt, n)
	}
	stateSizeBudget := int(n)

	// merge state across shards
	shards := pcp.shards
	m := shards[0].getM()
	shards = shards[1:]
	for i := range shards {
		if needStop(pcp.stopCh) {
			return nil
		}

		for streamID, rowsSrc := range shards[i].getM() {
			rows, ok := m[streamID]
			if !ok {
				m[streamID] = rowsSrc
			} else {
				m[streamID] = append(rows, rowsSrc...)
			}
		}
	}

	// write result
	wctx := &pipeStreamContextWriteContext{
		pcp: pcp,
	}

	// write output contexts in the ascending order of rows
	streamIDs := getStreamIDsSortedByMinRowTimestamp(m)
	for _, streamID := range streamIDs {
		rows := m[streamID]
		streamRowss, err := pcp.getStreamRowss(streamID, rows, stateSizeBudget)
		if err != nil {
			return err
		}
		if needStop(pcp.stopCh) {
			return nil
		}

		// Write streamRows to the output.
		for _, streamRows := range streamRowss {
			for _, streamRow := range streamRows {
				wctx.writeRow(streamRow.fields)
			}
			if len(m) > 1 || len(streamRowss) > 1 {
				lastRow := streamRows[len(streamRows)-1]
				fields := newDelimiterRowFields(lastRow, streamID)
				wctx.writeRow(fields)
			}
		}
	}

	wctx.flush()

	return nil
}

func getStreamIDsSortedByMinRowTimestamp(m map[string][]streamContextRow) []string {
	type streamTimestamp struct {
		streamID  string
		timestamp int64
	}
	streamTimestamps := make([]streamTimestamp, 0, len(m))
	for streamID, rows := range m {
		minTimestamp := rows[0].timestamp
		for _, r := range rows[1:] {
			if r.timestamp < minTimestamp {
				minTimestamp = r.timestamp
			}
		}
		streamTimestamps = append(streamTimestamps, streamTimestamp{
			streamID:  streamID,
			timestamp: minTimestamp,
		})
	}
	sort.Slice(streamTimestamps, func(i, j int) bool {
		return streamTimestamps[i].timestamp < streamTimestamps[j].timestamp
	})
	streamIDs := make([]string, len(streamTimestamps))
	for i := range streamIDs {
		streamIDs[i] = streamTimestamps[i].streamID
	}
	return streamIDs
}

func newDelimiterRowFields(r *streamContextRow, streamID string) []Field {
	return []Field{
		{
			Name:  "_time",
			Value: string(marshalTimestampRFC3339NanoString(nil, r.timestamp+1)),
		},
		{
			Name:  "_stream_id",
			Value: streamID,
		},
		{
			Name:  "_stream",
			Value: getFieldValue(r.fields, "_stream"),
		},
		{
			Name:  "_msg",
			Value: "---",
		},
	}
}

type pipeStreamContextWriteContext struct {
	pcp *pipeStreamContextProcessor
	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeStreamContextWriteContext) writeRow(rowFields []Field) {
	rcs := wctx.rcs

	areEqualColumns := len(rcs) == len(rowFields)
	if areEqualColumns {
		for i, f := range rowFields {
			if rcs[i].name != f.Name {
				areEqualColumns = false
				break
			}
		}
	}
	if !areEqualColumns {
		// send the current block to ppNext and construct a block with new set of columns
		wctx.flush()

		rcs = wctx.rcs[:0]
		for _, f := range rowFields {
			rcs = appendResultColumnWithName(rcs, f.Name)
		}
		wctx.rcs = rcs
	}

	for i, f := range rowFields {
		v := f.Value
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}

	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeStreamContextWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.pcp.ppNext.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

func parsePipeStreamContext(lex *lexer) (*pipeStreamContext, error) {
	if !lex.isKeyword("stream_context") {
		return nil, fmt.Errorf("expecting 'stream_context'; got %q", lex.token)
	}
	lex.nextToken()

	linesBefore, linesAfter, err := parsePipeStreamContextBeforeAfter(lex)
	if err != nil {
		return nil, err
	}

	pc := &pipeStreamContext{
		linesBefore: linesBefore,
		linesAfter:  linesAfter,
	}
	return pc, nil
}

func parsePipeStreamContextBeforeAfter(lex *lexer) (int, int, error) {
	linesBefore := 0
	linesAfter := 0
	beforeSet := false
	afterSet := false
	for {
		switch {
		case lex.isKeyword("before"):
			lex.nextToken()
			f, s, err := parseNumber(lex)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot parse 'before' value in 'stream_context': %w", err)
			}
			if f < 0 {
				return 0, 0, fmt.Errorf("'before' value cannot be smaller than 0; got %q", s)
			}
			linesBefore = int(f)
			beforeSet = true
		case lex.isKeyword("after"):
			lex.nextToken()
			f, s, err := parseNumber(lex)
			if err != nil {
				return 0, 0, fmt.Errorf("cannot parse 'after' value in 'stream_context': %w", err)
			}
			if f < 0 {
				return 0, 0, fmt.Errorf("'after' value cannot be smaller than 0; got %q", s)
			}
			linesAfter = int(f)
			afterSet = true
		default:
			if !beforeSet && !afterSet {
				return 0, 0, fmt.Errorf("missing 'before N' or 'after N' in 'stream_context'")
			}
			return linesBefore, linesAfter, nil
		}
	}
}

type streamContextRowsHeapMax []*streamContextRow

func (h *streamContextRowsHeapMax) Len() int {
	return len(*h)
}
func (h *streamContextRowsHeapMax) Less(i, j int) bool {
	a := *h
	return a[i].timestamp > a[j].timestamp
}
func (h *streamContextRowsHeapMax) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *streamContextRowsHeapMax) Push(v any) {
	x := v.(*streamContextRow)
	*h = append(*h, x)
}
func (h *streamContextRowsHeapMax) Pop() any {
	a := *h
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*h = a[:len(a)-1]
	return x
}

type streamContextRowsHeapMin streamContextRowsHeapMax

func (h *streamContextRowsHeapMin) Len() int {
	return len(*h)
}
func (h *streamContextRowsHeapMin) Less(i, j int) bool {
	a := *h
	return a[i].timestamp < a[j].timestamp
}
func (h *streamContextRowsHeapMin) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *streamContextRowsHeapMin) Push(v any) {
	x := v.(*streamContextRow)
	*h = append(*h, x)
}
func (h *streamContextRowsHeapMin) Pop() any {
	a := *h
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*h = a[:len(a)-1]
	return x
}
