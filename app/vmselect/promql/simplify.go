package promql

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promql"
)

func simplifyConstants(e promql.Expr) promql.Expr {
	if re, ok := e.(*promql.RollupExpr); ok {
		re.Expr = simplifyConstants(re.Expr)
		return re
	}
	if ae, ok := e.(*promql.AggrFuncExpr); ok {
		simplifyConstantsInplace(ae.Args)
		return ae
	}
	if fe, ok := e.(*promql.FuncExpr); ok {
		simplifyConstantsInplace(fe.Args)
		return fe
	}
	if pe, ok := e.(*promql.ParensExpr); ok {
		if len(*pe) == 1 {
			return simplifyConstants((*pe)[0])
		}
		simplifyConstantsInplace(*pe)
		return pe
	}
	be, ok := e.(*promql.BinaryOpExpr)
	if !ok {
		return e
	}

	be.Left = simplifyConstants(be.Left)
	be.Right = simplifyConstants(be.Right)

	lne, ok := be.Left.(*promql.NumberExpr)
	if !ok {
		return be
	}
	rne, ok := be.Right.(*promql.NumberExpr)
	if !ok {
		return be
	}
	n := binaryOpConstants(be.Op, lne.N, rne.N, be.Bool)
	ne := &promql.NumberExpr{
		N: n,
	}
	return ne
}

func simplifyConstantsInplace(args []promql.Expr) {
	for i, arg := range args {
		args[i] = simplifyConstants(arg)
	}
}
