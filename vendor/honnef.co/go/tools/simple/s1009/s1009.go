package s1009

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1009",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Omit redundant nil check on slices, maps, and channels`,
		Text: `The \'len\' function is defined for all slices, maps, and
channels, even nil ones, which have a length of zero. It is not necessary to
check for nil before checking that their length is not zero.`,
		Before:  `if x != nil && len(x) != 0 {}`,
		After:   `if len(x) != 0 {}`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

// run checks for the following redundant nil-checks:
//
//	if x == nil || len(x) == 0 {}
//	if x == nil || len(x) < N {} (where N != 0)
//	if x == nil || len(x) <= N {}
//	if x != nil && len(x) != 0 {}
//	if x != nil && len(x) == N {} (where N != 0)
//	if x != nil && len(x) > N {}
//	if x != nil && len(x) >= N {} (where N != 0)
func run(pass *analysis.Pass) (interface{}, error) {
	isConstZero := func(expr ast.Expr) (isConst bool, isZero bool) {
		_, ok := expr.(*ast.BasicLit)
		if ok {
			return true, code.IsIntegerLiteral(pass, expr, constant.MakeInt64(0))
		}
		id, ok := expr.(*ast.Ident)
		if !ok {
			return false, false
		}
		c, ok := pass.TypesInfo.ObjectOf(id).(*types.Const)
		if !ok {
			return false, false
		}
		return true, c.Val().Kind() == constant.Int && c.Val().String() == "0"
	}

	fn := func(node ast.Node) {
		// check that expr is "x || y" or "x && y"
		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.LOR && expr.Op != token.LAND {
			return
		}
		eqNil := expr.Op == token.LOR

		// check that x is "xx == nil" or "xx != nil"
		x, ok := expr.X.(*ast.BinaryExpr)
		if !ok {
			return
		}
		if eqNil && x.Op != token.EQL {
			return
		}
		if !eqNil && x.Op != token.NEQ {
			return
		}
		var xx *ast.Ident
		switch s := x.X.(type) {
		case *ast.Ident:
			xx = s
		case *ast.SelectorExpr:
			xx = s.Sel
		default:
			return
		}
		if !code.IsNil(pass, x.Y) {
			return
		}

		// check that y is "len(xx) == 0" or "len(xx) ... "
		y, ok := expr.Y.(*ast.BinaryExpr)
		if !ok {
			return
		}
		yx, ok := y.X.(*ast.CallExpr)
		if !ok {
			return
		}
		if !code.IsCallTo(pass, yx, "len") {
			return
		}
		var yxArg *ast.Ident
		switch s := yx.Args[knowledge.Arg("len.v")].(type) {
		case *ast.Ident:
			yxArg = s
		case *ast.SelectorExpr:
			yxArg = s.Sel
		default:
			return
		}
		if yxArg.Name != xx.Name {
			return
		}

		isConst, isZero := isConstZero(y.Y)
		if !isConst {
			return
		}

		if eqNil {
			switch y.Op {
			case token.EQL:
				// avoid false positive for "xx == nil || len(xx) == <non-zero>"
				if !isZero {
					return
				}
			case token.LEQ:
				// ok
			case token.LSS:
				// avoid false positive for "xx == nil || len(xx) < 0"
				if isZero {
					return
				}
			default:
				return
			}
		}

		if !eqNil {
			switch y.Op {
			case token.EQL:
				// avoid false positive for "xx != nil && len(xx) == 0"
				if isZero {
					return
				}
			case token.GEQ:
				// avoid false positive for "xx != nil && len(xx) >= 0"
				if isZero {
					return
				}
			case token.NEQ:
				// avoid false positive for "xx != nil && len(xx) != <non-zero>"
				if !isZero {
					return
				}
			case token.GTR:
				// ok
			default:
				return
			}
		}

		// finally check that xx type is one of array, slice, map or chan
		// this is to prevent false positive in case if xx is a pointer to an array
		typ := pass.TypesInfo.TypeOf(xx)
		var nilType string
		ok = typeutil.All(typ, func(term *types.Term) bool {
			switch term.Type().Underlying().(type) {
			case *types.Slice:
				nilType = "nil slices"
				return true
			case *types.Map:
				nilType = "nil maps"
				return true
			case *types.Chan:
				nilType = "nil channels"
				return true
			case *types.Pointer:
				return false
			case *types.TypeParam:
				return false
			default:
				lint.ExhaustiveTypeSwitch(term.Type().Underlying())
				return false
			}
		})
		if !ok {
			return
		}

		report.Report(pass, expr, fmt.Sprintf("should omit nil check; len() for %s is defined as zero", nilType), report.FilterGenerated())
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
