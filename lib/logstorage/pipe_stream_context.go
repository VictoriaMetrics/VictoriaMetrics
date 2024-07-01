package logstorage

import (
	"context"
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
				pc:              pc,
				stateSizeBudget: stateSizeBudgetChunk,
			},
		}
		maxStateSize -= stateSizeBudgetChunk
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

	shards []pipeStreamContextProcessorShard

	getStreamRows func(streamID string, stateSizeBudget int) ([]streamContextRow, error)

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

func (pcp *pipeStreamContextProcessor) init(ctx context.Context, s *Storage, minTimestamp, maxTimestamp int64) {
	pcp.getStreamRows = func(streamID string, stateSizeBudget int) ([]streamContextRow, error) {
		return getStreamRows(ctx, s, streamID, minTimestamp, maxTimestamp, stateSizeBudget)
	}
}

func getStreamRows(ctx context.Context, s *Storage, streamID string, minTimestamp, maxTimestamp int64, stateSizeBudget int) ([]streamContextRow, error) {
	tenantID, ok := getTenantIDFromStreamIDString(streamID)
	if !ok {
		logger.Panicf("BUG: cannot obtain tenantID from streamID %q", streamID)
	}

	qStr := "_stream_id:" + streamID
	q, err := ParseQuery(qStr)
	if err != nil {
		logger.Panicf("BUG: cannot parse query [%s]: %s", qStr, err)
	}
	q.AddTimeFilter(minTimestamp, maxTimestamp)

	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var rows []streamContextRow
	stateSize := 0
	writeBlock := func(_ uint, br *blockResult) {
		mu.Lock()
		defer mu.Unlock()

		if stateSize > stateSizeBudget {
			cancel()
		}

		cs := br.getColumns()
		for i, timestamp := range br.timestamps {
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
			rows = append(rows, row)
		}
	}

	if err := s.runQuery(ctxWithCancel, []TenantID{tenantID}, q, writeBlock); err != nil {
		return nil, err
	}
	if stateSize > stateSizeBudget {
		return nil, fmt.Errorf("more than %dMB of memory is needed for query [%s]", stateSizeBudget/(1<<20), q)
	}

	return rows, nil
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
	for i, timestamp := range br.timestamps {
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
	if len(br.timestamps) == 0 {
		return
	}
	if pcp.pc.linesBefore <= 0 && pcp.pc.linesAfter <= 0 {
		// Fast path - there is no need to fetch stream context.
		pcp.ppNext.writeBlock(workerID, br)
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
	if pcp.pc.linesBefore <= 0 && pcp.pc.linesAfter <= 0 {
		// Fast path - nothing to do.
		return nil
	}

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

	for streamID, rows := range m {
		streamRows, err := pcp.getStreamRows(streamID, stateSizeBudget)
		if err != nil {
			return fmt.Errorf("cannot read rows for _stream_id=%q: %w", streamID, err)
		}
		if needStop(pcp.stopCh) {
			return nil
		}
		if err := wctx.writeStreamContextRows(streamID, streamRows, rows, pcp.pc.linesBefore, pcp.pc.linesAfter); err != nil {
			return fmt.Errorf("cannot obtain context rows for _stream_id=%q: %w", streamID, err)
		}
	}

	wctx.flush()

	return nil
}

func (wctx *pipeStreamContextWriteContext) writeStreamContextRows(streamID string, streamRows, rows []streamContextRow, linesBefore, linesAfter int) error {
	sortStreamContextRows(streamRows)
	sortStreamContextRows(rows)

	idxNext := 0
	for i := range rows {
		r := &rows[i]
		idx := getStreamContextRowIdx(streamRows, r)
		if idx < 0 {
			// This error may happen when streamRows became out of sync with rows.
			// For example, when some streamRows were deleted after obtaining rows.
			return fmt.Errorf("missing row for timestamp=%d; len(streamRows)=%d, len(rows)=%d; re-execute the query", r.timestamp, len(streamRows), len(rows))
		}

		idxStart := idx - linesBefore
		if idxStart < idxNext {
			idxStart = idxNext
		} else if idxNext > 0 && idxStart > idxNext {
			// Write delimiter row between multiple contexts in the same stream.
			// This simplifies investigation of the returned logs.
			fields := []Field{
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
			wctx.writeRow(fields)
		}
		for idxStart < idx {
			wctx.writeRow(streamRows[idxStart].fields)
			idxStart++
		}

		if idx >= idxNext {
			wctx.writeRow(streamRows[idx].fields)
			idxNext = idx + 1
		}

		idxEnd := idx + 1 + linesAfter
		for idxNext < idxEnd && idxNext < len(streamRows) {
			wctx.writeRow(streamRows[idxNext].fields)
			idxNext++
		}

		if idxNext >= len(streamRows) {
			break
		}
	}

	return nil
}

func getStreamContextRowIdx(rows []streamContextRow, r *streamContextRow) int {
	n := sort.Search(len(rows), func(i int) bool {
		return rows[i].timestamp >= r.timestamp
	})
	if n == len(rows) {
		return -1
	}

	equalFields := func(fields []Field) bool {
		for _, f := range r.fields {
			if f.Value != getFieldValue(fields, f.Name) {
				return false
			}
		}
		return true
	}

	for rows[n].timestamp == r.timestamp && !equalFields(rows[n].fields) {
		n++
		if n >= len(rows) {
			return -1
		}
	}
	if rows[n].timestamp != r.timestamp {
		return -1
	}
	return n
}

func sortStreamContextRows(rows []streamContextRow) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].timestamp < rows[j].timestamp
	})
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
