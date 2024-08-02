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
		if expr.Window != nil {
			VisitAll(expr.Window, f)
		}
		if expr.Step != nil {
			VisitAll(expr.Step, f)
		}
		if expr.Offset != nil {
			VisitAll(expr.Offset, f)
		}
		if expr.At != nil {
			VisitAll(expr.At, f)
		}
	}
	f(e)
}

// IsLikelyInvalid returns true if e contains tricky implicit conversion, which is invalid most of the time.
//
// Examples of invalid expressions:
//
//	rate(sum(foo))
//	rate(abs(foo))
//	rate(foo + bar)
//	rate(foo > 10)
//
// These expressions are implicitly converted into another expressions, which returns unexpected results most of the time:
//
//	rate(default_rollup(sum(foo))[1i:1i])
//	rate(default_rollup(abs(foo))[1i:1i])
//	rate(default_rollup(foo + bar)[1i:1i])
//	rate(default_rollup(foo > 10)[1i:1i])
//
// See https://docs.victoriametrics.com/metricsql/#implicit-query-conversions
//
// Note that rate(foo) is valid expression, since it returns the expected results most of the time, e.g. rate(foo[1i]).
func IsLikelyInvalid(e Expr) bool {
	hasImplicitConversion := false
	VisitAll(e, func(expr Expr) {
		if hasImplicitConversion {
			return
		}
		fe, ok := expr.(*FuncExpr)
		if !ok {
			return
		}
		idx := GetRollupArgIdx(fe)
		if idx < 0 {
			return
		}
		arg := fe.Args[idx]
		re, ok := arg.(*RollupExpr)
		if !ok {
			if _, ok = arg.(*MetricExpr); !ok {
				hasImplicitConversion = true
			}
			return
		}
		if _, ok := re.Expr.(*MetricExpr); ok {
			return
		}
		if re.Window == nil {
			hasImplicitConversion = true
		}
	})
	return hasImplicitConversion
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
		fe, ok := expr.(*FuncExpr)
		if !ok {
			return
		}
		if !IsRollupFunc(fe.Name) && !IsTransformFunc(fe.Name) {
			err = fmt.Errorf("unsupported function %q", fe.Name)
		}
	})
	return err
}
