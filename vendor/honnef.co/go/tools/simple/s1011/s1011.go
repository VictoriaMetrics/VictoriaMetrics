package s1011

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/facts/purity"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1011",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer, purity.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Use a single \'append\' to concatenate two slices`,
		Before: `
for _, e := range y {
    x = append(x, e)
}

for i := range y {
    x = append(x, y[i])
}

for i := range y {
    v := y[i]
    x = append(x, v)
}`,

		After: `
x = append(x, y...)
x = append(x, y...)
x = append(x, y...)`,
		Since: "2017.1",
		// MergeIfAll because y might not be a slice under all build tags.
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkLoopAppendQ = pattern.MustParse(`
(Or
	(RangeStmt
		(Ident "_")
		val@(Object _)
		_
		x
		[(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs val])])])
	(RangeStmt
		idx@(Object _)
		nil
		_
		x
		[(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs (IndexExpr x idx)])])])
	(RangeStmt
		idx@(Object _)
		nil
		_
		x
		[(AssignStmt val@(Object _) ":=" (IndexExpr x idx))
		(AssignStmt [lhs] "=" [(CallExpr (Builtin "append") [lhs val])])]))`)

func run(pass *analysis.Pass) (interface{}, error) {
	pure := pass.ResultOf[purity.Analyzer].(purity.Result)

	fn := func(node ast.Node) {
		m, ok := code.Match(pass, checkLoopAppendQ, node)
		if !ok {
			return
		}

		if val, ok := m.State["val"].(types.Object); ok && code.RefersTo(pass, m.State["lhs"].(ast.Expr), val) {
			return
		}

		if m.State["idx"] != nil && code.MayHaveSideEffects(pass, m.State["x"].(ast.Expr), pure) {
			// When using an index-based loop, x gets evaluated repeatedly and thus should be pure.
			// This doesn't matter for value-based loops, because x only gets evaluated once.
			return
		}

		if idx, ok := m.State["idx"].(types.Object); ok && code.RefersTo(pass, m.State["lhs"].(ast.Expr), idx) {
			// The lhs mustn't refer to the index loop variable.
			return
		}

		if code.MayHaveSideEffects(pass, m.State["lhs"].(ast.Expr), pure) {
			// The lhs may be dynamic and return different values on each iteration. For example:
			//
			// 	func bar() map[int][]int { /* return one of several maps */ }
			//
			// 	func foo(x []int, y [][]int) {
			// 		for i := range x {
			// 			bar()[0] = append(bar()[0], x[i])
			// 		}
			// 	}
			//
			// The dynamic nature of the lhs might also affect the value of the index.
			return
		}

		src := pass.TypesInfo.TypeOf(m.State["x"].(ast.Expr))
		dst := pass.TypesInfo.TypeOf(m.State["lhs"].(ast.Expr))
		if !types.Identical(src, dst) {
			return
		}

		r := &ast.AssignStmt{
			Lhs: []ast.Expr{m.State["lhs"].(ast.Expr)},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.Ident{Name: "append"},
					Args: []ast.Expr{
						m.State["lhs"].(ast.Expr),
						m.State["x"].(ast.Expr),
					},
					Ellipsis: 1,
				},
			},
		}

		report.Report(pass, node, fmt.Sprintf("should replace loop with %s", report.Render(pass, r)),
			report.ShortRange(),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("replace loop with call to append", edit.ReplaceWithNode(pass.Fset, node, r))))
	}
	code.Preorder(pass, fn, (*ast.RangeStmt)(nil))
	return nil, nil
}
