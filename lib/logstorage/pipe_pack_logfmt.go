package logstorage

import (
	"fmt"
	"slices"
)

// pipePackLogfmt processes '| pack_logfmt ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#pack_logfmt-pipe
type pipePackLogfmt struct {
	resultField string

	fields []string
}

func (pp *pipePackLogfmt) String() string {
	s := "pack_logfmt"
	if len(pp.fields) > 0 {
		s += " fields (" + fieldsToString(pp.fields) + ")"
	}
	if !isMsgFieldName(pp.resultField) {
		s += " as " + quoteTokenIfNeeded(pp.resultField)
	}
	return s
}

func (pp *pipePackLogfmt) canLiveTail() bool {
	return true
}

func (pp *pipePackLogfmt) updateNeededFields(neededFields, unneededFields fieldsSet) {
	updateNeededFieldsForPipePack(neededFields, unneededFields, pp.resultField, pp.fields)
}

func (pp *pipePackLogfmt) optimize() {
	// nothing to do
}

func (pp *pipePackLogfmt) hasFilterInWithQuery() bool {
	return false
}

func (pp *pipePackLogfmt) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pp, nil
}

func (pp *pipePackLogfmt) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	return newPipePackProcessor(workersCount, ppNext, pp.resultField, pp.fields, MarshalFieldsToLogfmt)
}

func parsePackLogfmt(lex *lexer) (*pipePackLogfmt, error) {
	if !lex.isKeyword("pack_logfmt") {
		return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "pack_logfmt")
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
			return nil, fmt.Errorf("cannot parse result field for 'pack_logfmt': %w", err)
		}
		resultField = field
	}

	pp := &pipePackLogfmt{
		resultField: resultField,
		fields:      fields,
	}

	return pp, nil
}
