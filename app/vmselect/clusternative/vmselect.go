package clusternative

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
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
	maxConcurrentRequests = flagutil.NewIntWithDynamicDefault("clusternative.maxConcurrentRequests", 2*cgroup.AvailableCPUs(), "2*cgroup.AvailableCPUs()", "The maximum number of concurrent vmselect requests "+
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
	ctx, cancel := contextWithDeadline(deadline)
	bi := newBlockIterator(ctx, cancel, qt, true, sq)
	return bi, nil
}

func (api *vmstorageAPI) Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline uint64) ([]string, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	res, err := netstorage.Tenants(ctx, qt, tr)
	return res, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	metricNames, _, err := netstorage.SearchMetricNames(ctx, qt, true, sq)
	return metricNames, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	if maxLabelValues <= 0 || maxLabelValues > *maxTagValues {
		maxLabelValues = *maxTagValues
	}
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	labelValues, _, err := netstorage.LabelValues(ctx, qt, true, labelName, sq, maxLabelValues)
	return labelValues, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	if maxSuffixes <= 0 || maxSuffixes > *maxTagValueSuffixesPerSearch {
		maxSuffixes = *maxTagValueSuffixesPerSearch
	}
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	suffixes, _, err := netstorage.TagValueSuffixes(ctx, qt, accountID, projectID, true, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes)
	return suffixes, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	if maxLabelNames <= 0 || maxLabelNames > *maxTagKeys {
		maxLabelNames = *maxTagKeys
	}
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	labelNames, _, err := netstorage.LabelNames(ctx, qt, true, sq, maxLabelNames)
	return labelNames, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	seriesCount, _, err := netstorage.SeriesCount(ctx, qt, accountID, projectID, true)
	return seriesCount, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	tsdbStatus, _, err := netstorage.TSDBStatus(ctx, qt, true, sq, focusLabel, topN)
	return tsdbStatus, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	deletedTotal, err := netstorage.DeleteSeries(ctx, qt, sq)
	return deletedTotal, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	return wrapClusterNativeError(netstorage.RegisterMetricNames(ctx, qt, mrs))
}

func (api *vmstorageAPI) ResetMetricNamesUsageStats(qt *querytracer.Tracer, deadline uint64) error {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	return wrapClusterNativeError(netstorage.ResetMetricNamesStats(ctx, qt))
}

func (api *vmstorageAPI) GetMetricNamesUsageStats(qt *querytracer.Tracer, tt *storage.TenantToken, le, limit int, matchPattern string, deadline uint64) (metricnamestats.StatsResult, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	statResult, err := netstorage.GetMetricNamesStats(ctx, qt, tt, le, limit, matchPattern)
	return statResult, wrapClusterNativeError(err)
}

func (api *vmstorageAPI) GetMetadataRecords(qt *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string, deadline uint64) ([]*metricsmetadata.Row, error) {
	ctx, cancel := contextWithDeadline(deadline)
	defer cancel()
	meta, _, err := netstorage.GetMetricsMetadata(ctx, qt, tt, true, limit, metricName)
	return meta, wrapClusterNativeError(err)
}

// blockIterator implements vmselectapi.BlockIterator
type blockIterator struct {
	workCh chan workItem
	wis    []workItem
	wg     sync.WaitGroup
	err    error

	cancel func()
}

type workItem struct {
	rawMetricBlock []byte
	doneCh         chan struct{}
}

func newBlockIterator(ctx context.Context, cancel func(), qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery) *blockIterator {
	bi := getBlockIterator()
	bi.cancel = cancel
	workers, processBlocks := netstorage.PrepareProcessRawBlocks(ctx, qt, denyPartialResponse, sq)
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
		// iterator error cannot be forwarded to the upstream vmselect,
		// upstream receives only unexpected EOF error.
		// But it's better to wrap error in any way for consistency.
		bi.err = wrapClusterNativeError(err)
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
	bi.cancel()
	bi.cancel = nil
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

// wrapClusterNativeError wraps err with a "vmselectClusterNative" prefix,
// allowing callers (e.g. netstorage.collectResults) to distinguish errors
// originating from the cluster-native vmselect path.
func wrapClusterNativeError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("vmselectClusterNative: %w", err)
}

func contextWithDeadline(deadline uint64) (context.Context, func()) {
	d := time.Unix(int64(deadline), 0)
	return context.WithDeadline(context.Background(), d)
}
