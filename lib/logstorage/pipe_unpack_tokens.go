package logstorage

import (
	"fmt"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pipeUnpackTokens processes '| unpack_tokens ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_tokens-pipe
type pipeUnpackTokens struct {
	// field to unpack tokens from
	srcField string

	// field to put the unpack tokens to
	dstField string
}

func (pu *pipeUnpackTokens) String() string {
	s := "unpack_tokens"
	if pu.srcField != "_msg" {
		s += " from " + quoteTokenIfNeeded(pu.srcField)
	}
	if pu.dstField != pu.srcField {
		s += " as " + quoteTokenIfNeeded(pu.dstField)
	}
	return s
}

func (pu *pipeUnpackTokens) canLiveTail() bool {
	return true
}

func (pu *pipeUnpackTokens) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUnpackTokens) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pu, nil
}

func (pu *pipeUnpackTokens) visitSubqueries(_ func(q *Query)) {
	// do nothing
}

func (pu *pipeUnpackTokens) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		if !unneededFields.contains(pu.dstField) {
			unneededFields.add(pu.dstField)
			unneededFields.remove(pu.srcField)
		}
	} else {
		if neededFields.contains(pu.dstField) {
			neededFields.remove(pu.dstField)
			neededFields.add(pu.srcField)
		}
	}
}

func (pu *pipeUnpackTokens) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUnpackTokensProcessor{
		pu:     pu,
		stopCh: stopCh,
		ppNext: ppNext,

		shards: make([]pipeUnpackTokensProcessorShard, workersCount),
	}
}

type pipeUnpackTokensProcessor struct {
	pu     *pipeUnpackTokens
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards []pipeUnpackTokensProcessorShard
}

type pipeUnpackTokensProcessorShard struct {
	pipeUnpackTokensProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnpackTokensProcessorShardNopad{})%128]byte
}

type pipeUnpackTokensProcessorShardNopad struct {
	wctx pipeUnpackWriteContext
	a    arena

	fields [1]Field
	tokens []string
}

func (pup *pipeUnpackTokensProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	pu := pup.pu
	shard := &pup.shards[workerID]
	shard.wctx.init(workerID, pup.ppNext, false, false, br)

	c := br.getColumnByName(pu.srcField)
	values := c.getValues(br)

	t := getTokenizer()
	for rowIdx := range values {
		if needStop(pup.stopCh) {
			return
		}

		if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
			t.reset()
			shard.tokens = t.tokenizeString(shard.tokens[:0], values[rowIdx], true)
			bufLen := len(shard.a.b)
			shard.a.b = marshalJSONArray(shard.a.b, shard.tokens)
			shard.fields[0] = Field{
				Name:  pu.dstField,
				Value: bytesutil.ToUnsafeString(shard.a.b[bufLen:]),
			}
		}
		shard.wctx.writeRow(rowIdx, shard.fields[:])
	}
	putTokenizer(t)

	shard.wctx.flush()
	shard.wctx.reset()
	shard.a.reset()
}

func (pup *pipeUnpackTokensProcessor) flush() error {
	return nil
}

func parsePipeUnpackTokens(lex *lexer) (pipe, error) {
	if !lex.isKeyword("unpack_tokens") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_tokens")
	}
	lex.nextToken()

	srcField := "_msg"
	if !lex.isKeyword("as", ")", "|", "") {
		if lex.isKeyword("from") {
			lex.nextToken()
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse srcField name: %w", err)
		}
		srcField = field
	}

	dstField := srcField
	if !lex.isKeyword(")", "|", "") {
		if lex.isKeyword("as") {
			lex.nextToken()
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse dstField name: %w", err)
		}
		dstField = field
	}

	pu := &pipeUnpackTokens{
		srcField: srcField,
		dstField: dstField,
	}

	return pu, nil
}
