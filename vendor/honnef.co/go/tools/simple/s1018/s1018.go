package s1018

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1018",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Use \"copy\" for sliding elements`,
		Text: `\'copy()\' permits using the same source and destination slice, even with
overlapping ranges. This makes it ideal for sliding elements in a
slice.`,

		Before: `
for i := 0; i < n; i++ {
    bs[i] = bs[offset+i]
}`,
		After:   `copy(bs[:n], bs[offset:])`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkLoopSlideQ = pattern.MustParse(`
		(ForStmt
			(AssignStmt initvar@(Ident _) _ (IntegerLiteral "0"))
			(BinaryExpr initvar "<" limit@(Ident _))
			(IncDecStmt initvar "++")
			[(AssignStmt
				(IndexExpr slice@(Ident _) initvar)
				"="
				(IndexExpr slice (BinaryExpr offset@(Ident _) "+" initvar)))])`)
	checkLoopSlideR = pattern.MustParse(`
		(CallExpr
			(Ident "copy")
			[(SliceExpr slice nil limit nil)
				(SliceExpr slice offset nil nil)])`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	// TODO(dh): detect bs[i+offset] in addition to bs[offset+i]
	// TODO(dh): consider merging this function with LintLoopCopy
	// TODO(dh): detect length that is an expression, not a variable name
	// TODO(dh): support sliding to a different offset than the beginning of the slice

	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		m, edits, ok := code.MatchAndEdit(pass, checkLoopSlideQ, checkLoopSlideR, loop)
		if !ok {
			return
		}
		typ := pass.TypesInfo.TypeOf(m.State["slice"].(*ast.Ident))
		// The pattern probably needs a core type, but All is fine, too. Either way we only accept slices.
		if !typeutil.All(typ, typeutil.IsSlice) {
			return
		}

		report.Report(pass, loop, "should use copy() instead of loop for sliding slice elements",
			report.ShortRange(),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("use copy() instead of loop", edits...)))
	}
	code.Preorder(pass, fn, (*ast.ForStmt)(nil))
	return nil, nil
}
