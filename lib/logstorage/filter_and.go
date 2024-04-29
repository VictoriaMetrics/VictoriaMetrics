package logstorage

import (
	"strings"
	"sync"
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

func (fa *filterAnd) apply(bs *blockSearch, bm *bitmap) {
	if tokens := fa.getMsgTokens(); len(tokens) > 0 {
		// Verify whether fa tokens for the _msg field match bloom filter.
		ch := bs.csh.getColumnHeader("_msg")
		if ch == nil {
			// Fast path - there is no _msg field in the block.
			bm.resetBits()
			return
		}
		if !matchBloomFilterAllTokens(bs, ch, tokens) {
			// Fast path - fa tokens for the _msg field do not match bloom filter.
			bm.resetBits()
			return
		}
	}

	// Slow path - verify every filter separately.
	for _, f := range fa.filters {
		f.apply(bs, bm)
		if bm.isZero() {
			// Shortcut - there is no need in applying the remaining filters,
			// since the result will be zero anyway.
			return
		}
	}
}

func (fa *filterAnd) getMsgTokens() []string {
	fa.msgTokensOnce.Do(fa.initMsgTokens)
	return fa.msgTokens
}

func (fa *filterAnd) initMsgTokens() {
	var a []string
	for _, f := range fa.filters {
		switch t := f.(type) {
		case *phraseFilter:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterSequence:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *exactFilter:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *filterExactPrefix:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		case *prefixFilter:
			if isMsgFieldName(t.fieldName) {
				a = append(a, t.getTokens()...)
			}
		}
	}
	fa.msgTokens = a
}
