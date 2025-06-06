package sa1005

import (
	"go/ast"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1005",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Invalid first argument to \'exec.Command\'`,
		Text: `\'os/exec\' runs programs directly (using variants of the fork and exec
system calls on Unix systems). This shouldn't be confused with running
a command in a shell. The shell will allow for features such as input
redirection, pipes, and general scripting. The shell is also
responsible for splitting the user's input into a program name and its
arguments. For example, the equivalent to

    ls / /tmp

would be

    exec.Command("ls", "/", "/tmp")

If you want to run a command in a shell, consider using something like
the following – but be aware that not all systems, particularly
Windows, will have a \'/bin/sh\' program:

    exec.Command("/bin/sh", "-c", "ls | grep Awesome")`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !code.IsCallTo(pass, call, "os/exec.Command") {
			return
		}
		val, ok := code.ExprToString(pass, call.Args[knowledge.Arg("os/exec.Command.name")])
		if !ok {
			return
		}
		if !strings.Contains(val, " ") || strings.Contains(val, `\`) || strings.Contains(val, "/") {
			return
		}
		report.Report(pass, call.Args[knowledge.Arg("os/exec.Command.name")],
			"first argument to exec.Command looks like a shell command, but a program name or path are expected")
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
