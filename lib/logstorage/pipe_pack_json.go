package logstorage

import (
	"fmt"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// pipePackJSON processes '| pack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#pack_json-pipe
type pipePackJSON struct {
	resultField string

	// the field names and/or field name prefixes to put inside the packed json
	fieldFilters []string
}

func (pp *pipePackJSON) String() string {
	s := "pack_json"
	if len(pp.fieldFilters) > 0 {
		s += " fields (" + fieldNamesString(pp.fieldFilters) + ")"
	}
	if !isMsgFieldName(pp.resultField) {
		s += " as " + quoteTokenIfNeeded(pp.resultField)
	}
	return s
}

func (pp *pipePackJSON) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pp, nil
}

func (pp *pipePackJSON) canLiveTail() bool {
	return true
}

func (pp *pipePackJSON) updateNeededFields(pf *prefixfilter.Filter) {
	updateNeededFieldsForPipePack(pf, pp.resultField, pp.fieldFilters)
}

func (pp *pipePackJSON) hasFilterInWithQuery() bool {
	return false
}

func (pp *pipePackJSON) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pp, nil
}

func (pp *pipePackJSON) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pp *pipePackJSON) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return newPipePackProcessor(ppNext, pp.resultField, pp.fieldFilters, MarshalFieldsToJSON)
}

func parsePipePackJSON(lex *lexer) (pipe, error) {
	if !lex.isKeyword("pack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "pack_json")
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
			return nil, fmt.Errorf("cannot parse result field for 'pack_json': %w", err)
		}
		resultField = field
	}

	pp := &pipePackJSON{
		resultField:  resultField,
		fieldFilters: fieldFilters,
	}

	return pp, nil
}
