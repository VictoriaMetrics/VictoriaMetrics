package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeSplit processes '| split ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#split-pipe
type pipeSplit struct {
	// separator is the separator for splitting the input field
	separator string

	// field to split
	srcField string

	// field to put the split result
	dstField string
}

func (ps *pipeSplit) String() string {
	s := "split " + quoteTokenIfNeeded(ps.separator)
	if ps.srcField != "_msg" {
		s += " from " + quoteTokenIfNeeded(ps.srcField)
	}
	if ps.dstField != ps.srcField {
		s += " as " + quoteTokenIfNeeded(ps.dstField)
	}
	return s
}

func (ps *pipeSplit) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return ps, nil
}

func (ps *pipeSplit) canLiveTail() bool {
	return true
}

func (ps *pipeSplit) canReturnLastNResults() bool {
	return ps.dstField != "_time"
}

func (ps *pipeSplit) hasFilterInWithQuery() bool {
	return false
}

func (ps *pipeSplit) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return ps, nil
}

func (ps *pipeSplit) visitSubqueries(_ func(q *Query)) {
	// do nothing
}

func (ps *pipeSplit) updateNeededFields(pf *prefixfilter.Filter) {
	if pf.MatchString(ps.dstField) {
		pf.AddDenyFilter(ps.dstField)
		pf.AddAllowFilter(ps.srcField)
	}
}

func (ps *pipeSplit) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return &pipeSplitProcessor{
		ps:     ps,
		ppNext: ppNext,
	}
}

type pipeSplitProcessor struct {
	ps     *pipeSplit
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeSplitProcessorShard]
}

type pipeSplitProcessorShard struct {
	wctx pipeUnpackWriteContext
	a    arena

	fields [1]Field
	words  []string
}

func (psp *pipeSplitProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	ps := psp.ps
	shard := psp.shards.Get(workerID)
	shard.wctx.init(workerID, psp.ppNext, false, false, br)

	c := br.getColumnByName(ps.srcField)
	values := c.getValues(br)

	for rowIdx := range values {
		if rowIdx == 0 || values[rowIdx] != values[rowIdx-1] {
			shard.words = splitString(shard.words[:0], values[rowIdx], ps.separator)
			bufLen := len(shard.a.b)
			shard.a.b = marshalJSONArray(shard.a.b, shard.words)
			shard.fields[0] = Field{
				Name:  ps.dstField,
				Value: bytesutil.ToUnsafeString(shard.a.b[bufLen:]),
			}
		}
		shard.wctx.writeRow(rowIdx, shard.fields[:])
	}

	shard.wctx.flush()
	shard.wctx.reset()
	shard.a.reset()
}

func (psp *pipeSplitProcessor) flush() error {
	return nil
}

func parsePipeSplit(lex *lexer) (pipe, error) {
	if !lex.isKeyword("split") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "split")
	}
	lex.nextToken()

	if lex.isKeyword("as", "from") {
		return nil, fmt.Errorf("missing split separator in front of %q", lex.token)
	}

	separator, err := lex.nextCompoundToken()
	if err != nil {
		return nil, fmt.Errorf("cannot read split separator: %w", err)
	}

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

	ps := &pipeSplit{
		separator: separator,
		srcField:  srcField,
		dstField:  dstField,
	}

	return ps, nil
}

func splitString(dst []string, s, separator string) []string {
	if separator == "" {
		// special case for empty separator
		for _, r := range s {
			dst = append(dst, string(r))
		}
		return dst
	}

	for len(s) > 0 {
		n := strings.Index(s, separator)
		if n < 0 {
			return append(dst, s)
		}

		dst = append(dst, s[:n])
		s = s[n+len(separator):]
	}
	return append(dst, "")
}
