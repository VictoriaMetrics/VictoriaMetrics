package logstorage

import (
	"fmt"
	"unsafe"
)

// pipeUnrollTokens processes '| unroll_tokens ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unroll_tokens-pipe
type pipeUnrollTokens struct {
	// field to unroll tokens from
	srcField string

	// field to put the unrolled tokens to
	dstField string
}

func (pu *pipeUnrollTokens) String() string {
	s := "unroll_tokens"
	if pu.srcField != "_msg" {
		s += " " + quoteTokenIfNeeded(pu.srcField)
	}
	if pu.dstField != pu.srcField {
		s += " as " + quoteTokenIfNeeded(pu.dstField)
	}
	return s
}

func (pu *pipeUnrollTokens) canLiveTail() bool {
	return true
}

func (pu *pipeUnrollTokens) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUnrollTokens) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
	return pu, nil
}

func (pu *pipeUnrollTokens) visitSubqueries(_ func(q *Query)) {
	// do nothing
}

func (pu *pipeUnrollTokens) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.contains("*") {
		unneededFields.add(pu.dstField)
		unneededFields.remove(pu.srcField)
	} else {
		neededFields.remove(pu.dstField)
		neededFields.add(pu.srcField)
	}
}

func (pu *pipeUnrollTokens) newPipeProcessor(workersCount int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUnrollTokensProcessor{
		pu:     pu,
		stopCh: stopCh,
		ppNext: ppNext,

		shards: make([]pipeUnrollTokensProcessorShard, workersCount),
	}
}

type pipeUnrollTokensProcessor struct {
	pu     *pipeUnrollTokens
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards []pipeUnrollTokensProcessorShard
}

type pipeUnrollTokensProcessorShard struct {
	pipeUnrollTokensProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeUnrollTokensProcessorShardNopad{})%128]byte
}

type pipeUnrollTokensProcessorShardNopad struct {
	wctx pipeUnpackWriteContext
	a    arena

	fields [1]Field
	tokens []string
}

func (pup *pipeUnrollTokensProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	pu := pup.pu
	shard := &pup.shards[workerID]
	shard.wctx.init(workerID, pup.ppNext, false, false, br)

	c := br.getColumnByName(pu.srcField)
	values := c.getValues(br)

	t := getTokenizer()
	for rowIdx, v := range values {
		if needStop(pup.stopCh) {
			return
		}

		t.reset()
		shard.tokens = t.tokenizeString(shard.tokens[:0], v, true)
		for _, token := range shard.tokens {
			shard.fields[0] = Field{
				Name:  pu.dstField,
				Value: shard.a.copyString(token),
			}
			shard.wctx.writeRow(rowIdx, shard.fields[:])
		}
	}
	putTokenizer(t)

	shard.wctx.flush()
	shard.wctx.reset()
	shard.a.reset()
}

func (pup *pipeUnrollTokensProcessor) flush() error {
	return nil
}

func parsePipeUnrollTokens(lex *lexer) (pipe, error) {
	if !lex.isKeyword("unroll_tokens") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unroll_tokens")
	}
	lex.nextToken()

	srcField := "_msg"
	if !lex.isKeyword("as", ")", "|", "") {
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

	pu := &pipeUnrollTokens{
		srcField: srcField,
		dstField: dstField,
	}

	return pu, nil
}
