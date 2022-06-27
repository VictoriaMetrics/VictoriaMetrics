package transport

import (
	"flag"
	"fmt"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxUniqueTimeseries          = flag.Int("search.maxUniqueTimeseries", 0, "The maximum number of unique time series, which can be scanned during every query. This allows protecting against heavy queries, which select unexpectedly high number of series. Zero means 'no limit'. See also -search.max* command-line flags at vmselect")
	maxTagKeys                   = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search")
	maxTagValues                 = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")

	disableRPCCompression = flag.Bool(`rpc.disableCompression`, false, "Whether to disable compression of the data sent from vmstorage to vmselect. "+
		"This reduces CPU usage at the cost of higher network bandwidth usage")
	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside of the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
)

// NewVMSelectServer starts new server at the given addr, which serves vmselect requests from the given s.
func NewVMSelectServer(addr string, s *storage.Storage) (*vmselectapi.Server, error) {
	api := &vmstorageAPI{
		s: s,
	}
	limits := vmselectapi.Limits{
		MaxMetrics:          *maxUniqueTimeseries,
		MaxLabelNames:       *maxTagKeys,
		MaxLabelValues:      *maxTagValues,
		MaxTagValueSuffixes: *maxTagValueSuffixesPerSearch,
	}
	return vmselectapi.NewServer(addr, api, limits, *disableRPCCompression)
}

// vmstorageAPI impelemnts vmselectapi.API
type vmstorageAPI struct {
	s *storage.Storage
}

func (api *vmstorageAPI) InitSearch(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxMetrics int, deadline uint64) (vmselectapi.BlockIterator, error) {
	if err := checkTimeRange(api.s, tr); err != nil {
		return nil, err
	}
	bi := getBlockIterator()
	bi.sr.Init(qt, api.s, tfss, tr, maxMetrics, deadline)
	if err := bi.sr.Error(); err != nil {
		bi.MustClose()
		return nil, err
	}
	return bi, nil
}

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]storage.MetricName, error) {
	return api.s.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, tr storage.TimeRange, labelName string,
	maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	return api.s.SearchLabelValuesWithFiltersOnTimeRange(qt, accountID, projectID, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix []byte, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	return api.s.SearchTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
}

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, tr storage.TimeRange, maxLabelNames,
	maxMetrics int, deadline uint64) ([]string, error) {
	return api.s.SearchLabelNamesWithFiltersOnTimeRange(qt, accountID, projectID, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

func (api *vmstorageAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	return api.s.GetSeriesCount(accountID, projectID, deadline)
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*storage.TagFilters, date uint64, focusLabel string,
	topN, maxMetrics int, deadline uint64) (*storage.TSDBStatus, error) {
	return api.s.GetTSDBStatus(qt, accountID, projectID, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

func (api *vmstorageAPI) DeleteMetrics(qt *querytracer.Tracer, tfss []*storage.TagFilters, maxMetrics int, deadline uint64) (int, error) {
	return api.s.DeleteMetrics(qt, tfss)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow) error {
	return api.s.RegisterMetricNames(qt, mrs)
}

func (api *vmstorageAPI) SearchGraphitePaths(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, query []byte,
	maxMetrics int, deadline uint64) ([]string, error) {
	return api.s.SearchGraphitePaths(qt, accountID, projectID, tr, query, maxMetrics, deadline)
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

func (bi *blockIterator) NextBlock(mb *storage.MetricBlock, fetchData bool) bool {
	if !bi.sr.NextMetricBlock() {
		return false
	}
	mb.MetricName = bi.sr.MetricBlockRef.MetricName
	bi.sr.MetricBlockRef.BlockRef.MustReadBlock(&mb.Block, fetchData)
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
