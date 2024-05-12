package logstorage

import (
	"fmt"
	"regexp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterRegexp matches the given regexp
//
// Example LogsQL: `fieldName:re("regexp")`
type filterRegexp struct {
	fieldName string
	re        *regexp.Regexp
}

func (fr *filterRegexp) String() string {
	return fmt.Sprintf("%sre(%q)", quoteFieldNameIfNeeded(fr.fieldName), fr.re.String())
}

func (fr *filterRegexp) apply(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	re := fr.re

	// Verify whether filter matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !re.MatchString(v) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !re.MatchString("") {
			bm.resetBits()
		}
		return
	}

	switch ch.valueType {
	case valueTypeString:
		matchStringByRegexp(bs, ch, bm, re)
	case valueTypeDict:
		matchValuesDictByRegexp(bs, ch, bm, re)
	case valueTypeUint8:
		matchUint8ByRegexp(bs, ch, bm, re)
	case valueTypeUint16:
		matchUint16ByRegexp(bs, ch, bm, re)
	case valueTypeUint32:
		matchUint32ByRegexp(bs, ch, bm, re)
	case valueTypeUint64:
		matchUint64ByRegexp(bs, ch, bm, re)
	case valueTypeFloat64:
		matchFloat64ByRegexp(bs, ch, bm, re)
	case valueTypeIPv4:
		matchIPv4ByRegexp(bs, ch, bm, re)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByRegexp(bs, ch, bm, re)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchIPv4ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchFloat64ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64StringExt(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchValuesDictByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if re.MatchString(v) {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	visitValues(bs, ch, bm, func(v string) bool {
		return re.MatchString(v)
	})
}

func matchUint8ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint16ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint32ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint64ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexp.Regexp) {
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}
