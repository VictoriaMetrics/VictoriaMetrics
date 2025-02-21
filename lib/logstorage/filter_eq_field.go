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
	bmTmp := getBitmap(bm.bitsLen)
	bmTmp.setBits()

	br := getBlockResult()
	br.mustInit(bs, bmTmp)
	br.initRequestedColumns([]string{fe.fieldName, fe.otherFieldName})

	fe.applyToBlockResult(br, bm)

	putBlockResult(br)

	putBitmap(bmTmp)
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
