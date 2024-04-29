package logstorage

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterExact matches the exact value.
//
// Example LogsQL: `fieldName:exact("foo bar")`
type filterExact struct {
	fieldName string
	value     string

	tokensOnce sync.Once
	tokens     []string
}

func (fe *filterExact) String() string {
	return fmt.Sprintf("%sexact(%s)", quoteFieldNameIfNeeded(fe.fieldName), quoteTokenIfNeeded(fe.value))
}

func (fe *filterExact) getTokens() []string {
	fe.tokensOnce.Do(fe.initTokens)
	return fe.tokens
}

func (fe *filterExact) initTokens() {
	fe.tokens = tokenizeStrings(nil, []string{fe.value})
}

func (fe *filterExact) apply(bs *blockSearch, bm *bitmap) {
	fieldName := fe.fieldName
	value := fe.value

	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if value != v {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty value.
		if value != "" {
			bm.resetBits()
		}
		return
	}

	tokens := fe.getTokens()

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

func matchTimestampISO8601ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []string) {
	n, ok := tryParseTimestampISO8601(value)
	if !ok || n < ch.minValue || n > ch.maxValue {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchIPv4ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []string) {
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

func matchFloat64ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []string) {
	f, ok := tryParseFloat64(value)
	if !ok || f < math.Float64frombits(ch.minValue) || f > math.Float64frombits(ch.maxValue) {
		bm.resetBits()
		return
	}
	n := math.Float64bits(f)
	bb := bbPool.Get()
	bb.B = encoding.MarshalUint64(bb.B, n)
	matchBinaryValue(bs, ch, bm, bb.B, tokens)
	bbPool.Put(bb)
}

func matchValuesDictByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string) {
	bb := bbPool.Get()
	for i, v := range ch.valuesDict.values {
		if v == value {
			bb.B = append(bb.B, byte(i))
		}
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, value string, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == value
	})
}

func matchUint8ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []string) {
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

func matchUint16ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []string) {
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

func matchUint32ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []string) {
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

func matchUint64ByExactValue(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []string) {
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

func matchBinaryValue(bs *blockSearch, ch *columnHeader, bm *bitmap, binValue []byte, tokens []string) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return v == string(binValue)
	})
}
