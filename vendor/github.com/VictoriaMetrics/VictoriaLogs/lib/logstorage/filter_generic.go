package logstorage

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// filterGeneric applies the given filter f to the given fieldName
type filterGeneric struct {
	// fieldName is the name of the field to apply f to.
	//
	// It may end with '*' if isWildcard is true.
	fieldName string

	// isWildcard indicates whether fieldName is a wildcard ending with '*'
	//
	// In this case f is applied to all the fields with the given fieldName prefix until the first match.
	isWildcard bool

	// f is the filter to apply.
	f fieldFilter
}

func newFilterGeneric(fieldName string, f fieldFilter) *filterGeneric {
	if prefixfilter.IsWildcardFilter(fieldName) {
		return &filterGeneric{
			fieldName:  fieldName,
			isWildcard: true,
			f:          f,
		}
	}

	fieldNameCanonical := getCanonicalColumnName(fieldName)
	return &filterGeneric{
		fieldName: fieldNameCanonical,
		f:         f,
	}
}

func (fg *filterGeneric) getTokens() []string {
	switch t := fg.f.(type) {
	case *filterExact:
		return t.getTokens()
	case *filterExactPrefix:
		return t.getTokens()
	case *filterPhrase:
		return t.getTokens()
	case *filterPrefix:
		return t.getTokens()
	case *filterPatternMatch:
		return t.getTokens()
	case *filterRegexp:
		return t.getTokens()
	case *filterSequence:
		return t.getTokens()
	case *filterSubstring:
		return t.getTokens()
	default:
		return nil
	}
}

func (fg *filterGeneric) visitSubqueries(visitFunc func(q *Query)) {
	switch t := fg.f.(type) {
	case *filterContainsAll:
		t.values.q.visitSubqueries(visitFunc)
	case *filterContainsAny:
		t.values.q.visitSubqueries(visitFunc)
	case *filterIn:
		t.values.q.visitSubqueries(visitFunc)
	default:
		// nothing to do
	}
}

func (fg *filterGeneric) hasFilterInWithQuery() bool {
	switch t := fg.f.(type) {
	case *filterContainsAll:
		return t.values.q != nil
	case *filterContainsAny:
		return t.values.q != nil
	case *filterIn:
		return t.values.q != nil
	default:
		return false
	}
}

func (fg *filterGeneric) initFilterInValues(cache *inValuesCache, getFieldValues getFieldValuesFunc) (filter, error) {
	switch t := fg.f.(type) {
	case *filterContainsAll:
		values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValues)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
		}
		return newFilterContainsAllValues(fg.fieldName, values), nil
	case *filterContainsAny:
		values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValues)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
		}
		return newFilterContainsAnyValues(fg.fieldName, values), nil
	case *filterIn:
		values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValues)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
		}
		return newFilterInValues(fg.fieldName, values), nil
	default:
		return fg, nil
	}
}

// String returns string representation of the fg.
func (fg *filterGeneric) String() string {
	if !fg.isWildcard {
		return quoteFieldNameIfNeeded(fg.fieldName) + fg.f.String()
	}

	return quoteFieldFilterIfNeeded(fg.fieldName) + ":" + fg.f.String()
}

func (fg *filterGeneric) updateNeededFields(pf *prefixfilter.Filter) {
	pf.AddAllowFilter(fg.fieldName)
}

func (fg *filterGeneric) matchRow(fields []Field) bool {
	if !fg.isWildcard {
		// Fast path - match the row by the given fieldName.
		return fg.f.matchRowByField(fields, fg.fieldName)
	}

	// Slow path - match the row by wildcard
	prefix := fg.fieldName[:len(fg.fieldName)-1]
	for _, f := range fields {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		if fg.f.matchRowByField(fields, f.Name) {
			return true
		}
	}
	return false
}

func (fg *filterGeneric) applyToBlockSearch(bs *blockSearch, bm *bitmap) {
	if !fg.isWildcard {
		// Fast path - apply filter only to the given fieldName.
		fg.f.applyToBlockSearchByField(bs, bm, fg.fieldName)
		return
	}

	// Slow path - apply filter to all the matching fields.

	prefix := fg.fieldName[:len(fg.fieldName)-1]

	bmResult := getBitmap(bm.bitsLen)
	bmTmp := getBitmap(bm.bitsLen)
	defer putBitmap(bmTmp)
	defer putBitmap(bmResult)

	bmResult.copyFrom(bm)

	for _, fieldName := range specialColumns {
		if !strings.HasPrefix(fieldName, prefix) {
			continue
		}
		if bs.isHiddenField(fieldName) {
			continue
		}

		bmTmp.copyFrom(bmResult)
		fg.f.applyToBlockSearchByField(bs, bmTmp, fieldName)
		bmResult.andNot(bmTmp)
		if bmResult.isZero() {
			return
		}
	}

	csh := bs.getColumnsHeader()

	for _, cc := range csh.constColumns {
		if isSpecialColumn(cc.Name) {
			continue
		}
		if !strings.HasPrefix(cc.Name, prefix) {
			continue
		}
		if bs.isHiddenField(cc.Name) {
			continue
		}

		bmTmp.copyFrom(bmResult)
		fg.f.applyToBlockSearchByField(bs, bmTmp, cc.Name)
		bmResult.andNot(bmTmp)
		if bmResult.isZero() {
			return
		}
	}

	chs := csh.columnHeaders
	for i := range chs {
		ch := &chs[i]
		if isSpecialColumn(ch.name) {
			continue
		}
		if !strings.HasPrefix(ch.name, prefix) {
			continue
		}
		if bs.isHiddenField(ch.name) {
			continue
		}

		bmTmp.copyFrom(bmResult)
		fg.f.applyToBlockSearchByField(bs, bmTmp, ch.name)
		bmResult.andNot(bmTmp)
		if bmResult.isZero() {
			return
		}
	}

	bm.andNot(bmResult)
}

func (fg *filterGeneric) applyToBlockResult(br *blockResult, bm *bitmap) {
	if !fg.isWildcard {
		// Fast path - apply filter to the given fieldName
		fg.f.applyToBlockResultByField(br, bm, fg.fieldName)
		return
	}

	// Slow path - apply filter to all the matching fields.
	prefix := fg.fieldName[:len(fg.fieldName)-1]

	bmResult := getBitmap(bm.bitsLen)
	bmTmp := getBitmap(bm.bitsLen)
	defer putBitmap(bmTmp)
	defer putBitmap(bmResult)

	bmResult.copyFrom(bm)

	cs := br.getColumns()
	for _, c := range cs {
		if !strings.HasPrefix(c.name, prefix) {
			continue
		}

		bmTmp.copyFrom(bmResult)
		fg.f.applyToBlockResultByField(br, bmTmp, c.name)
		bmResult.andNot(bmTmp)
		if bmResult.isZero() {
			return
		}
	}

	bm.andNot(bmResult)
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
