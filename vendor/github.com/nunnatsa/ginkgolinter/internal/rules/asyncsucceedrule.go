package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type AsyncSucceedRule struct{}

func (AsyncSucceedRule) isApply(gexp *expression.GomegaExpression) bool {
	return gexp.IsAsync() &&
		gexp.MatcherTypeIs(matcher.SucceedMatcherType) &&
		gexp.ActualArgTypeIs(actual.FuncSigArgType) &&
		!gexp.ActualArgTypeIs(actual.ErrorTypeArgType|actual.GomegaParamArgType)
}

func (r AsyncSucceedRule) Apply(gexp *expression.GomegaExpression, _ types.Config, reportBuilder *reports.Builder) bool {
	if r.isApply(gexp) {
		if gexp.ActualArgTypeIs(actual.MultiRetsArgType) {
			reportBuilder.AddIssue(false, "Success matcher does not support multiple values")
		} else {
			reportBuilder.AddIssue(false, "Success matcher only support a single error value, or function with Gomega as its first parameter")
		}
	}

	return false
}
