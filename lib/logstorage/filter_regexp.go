package logstorage

import (
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
)

// filterRegexp matches the given regexp
//
// Example LogsQL: `fieldName:re("regexp")`
type filterRegexp struct {
	fieldName string
	re        *regexutil.Regex

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64
}

func (fr *filterRegexp) String() string {
	return fmt.Sprintf("%s~%s", quoteFieldNameIfNeeded(fr.fieldName), quoteTokenIfNeeded(fr.re.String()))
}

func (fr *filterRegexp) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fr.fieldName)
}

func (fr *filterRegexp) getTokens() []string {
	fr.tokensOnce.Do(fr.initTokens)
	return fr.tokens
}

func (fr *filterRegexp) getTokensHashes() []uint64 {
	fr.tokensOnce.Do(fr.initTokens)
	return fr.tokensHashes
}

func (fr *filterRegexp) initTokens() {
	literals := fr.re.GetLiterals()
	for i, literal := range literals {
		literals[i] = skipFirstLastToken(literal)
	}
	fr.tokens = tokenizeStrings(nil, literals)
	fr.tokensHashes = appendTokensHashes(nil, fr.tokens)
}

func skipFirstLastToken(s string) string {
	for {
		r, runeSize := utf8.DecodeRuneInString(s)
		if !isTokenRune(r) {
			break
		}
		s = s[runeSize:]
	}
	for {
		r, runeSize := utf8.DecodeLastRuneInString(s)
		if !isTokenRune(r) {
			break
		}
		s = s[:len(s)-runeSize]
	}
	return s
}

func (fr *filterRegexp) applyToBlockResult(br *blockResult, bm *bitmap) {
	re := fr.re
	applyToBlockResultGeneric(br, bm, fr.fieldName, "", func(v, _ string) bool {
		return re.MatchString(v)
	})
}

func (fr *filterRegexp) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fr.fieldName
	re := fr.re

	// Verify whether filter matches const column
	v := bs.getConstColumnValue(fieldName)
	if v != "" {
		if !re.MatchString(v) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := bs.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		if !re.MatchString("") {
			bm.resetBits()
		}
		return
	}

	tokens := fr.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringByRegexp(bs, ch, bm, re, tokens)
	case valueTypeDict:
		matchValuesDictByRegexp(bs, ch, bm, re)
	case valueTypeUint8:
		matchUint8ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeUint16:
		matchUint16ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeUint32:
		matchUint32ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeUint64:
		matchUint64ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeInt64:
		matchInt64ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeFloat64:
		matchFloat64ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeIPv4:
		matchIPv4ByRegexp(bs, ch, bm, re, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601ByRegexp(bs, ch, bm, re, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchIPv4ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchFloat64ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchValuesDictByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if re.MatchString(v) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return re.MatchString(v)
	})
}

func matchUint8ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint8String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint16ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint16String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint32ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint32String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchUint64ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toUint64String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}

func matchInt64ByRegexp(bs *blockSearch, ch *columnHeader, bm *bitmap, re *regexutil.Regex, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toInt64String(bs, bb, v)
		return re.MatchString(s)
	})
	bbPool.Put(bb)
}
