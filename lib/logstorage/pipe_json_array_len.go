package logstorage

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// pipeJSONArrayLen processes '| json_array_len ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#json_array_len-pipe
type pipeJSONArrayLen struct {
	fieldName   string
	resultField string
}

func (pl *pipeJSONArrayLen) String() string {
	s := "json_array_len(" + quoteTokenIfNeeded(pl.fieldName) + ")"
	if !isMsgFieldName(pl.resultField) {
		s += " as " + quoteTokenIfNeeded(pl.resultField)
	}
	return s
}

func (pl *pipeJSONArrayLen) canLiveTail() bool {
	return true
}

func (pl *pipeJSONArrayLen) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(pl.resultField) {
			unneededFields.add(pl.resultField)
			unneededFields.remove(pl.fieldName)
		}
	} else {
		if neededFields.contains(pl.resultField) {
			neededFields.remove(pl.resultField)
			neededFields.add(pl.fieldName)
		}
	}
}

func (pl *pipeJSONArrayLen) hasFilterInWithQuery() bool {
	return false
}

func (pl *pipeJSONArrayLen) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pl, nil
}

func (pl *pipeJSONArrayLen) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pl *pipeJSONArrayLen) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	shards := make([]pipeJSONArrayLenProcessorShard, workersCount)
	for i := range shards {
		shards[i].reset()
	}

	return &pipeJSONArrayLenProcessor{
		pl:     pl,
		ppNext: ppNext,

		shards: shards,
	}
}

type pipeJSONArrayLenProcessor struct {
	pl     *pipeJSONArrayLen
	ppNext pipeProcessor

	shards []pipeJSONArrayLenProcessorShard
}

type pipeJSONArrayLenProcessorShard struct {
	pipeJSONArrayLenProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeJSONArrayLenProcessorShardNopad{})%128]byte
}

type pipeJSONArrayLenProcessorShardNopad struct {
	a  arena
	rc resultColumn

	tmpValues  []string
	tmpValuesA arena

	minValue float64
	maxValue float64
}

func (plp *pipeJSONArrayLenProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &plp.shards[workerID]
	shard.rc.name = plp.pl.resultField

	c := br.getColumnByName(plp.pl.fieldName)
	if c.isConst {
		// Fast path for const column
		v := c.valuesEncoded[0]
		aLen := shard.getJSONArrayLen(v)
		bLen := len(shard.a.b)
		shard.a.b = marshalUint64String(shard.a.b, uint64(aLen))
		vEncoded := bytesutil.ToUnsafeString(shard.a.b[bLen:])
		shard.rc.addValue(vEncoded)
		br.addResultColumnConst(&shard.rc)
	} else {
		// Slow path for other columns
		values := c.getValues(br)
		vEncoded := ""
		for rowIdx := range values {
			if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
				vEncoded = shard.getEncodedLen(values[rowIdx])
			}
			shard.rc.addValue(vEncoded)
		}
		br.addResultColumnFloat64(&shard.rc, shard.minValue, shard.maxValue)
	}

	// Write the result to ppNext
	plp.ppNext.writeBlock(workerID, br)

	shard.reset()
}

func (shard *pipeJSONArrayLenProcessorShard) reset() {
	shard.a.reset()
	shard.rc.reset()

	shard.tmpValuesA.reset()
	shard.tmpValues = shard.tmpValues[:0]

	shard.minValue = nan
	shard.maxValue = nan
}

func (shard *pipeJSONArrayLenProcessorShard) getEncodedLen(v string) string {
	aLen := shard.getJSONArrayLen(v)
	f := float64(aLen)

	if math.IsNaN(shard.minValue) {
		shard.minValue = f
		shard.maxValue = f
	} else if f < shard.minValue {
		shard.minValue = f
	} else if f > shard.maxValue {
		shard.maxValue = f
	}

	n := math.Float64bits(f)
	bLen := len(shard.a.b)
	shard.a.b = encoding.MarshalUint64(shard.a.b, n)
	return bytesutil.ToUnsafeString(shard.a.b[bLen:])
}

func (shard *pipeJSONArrayLenProcessorShard) getJSONArrayLen(v string) int {
	shard.tmpValuesA.reset()
	shard.tmpValues = unpackJSONArray(shard.tmpValues[:0], &shard.tmpValuesA, v)
	return len(shard.tmpValues)
}

func (plp *pipeJSONArrayLenProcessor) flush() error {
	return nil
}

func parsePipeJSONArrayLen(lex *lexer) (pipe, error) {
	if !lex.isKeyword("json_array_len") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "json_array_len")
	}
	lex.nextToken()

	fieldName, err := parseFieldNameWithOptionalParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse field name for 'len' pipe: %w", err)
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
	}
	if !lex.isKeyword("|", ")", "") {
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field after 'len(%s)': %w", quoteTokenIfNeeded(fieldName), err)
		}
		resultField = field
	}

	pl := &pipeJSONArrayLen{
		fieldName:   fieldName,
		resultField: resultField,
	}

	return pl, nil
}
