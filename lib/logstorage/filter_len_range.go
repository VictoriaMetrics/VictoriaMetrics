package logstorage

import (
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

func (fr *filterLenRange) apply(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	minLen := fr.minLen
	maxLen := fr.maxLen

	if minLen > maxLen {
		bm.resetBits()
		return
	}

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchLenRange(v, minLen, maxLen) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
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
		s := toIPv4StringExt(bs, bb, v)
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
		s := toFloat64StringExt(bs, bb, v)
		return matchLenRange(s, minLen, maxLen)
	})
	bbPool.Put(bb)
}

func matchValuesDictByLenRange(bs *blockSearch, ch *columnHeader, bm *bitmap, minLen, maxLen uint64) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if matchLenRange(v, minLen, maxLen) {
			bb.B = append(bb.B, byte(i))
		}
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

func matchLenRange(s string, minLen, maxLen uint64) bool {
	sLen := uint64(utf8.RuneCountInString(s))
	return sLen >= minLen && sLen <= maxLen
}
