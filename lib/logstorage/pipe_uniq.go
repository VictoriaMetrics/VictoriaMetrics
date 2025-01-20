package logstorage

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// pipeUniq processes '| uniq ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#uniq-pipe
type pipeUniq struct {
	// fields contains field names for returning unique values
	byFields []string

	// if hitsFieldName isn't empty, then the number of hits per each unique value is stored in this field.
	hitsFieldName string

	limit uint64
}

func (pu *pipeUniq) String() string {
	s := "uniq"
	if len(pu.byFields) > 0 {
		s += " by (" + fieldNamesString(pu.byFields) + ")"
	}
	if pu.hitsFieldName != "" {
		s += " with hits"
	}
	if pu.limit > 0 {
		s += fmt.Sprintf(" limit %d", pu.limit)
	}
	return s
}

func (pu *pipeUniq) canLiveTail() bool {
	return false
}

func (pu *pipeUniq) updateNeededFields(neededFields, unneededFields fieldsSet) {
	neededFields.reset()
	unneededFields.reset()

	if len(pu.byFields) == 0 {
		neededFields.add("*")
	} else {
		neededFields.addFields(pu.byFields)
	}
}

func (pu *pipeUniq) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUniq) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pu, nil
}

func (pu *pipeUniq) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.4)

	shards := make([]pipeUniqProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeUniqProcessorShard{
			pipeUniqProcessorShardNopad: pipeUniqProcessorShardNopad{
				pu: pu,
			},
		}
		shards[i].m.init(&shards[i].stateSizeBudget)
	}

	pup := &pipeUniqProcessor{
		pu:     pu,
		stopCh: stopCh,
		cancel: cancel,
		ppNext: ppNext,

		shards: shards,

		maxStateSize: maxStateSize,
	}
	pup.stateSizeBudget.Store(maxStateSize)

	return pup
}

type pipeUniqProcessor struct {
	pu     *pipeUniq
	stopCh <-chan struct{}
	cancel func()
	ppNext pipeProcessor

	shards []pipeUniqProcessorShard

	maxStateSize    int64
	stateSizeBudget atomic.Int64
}

type pipeUniqProcessorShard struct {
	pipeUniqProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUniqProcessorShardNopad{})%128]byte
}

type pipeUniqProcessorShardNopad struct {
	// pu points to the parent pipeUniq.
	pu *pipeUniq

	// m holds per-row hits.
	m hitsMap

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
	if len(byFields) == 0 {
		// Take into account all the columns in br.
		keyBuf := shard.keyBuf
		cs := br.getColumns()
		for i := 0; i < br.rowsLen; i++ {
			keyBuf = keyBuf[:0]
			for _, c := range cs {
				v := c.getValueAtRow(br, i)
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			shard.m.updateStateString(keyBuf, 1)
		}
		shard.keyBuf = keyBuf
		return true
	}
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

	shard := &pup.shards[workerID]

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
		pup:      pup,
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

	if len(byFields) == 0 {
		for k, pHits := range hm.strings {
			if needStop(pup.stopCh) {
				return
			}

			rowFields = rowFields[:0]
			keyBuf := bytesutil.ToUnsafeBytes(k)
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
			rowFields = addHitsFieldIfNeeded(rowFields, pHits)
			wctx.writeRow(rowFields)
		}
	} else if len(byFields) == 1 {
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
	hms := make([]*hitsMap, 0, len(pup.shards))
	for i := range pup.shards {
		hm := &pup.shards[i].m
		if hm.entriesCount() > 0 {
			hms = append(hms, hm)
		}
	}

	cpusCount := cgroup.AvailableCPUs()
	hmsResult := make([]*hitsMap, 0, cpusCount)
	var hmsLock sync.Mutex
	hitsMapMergeParallel(hms, cpusCount, pup.stopCh, func(hm *hitsMap) {
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
	pup      *pipeUniqProcessor
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
	wctx.pup.ppNext.writeBlock(wctx.workerID, &wctx.br)
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

	var pu pipeUniq
	if lex.isKeyword("by", "(") {
		if lex.isKeyword("by") {
			lex.nextToken()
		}
		bfs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse 'by' clause: %w", err)
		}
		if slices.Contains(bfs, "*") {
			bfs = nil
		}
		pu.byFields = bfs
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

	return &pu, nil
}
