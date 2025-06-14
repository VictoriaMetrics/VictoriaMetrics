package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipePackLogfmt processes '| pack_logfmt ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#pack_logfmt-pipe
type pipePackLogfmt struct {
	resultField string

	// the fields and/or field name prefixes to put inside the packed json
	fieldFilters []string
}

func (pp *pipePackLogfmt) String() string {
	s := "pack_logfmt"
	if len(pp.fieldFilters) > 0 {
		s += " fields (" + fieldNamesString(pp.fieldFilters) + ")"
	}
	if !isMsgFieldName(pp.resultField) {
		s += " as " + quoteTokenIfNeeded(pp.resultField)
	}
	return s
}

func (pp *pipePackLogfmt) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pp, nil
}

func (pp *pipePackLogfmt) canLiveTail() bool {
	return true
}

func (pp *pipePackLogfmt) updateNeededFields(pf *prefixfilter.Filter) {
	updateNeededFieldsForPipePack(pf, pp.resultField, pp.fieldFilters)
}

func (pp *pipePackLogfmt) hasFilterInWithQuery() bool {
	return false
}

func (pp *pipePackLogfmt) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pp, nil
}

func (pp *pipePackLogfmt) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pp *pipePackLogfmt) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return newPipePackProcessor(ppNext, pp.resultField, pp.fieldFilters, MarshalFieldsToLogfmt)
}

func parsePipePackLogfmt(lex *lexer) (pipe, error) {
	if !lex.isKeyword("pack_logfmt") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "pack_logfmt")
	}
	lex.nextToken()

	var fieldFilters []string
	if lex.isKeyword("fields") {
		lex.nextToken()
		fs, err := parseFieldFiltersInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse fields: %w", err)
		}
		if slices.Contains(fs, "*") {
			fs = nil
		}
		fieldFilters = fs
	}

	// parse optional 'as ...` part
	resultField := "_msg"
	if lex.isKeyword("as") {
		lex.nextToken()
	}
	if !lex.isKeyword("|", ")", "") {
		field, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result field for 'pack_logfmt': %w", err)
		}
		resultField = field
	}

	pp := &pipePackLogfmt{
		resultField:  resultField,
		fieldFilters: fieldFilters,
	}

	return pp, nil
}
