package sa3001

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA3001",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Assigning to \'b.N\' in benchmarks distorts the results`,
		Text: `The testing package dynamically sets \'b.N\' to improve the reliability of
benchmarks and uses it in computations to determine the duration of a
single operation. Benchmark code must not alter \'b.N\' as this would
falsify results.`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		assign := node.(*ast.AssignStmt)
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			return
		}
		if sel.Sel.Name != "N" {
			return
		}
		if !code.IsOfPointerToTypeWithName(pass, sel.X, "testing.B") {
			return
		}
		report.Report(pass, assign, fmt.Sprintf("should not assign to %s", report.Render(pass, sel)))
	}
	code.Preorder(pass, fn, (*ast.AssignStmt)(nil))
	return nil, nil
}
