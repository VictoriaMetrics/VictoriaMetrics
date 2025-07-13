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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeStreamContextDefaultTimeWindow is the default time window to search for surrounding logs in `stream_context` pipe.
const pipeStreamContextDefaultTimeWindow = int64(nsecsPerHour)

// pipeStreamContext processes '| stream_context ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#stream_context-pipe
type pipeStreamContext struct {
	// linesBefore is the number of lines to return before the matching line
	linesBefore int

	// linesAfter is the number of lines to return after the matching line
	linesAfter int

	// timeWindow is the time window in nanoseconds for searching for surrounding logs
	timeWindow int64

	// runQuery and fieldsFilter must be initialized via withRunQuery().
	runQuery     runQueryFunc
	fieldsFilter *prefixfilter.Filter
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
	if pc.timeWindow != pipeStreamContextDefaultTimeWindow {
		s += " time_window " + string(marshalDurationString(nil, pc.timeWindow))
	}
	return s
}

func (pc *pipeStreamContext) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return nil, []pipe{pc}
}

func (pc *pipeStreamContext) canLiveTail() bool {
	return false
}

func (pc *pipeStreamContext) withRunQuery(runQuery runQueryFunc, fieldsFilter *prefixfilter.Filter) pipe {
	pcNew := *pc
	pcNew.runQuery = runQuery
	pcNew.fieldsFilter = fieldsFilter
	return &pcNew
}

func (pc *pipeStreamContext) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter("_time")
	pf.AddAllowFilter("_stream_id")
}

func (pc *pipeStreamContext) hasFilterInWithQuery() bool {
	return false
}

func (pc *pipeStreamContext) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pc, nil
}

func (pc *pipeStreamContext) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pc *pipeStreamContext) newPipeProcessor(_ int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	pcp := &pipeStreamContextProcessor{
		pc:     pc,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		maxStateSize: maxStateSize,
	}
	pcp.shards.Init = func(shard *pipeStreamContextProcessorShard) {
		shard.pc = pc
	}
	pcp.stateSizeBudget.Store(maxStateSize)

	return pcp
}

type pipeStreamContextProcessor struct {
	pc     *pipeStreamContext
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeStreamContextProcessorShard]

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type timeRange struct {
	start int64
	end   int64
}

func (pcp *pipeStreamContextProcessor) getStreamRowss(streamID string, neededRows []streamContextRow, stateSizeBudget int) ([][]*streamContextRow, error) {
	neededTimestamps := make([]int64, len(neededRows))
	stateSizeBudget -= int(unsafe.Sizeof(neededTimestamps[0])) * len(neededTimestamps)
	for i := range neededRows {
		neededTimestamps[i] = neededRows[i].timestamp
	}
	sort.Slice(neededTimestamps, func(i, j int) bool {
		return neededTimestamps[i] < neededTimestamps[j]
	})

	trs, stateSize, err := pcp.getTimeRangesForStreamRowss(streamID, neededTimestamps, stateSizeBudget)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain time ranges for the needed timestamps: %w", err)
	}
	stateSizeBudget -= stateSize

	rowss, err := pcp.getStreamRowssByTimeRanges(streamID, neededTimestamps, trs, stateSizeBudget)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain stream rows for the selected time ranges: %w", err)
	}
	for _, rows := range rowss {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].less(rows[j])
		})
	}

	rowss = deduplicateStreamRowss(rowss)

	return rowss, nil
}

func (pcp *pipeStreamContextProcessor) getTimeRangesForStreamRowss(streamID string, neededTimestamps []int64, stateSizeBudget int) ([]timeRange, int, error) {
	// construct the query for selecting only timestamps across all the logs for the given streamID
	tr := pcp.getTimeRangeForNeededTimestamps(neededTimestamps)
	timeFilter := getTimeFilter(tr.start, tr.end)
	qStr := fmt.Sprintf("_stream_id:%s %s | fields _time", streamID, timeFilter)

	rowss, stateSize, err := pcp.executeQuery(streamID, qStr, neededTimestamps, stateSizeBudget)
	if err != nil {
		return nil, 0, err
	}

	trs := make([]timeRange, len(rowss))
	newStateSize := int(unsafe.Sizeof(trs[0])) * len(rowss)
	if stateSize+newStateSize > stateSizeBudget {
		return nil, 0, fmt.Errorf("more than %dMB of memory is needed for fetching the surrounding logs for %d matching logs", stateSizeBudget/(1<<20), len(neededTimestamps))
	}
	for i, rows := range rowss {
		if len(rows) == 0 {
			// surrounding rows for the given row were included into the previous row.
			trs[i] = timeRange{
				start: math.MinInt64,
				end:   math.MaxInt64,
			}
			continue
		}
		minTimestamp := rows[0].timestamp
		maxTimestamp := minTimestamp
		for _, row := range rows[1:] {
			if row.timestamp < minTimestamp {
				minTimestamp = row.timestamp
			} else if row.timestamp > maxTimestamp {
				maxTimestamp = row.timestamp
			}
		}
		trs[i] = timeRange{
			start: minTimestamp,
			end:   maxTimestamp,
		}
	}
	return trs, newStateSize, nil
}

func (pcp *pipeStreamContextProcessor) getTimeRangeForNeededTimestamps(neededTimestamps []int64) timeRange {
	var tr timeRange
	tr.start = neededTimestamps[0]
	tr.end = neededTimestamps[0]
	for _, ts := range neededTimestamps[1:] {
		if ts < tr.start {
			tr.start = ts
		} else if ts > tr.end {
			tr.end = ts
		}
	}
	if pcp.pc.linesBefore > 0 {
		tr.start -= pcp.pc.timeWindow
	}
	if pcp.pc.linesAfter > 0 {
		tr.end += pcp.pc.timeWindow
	}
	return tr
}

func (pcp *pipeStreamContextProcessor) getStreamRowssByTimeRanges(streamID string, neededTimestamps []int64, trs []timeRange, stateSizeBudget int) ([][]*streamContextRow, error) {
	// construct the query for selecting rows on the given tr for the given streamID
	qStr := "_stream_id:" + streamID
	minTimestamp := int64(math.MaxInt64)
	maxTimestamp := int64(math.MinInt64)
	timeFilters := make([]string, 0, len(trs))
	for _, tr := range trs {
		if tr.start == math.MinInt64 && tr.end == math.MaxInt64 {
			continue
		}
		if tr.start < minTimestamp {
			minTimestamp = tr.start
		}
		if tr.end > maxTimestamp {
			maxTimestamp = tr.end
		}
		timeFilters = append(timeFilters, getTimeFilter(tr.start, tr.end))
	}
	if minTimestamp <= maxTimestamp {
		qStr += " " + getTimeFilter(minTimestamp, maxTimestamp)
	}
	if len(timeFilters) > 1 {
		qStr += " (" + strings.Join(timeFilters, " OR ") + ")"
	}
	qStr += toFieldsFilters(pcp.pc.fieldsFilter)

	rowss, _, err := pcp.executeQuery(streamID, qStr, neededTimestamps, stateSizeBudget)
	if err != nil {
		return nil, err
	}
	return rowss, nil
}

func getTimeFilter(start, end int64) string {
	startStr := marshalTimestampRFC3339NanoString(nil, start)
	endStr := marshalTimestampRFC3339NanoString(nil, end)
	return fmt.Sprintf("_time:[%s, %s]", startStr, endStr)
}

func (pcp *pipeStreamContextProcessor) executeQuery(streamID, qStr string, neededTimestamps []int64, stateSizeBudget int) ([][]*streamContextRow, int, error) {
	q, err := ParseQuery(qStr)
	if err != nil {
		logger.Panicf("BUG: cannot parse query [%s]: %s", qStr, err)
	}

	// mu protects contextRows and stateSize inside writeBlock callback.
	var mu sync.Mutex

	contextRows := make([]streamContextRows, len(neededTimestamps))
	for i := range neededTimestamps {
		contextRows[i] = streamContextRows{
			neededTimestamp: neededTimestamps[i],
			linesBefore:     pcp.pc.linesBefore,
			linesAfter:      pcp.pc.linesAfter,
		}
	}

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
				return
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

	tenantID, ok := getTenantIDFromStreamIDString(streamID)
	if !ok {
		logger.Panicf("BUG: cannot obtain tenantID from streamID %q", streamID)
	}
	if err := pcp.pc.runQuery(ctxWithCancel, []TenantID{tenantID}, q, writeBlock); err != nil {
		return nil, 0, err
	}
	if stateSize > stateSizeBudget {
		return nil, 0, fmt.Errorf("more than %dMB of memory is needed for fetching the surrounding logs for %d matching logs", stateSizeBudget/(1<<20), len(neededTimestamps))
	}

	rowss := make([][]*streamContextRow, len(contextRows))
	for i, ctx := range contextRows {
		rows := ctx.rowsBefore
		rows = append(rows, ctx.rowsMatched...)
		rows = append(rows, ctx.rowsAfter...)
		rowss[i] = rows
	}
	return rowss, stateSize, nil
}

func deduplicateStreamRowss(streamRowss [][]*streamContextRow) [][]*streamContextRow {
	i := 0
	for i < len(streamRowss) {
		if len(streamRowss[i]) > 0 {
			break
		}
		i++
	}
	streamRowss = streamRowss[i:]
	if len(streamRowss) == 0 {
		return nil
	}

	lastSeenRow := streamRowss[0][len(streamRowss[0])-1]
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

func (ctx *streamContextRows) canUpdate(br *blockResult) bool {
	if ctx.linesBefore > 0 {
		if len(ctx.rowsBefore) < ctx.linesBefore {
			return true
		}
		minTimestamp := ctx.rowsBefore[0].timestamp
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
		maxTimestamp := ctx.rowsAfter[0].timestamp
		if br.intersectsTimeRange(minTimestamp, maxTimestamp) {
			return true
		}
	}

	if ctx.linesBefore <= 0 && ctx.linesAfter <= 0 {
		if len(ctx.rowsMatched) == 0 {
			return true
		}
		timestamp := ctx.rowsMatched[0].timestamp
		if br.intersectsTimeRange(timestamp, timestamp) {
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
	if len(cs) == 0 {
		return &streamContextRow{
			timestamp: rowTimestamp,
		}
	}

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
	// pc points to the parent pipeStreamContext.
	pc *pipeStreamContext

	// m holds per-stream matching rows
	m map[string][]streamContextRow

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeStreamContextProcessor.
	stateSizeBudget int
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

// writeBlock writes br to shard.
func (shard *pipeStreamContextProcessorShard) writeBlock(pcp *pipeStreamContextProcessor, br *blockResult) {
	m := shard.getM()
	if len(m) > pipeStreamContextMaxStreams {
		// Ignore the rest of blocks because the number of streams is too big for showing stream context
		pcp.cancel()
		return
	}

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
		if len(rows) > pipeStreamContextMaxRowsPerStream {
			// Ignore the rest of blocks because the number of rows is too big for showing stream context
			pcp.cancel()
			return
		}
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

	shard := pcp.shards.Get(workerID)

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

	shard.writeBlock(pcp, br)
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
	shards := pcp.shards.All()
	if len(shards) == 0 {
		return nil
	}

	m := shards[0].getM()
	shards = shards[1:]
	for _, shard := range shards {
		if needStop(pcp.stopCh) {
			return nil
		}

		for streamID, rowsSrc := range shard.getM() {
			rows, ok := m[streamID]
			if !ok {
				m[streamID] = rowsSrc
			} else {
				m[streamID] = append(rows, rowsSrc...)
			}
		}
	}

	if len(m) > pipeStreamContextMaxStreams {
		return fmt.Errorf("logs from too many streams passed to 'stream_context': %d; the maximum supported number of streams, which can be passed to 'stream_context' is %d; "+
			"narrow down the matching log streams with additional filters according to https://docs.victoriametrics.com/victorialogs/logsql/#filters", len(m), pipeStreamContextMaxStreams)
	}

	// write result
	wctx := &pipeStreamContextWriteContext{
		pcp: pcp,
	}

	// write output contexts in the ascending order of rows
	streamIDs := getStreamIDsSortedByMinRowTimestamp(m)
	for _, streamID := range streamIDs {
		rows := m[streamID]
		if len(rows) > pipeStreamContextMaxRowsPerStream {
			return fmt.Errorf("too many logs from a single stream passed to 'stream_context': %d; the maximum supported number of logs, which can be passed to 'stream_context' is %d; "+
				"narrow down the matching logs with additional filters according to https://docs.victoriametrics.com/victorialogs/logsql/#filters",
				len(rows), pipeStreamContextMaxRowsPerStream)
		}
		streamRowss, err := pcp.getStreamRowss(streamID, rows, stateSizeBudget)
		if err != nil {
			return err
		}
		if needStop(pcp.stopCh) {
			return nil
		}

		// Write streamRowss to the output.
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

// `stream_context` pipe results are expected to be investigated by humans.
// There is no sense in spending CPU time and other resources for fetching surrounding logs
// for big number of log streams.
// There is no sense in spending CPU time and other resources for fetching surrounding logs
// for big number of log entries per each found log stream.
const pipeStreamContextMaxStreams = 100
const pipeStreamContextMaxRowsPerStream = 1000

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

func parsePipeStreamContext(lex *lexer) (pipe, error) {
	if !lex.isKeyword("stream_context") {
		return nil, fmt.Errorf("expecting 'stream_context'; got %q", lex.token)
	}
	lex.nextToken()

	linesBefore, linesAfter, err := parsePipeStreamContextBeforeAfter(lex)
	if err != nil {
		return nil, err
	}

	timeWindow := pipeStreamContextDefaultTimeWindow
	if lex.isKeyword("time_window") {
		lex.nextToken()
		d, ok := tryParseDuration(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'time_window %s'; it must contain valid duration", lex.token)
		}
		if timeWindow <= 0 {
			return nil, fmt.Errorf("'time_window' must be positive; got %s", lex.token)
		}
		lex.nextToken()
		timeWindow = d
	}

	pc := &pipeStreamContext{
		linesBefore: linesBefore,
		linesAfter:  linesAfter,
		timeWindow:  timeWindow,
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
