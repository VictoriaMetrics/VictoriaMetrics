package logstorage

import (
	"fmt"
)

// pipeFieldValues processes '| field_values ...' queries.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#field_values-pipe
type pipeFieldValues struct {
	field string

	limit uint64
}

func (pf *pipeFieldValues) String() string {
	s := "field_values " + quoteTokenIfNeeded(pf.field)
	if pf.limit > 0 {
		s += fmt.Sprintf(" limit %d", pf.limit)
	}
	return s
}

func (pf *pipeFieldValues) canLiveTail() bool {
	return false
}

func (pf *pipeFieldValues) updateNeededFields(neededFields, unneededFields fieldsSet) {
	if neededFields.isEmpty() {
		neededFields.add(pf.field)
		return
	}

	if neededFields.contains("*") {
		neededFields.reset()
		if !unneededFields.contains(pf.field) {
			neededFields.add(pf.field)
		}
		unneededFields.reset()
	} else {
		neededFieldsOrig := neededFields.clone()
		neededFields.reset()
		if neededFieldsOrig.contains(pf.field) {
			neededFields.add(pf.field)
		}
	}
}

func (pf *pipeFieldValues) optimize() {
	// nothing to do
}

func (pf *pipeFieldValues) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldValues) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pf, nil
}

func (pf *pipeFieldValues) newPipeProcessor(workersCount int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	hitsFieldName := "hits"
	if hitsFieldName == pf.field {
		hitsFieldName = "hitss"
	}
	pu := &pipeUniq{
		byFields:      []string{pf.field},
		hitsFieldName: hitsFieldName,
		limit:         pf.limit,
	}
	return pu.newPipeProcessor(workersCount, stopCh, cancel, ppNext)
}

func parsePipeFieldValues(lex *lexer) (*pipeFieldValues, error) {
	if !lex.isKeyword("field_values") {
		return nil, fmt.Errorf("expecting 'field_values'; got %q", lex.token)
	}
	lex.nextToken()

	field, err := parseFieldNameWithOptionalParens(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse field name for 'field_values': %w", err)
	}

	limit := uint64(0)
	if lex.isKeyword("limit") {
		lex.nextToken()
		n, ok := tryParseUint64(lex.token)
		if !ok {
			return nil, fmt.Errorf("cannot parse 'limit %s'", lex.token)
		}
		lex.nextToken()
		limit = n
	}

	pf := &pipeFieldValues{
		field: field,
		limit: limit,
	}

	return pf, nil
}
