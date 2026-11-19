package vmselectapi

import (
	"context"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

// API must implement vmselect API.
type API interface {
	// InitSearch initialize series search for the given sq.
	//
	// The returned BlockIterator must be closed with MustClose to free up resources when it is no longer needed.
	InitSearch(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) (BlockIterator, error)

	// SearchMetricNames returns metric names matching the given sq.
	SearchMetricNames(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) ([]string, error)

	// LabelValues returns values for labelName label acorss series matching the given sq.
	LabelValues(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int) ([]string, error)

	// TagValueSuffixes returns tag value suffixes for the given args.
	TagValueSuffixes(ctx context.Context, qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int) ([]string, error)

	// LabelNames returns lable names for series matching the given sq.
	LabelNames(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, maxLableNames int) ([]string, error)

	// SeriesCount returns the number of series for the given (accountID, projectID).
	SeriesCount(ctx context.Context, qt *querytracer.Tracer, accountID, projectID uint32) (uint64, error)

	// TSDBStatus returns tsdb status for the given sq.
	TSDBStatus(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int) (*storage.TSDBStatus, error)

	// DeleteSeries deletes series matching the given sq.
	DeleteSeries(ctx context.Context, qt *querytracer.Tracer, sq *storage.SearchQuery) (int, error)

	// RegisterMetricNames registers the given mrs in the storage.
	RegisterMetricNames(ctx context.Context, qt *querytracer.Tracer, mrs []storage.MetricRow) error

	// Tenants returns list of tenants in the storage on the given tr.
	Tenants(ctx context.Context, qt *querytracer.Tracer, tr storage.TimeRange) ([]string, error)

	// GetMetricNamesUsageStats returns statistics for metric names
	GetMetricNamesUsageStats(ctx context.Context, qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string) (metricnamestats.StatsResult, error)

	// ResetMetricNamesUsageStats resets internal state of metric names tracker
	ResetMetricNamesUsageStats(ctx context.Context, qt *querytracer.Tracer) error

	// GetMetadataRecords returns metrics metadata.
	GetMetadataRecords(ctx context.Context, qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string) ([]*metricsmetadata.Row, error)
}

// BlockIterator must iterate through series blocks found by VMSelect.InitSearch.
//
// MustClose must be called in order to free up allocated resources when BlockIterator is no longer needed.
type BlockIterator interface {
	// NextBlock marshals next storage.MetricBlock into dst.
	//
	// It returns true on success, false on error or if no blocks to read.
	NextBlock(ctx context.Context, dst []byte) ([]byte, bool)

	// MustClose frees up resources allocated by BlockIterator.
	MustClose()

	// Error returns the last error occurred in NextBlock(), which returns false.
	Error() error
}
