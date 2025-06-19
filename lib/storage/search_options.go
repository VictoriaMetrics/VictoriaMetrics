package storage

// searchOptions contains common options for Search.
type searchOptions struct {
	maxMetrics int
}

// getSearchOptions returns new searchOptions.
// Note that the readMetricIDs metric is only flushed when the searchOptions is put back with putSearchOptions.
func getSearchOptions(maxMetrics int) *searchOptions {
	return &searchOptions{
		maxMetrics: maxMetrics,
	}
}
