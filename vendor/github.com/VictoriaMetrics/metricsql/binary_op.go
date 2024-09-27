package metricsql

import (
	"fmt"
	"math"
	"strings"

	"github.com/VictoriaMetrics/metricsql/binaryop"
)

var binaryOps = map[string]bool{
	"+": true,
	"-": true,
	"*": true,
	"/": true,
	"%": true,
	"^": true,

	// See https://github.com/prometheus/prometheus/pull/9248
	"atan2": true,

	// cmp ops
	"==": true,
	"!=": true,
	">":  true,
	"<":  true,
	">=": true,
	"<=": true,

	// logical set ops
	"and":    true,
	"or":     true,
	"unless": true,

	// New ops for MetricsQL
	"if":      true,
	"ifnot":   true,
	"default": true,
}

var binaryOpPriorities = map[string]int{
	"default": -1,

	"if":    0,
	"ifnot": 0,

	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operator-precedence
	"or": 1,

	"and":    2,
	"unless": 2,

	"==": 3,
	"!=": 3,
	"<":  3,
	">":  3,
	"<=": 3,
	">=": 3,

	"+": 4,
	"-": 4,

	"*":     5,
	"/":     5,
	"%":     5,
	"atan2": 5,

	"^": 6,
}

func isBinaryOp(op string) bool {
	op = strings.ToLower(op)
	return binaryOps[op]
}

func binaryOpPriority(op string) int {
	op = strings.ToLower(op)
	return binaryOpPriorities[op]
}

func scanBinaryOpPrefix(s string) int {
	n := 0
	for op := range binaryOps {
		if len(s) < len(op) {
			continue
		}
		ss := strings.ToLower(s[:len(op)])
		if ss == op && len(op) > n {
			n = len(op)
		}
	}
	return n
}

func isRightAssociativeBinaryOp(op string) bool {
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#binary-operator-precedence
	return op == "^"
}

func isBinaryOpGroupModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	// See https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching
	case "on", "ignoring":
		return true
	default:
		return false
	}
}

func isBinaryOpJoinModifier(s string) bool {
	s = strings.ToLower(s)
	switch s {
	case "group_left", "group_right":
		return true
	default:
		return false
	}
}

func isBinaryOpBoolModifier(s string) bool {
	s = strings.ToLower(s)
	return s == "bool"
}

// IsBinaryOpCmp returns true if op is comparison operator such as '==', '!=', etc.
func IsBinaryOpCmp(op string) bool {
	switch op {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

func isBinaryOpLogicalSet(op string) bool {
	op = strings.ToLower(op)
	switch op {
	case "and", "or", "unless":
		return true
	default:
		return false
	}
}

func binaryOpEvalNumber(op string, left, right float64, isBool bool) float64 {
	op = strings.ToLower(op)
	if IsBinaryOpCmp(op) {
		evalCmp := func(cf func(left, right float64) bool) float64 {
			if isBool {
				if cf(left, right) {
					return 1
				}
				return 0
			}
			if cf(left, right) {
				return left
			}
			return nan
		}
		switch op {
		case "==":
			left = evalCmp(binaryop.Eq)
		case "!=":
			left = evalCmp(binaryop.Neq)
		case ">":
			left = evalCmp(binaryop.Gt)
		case "<":
			left = evalCmp(binaryop.Lt)
		case ">=":
			left = evalCmp(binaryop.Gte)
		case "<=":
			left = evalCmp(binaryop.Lte)
		default:
			panic(fmt.Errorf("BUG: unexpected comparison binaryOp: %q", op))
		}
	} else {
		switch op {
		case "+":
			left = binaryop.Plus(left, right)
		case "-":
			left = binaryop.Minus(left, right)
		case "*":
			left = binaryop.Mul(left, right)
		case "/":
			left = binaryop.Div(left, right)
		case "%":
			left = binaryop.Mod(left, right)
		case "atan2":
			left = binaryop.Atan2(left, right)
		case "^":
			left = binaryop.Pow(left, right)
		case "and":
			left = binaryop.And(left, right)
		case "or":
			left = binaryop.Or(left, right)
		case "unless":
			left = nan
		case "default":
			left = binaryop.Default(left, right)
		case "if":
			left = binaryop.If(left, right)
		case "ifnot":
			left = binaryop.Ifnot(left, right)
		default:
			panic(fmt.Errorf("BUG: unexpected non-comparison binaryOp: %q", op))
		}
	}
	return left
}

var nan = math.NaN()
