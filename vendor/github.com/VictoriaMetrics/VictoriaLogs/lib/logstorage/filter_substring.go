package logstorage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterSubstring filters field entries by substring match.
//
// An empty substring matches any string.
type filterSubstring struct {
	fieldName string
	substring string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func (fs *filterSubstring) String() string {
	return fmt.Sprintf("%s*%s*", quoteFieldNameIfNeeded(fs.fieldName), quoteTokenIfNeeded(fs.substring))
}

func (fs *filterSubstring) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fs.fieldName)
}

func (fs *filterSubstring) getTokens() []string {
	fs.tokensOnce.Do(fs.initTokens)
	return fs.tokens
}

func (fs *filterSubstring) getTokensHashes() []uint64 {
	fs.tokensOnce.Do(fs.initTokens)
	return fs.tokensHashes
}

func (fs *filterSubstring) initTokens() {
	s := skipFirstLastToken(fs.substring)
	fs.tokens = tokenizeStrings(nil, []string{s})
	fs.tokensHashes = appendTokensHashes(nil, fs.tokens)
}

func (fs *filterSubstring) applyToBlockResult(br *blockResult, bm *bitmap) {
	applyToBlockResultGeneric(br, bm, fs.fieldName, fs.substring, matchSubstring)
}

func (fs *filterSubstring) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fs.fieldName
	substring := fs.substring

	// Verify whether fs matches const column
	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchSubstring(v, substring) {
			bm.resetBits()
		}
		return
	}

	// Verify whether fs matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty substring.
		if len(substring) > 0 {
			bm.resetBits()
		}
		return
	}

	tokens := fs.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringBySubstring(bs, ch, bm, substring, tokens)
	case valueTypeDict:
		matchValuesDictBySubstring(bs, ch, bm, substring)
	case valueTypeUint8:
		matchUint8BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeUint16:
		matchUint16BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeUint32:
		matchUint32BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeUint64:
		matchUint64BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeInt64:
		matchInt64BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeFloat64:
		matchFloat64BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeIPv4:
		matchIPv4BySubstring(bs, ch, bm, substring, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601BySubstring(bs, ch, bm, substring, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchStringBySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchSubstring(v, substring)
	})
}

func matchValuesDictBySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchSubstring(v, substring) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchUint8BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the uint8 values match an empty substring
		return
	}
	// The substring may contain a part of the number.
	// For example, `*12*` matches `12`, `312`, `123` and `4123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole substring in string representation of numbers :(
	n, ok := tryParseUint64(substring)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchUint16BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the uint16 values match an empty substring
		return
	}
	// The substring may contain a part of the number.
	// For example, `*12*` matches `12`, `312`, `123` and `4123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole substring in string representation of numbers :(
	n, ok := tryParseUint64(substring)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchUint32BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the uint32 values match an empty substring
		return
	}
	// The substring may contain a part of the number.
	// For example, `*12*` matches `12`, `312`, `123` and `4123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole substring in string representation of numbers :(
	n, ok := tryParseUint64(substring)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchUint64BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the uint64 values match an empty substring
		return
	}
	// The substring may contain a part of the number.
	// For example, `*12*` matches `12`, `312`, `123` and `4123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole substring in string representation of numbers :(
	n, ok := tryParseUint64(substring)
	if !ok || n > ch.maxValue {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchInt64BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the int64 values match an empty substring
		return
	}
	// The substring may contain a part of the number.
	// For example, `*12*` matches `12`, `312`, `123` and `4123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole substring in string representation of numbers :(
	if substring != "-" {
		n, ok := tryParseInt64(substring)
		if !ok || n < int64(ch.minValue) || n > int64(ch.maxValue) {
			bm.resetBits()
			return
		}
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toInt64String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchFloat64BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the float64 values match an empty substring
		return
	}
	// The substring may contain a part of the floating-point number.
	// For example, `*12*` matches `12`, `123.456`, `-0.123`, `312` and `3124`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole substring in string representation
	// of floating-point numbers :(
	_, ok := tryParseFloat64Exact(substring)
	if !ok && !strings.Contains(substring, ".") && !strings.Contains(substring, "+") && !strings.Contains(substring, "-") && !strings.Contains(substring, "e") && !strings.Contains(substring, "E") {
		bm.resetBits()
		return
	}

	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchIPv4BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the ipv4 values match an empty substring
		return
	}
	// There is no sense in trying to parse substring, since it may contain incomplete ip.
	// We cannot compare binary representation of ip address and need converting
	// the ip to string before searching for the substring there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchTimestampISO8601BySubstring(bs *blockSearch, ch *columnHeader, bm *bitmap, substring string, tokens []uint64) {
	if substring == "" {
		// Fast path - all the timestamp values match an empty substring
		return
	}
	// There is no sense in trying to parse substring, since it may contain incomplete timestamp.
	// We cannot compare binary representation of timestamp and need converting
	// the timestamp to string before searching for the substring there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return matchSubstring(s, substring)
	})
	bbPool.Put(bb)
}

func matchSubstring(s, substring string) bool {
	if len(substring) == 0 {
		// Special case - empty substring matches anything.
		return true
	}
	if len(substring) > len(s) {
		// Fast path - the substring is too long
		return false
	}

	return strings.Contains(s, substring)
}
