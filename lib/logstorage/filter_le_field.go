package logstorage

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterLeField matches if the fieldName field is smaller or equal to the otherFieldName field
//
// Example LogsQL: `fieldName:le_field(otherField)`
type filterLeField struct {
	fieldName      string
	otherFieldName string

	excludeEqualValues bool

	prefixFilter     prefixfilter.Filter
	prefixFilterOnce sync.Once
}

func (fe *filterLeField) String() string {
	funcName := "le_field"
	if fe.excludeEqualValues {
		funcName = "lt_field"
	}
	return fmt.Sprintf("%s%s(%s)", quoteFieldNameIfNeeded(fe.fieldName), funcName, quoteTokenIfNeeded(fe.otherFieldName))
}

func (fe *filterLeField) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fe.fieldName)
	pf.AddAllowFilter(fe.otherFieldName)
}

func (fe *filterLeField) getPrefixFilter() *prefixfilter.Filter {
	fe.prefixFilterOnce.Do(fe.initPrefixFilter)
	return &fe.prefixFilter
}

func (fe *filterLeField) initPrefixFilter() {
	fe.prefixFilter.AddAllowFilters([]string{fe.fieldName, fe.otherFieldName})
}

func (fe *filterLeField) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fe.fieldName == fe.otherFieldName {
		if fe.excludeEqualValues {
			bm.resetBits()
		}
		return
	}

	c := br.getColumnByName(fe.fieldName)
	cOther := br.getColumnByName(fe.otherFieldName)

	if c.isConst && cOther.isConst {
		v := c.valuesEncoded[0]
		vOther := cOther.valuesEncoded[0]
		if !leValuesString(v, vOther, fe.excludeEqualValues) {
			bm.resetBits()
		}
		return
	}
	if c.isTime && cOther.isTime {
		// c and cOther point to the same _time column, since only a single _time column may exist
		if fe.excludeEqualValues {
			bm.resetBits()
		}
		return
	}

	if c.valueType != cOther.valueType {
		// Slow path - c and cOther have different valueType, so convert them to string values and compare them
		applyFilterLeString(br, bm, c, cOther, fe.excludeEqualValues)
		return
	}

	switch c.valueType {
	case valueTypeString:
		applyFilterLeString(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeDict:
		applyFilterLeDict(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeUint8:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeUint16:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeUint32:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeUint64:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeInt64:
		applyFilterLeInt64(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeFloat64:
		applyFilterLeFloat64(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeIPv4:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	case valueTypeTimestampISO8601:
		applyFilterLeUint(br, bm, c, cOther, fe.excludeEqualValues)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func applyFilterLeString(br *blockResult, bm *bitmap, c, cOther *blockResultColumn, excludeEqualValues bool) {
	values := c.getValues(br)
	valuesOther := cOther.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return leValuesString(values[idx], valuesOther[idx], excludeEqualValues)
	})
}

func applyFilterLeDict(br *blockResult, bm *bitmap, c, cOther *blockResultColumn, excludeEqualValues bool) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		dictIdx := valuesEncoded[idx][0]
		dictIdxOther := valuesEncodedOther[idx][0]
		v := c.dictValues[dictIdx]
		vOther := cOther.dictValues[dictIdxOther]
		return leValuesString(v, vOther, excludeEqualValues)
	})
}

func applyFilterLeUint(br *blockResult, bm *bitmap, c, cOther *blockResultColumn, excludeEqualValues bool) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		return leValuesString(valuesEncoded[idx], valuesEncodedOther[idx], excludeEqualValues)
	})
}

func applyFilterLeInt64(br *blockResult, bm *bitmap, c, cOther *blockResultColumn, excludeEqualValues bool) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		n := unmarshalInt64(valuesEncoded[idx])
		nOther := unmarshalInt64(valuesEncodedOther[idx])
		return leValuesInt64(n, nOther, excludeEqualValues)
	})
}

func applyFilterLeFloat64(br *blockResult, bm *bitmap, c, cOther *blockResultColumn, excludeEqualValues bool) {
	valuesEncoded := c.getValuesEncoded(br)
	valuesEncodedOther := cOther.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		f := unmarshalFloat64(valuesEncoded[idx])
		fOther := unmarshalFloat64(valuesEncodedOther[idx])
		return leValuesFloat64(f, fOther, excludeEqualValues)
	})
}

func (fe *filterLeField) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fe.fieldName == fe.otherFieldName {
		if fe.excludeEqualValues {
			bm.resetBits()
		}
		return
	}

	v := bs.getConstColumnValue(fe.fieldName)
	vOther := bs.getConstColumnValue(fe.otherFieldName)
	if v != "" || vOther != "" {
		if v != "" && vOther != "" {
			if !leValuesString(v, vOther, fe.excludeEqualValues) {
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
			if fe.excludeEqualValues {
				bm.resetBits()
			}
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
		fe.applyFilterUint(bs, bm, ch, chOther)
	case valueTypeUint16:
		fe.applyFilterUint(bs, bm, ch, chOther)
	case valueTypeUint32:
		fe.applyFilterUint(bs, bm, ch, chOther)
	case valueTypeUint64:
		fe.applyFilterUint(bs, bm, ch, chOther)
	case valueTypeInt64:
		fe.applyFilterInt64(bs, bm, ch, chOther)
	case valueTypeFloat64:
		fe.applyFilterFloat64(bs, bm, ch, chOther)
	case valueTypeIPv4:
		fe.applyFilterUint(bs, bm, ch, chOther)
	case valueTypeTimestampISO8601:
		fe.applyFilterUint(bs, bm, ch, chOther)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func (fe *filterLeField) applyFilterString(bs *blockSearch, bm *bitmap) {
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
		ok := leValuesString(values[srcIdx], valuesOther[srcIdx], fe.excludeEqualValues)
		srcIdx++
		return ok
	})

	putBlockResult(br)
}

func (fe *filterLeField) applyFilterDict(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		dictIdx := valuesEncoded[idx][0]
		dictIdxOther := valuesEncodedOther[idx][0]
		v := ch.valuesDict.values[dictIdx]
		vOther := chOther.valuesDict.values[dictIdxOther]
		return leValuesString(v, vOther, fe.excludeEqualValues)
	})
}

func (fe *filterLeField) applyFilterUint(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		return leValuesString(valuesEncoded[idx], valuesEncodedOther[idx], fe.excludeEqualValues)
	})
}

func (fe *filterLeField) applyFilterInt64(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		n := unmarshalInt64(valuesEncoded[idx])
		nOther := unmarshalInt64(valuesEncodedOther[idx])
		return leValuesInt64(n, nOther, fe.excludeEqualValues)
	})
}

func (fe *filterLeField) applyFilterFloat64(bs *blockSearch, bm *bitmap, ch, chOther *columnHeader) {
	valuesEncoded := bs.getValuesForColumn(ch)
	valuesEncodedOther := bs.getValuesForColumn(chOther)
	bm.forEachSetBit(func(idx int) bool {
		f := unmarshalFloat64(valuesEncoded[idx])
		fOther := unmarshalFloat64(valuesEncodedOther[idx])
		return leValuesFloat64(f, fOther, fe.excludeEqualValues)
	})
}

func leValuesString(a, b string, excludeEqualValues bool) bool {
	fA := parseMathNumber(a)
	if !math.IsNaN(fA) {
		fB := parseMathNumber(b)
		if !math.IsNaN(fB) {
			if excludeEqualValues {
				return fA < fB
			}
			return fA <= fB
		}
	}
	if excludeEqualValues {
		return a < b
	}
	return a <= b
}

func leValuesInt64(a, b int64, excludeEqualValues bool) bool {
	if excludeEqualValues {
		return a < b
	}
	return a <= b
}

func leValuesFloat64(a, b float64, excludeEqualValues bool) bool {
	if excludeEqualValues {
		return a < b
	}
	return a <= b
}
