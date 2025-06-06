//go:build go1.18
// +build go1.18

package typeparams

import (
	"go/ast"
)

const enabled = true

func unpackIndexExpr(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.IndexExpr:
		return e.X
	case *ast.IndexListExpr:
		return e.X
	}
	return e
}
