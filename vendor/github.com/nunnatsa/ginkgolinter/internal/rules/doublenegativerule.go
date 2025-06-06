package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const doubleNegativeWarningTemplate = "avoid double negative assertion"

type DoubleNegativeRule struct{}

func (DoubleNegativeRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.BeFalseMatcherType) &&
		gexp.IsNegativeAssertion()
}

func (r DoubleNegativeRule) Apply(gexp *expression.GomegaExpression, _ types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	gexp.ReverseAssertionFuncLogic()
	gexp.SetMatcherBeTrue()

	reportBuilder.AddIssue(true, doubleNegativeWarningTemplate)

	return true
}
