package logstorage

import (
	"fmt"
	"sync"
)

// filterEqField matches if the given fields have equivalent values.
//
// Example LogsQL: `fieldName:eq_field(otherField)`
type filterEqField struct {
	fieldName      string
	otherFieldName string
}

func (fe *filterEqField) String() string {
	return fmt.Sprintf("%seq_field(%s)", quoteFieldNameIfNeeded(fe.fieldName), quoteTokenIfNeeded(fe.otherFieldName))
}

func (fe *filterEqField) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fe.fieldName)
	neededFields.add(fe.otherFieldName)
}

func (fe *filterEqField) applyToBlockResult(br *blockResult, bm *bitmap) {
	c := br.getColumnByName(fe.fieldName)
	cOther := br.getColumnByName(fe.otherFieldName)

	if c.isConst && cOther.isConst {
		v := c.valuesEncoded[0]
		vOther := cOther.valuesEncoded[0]
		if v != vOther {
			bm.resetBits()
		}
		return
	}

	values := c.getValues(br)
	valuesOther := cOther.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return values[idx] == valuesOther[idx]
	})
}

func (fe *filterEqField) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	br := getBlockResult()
	br.mustInit(bs, bm)
	br.initRequestedColumns([]string{fe.fieldName, fe.otherFieldName})

	c := br.getColumnByName(fe.fieldName)
	cOther := br.getColumnByName(fe.otherFieldName)

	values := c.getValues(br)
	valuesOther := cOther.getValues(br)

	srcIdx := 0
	bm.forEachSetBit(func(_ int) bool {
		ok := values[srcIdx] == valuesOther[srcIdx]
		srcIdx++
		return ok
	})

	putBlockResult(br)
}

func getBlockResult() *blockResult {
	v := brPool.Get()
	if v == nil {
		return &blockResult{}
	}
	return v.(*blockResult)
}

func putBlockResult(br *blockResult) {
	br.reset()
	brPool.Put(br)
}

var brPool sync.Pool
