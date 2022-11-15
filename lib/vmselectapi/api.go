package vmselectapi

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// API must implement vmselect API.
type API interface {
	// InitSearch initialize series search for the given sq.
	//
	// The returned BlockIterator must be closed with MustClose to free up resources when it is no longer needed.
	InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (BlockIterator, error)

	// SearchMetricNames returns metric names matching the given sq.
	SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error)

	// LabelValues returns values for labelName label acorss series matching the given sq.
	LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error)

	// TagValueSuffixes returns tag value suffixes for the given args.
	TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline uint64) ([]string, error)

	// LabelNames returns lable names for series matching the given sq.
	LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLableNames int, deadline uint64) ([]string, error)

	// SeriesCount returns the number of series for the given (accountID, projectID).
	SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error)

	// TSDBStatus returns tsdb status for the given sq.
	TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error)

	// DeleteSeries deletes series matching the given sq.
	DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error)

	// RegisterMetricNames registers the given mrs in the storage.
	RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error

	// Tenants returns list of tenants in the storage.
	Tenants(qt *querytracer.Tracer, deadline uint64) ([]string, error)
}

// BlockIterator must iterate through series blocks found by VMSelect.InitSearch.
//
// MustClose must be called in order to free up allocated resources when BlockIterator is no longer needed.
type BlockIterator interface {
	// NextBlock reads the next block into mb.
	//
	// It returns true on success, false on error or if no blocks to read.
	NextBlock(mb *storage.MetricBlock) bool

	// MustClose frees up resources allocated by BlockIterator.
	MustClose()

	// Error returns the last error occurred in NextBlock(), which returns false.
	Error() error
}
