package netstorage

import (
	"container/heap"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastrand"
)

var (
	maxTagKeysPerSearch          = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned from /api/v1/labels")
	maxTagValuesPerSearch        = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned from /api/v1/label/<label_name>/values")
	maxTagValueSuffixesPerSearch = flag.Int("search.maxTagValueSuffixesPerSearch", 100e3, "The maximum number of tag value suffixes returned from /metrics/find")
	maxSamplesPerSeries          = flag.Int("search.maxSamplesPerSeries", 30e6, "The maximum number of raw samples a single query can scan per each time series. This option allows limiting memory usage")
	maxSamplesPerQuery           = flag.Int("search.maxSamplesPerQuery", 1e9, "The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries")
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
	tr        storage.TimeRange
	fetchData bool
	deadline  searchutils.Deadline

	packedTimeseries []packedTimeseries
	sr               *storage.Search
	tbf              *tmpBlocksFile
}

// Len returns the number of results in rss.
func (rss *Results) Len() int {
	return len(rss.packedTimeseries)
}

// Cancel cancels rss work.
func (rss *Results) Cancel() {
	rss.mustClose()
}

func (rss *Results) mustClose() {
	putStorageSearch(rss.sr)
	rss.sr = nil
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
	if err := tsw.pts.Unpack(r, rss.tbf, rss.tr, rss.fetchData); err != nil {
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
func (rss *Results) RunParallel(qt *querytracer.Tracer, f func(rs *Result, workerID uint) error) error {
	qt = qt.NewChild("parallel process of fetched data")
	defer rss.mustClose()

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
	qt.Donef("series=%d, samples=%d", seriesProcessedTotal, rowsProcessedTotal)

	return firstErr
}

var perQueryRowsProcessed = metrics.NewHistogram(`vm_per_query_rows_processed_count`)
var perQuerySeriesProcessed = metrics.NewHistogram(`vm_per_query_series_processed_count`)

var gomaxprocs = cgroup.AvailableCPUs()

type packedTimeseries struct {
	metricName string
	brs        []blockRef
}

type unpackWorkItem struct {
	br blockRef
	tr storage.TimeRange
}

type unpackWork struct {
	tbf    *tmpBlocksFile
	ws     []unpackWorkItem
	sbs    []*sortBlock
	doneCh chan error
}

func (upw *unpackWork) reset() {
	upw.tbf = nil
	ws := upw.ws
	for i := range ws {
		w := &ws[i]
		w.br = blockRef{}
		w.tr = storage.TimeRange{}
	}
	upw.ws = upw.ws[:0]
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
		if err := sb.unpackFrom(tmpBlock, upw.tbf, w.br, w.tr); err != nil {
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
func (pts *packedTimeseries) Unpack(dst *Result, tbf *tmpBlocksFile, tr storage.TimeRange, fetchData bool) error {
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
	brsLen := len(pts.brs)
	workers := brsLen / unpackBatchSize
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
	upws := make([]*unpackWork, 0, 1+brsLen/unpackBatchSize)
	upw := getUnpackWork()
	upw.tbf = tbf
	for _, br := range pts.brs {
		if len(upw.ws) >= unpackBatchSize {
			scheduleUnpackWork(workChs, upw)
			upws = append(upws, upw)
			upw = getUnpackWork()
			upw.tbf = tbf
		}
		upw.ws = append(upw.ws, unpackWorkItem{
			br: br,
			tr: tr,
		})
	}
	scheduleUnpackWork(workChs, upw)
	upws = append(upws, upw)
	pts.brs = pts.brs[:0]

	// Wait until work is complete
	samples := 0
	sbs := make([]*sortBlock, 0, brsLen)
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

func (sb *sortBlock) unpackFrom(tmpBlock *storage.Block, tbf *tmpBlocksFile, br blockRef, tr storage.TimeRange) error {
	tmpBlock.Reset()
	brReal := tbf.MustReadBlockRefAt(br.partRef, br.addr)
	brReal.MustReadBlock(tmpBlock, true)
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

// DeleteSeries deletes time series matching the given tagFilterss.
func DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) (int, error) {
	qt = qt.NewChild("delete series: %s", sq)
	defer qt.Done()
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return 0, err
	}
	return vmstorage.DeleteMetrics(tfss)
}

// GetLabelNames returns label names matching the given sq until the given deadline.
func GetLabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get labels: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if maxLabelNames > *maxTagKeysPerSearch || maxLabelNames <= 0 {
		maxLabelNames = *maxTagKeysPerSearch
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	labels, err := vmstorage.SearchLabelNamesWithFiltersOnTimeRange(qt, tfss, tr, maxLabelNames, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during labels search on time range: %w", err)
	}
	// Sort labels like Prometheus does
	sort.Strings(labels)
	qt.Printf("sort %d labels", len(labels))
	return labels, nil
}

// GetGraphiteTags returns Graphite tags until the given deadline.
func GetGraphiteTags(qt *querytracer.Tracer, filter string, limit int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get graphite tags: filter=%s, limit=%d", filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	sq := storage.NewSearchQuery(0, 0, nil, 0)
	labels, err := GetLabelNames(qt, sq, 0, deadline)
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}
	if limit > 0 && limit < len(labels) {
		labels = labels[:limit]
	}
	return labels, nil
}

func hasString(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// GetLabelValues returns label values matching the given labelName and sq until the given deadline.
func GetLabelValues(qt *querytracer.Tracer, labelName string, sq *storage.SearchQuery, maxLabelValues int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get values for label %s: %s", labelName, sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if maxLabelValues > *maxTagValuesPerSearch || maxLabelValues <= 0 {
		maxLabelValues = *maxTagValuesPerSearch
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	labelValues, err := vmstorage.SearchLabelValuesWithFiltersOnTimeRange(qt, labelName, tfss, tr, maxLabelValues, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during label values search on time range for labelName=%q: %w", labelName, err)
	}
	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)
	qt.Printf("sort %d label values", len(labelValues))
	return labelValues, nil
}

// GetGraphiteTagValues returns tag values for the given tagName until the given deadline.
func GetGraphiteTagValues(qt *querytracer.Tracer, tagName, filter string, limit int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get graphite tag values for tagName=%s, filter=%s, limit=%d", tagName, filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if tagName == "name" {
		tagName = ""
	}
	sq := storage.NewSearchQuery(0, 0, nil, 0)
	tagValues, err := GetLabelValues(qt, tagName, sq, 0, deadline)
	if err != nil {
		return nil, err
	}
	if len(filter) > 0 {
		tagValues, err = applyGraphiteRegexpFilter(filter, tagValues)
		if err != nil {
			return nil, err
		}
	}
	if limit > 0 && limit < len(tagValues) {
		tagValues = tagValues[:limit]
	}
	return tagValues, nil
}

// GetTagValueSuffixes returns tag value suffixes for the given tagKey and the given tagValuePrefix.
//
// It can be used for implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func GetTagValueSuffixes(qt *querytracer.Tracer, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get tag value suffixes for tagKey=%s, tagValuePrefix=%s, timeRange=%s", tagKey, tagValuePrefix, &tr)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	suffixes, err := vmstorage.SearchTagValueSuffixes(tr, []byte(tagKey), []byte(tagValuePrefix), delimiter, *maxTagValueSuffixesPerSearch, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during search for suffixes for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s: %w",
			tagKey, tagValuePrefix, delimiter, tr.String(), err)
	}
	if len(suffixes) >= *maxTagValueSuffixesPerSearch {
		return nil, fmt.Errorf("more than -search.maxTagValueSuffixesPerSearch=%d tag value suffixes found for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s; "+
			"either narrow down the query or increase -search.maxTagValueSuffixesPerSearch command-line flag value",
			*maxTagValueSuffixesPerSearch, tagKey, tagValuePrefix, delimiter, tr.String())
	}
	return suffixes, nil
}

// GetTSDBStatusForDate returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
func GetTSDBStatusForDate(qt *querytracer.Tracer, deadline searchutils.Deadline, date uint64, topN, maxMetrics int) (*storage.TSDBStatus, error) {
	qt = qt.NewChild("get tsdb stats for date=%d, topN=%d", date, topN)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	status, err := vmstorage.GetTSDBStatusForDate(qt, date, topN, maxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during tsdb status request: %w", err)
	}
	return status, nil
}

// GetTSDBStatusWithFilters returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It accepts aribtrary filters on time series in sq.
func GetTSDBStatusWithFilters(qt *querytracer.Tracer, deadline searchutils.Deadline, sq *storage.SearchQuery, topN int) (*storage.TSDBStatus, error) {
	qt = qt.NewChild("get tsdb stats: %s, topN=%d", sq, topN)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(tr.MinTimestamp) / (3600 * 24 * 1000)
	status, err := vmstorage.GetTSDBStatusWithFiltersForDate(qt, tfss, date, topN, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during tsdb status with filters request: %w", err)
	}
	return status, nil
}

// GetSeriesCount returns the number of unique series.
func GetSeriesCount(qt *querytracer.Tracer, deadline searchutils.Deadline) (uint64, error) {
	qt = qt.NewChild("get series count")
	defer qt.Done()
	if deadline.Exceeded() {
		return 0, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	n, err := vmstorage.GetSeriesCount(deadline.Deadline())
	if err != nil {
		return 0, fmt.Errorf("error during series count request: %w", err)
	}
	return n, nil
}

func getStorageSearch() *storage.Search {
	v := ssPool.Get()
	if v == nil {
		return &storage.Search{}
	}
	return v.(*storage.Search)
}

func putStorageSearch(sr *storage.Search) {
	sr.MustClose()
	ssPool.Put(sr)
}

var ssPool sync.Pool

// ExportBlocks searches for time series matching sq and calls f for each found block.
//
// f is called in parallel from multiple goroutines.
// Data processing is immediately stopped if f returns non-nil error.
// It is the responsibility of f to call b.UnmarshalData before reading timestamps and values from the block.
// It is the responsibility of f to filter blocks according to the given tr.
func ExportBlocks(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline, f func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error) error {
	qt = qt.NewChild("export blocks: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return fmt.Errorf("timeout exceeded before starting data export: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return err
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return err
	}

	vmstorage.WG.Add(1)
	defer vmstorage.WG.Done()

	sr := getStorageSearch()
	defer putStorageSearch(sr)
	startTime := time.Now()
	sr.Init(qt, vmstorage.Storage, tfss, tr, sq.MaxMetrics, deadline.Deadline())
	indexSearchDuration.UpdateDuration(startTime)

	// Start workers that call f in parallel on available CPU cores.
	gomaxprocs := cgroup.AvailableCPUs()
	workCh := make(chan *exportWork, gomaxprocs*8)
	var (
		errGlobal     error
		errGlobalLock sync.Mutex
		mustStop      uint32
	)
	var wg sync.WaitGroup
	wg.Add(gomaxprocs)
	for i := 0; i < gomaxprocs; i++ {
		go func() {
			defer wg.Done()
			for xw := range workCh {
				if err := f(&xw.mn, &xw.b, tr); err != nil {
					errGlobalLock.Lock()
					if errGlobal != nil {
						errGlobal = err
						atomic.StoreUint32(&mustStop, 1)
					}
					errGlobalLock.Unlock()
				}
				xw.reset()
				exportWorkPool.Put(xw)
			}
		}()
	}

	// Feed workers with work
	blocksRead := 0
	samples := 0
	for sr.NextMetricBlock() {
		blocksRead++
		if deadline.Exceeded() {
			return fmt.Errorf("timeout exceeded while fetching data block #%d from storage: %s", blocksRead, deadline.String())
		}
		if atomic.LoadUint32(&mustStop) != 0 {
			break
		}
		xw := exportWorkPool.Get().(*exportWork)
		if err := xw.mn.Unmarshal(sr.MetricBlockRef.MetricName); err != nil {
			return fmt.Errorf("cannot unmarshal metricName for block #%d: %w", blocksRead, err)
		}
		br := sr.MetricBlockRef.BlockRef
		br.MustReadBlock(&xw.b, true)
		samples += br.RowsCount()
		workCh <- xw
	}
	close(workCh)

	// Wait for workers to finish.
	wg.Wait()
	qt.Printf("export blocks=%d, samples=%d", blocksRead, samples)

	// Check errors.
	err = sr.Error()
	if err == nil {
		err = errGlobal
	}
	if err != nil {
		if errors.Is(err, storage.ErrDeadlineExceeded) {
			return fmt.Errorf("timeout exceeded during the query: %s", deadline.String())
		}
		return fmt.Errorf("search error after reading %d data blocks: %w", blocksRead, err)
	}
	return nil
}

type exportWork struct {
	mn storage.MetricName
	b  storage.Block
}

func (xw *exportWork) reset() {
	xw.mn.Reset()
	xw.b.Reset()
}

var exportWorkPool = &sync.Pool{
	New: func() interface{} {
		return &exportWork{}
	},
}

// SearchMetricNames returns all the metric names matching sq until the given deadline.
func SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) ([]storage.MetricName, error) {
	qt = qt.NewChild("fetch metric names: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting to search metric names: %s", deadline.String())
	}

	// Setup search.
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return nil, err
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}

	mns, err := vmstorage.SearchMetricNames(qt, tfss, tr, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("cannot find metric names: %w", err)
	}
	return mns, nil
}

// ProcessSearchQuery performs sq until the given deadline.
//
// Results.RunParallel or Results.Cancel must be called on the returned Results.
func ProcessSearchQuery(qt *querytracer.Tracer, sq *storage.SearchQuery, fetchData bool, deadline searchutils.Deadline) (*Results, error) {
	qt = qt.NewChild("fetch matching series: %s, fetchData=%v", sq, fetchData)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}

	// Setup search.
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return nil, err
	}
	tfss, err := setupTfss(tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}

	vmstorage.WG.Add(1)
	defer vmstorage.WG.Done()

	sr := getStorageSearch()
	startTime := time.Now()
	maxSeriesCount := sr.Init(qt, vmstorage.Storage, tfss, tr, sq.MaxMetrics, deadline.Deadline())
	indexSearchDuration.UpdateDuration(startTime)
	m := make(map[string][]blockRef, maxSeriesCount)
	orderedMetricNames := make([]string, 0, maxSeriesCount)
	blocksRead := 0
	samples := 0
	tbf := getTmpBlocksFile()
	var buf []byte
	for sr.NextMetricBlock() {
		blocksRead++
		if deadline.Exceeded() {
			putTmpBlocksFile(tbf)
			putStorageSearch(sr)
			return nil, fmt.Errorf("timeout exceeded while fetching data block #%d from storage: %s", blocksRead, deadline.String())
		}
		br := sr.MetricBlockRef.BlockRef
		samples += br.RowsCount()
		if *maxSamplesPerQuery > 0 && samples > *maxSamplesPerQuery {
			putTmpBlocksFile(tbf)
			putStorageSearch(sr)
			return nil, fmt.Errorf("cannot select more than -search.maxSamplesPerQuery=%d samples; possible solutions: to increase the -search.maxSamplesPerQuery; to reduce time range for the query; to use more specific label filters in order to select lower number of series", *maxSamplesPerQuery)
		}
		buf = br.Marshal(buf[:0])
		addr, err := tbf.WriteBlockRefData(buf)
		if err != nil {
			putTmpBlocksFile(tbf)
			putStorageSearch(sr)
			return nil, fmt.Errorf("cannot write %d bytes to temporary file: %w", len(buf), err)
		}
		metricName := sr.MetricBlockRef.MetricName
		brs := m[string(metricName)]
		brs = append(brs, blockRef{
			partRef: br.PartRef(),
			addr:    addr,
		})
		if len(brs) > 1 {
			m[string(metricName)] = brs
		} else {
			// An optimization for big number of time series with long metricName values:
			// use only a single copy of metricName for both orderedMetricNames and m.
			orderedMetricNames = append(orderedMetricNames, string(metricName))
			m[orderedMetricNames[len(orderedMetricNames)-1]] = brs
		}
	}
	if err := sr.Error(); err != nil {
		putTmpBlocksFile(tbf)
		putStorageSearch(sr)
		if errors.Is(err, storage.ErrDeadlineExceeded) {
			return nil, fmt.Errorf("timeout exceeded during the query: %s", deadline.String())
		}
		return nil, fmt.Errorf("search error after reading %d data blocks: %w", blocksRead, err)
	}
	if err := tbf.Finalize(); err != nil {
		putTmpBlocksFile(tbf)
		putStorageSearch(sr)
		return nil, fmt.Errorf("cannot finalize temporary file: %w", err)
	}
	qt.Printf("fetch unique series=%d, blocks=%d, samples=%d, bytes=%d", len(m), blocksRead, samples, tbf.Len())

	var rss Results
	rss.tr = tr
	rss.fetchData = fetchData
	rss.deadline = deadline
	pts := make([]packedTimeseries, len(orderedMetricNames))
	for i, metricName := range orderedMetricNames {
		pts[i] = packedTimeseries{
			metricName: metricName,
			brs:        m[metricName],
		}
	}
	rss.packedTimeseries = pts
	rss.sr = sr
	rss.tbf = tbf
	return &rss, nil
}

var indexSearchDuration = metrics.NewHistogram(`vm_index_search_duration_seconds`)

type blockRef struct {
	partRef storage.PartRef
	addr    tmpBlockAddr
}

func setupTfss(tr storage.TimeRange, tagFilterss [][]storage.TagFilter, maxMetrics int, deadline searchutils.Deadline) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(tagFilterss))
	for _, tagFilters := range tagFilterss {
		tfs := storage.NewTagFilters()
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				paths, err := vmstorage.SearchGraphitePaths(tr, query, maxMetrics, deadline.Deadline())
				if err != nil {
					return nil, fmt.Errorf("error when searching for Graphite paths for query %q: %w", query, err)
				}
				if len(paths) >= maxMetrics {
					return nil, fmt.Errorf("more than %d time series match Graphite query %q; "+
						"either narrow down the query or increase the corresponding -search.max* command-line flag value", maxMetrics, query)
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
