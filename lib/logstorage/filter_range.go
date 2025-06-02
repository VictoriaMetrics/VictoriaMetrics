package logstorage

import (
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterRange matches the given range [minValue..maxValue].
//
// Example LogsQL: `fieldName:range(minValue, maxValue]`
type filterRange struct {
	fieldName string

	minValue float64
	maxValue float64

	stringRepr string
}

func (fr *filterRange) String() string {
	return quoteFieldNameIfNeeded(fr.fieldName) + fr.stringRepr
}

func (fr *filterRange) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fr.fieldName)
}

func (fr *filterRange) applyToBlockResult(br *blockResult, bm *bitmap) {
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fr.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		minValueInt, maxValueInt := toInt64Range(minValue, maxValue)
		bm.forEachSetBit(func(idx int) bool {
			timestamp := timestamps[idx]
			return timestamp >= minValueInt && timestamp <= maxValueInt
		})
		return
	}

	switch c.valueType {
	case valueTypeString:
		values := c.getValues(br)
		bm.forEachSetBit(func(idx int) bool {
			v := values[idx]
			return matchRange(v, minValue, maxValue)
		})
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchRange(v, minValue, maxValue) {
				c = 1
			}
			bb.B = append(bb.B, c)
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := valuesEncoded[idx][0]
			return bb.B[n] == 1
		})
		bbPool.Put(bb)
	case valueTypeUint8:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint8(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint16:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint16(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint32:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := uint64(unmarshalUint32(v))
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeUint64:
		minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
		if maxValue < 0 || minValueUint > c.maxValue || maxValueUint < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := unmarshalUint64(v)
			return n >= minValueUint && n <= maxValueUint
		})
	case valueTypeInt64:
		minValueInt, maxValueInt := toInt64Range(minValue, maxValue)
		if minValueInt > int64(c.maxValue) || maxValueInt < int64(c.minValue) {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := unmarshalInt64(v)
			return n >= minValueInt && n <= maxValueInt
		})
	case valueTypeFloat64:
		if minValue > math.Float64frombits(c.maxValue) || maxValue < math.Float64frombits(c.minValue) {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			f := unmarshalFloat64(v)
			return f >= minValue && f <= maxValue
		})
	case valueTypeIPv4:
		minValueUint32, maxValueUint32 := toUint32Range(minValue, maxValue)
		if maxValue < 0 || uint64(minValueUint32) > c.maxValue || uint64(maxValueUint32) < c.minValue {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := unmarshalIPv4(v)
			return n >= minValueUint32 && n <= maxValueUint32
		})
	case valueTypeTimestampISO8601:
		minValueInt, maxValueInt := toInt64Range(minValue, maxValue)
		if maxValue < 0 || minValueInt > int64(c.maxValue) || maxValueInt < int64(c.minValue) {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			v := valuesEncoded[idx]
			n := unmarshalTimestampISO8601(v)
			return n >= minValueInt && n <= maxValueInt
		})
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fr *filterRange) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minValue := fr.minValue
	maxValue := fr.maxValue

	if minValue > maxValue {
		bm.resetBits()
		return
	}

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchRange(v, minValue, maxValue) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeDict:
		matchValuesDictByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint8:
		matchUint8ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint16:
		matchUint16ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint32:
		matchUint32ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeUint64:
		matchUint64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeInt64:
		matchInt64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeFloat64:
		matchFloat64ByRange(bs, ch, bm, minValue, maxValue)
	case valueTypeIPv4:
		minValueUint32, maxValueUint32 := toUint32Range(minValue, maxValue)
		matchIPv4ByRange(bs, ch, bm, minValueUint32, maxValueUint32)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByRange(bs, ch, bm, minValue, maxValue)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchFloat64ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	if minValue > math.Float64frombits(ch.maxValue) || maxValue < math.Float64frombits(ch.minValue) {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
		}
		f := unmarshalFloat64(v)
		return f >= minValue && f <= maxValue
	})
}

func matchValuesDictByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchRange(v, minValue, maxValue) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchRange(v, minValue, maxValue)
	})
}

func matchUint8ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 1 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 1", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint8(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint16ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 2 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint16 number: got %d; want 2", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint16(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint32ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 4 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint32 number: got %d; want 4", bs.partPath(), len(v))
		}
		n := uint64(unmarshalUint32(v))
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchUint64ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueUint, maxValueUint := toUint64Range(minValue, maxValue)
	if maxValue < 0 || minValueUint > ch.maxValue || maxValueUint < ch.minValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of uint64 number: got %d; want 8", bs.partPath(), len(v))
		}
		n := unmarshalUint64(v)
		return n >= minValueUint && n <= maxValueUint
	})
	bbPool.Put(bb)
}

func matchInt64ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueInt, maxValueInt := toInt64Range(minValue, maxValue)
	if minValueInt > int64(ch.maxValue) || maxValueInt < int64(ch.minValue) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of int64 number; got %d; want 8", bs.partPath(), len(v))
		}
		n := unmarshalInt64(v)
		return n >= minValueInt && n <= maxValueInt
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601ByRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minValue, maxValue float64) {
	minValueInt, maxValueInt := toInt64Range(minValue, maxValue)
	if maxValue < 0 || minValueInt > int64(ch.maxValue) || maxValueInt < int64(ch.minValue) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 8 {
			logger.Panicf("FATAL: %s: unexpected length for binary representation of timestampISO8601: got %d; want 8", bs.partPath(), len(v))
		}
		n := unmarshalTimestampISO8601(v)
		return n >= minValueInt && n <= maxValueInt
	})
	bbPool.Put(bb)
}

func matchRange(s string, minValue, maxValue float64) bool {
	f := parseMathNumber(s)
	return f >= minValue && f <= maxValue
}

func toUint64Range(minValue, maxValue float64) (uint64, uint64) {
	minValue = math.Ceil(minValue)
	maxValue = math.Floor(maxValue)
	return toUint64Clamp(minValue), toUint64Clamp(maxValue)
}

func toUint64Clamp(f float64) uint64 {
	if f < 0 {
		return 0
	}
	if f > math.MaxUint64 {
		return math.MaxUint64
	}
	return uint64(f)
}

func toInt64Range(minValue, maxValue float64) (int64, int64) {
	minValue = math.Ceil(minValue)
	maxValue = math.Floor(maxValue)
	return toInt64Clamp(minValue), toInt64Clamp(maxValue)
}

func toInt64Clamp(f float64) int64 {
	if f < math.MinInt64 {
		return math.MinInt64
	}
	if f > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(f)
}

func toUint32Range(minValue, maxValue float64) (uint32, uint32) {
	minValue = math.Ceil(minValue)
	maxValue = math.Floor(maxValue)
	return toUint32Clamp(minValue), toUint32Clamp(maxValue)
}

func toUint32Clamp(f float64) uint32 {
	if f < 0 {
		return 0
	}
	if f > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(f)
}
