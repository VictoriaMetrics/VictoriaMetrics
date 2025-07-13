package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
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

func (pf *pipeFieldValues) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	pLocal := &pipeFieldValuesLocal{
		pf: pf,
	}
	return pf, []pipe{pLocal}
}

func (pf *pipeFieldValues) canLiveTail() bool {
	return false
}

func (pf *pipeFieldValues) updateNeededFields(f *prefixfilter.Filter) {
	f.Reset()
	f.AddAllowFilter(pf.field)
}

func (pf *pipeFieldValues) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldValues) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pf, nil
}

func (pf *pipeFieldValues) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pf *pipeFieldValues) newPipeProcessor(concurrency int, stopCh <-chan struct{}, cancel func(), ppNext pipeProcessor) pipeProcessor {
	hitsFieldName := pf.getHitsFieldName()
	pu := &pipeUniq{
		byFields:      []string{pf.field},
		hitsFieldName: hitsFieldName,
		limit:         pf.limit,
	}
	return pu.newPipeProcessor(concurrency, stopCh, cancel, ppNext)
}

func (pf *pipeFieldValues) getHitsFieldName() string {
	if pf.field == "hits" {
		return "hitss"
	}
	return "hits"
}

func parsePipeFieldValues(lex *lexer) (pipe, error) {
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
