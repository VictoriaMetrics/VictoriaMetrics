package logstorage

import (
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

// filterAnd contains filters joined by AND operator.
//
// It is expressed as `f1 AND f2 ... AND fN` in LogsQL.
type filterAnd struct {
	filters []filter

	byFieldTokensOnce sync.Once
	byFieldTokens     []fieldTokens
}

type fieldTokens struct {
	field        string
	tokens       []string
	tokensHashes []uint64
}

func (fa *filterAnd) String() string {
	filters := fa.filters
	a := make([]string, len(filters))
	for i, f := range filters {
		s := f.String()
		if _, ok := f.(*filterOr); ok {
			s = "(" + s + ")"
		}
		a[i] = s
	}
	return strings.Join(a, " ")
}

func (fa *filterAnd) updateNeededFields(pf *prefixfilter.Filter) {
	for _, f := range fa.filters {
		f.updateNeededFields(pf)
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
	if !fa.matchBloomFilters(bs) {
		// Fast path - fa doesn't match bloom filters.
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

func (fa *filterAnd) matchBloomFilters(bs *blockSearch) bool {
	byFieldTokens := fa.getByFieldTokens()
	if len(byFieldTokens) == 0 {
		return true
	}

	for _, ft := range byFieldTokens {
		fieldName := ft.field
		tokens := ft.tokens

		v := bs.getConstColumnValue(fieldName)
		if v != "" {
			if matchStringByAllTokens(v, tokens) {
				continue
			}
			return false
		}

		ch := bs.getColumnHeader(fieldName)
		if ch == nil {
			return false
		}

		if ch.valueType == valueTypeDict {
			if matchDictValuesByAllTokens(ch.valuesDict.values, tokens) {
				continue
			}
			return false
		}
		if !matchBloomFilterAllTokens(bs, ch, ft.tokensHashes) {
			return false
		}
	}

	return true
}

func (fa *filterAnd) getByFieldTokens() []fieldTokens {
	fa.byFieldTokensOnce.Do(fa.initByFieldTokens)
	return fa.byFieldTokens
}

func (fa *filterAnd) initByFieldTokens() {
	fa.byFieldTokens = getCommonTokensForAndFilters(fa.filters)
}

func getCommonTokensForAndFilters(filters []filter) []fieldTokens {
	m := make(map[string][]string)
	var fieldNames []string

	mergeFieldTokens := func(fieldName string, tokens []string) {
		if len(tokens) == 0 {
			return
		}

		fieldName = getCanonicalColumnName(fieldName)
		if _, ok := m[fieldName]; !ok {
			fieldNames = append(fieldNames, fieldName)
		}
		m[fieldName] = append(m[fieldName], tokens...)
	}

	for _, f := range filters {
		switch t := f.(type) {
		case *filterExact:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterExactPrefix:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterPhrase:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterPrefix:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterRegexp:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterSequence:
			tokens := t.getTokens()
			mergeFieldTokens(t.fieldName, tokens)
		case *filterOr:
			bfts := t.getByFieldTokens()
			for _, bft := range bfts {
				mergeFieldTokens(bft.field, bft.tokens)
			}
		}
	}

	var byFieldTokens []fieldTokens
	for _, fieldName := range fieldNames {
		mTokens := m[fieldName]
		seenTokens := make(map[string]struct{})
		tokens := make([]string, 0, len(mTokens))
		for _, token := range mTokens {
			if _, ok := seenTokens[token]; ok {
				continue
			}
			seenTokens[token] = struct{}{}
			tokens = append(tokens, token)
		}

		byFieldTokens = append(byFieldTokens, fieldTokens{
			field:        fieldName,
			tokens:       tokens,
			tokensHashes: appendTokensHashes(nil, tokens),
		})
	}

	return byFieldTokens
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
