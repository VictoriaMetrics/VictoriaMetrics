package vmstorage

import (
	"errors"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. "+
		"Lower precision bits improves data compression at the cost of precision loss")
	maxUniqueTimeseries = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. "+
		"This allows protecting against heavy queries, which select unexpectedly high number of series. When set to zero, the limit is automatically calculated based on -search.maxConcurrentRequests (inversely proportional) and memory available to the process (proportional). See also -search.max* command-line flags at vmselect")
	maxTagKeys = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValues = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	snapshotsMaxAge              = flagutil.NewRetentionDuration("snapshotsMaxAge", "3d", "Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted")
)

// newVMStorage creates a new instance of of VMStorage.
//
// The created VMStorage instance takes ownership of s.
func newVMStorage(s *storage.Storage, maxConcurrentRequests int, resetCacheIfNeeded func(mrs []storage.MetricRow)) *VMStorage {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid -precisionBits: %d", err)
	}

	maxUniqueTimeseriesCalculated := *maxUniqueTimeseries
	if maxUniqueTimeseriesCalculated <= 0 {
		maxUniqueTimeseriesCalculated = calculateMaxUniqueTimeseries(maxConcurrentRequests, memory.Remaining())
	}

	vms := &VMStorage{
		s:                             s,
		maxUniqueTimeseries:           *maxUniqueTimeseries,
		maxUniqueTimeSeriesCalculated: maxUniqueTimeseriesCalculated,
		wg:                            syncwg.WaitGroup{},
		resetCacheIfNeeded:            resetCacheIfNeeded,
		staleSnapshotsRemoverCh:       make(chan struct{}),
	}
	vms.initStaleSnapshotsRemover()
	return vms
}

// calculateMaxUniqueTimeseries calculates the maxUniqueTimeseries based on the
// available system resources.
func calculateMaxUniqueTimeseries(maxConcurrentRequests, remainingMemory int) int {
	if maxConcurrentRequests <= 0 {
		// This line should NOT be reached unless the user has set an incorrect `search.maxConcurrentRequests`.
		// In such cases, fallback to unlimited.
		logger.Warnf("limiting -search.maxUniqueTimeseries to %v because -search.maxConcurrentRequests=%d.", 2e9, maxConcurrentRequests)
		return 2e9
	}

	// Calculate the max metrics limit for a single request in the worst-case concurrent scenario.
	// The approximate size of 1 unique series that could occupy in the vmstorage is 200 bytes.
	mts := remainingMemory / 200 / maxConcurrentRequests
	logger.Infof("limiting -search.maxUniqueTimeseries to %d according to -search.maxConcurrentRequests=%d and remaining memory=%d bytes. To increase the limit, reduce -search.maxConcurrentRequests or increase memory available to the process.", mts, maxConcurrentRequests, remainingMemory)
	return mts
}

// VMStorage impelements vmselectapi.API and vminsertapi.API.
type VMStorage struct {
	s                             *storage.Storage
	maxUniqueTimeseries           int
	maxUniqueTimeSeriesCalculated int

	// wg is used to wrap every storage call into wg.Add(1) ... wg.Done()
	// for proper graceful shutdown when Stop is called.
	//
	// Use syncwg instead of sync, since Add is called from concurrent
	// goroutines.
	wg syncwg.WaitGroup

	// resetCacheIfNeeded is a callback for automatic resetting of response
	// cache if needed.
	resetCacheIfNeeded func(mrs []storage.MetricRow)

	staleSnapshotsRemoverCh chan struct{}
	staleSnapshotsRemoverWG sync.WaitGroup
}

func (api *VMStorage) initStaleSnapshotsRemover() {
	if snapshotsMaxAge.Duration() <= 0 {
		return
	}
	snapshotsMaxAgeDuration := snapshotsMaxAge.Duration()
	api.staleSnapshotsRemoverWG.Go(func() {
		d := timeutil.AddJitterToDuration(time.Second * 11)
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-api.staleSnapshotsRemoverCh:
				return
			case <-t.C:
			}
			api.s.MustDeleteStaleSnapshots(snapshotsMaxAgeDuration)
		}
	})
}

func (api *VMStorage) Stop() {
	close(api.staleSnapshotsRemoverCh)
	api.staleSnapshotsRemoverWG.Wait()
	api.wg.WaitAndBlock()
	api.s.MustClose()
}

// WriteRows writes metric rows to the storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteRows() in
// order to limit memory usage.
func (api *VMStorage) WriteRows(rows []storage.MetricRow) error {
	api.wg.Add(1)
	defer api.wg.Done()

	if api.s.IsReadOnly() {
		return errReadOnly
	}
	api.resetCacheIfNeeded(rows)
	api.s.AddRows(rows, uint8(*precisionBits))
	return nil
}

// WriteMetadata writes metrics metadata to storage.
//
// Returns an error if the storage is in read-only mode.
//
// The caller should limit the number of concurrent calls to WriteMetadata() in
// order to limit memory usage.
func (api *VMStorage) WriteMetadata(rows []metricsmetadata.Row) error {
	api.wg.Add(1)
	defer api.wg.Done()

	if api.s.IsReadOnly() {
		return errReadOnly
	}
	api.s.AddMetadataRows(rows)
	return nil
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

// IsReadOnly returns true is the storage is in read-only mode.
func (api *VMStorage) IsReadOnly() bool {
	api.wg.Add(1)
	defer api.wg.Done()

	return api.s.IsReadOnly()
}

// GetSearch sets up an instance of storage search and returns it to the caller
// along with the max series count that the search can return.
//
// This method is not part of the vmselectapi.API and must only be used by
// vmsingle HTTP handlers.
//
// Callers of this method must call PutSearch() once the search instance is not
// needed anymore.
func (api *VMStorage) GetSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (*storage.Search, int, error) {
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
func (api *VMStorage) PutSearch(sr *storage.Search) {
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

func (api *VMStorage) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	tr := sq.GetTimeRange()
	maxMetrics := api.getMaxMetrics(sq.MaxMetrics)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(tfss) == 0 {
		return nil, fmt.Errorf("missing tag filters")
	}
	bi := getBlockIterator()
	bi.sr.Init(qt, api.s, tfss, tr, maxMetrics, deadline)
	if err := bi.sr.Error(); err != nil {
		bi.MustClose()
		return nil, err
	}
	return bi, nil
}

func (api *VMStorage) getMaxMetrics(searchQueryLimit int) int {
	if searchQueryLimit <= 0 {
		return api.maxUniqueTimeSeriesCalculated
	}
	// searchQueryLimit cannot exceed `-search.maxUniqueTimeseries`
	if api.maxUniqueTimeseries != 0 && searchQueryLimit > api.maxUniqueTimeseries {
		searchQueryLimit = api.maxUniqueTimeseries
	}
	return searchQueryLimit
}

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	sr storage.Search
	mb storage.MetricBlock
}

var blockIteratorsPool sync.Pool

func (bi *blockIterator) MustClose() {
	bi.sr.MustClose()
	bi.mb.MetricName = nil
	bi.mb.Block.Reset()
	blockIteratorsPool.Put(bi)
}

func getBlockIterator() *blockIterator {
	v := blockIteratorsPool.Get()
	if v == nil {
		v = &blockIterator{}
	}
	return v.(*blockIterator)
}

func (bi *blockIterator) NextBlock(dst []byte) ([]byte, bool) {
	if !bi.sr.NextMetricBlock() {
		return dst, false
	}
	mb := bi.mb
	mb.MetricName = bi.sr.MetricBlockRef.MetricName
	bi.sr.MetricBlockRef.BlockRef.MustReadBlock(&mb.Block)
	dst = mb.Marshal(dst[:0])
	return dst, true
}

func (bi *blockIterator) Error() error {
	return bi.sr.Error()
}

// SearchMetricNames returns metric names for the given tfss on the given tr.
func (api *VMStorage) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = api.maxUniqueTimeSeriesCalculated
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(tfss) == 0 {
		return nil, fmt.Errorf("missing tag filters")
	}
	return api.s.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
}

// SearchLabelValues searches for label values for the given labelName, tfss and
// tr.
func (api *VMStorage) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	tr := sq.GetTimeRange()
	if maxLabelValues <= 0 || maxLabelValues > *maxTagValues {
		maxLabelValues = *maxTagValues
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = api.maxUniqueTimeSeriesCalculated
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelValues(qt, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

// TagValueSuffixes returns all the tag value suffixes for the given tagKey and
// tagValuePrefix on the given tr.
//
// This allows implementing
// https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or
// similar APIs.
func (api *VMStorage) TagValueSuffixes(qt *querytracer.Tracer, _, _ uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	if maxSuffixes <= 0 || maxSuffixes > *maxTagValueSuffixesPerSearch {
		maxSuffixes = *maxTagValueSuffixesPerSearch
	}
	suffixes, err := api.s.SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
	if err != nil {
		return nil, err
	}
	if len(suffixes) >= maxSuffixes {
		return nil, fmt.Errorf("more than -search.maxTagValueSuffixesPerSearch=%d suffixes returned; "+
			"either narrow down the search or increase -search.maxTagValueSuffixesPerSearch command-line flag value", maxSuffixes)
	}
	return suffixes, nil
}

// SearchLabelNames searches for tag keys matching the given tfss on tr.
func (api *VMStorage) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	tr := sq.GetTimeRange()
	if maxLabelNames <= 0 || maxLabelNames > *maxTagKeys {
		maxLabelNames = *maxTagKeys
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = api.maxUniqueTimeSeriesCalculated
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelNames(qt, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

func (api *VMStorage) SeriesCount(_ *querytracer.Tracer, _, _ uint32, deadline uint64) (uint64, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	return api.s.GetSeriesCount(deadline)
}

func (api *VMStorage) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	return nil, nil
}

// GetTSDBStatus returns TSDB status for given filters on the given date.
func (api *VMStorage) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = api.maxUniqueTimeSeriesCalculated
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(sq.MinTimestamp) / (24 * 3600 * 1000)
	return api.s.GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

// DeleteSeries deletes series matching tfss.
//
// Returns the number of deleted series.
func (api *VMStorage) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	api.wg.Add(1)
	defer api.wg.Done()
	// TODO(@rtm0): Return an error if the storage is in read-only mode?

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = api.maxUniqueTimeSeriesCalculated
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return 0, err
	}
	if len(tfss) == 0 {
		return 0, fmt.Errorf("missing tag filters")
	}
	return api.s.DeleteSeries(qt, tfss, maxMetrics)
}

func (api *VMStorage) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, _ uint64) error {
	api.wg.Add(1)
	defer api.wg.Done()
	// TODO(@rtm0): Return an error if the storage is in read-only mode?

	api.s.RegisterMetricNames(qt, mrs)
	return nil
}

// GetMetricNamesUsageStats returns metric name usage stats.
func (api *VMStorage) GetMetricNamesUsageStats(qt *querytracer.Tracer, _ *storage.TenantToken, limit, le int, matchPattern string, _ uint64) (metricnamestats.StatsResult, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	return api.s.GetMetricNamesStats(qt, limit, le, matchPattern), nil
}

// ResetMetricNamesStats resets state for metric names usage tracker
func (api *VMStorage) ResetMetricNamesUsageStats(qt *querytracer.Tracer, _ uint64) error {
	api.wg.Add(1)
	defer api.wg.Done()

	api.s.ResetMetricNamesStats(qt)
	return nil
}

func (api *VMStorage) setupTfss(qt *querytracer.Tracer, sq *storage.SearchQuery, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(sq.TagFilterss))
	for _, tagFilters := range sq.TagFilterss {
		tfs := storage.NewTagFilters()
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				qtChild := qt.NewChild("searching for series matching __graphite__=%q", query)
				paths, err := api.s.SearchGraphitePaths(qtChild, tr, query, maxMetrics, deadline)
				qtChild.Donef("found %d series", len(paths))
				if err != nil {
					return nil, fmt.Errorf("error when searching for Graphite paths for query %q: %w", query, err)
				}
				if len(paths) >= maxMetrics {
					return nil, fmt.Errorf("more than %d time series match Graphite query %q; "+
						"either narrow down the query or increase the corresponding -search.max* command-line flag value at vmselect nodes; "+
						"see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#resource-usage-limits", maxMetrics, query)
				}
				tfs.AddGraphiteQuery(query, paths, tf.IsNegative)
				continue
			}
			if err := tfs.Add(tf.Key, tf.Value, tf.IsNegative, tf.IsRegexp); err != nil {
				return nil, fmt.Errorf("cannot parse tag filter %s: %w", tf, err)
			}
		}
		tfss = append(tfss, tfs)
	}
	return tfss, nil
}

func (api *VMStorage) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	api.wg.Add(1)
	defer api.wg.Done()

	return api.s.GetMetadataRows(qt, limit, metricName), nil
}

// deleteSnapshot deletes a snapshot by its name.
//
// Callers must wrap the call with wg.Add(1)...wg.Done().
func (api *VMStorage) deleteSnapshot(snapshotName string) error {
	snapshots := api.s.MustListSnapshots()
	for _, snName := range snapshots {
		if snName == snapshotName {
			if err := api.s.DeleteSnapshot(snName); err != nil {
				return fmt.Errorf("cannot delete snapshot %q: %w", snName, err)
			}
			return nil
		}
	}
	return fmt.Errorf("cannot find snapshot %q", snapshotName)
}
