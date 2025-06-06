package s1012

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
		Name:     "S1012",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Replace \'time.Now().Sub(x)\' with \'time.Since(x)\'`,
		Text: `The \'time.Since\' helper has the same effect as using \'time.Now().Sub(x)\'
but is easier to read.`,
		Before:  `time.Now().Sub(x)`,
		After:   `time.Since(x)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkTimeSinceQ = pattern.MustParse(`(CallExpr (SelectorExpr (CallExpr (Symbol "time.Now") []) (Symbol "(time.Time).Sub")) [arg])`)
	checkTimeSinceR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "time") (Ident "Since")) [arg])`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		if _, edits, ok := code.MatchAndEdit(pass, checkTimeSinceQ, checkTimeSinceR, node); ok {
			report.Report(pass, node, "should use time.Since instead of time.Now().Sub",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("replace with call to time.Since", edits...)))
		}
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
