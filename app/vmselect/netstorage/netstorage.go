package netstorage

import (
	"container/heap"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
	"github.com/valyala/fastrand"
)

var (
	replicationFactor = flag.Int("replicationFactor", 1, "How many copies of every time series is available on vmstorage nodes. "+
		"See -replicationFactor command-line flag for vminsert nodes")
	maxSamplesPerSeries = flag.Int("search.maxSamplesPerSeries", 30e6, "The maximum number of raw samples a single query can scan per each time series. See also -search.maxSamplesPerQuery")
	maxSamplesPerQuery  = flag.Int("search.maxSamplesPerQuery", 1e9, "The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries")
)

// Result is a single timeseries result.
//
// ProcessSearchQuery returns Result slice.
type Result struct {
	// The name of the metric.
	MetricName storage.MetricName

	// Values are sorted by Timestamps.
	Values     []float64
	Timestamps []int64
}

func (r *Result) reset() {
	r.MetricName.Reset()
	r.Values = r.Values[:0]
	r.Timestamps = r.Timestamps[:0]
}

// Results holds results returned from ProcessSearchQuery.
type Results struct {
	at        *auth.Token
	tr        storage.TimeRange
	fetchData bool
	deadline  searchutils.Deadline

	tbf *tmpBlocksFile

	packedTimeseries []packedTimeseries
}

// Len returns the number of results in rss.
func (rss *Results) Len() int {
	return len(rss.packedTimeseries)
}

// Cancel cancels rss work.
func (rss *Results) Cancel() {
	putTmpBlocksFile(rss.tbf)
	rss.tbf = nil
}

type timeseriesWork struct {
	mustStop *uint32
	rss      *Results
	pts      *packedTimeseries
	f        func(rs *Result, workerID uint) error
	doneCh   chan error

	rowsProcessed int
}

func (tsw *timeseriesWork) reset() {
	tsw.mustStop = nil
	tsw.rss = nil
	tsw.pts = nil
	tsw.f = nil
	if n := len(tsw.doneCh); n > 0 {
		logger.Panicf("BUG: tsw.doneCh must be empty during reset; it contains %d items instead", n)
	}
	tsw.rowsProcessed = 0
}

func getTimeseriesWork() *timeseriesWork {
	v := tswPool.Get()
	if v == nil {
		v = &timeseriesWork{
			doneCh: make(chan error, 1),
		}
	}
	return v.(*timeseriesWork)
}

func putTimeseriesWork(tsw *timeseriesWork) {
	tsw.reset()
	tswPool.Put(tsw)
}

var tswPool sync.Pool

func scheduleTimeseriesWork(workChs []chan *timeseriesWork, tsw *timeseriesWork) {
	if len(workChs) == 1 {
		// Fast path for a single worker
		workChs[0] <- tsw
		return
	}
	attempts := 0
	for {
		idx := fastrand.Uint32n(uint32(len(workChs)))
		select {
		case workChs[idx] <- tsw:
			return
		default:
			attempts++
			if attempts >= len(workChs) {
				workChs[idx] <- tsw
				return
			}
		}
	}
}

func (tsw *timeseriesWork) do(r *Result, workerID uint) error {
	if atomic.LoadUint32(tsw.mustStop) != 0 {
		return nil
	}
	rss := tsw.rss
	if rss.deadline.Exceeded() {
		atomic.StoreUint32(tsw.mustStop, 1)
		return fmt.Errorf("timeout exceeded during query execution: %s", rss.deadline.String())
	}
	if err := tsw.pts.Unpack(r, rss.tbf, rss.tr, rss.fetchData, rss.at); err != nil {
		atomic.StoreUint32(tsw.mustStop, 1)
		return fmt.Errorf("error during time series unpacking: %w", err)
	}
	if len(r.Timestamps) > 0 || !rss.fetchData {
		if err := tsw.f(r, workerID); err != nil {
			atomic.StoreUint32(tsw.mustStop, 1)
			return err
		}
	}
	tsw.rowsProcessed = len(r.Values)
	return nil
}

func timeseriesWorker(ch <-chan *timeseriesWork, workerID uint) {
	v := resultPool.Get()
	if v == nil {
		v = &result{}
	}
	r := v.(*result)
	for tsw := range ch {
		err := tsw.do(&r.rs, workerID)
		tsw.doneCh <- err
	}
	currentTime := fasttime.UnixTimestamp()
	if cap(r.rs.Values) > 1024*1024 && 4*len(r.rs.Values) < cap(r.rs.Values) && currentTime-r.lastResetTime > 10 {
		// Reset r.rs in order to preseve memory usage after processing big time series with millions of rows.
		r.rs = Result{}
		r.lastResetTime = currentTime
	}
	resultPool.Put(r)
}

type result struct {
	rs            Result
	lastResetTime uint64
}

var resultPool sync.Pool

// RunParallel runs f in parallel for all the results from rss.
//
// f shouldn't hold references to rs after returning.
// workerID is the id of the worker goroutine that calls f.
// Data processing is immediately stopped if f returns non-nil error.
//
// rss becomes unusable after the call to RunParallel.
func (rss *Results) RunParallel(f func(rs *Result, workerID uint) error) error {
	defer func() {
		putTmpBlocksFile(rss.tbf)
		rss.tbf = nil
	}()

	// Spin up local workers.
	//
	// Do not use a global workChs with a global pool of workers, since it may lead to a deadlock in the following case:
	// - RunParallel is called with f, which blocks without forward progress.
	// - All the workers in the global pool became blocked in f.
	// - workChs is filled up, so it cannot accept new work items from other RunParallel calls.
	workers := len(rss.packedTimeseries)
	if workers > gomaxprocs {
		workers = gomaxprocs
	}
	if workers < 1 {
		workers = 1
	}
	workChs := make([]chan *timeseriesWork, workers)
	var workChsWG sync.WaitGroup
	for i := 0; i < workers; i++ {
		workChs[i] = make(chan *timeseriesWork, 16)
		workChsWG.Add(1)
		go func(workerID int) {
			defer workChsWG.Done()
			timeseriesWorker(workChs[workerID], uint(workerID))
		}(i)
	}

	// Feed workers with work.
	tsws := make([]*timeseriesWork, len(rss.packedTimeseries))
	var mustStop uint32
	for i := range rss.packedTimeseries {
		tsw := getTimeseriesWork()
		tsw.rss = rss
		tsw.pts = &rss.packedTimeseries[i]
		tsw.f = f
		tsw.mustStop = &mustStop
		scheduleTimeseriesWork(workChs, tsw)
		tsws[i] = tsw
	}
	seriesProcessedTotal := len(rss.packedTimeseries)
	rss.packedTimeseries = rss.packedTimeseries[:0]

	// Wait until work is complete.
	var firstErr error
	rowsProcessedTotal := 0
	for _, tsw := range tsws {
		if err := <-tsw.doneCh; err != nil && firstErr == nil {
			// Return just the first error, since other errors are likely duplicate the first error.
			firstErr = err
		}
		rowsProcessedTotal += tsw.rowsProcessed
		putTimeseriesWork(tsw)
	}

	perQueryRowsProcessed.Update(float64(rowsProcessedTotal))
	perQuerySeriesProcessed.Update(float64(seriesProcessedTotal))

	// Shut down local workers
	for _, workCh := range workChs {
		close(workCh)
	}
	workChsWG.Wait()

	return firstErr
}

var perQueryRowsProcessed = metrics.NewHistogram(`vm_per_query_rows_processed_count`)
var perQuerySeriesProcessed = metrics.NewHistogram(`vm_per_query_series_processed_count`)

var gomaxprocs = cgroup.AvailableCPUs()

type packedTimeseries struct {
	metricName string
	addrs      []tmpBlockAddr
}

type unpackWorkItem struct {
	addr tmpBlockAddr
	tr   storage.TimeRange
}

type unpackWork struct {
	ws     []unpackWorkItem
	tbf    *tmpBlocksFile
	at     *auth.Token
	sbs    []*sortBlock
	doneCh chan error
}

func (upw *unpackWork) reset() {
	ws := upw.ws
	for i := range ws {
		w := &ws[i]
		w.addr = tmpBlockAddr{}
		w.tr = storage.TimeRange{}
	}
	upw.ws = upw.ws[:0]
	upw.tbf = nil
	upw.at = nil
	sbs := upw.sbs
	for i := range sbs {
		sbs[i] = nil
	}
	upw.sbs = upw.sbs[:0]
	if n := len(upw.doneCh); n > 0 {
		logger.Panicf("BUG: upw.doneCh must be empty; it contains %d items now", n)
	}
}

func (upw *unpackWork) unpack(tmpBlock *storage.Block) {
	for _, w := range upw.ws {
		sb := getSortBlock()
		if err := sb.unpackFrom(tmpBlock, upw.tbf, w.addr, w.tr, upw.at); err != nil {
			putSortBlock(sb)
			upw.doneCh <- fmt.Errorf("cannot unpack block: %w", err)
			return
		}
		upw.sbs = append(upw.sbs, sb)
	}
	upw.doneCh <- nil
}

func getUnpackWork() *unpackWork {
	v := unpackWorkPool.Get()
	if v != nil {
		return v.(*unpackWork)
	}
	return &unpackWork{
		doneCh: make(chan error, 1),
	}
}

func putUnpackWork(upw *unpackWork) {
	upw.reset()
	unpackWorkPool.Put(upw)
}

var unpackWorkPool sync.Pool

func scheduleUnpackWork(workChs []chan *unpackWork, uw *unpackWork) {
	if len(workChs) == 1 {
		// Fast path for a single worker
		workChs[0] <- uw
		return
	}
	attempts := 0
	for {
		idx := fastrand.Uint32n(uint32(len(workChs)))
		select {
		case workChs[idx] <- uw:
			return
		default:
			attempts++
			if attempts >= len(workChs) {
				workChs[idx] <- uw
				return
			}
		}
	}
}

func unpackWorker(ch <-chan *unpackWork) {
	v := tmpBlockPool.Get()
	if v == nil {
		v = &storage.Block{}
	}
	tmpBlock := v.(*storage.Block)
	for upw := range ch {
		upw.unpack(tmpBlock)
	}
	tmpBlockPool.Put(v)
}

var tmpBlockPool sync.Pool

// unpackBatchSize is the maximum number of blocks that may be unpacked at once by a single goroutine.
//
// It is better to load a single goroutine for up to one second on a system with many CPU cores
// in order to reduce inter-CPU memory ping-pong.
// A single goroutine can unpack up to 40 millions of rows per second, while a single block contains up to 8K rows.
// So the batch size should be 40M / 8K = 5K.
var unpackBatchSize = 5000

// Unpack unpacks pts to dst.
func (pts *packedTimeseries) Unpack(dst *Result, tbf *tmpBlocksFile, tr storage.TimeRange, fetchData bool, at *auth.Token) error {
	dst.reset()
	if err := dst.MetricName.Unmarshal(bytesutil.ToUnsafeBytes(pts.metricName)); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %w", pts.metricName, err)
	}
	if !fetchData {
		// Do not spend resources on data reading and unpacking.
		return nil
	}

	// Spin up local workers.
	// Do not use global workers pool, since it increases inter-CPU memory ping-poing,
	// which reduces the scalability on systems with many CPU cores.
	addrsLen := len(pts.addrs)
	workers := addrsLen / unpackBatchSize
	if workers > gomaxprocs {
		workers = gomaxprocs
	}
	if workers < 1 {
		workers = 1
	}
	workChs := make([]chan *unpackWork, workers)
	var workChsWG sync.WaitGroup
	for i := 0; i < workers; i++ {
		// Use unbuffered channel on purpose, since there are high chances
		// that only a single unpackWork is needed to unpack.
		// The unbuffered channel should reduce inter-CPU ping-pong in this case,
		// which should improve the performance in a system with many CPU cores.
		workChs[i] = make(chan *unpackWork)
		workChsWG.Add(1)
		go func(workerID int) {
			defer workChsWG.Done()
			unpackWorker(workChs[workerID])
		}(i)
	}

	// Feed workers with work
	upws := make([]*unpackWork, 0, 1+addrsLen/unpackBatchSize)
	upw := getUnpackWork()
	upw.tbf = tbf
	upw.at = at
	for _, addr := range pts.addrs {
		if len(upw.ws) >= unpackBatchSize {
			scheduleUnpackWork(workChs, upw)
			upws = append(upws, upw)
			upw = getUnpackWork()
			upw.tbf = tbf
			upw.at = at
		}
		upw.ws = append(upw.ws, unpackWorkItem{
			addr: addr,
			tr:   tr,
		})
	}
	scheduleUnpackWork(workChs, upw)
	upws = append(upws, upw)
	pts.addrs = pts.addrs[:0]

	// Wait until work is complete
	samples := 0
	sbs := make([]*sortBlock, 0, addrsLen)
	var firstErr error
	for _, upw := range upws {
		if err := <-upw.doneCh; err != nil && firstErr == nil {
			// Return the first error only, since other errors are likely the same.
			firstErr = err
		}
		if firstErr == nil {
			for _, sb := range upw.sbs {
				samples += len(sb.Timestamps)
			}
			if *maxSamplesPerSeries <= 0 || samples < *maxSamplesPerSeries {
				sbs = append(sbs, upw.sbs...)
			} else {
				firstErr = fmt.Errorf("cannot process more than %d samples per series; either increase -search.maxSamplesPerSeries "+
					"or reduce time range for the query", *maxSamplesPerSeries)
			}
		}
		if firstErr != nil {
			for _, sb := range upw.sbs {
				putSortBlock(sb)
			}
		}
		putUnpackWork(upw)
	}

	// Shut down local workers
	for _, workCh := range workChs {
		close(workCh)
	}
	workChsWG.Wait()

	if firstErr != nil {
		return firstErr
	}
	dedupInterval := storage.GetDedupInterval()
	mergeSortBlocks(dst, sbs, dedupInterval)
	return nil
}

func getSortBlock() *sortBlock {
	v := sbPool.Get()
	if v == nil {
		return &sortBlock{}
	}
	return v.(*sortBlock)
}

func putSortBlock(sb *sortBlock) {
	sb.reset()
	sbPool.Put(sb)
}

var sbPool sync.Pool

var metricRowsSkipped = metrics.NewCounter(`vm_metric_rows_skipped_total{name="vmselect"}`)

func mergeSortBlocks(dst *Result, sbh sortBlocksHeap, dedupInterval int64) {
	// Skip empty sort blocks, since they cannot be passed to heap.Init.
	src := sbh
	sbh = sbh[:0]
	for _, sb := range src {
		if len(sb.Timestamps) == 0 {
			putSortBlock(sb)
			continue
		}
		sbh = append(sbh, sb)
	}
	if len(sbh) == 0 {
		return
	}
	heap.Init(&sbh)
	for {
		top := sbh[0]
		heap.Pop(&sbh)
		if len(sbh) == 0 {
			dst.Timestamps = append(dst.Timestamps, top.Timestamps[top.NextIdx:]...)
			dst.Values = append(dst.Values, top.Values[top.NextIdx:]...)
			putSortBlock(top)
			break
		}
		sbNext := sbh[0]
		tsNext := sbNext.Timestamps[sbNext.NextIdx]
		idxNext := len(top.Timestamps)
		if top.Timestamps[idxNext-1] > tsNext {
			idxNext = top.NextIdx
			for top.Timestamps[idxNext] <= tsNext {
				idxNext++
			}
		}
		dst.Timestamps = append(dst.Timestamps, top.Timestamps[top.NextIdx:idxNext]...)
		dst.Values = append(dst.Values, top.Values[top.NextIdx:idxNext]...)
		if idxNext < len(top.Timestamps) {
			top.NextIdx = idxNext
			heap.Push(&sbh, top)
		} else {
			// Return top to the pool.
			putSortBlock(top)
		}
	}
	timestamps, values := storage.DeduplicateSamples(dst.Timestamps, dst.Values, dedupInterval)
	dedups := len(dst.Timestamps) - len(timestamps)
	dedupsDuringSelect.Add(dedups)
	dst.Timestamps = timestamps
	dst.Values = values
}

var dedupsDuringSelect = metrics.NewCounter(`vm_deduplicated_samples_total{type="select"}`)

type sortBlock struct {
	Timestamps []int64
	Values     []float64
	NextIdx    int
}

func (sb *sortBlock) reset() {
	sb.Timestamps = sb.Timestamps[:0]
	sb.Values = sb.Values[:0]
	sb.NextIdx = 0
}

func (sb *sortBlock) unpackFrom(tmpBlock *storage.Block, tbf *tmpBlocksFile, addr tmpBlockAddr, tr storage.TimeRange, at *auth.Token) error {
	tmpBlock.Reset()
	tbf.MustReadBlockAt(tmpBlock, addr)
	if err := tmpBlock.UnmarshalData(); err != nil {
		return fmt.Errorf("cannot unmarshal block: %w", err)
	}
	sb.Timestamps, sb.Values = tmpBlock.AppendRowsWithTimeRangeFilter(sb.Timestamps[:0], sb.Values[:0], tr)
	skippedRows := tmpBlock.RowsCount() - len(sb.Timestamps)
	metricRowsSkipped.Add(skippedRows)
	return nil
}

type sortBlocksHeap []*sortBlock

func (sbh sortBlocksHeap) Len() int {
	return len(sbh)
}

func (sbh sortBlocksHeap) Less(i, j int) bool {
	a := sbh[i]
	b := sbh[j]
	return a.Timestamps[a.NextIdx] < b.Timestamps[b.NextIdx]
}

func (sbh sortBlocksHeap) Swap(i, j int) {
	sbh[i], sbh[j] = sbh[j], sbh[i]
}

func (sbh *sortBlocksHeap) Push(x interface{}) {
	*sbh = append(*sbh, x.(*sortBlock))
}

func (sbh *sortBlocksHeap) Pop() interface{} {
	a := *sbh
	v := a[len(a)-1]
	*sbh = a[:len(a)-1]
	return v
}

// RegisterMetricNames registers metric names from mrs in the storage.
func RegisterMetricNames(at *auth.Token, mrs []storage.MetricRow, deadline searchutils.Deadline) error {
	// Split mrs among available vmstorage nodes.
	mrsPerNode := make([][]storage.MetricRow, len(storageNodes))
	for _, mr := range mrs {
		idx := 0
		if len(storageNodes) > 1 {
			// There is no need in using the same hash as for time series distribution in vminsert,
			// since RegisterMetricNames is used only in Graphite Tags API.
			h := xxhash.Sum64(mr.MetricNameRaw)
			idx = int(h % uint64(len(storageNodes)))
		}
		mrsPerNode[idx] = append(mrsPerNode[idx], mr)
	}

	// Push mrs to storage nodes in parallel.
	snr := startStorageNodesRequest(true, func(idx int, sn *storageNode) interface{} {
		sn.registerMetricNamesRequests.Inc()
		err := sn.registerMetricNames(mrsPerNode[idx], deadline)
		if err != nil {
			sn.registerMetricNamesErrors.Inc()
		}
		return &err
	})

	// Collect results
	err := snr.collectAllResults(func(result interface{}) error {
		errP := result.(*error)
		return *errP
	})
	if err != nil {
		return fmt.Errorf("cannot register series on all the vmstorage nodes: %w", err)
	}
	return nil
}

// DeleteSeries deletes time series matching the given sq.
func DeleteSeries(at *auth.Token, sq *storage.SearchQuery, deadline searchutils.Deadline) (int, error) {
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		deletedCount int
		err          error
	}
	snr := startStorageNodesRequest(true, func(idx int, sn *storageNode) interface{} {
		sn.deleteSeriesRequests.Inc()
		deletedCount, err := sn.deleteMetrics(requestData, deadline)
		if err != nil {
			sn.deleteSeriesErrors.Inc()
		}
		return &nodeResult{
			deletedCount: deletedCount,
			err:          err,
		}
	})

	// Collect results
	deletedTotal := 0
	err := snr.collectAllResults(func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		deletedTotal += nr.deletedCount
		return nil
	})
	if err != nil {
		return deletedTotal, fmt.Errorf("cannot delete time series on all the vmstorage nodes: %w", err)
	}
	return deletedTotal, nil
}

// GetLabelsOnTimeRange returns labels for the given tr until the given deadline.
func GetLabelsOnTimeRange(at *auth.Token, denyPartialResponse bool, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labels []string
		err    error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.labelsOnTimeRangeRequests.Inc()
		labels, err := sn.getLabelsOnTimeRange(at.AccountID, at.ProjectID, tr, deadline)
		if err != nil {
			sn.labelsOnTimeRangeErrors.Inc()
			err = fmt.Errorf("cannot get labels on time range from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labels: labels,
			err:    err,
		}
	})

	// Collect results
	var labels []string
	isPartial, err := snr.collectResults(partialLabelsOnTimeRangeResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labels = append(labels, nr.labels...)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch labels on time range from vmstorage nodes: %w", err)
	}

	// Deduplicate labels
	labels = deduplicateStrings(labels)
	// Substitute "" with "__name__"
	for i := range labels {
		if labels[i] == "" {
			labels[i] = "__name__"
		}
	}
	// Sort labels like Prometheus does
	sort.Strings(labels)
	return labels, isPartial, nil
}

// GetGraphiteTags returns Graphite tags until the given deadline.
func GetGraphiteTags(at *auth.Token, denyPartialResponse bool, filter string, limit int, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	labels, isPartial, err := GetLabels(at, denyPartialResponse, deadline)
	if err != nil {
		return nil, false, err
	}
	// Substitute "__name__" with "name" for Graphite compatibility
	for i := range labels {
		if labels[i] != "__name__" {
			continue
		}
		// Prevent from duplicate `name` tag.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/942
		if hasString(labels, "name") {
			labels = append(labels[:i], labels[i+1:]...)
		} else {
			labels[i] = "name"
			sort.Strings(labels)
		}
		break
	}
	if len(filter) > 0 {
		labels, err = applyGraphiteRegexpFilter(filter, labels)
		if err != nil {
			return nil, false, err
		}
	}
	if limit > 0 && limit < len(labels) {
		labels = labels[:limit]
	}
	return labels, isPartial, nil
}

func hasString(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// GetLabels returns labels until the given deadline.
func GetLabels(at *auth.Token, denyPartialResponse bool, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labels []string
		err    error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.labelsRequests.Inc()
		labels, err := sn.getLabels(at.AccountID, at.ProjectID, deadline)
		if err != nil {
			sn.labelsErrors.Inc()
			err = fmt.Errorf("cannot get labels from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labels: labels,
			err:    err,
		}
	})

	// Collect results
	var labels []string
	isPartial, err := snr.collectResults(partialLabelsResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labels = append(labels, nr.labels...)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch labels from vmstorage nodes: %w", err)
	}

	// Deduplicate labels
	labels = deduplicateStrings(labels)
	// Substitute "" with "__name__"
	for i := range labels {
		if labels[i] == "" {
			labels[i] = "__name__"
		}
	}
	// Sort labels like Prometheus does
	sort.Strings(labels)
	return labels, isPartial, nil
}

// GetLabelValuesOnTimeRange returns label values for the given labelName on the given tr
// until the given deadline.
func GetLabelValuesOnTimeRange(at *auth.Token, denyPartialResponse bool, labelName string, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if labelName == "__name__" {
		labelName = ""
	}

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelValues []string
		err         error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.labelValuesOnTimeRangeRequests.Inc()
		labelValues, err := sn.getLabelValuesOnTimeRange(at.AccountID, at.ProjectID, labelName, tr, deadline)
		if err != nil {
			sn.labelValuesOnTimeRangeErrors.Inc()
			err = fmt.Errorf("cannot get label values on time range from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labelValues: labelValues,
			err:         err,
		}
	})

	// Collect results
	var labelValues []string
	isPartial, err := snr.collectResults(partialLabelValuesOnTimeRangeResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labelValues = append(labelValues, nr.labelValues...)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch label values on time range from vmstorage nodes: %w", err)
	}

	// Deduplicate label values
	labelValues = deduplicateStrings(labelValues)
	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)
	return labelValues, isPartial, nil
}

// GetGraphiteTagValues returns tag values for the given tagName until the given deadline.
func GetGraphiteTagValues(at *auth.Token, denyPartialResponse bool, tagName, filter string, limit int, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if tagName == "name" {
		tagName = ""
	}
	tagValues, isPartial, err := GetLabelValues(at, denyPartialResponse, tagName, deadline)
	if err != nil {
		return nil, false, err
	}
	if len(filter) > 0 {
		tagValues, err = applyGraphiteRegexpFilter(filter, tagValues)
		if err != nil {
			return nil, false, err
		}
	}
	if limit > 0 && limit < len(tagValues) {
		tagValues = tagValues[:limit]
	}
	return tagValues, isPartial, nil
}

// GetLabelValues returns label values for the given labelName
// until the given deadline.
func GetLabelValues(at *auth.Token, denyPartialResponse bool, labelName string, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if labelName == "__name__" {
		labelName = ""
	}

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelValues []string
		err         error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.labelValuesRequests.Inc()
		labelValues, err := sn.getLabelValues(at.AccountID, at.ProjectID, labelName, deadline)
		if err != nil {
			sn.labelValuesErrors.Inc()
			err = fmt.Errorf("cannot get label values from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labelValues: labelValues,
			err:         err,
		}
	})

	// Collect results
	var labelValues []string
	isPartial, err := snr.collectResults(partialLabelValuesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labelValues = append(labelValues, nr.labelValues...)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch label values from vmstorage nodes: %w", err)
	}

	// Deduplicate label values
	labelValues = deduplicateStrings(labelValues)
	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)
	return labelValues, isPartial, nil
}

// GetTagValueSuffixes returns tag value suffixes for the given tagKey and the given tagValuePrefix.
//
// It can be used for implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func GetTagValueSuffixes(at *auth.Token, denyPartialResponse bool, tr storage.TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		suffixes []string
		err      error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.tagValueSuffixesRequests.Inc()
		suffixes, err := sn.getTagValueSuffixes(at.AccountID, at.ProjectID, tr, tagKey, tagValuePrefix, delimiter, deadline)
		if err != nil {
			sn.tagValueSuffixesErrors.Inc()
			err = fmt.Errorf("cannot get tag value suffixes for tr=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c from vmstorage %s: %w",
				tr.String(), tagKey, tagValuePrefix, delimiter, sn.connPool.Addr(), err)
		}
		return &nodeResult{
			suffixes: suffixes,
			err:      err,
		}
	})

	// Collect results
	m := make(map[string]struct{})
	isPartial, err := snr.collectResults(partialTagValueSuffixesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		for _, suffix := range nr.suffixes {
			m[suffix] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch tag value suffixes from vmstorage nodes: %w", err)
	}

	suffixes := make([]string, 0, len(m))
	for suffix := range m {
		suffixes = append(suffixes, suffix)
	}
	return suffixes, isPartial, nil
}

// GetLabelEntries returns all the label entries for at until the given deadline.
func GetLabelEntries(at *auth.Token, denyPartialResponse bool, deadline searchutils.Deadline) ([]storage.TagEntry, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelEntries []storage.TagEntry
		err          error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.labelEntriesRequests.Inc()
		labelEntries, err := sn.getLabelEntries(at.AccountID, at.ProjectID, deadline)
		if err != nil {
			sn.labelEntriesErrors.Inc()
			err = fmt.Errorf("cannot get label entries from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labelEntries: labelEntries,
			err:          err,
		}
	})

	// Collect results
	var labelEntries []storage.TagEntry
	isPartial, err := snr.collectResults(partialLabelEntriesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labelEntries = append(labelEntries, nr.labelEntries...)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot featch label etnries from vmstorage nodes: %w", err)
	}

	// Substitute "" with "__name__"
	for i := range labelEntries {
		e := &labelEntries[i]
		if e.Key == "" {
			e.Key = "__name__"
		}
	}

	// Deduplicate label entries
	labelEntries = deduplicateLabelEntries(labelEntries)

	// Sort labelEntries by the number of label values in each entry.
	sort.Slice(labelEntries, func(i, j int) bool {
		a, b := labelEntries[i].Values, labelEntries[j].Values
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return labelEntries[i].Key > labelEntries[j].Key
	})

	return labelEntries, isPartial, nil
}

func deduplicateLabelEntries(src []storage.TagEntry) []storage.TagEntry {
	m := make(map[string][]string, len(src))
	for i := range src {
		e := &src[i]
		m[e.Key] = append(m[e.Key], e.Values...)
	}
	dst := make([]storage.TagEntry, 0, len(m))
	for key, values := range m {
		values := deduplicateStrings(values)
		sort.Strings(values)
		dst = append(dst, storage.TagEntry{
			Key:    key,
			Values: values,
		})
	}
	return dst
}

func deduplicateStrings(a []string) []string {
	m := make(map[string]bool, len(a))
	for _, s := range a {
		m[s] = true
	}
	a = a[:0]
	for s := range m {
		a = append(a, s)
	}
	return a
}

// GetTSDBStatusForDate returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
func GetTSDBStatusForDate(at *auth.Token, denyPartialResponse bool, deadline searchutils.Deadline, date uint64, topN int) (*storage.TSDBStatus, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		status *storage.TSDBStatus
		err    error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.tsdbStatusRequests.Inc()
		status, err := sn.getTSDBStatusForDate(at.AccountID, at.ProjectID, date, topN, deadline)
		if err != nil {
			sn.tsdbStatusErrors.Inc()
			err = fmt.Errorf("cannot obtain tsdb status from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			status: status,
			err:    err,
		}
	})

	// Collect results.
	var statuses []*storage.TSDBStatus
	isPartial, err := snr.collectResults(partialTSDBStatusResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		statuses = append(statuses, nr.status)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch tsdb status from vmstorage nodes: %w", err)
	}

	status := mergeTSDBStatuses(statuses, topN)
	return status, isPartial, nil
}

func mergeTSDBStatuses(statuses []*storage.TSDBStatus, topN int) *storage.TSDBStatus {
	seriesCountByMetricName := make(map[string]uint64)
	labelValueCountByLabelName := make(map[string]uint64)
	seriesCountByLabelValuePair := make(map[string]uint64)
	for _, st := range statuses {
		for _, e := range st.SeriesCountByMetricName {
			seriesCountByMetricName[e.Name] += e.Count
		}
		for _, e := range st.LabelValueCountByLabelName {
			// Label values are copied among vmstorage nodes,
			// so select the maximum label values count.
			if e.Count > labelValueCountByLabelName[e.Name] {
				labelValueCountByLabelName[e.Name] = e.Count
			}
		}
		for _, e := range st.SeriesCountByLabelValuePair {
			seriesCountByLabelValuePair[e.Name] += e.Count
		}
	}
	return &storage.TSDBStatus{
		SeriesCountByMetricName:     toTopHeapEntries(seriesCountByMetricName, topN),
		LabelValueCountByLabelName:  toTopHeapEntries(labelValueCountByLabelName, topN),
		SeriesCountByLabelValuePair: toTopHeapEntries(seriesCountByLabelValuePair, topN),
	}
}

func toTopHeapEntries(m map[string]uint64, topN int) []storage.TopHeapEntry {
	a := make([]storage.TopHeapEntry, 0, len(m))
	for name, count := range m {
		a = append(a, storage.TopHeapEntry{
			Name:  name,
			Count: count,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].Count != a[j].Count {
			return a[i].Count > a[j].Count
		}
		return a[i].Name < a[j].Name
	})
	if len(a) > topN {
		a = a[:topN]
	}
	return a
}

// GetTSDBStatusWithFilters returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It accepts aribtrary filters on time series in sq.
func GetTSDBStatusWithFilters(at *auth.Token, denyPartialResponse bool, deadline searchutils.Deadline, sq *storage.SearchQuery, topN int) (*storage.TSDBStatus, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		status *storage.TSDBStatus
		err    error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.tsdbStatusWithFiltersRequests.Inc()
		status, err := sn.getTSDBStatusWithFilters(requestData, topN, deadline)
		if err != nil {
			sn.tsdbStatusWithFiltersErrors.Inc()
			err = fmt.Errorf("cannot obtain tsdb status with filters from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			status: status,
			err:    err,
		}
	})

	// Collect results.
	var statuses []*storage.TSDBStatus
	isPartial, err := snr.collectResults(partialTSDBStatusResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		statuses = append(statuses, nr.status)
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch tsdb status with filters from vmstorage nodes: %w", err)
	}

	status := mergeTSDBStatuses(statuses, topN)
	return status, isPartial, nil
}

// GetSeriesCount returns the number of unique series for the given at.
func GetSeriesCount(at *auth.Token, denyPartialResponse bool, deadline searchutils.Deadline) (uint64, bool, error) {
	if deadline.Exceeded() {
		return 0, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		n   uint64
		err error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.seriesCountRequests.Inc()
		n, err := sn.getSeriesCount(at.AccountID, at.ProjectID, deadline)
		if err != nil {
			sn.seriesCountErrors.Inc()
			err = fmt.Errorf("cannot get series count from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			n:   n,
			err: err,
		}
	})

	// Collect results
	var n uint64
	isPartial, err := snr.collectResults(partialSeriesCountResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		n += nr.n
		return nil
	})
	if err != nil {
		return 0, isPartial, fmt.Errorf("cannot fetch series count from vmstorage nodes: %w", err)
	}
	return n, isPartial, nil
}

type tmpBlocksFileWrapper struct {
	mu                 sync.Mutex
	tbf                *tmpBlocksFile
	m                  map[string][]tmpBlockAddr
	orderedMetricNames []string
}

func (tbfw *tmpBlocksFileWrapper) RegisterEmptyBlock(mb *storage.MetricBlock) {
	metricName := mb.MetricName
	tbfw.mu.Lock()
	if addrs := tbfw.m[string(metricName)]; addrs == nil {
		// An optimization for big number of time series with long names: store only a single copy of metricNameStr
		// in both tbfw.orderedMetricNames and tbfw.m.
		tbfw.orderedMetricNames = append(tbfw.orderedMetricNames, string(metricName))
		tbfw.m[tbfw.orderedMetricNames[len(tbfw.orderedMetricNames)-1]] = []tmpBlockAddr{{}}
	}
	tbfw.mu.Unlock()
}

func (tbfw *tmpBlocksFileWrapper) RegisterAndWriteBlock(mb *storage.MetricBlock) error {
	bb := tmpBufPool.Get()
	bb.B = storage.MarshalBlock(bb.B[:0], &mb.Block)
	tbfw.mu.Lock()
	addr, err := tbfw.tbf.WriteBlockData(bb.B)
	tmpBufPool.Put(bb)
	if err == nil {
		metricName := mb.MetricName
		addrs := tbfw.m[string(metricName)]
		addrs = append(addrs, addr)
		if len(addrs) > 1 {
			tbfw.m[string(metricName)] = addrs
		} else {
			// An optimization for big number of time series with long names: store only a single copy of metricNameStr
			// in both tbfw.orderedMetricNames and tbfw.m.
			tbfw.orderedMetricNames = append(tbfw.orderedMetricNames, string(metricName))
			tbfw.m[tbfw.orderedMetricNames[len(tbfw.orderedMetricNames)-1]] = addrs
		}
	}
	tbfw.mu.Unlock()
	return err
}

var metricNamePool = &sync.Pool{
	New: func() interface{} {
		return &storage.MetricName{}
	},
}

// ExportBlocks searches for time series matching sq and calls f for each found block.
//
// f is called in parallel from multiple goroutines.
// It is the responsibility of f to call b.UnmarshalData before reading timestamps and values from the block.
// It is the responsibility of f to filter blocks according to the given tr.
func ExportBlocks(at *auth.Token, sq *storage.SearchQuery, deadline searchutils.Deadline, f func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error) error {
	if deadline.Exceeded() {
		return fmt.Errorf("timeout exceeded before starting data export: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	var wg syncwg.WaitGroup
	var stopped uint32
	processBlock := func(mb *storage.MetricBlock) error {
		wg.Add(1)
		defer wg.Done()
		if atomic.LoadUint32(&stopped) != 0 {
			return nil
		}
		mn := metricNamePool.Get().(*storage.MetricName)
		if err := mn.Unmarshal(mb.MetricName); err != nil {
			return fmt.Errorf("cannot unmarshal metricName: %w", err)
		}
		if err := f(mn, &mb.Block, tr); err != nil {
			return err
		}
		mn.Reset()
		metricNamePool.Put(mn)
		return nil
	}
	_, err := processSearchQuery(at, true, sq, true, processBlock, deadline)

	// Make sure processBlock isn't called anymore in order to prevent from data races.
	atomic.StoreUint32(&stopped, 1)
	wg.Wait()

	if err != nil {
		return fmt.Errorf("error occured during export: %w", err)
	}
	return nil
}

// SearchMetricNames returns all the metric names matching sq until the given deadline.
func SearchMetricNames(at *auth.Token, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutils.Deadline) ([]storage.MetricName, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting to search metric names: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		metricNames [][]byte
		err         error
	}
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.searchMetricNamesRequests.Inc()
		metricNames, err := sn.processSearchMetricNames(requestData, deadline)
		if err != nil {
			sn.searchMetricNamesErrors.Inc()
			err = fmt.Errorf("cannot search metric names on vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			metricNames: metricNames,
			err:         err,
		}
	})

	// Collect results.
	metricNames := make(map[string]struct{})
	isPartial, err := snr.collectResults(partialSearchMetricNamesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		for _, metricName := range nr.metricNames {
			metricNames[string(metricName)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch metric names from vmstorage nodes: %w", err)
	}

	// Unmarshal metricNames
	mns := make([]storage.MetricName, len(metricNames))
	i := 0
	for metricName := range metricNames {
		mn := &mns[i]
		if err := mn.Unmarshal(bytesutil.ToUnsafeBytes(metricName)); err != nil {
			return nil, false, fmt.Errorf("cannot unmarshal metric name obtained from vmstorage: %w; metricName=%q", err, metricName)
		}
		i++
	}
	return mns, isPartial, nil
}

// ProcessSearchQuery performs sq until the given deadline.
//
// Results.RunParallel or Results.Cancel must be called on the returned Results.
func ProcessSearchQuery(at *auth.Token, denyPartialResponse bool, sq *storage.SearchQuery, fetchData bool, deadline searchutils.Deadline) (*Results, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tbfw := &tmpBlocksFileWrapper{
		tbf: getTmpBlocksFile(),
		m:   make(map[string][]tmpBlockAddr),
	}
	var wg syncwg.WaitGroup
	var stopped uint32
	var samples uint64
	processBlock := func(mb *storage.MetricBlock) error {
		wg.Add(1)
		defer wg.Done()
		if atomic.LoadUint32(&stopped) != 0 {
			return nil
		}
		if !fetchData {
			tbfw.RegisterEmptyBlock(mb)
			return nil
		}
		n := atomic.AddUint64(&samples, uint64(mb.Block.RowsCount()))
		if *maxSamplesPerQuery > 0 && n > uint64(*maxSamplesPerQuery) {
			return fmt.Errorf("cannot select more than -search.maxSamplesPerQuery=%d samples; possible solutions: to increase the -search.maxSamplesPerQuery; to reduce time range for the query; to use more specific label filters in order to select lower number of series", *maxSamplesPerQuery)
		}
		if err := tbfw.RegisterAndWriteBlock(mb); err != nil {
			return fmt.Errorf("cannot write MetricBlock to temporary blocks file: %w", err)
		}
		return nil
	}
	isPartial, err := processSearchQuery(at, denyPartialResponse, sq, fetchData, processBlock, deadline)

	// Make sure processBlock isn't called anymore in order to protect from data races.
	atomic.StoreUint32(&stopped, 1)
	wg.Wait()

	if err != nil {
		putTmpBlocksFile(tbfw.tbf)
		return nil, false, fmt.Errorf("error occured during search: %w", err)
	}
	if err := tbfw.tbf.Finalize(); err != nil {
		putTmpBlocksFile(tbfw.tbf)
		return nil, false, fmt.Errorf("cannot finalize temporary blocks file with %d time series: %w", len(tbfw.m), err)
	}

	var rss Results
	rss.at = at
	rss.tr = tr
	rss.fetchData = fetchData
	rss.deadline = deadline
	rss.tbf = tbfw.tbf
	pts := make([]packedTimeseries, len(tbfw.orderedMetricNames))
	for i, metricName := range tbfw.orderedMetricNames {
		pts[i] = packedTimeseries{
			metricName: metricName,
			addrs:      tbfw.m[metricName],
		}
	}
	rss.packedTimeseries = pts
	return &rss, isPartial, nil
}

func processSearchQuery(at *auth.Token, denyPartialResponse bool, sq *storage.SearchQuery, fetchData bool,
	processBlock func(mb *storage.MetricBlock) error, deadline searchutils.Deadline) (bool, error) {
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	snr := startStorageNodesRequest(denyPartialResponse, func(idx int, sn *storageNode) interface{} {
		sn.searchRequests.Inc()
		err := sn.processSearchQuery(requestData, fetchData, processBlock, deadline)
		if err != nil {
			sn.searchErrors.Inc()
			err = fmt.Errorf("cannot perform search on vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &err
	})

	// Collect results.
	isPartial, err := snr.collectResults(partialSearchResults, func(result interface{}) error {
		errP := result.(*error)
		return *errP
	})
	if err != nil {
		return isPartial, fmt.Errorf("cannot fetch query results from vmstorage nodes: %w", err)
	}
	return isPartial, nil
}

type storageNodesRequest struct {
	denyPartialResponse bool
	resultsCh           chan interface{}
}

func startStorageNodesRequest(denyPartialResponse bool, f func(idx int, sn *storageNode) interface{}) *storageNodesRequest {
	resultsCh := make(chan interface{}, len(storageNodes))
	for idx, sn := range storageNodes {
		go func(idx int, sn *storageNode) {
			result := f(idx, sn)
			resultsCh <- result
		}(idx, sn)
	}
	return &storageNodesRequest{
		denyPartialResponse: denyPartialResponse,
		resultsCh:           resultsCh,
	}
}

func (snr *storageNodesRequest) collectAllResults(f func(result interface{}) error) error {
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		result := <-snr.resultsCh
		if err := f(result); err != nil {
			errors = append(errors, err)
			continue
		}
	}
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

func (snr *storageNodesRequest) collectResults(partialResultsCounter *metrics.Counter, f func(result interface{}) error) (bool, error) {
	var errors []error
	resultsCollected := 0
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing the f function
		// passed to startStorageNodesRequest must be finished until the deadline.
		result := <-snr.resultsCh
		if err := f(result); err != nil {
			errors = append(errors, err)
			continue
		}
		resultsCollected++
		if resultsCollected > len(storageNodes)-*replicationFactor {
			// There is no need in waiting for the remaining results,
			// because the collected results contain all the data according to the given -replicationFactor.
			// This should speed up responses when a part of vmstorage nodes are slow and/or temporarily unavailable.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711
			//
			// It is expected that cap(snr.resultsCh) == len(storageNodes), otherwise goroutine leak is possible.
			return false, nil
		}
	}
	isPartial := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// All the vmstorage nodes returned error.
			// Return only the first error, since it has no sense in returning all errors.
			return false, errors[0]
		}

		// Return partial results.
		// This allows gracefully degrade vmselect in the case
		// if a part of storageNodes are temporarily unavailable.
		// Do not return the error, since it may spam logs on busy vmselect
		// serving high amounts of requests.
		partialResultsCounter.Inc()
		if snr.denyPartialResponse {
			return true, errors[0]
		}
		isPartial = true
	}
	return isPartial, nil
}

type storageNode struct {
	connPool *netutil.ConnPool

	// The number of concurrent queries to storageNode.
	concurrentQueries *metrics.Counter

	// The number of RegisterMetricNames requests to storageNode.
	registerMetricNamesRequests *metrics.Counter

	// The number of RegisterMetricNames request errors to storageNode.
	registerMetricNamesErrors *metrics.Counter

	// The number of DeleteSeries requests to storageNode.
	deleteSeriesRequests *metrics.Counter

	// The number of DeleteSeries request errors to storageNode.
	deleteSeriesErrors *metrics.Counter

	// The number of requests to labels.
	labelsOnTimeRangeRequests *metrics.Counter

	// The number of requests to labels.
	labelsRequests *metrics.Counter

	// The number of errors during requests to labels.
	labelsOnTimeRangeErrors *metrics.Counter

	// The number of errors during requests to labels.
	labelsErrors *metrics.Counter

	// The number of requests to labelValuesOnTimeRange.
	labelValuesOnTimeRangeRequests *metrics.Counter

	// The number of requests to labelValues.
	labelValuesRequests *metrics.Counter

	// The number of errors during requests to labelValuesOnTimeRange.
	labelValuesOnTimeRangeErrors *metrics.Counter

	// The number of errors during requests to labelValues.
	labelValuesErrors *metrics.Counter

	// The number of requests to labelEntries.
	labelEntriesRequests *metrics.Counter

	// The number of errors during requests to labelEntries.
	labelEntriesErrors *metrics.Counter

	// The number of requests to tagValueSuffixes.
	tagValueSuffixesRequests *metrics.Counter

	// The number of errors during requests to tagValueSuffixes.
	tagValueSuffixesErrors *metrics.Counter

	// The number of requests to tsdb status.
	tsdbStatusRequests *metrics.Counter

	// The number of errors during requests to tsdb status.
	tsdbStatusErrors *metrics.Counter

	// The number of requests to tsdb status.
	tsdbStatusWithFiltersRequests *metrics.Counter

	// The number of errors during requests to tsdb status.
	tsdbStatusWithFiltersErrors *metrics.Counter

	// The number of requests to seriesCount.
	seriesCountRequests *metrics.Counter

	// The number of errors during requests to seriesCount.
	seriesCountErrors *metrics.Counter

	// The number of 'search metric names' requests to storageNode.
	searchMetricNamesRequests *metrics.Counter

	// The number of search requests to storageNode.
	searchRequests *metrics.Counter

	// The number of 'search metric names' errors to storageNode.
	searchMetricNamesErrors *metrics.Counter

	// The number of search request errors to storageNode.
	searchErrors *metrics.Counter

	// The number of metric blocks read.
	metricBlocksRead *metrics.Counter

	// The number of read metric rows.
	metricRowsRead *metrics.Counter
}

func (sn *storageNode) registerMetricNames(mrs []storage.MetricRow, deadline searchutils.Deadline) error {
	if len(mrs) == 0 {
		return nil
	}
	f := func(bc *handshake.BufferedConn) error {
		return sn.registerMetricNamesOnConn(bc, mrs)
	}
	return sn.execOnConnWithPossibleRetry("registerMetricNames_v1", f, deadline)
}

func (sn *storageNode) deleteMetrics(requestData []byte, deadline searchutils.Deadline) (int, error) {
	var deletedCount int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.deleteMetricsOnConn(bc, requestData)
		if err != nil {
			return err
		}
		deletedCount = n
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("deleteMetrics_v3", f, deadline); err != nil {
		return 0, err
	}
	return deletedCount, nil
}

func (sn *storageNode) getLabelsOnTimeRange(accountID, projectID uint32, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	var labels []string
	f := func(bc *handshake.BufferedConn) error {
		ls, err := sn.getLabelsOnTimeRangeOnConn(bc, accountID, projectID, tr)
		if err != nil {
			return err
		}
		labels = ls
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("labelsOnTimeRange_v1", f, deadline); err != nil {
		return nil, err
	}
	return labels, nil
}

func (sn *storageNode) getLabels(accountID, projectID uint32, deadline searchutils.Deadline) ([]string, error) {
	var labels []string
	f := func(bc *handshake.BufferedConn) error {
		ls, err := sn.getLabelsOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		labels = ls
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("labels_v2", f, deadline); err != nil {
		return nil, err
	}
	return labels, nil
}

func (sn *storageNode) getLabelValuesOnTimeRange(accountID, projectID uint32, labelName string, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	var labelValues []string
	f := func(bc *handshake.BufferedConn) error {
		lvs, err := sn.getLabelValuesOnTimeRangeOnConn(bc, accountID, projectID, labelName, tr)
		if err != nil {
			return err
		}
		labelValues = lvs
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("labelValuesOnTimeRange_v1", f, deadline); err != nil {
		return nil, err
	}
	return labelValues, nil
}

func (sn *storageNode) getLabelValues(accountID, projectID uint32, labelName string, deadline searchutils.Deadline) ([]string, error) {
	var labelValues []string
	f := func(bc *handshake.BufferedConn) error {
		lvs, err := sn.getLabelValuesOnConn(bc, accountID, projectID, labelName)
		if err != nil {
			return err
		}
		labelValues = lvs
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("labelValues_v2", f, deadline); err != nil {
		return nil, err
	}
	return labelValues, nil
}

func (sn *storageNode) getTagValueSuffixes(accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, deadline searchutils.Deadline) ([]string, error) {
	var suffixes []string
	f := func(bc *handshake.BufferedConn) error {
		ss, err := sn.getTagValueSuffixesOnConn(bc, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter)
		if err != nil {
			return err
		}
		suffixes = ss
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("tagValueSuffixes_v1", f, deadline); err != nil {
		return nil, err
	}
	return suffixes, nil
}

func (sn *storageNode) getLabelEntries(accountID, projectID uint32, deadline searchutils.Deadline) ([]storage.TagEntry, error) {
	var tagEntries []storage.TagEntry
	f := func(bc *handshake.BufferedConn) error {
		tes, err := sn.getLabelEntriesOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		tagEntries = tes
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("labelEntries_v2", f, deadline); err != nil {
		return nil, err
	}
	return tagEntries, nil
}

func (sn *storageNode) getTSDBStatusForDate(accountID, projectID uint32, date uint64, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, error) {
	var status *storage.TSDBStatus
	f := func(bc *handshake.BufferedConn) error {
		st, err := sn.getTSDBStatusForDateOnConn(bc, accountID, projectID, date, topN)
		if err != nil {
			return err
		}
		status = st
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("tsdbStatus_v2", f, deadline); err != nil {
		return nil, err
	}
	return status, nil
}

func (sn *storageNode) getTSDBStatusWithFilters(requestData []byte, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, error) {
	var status *storage.TSDBStatus
	f := func(bc *handshake.BufferedConn) error {
		st, err := sn.getTSDBStatusWithFiltersOnConn(bc, requestData, topN)
		if err != nil {
			return err
		}
		status = st
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("tsdbStatusWithFilters_v1", f, deadline); err != nil {
		return nil, err
	}
	return status, nil
}

func (sn *storageNode) getSeriesCount(accountID, projectID uint32, deadline searchutils.Deadline) (uint64, error) {
	var n uint64
	f := func(bc *handshake.BufferedConn) error {
		nn, err := sn.getSeriesCountOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		n = nn
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("seriesCount_v2", f, deadline); err != nil {
		return 0, err
	}
	return n, nil
}

func (sn *storageNode) processSearchMetricNames(requestData []byte, deadline searchutils.Deadline) ([][]byte, error) {
	var metricNames [][]byte
	f := func(bc *handshake.BufferedConn) error {
		mns, err := sn.processSearchMetricNamesOnConn(bc, requestData)
		if err != nil {
			return err
		}
		metricNames = mns
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry("searchMetricNames_v1", f, deadline); err != nil {
		return nil, err
	}
	return metricNames, nil
}

func (sn *storageNode) processSearchQuery(requestData []byte, fetchData bool, processBlock func(mb *storage.MetricBlock) error, deadline searchutils.Deadline) error {
	f := func(bc *handshake.BufferedConn) error {
		if err := sn.processSearchQueryOnConn(bc, requestData, fetchData, processBlock); err != nil {
			return err
		}
		return nil
	}
	return sn.execOnConnWithPossibleRetry("search_v4", f, deadline)
}

func (sn *storageNode) execOnConnWithPossibleRetry(funcName string, f func(bc *handshake.BufferedConn) error, deadline searchutils.Deadline) error {
	err := sn.execOnConn(funcName, f, deadline)
	if err == nil {
		return nil
	}
	var er *errRemote
	var ne net.Error
	if errors.As(err, &er) || errors.As(err, &ne) && ne.Timeout() {
		// There is no sense in repeating the query on errors induced by vmstorage (errRemote) or on network timeout errors.
		return err
	}
	// Repeat the query in the hope the error was temporary.
	return sn.execOnConn(funcName, f, deadline)
}

func (sn *storageNode) execOnConn(rpcName string, f func(bc *handshake.BufferedConn) error, deadline searchutils.Deadline) error {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()

	d := time.Unix(int64(deadline.Deadline()), 0)
	nowSecs := fasttime.UnixTimestamp()
	currentTime := time.Unix(int64(nowSecs), 0)
	timeout := d.Sub(currentTime)
	if timeout <= 0 {
		return fmt.Errorf("request timeout reached: %s", deadline.String())
	}
	bc, err := sn.connPool.Get()
	if err != nil {
		return fmt.Errorf("cannot obtain connection from a pool: %w", err)
	}
	// Extend the connection deadline by 2 seconds, so the remote storage could return `timeout` error
	// without the need to break the connection.
	connDeadline := d.Add(2 * time.Second)
	if err := bc.SetDeadline(connDeadline); err != nil {
		_ = bc.Close()
		logger.Panicf("FATAL: cannot set connection deadline: %s", err)
	}
	if err := writeBytes(bc, []byte(rpcName)); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send rpcName=%q to the server: %w", rpcName, err)
	}

	// Send the remaining timeout instead of deadline to remote server, since it may have different time.
	timeoutSecs := uint32(timeout.Seconds() + 1)
	if err := writeUint32(bc, timeoutSecs); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send timeout=%d for rpcName=%q to the server: %w", timeout, rpcName, err)
	}

	if err := f(bc); err != nil {
		remoteAddr := bc.RemoteAddr()
		var er *errRemote
		if errors.As(err, &er) {
			// Remote error. The connection may be re-used. Return it to the pool.
			sn.connPool.Put(bc)
		} else {
			// Local error.
			// Close the connection instead of returning it to the pool,
			// since it may be broken.
			_ = bc.Close()
		}
		return fmt.Errorf("cannot execute rpcName=%q on vmstorage %q with timeout %s: %w", rpcName, remoteAddr, deadline.String(), err)
	}
	// Return the connection back to the pool, assuming it is healthy.
	sn.connPool.Put(bc)
	return nil
}

type errRemote struct {
	msg string
}

func (er *errRemote) Error() string {
	return er.msg
}

func newErrRemote(buf []byte) error {
	err := &errRemote{
		msg: string(buf),
	}
	if !strings.Contains(err.msg, "denyQueriesOutsideRetention") {
		return err
	}
	return &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: http.StatusServiceUnavailable,
	}
}

func (sn *storageNode) registerMetricNamesOnConn(bc *handshake.BufferedConn, mrs []storage.MetricRow) error {
	// Send the request to sn.
	if err := writeUint64(bc, uint64(len(mrs))); err != nil {
		return fmt.Errorf("cannot send metricsCount to conn: %w", err)
	}
	for i, mr := range mrs {
		if err := writeBytes(bc, mr.MetricNameRaw); err != nil {
			return fmt.Errorf("cannot send MetricNameRaw #%d to conn: %w", i+1, err)
		}
		if err := writeUint64(bc, uint64(mr.Timestamp)); err != nil {
			return fmt.Errorf("cannot send Timestamp #%d to conn: %w", i+1, err)
		}
	}
	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush registerMetricNames request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return newErrRemote(buf)
	}
	return nil
}

func (sn *storageNode) deleteMetricsOnConn(bc *handshake.BufferedConn, requestData []byte) (int, error) {
	// Send the request to sn
	if err := writeBytes(bc, requestData); err != nil {
		return 0, fmt.Errorf("cannot send deleteMetrics request to conn: %w", err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush deleteMetrics request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return 0, newErrRemote(buf)
	}

	// Read deletedCount
	deletedCount, err := readUint64(bc)
	if err != nil {
		return 0, fmt.Errorf("cannot read deletedCount value: %w", err)
	}
	return int(deletedCount), nil
}

const maxLabelSize = 16 * 1024 * 1024

func (sn *storageNode) getLabelsOnTimeRangeOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, tr storage.TimeRange) ([]string, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := writeTimeRange(bc, tr); err != nil {
		return nil, err
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	var labels []string
	for {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read labels: %w", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labels, nil
		}
		labels = append(labels, string(buf))
	}
}

func (sn *storageNode) getLabelsOnConn(bc *handshake.BufferedConn, accountID, projectID uint32) ([]string, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	var labels []string
	for {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read labels: %w", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labels, nil
		}
		labels = append(labels, string(buf))
	}
}

const maxLabelValueSize = 16 * 1024 * 1024

func (sn *storageNode) getLabelValuesOnTimeRangeOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, labelName string, tr storage.TimeRange) ([]string, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := writeBytes(bc, []byte(labelName)); err != nil {
		return nil, fmt.Errorf("cannot send labelName=%q to conn: %w", labelName, err)
	}
	if err := writeTimeRange(bc, tr); err != nil {
		return nil, err
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush labelName to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	labelValues, _, err := readLabelValues(buf, bc)
	if err != nil {
		return nil, err
	}
	return labelValues, nil
}

func (sn *storageNode) getLabelValuesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, labelName string) ([]string, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := writeBytes(bc, []byte(labelName)); err != nil {
		return nil, fmt.Errorf("cannot send labelName=%q to conn: %w", labelName, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush labelName to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	labelValues, _, err := readLabelValues(buf, bc)
	if err != nil {
		return nil, err
	}
	return labelValues, nil
}

func readLabelValues(buf []byte, bc *handshake.BufferedConn) ([]string, []byte, error) {
	var labelValues []string
	for {
		var err error
		buf, err = readBytes(buf[:0], bc, maxLabelValueSize)
		if err != nil {
			return nil, buf, fmt.Errorf("cannot read labelValue: %w", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labelValues, buf, nil
		}
		labelValues = append(labelValues, string(buf))
	}
}

func (sn *storageNode) getTagValueSuffixesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32,
	tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte) ([]string, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := writeTimeRange(bc, tr); err != nil {
		return nil, err
	}
	if err := writeBytes(bc, []byte(tagKey)); err != nil {
		return nil, fmt.Errorf("cannot send tagKey=%q to conn: %w", tagKey, err)
	}
	if err := writeBytes(bc, []byte(tagValuePrefix)); err != nil {
		return nil, fmt.Errorf("cannot send tagValuePrefix=%q to conn: %w", tagValuePrefix, err)
	}
	if err := writeByte(bc, delimiter); err != nil {
		return nil, fmt.Errorf("cannot send delimiter=%c to conn: %w", delimiter, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response.
	// The response may contain empty suffix, so it is prepended with the number of the following suffixes.
	suffixesCount, err := readUint64(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read the number of tag value suffixes: %w", err)
	}
	suffixes := make([]string, 0, suffixesCount)
	for i := 0; i < int(suffixesCount); i++ {
		buf, err = readBytes(buf[:0], bc, maxLabelValueSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read tag value suffix #%d: %w", i+1, err)
		}
		suffixes = append(suffixes, string(buf))
	}
	return suffixes, nil
}

func (sn *storageNode) getLabelEntriesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32) ([]storage.TagEntry, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response.
	var labelEntries []storage.TagEntry
	for {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read label: %w", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labelEntries, nil
		}
		label := string(buf)
		var values []string
		values, buf, err = readLabelValues(buf, bc)
		if err != nil {
			return nil, fmt.Errorf("cannot read values for label %q: %w", label, err)
		}
		labelEntries = append(labelEntries, storage.TagEntry{
			Key:    label,
			Values: values,
		})
	}
}

func (sn *storageNode) getTSDBStatusForDateOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, date uint64, topN int) (*storage.TSDBStatus, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return nil, err
	}
	// date shouldn't exceed 32 bits, so send it as uint32.
	if err := writeUint32(bc, uint32(date)); err != nil {
		return nil, fmt.Errorf("cannot send date=%d to conn: %w", date, err)
	}
	// topN shouldn't exceed 32 bits, so send it as uint32.
	if err := writeUint32(bc, uint32(topN)); err != nil {
		return nil, fmt.Errorf("cannot send topN=%d to conn: %w", topN, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush tsdbStatus args to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	seriesCountByMetricName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByMetricName: %w", err)
	}
	labelValueCountByLabelName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read labelValueCountByLabelName: %w", err)
	}
	seriesCountByLabelValuePair, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByLabelValuePair: %w", err)
	}
	status := &storage.TSDBStatus{
		SeriesCountByMetricName:     seriesCountByMetricName,
		LabelValueCountByLabelName:  labelValueCountByLabelName,
		SeriesCountByLabelValuePair: seriesCountByLabelValuePair,
	}
	return status, nil
}

func (sn *storageNode) getTSDBStatusWithFiltersOnConn(bc *handshake.BufferedConn, requestData []byte, topN int) (*storage.TSDBStatus, error) {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return nil, fmt.Errorf("cannot write requestData: %w", err)
	}
	// topN shouldn't exceed 32 bits, so send it as uint32.
	if err := writeUint32(bc, uint32(topN)); err != nil {
		return nil, fmt.Errorf("cannot send topN=%d to conn: %w", topN, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush tsdbStatusWithFilters args to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read response
	seriesCountByMetricName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByMetricName: %w", err)
	}
	labelValueCountByLabelName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read labelValueCountByLabelName: %w", err)
	}
	seriesCountByLabelValuePair, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByLabelValuePair: %w", err)
	}
	status := &storage.TSDBStatus{
		SeriesCountByMetricName:     seriesCountByMetricName,
		LabelValueCountByLabelName:  labelValueCountByLabelName,
		SeriesCountByLabelValuePair: seriesCountByLabelValuePair,
	}
	return status, nil
}

func readTopHeapEntries(bc *handshake.BufferedConn) ([]storage.TopHeapEntry, error) {
	n, err := readUint64(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read the number of topHeapEntries: %w", err)
	}
	var a []storage.TopHeapEntry
	var buf []byte
	for i := uint64(0); i < n; i++ {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read label name: %w", err)
		}
		count, err := readUint64(bc)
		if err != nil {
			return nil, fmt.Errorf("cannot read label count: %w", err)
		}
		a = append(a, storage.TopHeapEntry{
			Name:  string(buf),
			Count: count,
		})
	}
	return a, nil
}

func (sn *storageNode) getSeriesCountOnConn(bc *handshake.BufferedConn, accountID, projectID uint32) (uint64, error) {
	// Send the request to sn.
	if err := sendAccountIDProjectID(bc, accountID, projectID); err != nil {
		return 0, err
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush seriesCount args to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return 0, newErrRemote(buf)
	}

	// Read response
	n, err := readUint64(bc)
	if err != nil {
		return 0, fmt.Errorf("cannot read series count: %w", err)
	}
	return n, nil
}

// maxMetricBlockSize is the maximum size of serialized MetricBlock.
const maxMetricBlockSize = 1024 * 1024

// maxErrorMessageSize is the maximum size of error message received
// from vmstorage.
const maxErrorMessageSize = 64 * 1024

func (sn *storageNode) processSearchMetricNamesOnConn(bc *handshake.BufferedConn, requestData []byte) ([][]byte, error) {
	// Send the requst to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return nil, fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush requestData to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return nil, newErrRemote(buf)
	}

	// Read metricNames from response.
	metricNamesCount, err := readUint64(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read metricNamesCount: %w", err)
	}
	metricNames := make([][]byte, metricNamesCount)
	for i := int64(0); i < int64(metricNamesCount); i++ {
		buf, err = readBytes(buf[:0], bc, maxMetricNameSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read metricName #%d: %w", i+1, err)
		}
		metricNames[i] = append(metricNames[i][:0], buf...)
	}
	return metricNames, nil
}

const maxMetricNameSize = 64 * 1024

func (sn *storageNode) processSearchQueryOnConn(bc *handshake.BufferedConn, requestData []byte, fetchData bool, processBlock func(mb *storage.MetricBlock) error) error {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := writeBool(bc, fetchData); err != nil {
		return fmt.Errorf("cannot write fetchData=%v: %w", fetchData, err)
	}
	if err := bc.Flush(); err != nil {
		return fmt.Errorf("cannot flush requestData to conn: %w", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return newErrRemote(buf)
	}

	// Read response. It may consist of multiple MetricBlocks.
	blocksRead := 0
	var mb storage.MetricBlock
	for {
		buf, err = readBytes(buf[:0], bc, maxMetricBlockSize)
		if err != nil {
			return fmt.Errorf("cannot read MetricBlock #%d: %w", blocksRead, err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return nil
		}
		tail, err := mb.Unmarshal(buf)
		if err != nil {
			return fmt.Errorf("cannot unmarshal MetricBlock #%d: %w", blocksRead, err)
		}
		if len(tail) != 0 {
			return fmt.Errorf("non-empty tail after unmarshaling MetricBlock #%d: (len=%d) %q", blocksRead, len(tail), tail)
		}
		blocksRead++
		sn.metricBlocksRead.Inc()
		sn.metricRowsRead.Add(mb.Block.RowsCount())
		if err := processBlock(&mb); err != nil {
			return fmt.Errorf("cannot process MetricBlock #%d: %w", blocksRead, err)
		}
	}
}

func writeTimeRange(bc *handshake.BufferedConn, tr storage.TimeRange) error {
	if err := writeUint64(bc, uint64(tr.MinTimestamp)); err != nil {
		return fmt.Errorf("cannot send minTimestamp=%d to conn: %w", tr.MinTimestamp, err)
	}
	if err := writeUint64(bc, uint64(tr.MaxTimestamp)); err != nil {
		return fmt.Errorf("cannot send maxTimestamp=%d to conn: %w", tr.MaxTimestamp, err)
	}
	return nil
}

func writeBytes(bc *handshake.BufferedConn, buf []byte) error {
	sizeBuf := encoding.MarshalUint64(nil, uint64(len(buf)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return err
	}
	_, err := bc.Write(buf)
	return err
}

func writeUint32(bc *handshake.BufferedConn, n uint32) error {
	buf := encoding.MarshalUint32(nil, n)
	_, err := bc.Write(buf)
	return err
}

func writeUint64(bc *handshake.BufferedConn, n uint64) error {
	buf := encoding.MarshalUint64(nil, n)
	_, err := bc.Write(buf)
	return err
}

func writeBool(bc *handshake.BufferedConn, b bool) error {
	var buf [1]byte
	if b {
		buf[0] = 1
	}
	_, err := bc.Write(buf[:])
	return err
}

func writeByte(bc *handshake.BufferedConn, b byte) error {
	var buf [1]byte
	buf[0] = b
	_, err := bc.Write(buf[:])
	return err
}

func sendAccountIDProjectID(bc *handshake.BufferedConn, accountID, projectID uint32) error {
	if err := writeUint32(bc, accountID); err != nil {
		return fmt.Errorf("cannot send accountID=%d to conn: %w", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return fmt.Errorf("cannot send projectID=%d to conn: %w", projectID, err)
	}
	return nil
}

func readBytes(buf []byte, bc *handshake.BufferedConn, maxDataSize int) ([]byte, error) {
	buf = bytesutil.Resize(buf, 8)
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read %d bytes with data size: %w; read only %d bytes", len(buf), err, n)
	}
	dataSize := encoding.UnmarshalUint64(buf)
	if dataSize > uint64(maxDataSize) {
		return buf, fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxDataSize)
	}
	buf = bytesutil.Resize(buf, int(dataSize))
	if dataSize == 0 {
		return buf, nil
	}
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read data with size %d: %w; read only %d bytes", dataSize, err, n)
	}
	return buf, nil
}

func readUint64(bc *handshake.BufferedConn) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(bc, buf[:]); err != nil {
		return 0, fmt.Errorf("cannot read uint64: %w", err)
	}
	n := encoding.UnmarshalUint64(buf[:])
	return n, nil
}

var storageNodes []*storageNode

// InitStorageNodes initializes storage nodes' connections to the given addrs.
func InitStorageNodes(addrs []string) {
	if len(addrs) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}

	for _, addr := range addrs {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			// Automatically add missing port.
			addr += ":8401"
		}
		sn := &storageNode{
			// There is no need in requests compression, since they are usually very small.
			connPool: netutil.NewConnPool("vmselect", addr, handshake.VMSelectClient, 0),

			concurrentQueries: metrics.NewCounter(fmt.Sprintf(`vm_concurrent_queries{name="vmselect", addr=%q}`, addr)),

			registerMetricNamesRequests:    metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="registerMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			registerMetricNamesErrors:      metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="registerMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			deleteSeriesRequests:           metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			deleteSeriesErrors:             metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsOnTimeRangeRequests:      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelsOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsRequests:                 metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsOnTimeRangeErrors:        metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelsOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsErrors:                   metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesOnTimeRangeRequests: metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValuesOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesRequests:            metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesOnTimeRangeErrors:   metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValuesOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesErrors:              metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesRequests:           metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesErrors:             metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tagValueSuffixesRequests:       metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tagValueSuffixesErrors:         metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusRequests:             metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusErrors:               metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusWithFiltersRequests:  metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatusWithFilters", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusWithFiltersErrors:    metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatusWithFilters", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountRequests:            metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountErrors:              metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchMetricNamesRequests:      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="searchMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchRequests:                 metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchMetricNamesErrors:        metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="searchMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchErrors:                   metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			metricBlocksRead:               metrics.NewCounter(fmt.Sprintf(`vm_metric_blocks_read_total{name="vmselect", addr=%q}`, addr)),
			metricRowsRead:                 metrics.NewCounter(fmt.Sprintf(`vm_metric_rows_read_total{name="vmselect", addr=%q}`, addr)),
		}
		storageNodes = append(storageNodes, sn)
	}
}

// Stop gracefully stops netstorage.
func Stop() {
	// Nothing to do at the moment.
}

var (
	partialLabelsOnTimeRangeResults      = metrics.NewCounter(`vm_partial_results_total{type="labels_on_time_range", name="vmselect"}`)
	partialLabelsResults                 = metrics.NewCounter(`vm_partial_results_total{type="labels", name="vmselect"}`)
	partialLabelValuesOnTimeRangeResults = metrics.NewCounter(`vm_partial_results_total{type="label_values_on_time_range", name="vmselect"}`)
	partialLabelValuesResults            = metrics.NewCounter(`vm_partial_results_total{type="label_values", name="vmselect"}`)
	partialTagValueSuffixesResults       = metrics.NewCounter(`vm_partial_results_total{type="tag_value_suffixes", name="vmselect"}`)
	partialLabelEntriesResults           = metrics.NewCounter(`vm_partial_results_total{type="label_entries", name="vmselect"}`)
	partialTSDBStatusResults             = metrics.NewCounter(`vm_partial_results_total{type="tsdb_status", name="vmselect"}`)
	partialSeriesCountResults            = metrics.NewCounter(`vm_partial_results_total{type="series_count", name="vmselect"}`)
	partialSearchMetricNamesResults      = metrics.NewCounter(`vm_partial_results_total{type="search_metric_names", name="vmselect"}`)
	partialSearchResults                 = metrics.NewCounter(`vm_partial_results_total{type="search", name="vmselect"}`)
)

func applyGraphiteRegexpFilter(filter string, ss []string) ([]string, error) {
	// Anchor filter regexp to the beginning of the string as Graphite does.
	// See https://github.com/graphite-project/graphite-web/blob/3ad279df5cb90b211953e39161df416e54a84948/webapp/graphite/tags/localdatabase.py#L157
	filter = "^(?:" + filter + ")"
	re, err := regexp.Compile(filter)
	if err != nil {
		return nil, fmt.Errorf("cannot parse regexp filter=%q: %w", filter, err)
	}
	dst := ss[:0]
	for _, s := range ss {
		if re.MatchString(s) {
			dst = append(dst, s)
		}
	}
	return dst, nil
}
