package clusternative

import (
	"flag"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

var (
	maxTagKeys                   = flag.Int("clusternative.maxTagKeys", 100e3, "The maximum number of tag keys returned per search at -clusternativeListenAddr")
	maxTagValues                 = flag.Int("clusternative.maxTagValues", 100e3, "The maximum number of tag values returned per search at -clusternativeListenAddr")
	maxTagValueSuffixesPerSearch = flag.Int("clusternative.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned "+
		"from /metrics/find at -clusternativeListenAddr")

	disableRPCCompression = flag.Bool(`clusternative.disableCompression`, false, "Whether to disable compression of the data sent to vmselect via -clusternativeListenAddr. "+
		"This reduces CPU usage at the cost of higher network bandwidth usage")
)

// NewVMSelectServer starts new server at the given addr, which serves vmselect requests from netstorage.
func NewVMSelectServer(addr string) (*vmselectapi.Server, error) {
	api := &vmstorageAPI{}
	limits := vmselectapi.Limits{
		MaxLabelNames:       *maxTagKeys,
		MaxLabelValues:      *maxTagValues,
		MaxTagValueSuffixes: *maxTagValueSuffixesPerSearch,
	}
	return vmselectapi.NewServer(addr, api, limits, *disableRPCCompression)
}

// vmstorageAPI impelements vmselectapi.API
type vmstorageAPI struct{}

func (api *vmstorageAPI) Tenants(qt *querytracer.Tracer, deadline uint64) ([]string, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	metricNames, _, err := netstorage.Tenants(qt, denyPartialResponse, dl)
	return metricNames, err
}

func (api *vmstorageAPI) InitSearch(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (vmselectapi.BlockIterator, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	bi := newBlockIterator(qt, denyPartialResponse, sq, dl)
	return bi, nil
}

func (api *vmstorageAPI) SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) ([]string, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	metricNames, _, err := netstorage.SearchMetricNames(qt, denyPartialResponse, sq, dl)
	return metricNames, err
}

func (api *vmstorageAPI) LabelValues(qt *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int, deadline uint64) ([]string, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	labelValues, _, err := netstorage.LabelValues(qt, denyPartialResponse, labelName, sq, maxLabelValues, dl)
	return labelValues, err
}

func (api *vmstorageAPI) TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte,
	maxSuffixes int, deadline uint64) ([]string, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	suffixes, _, err := netstorage.TagValueSuffixes(qt, accountID, projectID, denyPartialResponse, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, dl)
	return suffixes, err
}

func (api *vmstorageAPI) LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline uint64) ([]string, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	labelNames, _, err := netstorage.LabelNames(qt, denyPartialResponse, sq, maxLabelNames, dl)
	return labelNames, err
}

func (api *vmstorageAPI) SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline uint64) (uint64, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	seriesCount, _, err := netstorage.SeriesCount(qt, accountID, projectID, denyPartialResponse, dl)
	return seriesCount, err
}

func (api *vmstorageAPI) TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline uint64) (*storage.TSDBStatus, error) {
	denyPartialResponse := searchutils.GetDenyPartialResponse(nil)
	dl := searchutils.DeadlineFromTimestamp(deadline)
	tsdbStatus, _, err := netstorage.TSDBStatus(qt, denyPartialResponse, sq, focusLabel, topN, dl)
	return tsdbStatus, err
}

func (api *vmstorageAPI) DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline uint64) (int, error) {
	dl := searchutils.DeadlineFromTimestamp(deadline)
	return netstorage.DeleteSeries(qt, sq, dl)
}

func (api *vmstorageAPI) RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline uint64) error {
	dl := searchutils.DeadlineFromTimestamp(deadline)
	return netstorage.RegisterMetricNames(qt, mrs, dl)
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

func newBlockIterator(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutils.Deadline) *blockIterator {
	var bi blockIterator
	bi.workCh = make(chan workItem, 16)
	bi.wg.Add(1)
	go func() {
		_, err := netstorage.ProcessBlocks(qt, denyPartialResponse, sq, func(mb *storage.MetricBlock, workerID uint) error {
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
