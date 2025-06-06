package intervals

import (
	"go/ast"
	"go/constant"
	"go/token"
	gotypes "go/types"
	"time"

	"golang.org/x/tools/go/analysis"
)

func GetDuration(pass *analysis.Pass, argOffset int, origInterval, intervalClone ast.Expr, timePkg string) DurationValue {
	tv := pass.TypesInfo.Types[origInterval]
	argType := tv.Type
	if durType, ok := argType.(*gotypes.Named); ok {
		if durType.String() == "time.Duration" {
			if tv.Value != nil {
				if val, ok := constant.Int64Val(tv.Value); ok {
					return &RealDurationValue{
						dur:  time.Duration(val),
						expr: intervalClone,
					}
				}
			}
			return &UnknownDurationTypeValue{
				expr: intervalClone,
			}
		}
	}

	if basic, ok := argType.(*gotypes.Basic); ok && tv.Value != nil {
		if basic.Info()&gotypes.IsInteger != 0 {
			if num, ok := constant.Int64Val(tv.Value); ok {
				return &NumericDurationValue{
					timePkg:    timePkg,
					numSeconds: num,
					offset:     argOffset,
					dur:        time.Duration(num) * time.Second,
					expr:       intervalClone,
				}
			}
		}

		if basic.Info()&gotypes.IsFloat != 0 {
			if num, ok := constant.Float64Val(tv.Value); ok {
				return &NumericDurationValue{
					timePkg:    timePkg,
					numSeconds: int64(num),
					offset:     argOffset,
					dur:        time.Duration(num) * time.Second,
					expr:       intervalClone,
				}
			}
		}
	}

	return &UnknownDurationValue{expr: intervalClone}
}

func GetDurationFromValue(pass *analysis.Pass, orig, clone ast.Expr) DurationValue {
	tv := pass.TypesInfo.Types[orig]
	interval := tv.Value
	if interval != nil {
		if val, ok := constant.Int64Val(interval); ok {
			return RealDurationValue{
				dur:  time.Duration(val),
				expr: orig,
			}
		}
	}
	return UnknownDurationTypeValue{expr: clone}
}

type DurationValue interface {
	Duration() time.Duration
}

type NumericValue interface {
	GetOffset() int
	GetDurationExpr() ast.Expr
}
type RealDurationValue struct {
	dur  time.Duration
	expr ast.Expr
}

func (r RealDurationValue) Duration() time.Duration {
	return r.dur
}

type NumericDurationValue struct {
	timePkg    string
	numSeconds int64
	offset     int
	dur        time.Duration
	expr       ast.Expr
}

func (r *NumericDurationValue) Duration() time.Duration {
	return r.dur
}

func (r *NumericDurationValue) GetOffset() int {
	return r.offset
}

func (r *NumericDurationValue) GetDurationExpr() ast.Expr {
	var newArg ast.Expr
	second := &ast.SelectorExpr{
		Sel: ast.NewIdent("Second"),
		X:   ast.NewIdent(r.timePkg),
	}

	if r.numSeconds == 1 {
		newArg = second
	} else {
		newArg = &ast.BinaryExpr{
			X:  second,
			Op: token.MUL,
			Y:  r.expr,
		}
	}

	return newArg
}

type UnknownDurationValue struct {
	expr ast.Expr
}

func (r UnknownDurationValue) Duration() time.Duration {
	return 0
}

type UnknownNumericValue struct {
	expr   ast.Expr
	offset int
}

func (r UnknownNumericValue) Duration() time.Duration {
	return 0
}

func (r UnknownNumericValue) GetDurationExpr() ast.Expr {
	return &ast.BinaryExpr{
		X: &ast.SelectorExpr{
			Sel: ast.NewIdent("Second"),
			X:   ast.NewIdent("time"),
		},
		Op: token.MUL,
		Y:  r.expr,
	}
}

func (r UnknownNumericValue) GetOffset() int {
	return r.offset
}

type UnknownDurationTypeValue struct {
	expr ast.Expr
}

func (r UnknownDurationTypeValue) Duration() time.Duration {
	return 0
}
