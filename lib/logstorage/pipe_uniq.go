package logstorage

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipeUniq processes '| uniq ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe
type pipeUniq struct {
	// fields contains field names for returning unique values
	byFields []string

	// if hitsFieldName isn't empty, then the number of hits per each unique value is stored in this field.
	hitsFieldName string

	// limit is the maximum number of unique values to return.
	// If hitsFieldName != "" and the limit is exceeded, then all the hits are set to 0.
	limit uint64
}

func (pu *pipeUniq) String() string {
	s := "uniq by (" + fieldNamesString(pu.byFields) + ")"
	if pu.hitsFieldName != "" {
		s += " with hits"
	}
	if pu.limit > 0 {
		s += fmt.Sprintf(" limit %d", pu.limit)
	}
	return s
}

func (pu *pipeUniq) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	if pu.hitsFieldName == "" {
		return pu, []pipe{pu}
	}

	pLocal := &pipeUniqLocal{
		pu: pu,
	}
	return pu, []pipe{pLocal}
}

func (pu *pipeUniq) canLiveTail() bool {
	return false
}

func (pu *pipeUniq) updateNeededFields(pf *prefixfilter.Filter) {
	pf.Reset()
	pf.AddAllowFilters(pu.byFields)
}

func (pu *pipeUniq) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUniq) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pu, nil
}

func (pu *pipeUniq) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pu *pipeUniq) newPipeProcessor(concurrency int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.4)

	pup := &pipeUniqProcessor{
		pu:     pu,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		maxStateSize: maxStateSize,
	}
	pup.shards.Init = func(shard *pipeUniqProcessorShard) {
		shard.pu = pu
		shard.m.init(uint(concurrency), &shard.stateSizeBudget)
	}
	pup.stateSizeBudget.Store(maxStateSize)

	return pup
}

type pipeUniqProcessor struct {
	pu     *pipeUniq
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeUniqProcessorShard]

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeUniqProcessorShard struct {
	// pu points to the parent pipeUniq.
	pu *pipeUniq

	// m holds per-row hits.
	m hitsMapAdaptive

	// keyBuf is a temporary buffer for building keys for m.
	keyBuf []byte

	// columnValues is a temporary buffer for the processed column values.
	columnValues [][]string

	// stateSizeBudget is the remaining budget for the whole state size for the shard.
	// The per-shard budget is provided in chunks from the parent pipeUniqProcessor.
	stateSizeBudget int
}

// writeBlock writes br to shard.
//
// It returns false if the block cannot be written because of the exceeded limit.
func (shard *pipeUniqProcessorShard) writeBlock(br *blockResult) bool {
	if limit := shard.pu.limit; limit > 0 && shard.m.entriesCount() > limit {
		return false
	}

	needHits := shard.pu.hitsFieldName != ""
	byFields := shard.pu.byFields
	if len(byFields) == 1 {
		// Fast path for a single field.
		shard.updateStatsSingleColumn(br, byFields[0], needHits)
		return true
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
	for i := 0; i < br.rowsLen; i++ {
		seenValue := true
		for _, values := range columnValues {
			if needHits || i == 0 || values[i-1] != values[i] {
				seenValue = false
				break
			}
		}
		if seenValue {
			continue
		}

		keyBuf = keyBuf[:0]
		for _, values := range columnValues {
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[i]))
		}
		shard.m.updateStateString(keyBuf, 1)
	}
	shard.keyBuf = keyBuf

	return true
}

func (shard *pipeUniqProcessorShard) updateStatsSingleColumn(br *blockResult, columnName string, needHits bool) {
	c := br.getColumnByName(columnName)
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
		for i, v := range values {
			if needHits || i == 0 || values[i-1] != v {
				shard.m.updateStateGeneric(v, 1)
			}
		}
	}
}

func (pup *pipeUniqProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pup.shards.Get(workerID)

	for shard.stateSizeBudget < 0 {
		// steal some budget for the state size from the global budget.
		remaining := pup.stateSizeBudget.Add(-stateSizeBudgetChunk)
		if remaining < 0 {
			// The state size is too big. Stop processing data in order to avoid OOM crash.
			if remaining+stateSizeBudgetChunk >= 0 {
				// Notify worker goroutines to stop calling writeBlock() in order to save CPU time.
				pup.cancel()
			}
			return
		}
		shard.stateSizeBudget += stateSizeBudgetChunk
	}

	if !shard.writeBlock(br) {
		pup.cancel()
	}
}

func (pup *pipeUniqProcessor) flush() error {
	if n := pup.stateSizeBudget.Load(); n <= 0 {
		return fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", pup.pu.String(), pup.maxStateSize/(1<<20))
	}

	// merge state across shards in parallel
	hms := pup.mergeShardsParallel()
	if needStop(pup.stopCh) {
		return nil
	}

	resetHits := false
	if limit := pup.pu.limit; limit > 0 {
		// Trim the number of entries according to the given limit
		entriesLen := uint64(0)
		result := hms[:0]
		for _, hm := range hms {
			entriesLen += hm.entriesCount()
			if entriesLen <= limit {
				result = append(result, hm)
				continue
			}

			// There is little sense in returning partial hits when the limit on the number of unique entries is reached,
			// since arbitrary number of unique entries and hits for these entries could be skipped.
			// It is better to return zero hits instead of misleading hits results.
			resetHits = true
			for n := range hm.u64 {
				if entriesLen <= limit {
					break
				}
				delete(hm.u64, n)
				entriesLen--
			}
			for n := range hm.negative64 {
				if entriesLen <= limit {
					break
				}
				delete(hm.negative64, n)
				entriesLen--
			}
			for k := range hm.strings {
				if entriesLen <= limit {
					break
				}
				delete(hm.strings, k)
				entriesLen--
			}
			if hm.entriesCount() > 0 {
				result = append(result, hm)
			}
			break
		}
		hms = result
	}

	// Write the calculated stats in parallel to the next pipe.
	var wg sync.WaitGroup
	for i := range hms {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			pup.writeShardData(workerID, hms[workerID], resetHits)
		}(uint(i))
	}
	wg.Wait()

	return nil
}

func (pup *pipeUniqProcessor) writeShardData(workerID uint, hm *hitsMap, resetHits bool) {
	wctx := &pipeUniqWriteContext{
		workerID: workerID,
		ppNext:   pup.ppNext,
	}

	byFields := pup.pu.byFields
	var rowFields []Field

	addHitsFieldIfNeeded := func(dst []Field, pHits *uint64) []Field {
		if pup.pu.hitsFieldName == "" {
			return dst
		}
		hits := uint64(0)
		if !resetHits {
			hits = *pHits
		}
		dst = append(dst, Field{
			Name:  pup.pu.hitsFieldName,
			Value: wctx.getUint64String(hits),
		})
		return dst
	}

	if len(byFields) == 1 {
		fieldName := byFields[0]
		for n, pHits := range hm.u64 {
			if needStop(pup.stopCh) {
				return
			}
			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: wctx.getUint64String(n),
			})
			rowFields = addHitsFieldIfNeeded(rowFields, pHits)
			wctx.writeRow(rowFields)
		}
		for n, pHits := range hm.negative64 {
			if needStop(pup.stopCh) {
				return
			}
			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: wctx.getInt64String(int64(n)),
			})
			rowFields = addHitsFieldIfNeeded(rowFields, pHits)
			wctx.writeRow(rowFields)
		}
		for k, pHits := range hm.strings {
			if needStop(pup.stopCh) {
				return
			}
			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: k,
			})
			rowFields = addHitsFieldIfNeeded(rowFields, pHits)
			wctx.writeRow(rowFields)
		}
	} else {
		for k, pHits := range hm.strings {
			if needStop(pup.stopCh) {
				return
			}

			rowFields = rowFields[:0]
			keyBuf := bytesutil.ToUnsafeBytes(k)
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
			rowFields = addHitsFieldIfNeeded(rowFields, pHits)
			wctx.writeRow(rowFields)
		}
	}

	wctx.flush()
}

func (pup *pipeUniqProcessor) mergeShardsParallel() []*hitsMap {
	shards := pup.shards.All()
	if len(shards) == 0 {
		return nil
	}

	hmas := make([]*hitsMapAdaptive, 0, len(shards))
	for _, shard := range shards {
		hma := &shard.m
		if hma.entriesCount() > 0 {
			hmas = append(hmas, hma)
		}
	}

	var hmsResult []*hitsMap
	var hmsLock sync.Mutex
	hitsMapMergeParallel(hmas, pup.stopCh, func(hm *hitsMap) {
		if hm.entriesCount() > 0 {
			hmsLock.Lock()
			hmsResult = append(hmsResult, hm)
			hmsLock.Unlock()
		}
	})
	if needStop(pup.stopCh) {
		return nil
	}

	return hmsResult
}

type pipeUniqWriteContext struct {
	workerID uint
	ppNext   pipeProcessor
	rcs      []resultColumn
	br       blockResult

	a arena

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeUniqWriteContext) getUint64String(n uint64) string {
	bLen := len(wctx.a.b)
	wctx.a.b = marshalUint64String(wctx.a.b, n)
	return bytesutil.ToUnsafeString(wctx.a.b[bLen:])
}

func (wctx *pipeUniqWriteContext) getInt64String(n int64) string {
	bLen := len(wctx.a.b)
	wctx.a.b = marshalInt64String(wctx.a.b, n)
	return bytesutil.ToUnsafeString(wctx.a.b[bLen:])
}

func (wctx *pipeUniqWriteContext) writeRow(rowFields []Field) {
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

func (wctx *pipeUniqWriteContext) flush() {
	if wctx.rowsCount == 0 {
		return
	}

	// Flush rcs to ppNext
	wctx.br.setResultColumns(wctx.rcs, wctx.rowsCount)
	wctx.valuesLen = 0
	wctx.rowsCount = 0
	wctx.ppNext.writeBlock(wctx.workerID, &wctx.br)
	wctx.br.reset()
	for i := range wctx.rcs {
		wctx.rcs[i].resetValues()
	}
	wctx.a.reset()
}

func parsePipeUniq(lex *lexer) (pipe, error) {
	if !lex.isKeyword("uniq") {
		return nil, fmt.Errorf("expecting 'uniq'; got %q", lex.token)
	}
	lex.nextToken()

	needFields := false
	if lex.isKeyword("by") {
		lex.nextToken()
		needFields = true
	}

	var byFields []string
	if lex.isKeyword("(") {
		bfs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by(...)': %w", err)
		}
		byFields = bfs
	} else if !lex.isKeyword("with", "hits", "limit", ")", "|", "") {
		bfs, err := parseCommaSeparatedFields(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by ...': %w", err)
		}
		byFields = bfs
	} else if needFields {
		return nil, fmt.Errorf("missing fields after 'by'")
	}
	if len(byFields) == 0 {
		return nil, fmt.Errorf("missing fields inside 'by(...)'")
	}

	pu := &pipeUniq{
		byFields: byFields,
	}

	if lex.isKeyword("with") {
		lex.nextToken()
		if !lex.isKeyword("hits") {
			return nil, fmt.Errorf("missing 'hits' after 'with'")
		}
	}
	if lex.isKeyword("hits") {
		lex.nextToken()
		hitsFieldName := "hits"
		for slices.Contains(pu.byFields, hitsFieldName) {
			hitsFieldName += "s"
		}

		pu.hitsFieldName = hitsFieldName
	}

	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s'", lex.token)
		}
		lex.nextToken()
		pu.limit = n
	}

	return pu, nil
}
