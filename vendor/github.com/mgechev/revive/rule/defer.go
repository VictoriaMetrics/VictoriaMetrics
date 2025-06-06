package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/lint"
)

// DeferRule lints gotchas in defer statements.
type DeferRule struct {
	allow map[string]bool
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *DeferRule) Configure(arguments lint.Arguments) error {
	list, err := r.allowFromArgs(arguments)
	if err != nil {
		return err
	}
	r.allow = list
	return nil
}

// Apply applies the rule to given file.
func (r *DeferRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}
	w := lintDeferRule{onFailure: onFailure, allow: r.allow}

	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*DeferRule) Name() string {
	return "defer"
}

func (*DeferRule) allowFromArgs(args lint.Arguments) (map[string]bool, error) {
	if len(args) < 1 {
		allow := map[string]bool{
			"loop":              true,
			"call-chain":        true,
			"method-call":       true,
			"return":            true,
			"recover":           true,
			"immediate-recover": true,
		}

		return allow, nil
	}

	aa, ok := args[0].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid argument '%v' for 'defer' rule. Expecting []string, got %T", args[0], args[0])
	}

	allow := make(map[string]bool, len(aa))
	for _, subcase := range aa {
		sc, ok := subcase.(string)
		if !ok {
			return nil, fmt.Errorf("invalid argument '%v' for 'defer' rule. Expecting string, got %T", subcase, subcase)
		}
		allow[sc] = true
	}

	return allow, nil
}

type lintDeferRule struct {
	onFailure  func(lint.Failure)
	inALoop    bool
	inADefer   bool
	inAFuncLit bool
	allow      map[string]bool
}

func (w lintDeferRule) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.ForStmt:
		w.visitSubtree(n.Body, w.inADefer, true, w.inAFuncLit)
		return nil
	case *ast.RangeStmt:
		w.visitSubtree(n.Body, w.inADefer, true, w.inAFuncLit)
		return nil
	case *ast.FuncLit:
		w.visitSubtree(n.Body, w.inADefer, false, true)
		return nil
	case *ast.ReturnStmt:
		if len(n.Results) != 0 && w.inADefer && w.inAFuncLit {
			w.newFailure("return in a defer function has no effect", n, 1.0, lint.FailureCategoryLogic, "return")
		}
	case *ast.CallExpr:
		isCallToRecover := isIdent(n.Fun, "recover")
		switch {
		case !w.inADefer && isCallToRecover:
			// func fn() { recover() }
			//
			// confidence is not 1 because recover can be in a function that is deferred elsewhere
			w.newFailure("recover must be called inside a deferred function", n, 0.8, lint.FailureCategoryLogic, "recover")
		case w.inADefer && !w.inAFuncLit && isCallToRecover:
			// defer helper(recover())
			//
			// confidence is not truly 1 because this could be in a correctly-deferred func,
			// but it is very likely to be a misunderstanding of defer's behavior around arguments.
			w.newFailure("recover must be called inside a deferred function, this is executing recover immediately", n, 1, lint.FailureCategoryLogic, "immediate-recover")
		}
		return nil // no need to analyze the arguments of the function call
	case *ast.DeferStmt:
		if isIdent(n.Call.Fun, "recover") {
			// defer recover()
			//
			// confidence is not truly 1 because this could be in a correctly-deferred func,
			// but normally this doesn't suppress a panic, and even if it did it would silently discard the value.
			w.newFailure("recover must be called inside a deferred function, this is executing recover immediately", n, 1, lint.FailureCategoryLogic, "immediate-recover")
		}
		w.visitSubtree(n.Call.Fun, true, false, false)
		for _, a := range n.Call.Args {
			switch a.(type) {
			case *ast.FuncLit:
				continue // too hard to analyze deferred calls with func literals args
			default:
				w.visitSubtree(a, true, false, false) // check arguments, they should not contain recover()
			}
		}

		if w.inALoop {
			w.newFailure("prefer not to defer inside loops", n, 1.0, lint.FailureCategoryBadPractice, "loop")
		}

		switch fn := n.Call.Fun.(type) {
		case *ast.CallExpr:
			w.newFailure("prefer not to defer chains of function calls", fn, 1.0, lint.FailureCategoryBadPractice, "call-chain")
		case *ast.SelectorExpr:
			if id, ok := fn.X.(*ast.Ident); ok {
				isMethodCall := id != nil && id.Obj != nil && id.Obj.Kind == ast.Typ
				if isMethodCall {
					w.newFailure("be careful when deferring calls to methods without pointer receiver", fn, 0.8, lint.FailureCategoryBadPractice, "method-call")
				}
			}
		}

		return nil
	}

	return w
}

func (w lintDeferRule) visitSubtree(n ast.Node, inADefer, inALoop, inAFuncLit bool) {
	nw := lintDeferRule{
		onFailure:  w.onFailure,
		inADefer:   inADefer,
		inALoop:    inALoop,
		inAFuncLit: inAFuncLit,
		allow:      w.allow,
	}
	ast.Walk(nw, n)
}

func (w lintDeferRule) newFailure(msg string, node ast.Node, confidence float64, cat lint.FailureCategory, subcase string) {
	if !w.allow[subcase] {
		return
	}

	w.onFailure(lint.Failure{
		Confidence: confidence,
		Node:       node,
		Category:   cat,
		Failure:    msg,
	})
}
