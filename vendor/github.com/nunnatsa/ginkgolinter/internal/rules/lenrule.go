package rules

import (
	"go/token"

	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const wrongLengthWarningTemplate = "wrong length assertion"

// LenRule does not allow using the len() function in actual with numeric comparison. Instead,
// it suggests to use the HaveLen matcher, or the BeEmpty matcher, if comparing to zero.
type LenRule struct{}

func (r *LenRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {

	if !r.isApplied(gexp, config) {
		return false
	}

	if r.fixExpression(gexp) {
		reportBuilder.AddIssue(true, wrongLengthWarningTemplate)
		return true
	}
	return false
}

func (r *LenRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	if config.SuppressLen {
		return false
	}

	if gexp.ActualArgTypeIs(actual.LenFuncActualArgType) {
		if gexp.MatcherTypeIs(matcher.EqualMatcherType | matcher.BeZeroMatcherType) {
			return true
		}

		if gexp.MatcherTypeIs(matcher.BeNumericallyMatcherType) {
			mtchr := gexp.GetMatcherInfo().(*matcher.BeNumericallyMatcher)
			return mtchr.GetOp() == token.EQL || mtchr.GetOp() == token.NEQ || gexp.MatcherTypeIs(matcher.EqualZero|matcher.GreaterThanZero)
		}
	}

	if gexp.ActualArgTypeIs(actual.LenComparisonActualArgType) && gexp.MatcherTypeIs(matcher.BeTrueMatcherType|matcher.BeFalseMatcherType|matcher.EqualBoolValueMatcherType) {
		return true
	}

	return false
}

func (r *LenRule) fixExpression(gexp *expression.GomegaExpression) bool {
	if gexp.ActualArgTypeIs(actual.LenFuncActualArgType) {
		return r.fixEqual(gexp)
	}

	if gexp.ActualArgTypeIs(actual.LenComparisonActualArgType) {
		return r.fixComparison(gexp)
	}

	return false
}

func (r *LenRule) fixEqual(gexp *expression.GomegaExpression) bool {

	if gexp.MatcherTypeIs(matcher.EqualMatcherType) {
		gexp.SetLenNumericMatcher()

	} else if gexp.MatcherTypeIs(matcher.BeZeroMatcherType) {
		gexp.SetMatcherBeEmpty()

	} else if gexp.MatcherTypeIs(matcher.BeNumericallyMatcherType) {
		mtchr := gexp.GetMatcherInfo().(*matcher.BeNumericallyMatcher)
		op := mtchr.GetOp()

		if op == token.EQL {
			gexp.SetLenNumericMatcher()
		} else if op == token.NEQ {
			gexp.ReverseAssertionFuncLogic()
			gexp.SetLenNumericMatcher()
		} else if gexp.MatcherTypeIs(matcher.GreaterThanZero) {
			gexp.ReverseAssertionFuncLogic()
			gexp.SetMatcherBeEmpty()
		} else {
			return false
		}
	} else {
		return false
	}

	gexp.ReplaceActualWithItsFirstArg()
	return true
}

func (r *LenRule) fixComparison(gexp *expression.GomegaExpression) bool {
	actl := gexp.GetActualArg().(*actual.FuncComparisonPayload)
	if op := actl.GetOp(); op == token.NEQ {
		gexp.ReverseAssertionFuncLogic()
	} else if op != token.EQL {
		return false
	}

	if gexp.MatcherTypeIs(matcher.BoolValueFalse) {
		gexp.ReverseAssertionFuncLogic()
	}

	if actl.IsValueZero() {
		gexp.SetMatcherBeEmpty()
	} else {
		gexp.SetMatcherLen(actl.GetValueExpr())
	}

	gexp.ReplaceActual(actl.GetFuncArg())

	return true
}
