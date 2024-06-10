package logstorage

import (
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

// filterAnyCasePrefix matches the given prefix in lower, upper and mixed case.
//
// Example LogsQL: `fieldName:i(prefix*)` or `fieldName:i("some prefix"*)`
//
// A special case `fieldName:i(*)` equals to `fieldName:*` and matches non-emtpy value for the given `fieldName` field.
type filterAnyCasePrefix struct {
	fieldName string
	prefix    string

	prefixLowercaseOnce sync.Once
	prefixLowercase     string

	tokensOnce sync.Once
	tokens     []string
}

func (fp *filterAnyCasePrefix) String() string {
	if fp.prefix == "" {
		return quoteFieldNameIfNeeded(fp.fieldName) + "i(*)"
	}
	return fmt.Sprintf("%si(%s*)", quoteFieldNameIfNeeded(fp.fieldName), quoteTokenIfNeeded(fp.prefix))
}

func (fp *filterAnyCasePrefix) updateNeededFields(neededFields fieldsSet) {
	neededFields.add(fp.fieldName)
}

func (fp *filterAnyCasePrefix) getTokens() []string {
	fp.tokensOnce.Do(fp.initTokens)
	return fp.tokens
}

func (fp *filterAnyCasePrefix) initTokens() {
	fp.tokens = getTokensSkipLast(fp.prefix)
}

func (fp *filterAnyCasePrefix) getPrefixLowercase() string {
	fp.prefixLowercaseOnce.Do(fp.initPrefixLowercase)
	return fp.prefixLowercase
}

func (fp *filterAnyCasePrefix) initPrefixLowercase() {
	fp.prefixLowercase = strings.ToLower(fp.prefix)
}

func (fp *filterAnyCasePrefix) applyToBlockResult(br *blockResult, bm *bitmap) {
	prefixLowercase := fp.getPrefixLowercase()
	applyToBlockResultGeneric(br, bm, fp.fieldName, prefixLowercase, matchAnyCasePrefix)
}

func (fp *filterAnyCasePrefix) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	fieldName := fp.fieldName
	prefixLowercase := fp.getPrefixLowercase()

	// Verify whether fp matches const column
	v := bs.csh.getConstColumnValue(fieldName)
	if v != "" {
		if !matchAnyCasePrefix(v, prefixLowercase) {
			bm.resetBits()
		}
		return
	}

	// Verify whether fp matches other columns
	ch := bs.csh.getColumnHeader(fieldName)
	if ch == nil {
		// Fast path - there are no matching columns.
		bm.resetBits()
		return
	}

	tokens := fp.getTokens()

	switch ch.valueType {
	case valueTypeString:
		matchStringByAnyCasePrefix(bs, ch, bm, prefixLowercase)
	case valueTypeDict:
		matchValuesDictByAnyCasePrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint8:
		matchUint8ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint16:
		matchUint16ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint32:
		matchUint32ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeUint64:
		matchUint64ByPrefix(bs, ch, bm, prefixLowercase)
	case valueTypeFloat64:
		matchFloat64ByPrefix(bs, ch, bm, prefixLowercase, tokens)
	case valueTypeIPv4:
		matchIPv4ByPrefix(bs, ch, bm, prefixLowercase, tokens)
	case valueTypeTimestampISO8601:
		prefixUppercase := strings.ToUpper(fp.prefix)
		matchTimestampISO8601ByPrefix(bs, ch, bm, prefixUppercase, tokens)
	default:
		logger.Panicf("FATAL: %s: unknown valueType=%d", bs.partPath(), ch.valueType)
	}
}

func matchValuesDictByAnyCasePrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefixLowercase string) {
	bb := bbPool.Get()
	for _, v := range ch.valuesDict.values {
		c := byte(0)
		if matchAnyCasePrefix(v, prefixLowercase) {
			c = 1
		}
		bb.B = append(bb.B, c)
	}
	matchEncodedValuesDict(bs, ch, bm, bb.B)
	bbPool.Put(bb)
}

func matchStringByAnyCasePrefix(bs *blockSearch, ch *columnHeader, bm *bitmap, prefixLowercase string) {
	visitValues(bs, ch, bm, func(v string) bool {
		return matchAnyCasePrefix(v, prefixLowercase)
	})
}

func matchAnyCasePrefix(s, prefixLowercase string) bool {
	if len(prefixLowercase) == 0 {
		// Special case - empty prefix matches any non-empty string.
		return len(s) > 0
	}
	if len(prefixLowercase) > len(s) {
		return false
	}

	if isASCIILowercase(s) {
		// Fast path - s is in lowercase
		return matchPrefix(s, prefixLowercase)
	}

	// Slow path - convert s to lowercase before matching
	bb := bbPool.Get()
	bb.B = stringsutil.AppendLowercase(bb.B, s)
	sLowercase := bytesutil.ToUnsafeString(bb.B)
	ok := matchPrefix(sLowercase, prefixLowercase)
	bbPool.Put(bb)

	return ok
}
