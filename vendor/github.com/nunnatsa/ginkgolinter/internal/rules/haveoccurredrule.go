package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type HaveOccurredRule struct{}

func (r HaveOccurredRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.HaveOccurredMatcherType)
}

func (r HaveOccurredRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	if !gexp.ActualArgTypeIs(actual.ErrorTypeArgType) {
		reportBuilder.AddIssue(false, "asserting a non-error type with HaveOccurred matcher")
		return true
	}

	if config.ForceSucceedForFuncs && gexp.GetActualArg().(*actual.ErrPayload).IsFunc() {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherSucceed()
		reportBuilder.AddIssue(true, "prefer using the Succeed matcher for error function, instead of HaveOccurred")
		return true
	}

	return false
}
