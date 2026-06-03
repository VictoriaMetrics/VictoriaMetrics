package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

type ifFilter struct {
	f            filter
	allowFilters []string
}

func (iff *ifFilter) String() string {
	return "if (" + iff.f.String() + ")"
}

func parseIfFilter(lex *lexer) (*ifFilter, error) {
	if !lex.isKeyword("if", "case") {
		return nil, fmt.Errorf("unexpected keyword %q; expecting 'if' or 'case'", lex.token)
	}
	lex.nextToken()
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("unexpected token %q after 'if'; expecting '('", lex.token)
	}
	lex.nextToken()

	if lex.isKeyword(")") {
		lex.nextToken()
		return newIfFilter(newFilterNoop()), nil
	}

	f, err := parseFilter(lex, true)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'if' filter: %w", err)
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q after 'if' filter; expecting ')'", lex.token)
	}
	lex.nextToken()

	return newIfFilter(f), nil
}

func newIfFilter(f filter) *ifFilter {
	var pf prefixfilter.Filter
	f.updateNeededFields(&pf)
	allowFilters := pf.GetAllowFilters()

	iff := &ifFilter{
		f:            f,
		allowFilters: allowFilters,
	}

	return iff
}
