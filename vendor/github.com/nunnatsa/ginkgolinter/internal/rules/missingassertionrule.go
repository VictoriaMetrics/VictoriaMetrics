package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const missingAssertionMessage = `%q: missing assertion method. Expected %s`

type MissingAssertionRule struct{}

func (r MissingAssertionRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.IsMissingAssertion()
}

func (r MissingAssertionRule) Apply(gexp *expression.GomegaExpression, _ types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	actualMethodName := gexp.GetActualFuncName()
	reportBuilder.AddIssue(false, missingAssertionMessage, actualMethodName, gomegainfo.GetAllowedAssertionMethods(actualMethodName))

	return true
}
