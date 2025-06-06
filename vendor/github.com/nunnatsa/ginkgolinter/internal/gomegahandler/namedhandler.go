package gomegahandler

import (
	"go/ast"

	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"

	"golang.org/x/tools/go/analysis"
)

// nameHandler is used when importing gomega without name; i.e.
// import "github.com/onsi/gomega"
//
// or with a custom name; e.g.
// import customname "github.com/onsi/gomega"
type nameHandler struct {
	name string
	pass *analysis.Pass
}

// GetGomegaBasicInfo returns the name of the gomega function, e.g. `Expect` + some additional info
func (g nameHandler) GetGomegaBasicInfo(expr *ast.CallExpr) (*GomegaBasicInfo, bool) {
	info := &GomegaBasicInfo{}
	for {
		selector, ok := expr.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}

		if selector.Sel.Name == "Error" {
			info.HasErrorMethod = true
		}

		switch x := selector.X.(type) {
		case *ast.Ident:
			if x.Name != g.name {
				if !g.isGomegaVar(x) {
					return nil, false
				}
				info.UseGomegaVar = true
			}

			info.MethodName = selector.Sel.Name

			return info, true

		case *ast.CallExpr:
			expr = x

		default:
			return nil, false
		}
	}
}

// ReplaceFunction replaces the function with another one, for fix suggestions
func (nameHandler) ReplaceFunction(caller *ast.CallExpr, newExpr *ast.Ident) {
	caller.Fun.(*ast.SelectorExpr).Sel = newExpr
}

func (g nameHandler) isGomegaVar(x ast.Expr) bool {
	return gomegainfo.IsGomegaVar(x, g.pass)
}

func (g nameHandler) GetActualExpr(assertionFunc *ast.SelectorExpr) *ast.CallExpr {
	actualExpr, ok := assertionFunc.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	switch fun := actualExpr.Fun.(type) {
	case *ast.Ident:
		return actualExpr
	case *ast.SelectorExpr:
		if x, ok := fun.X.(*ast.Ident); ok && x.Name == g.name {
			return actualExpr
		}
		if gomegainfo.IsActualMethod(fun.Sel.Name) {
			if g.isGomegaVar(fun.X) {
				return actualExpr
			}
		} else {
			return g.GetActualExpr(fun)
		}
	}
	return nil
}

func (g nameHandler) GetActualExprClone(origFunc, funcClone *ast.SelectorExpr) *ast.CallExpr {
	actualExpr, ok := funcClone.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	switch funClone := actualExpr.Fun.(type) {
	case *ast.Ident:
		return actualExpr
	case *ast.SelectorExpr:
		if x, ok := funClone.X.(*ast.Ident); ok && x.Name == g.name {
			return actualExpr
		}
		origFun := origFunc.X.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
		if gomegainfo.IsActualMethod(funClone.Sel.Name) {
			if g.isGomegaVar(origFun.X) {
				return actualExpr
			}
		} else {
			return g.GetActualExprClone(origFun, funClone)
		}

	}
	return nil
}

func (g nameHandler) GetNewWrapperMatcher(name string, existing *ast.CallExpr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(g.name),
			Sel: ast.NewIdent(name),
		},
		Args: []ast.Expr{existing},
	}
}
