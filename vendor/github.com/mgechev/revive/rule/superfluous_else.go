package rule

import (
	"fmt"

	"github.com/mgechev/revive/internal/ifelse"
	"github.com/mgechev/revive/lint"
)

// SuperfluousElseRule lints given else constructs.
type SuperfluousElseRule struct{}

// Apply applies the rule to given file.
func (e *SuperfluousElseRule) Apply(file *lint.File, args lint.Arguments) []lint.Failure {
	return ifelse.Apply(e.checkIfElse, file.AST, ifelse.TargetElse, args)
}

// Name returns the rule name.
func (*SuperfluousElseRule) Name() string {
	return "superfluous-else"
}

func (*SuperfluousElseRule) checkIfElse(chain ifelse.Chain, args ifelse.Args) (string, bool) {
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

	if chain.If.Returns() {
		// avoid overlapping with indent-error-flow
		return "", false
	}

	if args.PreserveScope && !chain.AtBlockEnd && (chain.HasInitializer || chain.Else.HasDecls()) {
		// avoid increasing variable scope
		return "", false
	}

	return fmt.Sprintf("if block ends with %v, so drop this else and outdent its block", chain.If.LongString()), true
}
