package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeUniqLocal processes local part of the pipeUniq in cluster if hits are requested with unique values.
type pipeUniqLocal struct {
	// It is expected that pu.hitsFieldName != ""
	pu *pipeUniq
}

func (pu *pipeUniqLocal) String() string {
	s := "uniq_local"
	if len(pu.pu.byFields) > 0 {
		s += " by (" + fieldNamesString(pu.pu.byFields) + ")"
	}
	s += fmt.Sprintf(" limit %d", pu.pu.limit)
	return s
}

func (pu *pipeUniqLocal) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	logger.Panicf("BUG: unexpected call for %T", pu)
	return nil, nil
}

func (pu *pipeUniqLocal) canLiveTail() bool {
	return false
}

func (pu *pipeUniqLocal) canReturnLastNResults() bool {
	return false
}

func (pu *pipeUniqLocal) updateNeededFields(pf *prefixfilter.Filter) {
	pf.Reset()

	if len(pu.pu.byFields) == 0 {
		pf.AddAllowFilter("*")
	} else {
		pf.AddAllowFilters(pu.pu.byFields)
		pf.AddAllowFilter(pu.pu.hitsFieldName)
	}
}

func (pu *pipeUniqLocal) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUniqLocal) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pu, nil
}

func (pu *pipeUniqLocal) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pu *pipeUniqLocal) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUniqLocalProcessor{
		pu:     pu,
		ppNext: ppNext,
	}
}

type pipeUniqLocalProcessor struct {
	pu     *pipeUniqLocal
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeUniqLocalProcessorShard]
}

type pipeUniqLocalProcessorShard struct {
	vhs []ValueWithHits
}

func (pup *pipeUniqLocalProcessor) writeBlock(workerID uint, br *blockResult) {
	shard := pup.shards.Get(workerID)

	pu := pup.pu.pu
	if pu.hitsFieldName == "" {
		logger.Panicf("BUG: expecting non-empty hitsFieldName; pu=%#v", pu)
	}

	columnValuess := getColumnValuess(br, pu.byFields)
	cHits := br.getColumnByName(pu.hitsFieldName)
	hits := cHits.getValues(br)

	var buf []byte
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		buf = buf[:0]
		for _, columnValues := range columnValuess {
			v := columnValues[rowIdx]
			buf = encoding.MarshalBytes(buf, bytesutil.ToUnsafeBytes(v))
		}
		value := string(buf)
		hits64, ok := tryParseUint64(hits[rowIdx])
		if !ok {
			logger.Panicf("BUG: unexpected hits received from the remote storage at the column %q: %q; it must be uint64", pu.hitsFieldName, hits[rowIdx])
		}
		shard.vhs = append(shard.vhs, ValueWithHits{
			Value: value,
			Hits:  hits64,
		})
	}
}

func getColumnValuess(br *blockResult, fields []string) [][]string {
	if len(fields) == 0 {
		cs := br.getColumns()
		columnValuess := make([][]string, len(cs))
		for i, c := range cs {
			columnValuess[i] = c.getValues(br)
		}
		return columnValuess
	}

	columnValuess := make([][]string, len(fields))
	for i, field := range fields {
		c := br.getColumnByName(field)
		columnValuess[i] = c.getValues(br)
	}
	return columnValuess
}

func (pup *pipeUniqLocalProcessor) flush() error {
	pu := pup.pu.pu
	shards := pup.shards.All()
	if len(shards) == 0 {
		return nil
	}

	a := make([][]ValueWithHits, len(shards))
	for i, shard := range shards {
		a[i] = shard.vhs
	}
	result := MergeValuesWithHits(a, pu.limit, true)

	// Write result.
	fields := append([]string{}, pu.byFields...)
	fields = append(fields, pu.hitsFieldName)
	wctx := newPipeFixedFieldsWriteContext(pup.ppNext, fields)

	rowValues := make([]string, len(pu.byFields)+1)
	for i := range result {
		src := bytesutil.ToUnsafeBytes(result[i].Value)
		for i := 0; i < len(rowValues)-1; i++ {
			v, n := encoding.UnmarshalBytes(src)
			if n <= 0 {
				logger.Panicf("BUG: cannot unmarshal field value")
			}
			src = src[n:]

			rowValues[i] = bytesutil.ToUnsafeString(v)
		}
		if len(src) > 0 {
			logger.Panicf("BUG: unexpected tail left after unmarshaling fields; len(tail)=%d", len(src))
		}

		rowValues[len(pu.byFields)] = string(marshalUint64String(nil, result[i].Hits))
		wctx.writeRow(rowValues)
	}
	wctx.flush()

	return nil
}

type pipeFixedFieldsWriteContext struct {
	ppNext pipeProcessor
	rcs    []resultColumn
	br     blockResult

	// rowsCount is the number of rows in the current block
	rowsCount int

	// valuesLen is the total length of values in the current block
	valuesLen int
}

func newPipeFixedFieldsWriteContext(ppNext pipeProcessor, fields []string) *pipeFixedFieldsWriteContext {
	rcs := make([]resultColumn, len(fields))
	for i, field := range fields {
		rcs[i].name = field
	}
	return &pipeFixedFieldsWriteContext{
		ppNext: ppNext,
		rcs:    rcs,
	}
}

func (wctx *pipeFixedFieldsWriteContext) writeRow(rowValues []string) {
	rcs := wctx.rcs

	for i, v := range rowValues {
		rcs[i].addValue(v)
		wctx.valuesLen += len(v)
	}

	wctx.rowsCount++

	// The 64_000 limit provides the best performance results.
	if wctx.valuesLen >= 64_000 {
		wctx.flush()
	}
}

func (wctx *pipeFixedFieldsWriteContext) flush() {
	if wctx.rowsCount == 0 {
		return
	}

	// Flush rcs to ppNext
	wctx.br.setResultColumns(wctx.rcs, wctx.rowsCount)
	wctx.valuesLen = 0
	wctx.rowsCount = 0
	wctx.ppNext.writeBlock(0, &wctx.br)
	wctx.br.reset()
	for i := range wctx.rcs {
		wctx.rcs[i].resetValues()
	}
}
