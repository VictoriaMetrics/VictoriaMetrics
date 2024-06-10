package logstorage

// filter must implement filtering for log entries.
type filter interface {
	// String returns string representation of the filter
	String() string

	// udpdateNeededFields must update neededFields with fields needed for the filter
	updateNeededFields(neededFields fieldsSet)

	// applyToBlockSearch must update bm according to the filter applied to the given bs block
	applyToBlockSearch(bs *blockSearch, bm *bitmap)

	// applyToBlockResult must update bm according to the filter applied to the given br block
	applyToBlockResult(br *blockResult, bm *bitmap)
}

// visitFilter sequentially calls visitFunc for filters inside f.
//
// It stops calling visitFunc on the remaining filters as soon as visitFunc returns true.
// It returns the result of the last visitFunc call.
func visitFilter(f filter, visitFunc func(f filter) bool) bool {
	switch t := f.(type) {
	case *filterAnd:
		return visitFilters(t.filters, visitFunc)
	case *filterOr:
		return visitFilters(t.filters, visitFunc)
	case *filterNot:
		return visitFilter(t.f, visitFunc)
	default:
		return visitFunc(f)
	}
}

// visitFilters calls visitFunc per each filter in filters.
//
// It stops calling visitFunc on the remaining filters as soon as visitFunc returns true.
// It returns the result of the last visitFunc call.
func visitFilters(filters []filter, visitFunc func(f filter) bool) bool {
	for _, f := range filters {
		if visitFilter(f, visitFunc) {
			return true
		}
	}
	return false
}

// copyFilter recursively copies f filters with the help of copyFunc if visitFunc returns true for them.
//
// It doesn't copy other filters by returning them as is.
func copyFilter(f filter, visitFunc func(f filter) bool, copyFunc func(f filter) (filter, error)) (filter, error) {
	switch t := f.(type) {
	case *filterAnd:
		filters, err := copyFilters(t.filters, visitFunc, copyFunc)
		if err != nil {
			return nil, err
		}
		fa := &filterAnd{
			filters: filters,
		}
		return fa, nil
	case *filterOr:
		filters, err := copyFilters(t.filters, visitFunc, copyFunc)
		if err != nil {
			return nil, err
		}
		fo := &filterOr{
			filters: filters,
		}
		return fo, nil
	case *filterNot:
		f, err := copyFilter(t.f, visitFunc, copyFunc)
		if err != nil {
			return nil, err
		}
		fn := &filterNot{
			f: f,
		}
		return fn, nil
	default:
		if !visitFunc(t) {
			// Nothing to copy
			return t, nil
		}
		return copyFunc(t)
	}
}

// copyFilters recursively copies filters with the help of copyfunc if visitFunc returns true for them.
//
// It doesn't copy other filters by returning them as is.
func copyFilters(filters []filter, visitFunc func(f filter) bool, copyFunc func(f filter) (filter, error)) ([]filter, error) {
	if !visitFilters(filters, visitFunc) {
		// Nothing to copy
		return filters, nil
	}

	// Copy filters.
	filtersNew := make([]filter, len(filters))
	for i, f := range filters {
		fNew, err := copyFilter(f, visitFunc, copyFunc)
		if err != nil {
			return nil, err
		}
		filtersNew[i] = fNew
	}
	return filtersNew, nil
}
