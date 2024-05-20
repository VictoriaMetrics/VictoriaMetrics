package logstorage

import (
	"strings"
)

// filterOr contains filters joined by OR operator.
//
// It is epxressed as `f1 OR f2 ... OR fN` in LogsQL.
type filterOr struct {
	filters []filter
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
