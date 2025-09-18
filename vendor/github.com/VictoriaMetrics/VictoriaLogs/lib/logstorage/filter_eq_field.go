package logstorage

import (
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterEqField matches if the given fields have equivalent values.
//
// Example LogsQL: `fieldName:eq_field(otherField)`
type filterEqField struct {
	fieldName      string
	otherFieldName string

	prefixFilter     prefixfilter.Filter
	prefixFilterOnce sync.Once
}

func (fe *filterEqField) String() string {
	return fmt.Sprintf("%seq_field(%s)", quoteFieldNameIfNeeded(fe.fieldName), quoteTokenIfNeeded(fe.otherFieldName))
}

func (fe *filterEqField) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fe.fieldName)
	pf.AddAllowFilter(fe.otherFieldName)
}

func (fe *filterEqField) getPrefixFilter() *prefixfilter.Filter {
	fe.prefixFilterOnce.Do(fe.initPrefixFilter)
	return &fe.prefixFilter
}

func (fe *filterEqField) initPrefixFilter() {
	fe.prefixFilter.AddAllowFilters([]string{fe.fieldName, fe.otherFieldName})
}

func (fe *filterEqField) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fe.fieldName == fe.otherFieldName {
		return
	}

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
	if c.isTime && cOther.isTime {
		// c and cOther point to the same _time column, since only a single _time column may exist
		return
	}

	if c.valueType != cOther.valueType {
		// Slow path - c and cOther have different valueType, so convert them to string values and compare them
		applyFilterEqString(br, bm, c, cOther)
		return
	}

	switch c.valueType {
	case valueTypeString:
		applyFilterEqString(br, bm, c, cOther)
	case valueTypeDict:
		applyFilterEqDict(br, bm, c, cOther)
	case valueTypeUint8:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeUint16:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeUint32:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeUint64:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeInt64:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeFloat64:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeIPv4:
		applyFilterEqBinValues(br, bm, c, cOther)
	case valueTypeTimestampISO8601:
		applyFilterEqBinValues(br, bm, c, cOther)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func applyFilterEqString(br *blockResult, bm *bitmap, c, cOther *blockResultColumn) {
	values := c.getValues(br)
	valuesOther := cOther.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return values[idx] == valuesOther[idx]
	})
}

func applyFilterEqDict(br *blockResult, bm *bitmap, c, cOther *blockResultColumn) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		dictIdx := valuesEncoded[idx][0]
		dictIdxOther := valuesEncodedOther[idx][0]
		v := c.dictValues[dictIdx]
		vOther := cOther.dictValues[dictIdxOther]
		return v == vOther
	})
}

func applyFilterEqBinValues(br *blockResult, bm *bitmap, c, cOther *blockResultColumn) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		return valuesEncoded[idx] == valuesEncodedOther[idx]
	})
}

func (fe *filterEqField) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fe.fieldName == fe.otherFieldName {
		return
	}

	v := bs.getConstColumnValue(fe.fieldName)
	vOther := bs.getConstColumnValue(fe.otherFieldName)
	if v != "" || vOther != "" {
		if v != "" && vOther != "" {
			if v != vOther {
				bm.resetBits()
			}
			return
		}
		fe.applyFilterString(bs, bm)
		return
	}

	ch := bs.getColumnHeader(fe.fieldName)
	chOther := bs.getColumnHeader(fe.otherFieldName)
	if ch == nil || chOther == nil {
		if ch == nil && chOther == nil {
			return
		}
		fe.applyFilterString(bs, bm)
		return
	}

	if ch.valueType != chOther.valueType {
		// Slow path - c and cOther have different valueType, so convert them to string values and compare them
		fe.applyFilterString(bs, bm)
		return
	}

	switch ch.valueType {
	case valueTypeString:
		fe.applyFilterString(bs, bm)
	case valueTypeDict:
		fe.applyFilterDict(bs, bm, ch, chOther)
	case valueTypeUint8:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeUint16:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeUint32:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeUint64:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeInt64:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeFloat64:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeIPv4:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	case valueTypeTimestampISO8601:
		fe.applyFilterBinValue(bs, bm, ch, chOther)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func (fe *filterEqField) applyFilterString(bs *blockSearch, bm *bitmap) {
	br := getBlockResult()
	br.mustInit(bs, bm)

	pf := fe.getPrefixFilter()
	br.initColumns(pf)

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

func (fe *filterEqField) applyFilterDict(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		dictIdx := valuesEncoded[idx][0]
		dictIdxOther := valuesEncodedOther[idx][0]
		v := ch.valuesDict.values[dictIdx]
		vOther := chOther.valuesDict.values[dictIdxOther]
		return v == vOther
	})
}

func (fe *filterEqField) applyFilterBinValue(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		return valuesEncoded[idx] == valuesEncodedOther[idx]
	})
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
