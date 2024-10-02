package logstorage

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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

	// if hitsFieldName isn't empty, then the number of hits per each unique value is returned in this field.
	hitsFieldName string
}

func (pt *pipeTop) String() string {
	s := "top"
	if pt.limit != pipeTopDefaultLimit {
		s += " " + pt.limitStr
	}
	if len(pt.byFields) > 0 {
		s += " by (" + fieldNamesString(pt.byFields) + ")"
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

func (pt *pipeTop) optimize() {
	// nothing to do
}

func (pt *pipeTop) hasFilterInWithQuery() bool {
	return false
}

func (pt *pipeTop) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pt, nil
}

func (pt *pipeTop) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeTopProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeTopProcessorShard{
			pipeTopProcessorShardNopad: pipeTopProcessorShardNopad{
				pt: pt,
			},
		}
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

	// m holds per-row hits.
	m map[string]*uint64

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
		for i := 0; i < br.rowsLen; i++ {
			keyBuf = keyBuf[:0]
			for _, c := range cs {
				v := c.getValueAtRow(br, i)
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			shard.updateState(bytesutil.ToUnsafeString(keyBuf), 1)
		}
		shard.keyBuf = keyBuf
		return
	}
	if len(byFields) == 1 {
		// Fast path for a single field.
		c := br.getColumnByName(byFields[0])
		if c.isConst {
			v := c.valuesEncoded[0]
			shard.updateState(v, uint64(br.rowsLen))
			return
		}
		if c.valueType == valueTypeDict {
			c.forEachDictValueWithHits(br, shard.updateState)
			return
		}

		values := c.getValues(br)
		for _, v := range values {
			shard.updateState(v, 1)
		}
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
	for i := 0; i < br.rowsLen; i++ {
		keyBuf = keyBuf[:0]
		for _, values := range columnValues {
			keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(values[i]))
		}
		shard.updateState(bytesutil.ToUnsafeString(keyBuf), 1)
	}
	shard.keyBuf = keyBuf
}

func (shard *pipeTopProcessorShard) updateState(v string, hits uint64) {
	m := shard.getM()
	pHits, ok := m[v]
	if !ok {
		vCopy := strings.Clone(v)
		hits := uint64(0)
		pHits = &hits
		m[vCopy] = pHits
		shard.stateSizeBudget -= len(vCopy) + int(unsafe.Sizeof(vCopy)+unsafe.Sizeof(hits)+unsafe.Sizeof(pHits))
	}
	*pHits += hits
}

func (shard *pipeTopProcessorShard) getM() map[string]*uint64 {
	if shard.m == nil {
		shard.m = make(map[string]*uint64)
	}
	return shard.m
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

	// merge state across shards
	shards := ptp.shards
	m := shards[0].getM()
	shards = shards[1:]
	for i := range shards {
		if needStop(ptp.stopCh) {
			return nil
		}

		for k, pHitsSrc := range shards[i].getM() {
			pHits, ok := m[k]
			if !ok {
				m[k] = pHitsSrc
			} else {
				*pHits += *pHitsSrc
			}
		}
	}

	// select top entries with the biggest number of hits
	entries := make([]pipeTopEntry, 0, len(m))
	for k, pHits := range m {
		entries = append(entries, pipeTopEntry{
			k:    k,
			hits: *pHits,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := &entries[i], &entries[j]
		if a.hits == b.hits {
			return a.k < b.k
		}
		return a.hits > b.hits
	})
	if uint64(len(entries)) > ptp.pt.limit {
		entries = entries[:ptp.pt.limit]
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

	if len(byFields) == 0 {
		for _, e := range entries {
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
			wctx.writeRow(rowFields)
		}
	} else if len(byFields) == 1 {
		fieldName := byFields[0]
		for _, e := range entries {
			if needStop(ptp.stopCh) {
				return nil
			}

			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: e.k,
			})
			rowFields = addHitsField(rowFields, e.hits)
			wctx.writeRow(rowFields)
		}
	} else {
		for _, e := range entries {
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
			wctx.writeRow(rowFields)
		}
	}

	wctx.flush()

	return nil
}

type pipeTopEntry struct {
	k    string
	hits uint64
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
	if wctx.valuesLen >= 1_000_000 {
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

func parsePipeTop(lex *lexer) (*pipeTop, error) {
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

	hitsFieldName := "hits"
	for slices.Contains(byFields, hitsFieldName) {
		hitsFieldName += "s"
	}

	pt := &pipeTop{
		byFields:      byFields,
		limit:         limit,
		limitStr:      limitStr,
		hitsFieldName: hitsFieldName,
	}

	return pt, nil
}
