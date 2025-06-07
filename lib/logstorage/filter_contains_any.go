package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterContainsAny matches any value from the values.
//
// Example LogsQL: `fieldName:contains_any("foo", "bar baz")`
type filterContainsAny struct {
	fieldName string

	values inValues
}

func (fi *filterContainsAny) String() string {
	args := fi.values.String()
	return fmt.Sprintf("%scontains_any(%s)", quoteFieldNameIfNeeded(fi.fieldName), args)
}

func (fi *filterContainsAny) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fi.fieldName)
}

func (fi *filterContainsAny) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fi.values.isEmpty() {
		bm.resetBits()
		return
	}
	if fi.values.hasEmptyValue() {
		// Special case - empty value matches everything
		return
	}

	c := br.getColumnByName(fi.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchAnyPhrase(v, fi.values.values) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		fi.matchColumnByStringValues(br, bm, c)
		return
	}

	switch c.valueType {
	case valueTypeString:
		fi.matchColumnByStringValues(br, bm, c)
	case valueTypeDict:
		phrases := fi.values.values
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchAnyPhrase(v, phrases) {
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
		binValues := fi.values.getUint8Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint16:
		binValues := fi.values.getUint16Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint32:
		binValues := fi.values.getUint32Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeUint64:
		binValues := fi.values.getUint64Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeInt64:
		fi.matchColumnByStringValues(br, bm, c)
	case valueTypeFloat64:
		fi.matchColumnByStringValues(br, bm, c)
	case valueTypeIPv4:
		fi.matchColumnByStringValues(br, bm, c)
	case valueTypeTimestampISO8601:
		fi.matchColumnByStringValues(br, bm, c)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fi *filterContainsAny) matchColumnByStringValues(br *blockResult, bm *bitmap, c *blockResultColumn) {
	phrases := fi.values.values
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return matchAnyPhrase(values[idx], phrases)
	})
}

func (fi *filterContainsAny) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fi.values.isEmpty() {
		bm.resetBits()
		return
	}
	if fi.values.hasEmptyValue() {
		// Special case - empty value matches everything
		return
	}

	v := bs.getConstColumnValue(fi.fieldName)
	if v != "" {
		if !matchAnyPhrase(v, fi.values.values) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fi.fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if !matchAnyPhrase("", fi.values.values) {
			bm.resetBits()
		}
		return
	}

	commonTokens, tokenSets := fi.values.getTokensHashesAny()

	switch ch.valueType {
	case valueTypeString:
		matchAnyPhraseString(bs, ch, bm, fi.values.values, commonTokens, tokenSets)
	case valueTypeDict:
		matchAnyPhraseDict(bs, ch, bm, fi.values.values)
	case valueTypeUint8:
		binValues := fi.values.getUint8Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeUint16:
		binValues := fi.values.getUint16Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeUint32:
		binValues := fi.values.getUint32Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeUint64:
		binValues := fi.values.getUint64Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeInt64:
		matchAnyPhraseInt64(bs, ch, bm, fi.values.values, commonTokens, tokenSets)
	case valueTypeFloat64:
		matchAnyPhraseFloat64(bs, ch, bm, fi.values.values, commonTokens, tokenSets)
	case valueTypeIPv4:
		matchAnyPhraseIPv4(bs, ch, bm, fi.values.values, commonTokens, tokenSets)
	case valueTypeTimestampISO8601:
		matchAnyPhraseTimestampISO8601(bs, ch, bm, fi.values.values, commonTokens, tokenSets)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchAnyPhraseString(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, commonTokens []uint64, tokenSets [][]uint64) {
	if len(phrases) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		bm.resetBits()
		return
	}

	matchValuesAnyPhrase(bs, ch, bm, phrases, tokenSets, matchAnyPhrase)
}

func matchValuesAnyPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokenSets [][]uint64, matchPhraseFunc func(value string, phrases []string) bool) {
	bf := bs.getBloomFilterForColumn(ch)
	sb := getStringBucket()

	for i := range phrases {
		if bf.containsAll(tokenSets[i]) {
			sb.a = append(sb.a, phrases[i])
		}
	}
	if len(sb.a) > 0 {
		values := bs.getValuesForColumn(ch)
		bm.forEachSetBit(func(idx int) bool {
			return matchPhraseFunc(values[idx], sb.a)
		})
	} else {
		bm.resetBits()
	}

	putStringBucket(sb)
}

func matchAnyPhraseInt64(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, commonTokens []uint64, tokenSets [][]uint64) {
	if len(phrases) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	matchValuesAnyPhrase(bs, ch, bm, phrases, tokenSets, func(v string, phrases []string) bool {
		n := unmarshalInt64(v)
		bb.B = marshalInt64String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAnyPhrase(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAnyPhraseFloat64(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, commonTokens []uint64, tokenSets [][]uint64) {
	if len(phrases) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	matchValuesAnyPhrase(bs, ch, bm, phrases, tokenSets, func(v string, phrases []string) bool {
		n := unmarshalFloat64(v)
		bb.B = marshalFloat64String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAnyPhrase(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAnyPhraseIPv4(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, commonTokens []uint64, tokenSets [][]uint64) {
	if len(phrases) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	matchValuesAnyPhrase(bs, ch, bm, phrases, tokenSets, func(v string, phrases []string) bool {
		n := unmarshalIPv4(v)
		bb.B = marshalIPv4String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAnyPhrase(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAnyPhraseTimestampISO8601(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, commonTokens []uint64, tokenSets [][]uint64) {
	if len(phrases) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	matchValuesAnyPhrase(bs, ch, bm, phrases, tokenSets, func(v string, phrases []string) bool {
		n := unmarshalTimestampISO8601(v)
		bb.B = marshalTimestampISO8601String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAnyPhrase(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAnyPhraseDict(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchAnyPhrase(v, phrases) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchAnyPhrase(v string, phrases []string) bool {
	for _, phrase := range phrases {
		if matchPhrase(v, phrase) {
			return true
		}
	}
	return false
}
