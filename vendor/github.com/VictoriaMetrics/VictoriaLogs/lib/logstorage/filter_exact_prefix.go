package logstorage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterExactPrefix matches the exact prefix.
//
// Example LogsQL: `="foo bar"*`
type filterExactPrefix struct {
	prefix string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func newFilterExactPrefix(fieldName, prefix string) *filterGeneric {
	fe := &filterExactPrefix{
		prefix: prefix,
	}
	return newFilterGeneric(fieldName, fe)
}

func (fep *filterExactPrefix) String() string {
	return fmt.Sprintf("=%s*", quoteTokenIfNeeded(fep.prefix))
}

func (fep *filterExactPrefix) getTokens() []string {
	fep.tokensOnce.Do(fep.initTokens)
	return fep.tokens
}

func (fep *filterExactPrefix) getTokensHashes() []uint64 {
	fep.tokensOnce.Do(fep.initTokens)
	return fep.tokensHashes
}

func (fep *filterExactPrefix) initTokens() {
	fep.tokens = getTokensSkipLast(fep.prefix)
	fep.tokensHashes = appendTokensHashes(nil, fep.tokens)
}

func (fep *filterExactPrefix) matchRowByField(fields []Field, fieldName string) bool {
	v := getFieldValueByName(fields, fieldName)
	return matchExactPrefix(v, fep.prefix)
}

func (fep *filterExactPrefix) applyToBlockResultByField(br *blockResult, bm *bitmap, fieldName string) {
	applyToBlockResultGeneric(br, bm, fieldName, fep.prefix, matchExactPrefix)
}

func (fep *filterExactPrefix) applyToBlockSearchByField(bs *blockSearch, bm *bitmap, fieldName string) {
	prefix := fep.prefix

	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchExactPrefix(v, prefix) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !matchExactPrefix("", prefix) {
			bm.resetBits()
		}
		return
	}

	tokens := fep.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeDict:
		matchValuesDictByExactPrefix(bs, ch, bm, prefix)
	case valueTypeUint8:
		matchUint8ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint16:
		matchUint16ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint32:
		matchUint32ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint64:
		matchUint64ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeInt64:
		matchInt64ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeFloat64:
		matchFloat64ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeIPv4:
		matchIPv4ByExactPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByExactPrefix(bs, ch, bm, prefix, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		return
	}
	if prefix < "0" || prefix > "9" || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchIPv4ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		return
	}
	if prefix < "0" || prefix > "9" || len(tokens) > 3*bloomFilterHashesCount || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchFloat64ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// An empty prefix matches all the values
		return
	}
	if len(tokens) > 2*bloomFilterHashesCount || !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchValuesDictByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchExactPrefix(v, prefix) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchExactPrefix(v, prefix)
	})
}

func matchUint8ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint16ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint32ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint64ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchMinMaxExactPrefix(ch, bm, prefix, tokens) {
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchInt64ByExactPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// An empty prefix matches all the values
		return
	}
	if len(tokens) > 0 {
		// Non-empty tokens means that the prefix contains at least two tokens.
		// Multiple tokens cannot match any uint value.
		bm.resetBits()
		return
	}
	if prefix != "-" {
		n, ok := tryParseInt64(prefix)
		if !ok || n > int64(ch.maxValue) || n < int64(ch.minValue) {
			bm.resetBits()
			return
		}
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toInt64String(bs, bb, v)
		return matchExactPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchMinMaxExactPrefix(ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) bool {
	if prefix == "" {
		// An empty prefix matches all the values
		return false
	}
	if len(tokens) > 0 {
		// Non-empty tokens means that the prefix contains at least two tokens.
		// Multiple tokens cannot match any uint value.
		bm.resetBits()
		return false
	}
	n, ok := tryParseUint64(prefix)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return false
	}
	return true
}

func matchExactPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}
