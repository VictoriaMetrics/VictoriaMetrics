package s1036

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
		Name:     "S1036",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Unnecessary guard around map access`,

		Text: `
When accessing a map key that doesn't exist yet, one receives a zero
value. Often, the zero value is a suitable value, for example when
using append or doing integer math.

The following

    if _, ok := m["foo"]; ok {
        m["foo"] = append(m["foo"], "bar")
    } else {
        m["foo"] = []string{"bar"}
    }

can be simplified to

    m["foo"] = append(m["foo"], "bar")

and

    if _, ok := m2["k"]; ok {
        m2["k"] += 4
    } else {
        m2["k"] = 4
    }

can be simplified to

    m["k"] += 4
`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkUnnecessaryGuardQ = pattern.MustParse(`
	(Or
		(IfStmt
			(AssignStmt [(Ident "_") ok@(Ident _)] ":=" indexexpr@(IndexExpr _ _))
			ok
			set@(AssignStmt indexexpr "=" (CallExpr (Builtin "append") indexexpr:values))
			(AssignStmt indexexpr "=" (CompositeLit _ values)))
		(IfStmt
			(AssignStmt [(Ident "_") ok] ":=" indexexpr@(IndexExpr _ _))
			ok
			set@(AssignStmt indexexpr "+=" value)
			(AssignStmt indexexpr "=" value))
		(IfStmt
			(AssignStmt [(Ident "_") ok] ":=" indexexpr@(IndexExpr _ _))
			ok
			set@(IncDecStmt indexexpr "++")
			(AssignStmt indexexpr "=" (IntegerLiteral "1"))))`)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		if m, ok := code.Match(pass, checkUnnecessaryGuardQ, node); ok {
			if code.MayHaveSideEffects(pass, m.State["indexexpr"].(ast.Expr), nil) {
				return
			}
			report.Report(pass, node, "unnecessary guard around map access",
				report.ShortRange(),
				report.Fixes(edit.Fix("simplify map access", edit.ReplaceWithNode(pass.Fset, node, m.State["set"].(ast.Node)))))
		}
	}
	code.Preorder(pass, fn, (*ast.IfStmt)(nil))
	return nil, nil
}
