package rule

import (
	"github.com/mgechev/revive/internal/ifelse"
	"github.com/mgechev/revive/lint"
)

// IndentErrorFlowRule prevents redundant else statements.
type IndentErrorFlowRule struct{}

// Apply applies the rule to given file.
func (e *IndentErrorFlowRule) Apply(file *lint.File, args lint.Arguments) []lint.Failure {
	return ifelse.Apply(e.checkIfElse, file.AST, ifelse.TargetElse, args)
}

// Name returns the rule name.
func (*IndentErrorFlowRule) Name() string {
	return "indent-error-flow"
}

func (*IndentErrorFlowRule) checkIfElse(chain ifelse.Chain, args ifelse.Args) (string, bool) {
	if !chain.HasElse {
		return "", false
	}

	if !chain.If.Deviates() {
		// this rule only applies if the if-block deviates control flow
		return "", false
	}

	if chain.HasPriorNonDeviating {
		// if we de-indent the "else" block then a previous branch
		// might flow into it, affecting program behavior
		return "", false
	}

	if !chain.If.Returns() {
		// avoid overlapping with superfluous-else
		return "", false
	}

	if args.PreserveScope && !chain.AtBlockEnd && (chain.HasInitializer || chain.Else.HasDecls()) {
		// avoid increasing variable scope
		return "", false
	}

	return "if block ends with a return statement, so drop this else and outdent its block", true
}
