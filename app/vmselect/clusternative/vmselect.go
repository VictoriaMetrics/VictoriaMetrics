package clusternative

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxTagKeys                   = flag.Int("clusternative.maxTagKeys", 100e3, "The maximum number of tag keys returned per search at -clusternativeListenAddr")
	maxTagValues                 = flag.Int("clusternative.maxTagValues", 100e3, "The maximum number of tag values returned per search at -clusternativeListenAddr")
	maxTagValueSuffixesPerSearch = flag.Int("clusternative.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned "+
		"from /metrics/find at -clusternativeListenAddr")
	maxConcurrentRequests = flag.Int("clusternative.maxConcurrentRequests", 2*cgroup.AvailableCPUs(), "The maximum number of concurrent vmselect requests "+
		"the server can process at -clusternativeListenAddr. Default value depends on the number of available CPU cores. It shouldn't be high, since a single request usually saturates a CPU core at the underlying vmstorage nodes, "+
		"and many concurrently executed requests may require high amounts of memory. See also -clusternative.maxQueueDuration")
	maxQueueDuration = flag.Duration("clusternative.maxQueueDuration", 10*time.Second, "The maximum time the incoming query to -clusternativeListenAddr waits for execution "+
		"when -clusternative.maxConcurrentRequests limit is reached")

	disableRPCCompression = flag.Bool("clusternative.disableCompression", false, "Whether to disable compression of the data sent to vmselect via -clusternativeListenAddr. "+
		"This reduces CPU usage at the cost of higher network bandwidth usage")
)

// NewVMSelectServer starts new server at the given addr, which serves vmselect requests from netstorage.
func NewVMSelectServer(addr string) (*vmselectapi.Server, error) {
	api := &vmstorageAPI{}
	limits := vmselectapi.Limits{
		MaxLabelNames:                 *maxTagKeys,
		MaxLabelValues:                *maxTagValues,
		MaxTagValueSuffixes:           *maxTagValueSuffixesPerSearch,
		MaxConcurrentRequests:         *maxConcurrentRequests,
		MaxConcurrentRequestsFlagName: "clusternative.maxConcurrentRequests",
		MaxQueueDuration:              *maxQueueDuration,
		MaxQueueDurationFlagName:      "clusternative.maxQueueDuration",
	}
	return vmselectapi.NewServer(addr, api, limits, *disableRPCCompression)
}

// vmstorageAPI impelements vmselectapi.API
type vmstorageAPI struct{}

func (api *vmstorageAPI) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	bi := newBlockIterator(qt, true, sq, dl)
	return bi, nil
}

func (api *vmstorageAPI) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	res, err := netstorage.Tenants(qt, tr, dl)
	return res, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	metricNames, _, err := netstorage.SearchMetricNames(qt, true, sq, dl)
	return metricNames, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	labelValues, _, err := netstorage.LabelValues(qt, true, labelName, sq, maxLabelValues, dl)
	return labelValues, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	suffixes, _, err := netstorage.TagValueSuffixes(qt, accountID, projectID, true, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, dl)
	return suffixes, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	labelNames, _, err := netstorage.LabelNames(qt, true, sq, maxLabelNames, dl)
	return labelNames, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	seriesCount, _, err := netstorage.SeriesCount(qt, accountID, projectID, true, dl)
	return seriesCount, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	tsdbStatus, _, err := netstorage.TSDBStatus(qt, true, sq, focusLabel, topN, dl)
	return tsdbStatus, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	deletedTotal, err := netstorage.DeleteSeries(qt, sq, dl)
	return deletedTotal, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	return vmselectapi.ErrorWithVmselectApiPrefix(netstorage.RegisterMetricNames(qt, mrs, dl))
}

func (api *vmstorageAPI) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	return vmselectapi.ErrorWithVmselectApiPrefix(netstorage.ResetMetricNamesStats(qt, dl))
}

func (api *vmstorageAPI) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, le, limit int, matchPattern string, deadline uint64) (metricnamestats.StatsResult, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	statResult, err := netstorage.GetMetricNamesStats(qt, tt, le, limit, matchPattern, dl)
	return statResult, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

func (api *vmstorageAPI) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	meta, _, err := netstorage.GetMetricsMetadata(qt, tt, true, limit, metricName, dl)
	return meta, vmselectapi.ErrorWithVmselectApiPrefix(err)
}

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	workCh chan workItem
	wis    []workItem
	wg     sync.WaitGroup
	err    error
}

type workItem struct {
	rawMetricBlock []byte
	doneCh         chan struct{}
}

func newBlockIterator(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutil.Deadline) *blockIterator {
	bi := getBlockIterator()
	workers, processBlocks := netstorage.PrepareProcessRawBlocks(qt, denyPartialResponse, sq, deadline)
	bi.workCh = make(chan workItem, workers)
	bi.wis = slicesutil.SetLength(bi.wis, workers)
	for i := range bi.wis {
		bi.wis[i].doneCh = make(chan struct{})
	}
	bi.wg.Go(func() {
		_, err := processBlocks(func(mb []byte, workerID uint) error {
			wi := bi.wis[workerID]
			wi.rawMetricBlock = mb
			bi.workCh <- wi
			<-wi.doneCh
			return nil
		})
		close(bi.workCh)
		bi.err = vmselectapi.ErrorWithVmselectApiPrefix(err)
	})
	return bi
}

func (bi *blockIterator) NextBlock(dst []byte) ([]byte, bool) {
	wi, ok := <-bi.workCh
	if !ok {
		return nil, false
	}
	dst = append(dst, wi.rawMetricBlock...)
	wi.doneCh <- struct{}{}
	return dst, true
}

func (bi *blockIterator) Error() error {
	bi.wg.Wait()
	return bi.err
}

func (bi *blockIterator) MustClose() {
	var buf []byte
	var ok bool
	for {
		buf, ok = bi.NextBlock(buf[:0])
		if !ok {
			break
		}
		// Drain pending blocks before exit in order to free up
		// the goroutine started at newBlockIterator
	}
	// Wait until the goroutine from newBlockIterator is finished.
	bi.wg.Wait()
	for i := range bi.wis {
		wi := &bi.wis[i]
		wi.rawMetricBlock = nil
		wi.doneCh = nil
	}
	bi.err = nil
	bi.workCh = nil
	blockIteratorsPool.Put(bi)
}

var blockIteratorsPool sync.Pool

func getBlockIterator() *blockIterator {
	v := blockIteratorsPool.Get()
	if v == nil {
		v = &blockIterator{}
	}
	return v.(*blockIterator)
}
