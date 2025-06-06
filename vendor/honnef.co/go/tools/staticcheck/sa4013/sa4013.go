package sa4013

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4013",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Negating a boolean twice (\'!!b\') is the same as writing \'b\'. This is either redundant, or a typo.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkDoubleNegationQ = pattern.MustParse(`(UnaryExpr "!" single@(UnaryExpr "!" x))`)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		if m, ok := code.Match(pass, checkDoubleNegationQ, node); ok {
			report.Report(pass, node, "negating a boolean twice has no effect; is this a typo?", report.Fixes(
				edit.Fix("turn into single negation", edit.ReplaceWithNode(pass.Fset, node, m.State["single"].(ast.Node))),
				edit.Fix("remove double negation", edit.ReplaceWithNode(pass.Fset, node, m.State["x"].(ast.Node)))))
		}
	}
	code.Preorder(pass, fn, (*ast.UnaryExpr)(nil))
	return nil, nil
}
