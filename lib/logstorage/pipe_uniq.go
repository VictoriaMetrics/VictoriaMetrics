package logstorage

import (
	"fmt"
	"slices"
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

func (pu *pipeUniq) optimize() {
	// nothing to do
}

func (pu *pipeUniq) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUniq) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pu, nil
}

func (pu *pipeUniq) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeUniqProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeUniqProcessorShard{
			pipeUniqProcessorShardNopad: pipeUniqProcessorShardNopad{
				pu: pu,
			},
		}
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
	m map[string]*uint64

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
	if limit := shard.pu.limit; limit > 0 && uint64(len(shard.m)) > limit {
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
			shard.updateState(bytesutil.ToUnsafeString(keyBuf), 1)
		}
		shard.keyBuf = keyBuf
		return true
	}
	if len(byFields) == 1 {
		// Fast path for a single field.
		c := br.getColumnByName(byFields[0])
		if c.isConst {
			v := c.valuesEncoded[0]
			shard.updateState(v, uint64(br.rowsLen))
			return true
		}
		if c.valueType == valueTypeDict {
			c.forEachDictValueWithHits(br, shard.updateState)
			return true
		}

		values := c.getValues(br)
		for i, v := range values {
			if needHits || i == 0 || values[i-1] != values[i] {
				shard.updateState(v, 1)
			}
		}
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
		shard.updateState(bytesutil.ToUnsafeString(keyBuf), 1)
	}
	shard.keyBuf = keyBuf

	return true
}

func (shard *pipeUniqProcessorShard) updateState(v string, hits uint64) {
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

func (shard *pipeUniqProcessorShard) getM() map[string]*uint64 {
	if shard.m == nil {
		shard.m = make(map[string]*uint64)
	}
	return shard.m
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
	ms, err := pup.mergeShardsParallel()
	if err != nil {
		return err
	}
	if needStop(pup.stopCh) {
		return nil
	}

	resetHits := false
	if limit := pup.pu.limit; limit > 0 {
		// Trim the number of entries according to the given limit
		entriesLen := 0
		result := ms[:0]
		for _, m := range ms {
			entriesLen += len(m)
			if uint64(entriesLen) <= limit {
				result = append(result, m)
				continue
			}

			// There is little sense in returning partial hits when the limit on the number of unique entries is reached,
			// since arbitrary number of unique entries and hits for these entries could be skipped.
			// It is better to return zero hits instead of misleading hits results.
			resetHits = true
			for k := range m {
				delete(m, k)
				entriesLen--
				if uint64(entriesLen) <= limit {
					break
				}
			}
			if len(m) > 0 {
				result = append(result, m)
			}
			break
		}
		ms = result
	}

	// Write the calculated stats in parallel to the next pipe.
	var wg sync.WaitGroup
	for i, m := range ms {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			pup.writeShardData(workerID, m, resetHits)
		}(uint(i))
	}
	wg.Wait()

	return nil
}

func (pup *pipeUniqProcessor) writeShardData(workerID uint, m map[string]*uint64, resetHits bool) {
	wctx := &pipeUniqWriteContext{
		workerID: workerID,
		pup:      pup,
	}
	byFields := pup.pu.byFields
	var rowFields []Field

	addHitsFieldIfNeeded := func(dst []Field, hits uint64) []Field {
		if pup.pu.hitsFieldName == "" {
			return dst
		}
		if resetHits {
			hits = 0
		}
		hitsStr := string(marshalUint64String(nil, hits))
		dst = append(dst, Field{
			Name:  pup.pu.hitsFieldName,
			Value: hitsStr,
		})
		return dst
	}

	if len(byFields) == 0 {
		for k, pHits := range m {
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
			rowFields = addHitsFieldIfNeeded(rowFields, *pHits)
			wctx.writeRow(rowFields)
		}
	} else if len(byFields) == 1 {
		fieldName := byFields[0]
		for k, pHits := range m {
			if needStop(pup.stopCh) {
				return
			}

			rowFields = append(rowFields[:0], Field{
				Name:  fieldName,
				Value: k,
			})
			rowFields = addHitsFieldIfNeeded(rowFields, *pHits)
			wctx.writeRow(rowFields)
		}
	} else {
		for k, pHits := range m {
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
			rowFields = addHitsFieldIfNeeded(rowFields, *pHits)
			wctx.writeRow(rowFields)
		}
	}

	wctx.flush()
}

func (pup *pipeUniqProcessor) mergeShardsParallel() ([]map[string]*uint64, error) {
	shards := pup.shards
	shardsLen := len(shards)
	if shardsLen == 1 {
		m := shards[0].getM()
		var ms []map[string]*uint64
		if len(m) > 0 {
			ms = append(ms, m)
		}
		return ms, nil
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
				if needStop(pup.stopCh) {
					return
				}
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(k))
				m := shardMaps[h%uint64(len(shardMaps))]
				n += updatePipeUniqMap(m, k, pHits)
				if n > stateSizeBudgetChunk {
					if nRemaining := pup.stateSizeBudget.Add(-n); nRemaining < 0 {
						return
					}
					nTotal += n
					n = 0
				}
			}
			nTotal += n
			pup.stateSizeBudget.Add(-n)

			perShardMaps[idx] = shardMaps

			// Clean the original map and return its state size budget back.
			shards[idx].m = nil
			pup.stateSizeBudget.Add(nTotal)
		}(i)
	}
	wg.Wait()
	if needStop(pup.stopCh) {
		return nil, nil
	}
	if n := pup.stateSizeBudget.Load(); n < 0 {
		return nil, fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", pup.pu.String(), pup.maxStateSize/(1<<20))
	}

	// Merge per-shard entries into perShardMaps[0]
	for i := range perShardMaps {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			m := perShardMaps[0][idx]
			for i := 1; i < len(perShardMaps); i++ {
				n := int64(0)
				nTotal := int64(0)
				for k, psg := range perShardMaps[i][idx] {
					if needStop(pup.stopCh) {
						return
					}
					n += updatePipeUniqMap(m, k, psg)
					if n > stateSizeBudgetChunk {
						if nRemaining := pup.stateSizeBudget.Add(-n); nRemaining < 0 {
							return
						}
						nTotal += n
						n = 0
					}
				}
				nTotal += n
				pup.stateSizeBudget.Add(-n)

				// Clean the original map and return its state size budget back.
				perShardMaps[i][idx] = nil
				pup.stateSizeBudget.Add(nTotal)
			}
		}(i)
	}
	wg.Wait()
	if needStop(pup.stopCh) {
		return nil, nil
	}
	if n := pup.stateSizeBudget.Load(); n < 0 {
		return nil, fmt.Errorf("cannot calculate [%s], since it requires more than %dMB of memory", pup.pu.String(), pup.maxStateSize/(1<<20))
	}

	// Filter out maps without entries
	ms := perShardMaps[0]
	result := ms[:0]
	for _, m := range ms {
		if len(m) > 0 {
			result = append(result, m)
		}
	}

	return result, nil
}

func updatePipeUniqMap(m map[string]*uint64, k string, pHitsSrc *uint64) int64 {
	pHitsDst := m[k]
	if pHitsDst != nil {
		*pHitsDst += *pHitsSrc
		return 0
	}

	m[k] = pHitsSrc
	return int64(unsafe.Sizeof(k) + unsafe.Sizeof(pHitsSrc))
}

type pipeUniqWriteContext struct {
	workerID uint
	pup      *pipeUniqProcessor
	rcs      []resultColumn
	br       blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
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
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeUniqWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.pup.ppNext.writeBlock(wctx.workerID, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

func parsePipeUniq(lex *lexer) (*pipeUniq, error) {
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
