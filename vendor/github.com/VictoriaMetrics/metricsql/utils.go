package metricsql

// ExpandWithExprs expands WITH expressions inside q and returns the resulting
// PromQL without WITH expressions.
func ExpandWithExprs(q string) (string, error) {
	e, err := Parse(q)
	if err != nil {
		return "", err
	}
	buf := e.AppendString(nil)
	return string(buf), nil
}

// VisitAll recursively calls f for all the Expr children in e.
//
// It visits leaf children at first and then visits parent nodes.
// It is safe modifying expr in f.
func VisitAll(e Expr, f func(expr Expr)) {
	switch expr := e.(type) {
	case *BinaryOpExpr:
		VisitAll(expr.Left, f)
		VisitAll(expr.Right, f)
		VisitAll(&expr.GroupModifier, f)
		VisitAll(&expr.JoinModifier, f)
	case *FuncExpr:
		for _, arg := range expr.Args {
			VisitAll(arg, f)
		}
	case *AggrFuncExpr:
		for _, arg := range expr.Args {
			VisitAll(arg, f)
		}
		VisitAll(&expr.Modifier, f)
	case *RollupExpr:
		VisitAll(expr.Expr, f)
	}
	f(e)
}
