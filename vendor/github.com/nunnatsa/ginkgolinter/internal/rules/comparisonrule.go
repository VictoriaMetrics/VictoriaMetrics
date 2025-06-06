package rules

import (
	"go/token"

	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const wrongCompareWarningTemplate = "wrong comparison assertion"

type ComparisonRule struct{}

func (r ComparisonRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	if config.SuppressCompare {
		return false
	}

	return gexp.ActualArgTypeIs(actual.ComparisonActualArgType)
}

func (r ComparisonRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	actl, ok := gexp.GetActualArg().(actual.ComparisonActualPayload)
	if !ok {
		return false
	}

	switch actl.GetOp() {
	case token.EQL:
		r.handleEqualComparison(gexp, actl)

	case token.NEQ:
		gexp.ReverseAssertionFuncLogic()
		r.handleEqualComparison(gexp, actl)
	case token.GTR, token.GEQ, token.LSS, token.LEQ:
		if !actl.GetRight().IsValueNumeric() {
			return false
		}

		gexp.SetMatcherBeNumerically(actl.GetOp(), actl.GetRight().GetValueExpr())

	default:
		return false
	}

	if gexp.MatcherTypeIs(matcher.BoolValueFalse) {
		gexp.ReverseAssertionFuncLogic()
	}

	gexp.ReplaceActual(actl.GetLeft().GetValueExpr())

	reportBuilder.AddIssue(true, wrongCompareWarningTemplate)
	return true
}

func (r ComparisonRule) handleEqualComparison(gexp *expression.GomegaExpression, actual actual.ComparisonActualPayload) {
	if actual.GetRight().IsValueZero() {
		gexp.SetMatcherBeZero()
	} else {
		left := actual.GetLeft()
		arg := actual.GetRight().GetValueExpr()
		if left.IsInterface() || left.IsPointer() {
			gexp.SetMatcherBeIdenticalTo(arg)
		} else {
			gexp.SetMatcherEqual(arg)
		}
	}
}
