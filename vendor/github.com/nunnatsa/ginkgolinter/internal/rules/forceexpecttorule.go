package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const forceExpectToTemplate = "must not use %s with %s"

type ForceExpectToRule struct{}

func (ForceExpectToRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	if !config.ForceExpectTo {
		return false
	}

	actlName := gexp.GetActualFuncName()
	return actlName == "Expect" || actlName == "ExpectWithOffset"
}

func (r ForceExpectToRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	var newName string

	switch gexp.GetAssertFuncName() {
	case "Should":
		newName = "To"
	case "ShouldNot":
		newName = "ToNot"
	default:
		return false
	}

	gexp.ReplaceAssertionMethod(newName)
	reportBuilder.AddIssue(true, forceExpectToTemplate, gexp.GetActualFuncName(), gexp.GetOrigAssertFuncName())

	// always return false, to keep checking another rules.
	return false
}
