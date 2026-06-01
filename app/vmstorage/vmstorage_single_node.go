package vmstorage

import (
	"errors"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
)

// newVMStorageSingleNode creates a new instance of of VMStorage for vmsingle.
func newVMStorageSingleNode(s *storage.Storage, maxConcurrentRequests int, resetCacheIfNeeded func(mrs []storage.MetricRow)) *VMStorageSingleNode {
	vms := newVMStorage(s, maxConcurrentRequests)
	return &VMStorageSingleNode{
		VMStorage:          vms,
		wg:                 syncwg.WaitGroup{},
		resetCacheIfNeeded: resetCacheIfNeeded,
	}
}

type VMStorageSingleNode struct {
	*VMStorage

	// wg is used to wrap every storage call into wg.Add(1) ... wg.Done()
	// for proper graceful shutdown when Stop is called.
	//
	// Use syncwg instead of sync, since Add is called from concurrent
	// goroutines.
	wg syncwg.WaitGroup

	// resetCacheIfNeeded is a callback for automatic resetting of response
	// cache if needed.
	resetCacheIfNeeded func(mrs []storage.MetricRow)
}

func (api *VMStorageSingleNode) Stop() {
	api.wg.WaitAndBlock()
	api.VMStorage.Stop()
}

// WriteRows writes metric rows to the storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteRows() in
// order to limit memory usage.
func (api *VMStorageSingleNode) WriteRows(rows []storage.MetricRow) error {
	api.wg.Add(1)
	defer api.wg.Done()

	if api.s.IsReadOnly() {
		return errReadOnly
	}
	api.resetCacheIfNeeded(rows)
	return api.VMStorage.WriteRows(rows)
}

// WriteMetadata writes metrics metadata to storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteMetadata() in
// order to limit memory usage.
func (api *VMStorageSingleNode) WriteMetadata(rows []metricsmetadata.Row) error {
	api.wg.Add(1)
	defer api.wg.Done()

	if api.s.IsReadOnly() {
		return errReadOnly
	}
	return api.VMStorage.WriteMetadata(rows)
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

func (api *VMStorageSingleNode) IsReadOnly() bool {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.IsReadOnly()
}

// GetSearch sets up an instance of storage search and returns it to the caller
// along with the max series count that the search can return.
//
// This method is not part of the vmselectapi.API and must only be used by
// vmsingle HTTP handlers.
//
// Callers of this method must call PutSearch() once the search instance is not
// needed anymore.
func (api *VMStorageSingleNode) GetSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (*storage.Search, int, error) {
	api.wg.Add(1)

	tr := sq.GetTimeRange()
	maxMetrics := api.getMaxMetrics(sq.MaxMetrics)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, 0, err
	}

	sr := getSearch()
	maxSeriesCount := sr.Init(qt, api.s, tfss, tr, sq.MaxMetrics, deadline)
	return sr, maxSeriesCount, nil
}

// PutSearch resets the search once it is not needed anymore and puts it aside
// for future reuse.
//
// This method is not part of the vmselectapi.API and must only be used by
// vmsingle HTTP handlers.
//
// The method must only be used on search instances that have been created with
// GetSearch().
func (api *VMStorageSingleNode) PutSearch(sr *storage.Search) {
	api.wg.Done()
	putSearch(sr)
}

func getSearch() *storage.Search {
	v := ssPool.Get()
	if v == nil {
		return &storage.Search{}
	}
	return v.(*storage.Search)
}

func putSearch(sr *storage.Search) {
	sr.MustClose()
	ssPool.Put(sr)
}

var ssPool sync.Pool

func (api *VMStorageSingleNode) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.SearchMetricNames(qt, sq, deadline)
}

func (api *VMStorageSingleNode) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.LabelValues(qt, sq, labelName, maxLabelValues, deadline)
}

func (api *VMStorageSingleNode) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.TagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
}

func (api *VMStorageSingleNode) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.LabelNames(qt, sq, maxLabelNames, deadline)
}

func (api *VMStorageSingleNode) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.SeriesCount(qt, accountID, projectID, deadline)
}

func (api *VMStorageSingleNode) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.TSDBStatus(qt, sq, focusLabel, topN, deadline)
}

func (api *VMStorageSingleNode) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	// TODO(@rtm0): Return an error if the storage is in read-only mode?
	return api.VMStorage.DeleteSeries(qt, sq, deadline)
}

func (api *VMStorageSingleNode) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	api.wg.Add(1)
	defer api.wg.Done()
	// TODO(@rtm0): Return an error if the storage is in read-only mode?
	return api.VMStorage.RegisterMetricNames(qt, mrs, deadline)
}

func (api *VMStorageSingleNode) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string, deadline uint64) (metricnamestats.StatsResult, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.GetMetricNamesUsageStats(qt, tt, limit, le, matchPattern, deadline)
}

func (api *VMStorageSingleNode) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.ResetMetricNamesUsageStats(qt, deadline)
}

func (api *VMStorageSingleNode) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	return api.VMStorage.GetMetadataRecords(qt, tt, limit, metricName, deadline)
}
