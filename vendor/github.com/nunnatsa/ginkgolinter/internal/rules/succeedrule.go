package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type SucceedRule struct{}

func (r SucceedRule) isApplied(gexp *expression.GomegaExpression) bool {
	return !gexp.IsAsync() && gexp.MatcherTypeIs(matcher.SucceedMatcherType)
}

func (r SucceedRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	if !gexp.ActualArgTypeIs(actual.ErrorTypeArgType) {
		if gexp.IsActualTuple() {
			reportBuilder.AddIssue(false, "the Success matcher does not support multiple values")
		} else {
			reportBuilder.AddIssue(false, "asserting a non-error type with Succeed matcher")
		}
		return true
	}

	if config.ForceSucceedForFuncs && !gexp.GetActualArg().(*actual.ErrPayload).IsFunc() {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherHaveOccurred()

		reportBuilder.AddIssue(true, "prefer using the HaveOccurred matcher for non-function error value, instead of Succeed")

		return true
	}

	return false
}
