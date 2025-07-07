package logstorage

import (
	"fmt"
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeHash processes '| hash ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#hash-pipe
type pipeHash struct {
	fieldName   string
	resultField string
}

func (ph *pipeHash) String() string {
	s := "hash(" + quoteTokenIfNeeded(ph.fieldName) + ")"
	if !isMsgFieldName(ph.resultField) {
		s += " as " + quoteTokenIfNeeded(ph.resultField)
	}
	return s
}

func (ph *pipeHash) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return ph, nil
}

func (ph *pipeHash) canLiveTail() bool {
	return true
}

func (ph *pipeHash) updateNeededFields(pf *prefixfilter.Filter) {
	if pf.MatchString(ph.resultField) {
		pf.AddDenyFilter(ph.resultField)
		pf.AddAllowFilter(ph.fieldName)
	}
}

func (ph *pipeHash) hasFilterInWithQuery() bool {
	return false
}

func (ph *pipeHash) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return ph, nil
}

func (ph *pipeHash) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (ph *pipeHash) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	php := &pipeHashProcessor{
		ph:     ph,
		ppNext: ppNext,
	}
	php.shards.Init = func(shard *pipeHashProcessorShard) {
		shard.reset()
	}
	return php
}

type pipeHashProcessor struct {
	ph     *pipeHash
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeHashProcessorShard]
}

type pipeHashProcessorShard struct {
	a  arena
	rc resultColumn

	minValue float64
	maxValue float64
}

func (php *pipeHashProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := php.shards.Get(workerID)
	shard.rc.name = php.ph.resultField

	c := br.getColumnByName(php.ph.fieldName)
	if c.isConst {
		// Fast path for const column
		v := c.valuesEncoded[0]
		f := getFloat64CompatibleHash(v)
		shard.a.b = marshalFloat64String(shard.a.b[:0], f)
		shard.rc.addValue(bytesutil.ToUnsafeString(shard.a.b))
		br.addResultColumnConst(shard.rc)
	} else {
		// Slow path for other columns
		values := c.getValues(br)
		vEncoded := ""
		for rowIdx := range values {
			if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
				vEncoded = shard.getEncodedHash(values[rowIdx])
			}
			shard.rc.addValue(vEncoded)
		}
		br.addResultColumnFloat64(shard.rc, shard.minValue, shard.maxValue)
	}

	// Write the result to ppNext
	php.ppNext.writeBlock(workerID, br)

	shard.reset()
}

func (shard *pipeHashProcessorShard) reset() {
	shard.a.reset()
	shard.rc.reset()
	shard.minValue = nan
	shard.maxValue = nan
}

func (shard *pipeHashProcessorShard) getEncodedHash(v string) string {
	f := getFloat64CompatibleHash(v)

	if math.IsNaN(shard.minValue) {
		shard.minValue = f
		shard.maxValue = f
	} else if f < shard.minValue {
		shard.minValue = f
	} else if f > shard.maxValue {
		shard.maxValue = f
	}

	bLen := len(shard.a.b)
	shard.a.b = marshalFloat64(shard.a.b, f)
	return bytesutil.ToUnsafeString(shard.a.b[bLen:])
}

func getFloat64CompatibleHash(v string) float64 {
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(v))
	h &= ((1 << 53) - 1)
	return float64(h)
}

func (php *pipeHashProcessor) flush() error {
	return nil
}

func parsePipeHash(lex *lexer) (pipe, error) {
	if !lex.isKeyword("hash") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "hash")
	}
	lex.nextToken()

	fieldName, err := parseFieldNameWithOptionalParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse field name for 'hash' pipe: %w", err)
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
	}
	if !lex.isKeyword("|", ")", "") {
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field after 'hash(%s)': %w", quoteTokenIfNeeded(fieldName), err)
		}
		resultField = field
	}

	ph := &pipeHash{
		fieldName:   fieldName,
		resultField: resultField,
	}

	return ph, nil
}
