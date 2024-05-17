package logstorage

import (
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// filterAnd contains filters joined by AND opertor.
//
// It is expressed as `f1 AND f2 ... AND fN` in LogsQL.
type filterAnd struct {
	filters []filter

	msgTokensOnce sync.Once
	msgTokens     []string
}

func (fa *filterAnd) String() string {
	filters := fa.filters
	a := make([]string, len(filters))
	for i, f := range filters {
		s := f.String()
		switch f.(type) {
		case *filterOr:
			s = "(" + s + ")"
		}
		a[i] = s
	}
	return strings.Join(a, " ")
}

func (fa *filterAnd) updateNeededFields(neededFields fieldsSet) {
	for _, f := range fa.filters {
		f.updateNeededFields(neededFields)
	}
}

func (fa *filterAnd) applyToBlockResult(br *blockResult, bm *bitmap) {
	for _, f := range fa.filters {
		f.applyToBlockResult(br, bm)
		if bm.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result will be zero anyway.
			return
		}
	}
}

func (fa *filterAnd) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if !fa.matchMessageBloomFilter(bs) {
		// Fast path - fa doesn't match _msg bloom filter.
		bm.resetBits()
		return
	}

	// Slow path - verify every filter separately.
	for _, f := range fa.filters {
		f.applyToBlockSearch(bs, bm)
		if bm.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result will be zero anyway.
			return
		}
	}
}

func (fa *filterAnd) matchMessageBloomFilter(bs *blockSearch) bool {
	tokens := fa.getMessageTokens()
	if len(tokens) == 0 {
		return true
	}

	v := bs.csh.getConstColumnValue("_msg")
	if v != "" {
		return matchStringByAllTokens(v, tokens)
	}

	ch := bs.csh.getColumnHeader("_msg")
	if ch == nil {
		return false
	}

	if ch.valueType == valueTypeDict {
		return matchDictValuesByAllTokens(ch.valuesDict.values, tokens)
	}
	return matchBloomFilterAllTokens(bs, ch, tokens)
}

func (fa *filterAnd) getMessageTokens() []string {
	fa.msgTokensOnce.Do(fa.initMsgTokens)
	return fa.msgTokens
}

func (fa *filterAnd) initMsgTokens() {
	var a []string
	for _, f := range fa.filters {
		switch t := f.(type) {
		case *filterPhrase:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterSequence:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterExact:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterExactPrefix:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterPrefix:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		}
	}
	fa.msgTokens = a
}

func matchStringByAllTokens(v string, tokens []string) bool {
	for _, token := range tokens {
		if !matchPhrase(v, token) {
			return false
		}
	}
	return true
}

func matchDictValuesByAllTokens(dictValues, tokens []string) bool {
	bb := bbPool.Get()
	for _, v := range dictValues {
		bb.B = append(bb.B, v...)
		bb.B = append(bb.B, ',')
	}
	v := bytesutil.ToUnsafeString(bb.B)
	ok := matchStringByAllTokens(v, tokens)
	bbPool.Put(bb)
	return ok
}
