package matcher

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
)

type MultipleMatchersMatcher struct {
	matherType Type
	matchers   []*Matcher
}

func (m *MultipleMatchersMatcher) Type() Type {
	return m.matherType
}

func (m *MultipleMatchersMatcher) MatcherName() string {
	if m.matherType.Is(OrMatherType) {
		return or
	}
	return and
}

func newMultipleMatchersMatcher(matherType Type, orig, clone []ast.Expr, pass *analysis.Pass, handler gomegahandler.Handler) (*MultipleMatchersMatcher, bool) {
	matchers := make([]*Matcher, len(orig))

	for i := range orig {
		nestedOrig, ok := orig[i].(*ast.CallExpr)
		if !ok {
			return nil, false
		}

		m, ok := New(nestedOrig, clone[i].(*ast.CallExpr), pass, handler)
		if !ok {
			return nil, false
		}

		m.reverseLogic = false

		matchers[i] = m
	}

	return &MultipleMatchersMatcher{
		matherType: matherType,
		matchers:   matchers,
	}, true
}

func (m *MultipleMatchersMatcher) Len() int {
	return len(m.matchers)
}

func (m *MultipleMatchersMatcher) At(i int) *Matcher {
	if i >= len(m.matchers) {
		panic("index out of range")
	}

	return m.matchers[i]
}
