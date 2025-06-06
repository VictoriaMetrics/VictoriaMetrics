package s1028

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1028",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:   `Simplify error construction with \'fmt.Errorf\'`,
		Before:  `errors.New(fmt.Sprintf(...))`,
		After:   `fmt.Errorf(...)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkErrorsNewSprintfQ = pattern.MustParse(`(CallExpr (Symbol "errors.New") [(CallExpr (Symbol "fmt.Sprintf") args)])`)
	checkErrorsNewSprintfR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "fmt") (Ident "Errorf")) args)`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		if _, edits, ok := code.MatchAndEdit(pass, checkErrorsNewSprintfQ, checkErrorsNewSprintfR, node); ok {
			// TODO(dh): the suggested fix may leave an unused import behind
			report.Report(pass, node, "should use fmt.Errorf(...) instead of errors.New(fmt.Sprintf(...))",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("use fmt.Errorf", edits...)))
		}
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
