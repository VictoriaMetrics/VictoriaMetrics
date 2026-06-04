package vmstorage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

// newVMStorageSingleNode creates a new instance of of VMStorage for vmsingle.
func newVMStorageSingleNode(s *storage.Storage, maxConcurrentRequests int, resetCacheIfNeeded func(mrs []storage.MetricRow)) *VMStorageSingleNode {
	vms := newVMStorage(s, maxConcurrentRequests)
	return &VMStorageSingleNode{
		vms:                vms,
		wg:                 syncwg.WaitGroup{},
		resetCacheIfNeeded: resetCacheIfNeeded,
	}
}

type VMStorageSingleNode struct {
	vms *VMStorage

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

func (vmssn *VMStorageSingleNode) Stop() {
	vmssn.wg.WaitAndBlock()
	vmssn.vms.Stop()
}

// WriteRows writes metric rows to the storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteRows() in
// order to limit memory usage.
func (vmssn *VMStorageSingleNode) WriteRows(rows []storage.MetricRow) error {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()

	if vmssn.vms.IsReadOnly() {
		return errReadOnly
	}
	vmssn.resetCacheIfNeeded(rows)
	return vmssn.vms.WriteRows(rows)
}

// WriteMetadata writes metrics metadata to storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteMetadata() in
// order to limit memory usage.
func (vmssn *VMStorageSingleNode) WriteMetadata(rows []metricsmetadata.Row) error {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()

	if vmssn.vms.IsReadOnly() {
		return errReadOnly
	}
	return vmssn.vms.WriteMetadata(rows)
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

func (vmssn *VMStorageSingleNode) IsReadOnly() bool {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.IsReadOnly()
}

func (vmssn *VMStorageSingleNode) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	return nil, fmt.Errorf("not implemented in vmsingle")
}

// GetSearch sets up an instance of storage search and returns it to the caller
// along with the max series count that the search can return.
//
// This method is not part of the vmselectapi.API and must only be used by
// vmsingle HTTP handlers.
//
// Callers of this method must call PutSearch() once the search instance is not
// needed anymore.
func (vmssn *VMStorageSingleNode) GetSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (*storage.Search, int, error) {
	vmssn.wg.Add(1)

	tr := sq.GetTimeRange()
	maxMetrics := vmssn.vms.getMaxMetrics(sq.MaxMetrics)
	tfss, err := vmssn.vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		vmssn.wg.Done()
		return nil, 0, err
	}

	sr := getSearch()
	maxSeriesCount := sr.Init(qt, vmssn.vms.s, tfss, tr, sq.MaxMetrics, deadline)
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
func (vmssn *VMStorageSingleNode) PutSearch(sr *storage.Search) {
	putSearch(sr)
	vmssn.wg.Done()
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

func (vmssn *VMStorageSingleNode) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.SearchMetricNames(qt, sq, deadline)
}

func (vmssn *VMStorageSingleNode) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.LabelValues(qt, sq, labelName, maxLabelValues, deadline)
}

func (vmssn *VMStorageSingleNode) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline uint64) ([]string, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.TagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
}

func (vmssn *VMStorageSingleNode) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.LabelNames(qt, sq, maxLabelNames, deadline)
}

func (vmssn *VMStorageSingleNode) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.SeriesCount(qt, accountID, projectID, deadline)
}

func (vmssn *VMStorageSingleNode) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.Tenants(qt, tr, deadline)
}

func (vmssn *VMStorageSingleNode) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.TSDBStatus(qt, sq, focusLabel, topN, deadline)
}

func (vmssn *VMStorageSingleNode) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.DeleteSeries(qt, sq, deadline)
}

func (vmssn *VMStorageSingleNode) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.RegisterMetricNames(qt, mrs, deadline)
}

func (vmssn *VMStorageSingleNode) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string, deadline uint64) (metricnamestats.StatsResult, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.GetMetricNamesUsageStats(qt, tt, limit, le, matchPattern, deadline)
}

func (vmssn *VMStorageSingleNode) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.ResetMetricNamesUsageStats(qt, deadline)
}

func (vmssn *VMStorageSingleNode) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	vmssn.wg.Add(1)
	defer vmssn.wg.Done()
	return vmssn.vms.GetMetadataRecords(qt, tt, limit, metricName, deadline)
}
