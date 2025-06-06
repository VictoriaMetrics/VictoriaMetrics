package sa4000

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4000",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Binary operator has identical expressions on both sides`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	var isFloat func(T types.Type) bool
	isFloat = func(T types.Type) bool {
		tset := typeutil.NewTypeSet(T)
		if len(tset.Terms) == 0 {
			// no terms, so floats are a possibility
			return true
		}
		return tset.Any(func(term *types.Term) bool {
			switch typ := term.Type().Underlying().(type) {
			case *types.Basic:
				kind := typ.Kind()
				return kind == types.Float32 || kind == types.Float64
			case *types.Array:
				return isFloat(typ.Elem())
			case *types.Struct:
				for i := 0; i < typ.NumFields(); i++ {
					if !isFloat(typ.Field(i).Type()) {
						return false
					}
				}
				return true
			default:
				return false
			}
		})
	}

	// TODO(dh): this check ignores the existence of side-effects and
	// happily flags fn() == fn() – so far, we've had nobody complain
	// about a false positive, and it's caught several bugs in real
	// code.
	//
	// We special case functions from the math/rand package. Someone ran
	// into the following false positive: "rand.Intn(2) - rand.Intn(2), which I wrote to generate values {-1, 0, 1} with {0.25, 0.5, 0.25} probability."
	fn := func(node ast.Node) {
		op := node.(*ast.BinaryExpr)
		switch op.Op {
		case token.EQL, token.NEQ:
		case token.SUB, token.QUO, token.AND, token.REM, token.OR, token.XOR, token.AND_NOT,
			token.LAND, token.LOR, token.LSS, token.GTR, token.LEQ, token.GEQ:
		default:
			// For some ops, such as + and *, it can make sense to
			// have identical operands
			return
		}

		if isFloat(pass.TypesInfo.TypeOf(op.X)) {
			// 'float <op> float' makes sense for several operators.
			// We've tried keeping an exact list of operators to allow, but floats keep surprising us. Let's just give up instead.
			return
		}

		if reflect.TypeOf(op.X) != reflect.TypeOf(op.Y) {
			return
		}
		if report.Render(pass, op.X) != report.Render(pass, op.Y) {
			return
		}
		l1, ok1 := op.X.(*ast.BasicLit)
		l2, ok2 := op.Y.(*ast.BasicLit)
		if ok1 && ok2 && l1.Kind == token.INT && l2.Kind == l1.Kind && l1.Value == "0" && l2.Value == l1.Value && code.IsGenerated(pass, l1.Pos()) {
			// cgo generates the following function call:
			// _cgoCheckPointer(_cgoBase0, 0 == 0) – it uses 0 == 0
			// instead of true in case the user shadowed the
			// identifier. Ideally we'd restrict this exception to
			// calls of _cgoCheckPointer, but it's not worth the
			// hassle of keeping track of the stack. <lit> <op> <lit>
			// are very rare to begin with, and we're mostly checking
			// for them to catch typos such as 1 == 1 where the user
			// meant to type i == 1. The odds of a false negative for
			// 0 == 0 are slim.
			return
		}

		if expr, ok := op.X.(*ast.CallExpr); ok {
			call := code.CallName(pass, expr)
			switch call {
			case "math/rand.Int",
				"math/rand.Int31",
				"math/rand.Int31n",
				"math/rand.Int63",
				"math/rand.Int63n",
				"math/rand.Intn",
				"math/rand.Uint32",
				"math/rand.Uint64",
				"math/rand.ExpFloat64",
				"math/rand.Float32",
				"math/rand.Float64",
				"math/rand.NormFloat64",
				"(*math/rand.Rand).Int",
				"(*math/rand.Rand).Int31",
				"(*math/rand.Rand).Int31n",
				"(*math/rand.Rand).Int63",
				"(*math/rand.Rand).Int63n",
				"(*math/rand.Rand).Intn",
				"(*math/rand.Rand).Uint32",
				"(*math/rand.Rand).Uint64",
				"(*math/rand.Rand).ExpFloat64",
				"(*math/rand.Rand).Float32",
				"(*math/rand.Rand).Float64",
				"(*math/rand.Rand).NormFloat64":
				return
			}
		}

		report.Report(pass, op, fmt.Sprintf("identical expressions on the left and right side of the '%s' operator", op.Op))
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
