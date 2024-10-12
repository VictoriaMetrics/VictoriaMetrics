package servers

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxUniqueTimeseries = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. "+
		"This allows protecting against heavy queries, which select unexpectedly high number of series. When set to zero, the limit is automatically calculated based on -search.maxConcurrentRequests (inversely proportional) and memory available to the process (proportional). See also -search.max* command-line flags at vmselect")
	maxTagKeys = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValues = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxConcurrentRequests        = flag.Int("search.maxConcurrentRequests", 2*cgroup.AvailableCPUs(), "The maximum number of concurrent vmselect requests "+
		"the vmstorage can process at -vmselectAddr. It shouldn't be high, since a single request usually saturates a CPU core, and many concurrently executed requests "+
		"may require high amounts of memory. See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the incoming vmselect request waits for execution "+
		"when -search.maxConcurrentRequests limit is reached")

	disableRPCCompression = flag.Bool("rpc.disableCompression", false, "Whether to disable compression of the data sent from vmstorage to vmselect. "+
		"This reduces CPU usage at the cost of higher network bandwidth usage")
	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside of the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
)

var (
	maxMetricsLimitByResource int
	once                      sync.Once
)

// NewVMSelectServer starts new server at the given addr, which serves vmselect requests from the given s.
func NewVMSelectServer(addr string, s *storage.Storage) (*vmselectapi.Server, error) {
	api := &vmstorageAPI{
		s: s,
	}
	limits := vmselectapi.Limits{
		MaxLabelNames:                 *maxTagKeys,
		MaxLabelValues:                *maxTagValues,
		MaxTagValueSuffixes:           *maxTagValueSuffixesPerSearch,
		MaxConcurrentRequests:         *maxConcurrentRequests,
		MaxConcurrentRequestsFlagName: "search.maxConcurrentRequests",
		MaxQueueDuration:              *maxQueueDuration,
		MaxQueueDurationFlagName:      "search.maxQueueDuration",
	}
	return vmselectapi.NewServer(addr, api, limits, *disableRPCCompression)
}

// vmstorageAPI impelements vmselectapi.API
type vmstorageAPI struct {
	s *storage.Storage
}

func (api *vmstorageAPI) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	tr := sq.GetTimeRange()
	if err := checkTimeRange(api.s, tr); err != nil {
		return nil, err
	}
	maxMetrics := getMaxMetrics(sq)
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

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(tfss) == 0 {
		return nil, fmt.Errorf("missing tag filters")
	}
	return api.s.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelValuesWithFiltersOnTimeRange(qt, sq.AccountID, sq.ProjectID, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
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

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	return api.s.SearchLabelNamesWithFiltersOnTimeRange(qt, sq.AccountID, sq.ProjectID, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

func (api *vmstorageAPI) SeriesCount(_ *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	return api.s.GetSeriesCount(accountID, projectID, deadline)
}

func (api *vmstorageAPI) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	return api.s.SearchTenants(qt, tr, deadline)
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(sq.MinTimestamp) / (24 * 3600 * 1000)
	return api.s.GetTSDBStatus(qt, sq.AccountID, sq.ProjectID, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

func (api *vmstorageAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	tr := sq.GetTimeRange()
	maxMetrics := getMaxMetrics(sq)
	tfss, err := api.setupTfss(qt, sq, tr, maxMetrics, deadline)
	if err != nil {
		return 0, err
	}
	if len(tfss) == 0 {
		return 0, fmt.Errorf("missing tag filters")
	}
	return api.s.DeleteSeries(qt, tfss, maxMetrics)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, _ uint64) error {
	api.s.RegisterMetricNames(qt, mrs)
	return nil
}

func (api *vmstorageAPI) setupTfss(qt *querytracer.Tracer, sq *storage.SearchQuery, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]*storage.TagFilters, error) {
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
						"see https://docs.victoriametrics.com/#resource-usage-limits", maxMetrics, query)
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

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	sr storage.Search
}

var blockIteratorsPool sync.Pool

func (bi *blockIterator) MustClose() {
	bi.sr.MustClose()
	blockIteratorsPool.Put(bi)
}

func getBlockIterator() *blockIterator {
	v := blockIteratorsPool.Get()
	if v == nil {
		v = &blockIterator{}
	}
	return v.(*blockIterator)
}

func (bi *blockIterator) NextBlock(mb *storage.MetricBlock) bool {
	if !bi.sr.NextMetricBlock() {
		return false
	}
	mb.MetricName = append(mb.MetricName[:0], bi.sr.MetricBlockRef.MetricName...)
	bi.sr.MetricBlockRef.BlockRef.MustReadBlock(&mb.Block)
	return true
}

func (bi *blockIterator) Error() error {
	return bi.sr.Error()
}

// checkTimeRange returns true if the given tr is denied for querying.
func checkTimeRange(s *storage.Storage, tr storage.TimeRange) error {
	if !*denyQueriesOutsideRetention {
		return nil
	}
	retentionMsecs := s.RetentionMsecs()
	minAllowedTimestamp := int64(fasttime.UnixTimestamp()*1000) - retentionMsecs
	if tr.MinTimestamp > minAllowedTimestamp {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err: fmt.Errorf("the given time range %s is outside the allowed retention %.3f days according to -denyQueriesOutsideRetention",
			&tr, float64(retentionMsecs)/(24*3600*1000)),
		StatusCode: http.StatusServiceUnavailable,
	}
}

func getMaxMetrics(sq *storage.SearchQuery) int {
	maxMetrics := sq.MaxMetrics
	maxMetricsLimit := *maxUniqueTimeseries
	if maxMetricsLimit <= 0 {
		maxMetricsLimit = GetMaxUniqueTimeSeries()
	}
	if maxMetrics <= 0 || maxMetrics > maxMetricsLimit {
		maxMetrics = maxMetricsLimit
	}
	return maxMetrics
}

// GetMaxUniqueTimeSeries returns the max metrics limit calculated by available resources.
// The calculation is split into calculateMaxUniqueTimeSeriesByResource for unit testing.
func GetMaxUniqueTimeSeries() int {
	once.Do(func() {
		maxMetricsLimitByResource = calculateMaxUniqueTimeSeriesByResource(*maxConcurrentRequests, memory.Remaining())
		logger.Infof("limiting -search.maxUniqueTimeseries to %d according to -search.maxConcurrentRequests=%d and remaining memory=%d bytes. To increase the limit, reduce -search.maxConcurrentRequests or increase memory available to the process.", maxMetricsLimitByResource, *maxConcurrentRequests, memory.Remaining())
	})
	return maxMetricsLimitByResource
}

// calculateMaxUniqueTimeSeriesByResource calculate the max metrics limit calculated by available resources.
func calculateMaxUniqueTimeSeriesByResource(maxConcurrentRequests, remainingMemory int) int {
	if maxConcurrentRequests <= 0 {
		// This line should NOT be reached unless the user has set an incorrect `search.maxConcurrentRequests`.
		// In such cases, fallback to unlimited.
		logger.Warnf("limiting -search.maxUniqueTimeseries to %v because -search.maxConcurrentRequests=%d.", 2e9, maxConcurrentRequests)
		return 2e9
	}
	// Calculate the max metrics limit for a single request in the worst-case concurrent scenario.
	// The approximate size of 1 unique series that could occupy in the vmstorage is 200 bytes.
	return remainingMemory / 200 / maxConcurrentRequests
}
