package promql

// A Visitor is used to walk a parsed query
type Visitor interface {
	Visit(expr Expr) Visitor
}

// Walk invokes Visit on v for each node in the parsed query tree
func Walk(expr Expr, v Visitor) {
	nv := v.Visit(expr)
	if nv == nil {
		return
	}
	switch t := expr.(type) {
	case *ParensExpr:
		for _, e := range *t {
			Walk(e, nv)
		}
	case *BinaryOpExpr:
		Walk(t.Left, nv)
		Walk(t.Right, nv)
	case *FuncExpr:
		for _, ae := range t.Args {
			Walk(ae, nv)
		}
	case *AggrFuncExpr:
		for _, ae := range t.Args {
			Walk(ae, nv)
		}
		Walk(&t.Modifier, nv)
	case *WithExpr:
		for _, wa := range t.Was {
			Walk(wa, nv)
		}
		Walk(t.Expr, nv)
	case *WithArgExpr:
		Walk(t.Expr, nv)
	case *RollupExpr:
		Walk(t.Expr, nv)
	case *MetricTemplateExpr:
		for _, tfe := range t.TagFilters {
			Walk(tfe, nv)
		}
	case *TagFilterExpr:
		Walk(t.Value, nv)
	}
}
