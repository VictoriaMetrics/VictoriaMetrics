package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// UselessAssert detects useless asserts like
//
//	assert.Contains(t, tt.value, tt.value)
//	assert.ElementsMatch(t, tt.value, tt.value)
//	assert.Equal(t, tt.value, tt.value)
//	assert.EqualExportedValues(t, tt.value, tt.value)
//	...
//
//	assert.True(t, num > num)
//	assert.True(t, num < num)
//	assert.True(t, num >= num)
//	assert.True(t, num <= num)
//	assert.True(t, num == num)
//	assert.True(t, num != num)
//
//	assert.False(t, num > num)
//	assert.False(t, num < num)
//	assert.False(t, num >= num)
//	assert.False(t, num <= num)
//	assert.False(t, num == num)
//	assert.False(t, num != num)
//
//	assert.Empty(t, "")
//	assert.False(t, false)
//	assert.Implements(t, (*any)(nil), new(Conn))
//	assert.Negative(t, -42)
//	assert.Nil(t, nil)
//	assert.NoError(t, nil)
//	assert.NotEmpty(t, "value")
//	assert.NotZero(t, 42)
//	assert.NotZero(t, "value")
//	assert.Positive(t, 42)
//	assert.True(t, true)
//	assert.Zero(t, 0)
//	assert.Zero(t, "")
//	assert.Zero(t, nil)
type UselessAssert struct{}

// NewUselessAssert constructs UselessAssert checker.
func NewUselessAssert() UselessAssert { return UselessAssert{} }
func (UselessAssert) Name() string    { return "useless-assert" }

func (checker UselessAssert) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if d := checker.checkSameVars(pass, call); d != nil {
		return d
	}

	var isMeaningless bool
	switch call.Fn.NameFTrimmed {
	case "Empty":
		isMeaningless = (len(call.Args) >= 1) && isEmptyStringLit(call.Args[0])

	case "False":
		isMeaningless = (len(call.Args) >= 1) && isUntypedFalse(pass, call.Args[0])

	case "Implements":
		if len(call.Args) < 2 {
			return nil
		}

		elem, ok := isPointer(pass, call.Args[0])
		isMeaningless = ok && isEmptyInterfaceType(elem)

	case "Negative":
		isMeaningless = (len(call.Args) >= 1) && isNegativeIntNumber(call.Args[0])

	case "Nil", "NoError":
		isMeaningless = (len(call.Args) >= 1) && isNil(call.Args[0])

	case "NotEmpty":
		isMeaningless = (len(call.Args) >= 1) && isNotEmptyStringLit(call.Args[0])

	case "NotZero":
		isMeaningless = (len(call.Args) >= 1) &&
			(isNotEmptyStringLit(call.Args[0]) ||
				isNegativeIntNumber(call.Args[0]) || isPositiveIntNumber(call.Args[0]))

	case "Positive":
		isMeaningless = (len(call.Args) >= 1) && isPositiveIntNumber(call.Args[0])

	case "True":
		isMeaningless = (len(call.Args) >= 1) && isUntypedTrue(pass, call.Args[0])

	case "Zero":
		isMeaningless = (len(call.Args) >= 1) &&
			(isZero(call.Args[0]) || isEmptyStringLit(call.Args[0]) || isNil(call.Args[0]))
	}

	if isMeaningless {
		return newDiagnostic(checker.Name(), call, "meaningless assertion")
	}
	return nil
}

func (checker UselessAssert) checkSameVars(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	var first, second ast.Node

	switch call.Fn.NameFTrimmed {
	case
		"Contains",
		"ElementsMatch",
		"Equal",
		"EqualExportedValues",
		"EqualValues",
		"ErrorAs",
		"ErrorIs",
		"Exactly",
		"Greater",
		"GreaterOrEqual",
		"Implements",
		"InDelta",
		"InDeltaMapValues",
		"InDeltaSlice",
		"InEpsilon",
		"InEpsilonSlice",
		"IsType",
		"JSONEq",
		"Less",
		"LessOrEqual",
		"NotEqual",
		"NotEqualValues",
		"NotErrorIs",
		"NotRegexp",
		"NotSame",
		"NotSubset",
		"Regexp",
		"Same",
		"Subset",
		"WithinDuration",
		"YAMLEq":
		if len(call.Args) < 2 {
			return nil
		}
		first, second = call.Args[0], call.Args[1]

	case "True", "False":
		if len(call.Args) < 1 {
			return nil
		}

		be, ok := call.Args[0].(*ast.BinaryExpr)
		if !ok {
			return nil
		}
		first, second = be.X, be.Y

	default:
		return nil
	}

	if analysisutil.NodeString(pass.Fset, first) == analysisutil.NodeString(pass.Fset, second) {
		return newDiagnostic(checker.Name(), call, "asserting of the same variable")
	}
	return nil
}
