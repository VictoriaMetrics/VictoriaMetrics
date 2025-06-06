package sa1016

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1016",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Trapping a signal that cannot be trapped`,
		Text: `Not all signals can be intercepted by a process. Specifically, on
UNIX-like systems, the \'syscall.SIGKILL\' and \'syscall.SIGSTOP\' signals are
never passed to the process, but instead handled directly by the
kernel. It is therefore pointless to try and handle these signals.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	isSignal := func(pass *analysis.Pass, expr ast.Expr, name string) bool {
		if expr, ok := expr.(*ast.SelectorExpr); ok {
			return code.SelectorName(pass, expr) == name
		} else {
			return false
		}
	}

	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !code.IsCallToAny(pass, call,
			"os/signal.Ignore", "os/signal.Notify", "os/signal.Reset") {
			return
		}

		hasSigterm := false
		for _, arg := range call.Args {
			if conv, ok := arg.(*ast.CallExpr); ok && isSignal(pass, conv.Fun, "os.Signal") {
				arg = conv.Args[0]
			}

			if isSignal(pass, arg, "syscall.SIGTERM") {
				hasSigterm = true
				break
			}

		}
		for i, arg := range call.Args {
			if conv, ok := arg.(*ast.CallExpr); ok && isSignal(pass, conv.Fun, "os.Signal") {
				arg = conv.Args[0]
			}

			if isSignal(pass, arg, "os.Kill") || isSignal(pass, arg, "syscall.SIGKILL") {
				var fixes []analysis.SuggestedFix
				if !hasSigterm {
					nargs := make([]ast.Expr, len(call.Args))
					for j, a := range call.Args {
						if i == j {
							nargs[j] = edit.Selector("syscall", "SIGTERM")
						} else {
							nargs[j] = a
						}
					}
					ncall := *call
					ncall.Args = nargs
					fixes = append(fixes, edit.Fix(fmt.Sprintf("use syscall.SIGTERM instead of %s", report.Render(pass, arg)), edit.ReplaceWithNode(pass.Fset, call, &ncall)))
				}
				nargs := make([]ast.Expr, 0, len(call.Args))
				for j, a := range call.Args {
					if i == j {
						continue
					}
					nargs = append(nargs, a)
				}
				ncall := *call
				ncall.Args = nargs
				fixes = append(fixes, edit.Fix(fmt.Sprintf("remove %s from list of arguments", report.Render(pass, arg)), edit.ReplaceWithNode(pass.Fset, call, &ncall)))
				report.Report(pass, arg, fmt.Sprintf("%s cannot be trapped (did you mean syscall.SIGTERM?)", report.Render(pass, arg)), report.Fixes(fixes...))
			}
			if isSignal(pass, arg, "syscall.SIGSTOP") {
				nargs := make([]ast.Expr, 0, len(call.Args)-1)
				for j, a := range call.Args {
					if i == j {
						continue
					}
					nargs = append(nargs, a)
				}
				ncall := *call
				ncall.Args = nargs
				report.Report(pass, arg, "syscall.SIGSTOP cannot be trapped", report.Fixes(edit.Fix("remove syscall.SIGSTOP from list of arguments", edit.ReplaceWithNode(pass.Fset, call, &ncall))))
			}
		}
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
