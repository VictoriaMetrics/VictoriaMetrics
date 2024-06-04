package logstorage

// filterNoop does nothing
type filterNoop struct {
}

func (fn *filterNoop) String() string {
	return "*"
}

func (fn *filterNoop) updateNeededFields(_ fieldsSet) {
	// nothing to do
}

func (fn *filterNoop) applyToBlockResult(_ *blockResult, _ *bitmap) {
	// nothing to do
}

func (fn *filterNoop) applyToBlockSearch(_ *blockSearch, _ *bitmap) {
	// nothing to do
}
