package clusternative

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxTagKeys                   = flag.Int("clusternative.maxTagKeys", 100e3, "The maximum number of tag keys returned per search at -clusternativeListenAddr")
	maxTagValues                 = flag.Int("clusternative.maxTagValues", 100e3, "The maximum number of tag values returned per search at -clusternativeListenAddr")
	maxTagValueSuffixesPerSearch = flag.Int("clusternative.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned "+
		"from /metrics/find at -clusternativeListenAddr")
	maxConcurrentRequests = flag.Int("clusternative.maxConcurrentRequests", 2*cgroup.AvailableCPUs(), "The maximum number of concurrent vmselect requests "+
		"the server can process at -clusternativeListenAddr. It shouldn't be high, since a single request usually saturates a CPU core at the underlying vmstorage nodes, "+
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
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	bi := newBlockIterator(qt, sq, dl)
	return bi, nil
}

func (api *vmstorageAPI) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	dl := searchutil.DeadlineFromTimestamp(deadline)
	return netstorage.Tenants(qt, tr, dl)
}

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	metricNames, err := netstorage.SearchMetricNames(qt, sq, dl)
	return metricNames, err
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	labelValues, err := netstorage.LabelValues(qt, labelName, sq, maxLabelValues, dl)
	return labelValues, err
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	if !netstorage.IsSameTenant(accountID, projectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	suffixes, err := netstorage.TagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, dl)
	return suffixes, err
}

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	labelNames, err := netstorage.LabelNames(qt, sq, maxLabelNames, dl)
	return labelNames, err
}

func (api *vmstorageAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	if !netstorage.IsSameTenant(accountID, projectID) {
		return 0, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	seriesCount, err := netstorage.SeriesCount(qt, dl)
	return seriesCount, err
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return nil, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	tsdbStatus, err := netstorage.TSDBStatus(qt, sq, focusLabel, topN, dl)
	return tsdbStatus, err
}

func (api *vmstorageAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	if !netstorage.IsSameTenant(sq.AccountID, sq.ProjectID) {
		return 0, nil
	}
	dl := searchutil.DeadlineFromTimestamp(deadline)
	return netstorage.DeleteSeries(qt, sq, dl)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	dest := mrs[:0]
	for _, m := range mrs {
		if netstorage.HasSameTenantPrefix(m.MetricNameRaw) {
			dest = append(dest, m)
		}
	}
	mrs = dest
	dl := searchutil.DeadlineFromTimestamp(deadline)
	return netstorage.RegisterMetricNames(qt, mrs, dl)
}

func (api *vmstorageAPI) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	return netstorage.ResetMetricNamesStats(qt)
}

func (api *vmstorageAPI) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, le, limit int, matchPattern string, deadline uint64) (storage.MetricNamesStatsResponse, error) {
	if !netstorage.IsSameTenant(tt.AccountID, tt.ProjectID) {
		return storage.MetricNamesStatsResponse{}, nil
	}
	return netstorage.GetMetricNamesStats(qt, le, limit, matchPattern)
}

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	workCh chan workItem
	wg     sync.WaitGroup
	err    error
}

type workItem struct {
	mb     *storage.MetricBlock
	doneCh chan struct{}
}

func newBlockIterator(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutil.Deadline) *blockIterator {
	var bi blockIterator
	bi.workCh = make(chan workItem, 16)
	bi.wg.Add(1)
	go func() {
		err := netstorage.ProcessBlocks(qt, sq, func(mb *storage.MetricBlock) error {
			wi := workItem{
				mb:     mb,
				doneCh: make(chan struct{}),
			}
			bi.workCh <- wi
			<-wi.doneCh
			return nil
		}, deadline)
		close(bi.workCh)
		bi.err = err
		bi.wg.Done()
	}()
	return &bi
}

func (bi *blockIterator) NextBlock(mb *storage.MetricBlock) bool {
	wi, ok := <-bi.workCh
	if !ok {
		return false
	}
	mb.CopyFrom(wi.mb)
	wi.doneCh <- struct{}{}
	return true
}

func (bi *blockIterator) Error() error {
	bi.wg.Wait()
	return bi.err
}

func (bi *blockIterator) MustClose() {
	var mb storage.MetricBlock
	for bi.NextBlock(&mb) {
		// Drain pending blocks before exit in order to free up
		// the goroutine started at newBlockIterator
	}
	// Wait until the goroutine from newBlockIterator is finished.
	bi.wg.Wait()
}
