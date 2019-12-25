package binaryop

import (
	"math"
)

var nan = math.NaN()

// Eq returns true of left == right.
func Eq(left, right float64) bool {
	// Special handling for nan == nan.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150 .
	if math.IsNaN(left) {
		return math.IsNaN(right)
	}
	return left == right
}

// Neq returns true of left != right.
func Neq(left, right float64) bool {
	// Special handling for comparison with nan.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/150 .
	if math.IsNaN(left) {
		return !math.IsNaN(right)
	}
	if math.IsNaN(right) {
		return true
	}
	return left != right
}

// Gt returns true of left > right
func Gt(left, right float64) bool {
	return left > right
}

// Lt returns true if left < right
func Lt(left, right float64) bool {
	return left < right
}

// Gte returns true if left >= right
func Gte(left, right float64) bool {
	return left >= right
}

// Lte returns true if left <= right
func Lte(left, right float64) bool {
	return left <= right
}

// Plus returns left + right
func Plus(left, right float64) float64 {
	return left + right
}

// Minus returns left - right
func Minus(left, right float64) float64 {
	return left - right
}

// Mul returns left * right
func Mul(left, right float64) float64 {
	return left * right
}

// Div returns left / right
func Div(left, right float64) float64 {
	return left / right
}

// Mod returns mod(left, right)
func Mod(left, right float64) float64 {
	return math.Mod(left, right)
}

// Pow returns pow(left, right)
func Pow(left, right float64) float64 {
	return math.Pow(left, right)
}

// Default returns left or right if left is NaN.
func Default(left, right float64) float64 {
	if math.IsNaN(left) {
		return right
	}
	return left
}

// If returns left if right is not NaN. Otherwise NaN is returned.
func If(left, right float64) float64 {
	if math.IsNaN(right) {
		return nan
	}
	return left
}

// Ifnot returns left if right is NaN. Otherwise NaN is returned.
func Ifnot(left, right float64) float64 {
	if math.IsNaN(right) {
		return left
	}
	return nan
}
