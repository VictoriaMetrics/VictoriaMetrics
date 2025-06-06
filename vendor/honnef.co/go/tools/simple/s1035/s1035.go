package s1035

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1035",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Redundant call to \'net/http.CanonicalHeaderKey\' in method call on \'net/http.Header\'`,
		Text: `
The methods on \'net/http.Header\', namely \'Add\', \'Del\', \'Get\'
and \'Set\', already canonicalize the given header name.`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		callName := code.CallName(pass, call)
		switch callName {
		case "(net/http.Header).Add", "(net/http.Header).Del", "(net/http.Header).Get", "(net/http.Header).Set":
		default:
			return
		}

		if !code.IsCallTo(pass, call.Args[0], "net/http.CanonicalHeaderKey") {
			return
		}

		report.Report(pass, call,
			fmt.Sprintf("calling net/http.CanonicalHeaderKey on the 'key' argument of %s is redundant", callName),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("remove call to CanonicalHeaderKey", edit.ReplaceWithNode(pass.Fset, call.Args[0], call.Args[0].(*ast.CallExpr).Args[0]))))
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
