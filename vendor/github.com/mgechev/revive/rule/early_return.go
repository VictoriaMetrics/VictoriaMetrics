package rule

import (
	"fmt"

	"github.com/mgechev/revive/internal/ifelse"
	"github.com/mgechev/revive/lint"
)

// EarlyReturnRule finds opportunities to reduce nesting by inverting
// the condition of an "if" block.
type EarlyReturnRule struct{}

// Apply applies the rule to given file.
func (e *EarlyReturnRule) Apply(file *lint.File, args lint.Arguments) []lint.Failure {
	return ifelse.Apply(e.checkIfElse, file.AST, ifelse.TargetIf, args)
}

// Name returns the rule name.
func (*EarlyReturnRule) Name() string {
	return "early-return"
}

func (*EarlyReturnRule) checkIfElse(chain ifelse.Chain, args ifelse.Args) (string, bool) {
	if chain.HasElse {
		if !chain.Else.BranchKind.Deviates() {
			// this rule only applies if the else-block deviates control flow
			return "", false
		}
	} else if !args.AllowJump || !chain.AtBlockEnd || !chain.BlockEndKind.Deviates() || chain.If.IsShort() {
		// this kind of refactor requires introducing a new indented "return", "continue" or "break" statement,
		// so ignore unless we are able to outdent multiple statements in exchange.
		return "", false
	}

	if chain.HasPriorNonDeviating && !chain.If.IsEmpty() {
		// if we de-indent this block then a previous branch
		// might flow into it, affecting program behavior
		return "", false
	}

	if chain.HasElse && chain.If.Deviates() {
		// avoid overlapping with superfluous-else
		return "", false
	}

	if args.PreserveScope && !chain.AtBlockEnd && (chain.HasInitializer || chain.If.HasDecls()) {
		// avoid increasing variable scope
		return "", false
	}

	if !chain.HasElse {
		return fmt.Sprintf("if c { ... } can be rewritten if !c { %v } ... to reduce nesting", chain.BlockEndKind), true
	}

	if chain.If.IsEmpty() {
		return fmt.Sprintf("if c { } else %[1]v can be simplified to if !c %[1]v", chain.Else), true
	}
	return fmt.Sprintf("if c { ... } else %[1]v can be simplified to if !c %[1]v ...", chain.Else), true
}
