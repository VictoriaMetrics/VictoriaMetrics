package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// Empty detects situations like
//
//	assert.Len(t, arr, 0)
//	assert.Equal(t, 0, len(arr))
//	assert.EqualValues(t, 0, len(arr))
//	assert.Exactly(t, 0, len(arr))
//	assert.LessOrEqual(t, len(arr), 0)
//	assert.GreaterOrEqual(t, 0, len(arr))
//	assert.Less(t, len(arr), 0)
//	assert.Greater(t, 0, len(arr))
//	assert.Less(t, len(arr), 1)
//	assert.Greater(t, 1, len(arr))
//	assert.Zero(t, len(arr))
//	assert.Empty(t, len(arr))
//
//	assert.NotEqual(t, 0, len(arr))
//	assert.NotEqualValues(t, 0, len(arr))
//	assert.Less(t, 0, len(arr))
//	assert.Greater(t, len(arr), 0)
//	assert.Positive(t, len(arr))
//	assert.NotZero(t, len(arr))
//	assert.NotEmpty(t, len(arr))
//
// and requires
//
//	assert.Empty(t, arr)
//	assert.NotEmpty(t, arr)
type Empty struct{}

// NewEmpty constructs Empty checker.
func NewEmpty() Empty      { return Empty{} }
func (Empty) Name() string { return "empty" }

func (checker Empty) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if d := checker.checkEmpty(pass, call); d != nil {
		return d
	}
	return checker.checkNotEmpty(pass, call)
}

func (checker Empty) checkEmpty(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic { //nolint:gocognit
	newUseEmptyDiagnostic := func(replaceStart, replaceEnd token.Pos, replaceWith ast.Expr) *analysis.Diagnostic {
		const proposed = "Empty"
		return newUseFunctionDiagnostic(checker.Name(), call, proposed,
			analysis.TextEdit{
				Pos:     replaceStart,
				End:     replaceEnd,
				NewText: analysisutil.NodeBytes(pass.Fset, replaceWith),
			})
	}

	if len(call.Args) == 0 {
		return nil
	}
	a := call.Args[0]

	switch call.Fn.NameFTrimmed {
	case "Zero":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok {
			return newUseEmptyDiagnostic(a.Pos(), a.End(), lenArg)
		}

	case "Empty":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok {
			return newRemoveLenDiagnostic(pass, checker.Name(), call, a, lenArg)
		}
	}

	if len(call.Args) < 2 {
		return nil
	}
	b := call.Args[1]

	switch call.Fn.NameFTrimmed {
	case "Len":
		if isZero(b) {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), a)
		}

	case "Equal", "EqualValues", "Exactly":
		arg1, ok1 := isLenCallAndZero(pass, a, b)
		arg2, ok2 := isLenCallAndZero(pass, b, a)

		if lenArg, ok := anyVal([]bool{ok1, ok2}, arg1, arg2); ok {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "LessOrEqual":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok && isZero(b) {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "GreaterOrEqual":
		if lenArg, ok := isBuiltinLenCall(pass, b); ok && isZero(a) {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "Less":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok && (isOne(b) || isZero(b)) {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "Greater":
		if lenArg, ok := isBuiltinLenCall(pass, b); ok && (isOne(a) || isZero(a)) {
			return newUseEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}
	}
	return nil
}

func (checker Empty) checkNotEmpty(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic { //nolint:gocognit
	newUseNotEmptyDiagnostic := func(replaceStart, replaceEnd token.Pos, replaceWith ast.Expr) *analysis.Diagnostic {
		const proposed = "NotEmpty"
		return newUseFunctionDiagnostic(checker.Name(), call, proposed,
			analysis.TextEdit{
				Pos:     replaceStart,
				End:     replaceEnd,
				NewText: analysisutil.NodeBytes(pass.Fset, replaceWith),
			})
	}

	if len(call.Args) == 0 {
		return nil
	}
	a := call.Args[0]

	switch call.Fn.NameFTrimmed {
	case "NotZero", "Positive":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok {
			return newUseNotEmptyDiagnostic(a.Pos(), a.End(), lenArg)
		}

	case "NotEmpty":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok {
			return newRemoveLenDiagnostic(pass, checker.Name(), call, a, lenArg)
		}
	}

	if len(call.Args) < 2 {
		return nil
	}
	b := call.Args[1]

	switch call.Fn.NameFTrimmed {
	case "NotEqual", "NotEqualValues":
		arg1, ok1 := isLenCallAndZero(pass, a, b)
		arg2, ok2 := isLenCallAndZero(pass, b, a)

		if lenArg, ok := anyVal([]bool{ok1, ok2}, arg1, arg2); ok {
			return newUseNotEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "Less":
		if lenArg, ok := isBuiltinLenCall(pass, b); ok && isZero(a) {
			return newUseNotEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}

	case "Greater":
		if lenArg, ok := isBuiltinLenCall(pass, a); ok && isZero(b) {
			return newUseNotEmptyDiagnostic(a.Pos(), b.End(), lenArg)
		}
	}
	return nil
}
