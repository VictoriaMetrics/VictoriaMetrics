package logstorage

import (
	"container/heap"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// pipeTopDefaultLimit is the default number of entries pipeTop returns.
const pipeTopDefaultLimit = 10

// pipeTop processes '| top ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#top-pipe
type pipeTop struct {
	// fields contains field names for returning top values for.
	byFields []string

	// limit is the number of top (byFields) sets to return.
	limit uint64

	// limitStr is string representation of the limit.
	limitStr string

	// the number of hits per each unique value is returned in this field.
	hitsFieldName string

	// if rankFieldName isn't empty, then the rank per each unique value is returned in this field.
	rankFieldName string
}

func (pt *pipeTop) String() string {
	s := "top"
	if pt.limit != pipeTopDefaultLimit {
		s += " " + pt.limitStr
	}
	if len(pt.byFields) > 0 {
		s += " by (" + fieldNamesString(pt.byFields) + ")"
	}
	if pt.hitsFieldName != "hits" {
		s += " hits as " + quoteTokenIfNeeded(pt.hitsFieldName)
	}
	if pt.rankFieldName != "" {
		s += rankFieldNameString(pt.rankFieldName)
	}
	return s
}

func (pt *pipeTop) canLiveTail() bool {
	return false
}

func (pt *pipeTop) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.reset()
	unneededFields.reset()

	if len(pt.byFields) == 0 {
		neededFields.add("*")
	} else {
		neededFields.addFields(pt.byFields)
	}
}

func (pt *pipeTop) hasFilterInWithQuery() bool {
	return false
}

func (pt *pipeTop) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pt, nil
}

func (pt *pipeTop) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.4)

	shards := make([]pipeTopProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeTopProcessorShard{
			pipeTopProcessorShardNopad: pipeTopProcessorShardNopad{
				pt: pt,
			},
		}
		shards[i].m.init(&shards[i].stateSizeBudget)
	}

	ptp := &pipeTopProcessor{
		pt:     pt,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	ptp.stateSizeBudget.Store(maxStateSize)

	return ptp
}

type pipeTopProcessor struct {
	pt     *pipeTop
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards []pipeTopProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeTopProcessorShard struct {
	pipeTopProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeTopProcessorShardNopad{})%128]byte
}

type pipeTopProcessorShardNopad struct {
	// pt points to the parent pipeTop.
	pt *pipeTop

	// m holds per-value hits.
	m hitsMap

	// keyBuf is a temporary buffer for building keys for m.
	keyBuf []byte

	// columnValues is a temporary buffer for the processed column values.
	columnValues [][]string

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeTopProcessor.
	stateSizeBudget int
}

// writeBlock writes br to shard.
func (shard *pipeTopProcessorShard) writeBlock(br *blockResult) {
	byFields := shard.pt.byFields
	if len(byFields) == 0 {
		// Take into account all the columns in br.
		keyBuf := shard.keyBuf
		cs := br.getColumns()
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			keyBuf = keyBuf[:0]
			for _, c := range cs {
				v := c.getValueAtRow(br, rowIdx)
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			shard.m.updateStateString(keyBuf, 1)
		}
		shard.keyBuf = keyBuf
		return
	}
	if len(byFields) == 1 {
		// Fast path for a single field.
		shard.updateStatsSingleColumn(br, byFields[0])
		return
	}

	// Take into account only the selected columns.
	columnValues := shard.columnValues[:0]
	for _, f := range byFields {
		c := br.getColumnByName(f)
		values := c.getValues(br)
		columnValues = append(columnValues, values)
	}
	shard.columnValues = columnValues

	keyBuf := shard.keyBuf
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		keyBuf = keyBuf[:0]
		for _, values := range columnValues {
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[rowIdx]))
		}
		shard.m.updateStateString(keyBuf, 1)
	}
	shard.keyBuf = keyBuf
}

func (shard *pipeTopProcessorShard) updateStatsSingleColumn(br *blockResult, fieldName string) {
	c := br.getColumnByName(fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		shard.m.updateStateGeneric(v, uint64(br.rowsLen))
		return
	}
	switch c.valueType {
	case valueTypeDict:
		c.forEachDictValueWithHits(br, shard.m.updateStateGeneric)
	case valueTypeUint8:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint8(v)
			shard.m.updateStateUint64(uint64(n), 1)
		}
	case valueTypeUint16:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint16(v)
			shard.m.updateStateUint64(uint64(n), 1)
		}
	case valueTypeUint32:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint32(v)
			shard.m.updateStateUint64(uint64(n), 1)
		}
	case valueTypeUint64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalUint64(v)
			shard.m.updateStateUint64(n, 1)
		}
	case valueTypeInt64:
		values := c.getValuesEncoded(br)
		for _, v := range values {
			n := unmarshalInt64(v)
			shard.m.updateStateInt64(n, 1)
		}
	default:
		values := c.getValues(br)
		for _, v := range values {
			shard.m.updateStateGeneric(v, 1)
		}
	}
}

func (ptp *pipeTopProcessor) writeBlock(workerID uint, br *blockResult) {
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

func (ptp *pipeTopProcessor) flush() error {
	if n := ptp.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", ptp.pt.String(), ptp.maxStateSize/(1<<20))
	}

	// merge state across shards in parallel
	entries := ptp.mergeShardsParallel()
	if needStop(ptp.stopCh) {
		return nil
	}

	// write result
	wctx := &pipeTopWriteContext{
		ptp: ptp,
	}
	byFields := ptp.pt.byFields
	var rowFields []Field

	addHitsField := func(dst []Field, hits uint64) []Field {
		hitsStr := string(marshalUint64String(nil, hits))
		dst = append(dst, Field{
			Name:  ptp.pt.hitsFieldName,
			Value: hitsStr,
		})
		return dst
	}

	addRankField := func(dst []Field, rank int) []Field {
		if ptp.pt.rankFieldName == "" {
			return dst
		}
		rankStr := strconv.Itoa(rank + 1)
		dst = append(dst, Field{
			Name:  ptp.pt.rankFieldName,
			Value: rankStr,
		})
		return dst
	}

	if len(byFields) == 0 {
		for i, e := range entries {
			if needStop(ptp.stopCh) {
				return nil
			}

			rowFields = rowFields[:0]
			keyBuf := bytesutil.ToUnsafeBytes(e.k)
			for len(keyBuf) > 0 {
				name, nSize := encoding.UnmarshalBytes(keyBuf)
				if nSize <= 0 {
					logger.Panicf("BUG: cannot unmarshal field name")
				}
				keyBuf = keyBuf[nSize:]

				value, nSize := encoding.UnmarshalBytes(keyBuf)
				if nSize <= 0 {
					logger.Panicf("BUG: cannot unmarshal field value")
				}
				keyBuf = keyBuf[nSize:]

				rowFields = append(rowFields, Field{
					Name:  bytesutil.ToUnsafeString(name),
					Value: bytesutil.ToUnsafeString(value),
				})
			}
			rowFields = addHitsField(rowFields, e.hits)
			rowFields = addRankField(rowFields, i)
			wctx.writeRow(rowFields)
		}
	} else if len(byFields) == 1 {
		fieldName := byFields[0]
		for i, e := range entries {
			if needStop(ptp.stopCh) {
				return nil
			}

			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: e.k,
			})
			rowFields = addHitsField(rowFields, e.hits)
			rowFields = addRankField(rowFields, i)
			wctx.writeRow(rowFields)
		}
	} else {
		for i, e := range entries {
			if needStop(ptp.stopCh) {
				return nil
			}

			rowFields = rowFields[:0]
			keyBuf := bytesutil.ToUnsafeBytes(e.k)
			fieldIdx := 0
			for len(keyBuf) > 0 {
				value, nSize := encoding.UnmarshalBytes(keyBuf)
				if nSize <= 0 {
					logger.Panicf("BUG: cannot unmarshal field value")
				}
				keyBuf = keyBuf[nSize:]

				rowFields = append(rowFields, Field{
					Name:  byFields[fieldIdx],
					Value: bytesutil.ToUnsafeString(value),
				})
				fieldIdx++
			}
			rowFields = addHitsField(rowFields, e.hits)
			rowFields = addRankField(rowFields, i)
			wctx.writeRow(rowFields)
		}
	}

	wctx.flush()

	return nil
}

func (ptp *pipeTopProcessor) mergeShardsParallel() []*pipeTopEntry {
	limit := ptp.pt.limit
	if limit == 0 {
		return nil
	}

	hms := make([]*hitsMap, 0, len(ptp.shards))
	for i := range ptp.shards {
		hm := &ptp.shards[i].m
		if hm.entriesCount() > 0 {
			hms = append(hms, hm)
		}
	}

	cpusCount := cgroup.AvailableCPUs()
	var entries []*pipeTopEntry
	var entriesLock sync.Mutex
	hitsMapMergeParallel(hms, cpusCount, ptp.stopCh, func(hm *hitsMap) {
		es := getTopEntries(hm, limit, ptp.stopCh)
		entriesLock.Lock()
		entries = append(entries, es...)
		entriesLock.Unlock()
	})
	if needStop(ptp.stopCh) {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[j].less(entries[i])
	})
	if uint64(len(entries)) > limit {
		entries = entries[:limit]
	}

	return entries
}

func getTopEntries(hm *hitsMap, limit uint64, stopCh <-chan struct{}) []*pipeTopEntry {
	if limit == 0 {
		return nil
	}

	var eh topEntriesHeap
	var e pipeTopEntry

	pushEntry := func(k string, hits uint64, kCopy bool) {
		e.k = k
		e.hits = hits
		if uint64(len(eh)) < limit {
			eCopy := e
			if kCopy {
				eCopy.k = strings.Clone(eCopy.k)
			}
			heap.Push(&eh, &eCopy)
			return
		}

		if !eh[0].less(&e) {
			return
		}
		eCopy := e
		if kCopy {
			eCopy.k = strings.Clone(eCopy.k)
		}
		eh[0] = &eCopy
		heap.Fix(&eh, 0)
	}

	var b []byte
	for n, pHits := range hm.u64 {
		if needStop(stopCh) {
			return nil
		}
		b = marshalUint64String(b[:0], n)
		pushEntry(bytesutil.ToUnsafeString(b), *pHits, true)
	}
	for n, pHits := range hm.negative64 {
		if needStop(stopCh) {
			return nil
		}
		b = marshalInt64String(b[:0], int64(n))
		pushEntry(bytesutil.ToUnsafeString(b), *pHits, true)
	}
	for k, pHits := range hm.strings {
		if needStop(stopCh) {
			return nil
		}
		pushEntry(k, *pHits, false)
	}

	result := ([]*pipeTopEntry)(eh)
	for len(eh) > 0 {
		x := heap.Pop(&eh)
		result[len(eh)] = x.(*pipeTopEntry)
	}

	return result
}

type topEntriesHeap []*pipeTopEntry

func (h *topEntriesHeap) Less(i, j int) bool {
	a := *h
	return a[i].less(a[j])
}
func (h *topEntriesHeap) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *topEntriesHeap) Len() int {
	return len(*h)
}
func (h *topEntriesHeap) Push(v any) {
	x := v.(*pipeTopEntry)
	*h = append(*h, x)
}
func (h *topEntriesHeap) Pop() any {
	a := *h
	x := a[len(a)-1]
	a[len(a)-1] = nil
	*h = a[:len(a)-1]
	return x
}

type pipeTopEntry struct {
	k    string
	hits uint64
}

func (e *pipeTopEntry) less(r *pipeTopEntry) bool {
	if e.hits == r.hits {
		return e.k > r.k
	}
	return e.hits < r.hits
}

type pipeTopWriteContext struct {
	ptp *pipeTopProcessor
	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeTopWriteContext) writeRow(rowFields []Field) {
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

	// The 64_000 limit provides the best performance results.
	if wctx.valuesLen >= 64_000 {
		wctx.flush()
	}
}

func (wctx *pipeTopWriteContext) flush() {
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
}

func parsePipeTop(lex *lexer) (pipe, error) {
	if !lex.isKeyword("top") {
		return nil, fmt.Errorf("expecting 'top'; got %q", lex.token)
	}
	lex.nextToken()

	limit := uint64(pipeTopDefaultLimit)
	limitStr := ""
	if isNumberPrefix(lex.token) {
		limitF, s, err := parseNumber(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse N in 'top': %w", err)
		}
		if limitF < 1 {
			return nil, fmt.Errorf("N in 'top %s' must be integer bigger than 0", s)
		}
		limit = uint64(limitF)
		limitStr = s
	}

	var byFields []string
	if lex.isKeyword("by", "(") {
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		bfs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by' clause in 'top': %w", err)
		}
		if slices.Contains(bfs, "*") {
			bfs = nil
		}
		byFields = bfs
	}

	pt := &pipeTop{
		byFields:      byFields,
		limit:         limit,
		limitStr:      limitStr,
		hitsFieldName: "hits",
	}

	for {
		switch {
		case lex.isKeyword("hits"):
			lex.nextToken()
			if lex.isKeyword("as") {
				lex.nextToken()
			}
			s, err := getCompoundToken(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse 'hits' name: %w", err)
			}
			pt.hitsFieldName = s
		case lex.isKeyword("rank"):
			rankFieldName, err := parseRankFieldName(lex)
			if err != nil {
				return nil, fmt.Errorf("cannot parse rank field name in [%s]: %w", pt, err)
			}
			pt.rankFieldName = rankFieldName
			for slices.Contains(byFields, pt.rankFieldName) {
				pt.rankFieldName += "s"
			}
		default:
			for slices.Contains(byFields, pt.hitsFieldName) {
				pt.hitsFieldName += "s"
			}
			return pt, nil
		}
	}
}

func parseRankFieldName(lex *lexer) (string, error) {
	if !lex.isKeyword("rank") {
		return "", fmt.Errorf("unexpected token: %q; want 'rank'", lex.token)
	}
	lex.nextToken()

	rankFieldName := "rank"
	if lex.isKeyword("as") {
		lex.nextToken()
		if lex.isKeyword("", "|", ")", "(") {
			return "", fmt.Errorf("missing rank name")
		}
	}
	if !lex.isKeyword("", "|", ")", "limit") {
		s, err := getCompoundToken(lex)
		if err != nil {
			return "", err
		}
		rankFieldName = s
	}
	return rankFieldName, nil
}

func rankFieldNameString(rankFieldName string) string {
	s := " rank"
	if rankFieldName != "rank" {
		s += " as " + quoteTokenIfNeeded(rankFieldName)
	}
	return s
}
