package logstorage

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

// filterAnyCasePhrase filters field entries by case-insensitive phrase match.
//
// An example LogsQL query: `fieldName:i(word)` or `fieldName:i("word1 ... wordN")`
type filterAnyCasePhrase struct {
	fieldName string
	phrase    string

	phraseLowercaseOnce sync.Once
	phraseLowercase     string

	phraseUppercaseOnce sync.Once
	phraseUppercase     string

	tokensOnce sync.Once
	tokens     []string

	tokensUppercaseOnce sync.Once
	tokensUppercase     []string
}

func (fp *filterAnyCasePhrase) String() string {
	return fmt.Sprintf("%si(%s)", quoteFieldNameIfNeeded(fp.fieldName), quoteTokenIfNeeded(fp.phrase))
}

func (fp *filterAnyCasePhrase) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fp.fieldName)
}

func (fp *filterAnyCasePhrase) getTokens() []string {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokens
}

func (fp *filterAnyCasePhrase) initTokens() {
	fp.tokens = tokenizeStrings(nil, []string{fp.phrase})
}

func (fp *filterAnyCasePhrase) getTokensUppercase() []string {
	fp.tokensUppercaseOnce.Do(fp.initTokensUppercase)
	return fp.tokensUppercase
}

func (fp *filterAnyCasePhrase) initTokensUppercase() {
	tokens := fp.getTokens()
	tokensUppercase := make([]string, len(tokens))
	for i, token := range tokens {
		tokensUppercase[i] = strings.ToUpper(token)
	}
	fp.tokensUppercase = tokensUppercase
}

func (fp *filterAnyCasePhrase) getPhraseLowercase() string {
	fp.phraseLowercaseOnce.Do(fp.initPhraseLowercase)
	return fp.phraseLowercase
}

func (fp *filterAnyCasePhrase) initPhraseLowercase() {
	fp.phraseLowercase = strings.ToLower(fp.phrase)
}

func (fp *filterAnyCasePhrase) getPhraseUppercase() string {
	fp.phraseUppercaseOnce.Do(fp.initPhraseUppercase)
	return fp.phraseUppercase
}

func (fp *filterAnyCasePhrase) initPhraseUppercase() {
	fp.phraseUppercase = strings.ToUpper(fp.phrase)
}

func (fp *filterAnyCasePhrase) applyToBlockResult(br *blockResult, bm *bitmap) {
	phraseLowercase := fp.getPhraseLowercase()
	applyToBlockResultGeneric(br, bm, fp.fieldName, phraseLowercase, matchAnyCasePhrase)
}

func (fp *filterAnyCasePhrase) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fp.fieldName
	phraseLowercase := fp.getPhraseLowercase()

	// Verify whether fp matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchAnyCasePhrase(v, phraseLowercase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether fp matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		// It matches anything only for empty phrase.
		if len(phraseLowercase) > 0 {
			bm.resetBits()
		}
		return
	}

	tokens := fp.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByAnyCasePhrase(bs, ch, bm, phraseLowercase)
	case valueTypeDict:
		matchValuesDictByAnyCasePhrase(bs, ch, bm, phraseLowercase)
	case valueTypeUint8:
		matchUint8ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint16:
		matchUint16ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint32:
		matchUint32ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeUint64:
		matchUint64ByExactValue(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeFloat64:
		matchFloat64ByPhrase(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPhrase(bs, ch, bm, phraseLowercase, tokens)
	case valueTypeTimestampISO8601:
		phraseUppercase := fp.getPhraseUppercase()
		tokensUppercase := fp.getTokensUppercase()
		matchTimestampISO8601ByPhrase(bs, ch, bm, phraseUppercase, tokensUppercase)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchValuesDictByAnyCasePhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phraseLowercase string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchAnyCasePhrase(v, phraseLowercase) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByAnyCasePhrase(bs *blockSearch, ch *columnHeader, bm *bitmap, phraseLowercase string) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchAnyCasePhrase(v, phraseLowercase)
	})
}

func matchAnyCasePhrase(s, phraseLowercase string) bool {
	if len(phraseLowercase) == 0 {
		// Special case - empty phrase matches only empty string.
		return len(s) == 0
	}
	if len(phraseLowercase) > len(s) {
		return false
	}

	if isASCIILowercase(s) {
		// Fast path - s is in lowercase
		return matchPhrase(s, phraseLowercase)
	}

	// Slow path - convert s to lowercase before matching
	bb := bbPool.Get()
	bb.B = stringsutil.AppendLowercase(bb.B, s)
	sLowercase := bytesutil.ToUnsafeString(bb.B)
	ok := matchPhrase(sLowercase, phraseLowercase)
	bbPool.Put(bb)

	return ok
}

func isASCIILowercase(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf || (c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}
