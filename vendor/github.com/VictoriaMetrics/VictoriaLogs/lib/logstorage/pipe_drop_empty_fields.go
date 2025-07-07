package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeDropEmptyFields processes '| drop_empty_fields ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#drop_empty_fields-pipe
type pipeDropEmptyFields struct {
}

func (pd *pipeDropEmptyFields) String() string {
	return "drop_empty_fields"
}

func (pd *pipeDropEmptyFields) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pd, nil
}

func (pd *pipeDropEmptyFields) canLiveTail() bool {
	return true
}

func (pd *pipeDropEmptyFields) hasFilterInWithQuery() bool {
	return false
}

func (pd *pipeDropEmptyFields) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pd, nil
}

func (pd *pipeDropEmptyFields) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pd *pipeDropEmptyFields) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (pd *pipeDropEmptyFields) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeDropEmptyFieldsProcessor{
		ppNext: ppNext,
	}
}

type pipeDropEmptyFieldsProcessor struct {
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeDropEmptyFieldsProcessorShard]
}

type pipeDropEmptyFieldsProcessorShard struct {
	columnValues [][]string
	fields       []Field

	wctx pipeDropEmptyFieldsWriteContext
}

func (pdp *pipeDropEmptyFieldsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pdp.shards.Get(workerID)

	cs := br.getColumns()

	shard.columnValues = slicesutil.SetLength(shard.columnValues, len(cs))
	columnValues := shard.columnValues
	for i, c := range cs {
		columnValues[i] = c.getValues(br)
	}

	if !hasEmptyValues(columnValues) {
		// Fast path - just write br to ppNext, since it has no empty values.
		pdp.ppNext.writeBlock(workerID, br)
		return
	}

	// Slow path - drop fields with empty values
	shard.wctx.init(workerID, pdp.ppNext)

	fields := shard.fields
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		fields = fields[:0]
		for i, values := range columnValues {
			v := values[rowIdx]
			if v == "" {
				continue
			}
			fields = append(fields, Field{
				Name:  cs[i].name,
				Value: values[rowIdx],
			})
		}
		shard.wctx.writeRow(fields)
	}
	shard.fields = fields

	shard.wctx.flush()
}

func (pdp *pipeDropEmptyFieldsProcessor) flush() error {
	return nil
}

type pipeDropEmptyFieldsWriteContext struct {
	workerID uint
	ppNext   pipeProcessor

	rcs []resultColumn
	br  blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func (wctx *pipeDropEmptyFieldsWriteContext) reset() {
	wctx.workerID = 0
	wctx.ppNext = nil

	rcs := wctx.rcs
	for i := range rcs {
		rcs[i].reset()
	}
	wctx.rcs = rcs[:0]

	wctx.rowsCount = 0
	wctx.valuesLen = 0
}

func (wctx *pipeDropEmptyFieldsWriteContext) init(workerID uint, ppNext pipeProcessor) {
	wctx.reset()

	wctx.workerID = workerID
	wctx.ppNext = ppNext
}

func (wctx *pipeDropEmptyFieldsWriteContext) writeRow(fields []Field) {
	if len(fields) == 0 {
		// skip rows without non-empty fields
		return
	}

	rcs := wctx.rcs

	areEqualColumns := len(rcs) == len(fields)
	if areEqualColumns {
		for i, f := range fields {
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
		for _, f := range fields {
			rcs = appendResultColumnWithName(rcs, f.Name)
		}
		wctx.rcs = rcs
	}

	for i, f := range fields {
		v := f.Value
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}

	wctx.rowsCount++
	if wctx.valuesLen >= 1_000_000 {
		wctx.flush()
	}
}

func (wctx *pipeDropEmptyFieldsWriteContext) flush() {
	rcs := wctx.rcs

	wctx.valuesLen = 0

	// Flush rcs to ppNext
	br := &wctx.br
	br.setResultColumns(rcs, wctx.rowsCount)
	wctx.rowsCount = 0
	wctx.ppNext.writeBlock(wctx.workerID, br)
	br.reset()
	for i := range rcs {
		rcs[i].resetValues()
	}
}

func parsePipeDropEmptyFields(lex *lexer) (pipe, error) {
	if !lex.isKeyword("drop_empty_fields") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "drop_empty_fields")
	}
	lex.nextToken()

	pd := &pipeDropEmptyFields{}

	return pd, nil
}

func hasEmptyValues(columnValues [][]string) bool {
	for _, values := range columnValues {
		for _, v := range values {
			if v == "" {
				return true
			}
		}
	}
	return false
}
