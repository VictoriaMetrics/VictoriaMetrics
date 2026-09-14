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
	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression "+
		"at the cost of precision loss")
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
func newVMStorage(s *storage.Storage, vmselectMaxConcurrentRequests int, resetCacheIfNeeded func(mrs []storage.MetricRow)) *VMStorage {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid -precisionBits=%d: %s", *precisionBits, err)
	}

	maxUniqueTimeseriesCalculated := *maxUniqueTimeseries
	if maxUniqueTimeseriesCalculated <= 0 {
		maxUniqueTimeseriesCalculated = calculateMaxUniqueTimeseries(vmselectMaxConcurrentRequests, memory.Remaining())
	}

	vms := &VMStorage{
		s:                             s,
		maxUniqueTimeseries:           *maxUniqueTimeseries,
		maxUniqueTimeSeriesCalculated: maxUniqueTimeseriesCalculated,
		staleSnapshotsRemoverCh:       make(chan struct{}),
		wg:                            syncwg.WaitGroup{},
		resetCacheIfNeeded:            resetCacheIfNeeded,
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
	staleSnapshotsRemoverCh       chan struct{}
	staleSnapshotsRemoverWG       sync.WaitGroup

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

func (vms *VMStorage) initStaleSnapshotsRemover() {
	if snapshotsMaxAge.Duration() <= 0 {
		return
	}
	snapshotsMaxAgeDuration := snapshotsMaxAge.Duration()
	vms.staleSnapshotsRemoverWG.Go(func() {
		d := timeutil.AddJitterToDuration(time.Second * 11)
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-vms.staleSnapshotsRemoverCh:
				return
			case <-t.C:
			}
			vms.s.MustDeleteStaleSnapshots(snapshotsMaxAgeDuration)
		}
	})
}

func (vms *VMStorage) Stop() {
	close(vms.staleSnapshotsRemoverCh)
	vms.staleSnapshotsRemoverWG.Wait()
	vms.wg.WaitAndBlock()
	vms.s.MustClose()
}

// WriteRows writes metric rows to the storage.
//
// The caller should limit the number of concurrent calls to WriteRows() in
// order to limit memory usage.
func (vms *VMStorage) WriteRows(rows []storage.MetricRow) error {
	vms.wg.Add(1)
	defer vms.wg.Done()

	if vms.s.IsReadOnly() {
		return errReadOnly
	}
	vms.resetCacheIfNeeded(rows)

	vms.s.AddRows(rows, uint8(*precisionBits))
	return nil
}

// WriteMetadata writes metrics metadata to storage.
//
// The caller should limit the number of concurrent calls to WriteMetadata() in
// order to limit memory usage.
func (vms *VMStorage) WriteMetadata(rows []metricsmetadata.Row) error {
	vms.wg.Add(1)
	defer vms.wg.Done()

	if vms.s.IsReadOnly() {
		return errReadOnly
	}
	vms.s.AddMetadataRows(rows)
	return nil
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

// IsReadOnly returns true is the storage is in read-only mode.
func (vms *VMStorage) IsReadOnly() bool {
	vms.wg.Add(1)
	defer vms.wg.Done()
	return vms.s.IsReadOnly()
}

func (vms *VMStorage) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	return vms.initSearch(qt, sq, marshalDefault, deadline)
}

func (vms *VMStorage) initSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, marshal marshalFunc, deadline uint64) (vmselectapi.BlockIterator, error) {
	vms.wg.Add(1)

	tr := sq.GetTimeRange()
	maxMetrics := vms.getMaxMetrics(sq.MaxMetrics)
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		vms.wg.Done()
		return nil, err
	}
	if len(tfss) == 0 {
		vms.wg.Done()
		return nil, fmt.Errorf("missing tag filters")
	}
	bi := getBlockIterator()
	bi.marshal = marshal
	bi.wgDone = vms.wg.Done
	bi.sr.Init(qt, vms.s, tfss, tr, maxMetrics, deadline)
	if err := bi.sr.Error(); err != nil {
		bi.MustClose()
		return nil, err
	}
	return bi, nil
}

func (vms *VMStorage) getMaxMetrics(searchQueryLimit int) int {
	if searchQueryLimit <= 0 {
		return vms.maxUniqueTimeSeriesCalculated
	}
	// searchQueryLimit cannot exceed `-search.maxUniqueTimeseries`
	if vms.maxUniqueTimeseries != 0 && searchQueryLimit > vms.maxUniqueTimeseries {
		searchQueryLimit = vms.maxUniqueTimeseries
	}
	return searchQueryLimit
}

type marshalFunc func(dst []byte, src *storage.MetricBlock) []byte

// marshalDefault is the default implementation of the MetricBlock marshaling.
func marshalDefault(dst []byte, src *storage.MetricBlock) []byte {
	return src.Marshal(dst)
}

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	sr      storage.Search
	mb      storage.MetricBlock
	marshal marshalFunc
	wgDone  func()
}

var blockIteratorsPool sync.Pool

func (bi *blockIterator) MustClose() {
	bi.sr.MustClose()
	bi.mb.MetricName = nil
	bi.mb.Block.Reset()
	bi.wgDone()
	bi.wgDone = nil
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
	mb := &bi.mb
	mb.MetricName = bi.sr.MetricBlockRef.MetricName
	bi.sr.MetricBlockRef.BlockRef.MustReadBlock(&mb.Block)
	dst = bi.marshal(dst[:0], mb)
	return dst, true
}

func (bi *blockIterator) Error() error {
	return bi.sr.Error()
}

// GetSearch sets up an instance of storage search and returns it to the caller
// along with the max series count that the search can return.
//
// This method is not part of the vmselectapi.API and must only be used by
// vmsingle HTTP handlers.
//
// Callers of this method must call PutSearch() once the search instance is not
// needed anymore.
func (vms *VMStorage) GetSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (*storage.Search, int, error) {
	vms.wg.Add(1)

	tr := sq.GetTimeRange()
	maxMetrics := vms.getMaxMetrics(sq.MaxMetrics)
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		vms.wg.Done()
		return nil, 0, err
	}

	sr := getSearch()
	maxSeriesCount := sr.Init(qt, vms.s, tfss, tr, sq.MaxMetrics, deadline)
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
func (vms *VMStorage) PutSearch(sr *storage.Search) {
	putSearch(sr)
	vms.wg.Done()
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

// SearchMetricNames returns metric names for the given tfss on the given tr.
func (vms *VMStorage) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = vms.maxUniqueTimeSeriesCalculated
	}
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(tfss) == 0 {
		return nil, fmt.Errorf("missing tag filters")
	}
	return vms.s.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
}

// SearchLabelValues searches for label values for the given labelName, tfss and
// tr.
func (vms *VMStorage) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	tr := sq.GetTimeRange()
	if maxLabelValues <= 0 || maxLabelValues > *maxTagValues {
		maxLabelValues = *maxTagValues
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = vms.maxUniqueTimeSeriesCalculated
	}
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return vms.s.SearchLabelValues(qt, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

// TagValueSuffixes returns all the tag value suffixes for the given tagKey and
// tagValuePrefix on the given tr.
//
// This allows implementing
// https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or
// similar APIs.
func (vms *VMStorage) TagValueSuffixes(qt *querytracer.Tracer, _, _ uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	if maxSuffixes <= 0 || maxSuffixes > *maxTagValueSuffixesPerSearch {
		maxSuffixes = *maxTagValueSuffixesPerSearch
	}
	suffixes, err := vms.s.SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
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
func (vms *VMStorage) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	tr := sq.GetTimeRange()
	if maxLabelNames <= 0 || maxLabelNames > *maxTagKeys {
		maxLabelNames = *maxTagKeys
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = vms.maxUniqueTimeSeriesCalculated
	}
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return vms.s.SearchLabelNames(qt, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

func (vms *VMStorage) SeriesCount(_ *querytracer.Tracer, _, _ uint32, deadline uint64) (uint64, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()
	return vms.s.GetSeriesCount(deadline)
}

func (vms *VMStorage) Tenants(_ *querytracer.Tracer, _ storage.TimeRange, _ uint64) ([]string, error) {
	return nil, nil
}

// GetTSDBStatus returns TSDB status for given filters on the given date.
func (vms *VMStorage) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = vms.maxUniqueTimeSeriesCalculated
	}
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(sq.MinTimestamp) / (24 * 3600 * 1000)
	return vms.s.GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

// DeleteSeries deletes series matching tfss.
//
// Returns the number of deleted series.
func (vms *VMStorage) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = vms.maxUniqueTimeSeriesCalculated
	}
	tfss, err := vms.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return 0, err
	}
	if len(tfss) == 0 {
		return 0, fmt.Errorf("missing tag filters")
	}
	return vms.s.DeleteSeries(qt, tfss, maxMetrics)
}

func (vms *VMStorage) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, _ uint64) error {
	vms.wg.Add(1)
	defer vms.wg.Done()

	vms.s.RegisterMetricNames(qt, mrs)
	return nil
}

// GetMetricNamesUsageStats returns metric name usage stats.
func (vms *VMStorage) GetMetricNamesUsageStats(qt *querytracer.Tracer, _ *storage.TenantToken, limit, le int, matchPattern string, _ uint64) (metricnamestats.StatsResult, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()

	return vms.s.GetMetricNamesStats(qt, limit, le, matchPattern), nil
}

// ResetMetricNamesStats resets state for metric names usage tracker
func (vms *VMStorage) ResetMetricNamesUsageStats(qt *querytracer.Tracer, _ uint64) error {
	vms.wg.Add(1)
	defer vms.wg.Done()

	vms.s.ResetMetricNamesStats(qt)
	return nil
}

func (vms *VMStorage) setupTfss(qt *querytracer.Tracer, sq *storage.SearchQuery, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(sq.TagFilterss))
	for _, tagFilters := range sq.TagFilterss {
		tfs := storage.NewTagFilters()
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				qtChild := qt.NewChild("searching for series matching __graphite__=%q", query)
				paths, err := vms.s.SearchGraphitePaths(qtChild, tr, query, maxMetrics, deadline)
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

func (vms *VMStorage) GetMetadataRecords(qt *querytracer.Tracer, _ *storage.TenantToken, limit int, metricName string, _ uint64) ([]*metricsmetadata.Row, error) {
	vms.wg.Add(1)
	defer vms.wg.Done()
	return vms.s.GetMetadataRows(qt, limit, metricName), nil
}

// deleteSnapshot deletes a snapshot by its name.
//
// Callers must wrap the call with wg.Add(1)...wg.Done().
func (vms *VMStorage) deleteSnapshot(snapshotName string) error {
	snapshots := vms.s.MustListSnapshots()
	for _, snName := range snapshots {
		if snName == snapshotName {
			if err := vms.s.DeleteSnapshot(snName); err != nil {
				return fmt.Errorf("cannot delete snapshot %q: %w", snName, err)
			}
			return nil
		}
	}
	return fmt.Errorf("cannot find snapshot %q", snapshotName)
}
