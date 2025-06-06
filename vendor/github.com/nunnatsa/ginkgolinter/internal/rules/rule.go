package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

type Rule interface {
	Apply(*expression.GomegaExpression, types.Config, *reports.Builder) bool
}

var rules = Rules{
	&ForceExpectToRule{},
	&LenRule{},
	&CapRule{},
	&ComparisonRule{},
	&NilCompareRule{},
	&ComparePointRule{},
	&ErrorEqualNilRule{},
	&MatchErrorRule{},
	getMatcherOnlyRules(),
	&EqualDifferentTypesRule{},
	&HaveOccurredRule{},
	&SucceedRule{},
}

var asyncRules = Rules{
	&AsyncFuncCallRule{},
	&AsyncTimeIntervalsRule{},
	&ErrorEqualNilRule{},
	&MatchErrorRule{},
	&AsyncSucceedRule{},
	getMatcherOnlyRules(),
}

func GetRules() Rules {
	return rules
}

func GetAsyncRules() Rules {
	return asyncRules
}

type Rules []Rule

func (r Rules) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	for _, rule := range r {
		if rule.Apply(gexp, config, reportBuilder) {
			return true
		}
	}

	return false
}

var missingAssertionRule = MissingAssertionRule{}

func GetMissingAssertionRule() Rule {
	return missingAssertionRule
}
