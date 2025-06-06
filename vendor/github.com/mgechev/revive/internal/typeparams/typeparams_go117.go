//go:build !go1.18
// +build !go1.18

package typeparams

import (
	"go/ast"
)

const enabled = false

func unpackIndexExpr(e ast.Expr) ast.Expr { return e }
