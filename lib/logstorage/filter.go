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
