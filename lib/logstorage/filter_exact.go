package logstorage

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterExact matches the exact value.
//
// Example LogsQL: `fieldName:exact("foo bar")` of `fieldName:="foo bar"
type filterExact struct {
	fieldName string
	value     string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func (fe *filterExact) String() string {
	return fmt.Sprintf("%s=%s", quoteFieldNameIfNeeded(fe.fieldName), quoteTokenIfNeeded(fe.value))
}

func (fe *filterExact) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fe.fieldName)
}

func (fe *filterExact) getTokens() []string {
	fe.tokensOnce.Do(fe.initTokens)
	return fe.tokens
}

func (fe *filterExact) getTokensHashes() []uint64 {
	fe.tokensOnce.Do(fe.initTokens)
	return fe.tokensHashes
}

func (fe *filterExact) initTokens() {
	fe.tokens = tokenizeStrings(nil, []string{fe.value})
	fe.tokensHashes = appendTokensHashes(nil, fe.tokens)
}

func (fe *filterExact) applyToBlockResult(br *blockResult, bm *bitmap) {
	value := fe.value

	c := br.getColumnByName(fe.fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if v != value {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		matchColumnByExactValue(br, bm, c, value)
		return
	}

	switch c.valueType {
	case valueTypeString:
		matchColumnByExactValue(br, bm, c, value)
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if v == value {
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
		n, ok := tryParseUint64(value)
		if !ok || n >= (1<<8) {
			bm.resetBits()
			return
		}
		nNeeded := uint8(n)
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := unmarshalUint8(valuesEncoded[idx])
			return n == nNeeded
		})
	case valueTypeUint16:
		n, ok := tryParseUint64(value)
		if !ok || n >= (1<<16) {
			bm.resetBits()
			return
		}
		nNeeded := uint16(n)
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := unmarshalUint16(valuesEncoded[idx])
			return n == nNeeded
		})
	case valueTypeUint32:
		n, ok := tryParseUint64(value)
		if !ok || n >= (1<<32) {
			bm.resetBits()
			return
		}
		nNeeded := uint32(n)
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := unmarshalUint32(valuesEncoded[idx])
			return n == nNeeded
		})
	case valueTypeUint64:
		nNeeded, ok := tryParseUint64(value)
		if !ok {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := unmarshalUint64(valuesEncoded[idx])
			return n == nNeeded
		})
	case valueTypeInt64:
		nNeeded, ok := tryParseInt64(value)
		if !ok {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			n := unmarshalInt64(valuesEncoded[idx])
			return n == nNeeded
		})
	case valueTypeFloat64:
		fNeeded, ok := tryParseFloat64Exact(value)
		if !ok {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			f := unmarshalFloat64(valuesEncoded[idx])
			return f == fNeeded
		})
	case valueTypeIPv4:
		ipNeeded, ok := tryParseIPv4(value)
		if !ok {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			ip := unmarshalIPv4(valuesEncoded[idx])
			return ip == ipNeeded
		})
	case valueTypeTimestampISO8601:
		timestampNeeded, ok := tryParseTimestampISO8601(value)
		if !ok {
			bm.resetBits()
			return
		}
		valuesEncoded := c.getValuesEncoded(br)
		bm.forEachSetBit(func(idx int) bool {
			timestamp := unmarshalTimestampISO8601(valuesEncoded[idx])
			return timestamp == timestampNeeded
		})
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func matchColumnByExactValue(br *blockResult, bm *bitmap, c *blockResultColumn, value string) {
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return values[idx] == value
	})
}

func (fe *filterExact) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fe.fieldName
	value := fe.value

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if value != v {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty value.
		if value != "" {
			bm.resetBits()
		}
		return
	}

	tokens := fe.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringByExactValue(bs, ch, bm, value, tokens)
	case valueTypeDict:
		matchValuesDictByExactValue(bs, ch, bm, value)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeInt64:
		matchInt64ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeFloat64:
		matchFloat64ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeIPv4:
		matchIPv4ByExactValue(bs, ch, bm, value, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByExactValue(bs, ch, bm, value, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []uint64) {
	n, ok := tryParseTimestampISO8601(value)
	if !ok || n < int64(ch.minValue) || n > int64(ch.maxValue) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, uint64(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchIPv4ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []uint64) {
	n, ok := tryParseIPv4(value)
	if !ok || uint64(n) < ch.minValue || uint64(n) > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint32(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchFloat64ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []uint64) {
	f, ok := tryParseFloat64Exact(value)
	if !ok || f < math.Float64frombits(ch.minValue) || f > math.Float64frombits(ch.maxValue) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = marshalFloat64(bb.B, f)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchValuesDictByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if v == value {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == value
	})
}

func matchUint8ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = append(bb.B, byte(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint16ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint16(bb.B, uint16(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint32ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint32(bb.B, uint32(n))
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchUint64ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	n, ok := tryParseUint64(phrase)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchInt64ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	n, ok := tryParseInt64(phrase)
	if !ok || n < int64(ch.minValue) || n > int64(ch.maxValue) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalInt64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchBinaryValue(bs *blockSearch, ch *columnHeader, bm *bitmap, binValue []byte, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == string(binValue)
	})
}
