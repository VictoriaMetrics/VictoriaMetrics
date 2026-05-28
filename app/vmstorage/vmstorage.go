package main

import (
	"flag"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
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
)

var (
	maxUniqueTimeseriesValue     int
	maxUniqueTimeseriesValueOnce sync.Once
)

func newVMStorage(s *storage.Storage) *VMStorage {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid -precisionBits: %d", err)
	}

	GetMaxUniqueTimeSeries() // for init and logging only.

	return &VMStorage{s: s}
}

// VMStorage impelements vmselectapi.API and vminsertapi.API.
type VMStorage struct {
	s *storage.Storage
}

// WriteRows implements lib/vminsertapi.API interface
func (api *VMStorage) WriteRows(rows []storage.MetricRow) error {
	api.s.AddRows(rows, uint8(*precisionBits))
	return nil
}

// WriteMetadata implements lib/vminsertapi.API interface
func (api *VMStorage) WriteMetadata(rows []metricsmetadata.Row) error {
	api.s.AddMetadataRows(rows)
	return nil
}

// IsReadOnly implements lib/vminsertapi.API interface
func (api *VMStorage) IsReadOnly() bool {
	return api.s.IsReadOnly()
}

func (api *VMStorage) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq.MaxMetrics)
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

func (api *VMStorage) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = GetMaxUniqueTimeSeries()
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

func (api *VMStorage) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	if maxLabelValues <= 0 || maxLabelValues > *maxTagValues {
		maxLabelValues = *maxTagValues
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = GetMaxUniqueTimeSeries()
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelValues(qt, sq.AccountID, sq.ProjectID, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

func (api *VMStorage) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	if maxSuffixes <= 0 || maxSuffixes > *maxTagValueSuffixesPerSearch {
		maxSuffixes = *maxTagValueSuffixesPerSearch
	}
	suffixes, err := api.s.SearchTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
	if err != nil {
		return nil, err
	}
	if len(suffixes) >= maxSuffixes {
		return nil, fmt.Errorf("more than -search.maxTagValueSuffixesPerSearch=%d suffixes returned; "+
			"either narrow down the search or increase -search.maxTagValueSuffixesPerSearch command-line flag value", maxSuffixes)
	}
	return suffixes, nil
}

func (api *VMStorage) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	if maxLabelNames <= 0 || maxLabelNames > *maxTagKeys {
		maxLabelNames = *maxTagKeys
	}
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = GetMaxUniqueTimeSeries()
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelNames(qt, sq.AccountID, sq.ProjectID, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

func (api *VMStorage) SeriesCount(_ *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	return api.s.GetSeriesCount(accountID, projectID, deadline)
}

func (api *VMStorage) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	return api.s.SearchTenants(qt, tr, deadline)
}

func (api *VMStorage) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = GetMaxUniqueTimeSeries()
	}
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(sq.MinTimestamp) / (24 * 3600 * 1000)
	return api.s.GetTSDBStatus(qt, sq.AccountID, sq.ProjectID, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

func (api *VMStorage) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	tr := sq.GetTimeRange()
	maxMetrics := sq.MaxMetrics
	if maxMetrics <= 0 {
		// fallback to maxUniqueTimeSeries if no limit is provided,
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7857
		maxMetrics = GetMaxUniqueTimeSeries()
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
	api.s.RegisterMetricNames(qt, mrs)
	return nil
}

func (api *VMStorage) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string, _ uint64) (metricnamestats.StatsResult, error) {
	return api.s.GetMetricNamesStats(qt, tt, limit, le, matchPattern), nil
}

func (api *VMStorage) ResetMetricNamesUsageStats(qt *querytracer.Tracer, _ uint64) error {
	api.s.ResetMetricNamesStats(qt)
	return nil
}

func (api *VMStorage) setupTfss(qt *querytracer.Tracer, sq *storage.SearchQuery, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(sq.TagFilterss))
	accountID := sq.AccountID
	projectID := sq.ProjectID
	for _, tagFilters := range sq.TagFilterss {
		tfs := storage.NewTagFilters(accountID, projectID)
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				qtChild := qt.NewChild("searching for series matching __graphite__=%q", query)
				paths, err := api.s.SearchGraphitePaths(qtChild, accountID, projectID, tr, query, maxMetrics, deadline)
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
	return api.s.GetMetadataRows(qt, tt, limit, metricName, deadline)
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

func getMaxMetrics(searchQueryLimit int) int {
	if searchQueryLimit <= 0 {
		return GetMaxUniqueTimeSeries()
	}
	// searchQueryLimit cannot exceed `-search.maxUniqueTimeseries`
	if *maxUniqueTimeseries != 0 && searchQueryLimit > *maxUniqueTimeseries {
		searchQueryLimit = *maxUniqueTimeseries
	}
	return searchQueryLimit
}

// GetMaxUniqueTimeSeries returns `-search.maxUniqueTimeseries` or the auto-calculated value based on available resources.
// The calculation is split into calculateMaxUniqueTimeSeriesForResource for unit testing.
func GetMaxUniqueTimeSeries() int {
	maxUniqueTimeseriesValueOnce.Do(func() {
		maxUniqueTimeseriesValue = *maxUniqueTimeseries
		if maxUniqueTimeseriesValue <= 0 {
			maxUniqueTimeseriesValue = calculateMaxUniqueTimeSeriesForResource(*maxConcurrentRequests, memory.Remaining())
		}
	})
	return maxUniqueTimeseriesValue
}

// calculateMaxUniqueTimeSeriesForResource calculate the max metrics limit calculated by available resources.
func calculateMaxUniqueTimeSeriesForResource(maxConcurrentRequests, remainingMemory int) int {
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
