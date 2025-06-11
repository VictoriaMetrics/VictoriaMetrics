package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipeDecolorize processes '| decolorize ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#decolorize-pipe
type pipeDecolorize struct {
	field string
}

func (pd *pipeDecolorize) String() string {
	s := "decolorize"
	if pd.field != "_msg" {
		s += " " + quoteTokenIfNeeded(pd.field)
	}
	return s
}

func (pd *pipeDecolorize) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pd, nil
}

func (pd *pipeDecolorize) canLiveTail() bool {
	return true
}

func (pd *pipeDecolorize) updateNeededFields(_ *prefixfilter.Filter) {
	// nothing to do
}

func (pd *pipeDecolorize) hasFilterInWithQuery() bool {
	return false
}

func (pd *pipeDecolorize) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pd, nil
}

func (pd *pipeDecolorize) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pd *pipeDecolorize) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	updateFunc := func(a *arena, v string) string {
		bLen := len(a.b)
		a.b = dropColorSequences(a.b, v)
		return bytesutil.ToUnsafeString(a.b[bLen:])
	}

	return newPipeUpdateProcessor(updateFunc, ppNext, pd.field, nil)
}

func parsePipeDecolorize(lex *lexer) (pipe, error) {
	if !lex.isKeyword("decolorize") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "decolorize")
	}
	lex.nextToken()

	field := "_msg"
	if !lex.isKeyword("|", ")", "") {
		f, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse field name after 'decolorize': %w", err)
		}
		field = f
	}

	pd := &pipeDecolorize{
		field: field,
	}

	return pd, nil
}
