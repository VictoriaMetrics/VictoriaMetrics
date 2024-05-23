package logstorage

import (
	"fmt"
)

type ifFilter struct {
	f            filter
	neededFields []string
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

	neededFields := newFieldsSet()
	f.updateNeededFields(neededFields)

	iff := &ifFilter{
		f:            f,
		neededFields: neededFields.getAll(),
	}

	return iff, nil
}

func (iff *ifFilter) optimizeFilterIn() {
	if iff == nil {
		return
	}

	optimizeFilterIn(iff.f)
}

func optimizeFilterIn(f filter) {
	if f == nil {
		return
	}

	visitFunc := func(f filter) bool {
		fi, ok := f.(*filterIn)
		if ok && fi.q != nil {
			fi.q.Optimize()
		}
		return false
	}
	_ = visitFilter(f, visitFunc)
}
