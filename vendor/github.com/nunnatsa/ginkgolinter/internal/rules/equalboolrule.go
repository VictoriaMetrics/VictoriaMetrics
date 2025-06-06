package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const wrongBoolWarningTemplate = "wrong boolean assertion"

type EqualBoolRule struct{}

func (r EqualBoolRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.EqualBoolValueMatcherType)
}

func (r EqualBoolRule) Apply(gexp *expression.GomegaExpression, _ types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	if gexp.MatcherTypeIs(matcher.BoolValueTrue) {
		gexp.SetMatcherBeTrue()
	} else {
		if gexp.IsNegativeAssertion() {
			gexp.ReverseAssertionFuncLogic()
			gexp.SetMatcherBeTrue()
		} else {
			gexp.SetMatcherBeFalse()
		}
	}

	reportBuilder.AddIssue(true, wrongBoolWarningTemplate)
	return true
}
