package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const comparePointerToValue = "comparing a pointer to a value will always fail"

type ComparePointRule struct{}

func (r ComparePointRule) isApplied(gexp *expression.GomegaExpression) bool {
	actl, ok := gexp.GetActualArg().(*actual.RegularArgPayload)
	if !ok {
		return false
	}

	return actl.IsPointer()
}

func (r ComparePointRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	switch mtchr := gexp.GetMatcherInfo().(type) {
	case *matcher.EqualMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() {
			return false
		}

	case *matcher.BeEquivalentToMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() || mtchr.IsNil() {
			return false
		}

	case *matcher.BeIdenticalToMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() || mtchr.IsNil() {
			return false
		}

	case *matcher.EqualNilMatcher:
		return false

	case *matcher.BeTrueMatcher,
		*matcher.BeFalseMatcher,
		*matcher.BeNumericallyMatcher,
		*matcher.EqualTrueMatcher,
		*matcher.EqualFalseMatcher:

	default:
		return false
	}

	getMatcherOnlyRules().Apply(gexp, config, reportBuilder)

	gexp.SetMatcherHaveValue()
	reportBuilder.AddIssue(true, comparePointerToValue)

	return true
}
