package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type HaveLen0 struct{}

func (r *HaveLen0) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	return gexp.MatcherTypeIs(matcher.HaveLenZeroMatcherType) && !config.AllowHaveLen0
}

func (r *HaveLen0) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}
	gexp.SetMatcherBeEmpty()
	reportBuilder.AddIssue(true, wrongLengthWarningTemplate)
	return true
}
