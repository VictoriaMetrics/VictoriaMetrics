package rules

import (
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const valueInEventually = "use a function call in %[1]s. This actually checks nothing, because %[1]s receives the function returned value, instead of function itself, and this value is never changed"

// AsyncFuncCallRule checks that there is no function call actual parameter,
// in an async actual method (e.g. Eventually).
//
// Async actual methods should get the function itself, not a function call, because
// then there is no async operation at all, and we're waiting for the function to be
// returned before calling the assertion.
//
// We do allow functions that return a function, a channel or a pointer.
type AsyncFuncCallRule struct{}

func (r AsyncFuncCallRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	if config.SuppressAsync || !gexp.IsAsync() {
		return false
	}

	if asyncArg := gexp.GetAsyncActualArg(); asyncArg != nil {
		return !asyncArg.IsValid()
	}

	return false
}

func (r AsyncFuncCallRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if r.isApplied(gexp, config) {

		gexp.AppendWithArgsToActual()

		reportBuilder.AddIssue(true, valueInEventually, gexp.GetActualFuncName())
	}
	return false
}
