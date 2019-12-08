package promql

import (
	"strings"
)

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

	"*": 5,
	"/": 5,
	"%": 5,

	"^": 6,
}

func isBinaryOp(op string) bool {
	op = strings.ToLower(op)
	_, ok := binaryOpPriorities[op]
	return ok
}

func binaryOpPriority(op string) int {
	op = strings.ToLower(op)
	return binaryOpPriorities[op]
}

func scanBinaryOpPrefix(s string) int {
	n := 0
	for op := range binaryOpPriorities {
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

func isBinaryOpCmp(op string) bool {
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
