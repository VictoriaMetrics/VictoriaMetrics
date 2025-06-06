package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// NegativePositive detects situations like
//
//	assert.Less(t, a, 0)
//	assert.Greater(t, 0, a)
//	assert.True(t, a < 0)
//	assert.True(t, 0 > a)
//	assert.False(t, a >= 0)
//	assert.False(t, 0 <= a)
//
//	assert.Greater(t, a, 0)
//	assert.Less(t, 0, a)
//	assert.True(t, a > 0)
//	assert.True(t, 0 < a)
//	assert.False(t, a <= 0)
//	assert.False(t, 0 >= a)
//
// and requires
//
//	assert.Negative(t, value)
//	assert.Positive(t, value)
//
// Typed zeros (like `int8(0)`, ..., `uint64(0)`) are also supported.
type NegativePositive struct{}

// NewNegativePositive constructs NegativePositive checker.
func NewNegativePositive() NegativePositive { return NegativePositive{} }
func (NegativePositive) Name() string       { return "negative-positive" }

func (checker NegativePositive) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if d := checker.checkNegative(pass, call); d != nil {
		return d
	}
	return checker.checkPositive(pass, call)
}

func (checker NegativePositive) checkNegative(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	newUseNegativeDiagnostic := func(replaceStart, replaceEnd token.Pos, replaceWith ast.Expr) *analysis.Diagnostic {
		const proposed = "Negative"
		return newUseFunctionDiagnostic(checker.Name(), call, proposed,
			analysis.TextEdit{
				Pos:     replaceStart,
				End:     replaceEnd,
				NewText: analysisutil.NodeBytes(pass.Fset, replaceWith),
			})
	}

	// NOTE(a.telyshev): We ignore uint-asserts as being no sense for assert.Negative.

	switch call.Fn.NameFTrimmed {
	case "Less":
		if len(call.Args) < 2 {
			return nil
		}
		a, b := call.Args[0], call.Args[1]

		if isSignedNotZero(pass, a) && isZeroOrSignedZero(b) {
			return newUseNegativeDiagnostic(a.Pos(), b.End(), untype(a))
		}

	case "Greater":
		if len(call.Args) < 2 {
			return nil
		}
		a, b := call.Args[0], call.Args[1]

		if isZeroOrSignedZero(a) && isSignedNotZero(pass, b) {
			return newUseNegativeDiagnostic(a.Pos(), b.End(), untype(b))
		}

	case "True":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		a, _, ok1 := isStrictComparisonWith(pass, expr, isSignedNotZero, token.LSS, p(isZeroOrSignedZero)) // a < 0
		_, b, ok2 := isStrictComparisonWith(pass, expr, p(isZeroOrSignedZero), token.GTR, isSignedNotZero) // 0 > a

		survivingArg, ok := anyVal([]bool{ok1, ok2}, a, b)
		if ok {
			return newUseNegativeDiagnostic(expr.Pos(), expr.End(), untype(survivingArg))
		}

	case "False":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		a, _, ok1 := isStrictComparisonWith(pass, expr, isSignedNotZero, token.GEQ, p(isZeroOrSignedZero)) // a >= 0
		_, b, ok2 := isStrictComparisonWith(pass, expr, p(isZeroOrSignedZero), token.LEQ, isSignedNotZero) // 0 <= a

		survivingArg, ok := anyVal([]bool{ok1, ok2}, a, b)
		if ok {
			return newUseNegativeDiagnostic(expr.Pos(), expr.End(), untype(survivingArg))
		}
	}
	return nil
}

func (checker NegativePositive) checkPositive(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	newUsePositiveDiagnostic := func(replaceStart, replaceEnd token.Pos, replaceWith ast.Expr) *analysis.Diagnostic {
		const proposed = "Positive"
		return newUseFunctionDiagnostic(checker.Name(), call, proposed,
			analysis.TextEdit{
				Pos:     replaceStart,
				End:     replaceEnd,
				NewText: analysisutil.NodeBytes(pass.Fset, replaceWith),
			})
	}

	switch call.Fn.NameFTrimmed {
	case "Greater":
		if len(call.Args) < 2 {
			return nil
		}
		a, b := call.Args[0], call.Args[1]

		if isNotAnyZero(a) && isAnyZero(b) {
			return newUsePositiveDiagnostic(a.Pos(), b.End(), untype(a))
		}

	case "Less":
		if len(call.Args) < 2 {
			return nil
		}
		a, b := call.Args[0], call.Args[1]

		if isAnyZero(a) && isNotAnyZero(b) {
			return newUsePositiveDiagnostic(a.Pos(), b.End(), untype(b))
		}

	case "True":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		a, _, ok1 := isStrictComparisonWith(pass, expr, p(isNotAnyZero), token.GTR, p(isAnyZero)) // a > 0
		_, b, ok2 := isStrictComparisonWith(pass, expr, p(isAnyZero), token.LSS, p(isNotAnyZero)) // 0 < a

		survivingArg, ok := anyVal([]bool{ok1, ok2}, a, b)
		if ok {
			return newUsePositiveDiagnostic(expr.Pos(), expr.End(), untype(survivingArg))
		}

	case "False":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		a, _, ok1 := isStrictComparisonWith(pass, expr, p(isNotAnyZero), token.LEQ, p(isAnyZero)) // a <= 0
		_, b, ok2 := isStrictComparisonWith(pass, expr, p(isAnyZero), token.GEQ, p(isNotAnyZero)) // 0 >= a

		survivingArg, ok := anyVal([]bool{ok1, ok2}, a, b)
		if ok {
			return newUsePositiveDiagnostic(expr.Pos(), expr.End(), untype(survivingArg))
		}
	}
	return nil
}
