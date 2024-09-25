package logstorage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// filterSequence matches an ordered sequence of phrases
//
// Example LogsQL: `fieldName:seq(foo, "bar baz")`
type filterSequence struct {
	fieldName string
	phrases   []string

	tokensOnce   sync.Once
	tokens       []string
	tokensHashes []uint64

	nonEmptyPhrasesOnce sync.Once
	nonEmptyPhrases     []string
}

func (fs *filterSequence) String() string {
	phrases := fs.phrases
	a := make([]string, len(phrases))
	for i, phrase := range phrases {
		a[i] = quoteTokenIfNeeded(phrase)
	}
	return fmt.Sprintf("%sseq(%s)", quoteFieldNameIfNeeded(fs.fieldName), strings.Join(a, ","))
}

func (fs *filterSequence) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fs.fieldName)
}

func (fs *filterSequence) getTokens() []string {
	fs.tokensOnce.Do(fs.initTokens)
	return fs.tokens
}

func (fs *filterSequence) getTokensHashes() []uint64 {
	fs.tokensOnce.Do(fs.initTokens)
	return fs.tokensHashes
}

func (fs *filterSequence) initTokens() {
	phrases := fs.getNonEmptyPhrases()
	fs.tokens = tokenizeStrings(nil, phrases)
	fs.tokensHashes = appendTokensHashes(nil, fs.tokens)
}

func (fs *filterSequence) getNonEmptyPhrases() []string {
	fs.nonEmptyPhrasesOnce.Do(fs.initNonEmptyPhrases)
	return fs.nonEmptyPhrases
}

func (fs *filterSequence) initNonEmptyPhrases() {
	phrases := fs.phrases
	result := make([]string, 0, len(phrases))
	for _, phrase := range phrases {
		if phrase != "" {
			result = append(result, phrase)
		}
	}
	fs.nonEmptyPhrases = result
}

func (fs *filterSequence) applyToBlockResult(br *blockResult, bm *bitmap) {
	phrases := fs.getNonEmptyPhrases()
	if len(phrases) == 0 {
		return
	}

	applyToBlockResultGeneric(br, bm, fs.fieldName, "", func(v, _ string) bool {
		return matchSequence(v, phrases)
	})
}

func (fs *filterSequence) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fs.fieldName
	phrases := fs.getNonEmptyPhrases()

	if len(phrases) == 0 {
		return
	}

	csh := bs.getColumnsHeader()
	v := csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchSequence(v, phrases) {
			bm.resetBits()
		}
		return
	}

	// Verify whether filter matches other columns
	ch := csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if !matchSequence("", phrases) {
			bm.resetBits()
		}
		return
	}

	tokens := fs.getTokensHashes()

	switch ch.valueType {
	case valueTypeString:
		matchStringBySequence(bs, ch, bm, phrases, tokens)
	case valueTypeDict:
		matchValuesDictBySequence(bs, ch, bm, phrases)
	case valueTypeUint8:
		matchUint8BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint16:
		matchUint16BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint32:
		matchUint32BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeUint64:
		matchUint64BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeFloat64:
		matchFloat64BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeIPv4:
		matchIPv4BySequence(bs, ch, bm, phrases, tokens)
	case valueTypeTimestampISO8601:
		matchTimestampISO8601BySequence(bs, ch, bm, phrases, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchTimestampISO8601BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 1 {
		matchTimestampISO8601ByPhrase(bs, ch, bm, phrases[0], tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	// Slow path - phrases contain incomplete timestamp. Search over string representation of the timestamp.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toTimestampISO8601String(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchIPv4BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) == 1 {
		matchIPv4ByPhrase(bs, ch, bm, phrases[0], tokens)
		return
	}
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}

	// Slow path - phrases contain parts of IP address. For example, `1.23` should match `1.23.4.5` and `4.1.23.54`.
	// We cannot compare binary represetnation of ip address and need converting
	// the ip to string before searching for prefix there.
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toIPv4String(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchFloat64BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	// The phrase may contain a part of the floating-point number.
	// For example, `foo:"123"` must match `123`, `123.456` and `-0.123`.
	// This means we cannot search in binary representation of floating-point numbers.
	// Instead, we need searching for the whole phrase in string representation
	// of floating-point numbers :(
	bb := bbPool.Get()
	visitValues(bs, ch, bm, func(v string) bool {
		s := toFloat64String(bs, bb, v)
		return matchSequence(s, phrases)
	})
	bbPool.Put(bb)
}

func matchValuesDictBySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchSequence(v, phrases) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringBySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if !matchBloomFilterAllTokens(bs, ch, tokens) {
		bm.resetBits()
		return
	}
	visitValues(bs, ch, bm, func(v string) bool {
		return matchSequence(v, phrases)
	})
}

func matchUint8BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint8ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint16BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint16ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint32BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint32ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchUint64BySequence(bs *blockSearch, ch *columnHeader, bm *bitmap, phrases []string, tokens []uint64) {
	if len(phrases) > 1 {
		bm.resetBits()
		return
	}
	matchUint64ByExactValue(bs, ch, bm, phrases[0], tokens)
}

func matchSequence(s string, phrases []string) bool {
	for _, phrase := range phrases {
		n := getPhrasePos(s, phrase)
		if n < 0 {
			return false
		}
		s = s[n+len(phrase):]
	}
	return true
}
