package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type ErrorEqualNilRule struct{}

func (ErrorEqualNilRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	if config.SuppressErr {
		return false
	}

	if !gexp.IsAsync() && gexp.ActualArgTypeIs(actual.FuncSigArgType) {
		return false
	}

	return gexp.ActualArgTypeIs(actual.ErrorTypeArgType) &&
		gexp.MatcherTypeIs(matcher.BeNilMatcherType|matcher.EqualNilMatcherType)
}

func (r ErrorEqualNilRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	if v, ok := gexp.GetActualArg().(value.Valuer); ok && v.IsFunc() || gexp.ActualArgTypeIs(actual.ErrFuncActualArgType) {
		gexp.SetMatcherSucceed()
	} else {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherHaveOccurred()
	}

	reportBuilder.AddIssue(true, wrongErrWarningTemplate)

	return true
}
