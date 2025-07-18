package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeUnpackWords processes '| unpack_words ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_words-pipe
type pipeUnpackWords struct {
	// field to unpack words from
	srcField string

	// field to put the unpack words to
	dstField string

	// whether to drop duplicate words
	dropDuplicates bool
}

func (pu *pipeUnpackWords) String() string {
	s := "unpack_words"
	if pu.srcField != "_msg" {
		s += " from " + quoteTokenIfNeeded(pu.srcField)
	}
	if pu.dstField != pu.srcField {
		s += " as " + quoteTokenIfNeeded(pu.dstField)
	}
	if pu.dropDuplicates {
		s += " drop_duplicates"
	}
	return s
}

func (pu *pipeUnpackWords) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pu, nil
}

func (pu *pipeUnpackWords) canLiveTail() bool {
	return true
}

func (pu *pipeUnpackWords) hasFilterInWithQuery() bool {
	return false
}

func (pu *pipeUnpackWords) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pu, nil
}

func (pu *pipeUnpackWords) visitSubqueries(_ func(q *Query)) {
	// do nothing
}

func (pu *pipeUnpackWords) updateNeededFields(pf *prefixfilter.Filter) {
	if pf.MatchString(pu.dstField) {
		pf.AddDenyFilter(pu.dstField)
		pf.AddAllowFilter(pu.srcField)
	}
}

func (pu *pipeUnpackWords) newPipeProcessor(_ int, stopCh <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeUnpackWordsProcessor{
		pu:     pu,
		stopCh: stopCh,
		ppNext: ppNext,
	}
}

type pipeUnpackWordsProcessor struct {
	pu     *pipeUnpackWords
	stopCh <-chan struct{}
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeUnpackWordsProcessorShard]
}

type pipeUnpackWordsProcessorShard struct {
	wctx pipeUnpackWriteContext
	a    arena

	fields [1]Field
	words  []string
}

func (pup *pipeUnpackWordsProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	pu := pup.pu
	shard := pup.shards.Get(workerID)
	shard.wctx.init(workerID, pup.ppNext, false, false, br)

	c := br.getColumnByName(pu.srcField)
	values := c.getValues(br)

	t := getTokenizer()
	keepDuplicateTokens := !pu.dropDuplicates
	for rowIdx := range values {
		if needStop(pup.stopCh) {
			return
		}

		if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
			t.reset()
			shard.words = t.tokenizeString(shard.words[:0], values[rowIdx], keepDuplicateTokens)
			bufLen := len(shard.a.b)
			shard.a.b = marshalJSONArray(shard.a.b, shard.words)
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

func (pup *pipeUnpackWordsProcessor) flush() error {
	return nil
}

func parsePipeUnpackWords(lex *lexer) (pipe, error) {
	if !lex.isKeyword("unpack_words") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "unpack_words")
	}
	lex.nextToken()

	srcField := "_msg"
	if !lex.isKeyword("drop_duplicates", "as", ")", "|", "") {
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
	if !lex.isKeyword("drop_duplicates", ")", "|", "") {
		if lex.isKeyword("as") {
			lex.nextToken()
		}
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse dstField name: %w", err)
		}
		dstField = field
	}

	dropDuplicates := false
	if lex.isKeyword("drop_duplicates") {
		lex.nextToken()
		dropDuplicates = true
	}

	pu := &pipeUnpackWords{
		srcField: srcField,
		dstField: dstField,

		dropDuplicates: dropDuplicates,
	}

	return pu, nil
}
