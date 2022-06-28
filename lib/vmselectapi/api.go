package vmselectapi

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// API must implement vmselect API.
type API interface {
	// InitSearch initialize series search for the given tfss.
	//
	// The returned BlockIterator must be closed with MustClose to free up resources when it is no longer needed.
	InitSearch(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxMetrics int, deadline uint64) (BlockIterator, error)

	// SearchMetricNames returns metric names matching the given tfss.
	SearchMetricNames(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]storage.MetricName, error)

	// LabelValues returns values for labelName label acorss series matching the given tfss.
	LabelValues(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, tr storage.TimeRange, labelName string, maxLabelValues, maxMetrics int, deadline uint64) ([]string, error)

	// TagValueSuffixes returns tag value suffixes for the given args.
	TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix []byte, delimiter byte, maxSuffixes int, deadline uint64) ([]string, error)

	// LabelNames returns lable names for series matching the given tfss.
	LabelNames(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, tr storage.TimeRange, maxLableNames, maxMetrics int, deadline uint64) ([]string, error)

	// SeriesCount returns the number of series for the given (accountID, projectID).
	SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error)

	// TSDBStatus returns tsdb status for the given sq.
	TSDBStatus(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*storage.TSDBStatus, error)

	// DeleteMetrics deletes metrics matching the given tfss.
	DeleteMetrics(qt *querytracer.Tracer, tfss []*storage.TagFilters, maxMetrics int, deadline uint64) (int, error)

	// RegisterMetricNames registers the given mrs in the storage.
	RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow) error

	// SearchGraphitePaths searches for Graphite paths for the given query.
	SearchGraphitePaths(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, query []byte, maxMetrics int, deadline uint64) ([]string, error)
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
