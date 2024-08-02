package logstorage

import (
	"fmt"
	"slices"
)

// pipePackJSON processes '| pack_json ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#pack_json-pipe
type pipePackJSON struct {
	resultField string

	fields []string
}

func (pp *pipePackJSON) String() string {
	s := "pack_json"
	if len(pp.fields) > 0 {
		s += " fields (" + fieldsToString(pp.fields) + ")"
	}
	if !isMsgFieldName(pp.resultField) {
		s += " as " + quoteTokenIfNeeded(pp.resultField)
	}
	return s
}

func (pp *pipePackJSON) canLiveTail() bool {
	return true
}

func (pp *pipePackJSON) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForPipePack(neededFields, unneededFields, pp.resultField, pp.fields)
}

func (pp *pipePackJSON) optimize() {
	// nothing to do
}

func (pp *pipePackJSON) hasFilterInWithQuery() bool {
	return false
}

func (pp *pipePackJSON) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pp, nil
}

func (pp *pipePackJSON) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return newPipePackProcessor(workersCount, ppNext, pp.resultField, pp.fields, MarshalFieldsToJSON)
}

func parsePackJSON(lex *lexer) (*pipePackJSON, error) {
	if !lex.isKeyword("pack_json") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "pack_json")
	}
	lex.nextToken()

	var fields []string
	if lex.isKeyword("fields") {
		lex.nextToken()
		fs, err := parseFieldNamesInParens(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse fields: %w", err)
		}
		if slices.Contains(fs, "*") {
			fs = nil
		}
		fields = fs
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
		resultField: resultField,
		fields:      fields,
	}

	return pp, nil
}
