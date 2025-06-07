package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterValueType filters field entries by value type.
//
// For example, the following filter returns all the logs with uint64 fieldName:
//
//	fieldName:value_type("uint64")
type filterValueType struct {
	fieldName string
	valueType string
}

func (fv *filterValueType) String() string {
	return fmt.Sprintf("%svalue_type(%s)", quoteFieldNameIfNeeded(fv.fieldName), quoteTokenIfNeeded(fv.valueType))
}

func (fv *filterValueType) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fv.fieldName)
}

func (fv *filterValueType) applyToBlockResult(br *blockResult, bm *bitmap) {
	var typ string
	c := br.getColumnByName(fv.fieldName)
	if c.isConst {
		typ = "const"
	} else if c.isTime {
		typ = "time"
	} else if br.bs == nil {
		typ = "inmemory"
	} else {
		typ = c.valueType.String()
	}
	if fv.valueType != typ {
		bm.resetBits()
	}
}

func (fv *filterValueType) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fv.fieldName

	// Verify whether fp matches const column
	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if fv.valueType != "const" {
			bm.resetBits()
		}
		return
	}

	// Verify whether fp matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		bm.resetBits()
		return
	}

	typ := ch.valueType.String()
	if fv.valueType != typ {
		bm.resetBits()
	}
}
