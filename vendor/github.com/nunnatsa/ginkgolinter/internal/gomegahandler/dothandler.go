package gomegahandler

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
)

// dotHandler is used when importing gomega with dot; i.e.
// import . "github.com/onsi/gomega"
type dotHandler struct {
	pass *analysis.Pass
}

// GetGomegaBasicInfo returns the name of the gomega function, e.g. `Expect` + some additional info
func (h dotHandler) GetGomegaBasicInfo(expr *ast.CallExpr) (*GomegaBasicInfo, bool) {
	info := &GomegaBasicInfo{}
	for {
		switch actualFunc := expr.Fun.(type) {
		case *ast.Ident:
			info.MethodName = actualFunc.Name
			return info, true
		case *ast.SelectorExpr:
			if h.isGomegaVar(actualFunc.X) {
				info.UseGomegaVar = true
				info.MethodName = actualFunc.Sel.Name
				return info, true
			}

			if actualFunc.Sel.Name == "Error" {
				info.HasErrorMethod = true
			}

			if x, ok := actualFunc.X.(*ast.CallExpr); ok {
				expr = x
			} else {
				return nil, false
			}
		default:
			return nil, false
		}
	}
}

// ReplaceFunction replaces the function with another one, for fix suggestions
func (dotHandler) ReplaceFunction(caller *ast.CallExpr, newExpr *ast.Ident) {
	switch f := caller.Fun.(type) {
	case *ast.Ident:
		caller.Fun = newExpr
	case *ast.SelectorExpr:
		f.Sel = newExpr
	}
}

func (dotHandler) GetNewWrapperMatcher(name string, existing *ast.CallExpr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  ast.NewIdent(name),
		Args: []ast.Expr{existing},
	}
}

func (h dotHandler) GetActualExpr(assertionFunc *ast.SelectorExpr) *ast.CallExpr {
	actualExpr, ok := assertionFunc.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	switch fun := actualExpr.Fun.(type) {
	case *ast.Ident:
		return actualExpr
	case *ast.SelectorExpr:
		if gomegainfo.IsActualMethod(fun.Sel.Name) {
			if h.isGomegaVar(fun.X) {
				return actualExpr
			}
		} else {
			return h.GetActualExpr(fun)
		}
	}
	return nil
}

func (h dotHandler) GetActualExprClone(origFunc, funcClone *ast.SelectorExpr) *ast.CallExpr {
	actualExpr, ok := funcClone.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	switch funClone := actualExpr.Fun.(type) {
	case *ast.Ident:
		return actualExpr
	case *ast.SelectorExpr:
		origFun := origFunc.X.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
		if gomegainfo.IsActualMethod(funClone.Sel.Name) {
			if h.isGomegaVar(origFun.X) {
				return actualExpr
			}
		} else {
			return h.GetActualExprClone(origFun, funClone)
		}
	}
	return nil
}

func (h dotHandler) isGomegaVar(x ast.Expr) bool {
	return gomegainfo.IsGomegaVar(x, h.pass)
}
