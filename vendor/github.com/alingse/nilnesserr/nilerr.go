// some code was copy from https://github.com/gostaticanalysis/nilerr/blob/master/nilerr.go

package nilnesserr

import (
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

var errType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface) // nolint: forcetypeassert

func isErrType(res ssa.Value) bool {
	return types.Implements(res.Type(), errType)
}

func isConstNil(res ssa.Value) bool {
	v, ok := res.(*ssa.Const)
	if ok && v.IsNil() {
		return true
	}

	return false
}

func extractCheckedErrorValue(binOp *ssa.BinOp) ssa.Value {
	if isErrType(binOp.X) && isConstNil(binOp.Y) {
		return binOp.X
	}
	if isErrType(binOp.Y) && isConstNil(binOp.X) {
		return binOp.Y
	}

	return nil
}

type errFact fact

func findLastNonnilValue(errors []errFact, res ssa.Value) ssa.Value {
	if len(errors) == 0 {
		return nil
	}

	for j := len(errors) - 1; j >= 0; j-- {
		last := errors[j]
		if last.value == res {
			return nil
		} else if last.nilness == isnonnil {
			return last.value
		}
	}

	return nil
}

func checkNilnesserr(pass *analysis.Pass, b *ssa.BasicBlock, errors []errFact, isNilnees func(value ssa.Value) bool) {
	for i := range b.Instrs {
		instr, ok := b.Instrs[i].(*ssa.Return)
		if !ok {
			continue
		}

		for _, res := range instr.Results {
			if !isErrType(res) || isConstNil(res) || !isNilnees(res) {
				continue
			}
			// check the lastValue error that is isnonnil
			lastValue := findLastNonnilValue(errors, res)
			if lastValue == nil {
				continue
			}
			// report
			pos := instr.Pos()
			if pos.IsValid() {
				pass.Report(analysis.Diagnostic{
					Pos:     pos,
					Message: linterMessage,
				})
			}
		}
	}
}
