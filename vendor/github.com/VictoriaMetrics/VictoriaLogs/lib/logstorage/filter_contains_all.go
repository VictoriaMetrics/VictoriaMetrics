package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterContainsAll matches logs containing all the given values.
//
// Example LogsQL: `fieldName:contains_all("foo", "bar baz")`
type filterContainsAll struct {
	fieldName string

	values inValues
}

func (fi *filterContainsAll) String() string {
	args := fi.values.String()
	return fmt.Sprintf("%scontains_all(%s)", quoteFieldNameIfNeeded(fi.fieldName), args)
}

func (fi *filterContainsAll) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fi.fieldName)
}

func (fi *filterContainsAll) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fi.values.isEmpty() || fi.values.isOnlyEmptyValue() {
		return
	}

	c := br.getColumnByName(fi.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchAllPhrases(v, fi.values.values) {
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
			if matchAllPhrases(v, phrases) {
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
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchColumnByAllBinValues(br, bm, c, binValues, nonEmptyValuesLen)
	case valueTypeUint16:
		binValues := fi.values.getUint16Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchColumnByAllBinValues(br, bm, c, binValues, nonEmptyValuesLen)
	case valueTypeUint32:
		binValues := fi.values.getUint32Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchColumnByAllBinValues(br, bm, c, binValues, nonEmptyValuesLen)
	case valueTypeUint64:
		binValues := fi.values.getUint64Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchColumnByAllBinValues(br, bm, c, binValues, nonEmptyValuesLen)
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

func matchColumnByAllBinValues(br *blockResult, bm *bitmap, c *blockResultColumn, binValues map[string]struct{}, nonEmptyValuesLen int) {
	if nonEmptyValuesLen == 0 {
		return
	}
	if nonEmptyValuesLen != 1 || nonEmptyValuesLen != len(binValues) {
		bm.resetBits()
		return
	}
	binValue := ""
	for k := range binValues {
		binValue = k
	}

	valuesEncoded := c.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		return valuesEncoded[idx] == binValue
	})
}

func (fi *filterContainsAll) matchColumnByStringValues(br *blockResult, bm *bitmap, c *blockResultColumn) {
	phrases := fi.values.values
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return matchAllPhrases(values[idx], phrases)
	})
}

func (fi *filterContainsAll) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if fi.values.isEmpty() || fi.values.isOnlyEmptyValue() {
		return
	}

	v := bs.getConstColumnValue(fi.fieldName)
	if v != "" {
		if !matchAllPhrases(v, fi.values.values) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fi.fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if !matchAllPhrases("", fi.values.values) {
			bm.resetBits()
		}
		return
	}

	tokens := fi.values.getTokensHashesAll()

	switch ch.valueType {
	case valueTypeString:
		matchAllPhrasesString(bs, ch, bm, fi.values.values, tokens)
	case valueTypeDict:
		matchAllPhrasesDict(bs, ch, bm, fi.values.values)
	case valueTypeUint8:
		binValues := fi.values.getUint8Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchAllValues(bs, ch, bm, binValues, nonEmptyValuesLen, tokens)
	case valueTypeUint16:
		binValues := fi.values.getUint16Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchAllValues(bs, ch, bm, binValues, nonEmptyValuesLen, tokens)
	case valueTypeUint32:
		binValues := fi.values.getUint32Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchAllValues(bs, ch, bm, binValues, nonEmptyValuesLen, tokens)
	case valueTypeUint64:
		binValues := fi.values.getUint64Values()
		nonEmptyValuesLen := fi.values.getNonEmptyValuesLen()
		matchAllValues(bs, ch, bm, binValues, nonEmptyValuesLen, tokens)
	case valueTypeInt64:
		matchAllPhrasesInt64(bs, ch, bm, fi.values.values, tokens)
	case valueTypeFloat64:
		matchAllPhrasesFloat64(bs, ch, bm, fi.values.values, tokens)
	case valueTypeIPv4:
		matchAllPhrasesIPv4(bs, ch, bm, fi.values.values, tokens)
	case valueTypeTimestampISO8601:
		matchAllPhrasesTimestampISO8601(bs, ch, bm, fi.values.values, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchAllValues(bs *blockSearch, ch *columnHeader, bm *bitmap, binValues map[string]struct{}, nonEmptyValuesLen int, tokens []uint64) {
	if nonEmptyValuesLen == 0 {
		return
	}
	if nonEmptyValuesLen != 1 || nonEmptyValuesLen != len(binValues) {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	binValue := ""
	for k := range binValues {
		binValue = k
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == binValue
	})
}

func matchAllPhrasesString(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 0 {
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	values := bs.getValuesForColumn(ch)
	bm.forEachSetBit(func(idx int) bool {
		return matchAllPhrases(values[idx], phrases)
	})
}

func matchAllPhrasesInt64(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 0 {
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		n := unmarshalInt64(v)
		bb.B = marshalInt64String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAllPhrases(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAllPhrasesFloat64(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 0 {
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		n := unmarshalFloat64(v)
		bb.B = marshalFloat64String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAllPhrases(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAllPhrasesIPv4(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 0 {
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		n := unmarshalIPv4(v)
		bb.B = marshalIPv4String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAllPhrases(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAllPhrasesTimestampISO8601(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 0 {
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		n := unmarshalTimestampISO8601(v)
		bb.B = marshalTimestampISO8601String(bb.B[:0], n)
		s := bytesutil.ToUnsafeString(bb.B)
		return matchAllPhrases(s, phrases)
	})
	bbPool.Put(bb)
}

func matchAllPhrasesDict(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchAllPhrases(v, phrases) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchAllPhrases(v string, phrases []string) bool {
	for _, phrase := range phrases {
		if phrase == "" {
			// Special case - empty phrase matches everything
			continue
		}
		if !matchPhrase(v, phrase) {
			return false
		}
	}
	return true
}
