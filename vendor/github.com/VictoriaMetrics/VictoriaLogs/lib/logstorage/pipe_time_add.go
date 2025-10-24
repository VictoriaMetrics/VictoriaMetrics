package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeTimeAdd processes '| time_add ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#time_add-pipe
type pipeTimeAdd struct {
	field string

	offset    int64
	offsetStr string
}

func (pa *pipeTimeAdd) String() string {
	s := "time_add " + pa.offsetStr
	if pa.field != "_time" {
		s += " at " + quoteTokenIfNeeded(pa.field)
	}
	return s
}

func (pa *pipeTimeAdd) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pa, nil
}

func (pa *pipeTimeAdd) canLiveTail() bool {
	return true
}

func (pa *pipeTimeAdd) canReturnLastNResults() bool {
	return true
}

func (pa *pipeTimeAdd) updateNeededFields(_ *prefixfilter.Filter) {
	// do nothing
}

func (pa *pipeTimeAdd) hasFilterInWithQuery() bool {
	return false
}

func (pa *pipeTimeAdd) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pa, nil
}

func (pa *pipeTimeAdd) visitSubqueries(_ func(q *Query)) {
	// do nothing
}

func (pa *pipeTimeAdd) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeTimeAddProcessor{
		pa:     pa,
		ppNext: ppNext,
	}
}

type pipeTimeAddProcessor struct {
	pa     *pipeTimeAdd
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeTimeAddProcessorShard]
}

type pipeTimeAddProcessorShard struct {
	rc  resultColumn
	buf []byte
}

func (pap *pipeTimeAddProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pap.shards.Get(workerID)
	pa := pap.pa

	shard.rc.name = pa.field

	c := br.getColumnByName(pa.field)
	for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
		v := c.getValueAtRow(br, rowIdx)
		ts, ok := TryParseTimestampRFC3339Nano(v)
		if ok {
			ts = subNoOverflowInt64(ts, pa.offset)
			bufLen := len(shard.buf)
			shard.buf = marshalTimestampRFC3339NanoString(shard.buf, ts)
			v = bytesutil.ToUnsafeString(shard.buf[bufLen:])
		}
		shard.rc.addValue(v)
	}

	br.addResultColumn(shard.rc)
	pap.ppNext.writeBlock(workerID, br)

	shard.rc.reset()
	shard.buf = shard.buf[:0]
}

func (pap *pipeTimeAddProcessor) flush() error {
	return nil
}

func parsePipeTimeAdd(lex *lexer) (pipe, error) {
	if !lex.isKeyword("time_add") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "time_add")
	}
	lex.nextToken()

	offset, offsetStr, err := parseDuration(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse offset: %w", err)
	}

	// Parse optional field
	field := "_time"
	if lex.isKeyword("at") {
		lex.nextToken()
		fieldName, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot read field name: %w", err)
		}
		field = fieldName
	}

	pa := &pipeTimeAdd{
		field:     field,
		offset:    -offset,
		offsetStr: offsetStr,
	}

	return pa, nil
}
