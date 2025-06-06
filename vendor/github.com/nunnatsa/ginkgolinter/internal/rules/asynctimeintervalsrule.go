package rules

import (
	"go/ast"
	"time"

	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/intervals"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
	"github.com/nunnatsa/ginkgolinter/types"
)

const (
	multipleTimeouts               = "timeout defined more than once"
	multiplePolling                = "polling defined more than once"
	onlyUseTimeDurationForInterval = "only use time.Duration for timeout and polling in Eventually() or Consistently()"
	pollingGreaterThanTimeout      = "timeout must not be shorter than the polling interval"
)

type AsyncTimeIntervalsRule struct{}

func (r AsyncTimeIntervalsRule) isApplied(gexp *expression.GomegaExpression, config types.Config) bool {
	return !config.SuppressAsync && config.ValidateAsyncIntervals && gexp.IsAsync()
}

func (r AsyncTimeIntervalsRule) Apply(gexp *expression.GomegaExpression, config types.Config, reportBuilder *reports.Builder) bool {
	if r.isApplied(gexp, config) {
		asyncArg := gexp.GetAsyncActualArg()
		if asyncArg.TooManyTimeouts() {
			reportBuilder.AddIssue(false, multipleTimeouts)
		}

		if asyncArg.TooManyPolling() {
			reportBuilder.AddIssue(false, multiplePolling)
		}

		timeoutDuration := checkInterval(gexp, asyncArg.Timeout(), reportBuilder)
		pollingDuration := checkInterval(gexp, asyncArg.Polling(), reportBuilder)

		if timeoutDuration > 0 && pollingDuration > 0 && pollingDuration > timeoutDuration {
			reportBuilder.AddIssue(false, pollingGreaterThanTimeout)
		}
	}

	return false
}

func checkInterval(gexp *expression.GomegaExpression, durVal intervals.DurationValue, reportBuilder *reports.Builder) time.Duration {
	if durVal != nil {
		switch to := durVal.(type) {
		case *intervals.RealDurationValue, *intervals.UnknownDurationTypeValue:

		case *intervals.NumericDurationValue:
			if checkNumericInterval(gexp.GetActualClone(), to) {
				reportBuilder.AddIssue(true, onlyUseTimeDurationForInterval)
			}

		case *intervals.UnknownDurationValue:
			reportBuilder.AddIssue(true, onlyUseTimeDurationForInterval)
		}

		return durVal.Duration()
	}

	return 0
}

func checkNumericInterval(intervalMethod *ast.CallExpr, interval intervals.DurationValue) bool {
	if interval != nil {
		if numVal, ok := interval.(intervals.NumericValue); ok {
			if offset := numVal.GetOffset(); offset > 0 {
				intervalMethod.Args[offset] = numVal.GetDurationExpr()
				return true
			}
		}
	}

	return false
}
