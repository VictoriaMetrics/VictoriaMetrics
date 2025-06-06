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
	"errors"
	"fmt"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/issue"
)

type bound int

const (
	lowerUnbounded bound = iota
	upperUnbounded
	unbounded
	upperBounded
)

const maxDepth = 20

func newSliceBoundsAnalyzer(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runSliceBounds,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runSliceBounds(pass *analysis.Pass) (interface{}, error) {
	ssaResult, err := getSSAResult(pass)
	if err != nil {
		return nil, err
	}

	issues := map[ssa.Instruction]*issue.Issue{}
	ifs := map[ssa.If]*ssa.BinOp{}
	for _, mcall := range ssaResult.SSA.SrcFuncs {
		for _, block := range mcall.DomPreorder() {
			for _, instr := range block.Instrs {
				switch instr := instr.(type) {
				case *ssa.Alloc:
					sliceCap, err := extractSliceCapFromAlloc(instr.String())
					if err != nil {
						break
					}
					allocRefs := instr.Referrers()
					if allocRefs == nil {
						break
					}
					for _, instr := range *allocRefs {
						if slice, ok := instr.(*ssa.Slice); ok {
							if _, ok := slice.X.(*ssa.Alloc); ok {
								if slice.Parent() != nil {
									l, h := extractSliceBounds(slice)
									newCap := computeSliceNewCap(l, h, sliceCap)
									violations := []ssa.Instruction{}
									trackSliceBounds(0, newCap, slice, &violations, ifs)
									for _, s := range violations {
										switch s := s.(type) {
										case *ssa.Slice:
											issue := newIssue(
												pass.Analyzer.Name,
												"slice bounds out of range",
												pass.Fset,
												s.Pos(),
												issue.Low,
												issue.High)
											issues[s] = issue
										case *ssa.IndexAddr:
											issue := newIssue(
												pass.Analyzer.Name,
												"slice index out of range",
												pass.Fset,
												s.Pos(),
												issue.Low,
												issue.High)
											issues[s] = issue
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	for ifref, binop := range ifs {
		bound, value, err := extractBinOpBound(binop)
		if err != nil {
			continue
		}
		for i, block := range ifref.Block().Succs {
			if i == 1 {
				bound = invBound(bound)
			}
			var processBlock func(block *ssa.BasicBlock, depth int)
			processBlock = func(block *ssa.BasicBlock, depth int) {
				if depth == maxDepth {
					return
				}
				depth++
				for _, instr := range block.Instrs {
					if _, ok := issues[instr]; ok {
						switch bound {
						case lowerUnbounded:
							break
						case upperUnbounded, unbounded:
							delete(issues, instr)
						case upperBounded:
							switch tinstr := instr.(type) {
							case *ssa.Slice:
								lower, upper := extractSliceBounds(tinstr)
								if isSliceInsideBounds(0, value, lower, upper) {
									delete(issues, instr)
								}
							case *ssa.IndexAddr:
								indexValue, err := extractIntValue(tinstr.Index.String())
								if err != nil {
									break
								}
								if isSliceIndexInsideBounds(0, value, indexValue) {
									delete(issues, instr)
								}
							}
						}
					} else if nestedIfInstr, ok := instr.(*ssa.If); ok {
						for _, nestedBlock := range nestedIfInstr.Block().Succs {
							processBlock(nestedBlock, depth)
						}
					}
				}
			}

			processBlock(block, 0)
		}
	}

	foundIssues := []*issue.Issue{}
	for _, issue := range issues {
		foundIssues = append(foundIssues, issue)
	}
	if len(foundIssues) > 0 {
		return foundIssues, nil
	}
	return nil, nil
}

func trackSliceBounds(depth int, sliceCap int, slice ssa.Node, violations *[]ssa.Instruction, ifs map[ssa.If]*ssa.BinOp) {
	if depth == maxDepth {
		return
	}
	depth++
	if violations == nil {
		violations = &[]ssa.Instruction{}
	}
	referrers := slice.Referrers()
	if referrers != nil {
		for _, refinstr := range *referrers {
			switch refinstr := refinstr.(type) {
			case *ssa.Slice:
				checkAllSlicesBounds(depth, sliceCap, refinstr, violations, ifs)
				switch refinstr.X.(type) {
				case *ssa.Alloc, *ssa.Parameter:
					l, h := extractSliceBounds(refinstr)
					newCap := computeSliceNewCap(l, h, sliceCap)
					trackSliceBounds(depth, newCap, refinstr, violations, ifs)
				}
			case *ssa.IndexAddr:
				indexValue, err := extractIntValue(refinstr.Index.String())
				if err == nil && !isSliceIndexInsideBounds(0, sliceCap, indexValue) {
					*violations = append(*violations, refinstr)
				}
			case *ssa.Call:
				if ifref, cond := extractSliceIfLenCondition(refinstr); ifref != nil && cond != nil {
					ifs[*ifref] = cond
				} else {
					parPos := -1
					for pos, arg := range refinstr.Call.Args {
						if a, ok := arg.(*ssa.Slice); ok && a == slice {
							parPos = pos
						}
					}
					if fn, ok := refinstr.Call.Value.(*ssa.Function); ok {
						if len(fn.Params) > parPos && parPos > -1 {
							param := fn.Params[parPos]
							trackSliceBounds(depth, sliceCap, param, violations, ifs)
						}
					}
				}
			}
		}
	}
}

func checkAllSlicesBounds(depth int, sliceCap int, slice *ssa.Slice, violations *[]ssa.Instruction, ifs map[ssa.If]*ssa.BinOp) {
	if depth == maxDepth {
		return
	}
	depth++
	if violations == nil {
		violations = &[]ssa.Instruction{}
	}
	sliceLow, sliceHigh := extractSliceBounds(slice)
	if !isSliceInsideBounds(0, sliceCap, sliceLow, sliceHigh) {
		*violations = append(*violations, slice)
	}
	switch slice.X.(type) {
	case *ssa.Alloc, *ssa.Parameter, *ssa.Slice:
		l, h := extractSliceBounds(slice)
		newCap := computeSliceNewCap(l, h, sliceCap)
		trackSliceBounds(depth, newCap, slice, violations, ifs)
	}

	references := slice.Referrers()
	if references == nil {
		return
	}
	for _, ref := range *references {
		switch s := ref.(type) {
		case *ssa.Slice:
			checkAllSlicesBounds(depth, sliceCap, s, violations, ifs)
			switch s.X.(type) {
			case *ssa.Alloc, *ssa.Parameter:
				l, h := extractSliceBounds(s)
				newCap := computeSliceNewCap(l, h, sliceCap)
				trackSliceBounds(depth, newCap, s, violations, ifs)
			}
		}
	}
}

func extractSliceIfLenCondition(call *ssa.Call) (*ssa.If, *ssa.BinOp) {
	if builtInLen, ok := call.Call.Value.(*ssa.Builtin); ok {
		if builtInLen.Name() == "len" {
			refs := call.Referrers()
			if refs != nil {
				for _, ref := range *refs {
					if binop, ok := ref.(*ssa.BinOp); ok {
						binoprefs := binop.Referrers()
						for _, ref := range *binoprefs {
							if ifref, ok := ref.(*ssa.If); ok {
								return ifref, binop
							}
						}
					}
				}
			}
		}
	}
	return nil, nil
}

func computeSliceNewCap(l, h, oldCap int) int {
	if l == 0 && h == 0 {
		return oldCap
	}
	if l > 0 && h == 0 {
		return oldCap - l
	}
	if l == 0 && h > 0 {
		return h
	}
	return h - l
}

func invBound(bound bound) bound {
	switch bound {
	case lowerUnbounded:
		return upperUnbounded
	case upperUnbounded:
		return lowerUnbounded
	case upperBounded:
		return unbounded
	case unbounded:
		return upperBounded
	default:
		return unbounded
	}
}

func extractBinOpBound(binop *ssa.BinOp) (bound, int, error) {
	if binop.X != nil {
		if x, ok := binop.X.(*ssa.Const); ok {
			value, err := strconv.Atoi(x.Value.String())
			if err != nil {
				return lowerUnbounded, value, err
			}
			switch binop.Op {
			case token.LSS, token.LEQ:
				return upperUnbounded, value, nil
			case token.GTR, token.GEQ:
				return lowerUnbounded, value, nil
			case token.EQL:
				return upperBounded, value, nil
			case token.NEQ:
				return unbounded, value, nil
			}
		}
	}
	if binop.Y != nil {
		if y, ok := binop.Y.(*ssa.Const); ok {
			value, err := strconv.Atoi(y.Value.String())
			if err != nil {
				return lowerUnbounded, value, err
			}
			switch binop.Op {
			case token.LSS, token.LEQ:
				return lowerUnbounded, value, nil
			case token.GTR, token.GEQ:
				return upperUnbounded, value, nil
			case token.EQL:
				return upperBounded, value, nil
			case token.NEQ:
				return unbounded, value, nil
			}
		}
	}
	return lowerUnbounded, 0, fmt.Errorf("unable to extract constant from binop")
}

func isSliceIndexInsideBounds(l, h int, index int) bool {
	return (l <= index && index < h)
}

func isSliceInsideBounds(l, h int, cl, ch int) bool {
	return (l <= cl && h >= ch) && (l <= ch && h >= cl)
}

func extractSliceBounds(slice *ssa.Slice) (int, int) {
	var low int
	if slice.Low != nil {
		l, err := extractIntValue(slice.Low.String())
		if err == nil {
			low = l
		}
	}
	var high int
	if slice.High != nil {
		h, err := extractIntValue(slice.High.String())
		if err == nil {
			high = h
		}
	}
	return low, high
}

func extractIntValue(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid value: %s", value)
	}
	if parts[1] != "int" {
		return 0, fmt.Errorf("invalid value: %s", value)
	}
	return strconv.Atoi(parts[0])
}

func extractSliceCapFromAlloc(instr string) (int, error) {
	re := regexp.MustCompile(`new \[(\d+)\]*`)
	var sliceCap int
	matches := re.FindAllStringSubmatch(instr, -1)
	if matches == nil {
		return sliceCap, errors.New("no slice cap found")
	}

	if len(matches) > 0 {
		m := matches[0]
		if len(m) > 1 {
			return strconv.Atoi(m[1])
		}
	}

	return 0, errors.New("no slice cap found")
}
