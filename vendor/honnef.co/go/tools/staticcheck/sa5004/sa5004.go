package sa5004

import (
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
		Name:     "SA5004",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `\"for { select { ...\" with an empty default branch spins`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		if len(loop.Body.List) != 1 || loop.Cond != nil || loop.Init != nil {
			return
		}
		sel, ok := loop.Body.List[0].(*ast.SelectStmt)
		if !ok {
			return
		}
		for _, c := range sel.Body.List {
			// FIXME this leaves behind an empty line, and possibly
			// comments in the default branch. We can't easily fix
			// either.
			if comm, ok := c.(*ast.CommClause); ok && comm.Comm == nil && len(comm.Body) == 0 {
				report.Report(pass, comm, "should not have an empty default case in a for+select loop; the loop will spin",
					report.Fixes(edit.Fix("remove empty default branch", edit.Delete(comm))))
				// there can only be one default case
				break
			}
		}
	}
	code.Preorder(pass, fn, (*ast.ForStmt)(nil))
	return nil, nil
}
