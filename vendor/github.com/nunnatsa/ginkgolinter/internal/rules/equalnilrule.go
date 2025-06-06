package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

// EqualNilRule validate that there is no use of Equal(nil) in the code
// It is part of assertion only rules
type EqualNilRule struct{}

func (r EqualNilRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	return !config.SuppressNil &&
		gexp.MatcherTypeIs(matcher.EqualValueMatcherType)
}

func (r EqualNilRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	gexp.SetMatcherBeNil()

	reportBuilder.AddIssue(true, wrongNilWarningTemplate)

	return true
}
