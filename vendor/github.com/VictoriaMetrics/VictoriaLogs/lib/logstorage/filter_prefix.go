package logstorage

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterPrefix matches the given prefix.
//
// Example LogsQL: `fieldName:prefix*` or `fieldName:"some prefix"*`
//
// A special case `fieldName:*` matches non-empty value for the given `fieldName` field
type filterPrefix struct {
	fieldName string
	prefix    string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func (fp *filterPrefix) String() string {
	if fp.prefix == "" {
		return quoteFieldNameIfNeeded(fp.fieldName) + "*"
	}
	return fmt.Sprintf("%s%s*", quoteFieldNameIfNeeded(fp.fieldName), quoteTokenIfNeeded(fp.prefix))
}

func (fp *filterPrefix) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fp.fieldName)
}

func (fp *filterPrefix) getTokens() []string {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokens
}

func (fp *filterPrefix) getTokensHashes() []uint64 {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokensHashes
}

func (fp *filterPrefix) initTokens() {
	fp.tokens = getTokensSkipLast(fp.prefix)
	fp.tokensHashes = appendTokensHashes(nil, fp.tokens)
}

func (fp *filterPrefix) applyToBlockResult(bs *blockResult, bm *bitmap) {
	applyToBlockResultGeneric(bs, bm, fp.fieldName, fp.prefix, matchPrefix)
}

func (fp *filterPrefix) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fp.fieldName
	prefix := fp.prefix

	// Verify whether fp matches const column
	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchPrefix(v, prefix) {
			bm.resetBits()
		}
		return
	}

	// Verify whether fp matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	tokens := fp.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeDict:
		matchValuesDictByPrefix(bs, ch, bm, prefix)
	case valueTypeUint8:
		matchUint8ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint16:
		matchUint16ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint32:
		matchUint32ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeUint64:
		matchUint64ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeInt64:
		matchInt64ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeFloat64:
		matchFloat64ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeIPv4:
		matchIPv4ByPrefix(bs, ch, bm, prefix, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByPrefix(bs, ch, bm, prefix, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the timestamp values match an empty prefix aka `*`
		return
	}
	// There is no sense in trying to parse prefix, since it may contain incomplete timestamp.
	// We cannot compare binary representation of timestamp and need converting
	// the timestamp to string before searching for the prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchIPv4ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the ipv4 values match an empty prefix aka `*`
		return
	}
	// There is no sense in trying to parse prefix, since it may contain incomplete ip.
	// We cannot compare binary representation of ip address and need converting
	// the ip to string before searching for the prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchFloat64ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the float64 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the floating-point number.
	// For example, `foo:12*` must match `12`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole prefix in string representation
	// of floating-point numbers :(
	_, ok := tryParseFloat64Exact(prefix)
	if !ok && prefix != "." && prefix != "+" && prefix != "-" && !strings.HasPrefix(prefix, "e") && !strings.HasPrefix(prefix, "E") {
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchValuesDictByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchPrefix(v, prefix) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchPrefix(v, prefix)
	})
}

func matchUint8ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the uint8 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint16ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the uint16 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint32ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the uint32 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchUint64ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the uint64 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	n, ok := tryParseUint64(prefix)
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchInt64ByPrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefix string, tokens []uint64) {
	if prefix == "" {
		// Fast path - all the int64 values match an empty prefix aka `*`
		return
	}
	// The prefix may contain a part of the number.
	// For example, `foo:12*` must match `12` and `123`.
	// This means we cannot search in binary representation of numbers.
	// Instead, we need searching for the whole prefix in string representation of numbers :(
	if prefix != "-" {
		n, ok := tryParseInt64(prefix)
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
		return matchPrefix(s, prefix)
	})
	bbPool.Put(bb)
}

func matchPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		// Special case - empty prefix matches any string.
		return len(s) > 0
	}
	if len(prefix) > len(s) {
		return false
	}

	r := rune(prefix[0])
	if r >= utf8.RuneSelf {
		r, _ = utf8.DecodeRuneInString(prefix)
	}
	startsWithToken := isTokenRune(r)
	offset := 0
	for {
		n := strings.Index(s[offset:], prefix)
		if n < 0 {
			return false
		}
		offset += n
		// Make sure that the found phrase contains non-token chars at the beginning
		if startsWithToken && offset > 0 {
			r := rune(s[offset-1])
			if r >= utf8.RuneSelf {
				r, _ = utf8.DecodeLastRuneInString(s[:offset])
			}
			if r == utf8.RuneError || isTokenRune(r) {
				offset++
				continue
			}
		}
		return true
	}
}

func getTokensSkipLast(s string) []string {
	s = skipLastToken(s)
	return tokenizeStrings(nil, []string{s})
}

func toUint8String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 1 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint8 number: got %d; want 1", bs.partPath(), len(v))
	}
	n := unmarshalUint8(v)
	bb.B = marshalUint8String(bb.B[:0], n)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint16String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 2 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint16 number: got %d; want 2", bs.partPath(), len(v))
	}
	n := unmarshalUint16(v)
	bb.B = marshalUint16String(bb.B[:0], n)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint32String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 4 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint32 number: got %d; want 4", bs.partPath(), len(v))
	}
	n := unmarshalUint32(v)
	bb.B = marshalUint32String(bb.B[:0], n)
	return bytesutil.ToUnsafeString(bb.B)
}

func toUint64String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of uint64 number: got %d; want 8", bs.partPath(), len(v))
	}
	n := unmarshalUint64(v)
	bb.B = marshalUint64String(bb.B[:0], n)
	return bytesutil.ToUnsafeString(bb.B)
}

func toInt64String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of int64 number; got %d; want 8", bs.partPath(), len(v))
	}
	n := unmarshalInt64(v)
	bb.B = marshalInt64String(bb.B[:0], n)
	return bytesutil.ToUnsafeString(bb.B)
}
