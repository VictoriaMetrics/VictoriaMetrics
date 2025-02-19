package logstorage

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// pipeLen processes '| len ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#len-pipe
type pipeLen struct {
	fieldName   string
	resultField string
}

func (pl *pipeLen) String() string {
	s := "len(" + quoteTokenIfNeeded(pl.fieldName) + ")"
	if !isMsgFieldName(pl.resultField) {
		s += " as " + quoteTokenIfNeeded(pl.resultField)
	}
	return s
}

func (pl *pipeLen) canLiveTail() bool {
	return true
}

func (pl *pipeLen) updateNeededFields(neededFields, unneededFields fieldsSet) {
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

func (pl *pipeLen) hasFilterInWithQuery() bool {
	return false
}

func (pl *pipeLen) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pl, nil
}

func (pl *pipeLen) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pl *pipeLen) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	shards := make([]pipeLenProcessorShard, workersCount)
	for i := range shards {
		shards[i].reset()
	}

	return &pipeLenProcessor{
		pl:     pl,
		ppNext: ppNext,

		shards: shards,
	}
}

type pipeLenProcessor struct {
	pl     *pipeLen
	ppNext pipeProcessor

	shards []pipeLenProcessorShard
}

type pipeLenProcessorShard struct {
	pipeLenProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeLenProcessorShardNopad{})%128]byte
}

type pipeLenProcessorShardNopad struct {
	a  arena
	rc resultColumn

	minValue float64
	maxValue float64
}

func (plp *pipeLenProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &plp.shards[workerID]
	shard.rc.name = plp.pl.resultField

	c := br.getColumnByName(plp.pl.fieldName)
	if c.isConst {
		// Fast path for const column
		v := c.valuesEncoded[0]
		shard.a.b = marshalUint64String(shard.a.b[:0], uint64(len(v)))
		shard.rc.addValue(bytesutil.ToUnsafeString(shard.a.b))
		br.addResultColumnConst(&shard.rc)
	} else {
		// Slow path for other columns
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			v := c.getValueAtRow(br, rowIdx)
			vEncoded := shard.getEncodedLen(v)
			shard.rc.addValue(vEncoded)
		}
		br.addResultColumnFloat64(&shard.rc, shard.minValue, shard.maxValue)
	}

	// Write the result to ppNext
	plp.ppNext.writeBlock(workerID, br)

	shard.reset()
}

func (shard *pipeLenProcessorShard) reset() {
	shard.a.reset()
	shard.rc.reset()
	shard.minValue = nan
	shard.maxValue = nan
}

func (shard *pipeLenProcessorShard) getEncodedLen(v string) string {
	f := float64(len(v))
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

func (plp *pipeLenProcessor) flush() error {
	return nil
}

func parsePipeLen(lex *lexer) (pipe, error) {
	if !lex.isKeyword("len") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "len")
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

	pl := &pipeLen{
		fieldName:   fieldName,
		resultField: resultField,
	}

	return pl, nil
}

func parseFieldNameWithOptionalParens(lex *lexer) (string, error) {
	hasParens := false
	if lex.isKeyword("(") {
		lex.nextToken()
		hasParens = true
	}
	fieldName, err := parseFieldName(lex)
	if err != nil {
		return "", err
	}
	if hasParens {
		if !lex.isKeyword(")") {
			return "", fmt.Errorf("missing ')' after '%s'", quoteTokenIfNeeded(fieldName))
		}
		lex.nextToken()
	}
	return fieldName, nil
}
