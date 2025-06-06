package checkers

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

var lenObj = types.Universe.Lookup("len")

func isLenEquality(pass *analysis.Pass, e ast.Expr) (ast.Expr, ast.Expr, bool) {
	be, ok := e.(*ast.BinaryExpr)
	if !ok {
		return nil, nil, false
	}

	if be.Op != token.EQL {
		return nil, nil, false
	}
	return xorLenCall(pass, be.X, be.Y)
}

func xorLenCall(pass *analysis.Pass, a, b ast.Expr) (lenArg ast.Expr, expectedLen ast.Expr, ok bool) {
	arg1, ok1 := isBuiltinLenCall(pass, a)
	arg2, ok2 := isBuiltinLenCall(pass, b)

	if xor(ok1, ok2) {
		if ok1 {
			return arg1, b, true
		}
		return arg2, a, true
	}
	return nil, nil, false
}

func isLenCallAndZero(pass *analysis.Pass, a, b ast.Expr) (ast.Expr, bool) {
	lenArg, ok := isBuiltinLenCall(pass, a)
	return lenArg, ok && isZero(b)
}

func isBuiltinLenCall(pass *analysis.Pass, e ast.Expr) (ast.Expr, bool) {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}

	if analysisutil.IsObj(pass.TypesInfo, ce.Fun, lenObj) && len(ce.Args) == 1 {
		return ce.Args[0], true
	}
	return nil, false
}
