// (c) Copyright gosec's authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analyzers

import (
	"cmp"
	"fmt"
	"go/token"
	"math"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/issue"
)

type integer struct {
	signed bool
	size   int
	min    int
	max    uint
}

type rangeResult struct {
	minValue             int
	maxValue             uint
	explicitPositiveVals []uint
	explicitNegativeVals []int
	isRangeCheck         bool
	convertFound         bool
}

type branchResults struct {
	minValue             *int
	maxValue             *uint
	explicitPositiveVals []uint
	explicitNegativeVals []int
	convertFound         bool
}

func newConversionOverflowAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runConversionOverflow,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runConversionOverflow(pass *analysis.Pass) (interface{}, error) {
	ssaResult, err := getSSAResult(pass)
	if err != nil {
		return nil, fmt.Errorf("building ssa representation: %w", err)
	}

	issues := []*issue.Issue{}
	for _, mcall := range ssaResult.SSA.SrcFuncs {
		for _, block := range mcall.DomPreorder() {
			for _, instr := range block.Instrs {
				switch instr := instr.(type) {
				case *ssa.Convert:
					src := instr.X.Type().Underlying().String()
					dst := instr.Type().Underlying().String()
					if isIntOverflow(src, dst) {
						if isSafeConversion(instr) {
							continue
						}
						issue := newIssue(pass.Analyzer.Name,
							fmt.Sprintf("integer overflow conversion %s -> %s", src, dst),
							pass.Fset,
							instr.Pos(),
							issue.High,
							issue.Medium,
						)
						issues = append(issues, issue)
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		return issues, nil
	}
	return nil, nil
}

func isIntOverflow(src string, dst string) bool {
	srcInt, err := parseIntType(src)
	if err != nil {
		return false
	}

	dstInt, err := parseIntType(dst)
	if err != nil {
		return false
	}

	return srcInt.min < dstInt.min || srcInt.max > dstInt.max
}

func parseIntType(intType string) (integer, error) {
	re := regexp.MustCompile(`^(?P<type>u?int)(?P<size>\d{1,2})?$`)
	matches := re.FindStringSubmatch(intType)
	if matches == nil {
		return integer{}, fmt.Errorf("no integer type match found for %s", intType)
	}

	it := matches[re.SubexpIndex("type")]
	is := matches[re.SubexpIndex("size")]

	signed := it == "int"

	// use default system int type in case size is not present in the type.
	intSize := strconv.IntSize
	if is != "" {
		var err error
		intSize, err = strconv.Atoi(is)
		if err != nil {
			return integer{}, fmt.Errorf("failed to parse the integer type size: %w", err)
		}
	}

	if intSize != 8 && intSize != 16 && intSize != 32 && intSize != 64 && is != "" {
		return integer{}, fmt.Errorf("invalid bit size: %d", intSize)
	}

	var minVal int
	var maxVal uint

	if signed {
		shiftAmount := intSize - 1

		// Perform a bounds check.
		if shiftAmount < 0 {
			return integer{}, fmt.Errorf("invalid shift amount: %d", shiftAmount)
		}

		maxVal = (1 << uint(shiftAmount)) - 1
		minVal = -1 << (intSize - 1)

	} else {
		maxVal = (1 << uint(intSize)) - 1
		minVal = 0
	}

	return integer{
		signed: signed,
		size:   intSize,
		min:    minVal,
		max:    maxVal,
	}, nil
}

func isSafeConversion(instr *ssa.Convert) bool {
	dstType := instr.Type().Underlying().String()

	// Check for constant conversions.
	if constVal, ok := instr.X.(*ssa.Const); ok {
		if isConstantInRange(constVal, dstType) {
			return true
		}
	}

	// Check for string to integer conversions with specified bit size.
	if isStringToIntConversion(instr, dstType) {
		return true
	}

	// Check for explicit range checks.
	if hasExplicitRangeCheck(instr, dstType) {
		return true
	}

	return false
}

func isConstantInRange(constVal *ssa.Const, dstType string) bool {
	value, err := strconv.ParseInt(constVal.Value.String(), 10, 64)
	if err != nil {
		return false
	}

	dstInt, err := parseIntType(dstType)
	if err != nil {
		return false
	}

	if dstInt.signed {
		return value >= -(1<<(dstInt.size-1)) && value <= (1<<(dstInt.size-1))-1
	}
	return value >= 0 && value <= (1<<dstInt.size)-1
}

func isStringToIntConversion(instr *ssa.Convert, dstType string) bool {
	// Traverse the SSA instructions to find the original variable.
	original := instr.X
	for {
		switch v := original.(type) {
		case *ssa.Call:
			if v.Call.StaticCallee() != nil && (v.Call.StaticCallee().Name() == "ParseInt" || v.Call.StaticCallee().Name() == "ParseUint") {
				if len(v.Call.Args) == 3 {
					if bitSize, ok := v.Call.Args[2].(*ssa.Const); ok {
						signed := v.Call.StaticCallee().Name() == "ParseInt"
						bitSizeValue, err := strconv.Atoi(bitSize.Value.String())
						if err != nil {
							return false
						}
						dstInt, err := parseIntType(dstType)
						if err != nil {
							return false
						}

						// we're good if:
						// - signs match and bit size is <= than destination
						// - parsing unsigned and bit size is < than destination
						isSafe := (bitSizeValue <= dstInt.size && signed == dstInt.signed) ||
							(bitSizeValue < dstInt.size && !signed)
						return isSafe
					}
				}
			}
			return false
		case *ssa.Phi:
			original = v.Edges[0]
		case *ssa.Extract:
			original = v.Tuple
		default:
			return false
		}
	}
}

func hasExplicitRangeCheck(instr *ssa.Convert, dstType string) bool {
	dstInt, err := parseIntType(dstType)
	if err != nil {
		return false
	}

	srcInt, err := parseIntType(instr.X.Type().String())
	if err != nil {
		return false
	}

	minValue := srcInt.min
	maxValue := srcInt.max
	explicitPositiveVals := []uint{}
	explicitNegativeVals := []int{}

	if minValue > dstInt.min && maxValue < dstInt.max {
		return true
	}

	visitedIfs := make(map[*ssa.If]bool)
	for _, block := range instr.Parent().Blocks {
		for _, blockInstr := range block.Instrs {
			switch v := blockInstr.(type) {
			case *ssa.If:
				result := getResultRange(v, instr, visitedIfs)
				if result.isRangeCheck {
					minValue = max(minValue, result.minValue)
					maxValue = min(maxValue, result.maxValue)
					explicitPositiveVals = append(explicitPositiveVals, result.explicitPositiveVals...)
					explicitNegativeVals = append(explicitNegativeVals, result.explicitNegativeVals...)
				}
			case *ssa.Call:
				// These function return an int of a guaranteed size.
				if v != instr.X {
					continue
				}
				if fn, isBuiltin := v.Call.Value.(*ssa.Builtin); isBuiltin {
					switch fn.Name() {
					case "len", "cap":
						minValue = 0
					}
				}
			}

			if explicitValsInRange(explicitPositiveVals, explicitNegativeVals, dstInt) {
				return true
			} else if minValue >= dstInt.min && maxValue <= dstInt.max {
				return true
			}
		}
	}
	return false
}

// getResultRange is a recursive function that walks the branches of the if statement to find the range of the variable.
func getResultRange(ifInstr *ssa.If, instr *ssa.Convert, visitedIfs map[*ssa.If]bool) rangeResult {
	if visitedIfs[ifInstr] {
		return rangeResult{minValue: math.MinInt, maxValue: math.MaxUint}
	}
	visitedIfs[ifInstr] = true

	cond := ifInstr.Cond
	binOp, ok := cond.(*ssa.BinOp)
	if !ok || !isRangeCheck(binOp, instr.X) {
		return rangeResult{minValue: math.MinInt, maxValue: math.MaxUint}
	}

	result := rangeResult{
		minValue:     math.MinInt,
		maxValue:     math.MaxUint,
		isRangeCheck: true,
	}

	thenBounds := walkBranchForConvert(ifInstr.Block().Succs[0], instr, visitedIfs)
	elseBounds := walkBranchForConvert(ifInstr.Block().Succs[1], instr, visitedIfs)

	updateResultFromBinOp(&result, binOp, instr, thenBounds.convertFound)

	if thenBounds.convertFound {
		result.convertFound = true
		result.minValue = maxWithPtr(result.minValue, thenBounds.minValue)
		result.maxValue = minWithPtr(result.maxValue, thenBounds.maxValue)
	} else if elseBounds.convertFound {
		result.convertFound = true
		result.minValue = maxWithPtr(result.minValue, elseBounds.minValue)
		result.maxValue = minWithPtr(result.maxValue, elseBounds.maxValue)
	}

	result.explicitPositiveVals = append(result.explicitPositiveVals, thenBounds.explicitPositiveVals...)
	result.explicitNegativeVals = append(result.explicitNegativeVals, thenBounds.explicitNegativeVals...)
	result.explicitPositiveVals = append(result.explicitPositiveVals, elseBounds.explicitPositiveVals...)
	result.explicitNegativeVals = append(result.explicitNegativeVals, elseBounds.explicitNegativeVals...)

	return result
}

// updateResultFromBinOp updates the rangeResult based on the BinOp instruction and the location of the Convert instruction.
func updateResultFromBinOp(result *rangeResult, binOp *ssa.BinOp, instr *ssa.Convert, successPathConvert bool) {
	x, y := binOp.X, binOp.Y
	operandsFlipped := false

	compareVal, op := getRealValueFromOperation(instr.X)

	// Handle FieldAddr
	if fieldAddr, ok := compareVal.(*ssa.FieldAddr); ok {
		compareVal = fieldAddr
	}

	if !isSameOrRelated(x, compareVal) {
		y = x
		operandsFlipped = true
	}

	constVal, ok := y.(*ssa.Const)
	if !ok {
		return
	}
	// TODO: constVal.Value nil check avoids #1229 panic but seems to be hiding a bug in the code above or in x/tools/go/ssa.
	if constVal.Value == nil {
		// log.Fatalf("[gosec] constVal.Value is nil flipped=%t, constVal=%#v, binOp=%#v", operandsFlipped, constVal, binOp)
		return
	}
	switch binOp.Op {
	case token.LEQ, token.LSS:
		updateMinMaxForLessOrEqual(result, constVal, binOp.Op, operandsFlipped, successPathConvert)
	case token.GEQ, token.GTR:
		updateMinMaxForGreaterOrEqual(result, constVal, binOp.Op, operandsFlipped, successPathConvert)
	case token.EQL:
		if !successPathConvert {
			break
		}
		updateExplicitValues(result, constVal)
	case token.NEQ:
		if successPathConvert {
			break
		}
		updateExplicitValues(result, constVal)
	}

	if op == "neg" {
		minVal := result.minValue
		maxVal := result.maxValue

		if minVal >= 0 {
			result.maxValue = uint(minVal)
		}
		if maxVal <= math.MaxInt {
			result.minValue = int(maxVal)
		}
	}
}

func updateExplicitValues(result *rangeResult, constVal *ssa.Const) {
	if strings.Contains(constVal.String(), "-") {
		result.explicitNegativeVals = append(result.explicitNegativeVals, int(constVal.Int64()))
	} else {
		result.explicitPositiveVals = append(result.explicitPositiveVals, uint(constVal.Uint64()))
	}
}

func updateMinMaxForLessOrEqual(result *rangeResult, constVal *ssa.Const, op token.Token, operandsFlipped bool, successPathConvert bool) {
	// If the success path has a conversion and the operands are not flipped, then the constant value is the maximum value.
	if successPathConvert && !operandsFlipped {
		result.maxValue = uint(constVal.Uint64())
		if op == token.LEQ {
			result.maxValue--
		}
	} else {
		result.minValue = int(constVal.Int64())
		if op == token.GTR {
			result.minValue++
		}
	}
}

func updateMinMaxForGreaterOrEqual(result *rangeResult, constVal *ssa.Const, op token.Token, operandsFlipped bool, successPathConvert bool) {
	// If the success path has a conversion and the operands are not flipped, then the constant value is the minimum value.
	if successPathConvert && !operandsFlipped {
		result.minValue = int(constVal.Int64())
		if op == token.GEQ {
			result.minValue++
		}
	} else {
		result.maxValue = uint(constVal.Uint64())
		if op == token.LSS {
			result.maxValue--
		}
	}
}

// walkBranchForConvert walks the branch of the if statement to find the range of the variable and where the conversion is.
func walkBranchForConvert(block *ssa.BasicBlock, instr *ssa.Convert, visitedIfs map[*ssa.If]bool) branchResults {
	bounds := branchResults{}

	for _, blockInstr := range block.Instrs {
		switch v := blockInstr.(type) {
		case *ssa.If:
			result := getResultRange(v, instr, visitedIfs)
			bounds.convertFound = bounds.convertFound || result.convertFound

			if result.isRangeCheck {
				bounds.minValue = toPtr(maxWithPtr(result.minValue, bounds.minValue))
				bounds.maxValue = toPtr(minWithPtr(result.maxValue, bounds.maxValue))
				bounds.explicitPositiveVals = append(bounds.explicitPositiveVals, result.explicitPositiveVals...)
				bounds.explicitNegativeVals = append(bounds.explicitNegativeVals, result.explicitNegativeVals...)
			}
		case *ssa.Call:
			if v == instr.X {
				if fn, isBuiltin := v.Call.Value.(*ssa.Builtin); isBuiltin && (fn.Name() == "len" || fn.Name() == "cap") {
					bounds.minValue = toPtr(0)
				}
			}
		case *ssa.Convert:
			if v == instr {
				bounds.convertFound = true
				return bounds
			}
		}
	}

	return bounds
}

func isRangeCheck(v ssa.Value, x ssa.Value) bool {
	compareVal, _ := getRealValueFromOperation(x)

	switch op := v.(type) {
	case *ssa.BinOp:
		switch op.Op {
		case token.LSS, token.LEQ, token.GTR, token.GEQ, token.EQL, token.NEQ:
			leftMatch := isSameOrRelated(op.X, compareVal)
			rightMatch := isSameOrRelated(op.Y, compareVal)
			return leftMatch || rightMatch
		}
	}
	return false
}

func getRealValueFromOperation(v ssa.Value) (ssa.Value, string) {
	switch v := v.(type) {
	case *ssa.UnOp:
		if v.Op == token.SUB {
			val, _ := getRealValueFromOperation(v.X)
			return val, "neg"
		}
		return getRealValueFromOperation(v.X)
	case *ssa.FieldAddr:
		return v, "field"
	case *ssa.Alloc:
		return v, "alloc"
	}
	return v, ""
}

func isSameOrRelated(a, b ssa.Value) bool {
	aVal, _ := getRealValueFromOperation(a)
	bVal, _ := getRealValueFromOperation(b)

	if aVal == bVal {
		return true
	}

	// Check if both are FieldAddr operations referring to the same field of the same struct
	if aField, aOk := aVal.(*ssa.FieldAddr); aOk {
		if bField, bOk := bVal.(*ssa.FieldAddr); bOk {
			return aField.X == bField.X && aField.Field == bField.Field
		}
	}

	return false
}

func explicitValsInRange(explicitPosVals []uint, explicitNegVals []int, dstInt integer) bool {
	if len(explicitPosVals) == 0 && len(explicitNegVals) == 0 {
		return false
	}

	for _, val := range explicitPosVals {
		if val > dstInt.max {
			return false
		}
	}

	for _, val := range explicitNegVals {
		if val < dstInt.min {
			return false
		}
	}

	return true
}

func minWithPtr[T cmp.Ordered](a T, b *T) T {
	if b == nil {
		return a
	}
	return min(a, *b)
}

func maxWithPtr[T cmp.Ordered](a T, b *T) T {
	if b == nil {
		return a
	}
	return max(a, *b)
}

func toPtr[T any](a T) *T {
	return &a
}
