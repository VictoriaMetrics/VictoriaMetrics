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

	// if the filter is non-empty then only the field values containing the given filter substring are returned.
	filter string

	limit uint64
}

func (pf *pipeFieldValues) String() string {
	s := "field_values " + quoteTokenIfNeeded(pf.field)
	if pf.filter != "" {
		s += " filter " + quoteTokenIfNeeded(pf.filter)
	}
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

func (pf *pipeFieldValues) canReturnLastNResults() bool {
	return false
}

func (pf *pipeFieldValues) isFixedOutputFieldsOrder() bool {
	return true
}

func (pf *pipeFieldValues) updateNeededFields(f *prefixfilter.Filter) {
	f.Reset()
	f.AddAllowFilter(pf.field)
}

func (pf *pipeFieldValues) hasFilterInWithQuery() bool {
	return false
}

func (pf *pipeFieldValues) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc) (pipe, error) {
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
		filter:        pf.filter,
		limit:         pf.limit,
	}
	return pu.newPipeProcessor(concurrency, stopCh, cancel, ppNext)
}

func (pf *pipeFieldValues) getHitsFieldName() string {
	return getUniqueResultName("hits", []string{pf.field})
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

	filter := ""
	if lex.isKeyword("filter") {
		lex.nextToken()
		f, err := lex.nextCompoundToken()
		if err != nil {
			return nil, fmt.Errorf("cannot parse filter for 'field_values': %w", err)
		}
		filter = f
	}

	limit := uint64(0)
	if lex.isKeyword("limit") {
		n, err := parseLimit(lex)
		if err != nil {
			return nil, err
		}
		limit = n
	}

	pf := &pipeFieldValues{
		field:  field,
		filter: filter,
		limit:  limit,
	}

	return pf, nil
}
