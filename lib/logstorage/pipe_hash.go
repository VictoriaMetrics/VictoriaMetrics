package logstorage

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
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

func (ph *pipeHash) canLiveTail() bool {
	return true
}

func (ph *pipeHash) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(ph.resultField) {
			unneededFields.add(ph.resultField)
			unneededFields.remove(ph.fieldName)
		}
	} else {
		if neededFields.contains(ph.resultField) {
			neededFields.remove(ph.resultField)
			neededFields.add(ph.fieldName)
		}
	}
}

func (ph *pipeHash) hasFilterInWithQuery() bool {
	return false
}

func (ph *pipeHash) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return ph, nil
}

func (ph *pipeHash) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (ph *pipeHash) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	shards := make([]pipeHashProcessorShard, workersCount)
	for i := range shards {
		shards[i].reset()
	}

	return &pipeHashProcessor{
		ph:     ph,
		ppNext: ppNext,

		shards: shards,
	}
}

type pipeHashProcessor struct {
	ph     *pipeHash
	ppNext pipeProcessor

	shards []pipeHashProcessorShard
}

type pipeHashProcessorShard struct {
	pipeHashProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeHashProcessorShardNopad{})%128]byte
}

type pipeHashProcessorShardNopad struct {
	a  arena
	rc resultColumn

	minValue float64
	maxValue float64
}

func (php *pipeHashProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := &php.shards[workerID]
	shard.rc.name = php.ph.resultField

	c := br.getColumnByName(php.ph.fieldName)
	if c.isConst {
		// Fast path for const column
		v := c.valuesEncoded[0]
		f := getFloat64CompatibleHash(v)
		shard.a.b = marshalFloat64String(shard.a.b[:0], f)
		shard.rc.addValue(bytesutil.ToUnsafeString(shard.a.b))
		br.addResultColumnConst(&shard.rc)
	} else {
		// Slow path for other columns
		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			v := c.getValueAtRow(br, rowIdx)
			vEncoded := shard.getEncodedHash(v)
			shard.rc.addValue(vEncoded)
		}
		br.addResultColumnFloat64(&shard.rc, shard.minValue, shard.maxValue)
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
	n := math.Float64bits(f)
	bLen := len(shard.a.b)
	shard.a.b = encoding.MarshalUint64(shard.a.b, n)
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
