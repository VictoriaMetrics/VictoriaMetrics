package logstorage

import (
	"fmt"
	"slices"
	"sync/atomic"
	"unsafe"

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

	limit uint64
}

func (pu *pipeUniq) String() string {
	s := "uniq"
	if len(pu.byFields) > 0 {
		s += " by (" + fieldNamesString(pu.byFields) + ")"
	}
	if pu.limit > 0 {
		s += fmt.Sprintf(" limit %d", pu.limit)
	}
	return s
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

func (pu *pipeUniq) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppBase pipeProcessor) pipeProcessor {
	maxStateSize := int64(float64(memory.Allowed()) * 0.2)

	shards := make([]pipeUniqProcessorShard, workersCount)
	for i := range shards {
		shards[i] = pipeUniqProcessorShard{
			pipeUniqProcessorShardNopad: pipeUniqProcessorShardNopad{
				pu:              pu,
				m:               make(map[string]struct{}),
				stateSizeBudget: stateSizeBudgetChunk,
			},
		}
		maxStateSize -= stateSizeBudgetChunk
	}

	pup := &pipeUniqProcessor{
		pu:     pu,
		stopCh: stopCh,
		cancel: cancel,
		ppBase: ppBase,

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
	ppBase pipeProcessor

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

	// m holds unique rows.
	m map[string]struct{}

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
	if limit := shard.pu.limit; limit > 0 && uint64(len(shard.m)) >= limit {
		return false
	}

	m := shard.m
	byFields := shard.pu.byFields
	if len(byFields) == 0 {
		// Take into account all the columns in br.
		keyBuf := shard.keyBuf
		cs := br.getColumns()
		for i := range br.timestamps {
			keyBuf = keyBuf[:0]
			for _, c := range cs {
				v := c.getValueAtRow(br, i)
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(c.name))
				keyBuf = encoding.MarshalBytes(keyBuf, bytesutil.ToUnsafeBytes(v))
			}
			if _, ok := m[string(keyBuf)]; !ok {
				m[string(keyBuf)] = struct{}{}
				shard.stateSizeBudget -= len(keyBuf) + int(unsafe.Sizeof(""))
			}
		}
		shard.keyBuf = keyBuf
		return true
	}

	// Take into account only the selected columns.
	columnValues := shard.columnValues[:0]
	for _, f := range byFields {
		c := br.getColumnByName(f)
		columnValues = append(columnValues, c.getValues(br))
	}
	shard.columnValues = columnValues

	keyBuf := shard.keyBuf
	for i := range br.timestamps {
		seenValue := true
		for _, values := range columnValues {
			if i == 0 || values[i-1] != values[i] {
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
		if _, ok := m[string(keyBuf)]; !ok {
			m[string(keyBuf)] = struct{}{}
			shard.stateSizeBudget -= len(keyBuf) + int(unsafe.Sizeof(""))
		}
	}
	shard.keyBuf = keyBuf

	return true
}

func (pup *pipeUniqProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
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

	// merge state across shards
	shards := pup.shards
	m := shards[0].m
	shards = shards[1:]
	for i := range shards {
		if needStop(pup.stopCh) {
			return nil
		}

		for k := range shards[i].m {
			m[k] = struct{}{}
		}
	}

	// write result
	wctx := &pipeUniqWriteContext{
		pup: pup,
	}
	byFields := pup.pu.byFields
	var rowFields []Field

	if len(byFields) == 0 {
		for k := range m {
			if needStop(pup.stopCh) {
				return nil
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
			wctx.writeRow(rowFields)
		}
	} else {
		for k := range m {
			if needStop(pup.stopCh) {
				return nil
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
			wctx.writeRow(rowFields)
		}
	}

	wctx.flush()

	return nil
}

type pipeUniqWriteContext struct {
	pup *pipeUniqProcessor
	rcs []resultColumn
	br  blockResult

	rowsWritten uint64

	valuesLen int
}

func (wctx *pipeUniqWriteContext) writeRow(rowFields []Field) {
	if limit := wctx.pup.pu.limit; limit > 0 && wctx.rowsWritten >= limit {
		return
	}
	wctx.rowsWritten++

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
		// send the current block to bbBase and construct a block with new set of columns
		wctx.flush()

		rcs = wctx.rcs[:0]
		for _, f := range rowFields {
			rcs = append(rcs, resultColumn{
				name: f.Name,
			})
		}
		wctx.rcs = rcs
	}

	for i, f := range rowFields {
		v := f.Value
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeUniqWriteContext) flush() {
	rcs := wctx.rcs
	br := &wctx.br

	wctx.valuesLen = 0

	if len(rcs) == 0 {
		return
	}

	// Flush rcs to ppBase
	br.setResultColumns(rcs)
	wctx.pup.ppBase.writeBlock(0, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetKeepName()
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
