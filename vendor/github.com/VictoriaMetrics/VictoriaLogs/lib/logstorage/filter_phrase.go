package logstorage

import (
	"bytes"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterPhrase filters field entries by phrase match (aka full text search).
//
// A phrase consists of any number of words with delimiters between them.
//
// An empty phrase matches only an empty string.
// A single-word phrase is the simplest LogsQL query: `fieldName:word`
//
// Multi-word phrase is expressed as `fieldName:"word1 ... wordN"` in LogsQL.
//
// A special case `fieldName:""` matches any value without `fieldName` field.
type filterPhrase struct {
	fieldName string
	phrase    string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func (fp *filterPhrase) String() string {
	return quoteFieldNameIfNeeded(fp.fieldName) + quoteTokenIfNeeded(fp.phrase)
}

func (fp *filterPhrase) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fp.fieldName)
}

func (fp *filterPhrase) getTokens() []string {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokens
}

func (fp *filterPhrase) getTokensHashes() []uint64 {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokensHashes
}

func (fp *filterPhrase) initTokens() {
	fp.tokens = tokenizeStrings(nil, []string{fp.phrase})
	fp.tokensHashes = appendTokensHashes(nil, fp.tokens)
}

func (fp *filterPhrase) applyToBlockResult(br *blockResult, bm *bitmap) {
	applyToBlockResultGeneric(br, bm, fp.fieldName, fp.phrase, matchPhrase)
}

func (fp *filterPhrase) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fp.fieldName
	phrase := fp.phrase

	// Verify whether fp matches const column
	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !matchPhrase(v, phrase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether fp matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if len(phrase) > 0 {
			bm.resetBits()
		}
		return
	}

	tokens := fp.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeDict:
		matchValuesDictByPhrase(bs, ch, bm, phrase)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeInt64:
		matchInt64ByExactValue(bs, ch, bm, phrase, tokens)
	case valueTypeFloat64:
		matchFloat64ByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPhrase(bs, ch, bm, phrase, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByPhrase(bs, ch, bm, phrase, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	_, ok := tryParseTimestampISO8601(phrase)
	if ok {
		// Fast path - the phrase contains complete timestamp, so we can use exact search
		matchTimestampISO8601ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}

	// Slow path - the phrase contains incomplete timestamp. Search over string representation of the timestamp.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchIPv4ByPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	_, ok := tryParseIPv4(phrase)
	if ok {
		// Fast path - phrase contains the full IP address, so we can use exact matching
		matchIPv4ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}

	// Slow path - the phrase may contain a part of IP address. For example, `1.23` should match `1.23.4.5` and `4.1.23.54`.
	// We cannot compare binary represetnation of ip address and need converting
	// the ip to string before searching for prefix there.
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchFloat64ByPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	// The phrase may contain a part of the floating-point number.
	// For example, `foo:"123"` must match `123`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole phrase in string representation
	// of floating-point numbers :(
	_, ok := tryParseFloat64Exact(phrase)
	if !ok && phrase != "." && phrase != "+" && phrase != "-" {
		bm.resetBits()
		return
	}
	if n := strings.IndexByte(phrase, '.'); n > 0 && n < len(phrase)-1 {
		// Fast path - the phrase contains the exact floating-point number, so we can use exact search
		matchFloat64ByExactValue(bs, ch, bm, phrase, tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return matchPhrase(s, phrase)
	})
	bbPool.Put(bb)
}

func matchValuesDictByPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchPhrase(v, phrase) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByPhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phrase string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchPhrase(v, phrase)
	})
}

func matchPhrase(s, phrase string) bool {
	if len(phrase) == 0 {
		// Special case - empty phrase matches only empty string.
		return len(s) == 0
	}
	n := getPhrasePos(s, phrase)
	return n >= 0
}

func getPhrasePos(s, phrase string) int {
	if len(phrase) == 0 {
		return 0
	}
	if len(phrase) > len(s) {
		return -1
	}

	r := rune(phrase[0])
	if r >= utf8.RuneSelf {
		r, _ = utf8.DecodeRuneInString(phrase)
	}
	startsWithToken := isTokenRune(r)

	r = rune(phrase[len(phrase)-1])
	if r >= utf8.RuneSelf {
		r, _ = utf8.DecodeLastRuneInString(phrase)
	}
	endsWithToken := isTokenRune(r)

	pos := 0
	for {
		n := strings.Index(s[pos:], phrase)
		if n < 0 {
			return -1
		}
		pos += n
		// Make sure that the found phrase contains non-token chars at the beginning and at the end
		if startsWithToken && pos > 0 {
			r := rune(s[pos-1])
			if r >= utf8.RuneSelf {
				r, _ = utf8.DecodeLastRuneInString(s[:pos])
			}
			if r == utf8.RuneError || isTokenRune(r) {
				pos++
				continue
			}
		}
		if endsWithToken && pos+len(phrase) < len(s) {
			r := rune(s[pos+len(phrase)])
			if r >= utf8.RuneSelf {
				r, _ = utf8.DecodeRuneInString(s[pos+len(phrase):])
			}
			if r == utf8.RuneError || isTokenRune(r) {
				pos++
				continue
			}
		}
		return pos
	}
}

func matchEncodedValuesDict(bs *blockSearch, ch *columnHeader, bm *bitmap, encodedValues []byte) {
	if bytes.IndexByte(encodedValues, 1) < 0 {
		// Fast path - the phrase is missing in the valuesDict
		bm.resetBits()
		return
	}
	// Slow path - iterate over values
	visitValues(bs, ch, bm, func(v string) bool {
		if len(v) != 1 {
			logger.Panicf("FATAL: %s: unexpected length for dict value: got %d; want 1", bs.partPath(), len(v))
		}
		idx := v[0]
		if int(idx) >= len(encodedValues) {
			logger.Panicf("FATAL: %s: too big index for dict value; got %d; must be smaller than %d", bs.partPath(), idx, len(encodedValues))
		}
		return encodedValues[idx] == 1
	})
}

func visitValues(bs *blockSearch, ch *columnHeader, bm *bitmap, f func(value string) bool) {
	if bm.isZero() {
		// Fast path - nothing to visit
		return
	}
	values := bs.getValuesForColumn(ch)
	bm.forEachSetBit(func(idx int) bool {
		return f(values[idx])
	})
}

func matchBloomFilterAllTokens(bs *blockSearch, ch *columnHeader, tokens []uint64) bool {
	if len(tokens) == 0 {
		return true
	}
	bf := bs.getBloomFilterForColumn(ch)
	return bf.containsAll(tokens)
}

func quoteFieldNameIfNeeded(s string) string {
	if isMsgFieldName(s) {
		return ""
	}
	return quoteTokenIfNeeded(s) + ":"
}

func isMsgFieldName(fieldName string) bool {
	return fieldName == "" || fieldName == "_msg"
}

func toFloat64String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of floating-point number: got %d; want 8", bs.partPath(), len(v))
	}
	f := unmarshalFloat64(v)
	bb.B = marshalFloat64String(bb.B[:0], f)
	return bytesutil.ToUnsafeString(bb.B)
}

func toIPv4String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 4 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of IPv4: got %d; want 4", bs.partPath(), len(v))
	}
	ip := unmarshalIPv4(v)
	bb.B = marshalIPv4String(bb.B[:0], ip)
	return bytesutil.ToUnsafeString(bb.B)
}

func toTimestampISO8601String(bs *blockSearch, bb *bytesutil.ByteBuffer, v string) string {
	if len(v) != 8 {
		logger.Panicf("FATAL: %s: unexpected length for binary representation of ISO8601 timestamp: got %d; want 8", bs.partPath(), len(v))
	}
	timestamp := unmarshalTimestampISO8601(v)
	bb.B = marshalTimestampISO8601String(bb.B[:0], timestamp)
	return bytesutil.ToUnsafeString(bb.B)
}

func applyToBlockResultGeneric(br *blockResult, bm *bitmap, fieldName, phrase string, matchFunc func(v, phrase string) bool) {
	c := br.getColumnByName(fieldName)
	if c.isConst {
		v := c.valuesEncoded[0]
		if !matchFunc(v, phrase) {
			bm.resetBits()
		}
		return
	}
	if c.isTime {
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
		return
	}

	switch c.valueType {
	case valueTypeString:
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeDict:
		bb := bbPool.Get()
		for _, v := range c.dictValues {
			c := byte(0)
			if matchFunc(v, phrase) {
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
		n, ok := tryParseUint64(phrase)
		if !ok || n >= (1<<8) {
			bm.resetBits()
			return
		}
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeUint16:
		n, ok := tryParseUint64(phrase)
		if !ok || n >= (1<<16) {
			bm.resetBits()
			return
		}
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeUint32:
		n, ok := tryParseUint64(phrase)
		if !ok || n >= (1<<32) {
			bm.resetBits()
			return
		}
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeUint64:
		_, ok := tryParseUint64(phrase)
		if !ok {
			bm.resetBits()
			return
		}
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeInt64:
		_, ok := tryParseInt64(phrase)
		if !ok {
			bm.resetBits()
			return
		}
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeFloat64:
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeIPv4:
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	case valueTypeTimestampISO8601:
		matchColumnByPhraseGeneric(br, bm, c, phrase, matchFunc)
	default:
		logger.Panicf("FATAL: unknown valueType=%d", c.valueType)
	}
}

func matchColumnByPhraseGeneric(br *blockResult, bm *bitmap, c *blockResultColumn, phrase string, matchFunc func(v, phrase string) bool) {
	values := c.getValues(br)
	bm.forEachSetBit(func(idx int) bool {
		return matchFunc(values[idx], phrase)
	})
}
