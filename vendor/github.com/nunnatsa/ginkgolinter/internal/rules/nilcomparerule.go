package rules

import (
	"go/token"

	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const (
	wrongNilWarningTemplate = "wrong nil assertion"
	wrongErrWarningTemplate = "wrong error assertion"
)

type NilCompareRule struct{}

func (r NilCompareRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	isErr, ruleApplied := r.isApplied(gexp, config)
	if !ruleApplied {
		return false
	}

	if gexp.MatcherTypeIs(matcher.BoolValueFalse) {
		gexp.ReverseAssertionFuncLogic()
	}

	r.handleNilBeBoolMatcher(gexp, gexp.GetActualArg().(*actual.NilComparisonPayload), reportBuilder, isErr)

	return true
}

func (r NilCompareRule) isApplied(gexp *expression.GomegaExpression, config types.Config) (bool, bool) {
	if !gexp.MatcherTypeIs(matcher.EqualBoolValueMatcherType | matcher.BeTrueMatcherType | matcher.BeFalseMatcherType) {
		return false, false
	}

	actl, ok := gexp.GetActualArg().(*actual.NilComparisonPayload)
	if !ok {
		return false, false
	}

	isErr := actl.IsError() && !config.SuppressErr

	if !isErr && config.SuppressNil {
		return isErr, false
	}

	return isErr, true
}

func (r NilCompareRule) handleNilBeBoolMatcher(gexp *expression.GomegaExpression, actl *actual.NilComparisonPayload, reportBuilder *reports.Builder, isErr bool) {
	template := wrongNilWarningTemplate
	if isErr {
		template = wrongErrWarningTemplate
		if actl.IsFunc() {
			gexp.SetMatcherSucceed()
		} else {
			gexp.ReverseAssertionFuncLogic()
			gexp.SetMatcherHaveOccurred()
		}
	} else {
		gexp.SetMatcherBeNil()
	}

	gexp.ReplaceActual(actl.GetValueExpr())

	if actl.GetOp() == token.NEQ {
		gexp.ReverseAssertionFuncLogic()
	}

	reportBuilder.AddIssue(true, template)
}
