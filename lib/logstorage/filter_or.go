package logstorage

import (
	"strings"
	"sync"
)

// filterOr contains filters joined by OR operator.
//
// It is epxressed as `f1 OR f2 ... OR fN` in LogsQL.
type filterOr struct {
	filters []filter

	byFieldTokensOnce sync.Once
	byFieldTokens     []fieldTokens
}

func (fo *filterOr) String() string {
	filters := fo.filters
	a := make([]string, len(filters))
	for i, f := range filters {
		s := f.String()
		a[i] = s
	}
	return strings.Join(a, " or ")
}

func (fo *filterOr) updateNeededFields(neededFields fieldsSet) {
	for _, f := range fo.filters {
		f.updateNeededFields(neededFields)
	}
}

func (fo *filterOr) applyToBlockResult(br *blockResult, bm *bitmap) {
	bmResult := getBitmap(bm.bitsLen)
	bmTmp := getBitmap(bm.bitsLen)
	for _, f := range fo.filters {
		// Minimize the number of rows to check by the filter by checking only
		// the rows, which may change the output bm:
		// - bm matches them, e.g. the caller wants to get them
		// - bmResult doesn't match them, e.g. all the previous OR filters didn't match them
		bmTmp.copyFrom(bm)
		bmTmp.andNot(bmResult)
		if bmTmp.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result already matches all the values from the block.
			break
		}
		f.applyToBlockResult(br, bmTmp)
		bmResult.or(bmTmp)
	}
	putBitmap(bmTmp)
	bm.copyFrom(bmResult)
	putBitmap(bmResult)
}

func (fo *filterOr) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if !fo.matchBloomFilters(bs) {
		// Fast path - fo doesn't match bloom filters.
		bm.resetBits()
		return
	}

	bmResult := getBitmap(bm.bitsLen)
	bmTmp := getBitmap(bm.bitsLen)
	for _, f := range fo.filters {
		// Minimize the number of rows to check by the filter by checking only
		// the rows, which may change the output bm:
		// - bm matches them, e.g. the caller wants to get them
		// - bmResult doesn't match them, e.g. all the previous OR filters didn't match them
		bmTmp.copyFrom(bm)
		bmTmp.andNot(bmResult)
		if bmTmp.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result already matches all the values from the block.
			break
		}
		f.applyToBlockSearch(bs, bmTmp)
		bmResult.or(bmTmp)
	}
	putBitmap(bmTmp)
	bm.copyFrom(bmResult)
	putBitmap(bmResult)
}

func (fo *filterOr) matchBloomFilters(bs *blockSearch) bool {
	byFieldTokens := fo.getByFieldTokens()
	if len(byFieldTokens) == 0 {
		return true
	}

	for _, fieldTokens := range byFieldTokens {
		fieldName := fieldTokens.field
		tokens := fieldTokens.tokens

		v := bs.csh.getConstColumnValue(fieldName)
		if v != "" {
			if matchStringByAllTokens(v, tokens) {
				return true
			}
			continue
		}

		ch := bs.csh.getColumnHeader(fieldName)
		if ch == nil {
			continue
		}

		if ch.valueType == valueTypeDict {
			if matchDictValuesByAllTokens(ch.valuesDict.values, tokens) {
				return true
			}
			continue
		}
		if matchBloomFilterAllTokens(bs, ch, tokens) {
			return true
		}
	}

	return false
}

func (fo *filterOr) getByFieldTokens() []fieldTokens {
	fo.byFieldTokensOnce.Do(fo.initByFieldTokens)
	return fo.byFieldTokens
}

func (fo *filterOr) initByFieldTokens() {
	m := make(map[string][][]string)
	var fieldNames []string

	mergeFieldTokens := func(fieldName string, tokens []string) {
		if len(tokens) == 0 {
			return
		}

		fieldName = getCanonicalColumnName(fieldName)
		if _, ok := m[fieldName]; !ok {
			fieldNames = append(fieldNames, fieldName)
		}
		m[fieldName] = append(m[fieldName], tokens)
	}

	for _, f := range fo.filters {
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
		case *filterAnd:
			bfts := t.getByFieldTokens()
			for _, bft := range bfts {
				mergeFieldTokens(bft.field, bft.tokens)
			}
		}
	}

	var byFieldTokens []fieldTokens
	for _, fieldName := range fieldNames {
		commonTokens := getCommonTokens(m[fieldName])
		if len(commonTokens) > 0 {
			byFieldTokens = append(byFieldTokens, fieldTokens{
				field:  fieldName,
				tokens: commonTokens,
			})
		}
	}

	fo.byFieldTokens = byFieldTokens
}
