package sa9008

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9008",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `\'else\' branch of a type assertion is probably not reading the right value`,
		Text: `
When declaring variables as part of an \'if\' statement (like in \"if
foo := ...; foo {\"), the same variables will also be in the scope of
the \'else\' branch. This means that in the following example

    if x, ok := x.(int); ok {
        // ...
    } else {
        fmt.Printf("unexpected type %T", x)
    }

\'x\' in the \'else\' branch will refer to the \'x\' from \'x, ok
:=\'; it will not refer to the \'x\' that is being type-asserted. The
result of a failed type assertion is the zero value of the type that
is being asserted to, so \'x\' in the else branch will always have the
value \'0\' and the type \'int\'.
`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var typeAssertionShadowingElseQ = pattern.MustParse(`(IfStmt (AssignStmt [obj@(Ident _) ok@(Ident _)] ":=" assert@(TypeAssertExpr obj _)) ok _ elseBranch)`)

func run(pass *analysis.Pass) (interface{}, error) {
	// TODO(dh): without the IR-based verification, this check is able
	// to find more bugs, but also more prone to false positives. It
	// would be a good candidate for the 'codereview' category of
	// checks.

	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg
	fn := func(node ast.Node) {
		m, ok := code.Match(pass, typeAssertionShadowingElseQ, node)
		if !ok {
			return
		}
		shadow := pass.TypesInfo.ObjectOf(m.State["obj"].(*ast.Ident))
		shadowed := m.State["assert"].(*ast.TypeAssertExpr).X

		path, exact := astutil.PathEnclosingInterval(code.File(pass, shadow), shadow.Pos(), shadow.Pos())
		if !exact {
			// TODO(dh): when can this happen?
			return
		}
		irfn := ir.EnclosingFunction(irpkg, path)
		if irfn == nil {
			// For example for functions named "_", because we don't generate IR for them.
			return
		}

		shadoweeIR, isAddr := irfn.ValueForExpr(m.State["obj"].(*ast.Ident))
		if shadoweeIR == nil || isAddr {
			// TODO(dh): is this possible?
			return
		}

		var branch ast.Node
		switch br := m.State["elseBranch"].(type) {
		case ast.Node:
			branch = br
		case []ast.Stmt:
			branch = &ast.BlockStmt{List: br}
		case nil:
			return
		default:
			panic(fmt.Sprintf("unexpected type %T", br))
		}

		ast.Inspect(branch, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			if pass.TypesInfo.ObjectOf(ident) != shadow {
				return true
			}

			v, isAddr := irfn.ValueForExpr(ident)
			if v == nil || isAddr {
				return true
			}
			if irutil.Flatten(v) != shadoweeIR {
				// Same types.Object, but different IR value. This
				// either means that the variable has been
				// assigned to since the type assertion, or that
				// the variable has escaped to the heap. Either
				// way, we shouldn't flag reads of it.
				return true
			}

			report.Report(pass, ident,
				fmt.Sprintf("%s refers to the result of a failed type assertion and is a zero value, not the value that was being type-asserted", report.Render(pass, ident)),
				report.Related(shadow, "this is the variable being read"),
				report.Related(shadowed, "this is the variable being shadowed"))
			return true
		})
	}
	code.Preorder(pass, fn, (*ast.IfStmt)(nil))
	return nil, nil
}
