package logstorage

import (
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterLenRange matches field values with the length in the given range [minLen, maxLen].
//
// Example LogsQL: `fieldName:len_range(10, 20)`
type filterLenRange struct {
	fieldName string
	minLen    uint64
	maxLen    uint64

	stringRepr string
}

func (fr *filterLenRange) String() string {
	return quoteFieldNameIfNeeded(fr.fieldName) + "len_range" + fr.stringRepr
}

func (fr *filterLenRange) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fr.fieldName)
}

func (fr *filterLenRange) applyToBlockResult(br *blockResult, bm *bitmap) {
	minLen := fr.minLen
	maxLen := fr.maxLen

	if minLen > maxLen {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fr.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchLenRange(v, minLen, maxLen) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	}

	switch c.valueType {
	case valueTypeString:
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchLenRange(v, minLen, maxLen) {
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
		if minLen > 3 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeUint16:
		if minLen > 5 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeUint32:
		if minLen > 10 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeUint64:
		if minLen > 20 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeInt64:
		if minLen > 21 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeFloat64:
		if minLen > 24 || maxLen == 0 {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeIPv4:
		if minLen > uint64(len("255.255.255.255")) || maxLen < uint64(len("0.0.0.0")) {
			bm.resetBits()
			return
		}
		matchColumnByLenRange(br, bm, c, minLen, maxLen)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByLenRange(bm, minLen, maxLen)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func matchColumnByLenRange(br *blockResult, bm *bitmap, c *blockResultColumn, minLen, maxLen uint64) {
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		v := values[idx]
		return matchLenRange(v, minLen, maxLen)
	})
}

func (fr *filterLenRange) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minLen := fr.minLen
	maxLen := fr.maxLen

	if minLen > maxLen {
		bm.resetBits()
		return
	}

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchLenRange(v, minLen, maxLen) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !matchLenRange("", minLen, maxLen) {
			bm.resetBits()
		}
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeDict:
		matchValuesDictByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint8:
		matchUint8ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint16:
		matchUint16ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint32:
		matchUint32ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeUint64:
		matchUint64ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeInt64:
		matchInt64ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeFloat64:
		matchFloat64ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeIPv4:
		matchIPv4ByLenRange(bs, ch, bm, minLen, maxLen)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByLenRange(bm, minLen, maxLen)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByLenRange(bm *bitmap, minLen, maxLen uint64) {
	if minLen > uint64(len(iso8601Timestamp)) || maxLen < uint64(len(iso8601Timestamp)) {
		bm.resetBits()
		return
	}
}

func matchIPv4ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > uint64(len("255.255.255.255")) || maxLen < uint64(len("0.0.0.0")) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchFloat64ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 24 || maxLen == 0 {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchValuesDictByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchLenRange(v, minLen, maxLen) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchLenRange(v, minLen, maxLen)
	})
}

func matchUint8ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 3 || maxLen == 0 {
		bm.resetBits()
		return
	}
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint16ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 5 || maxLen == 0 {
		bm.resetBits()
		return
	}
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint32ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 10 || maxLen == 0 {
		bm.resetBits()
		return
	}
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchUint64ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 20 || maxLen == 0 {
		bm.resetBits()
		return
	}
	if !matchMinMaxValueLen(ch, minLen, maxLen) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchInt64ByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	if minLen > 21 || maxLen == 0 {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()

	bb.B = marshalInt64String(bb.B[:0], int64(ch.minValue))
	maxvLen := len(bb.B)
	bb.B = marshalInt64String(bb.B[:0], int64(ch.maxValue))
	if len(bb.B) > maxvLen {
		maxvLen = len(bb.B)
	}
	if uint64(maxvLen) < minLen {
		bm.resetBits()
		return
	}

	visitValues(bs, ch, bm, func(v string) bool {
		s := toInt64String(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})

	bbPool.Put(bb)
}

func matchLenRange(s string, minLen, maxLen uint64) bool {
	sLen := uint64(utf8.RuneCountInString(s))
	return sLen >= minLen && sLen <= maxLen
}

func matchMinMaxValueLen(ch *columnHeader, minLen, maxLen uint64) bool {
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = marshalUint64String(bb.B[:0], ch.minValue)
	if maxLen < uint64(len(bb.B)) {
		return false
	}
	bb.B = marshalUint64String(bb.B[:0], ch.maxValue)
	return minLen <= uint64(len(bb.B))
}
