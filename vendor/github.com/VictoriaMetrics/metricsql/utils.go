package metricsql

import (
	"fmt"
	"strings"
)

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
// It is safe modifying e in f.
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

// IsSupportedFunction returns true if funcName contains supported MetricsQL function
func IsSupportedFunction(funcName string) bool {
	funcName = strings.ToLower(funcName)
	if IsRollupFunc(funcName) {
		return true
	}
	if IsTransformFunc(funcName) {
		return true
	}
	if IsAggrFunc(funcName) {
		return true
	}
	return false
}

func checkSupportedFunctions(e Expr) error {
	var err error
	VisitAll(e, func(expr Expr) {
		if err != nil {
			return
		}
		switch t := expr.(type) {
		case *FuncExpr:
			if !IsRollupFunc(t.Name) && !IsTransformFunc(t.Name) {
				err = fmt.Errorf("unsupported function %q", t.Name)
			}
		case *AggrFuncExpr:
			if !IsAggrFunc(t.Name) {
				err = fmt.Errorf("unsupported aggregate function %q", t.Name)
			}
		}
	})
	return err
}
