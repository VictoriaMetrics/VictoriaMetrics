package logstorage

// filterNoop does nothing
type filterNoop struct {
}

func (fn *filterNoop) String() string {
	return ""
}

func (fn *filterNoop) apply(_ *blockSearch, _ *bitmap) {
	// nothing to do
}
