package checkers

import (
	"golang.org/x/tools/go/analysis"
)

// Len detects situations like
//
//	assert.Equal(t, 3, len(arr))
//	assert.EqualValues(t, 3, len(arr))
//	assert.Exactly(t, 3, len(arr))
//	assert.True(t, len(arr) == 3)
//
// and requires
//
//	assert.Len(t, arr, 3)
type Len struct{}

// NewLen constructs Len checker.
func NewLen() Len        { return Len{} }
func (Len) Name() string { return "len" }

func (checker Len) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	const proposedFn = "Len"

	switch call.Fn.NameFTrimmed {
	case "Equal", "EqualValues", "Exactly":
		if len(call.Args) < 2 {
			return nil
		}
		a, b := call.Args[0], call.Args[1]

		if lenArg, expectedLen, ok := xorLenCall(pass, a, b); ok {
			if _, ok := isIntBasicLit(expectedLen); (expectedLen == b) && !ok {
				// https://github.com/Antonboom/testifylint/issues/9
				return nil
			}
			return newUseFunctionDiagnostic(checker.Name(), call, proposedFn,
				analysis.TextEdit{
					Pos:     a.Pos(),
					End:     b.End(),
					NewText: formatAsCallArgs(pass, lenArg, expectedLen),
				})
		}

	case "True":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		if lenArg, expectedLen, ok := isLenEquality(pass, expr); ok {
			if _, ok := isIntBasicLit(expectedLen); !ok {
				return nil
			}
			return newUseFunctionDiagnostic(checker.Name(), call, proposedFn,
				analysis.TextEdit{
					Pos:     expr.Pos(),
					End:     expr.End(),
					NewText: formatAsCallArgs(pass, lenArg, expectedLen),
				})
		}
	}
	return nil
}
