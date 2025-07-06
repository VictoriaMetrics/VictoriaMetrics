package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

type ifFilter struct {
	f            filter
	allowFilters []string
}

func (iff *ifFilter) String() string {
	return "if (" + iff.f.String() + ")"
}

func parseIfFilter(lex *lexer) (*ifFilter, error) {
	if !lex.isKeyword("if") {
		return nil, fmt.Errorf("unexpected keyword %q; expecting 'if'", lex.token)
	}
	lex.nextToken()
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("unexpected token %q after 'if'; expecting '('", lex.token)
	}
	lex.nextToken()

	if lex.isKeyword(")") {
		lex.nextToken()
		iff := &ifFilter{
			f: &filterNoop{},
		}
		return iff, nil
	}

	f, err := parseFilter(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse 'if' filter: %w", err)
	}
	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("unexpected token %q after 'if' filter; expecting ')'", lex.token)
	}
	lex.nextToken()

	var pf prefixfilter.Filter
	f.updateNeededFields(&pf)
	allowFilters := pf.GetAllowFilters()

	iff := &ifFilter{
		f:            f,
		allowFilters: allowFilters,
	}

	return iff, nil
}
