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

	byFieldTokensOnce sync.Once
	byFieldTokens     []fieldTokens
}

type fieldTokens struct {
	field  string
	tokens []string
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

	for _, fieldTokens := range byFieldTokens {
		fieldName := fieldTokens.field
		tokens := fieldTokens.tokens

		v := bs.csh.getConstColumnValue(fieldName)
		if v != "" {
			if !matchStringByAllTokens(v, tokens) {
				return false
			}
			continue
		}

		ch := bs.csh.getColumnHeader(fieldName)
		if ch == nil {
			return false
		}

		if ch.valueType == valueTypeDict {
			if !matchDictValuesByAllTokens(ch.valuesDict.values, tokens) {
				return false
			}
			continue
		}
		if !matchBloomFilterAllTokens(bs, ch, tokens) {
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
	m := make(map[string]map[string]struct{})
	var fieldNames []string

	mergeFieldTokens := func(fieldName string, tokens []string) {
		if len(tokens) == 0 {
			return
		}

		fieldName = getCanonicalColumnName(fieldName)
		mTokens, ok := m[fieldName]
		if !ok {
			fieldNames = append(fieldNames, fieldName)
			mTokens = make(map[string]struct{})
			m[fieldName] = mTokens
		}
		for _, token := range tokens {
			mTokens[token] = struct{}{}
		}
	}

	for _, f := range fa.filters {
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
		tokens := make([]string, 0, len(mTokens))
		for token := range mTokens {
			tokens = append(tokens, token)
		}

		byFieldTokens = append(byFieldTokens, fieldTokens{
			field:  fieldName,
			tokens: tokens,
		})
	}

	fa.byFieldTokens = byFieldTokens
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
