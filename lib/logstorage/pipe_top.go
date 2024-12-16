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

	"github.com/cespare/xxhash/v2"

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
	pHits := m[v]
	if pHits == nil {
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

	// merge state across shards in parallel
	entries, err := ptp.mergeShardsParallel()
	if err != nil {
		return err
	}
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

func (ptp *pipeTopProcessor) mergeShardsParallel() ([]*pipeTopEntry, error) {
	limit := ptp.pt.limit
	if limit == 0 {
		return nil, nil
	}

	shards := ptp.shards
	shardsLen := len(shards)
	if shardsLen == 1 {
		entries := getTopEntries(shards[0].getM(), limit, ptp.stopCh)
		return entries, nil
	}

	var wg sync.WaitGroup
	perShardMaps := make([][]map[string]*uint64, shardsLen)
	for i := range shards {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			shardMaps := make([]map[string]*uint64, shardsLen)
			for i := range shardMaps {
				shardMaps[i] = make(map[string]*uint64)
			}

			n := int64(0)
			nTotal := int64(0)
			for k, pHits := range shards[idx].getM() {
				if needStop(ptp.stopCh) {
					return
				}
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(k))
				m := shardMaps[h%uint64(len(shardMaps))]
				n += updatePipeTopMap(m, k, pHits)
				if n > stateSizeBudgetChunk {
					if nRemaining := ptp.stateSizeBudget.Add(-n); nRemaining < 0 {
						return
					}
					nTotal += n
					n = 0
				}
			}
			nTotal += n
			ptp.stateSizeBudget.Add(-n)

			perShardMaps[idx] = shardMaps

			// Clean the original map and return its state size budget back.
			shards[idx].m = nil
			ptp.stateSizeBudget.Add(nTotal)
		}(i)
	}
	wg.Wait()
	if needStop(ptp.stopCh) {
		return nil, nil
	}
	if n := ptp.stateSizeBudget.Load(); n < 0 {
		return nil, fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", ptp.pt.String(), ptp.maxStateSize/(1<<20))
	}

	// Obtain topN entries per each shard
	entriess := make([][]*pipeTopEntry, shardsLen)
	for i := range entriess {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			m := perShardMaps[0][idx]
			for i := 1; i < len(perShardMaps); i++ {
				n := int64(0)
				nTotal := int64(0)
				for k, pHits := range perShardMaps[i][idx] {
					if needStop(ptp.stopCh) {
						return
					}
					n += updatePipeTopMap(m, k, pHits)
					if n > stateSizeBudgetChunk {
						if nRemaining := ptp.stateSizeBudget.Add(-n); nRemaining < 0 {
							return
						}
						nTotal += n
						n = 0
					}
				}
				nTotal += n
				ptp.stateSizeBudget.Add(-n)

				// Clean the original map and return its state size budget back.
				perShardMaps[i][idx] = nil
				ptp.stateSizeBudget.Add(nTotal)
			}
			perShardMaps[0][idx] = nil

			entriess[idx] = getTopEntries(m, limit, ptp.stopCh)
		}(i)
	}
	wg.Wait()
	if needStop(ptp.stopCh) {
		return nil, nil
	}
	if n := ptp.stateSizeBudget.Load(); n < 0 {
		return nil, fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", ptp.pt.String(), ptp.maxStateSize/(1<<20))
	}

	// merge entriess
	entries := entriess[0]
	for _, es := range entriess[1:] {
		entries = append(entries, es...)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[j].less(entries[i])
	})
	if uint64(len(entries)) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func getTopEntries(m map[string]*uint64, limit uint64, stopCh <-chan struct{}) []*pipeTopEntry {
	if limit == 0 {
		return nil
	}

	var eh topEntriesHeap
	for k, pHits := range m {
		if needStop(stopCh) {
			return nil
		}

		e := pipeTopEntry{
			k:    k,
			hits: *pHits,
		}
		if uint64(len(eh)) < limit {
			eCopy := e
			heap.Push(&eh, &eCopy)
			continue
		}
		if eh[0].less(&e) {
			eCopy := e
			eh[0] = &eCopy
			heap.Fix(&eh, 0)
		}
	}

	result := ([]*pipeTopEntry)(eh)
	for len(eh) > 0 {
		x := heap.Pop(&eh)
		result[len(eh)] = x.(*pipeTopEntry)
	}

	return result
}

func updatePipeTopMap(m map[string]*uint64, k string, pHitsSrc *uint64) int64 {
	pHitsDst := m[k]
	if pHitsDst != nil {
		*pHitsDst += *pHitsSrc
		return 0
	}

	m[k] = pHitsSrc
	return int64(unsafe.Sizeof(k) + unsafe.Sizeof(pHitsSrc))
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

	if lex.isKeyword("rank") {
		rankFieldName, err := parseRankFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse rank field name in [%s]: %w", pt, err)
		}
		pt.rankFieldName = rankFieldName
	}
	return pt, nil
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
		s += " as " + rankFieldName
	}
	return s
}
