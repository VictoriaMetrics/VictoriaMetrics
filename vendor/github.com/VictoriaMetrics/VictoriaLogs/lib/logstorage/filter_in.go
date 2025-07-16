package logstorage

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterIn matches any exact value from the values map.
//
// Example LogsQL: `fieldName:in("foo", "bar baz")`
type filterIn struct {
	fieldName string

	values inValues
}

func (fi *filterIn) String() string {
	args := fi.values.String()
	return fmt.Sprintf("%sin(%s)", quoteFieldNameIfNeeded(fi.fieldName), args)
}

func (fi *filterIn) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fi.fieldName)
}

func (fi *filterIn) applyToBlockResult(br *blockResult, bm *bitmap) {
	if fi.values.isEmpty() {
		bm.resetBits()
		return
	}

	c := br.getColumnByName(fi.fieldName)
	if c.isConst {
		stringValues := fi.values.getStringValues()
		v := c.valuesEncoded[0]
		if _, ok := stringValues[v]; !ok {
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
		stringValues := fi.values.getStringValues()
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if _, ok := stringValues[v]; ok {
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
		binValues := fi.values.getInt64Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeFloat64:
		binValues := fi.values.getFloat64Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeIPv4:
		binValues := fi.values.getIPv4Values()
		matchColumnByBinValues(br, bm, c, binValues)
	case valueTypeTimestampISO8601:
		binValues := fi.values.getTimestampISO8601Values()
		matchColumnByBinValues(br, bm, c, binValues)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func (fi *filterIn) matchColumnByStringValues(br *blockResult, bm *bitmap, c *blockResultColumn) {
	stringValues := fi.values.getStringValues()
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		v := values[idx]
		_, ok := stringValues[v]
		return ok
	})
}

func matchColumnByBinValues(br *blockResult, bm *bitmap, c *blockResultColumn, binValues map[string]struct{}) {
	if len(binValues) == 0 {
		bm.resetBits()
		return
	}
	valuesEncoded := c.getValuesEncoded(br)
	bm.forEachSetBit(func(idx int) bool {
		v := valuesEncoded[idx]
		_, ok := binValues[v]
		return ok
	})
}

func (fi *filterIn) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fi.fieldName

	if fi.values.isEmpty() {
		bm.resetBits()
		return
	}

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		stringValues := fi.values.getStringValues()
		if _, ok := stringValues[v]; !ok {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		stringValues := fi.values.getStringValues()
		if _, ok := stringValues[""]; !ok {
			bm.resetBits()
		}
		return
	}

	commonTokens, tokenSets := fi.values.getTokensHashesAny()

	switch ch.valueType {
	case valueTypeString:
		stringValues := fi.values.getStringValues()
		matchAnyValue(bs, ch, bm, stringValues, commonTokens, tokenSets)
	case valueTypeDict:
		stringValues := fi.values.getStringValues()
		matchValuesDictByAnyValue(bs, ch, bm, stringValues)
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
		binValues := fi.values.getInt64Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeFloat64:
		binValues := fi.values.getFloat64Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeIPv4:
		binValues := fi.values.getIPv4Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	case valueTypeTimestampISO8601:
		binValues := fi.values.getTimestampISO8601Values()
		matchAnyValue(bs, ch, bm, binValues, commonTokens, tokenSets)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchAnyValue(bs *blockSearch, ch *columnHeader, bm *bitmap, binValues map[string]struct{}, commonTokens []uint64, tokenSets [][]uint64) {
	if len(binValues) == 0 {
		bm.resetBits()
		return
	}
	if !matchBloomFilterAnyTokenSet(bs, ch, commonTokens, tokenSets) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		_, ok := binValues[v]
		return ok
	})
}

func matchBloomFilterAnyTokenSet(bs *blockSearch, ch *columnHeader, commonTokens []uint64, tokenSets [][]uint64) bool {
	if !matchBloomFilterAllTokens(bs, ch, commonTokens) {
		return false
	}
	if len(tokenSets) > maxTokenSetsToInit || uint64(len(tokenSets)) > 10*bs.bsw.bh.rowsCount {
		// It is faster to match every row in the block against all the values
		// instead of using bloom filter for too big number of tokenSets.
		return true
	}
	bf := bs.getBloomFilterForColumn(ch)
	for _, tokens := range tokenSets {
		if bf.containsAll(tokens) {
			return true
		}
	}
	return false
}

// It is faster to match every row in the block instead of checking too big number of tokenSets against bloom filter.
const maxTokenSetsToInit = 1000

func matchValuesDictByAnyValue(bs *blockSearch, ch *columnHeader, bm *bitmap, values map[string]struct{}) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if _, ok := values[v]; ok {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}
