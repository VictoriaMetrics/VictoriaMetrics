package netstorage

import (
	"container/heap"
	"errors"
	"flag"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	maxTagKeysPerSearch   = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned from /api/v1/labels")
	maxTagValuesPerSearch = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned from /api/v1/label/<label_name>/values")
	maxSamplesPerSeries   = flag.Int("search.maxSamplesPerSeries", 30e6, "The maximum number of raw samples a single query can scan per each time series. This option allows limiting memory usage")
	maxSamplesPerQuery    = flag.Int("search.maxSamplesPerQuery", 1e9, "The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries")
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
	tr       storage.TimeRange
	deadline searchutils.Deadline

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
	err      error

	rowsProcessed int
}

func (tsw *timeseriesWork) reset() {
	tsw.mustStop = nil
	tsw.rss = nil
	tsw.pts = nil
	tsw.f = nil
	tsw.err = nil
	tsw.rowsProcessed = 0
}

func getTimeseriesWork() *timeseriesWork {
	v := tswPool.Get()
	if v == nil {
		v = &timeseriesWork{}
	}
	return v.(*timeseriesWork)
}

func putTimeseriesWork(tsw *timeseriesWork) {
	tsw.reset()
	tswPool.Put(tsw)
}

var tswPool sync.Pool

func (tsw *timeseriesWork) do(r *Result, workerID uint) error {
	if atomic.LoadUint32(tsw.mustStop) != 0 {
		return nil
	}
	rss := tsw.rss
	if rss.deadline.Exceeded() {
		atomic.StoreUint32(tsw.mustStop, 1)
		return fmt.Errorf("timeout exceeded during query execution: %s", rss.deadline.String())
	}
	if err := tsw.pts.Unpack(r, rss.tbf, rss.tr); err != nil {
		atomic.StoreUint32(tsw.mustStop, 1)
		return fmt.Errorf("error during time series unpacking: %w", err)
	}
	tsw.rowsProcessed = len(r.Timestamps)
	if len(r.Timestamps) > 0 {
		if err := tsw.f(r, workerID); err != nil {
			atomic.StoreUint32(tsw.mustStop, 1)
			return err
		}
	}
	return nil
}

func timeseriesWorker(qt *querytracer.Tracer, workChs []chan *timeseriesWork, workerID uint) {
	tmpResult := getTmpResult()

	// Perform own work at first.
	rowsProcessed := 0
	seriesProcessed := 0
	ch := workChs[workerID]
	for tsw := range ch {
		tsw.err = tsw.do(&tmpResult.rs, workerID)
		rowsProcessed += tsw.rowsProcessed
		seriesProcessed++
	}
	qt.Printf("own work processed: series=%d, samples=%d", seriesProcessed, rowsProcessed)

	// Then help others with the remaining work.
	rowsProcessed = 0
	seriesProcessed = 0
	for i := uint(1); i < uint(len(workChs)); i++ {
		idx := (i + workerID) % uint(len(workChs))
		ch := workChs[idx]
		for len(ch) > 0 {
			// It is expected that every channel in the workChs is already closed,
			// so the next line should return immediately.
			tsw, ok := <-ch
			if !ok {
				break
			}
			tsw.err = tsw.do(&tmpResult.rs, workerID)
			rowsProcessed += tsw.rowsProcessed
			seriesProcessed++
		}
	}
	qt.Printf("others work processed: series=%d, samples=%d", seriesProcessed, rowsProcessed)

	putTmpResult(tmpResult)
}

func getTmpResult() *result {
	v := resultPool.Get()
	if v == nil {
		v = &result{}
	}
	return v.(*result)
}

func putTmpResult(r *result) {
	currentTime := fasttime.UnixTimestamp()
	if cap(r.rs.Values) > 1024*1024 && 4*len(r.rs.Values) < cap(r.rs.Values) && currentTime-r.lastResetTime > 10 {
		// Reset r.rs in order to preserve memory usage after processing big time series with millions of rows.
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

// MaxWorkers returns the maximum number of workers netstorage can spin when calling RunParallel()
func MaxWorkers() int {
	return gomaxprocs
}

var gomaxprocs = cgroup.AvailableCPUs()

// RunParallel runs f in parallel for all the results from rss.
//
// f shouldn't hold references to rs after returning.
// workerID is the id of the worker goroutine that calls f. The workerID is in the range [0..MaxWorkers()-1].
// Data processing is immediately stopped if f returns non-nil error.
//
// rss becomes unusable after the call to RunParallel.
func (rss *Results) RunParallel(qt *querytracer.Tracer, f func(rs *Result, workerID uint) error) error {
	qt = qt.NewChild("parallel process of fetched data")
	defer rss.mustClose()

	rowsProcessedTotal, err := rss.runParallel(qt, f)
	seriesProcessedTotal := len(rss.packedTimeseries)
	rss.packedTimeseries = rss.packedTimeseries[:0]

	rowsReadPerQuery.Update(float64(rowsProcessedTotal))
	seriesReadPerQuery.Update(float64(seriesProcessedTotal))

	qt.Donef("series=%d, samples=%d", seriesProcessedTotal, rowsProcessedTotal)

	return err
}

func (rss *Results) runParallel(qt *querytracer.Tracer, f func(rs *Result, workerID uint) error) (int, error) {
	tswsLen := len(rss.packedTimeseries)
	if tswsLen == 0 {
		// Nothing to process
		return 0, nil
	}

	var mustStop uint32
	initTimeseriesWork := func(tsw *timeseriesWork, pts *packedTimeseries) {
		tsw.rss = rss
		tsw.pts = pts
		tsw.f = f
		tsw.mustStop = &mustStop
	}
	maxWorkers := MaxWorkers()
	if maxWorkers == 1 || tswsLen == 1 {
		// It is faster to process time series in the current goroutine.
		tsw := getTimeseriesWork()
		tmpResult := getTmpResult()
		rowsProcessedTotal := 0
		var err error
		for i := range rss.packedTimeseries {
			initTimeseriesWork(tsw, &rss.packedTimeseries[i])
			err = tsw.do(&tmpResult.rs, 0)
			rowsReadPerSeries.Update(float64(tsw.rowsProcessed))
			rowsProcessedTotal += tsw.rowsProcessed
			if err != nil {
				break
			}
			tsw.reset()
		}
		putTmpResult(tmpResult)
		putTimeseriesWork(tsw)

		return rowsProcessedTotal, err
	}

	// Slow path - spin up multiple local workers for parallel data processing.
	// Do not use global workers pool, since it increases inter-CPU memory ping-poing,
	// which reduces the scalability on systems with many CPU cores.

	// Prepare the work for workers.
	tsws := make([]*timeseriesWork, len(rss.packedTimeseries))
	for i := range rss.packedTimeseries {
		tsw := getTimeseriesWork()
		initTimeseriesWork(tsw, &rss.packedTimeseries[i])
		tsws[i] = tsw
	}

	// Prepare worker channels.
	workers := len(tsws)
	if workers > maxWorkers {
		workers = maxWorkers
	}
	itemsPerWorker := (len(tsws) + workers - 1) / workers
	workChs := make([]chan *timeseriesWork, workers)
	for i := range workChs {
		workChs[i] = make(chan *timeseriesWork, itemsPerWorker)
	}

	// Spread work among workers.
	for i, tsw := range tsws {
		idx := i % len(workChs)
		workChs[idx] <- tsw
	}
	// Mark worker channels as closed.
	for _, workCh := range workChs {
		close(workCh)
	}

	// Start workers and wait until they finish the work.
	var wg sync.WaitGroup
	for i := range workChs {
		wg.Add(1)
		qtChild := qt.NewChild("worker #%d", i)
		go func(workerID uint) {
			timeseriesWorker(qtChild, workChs, workerID)
			qtChild.Done()
			wg.Done()
		}(uint(i))
	}
	wg.Wait()

	// Collect results.
	var firstErr error
	rowsProcessedTotal := 0
	for _, tsw := range tsws {
		if tsw.err != nil && firstErr == nil {
			// Return just the first error, since other errors are likely duplicate the first error.
			firstErr = tsw.err
		}
		rowsReadPerSeries.Update(float64(tsw.rowsProcessed))
		rowsProcessedTotal += tsw.rowsProcessed
		putTimeseriesWork(tsw)
	}
	return rowsProcessedTotal, firstErr
}

var (
	rowsReadPerSeries  = metrics.NewHistogram(`vm_rows_read_per_series`)
	rowsReadPerQuery   = metrics.NewHistogram(`vm_rows_read_per_query`)
	seriesReadPerQuery = metrics.NewHistogram(`vm_series_read_per_query`)
)

type packedTimeseries struct {
	metricName string
	brs        []blockRef
}

type unpackWork struct {
	tbf *tmpBlocksFile
	br  blockRef
	tr  storage.TimeRange
	sb  *sortBlock
	err error
}

func (upw *unpackWork) reset() {
	upw.tbf = nil
	upw.br = blockRef{}
	upw.tr = storage.TimeRange{}
	upw.sb = nil
	upw.err = nil
}

func (upw *unpackWork) unpack(tmpBlock *storage.Block) {
	sb := getSortBlock()
	if err := sb.unpackFrom(tmpBlock, upw.tbf, upw.br, upw.tr); err != nil {
		putSortBlock(sb)
		upw.err = fmt.Errorf("cannot unpack block: %w", err)
		return
	}
	upw.sb = sb
}

func getUnpackWork() *unpackWork {
	v := unpackWorkPool.Get()
	if v != nil {
		return v.(*unpackWork)
	}
	return &unpackWork{}
}

func putUnpackWork(upw *unpackWork) {
	upw.reset()
	unpackWorkPool.Put(upw)
}

var unpackWorkPool sync.Pool

func unpackWorker(workChs []chan *unpackWork, workerID uint) {
	tmpBlock := getTmpStorageBlock()

	// Deal with own work at first.
	ch := workChs[workerID]
	for upw := range ch {
		upw.unpack(tmpBlock)
	}

	// Then help others with their work.
	for i := uint(1); i < uint(len(workChs)); i++ {
		idx := (i + workerID) % uint(len(workChs))
		ch := workChs[idx]
		for len(ch) > 0 {
			// Give a chance other goroutines to perform their work
			runtime.Gosched()
			// It is expected that every channel in the workChs is already closed,
			// so the next line should return immediately.
			upw, ok := <-ch
			if !ok {
				break
			}
			upw.unpack(tmpBlock)
		}
	}

	putTmpStorageBlock(tmpBlock)
}

func getTmpStorageBlock() *storage.Block {
	v := tmpStorageBlockPool.Get()
	if v == nil {
		v = &storage.Block{}
	}
	return v.(*storage.Block)
}

func putTmpStorageBlock(sb *storage.Block) {
	tmpStorageBlockPool.Put(sb)
}

var tmpStorageBlockPool sync.Pool

// Unpack unpacks pts to dst.
func (pts *packedTimeseries) Unpack(dst *Result, tbf *tmpBlocksFile, tr storage.TimeRange) error {
	dst.reset()
	if err := dst.MetricName.Unmarshal(bytesutil.ToUnsafeBytes(pts.metricName)); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %w", pts.metricName, err)
	}
	sbh := getSortBlocksHeap()
	var err error
	sbh.sbs, err = pts.unpackTo(sbh.sbs[:0], tbf, tr)
	pts.brs = pts.brs[:0]
	if err != nil {
		putSortBlocksHeap(sbh)
		return err
	}
	dedupInterval := storage.GetDedupInterval()
	mergeSortBlocks(dst, sbh, dedupInterval)
	putSortBlocksHeap(sbh)
	return nil
}

func (pts *packedTimeseries) unpackTo(dst []*sortBlock, tbf *tmpBlocksFile, tr storage.TimeRange) ([]*sortBlock, error) {
	upwsLen := len(pts.brs)
	if upwsLen == 0 {
		// Nothing to do
		return nil, nil
	}
	initUnpackWork := func(upw *unpackWork, br blockRef) {
		upw.tbf = tbf
		upw.br = br
		upw.tr = tr
	}
	if gomaxprocs == 1 || upwsLen <= 1000 {
		// It is faster to unpack all the data in the current goroutine.
		upw := getUnpackWork()
		samples := 0
		tmpBlock := getTmpStorageBlock()
		var err error
		for _, br := range pts.brs {
			initUnpackWork(upw, br)
			upw.unpack(tmpBlock)
			if upw.err != nil {
				return dst, upw.err
			}
			samples += len(upw.sb.Timestamps)
			if *maxSamplesPerSeries > 0 && samples > *maxSamplesPerSeries {
				putSortBlock(upw.sb)
				err = fmt.Errorf("cannot process more than %d samples per series; either increase -search.maxSamplesPerSeries "+
					"or reduce time range for the query", *maxSamplesPerSeries)
				break
			}
			dst = append(dst, upw.sb)
			upw.reset()
		}
		putTmpStorageBlock(tmpBlock)
		putUnpackWork(upw)

		return dst, err
	}

	// Slow path - spin up multiple local workers for parallel data unpacking.
	// Do not use global workers pool, since it increases inter-CPU memory ping-poing,
	// which reduces the scalability on systems with many CPU cores.

	// Prepare the work for workers.
	upws := make([]*unpackWork, upwsLen)
	for i, br := range pts.brs {
		upw := getUnpackWork()
		initUnpackWork(upw, br)
		upws[i] = upw
	}

	// Prepare worker channels.
	workers := len(upws)
	if workers > gomaxprocs {
		workers = gomaxprocs
	}
	if workers < 1 {
		workers = 1
	}
	itemsPerWorker := (len(upws) + workers - 1) / workers
	workChs := make([]chan *unpackWork, workers)
	for i := range workChs {
		workChs[i] = make(chan *unpackWork, itemsPerWorker)
	}

	// Spread work among worker channels.
	for i, upw := range upws {
		idx := i % len(workChs)
		workChs[idx] <- upw
	}
	// Mark worker channels as closed.
	for _, workCh := range workChs {
		close(workCh)
	}

	// Start workers and wait until they finish the work.
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID uint) {
			unpackWorker(workChs, workerID)
			wg.Done()
		}(uint(i))
	}
	wg.Wait()

	// Collect results.
	samples := 0
	var firstErr error
	for _, upw := range upws {
		if upw.err != nil && firstErr == nil {
			// Return the first error only, since other errors are likely the same.
			firstErr = upw.err
		}
		if firstErr == nil {
			sb := upw.sb
			samples += len(sb.Timestamps)
			if *maxSamplesPerSeries > 0 && samples > *maxSamplesPerSeries {
				putSortBlock(sb)
				firstErr = fmt.Errorf("cannot process more than %d samples per series; either increase -search.maxSamplesPerSeries "+
					"or reduce time range for the query", *maxSamplesPerSeries)
			} else {
				dst = append(dst, sb)
			}
		} else {
			putSortBlock(upw.sb)
		}
		putUnpackWork(upw)
	}

	return dst, firstErr
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

func mergeSortBlocks(dst *Result, sbh *sortBlocksHeap, dedupInterval int64) {
	// Skip empty sort blocks, since they cannot be passed to heap.Init.
	sbs := sbh.sbs[:0]
	for _, sb := range sbh.sbs {
		if len(sb.Timestamps) == 0 {
			putSortBlock(sb)
			continue
		}
		sbs = append(sbs, sb)
	}
	sbh.sbs = sbs
	if sbh.Len() == 0 {
		return
	}
	heap.Init(sbh)
	for {
		sbs := sbh.sbs
		top := sbs[0]
		if len(sbs) == 1 {
			dst.Timestamps = append(dst.Timestamps, top.Timestamps[top.NextIdx:]...)
			dst.Values = append(dst.Values, top.Values[top.NextIdx:]...)
			putSortBlock(top)
			break
		}
		sbNext := sbh.getNextBlock()
		tsNext := sbNext.Timestamps[sbNext.NextIdx]
		topNextIdx := top.NextIdx
		if n := equalSamplesPrefix(top, sbNext); n > 0 && dedupInterval > 0 {
			// Skip n replicated samples at top if deduplication is enabled.
			top.NextIdx = topNextIdx + n
		} else {
			// Copy samples from top to dst with timestamps not exceeding tsNext.
			top.NextIdx = topNextIdx + binarySearchTimestamps(top.Timestamps[topNextIdx:], tsNext)
			dst.Timestamps = append(dst.Timestamps, top.Timestamps[topNextIdx:top.NextIdx]...)
			dst.Values = append(dst.Values, top.Values[topNextIdx:top.NextIdx]...)
		}
		if top.NextIdx < len(top.Timestamps) {
			heap.Fix(sbh, 0)
		} else {
			heap.Pop(sbh)
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

func equalSamplesPrefix(a, b *sortBlock) int {
	n := equalTimestampsPrefix(a.Timestamps[a.NextIdx:], b.Timestamps[b.NextIdx:])
	if n == 0 {
		return 0
	}
	return equalValuesPrefix(a.Values[a.NextIdx:a.NextIdx+n], b.Values[b.NextIdx:b.NextIdx+n])
}

func equalTimestampsPrefix(a, b []int64) int {
	for i, v := range a {
		if i >= len(b) || v != b[i] {
			return i
		}
	}
	return len(a)
}

func equalValuesPrefix(a, b []float64) int {
	for i, v := range a {
		if i >= len(b) || v != b[i] {
			return i
		}
	}
	return len(a)
}

func binarySearchTimestamps(timestamps []int64, ts int64) int {
	// The code has been adapted from sort.Search.
	n := len(timestamps)
	if n > 0 && timestamps[n-1] <= ts {
		// Fast path for timestamps scanned in ascending order.
		return n
	}
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1)
		if h >= 0 && h < len(timestamps) && timestamps[h] <= ts {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

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
	brReal.MustReadBlock(tmpBlock)
	if err := tmpBlock.UnmarshalData(); err != nil {
		return fmt.Errorf("cannot unmarshal block: %w", err)
	}
	sb.Timestamps, sb.Values = tmpBlock.AppendRowsWithTimeRangeFilter(sb.Timestamps[:0], sb.Values[:0], tr)
	skippedRows := tmpBlock.RowsCount() - len(sb.Timestamps)
	metricRowsSkipped.Add(skippedRows)
	return nil
}

type sortBlocksHeap struct {
	sbs []*sortBlock
}

func (sbh *sortBlocksHeap) getNextBlock() *sortBlock {
	sbs := sbh.sbs
	if len(sbs) < 2 {
		return nil
	}
	if len(sbs) < 3 {
		return sbs[1]
	}
	a := sbs[1]
	b := sbs[2]
	if a.Timestamps[a.NextIdx] <= b.Timestamps[b.NextIdx] {
		return a
	}
	return b
}

func (sbh *sortBlocksHeap) Len() int {
	return len(sbh.sbs)
}

func (sbh *sortBlocksHeap) Less(i, j int) bool {
	sbs := sbh.sbs
	a := sbs[i]
	b := sbs[j]
	return a.Timestamps[a.NextIdx] < b.Timestamps[b.NextIdx]
}

func (sbh *sortBlocksHeap) Swap(i, j int) {
	sbs := sbh.sbs
	sbs[i], sbs[j] = sbs[j], sbs[i]
}

func (sbh *sortBlocksHeap) Push(x interface{}) {
	sbh.sbs = append(sbh.sbs, x.(*sortBlock))
}

func (sbh *sortBlocksHeap) Pop() interface{} {
	sbs := sbh.sbs
	v := sbs[len(sbs)-1]
	sbs[len(sbs)-1] = nil
	sbh.sbs = sbs[:len(sbs)-1]
	return v
}

func getSortBlocksHeap() *sortBlocksHeap {
	v := sbhPool.Get()
	if v == nil {
		return &sortBlocksHeap{}
	}
	return v.(*sortBlocksHeap)
}

func putSortBlocksHeap(sbh *sortBlocksHeap) {
	sbs := sbh.sbs
	for i := range sbs {
		sbs[i] = nil
	}
	sbh.sbs = sbs[:0]
	sbhPool.Put(sbh)
}

var sbhPool sync.Pool

// DeleteSeries deletes time series matching the given tagFilterss.
func DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) (int, error) {
	qt = qt.NewChild("delete series: %s", sq)
	defer qt.Done()
	tr := sq.GetTimeRange()
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return 0, err
	}
	return vmstorage.DeleteSeries(qt, tfss)
}

// LabelNames returns label names matching the given sq until the given deadline.
func LabelNames(qt *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get labels: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if maxLabelNames > *maxTagKeysPerSearch || maxLabelNames <= 0 {
		maxLabelNames = *maxTagKeysPerSearch
	}
	tr := sq.GetTimeRange()
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
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

// GraphiteTags returns Graphite tags until the given deadline.
func GraphiteTags(qt *querytracer.Tracer, filter string, limit int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get graphite tags: filter=%s, limit=%d", filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	sq := storage.NewSearchQuery(0, 0, nil, 0)
	labels, err := LabelNames(qt, sq, 0, deadline)
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

// LabelValues returns label values matching the given labelName and sq until the given deadline.
func LabelValues(qt *querytracer.Tracer, labelName string, sq *storage.SearchQuery, maxLabelValues int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get values for label %s: %s", labelName, sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if maxLabelValues > *maxTagValuesPerSearch || maxLabelValues <= 0 {
		maxLabelValues = *maxTagValuesPerSearch
	}
	tr := sq.GetTimeRange()
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
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

// GraphiteTagValues returns tag values for the given tagName until the given deadline.
func GraphiteTagValues(qt *querytracer.Tracer, tagName, filter string, limit int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get graphite tag values for tagName=%s, filter=%s, limit=%d", tagName, filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if tagName == "name" {
		tagName = ""
	}
	sq := storage.NewSearchQuery(0, 0, nil, 0)
	tagValues, err := LabelValues(qt, tagName, sq, 0, deadline)
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

// TagValueSuffixes returns tag value suffixes for the given tagKey and the given tagValuePrefix.
//
// It can be used for implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func TagValueSuffixes(qt *querytracer.Tracer, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get tag value suffixes for tagKey=%s, tagValuePrefix=%s, maxSuffixes=%d, timeRange=%s", tagKey, tagValuePrefix, maxSuffixes, &tr)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	suffixes, err := vmstorage.SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during search for suffixes for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s: %w",
			tagKey, tagValuePrefix, delimiter, tr.String(), err)
	}
	if len(suffixes) >= maxSuffixes {
		return nil, fmt.Errorf("more than -search.maxTagValueSuffixesPerSearch=%d tag value suffixes found for tagKey=%q, tagValuePrefix=%q, delimiter=%c on time range %s; "+
			"either narrow down the query or increase -search.maxTagValueSuffixesPerSearch command-line flag value",
			maxSuffixes, tagKey, tagValuePrefix, delimiter, tr.String())
	}
	return suffixes, nil
}

// TSDBStatus returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It accepts arbitrary filters on time series in sq.
func TSDBStatus(qt *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, error) {
	qt = qt.NewChild("get tsdb stats: %s, focusLabel=%q, topN=%d", sq, focusLabel, topN)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	tr := sq.GetTimeRange()
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	date := uint64(tr.MinTimestamp) / (3600 * 24 * 1000)
	status, err := vmstorage.GetTSDBStatus(qt, tfss, date, focusLabel, topN, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("error during tsdb status request: %w", err)
	}
	return status, nil
}

// SeriesCount returns the number of unique series.
func SeriesCount(qt *querytracer.Tracer, deadline searchutils.Deadline) (uint64, error) {
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
func ExportBlocks(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline,
	f func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange, workerID uint) error) error {
	qt = qt.NewChild("export blocks: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return fmt.Errorf("timeout exceeded before starting data export: %s", deadline.String())
	}
	tr := sq.GetTimeRange()
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return err
	}
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
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
	workCh := make(chan *exportWork, gomaxprocs*8)
	var (
		errGlobal     error
		errGlobalLock sync.Mutex
		mustStop      uint32
	)
	var wg sync.WaitGroup
	wg.Add(gomaxprocs)
	for i := 0; i < gomaxprocs; i++ {
		go func(workerID uint) {
			defer wg.Done()
			for xw := range workCh {
				if err := f(&xw.mn, &xw.b, tr, workerID); err != nil {
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
		}(uint(i))
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
		br.MustReadBlock(&xw.b)
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
//
// The returned metric names must be unmarshaled via storage.MetricName.UnmarshalString().
func SearchMetricNames(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("fetch metric names: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting to search metric names: %s", deadline.String())
	}

	// Setup search.
	tr := sq.GetTimeRange()
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return nil, err
	}
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}

	metricNames, err := vmstorage.SearchMetricNames(qt, tfss, tr, sq.MaxMetrics, deadline.Deadline())
	if err != nil {
		return nil, fmt.Errorf("cannot find metric names: %w", err)
	}
	sort.Strings(metricNames)
	qt.Printf("sort %d metric names", len(metricNames))
	return metricNames, nil
}

// ProcessSearchQuery performs sq until the given deadline.
//
// Results.RunParallel or Results.Cancel must be called on the returned Results.
func ProcessSearchQuery(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) (*Results, error) {
	qt = qt.NewChild("fetch matching series: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}

	// Setup search.
	tr := sq.GetTimeRange()
	if err := vmstorage.CheckTimeRange(tr); err != nil {
		return nil, err
	}
	tfss, err := setupTfss(qt, tr, sq.TagFilterss, sq.MaxMetrics, deadline)
	if err != nil {
		return nil, err
	}

	vmstorage.WG.Add(1)
	defer vmstorage.WG.Done()

	sr := getStorageSearch()
	startTime := time.Now()
	maxSeriesCount := sr.Init(qt, vmstorage.Storage, tfss, tr, sq.MaxMetrics, deadline.Deadline())
	indexSearchDuration.UpdateDuration(startTime)
	type blockRefs struct {
		brsPrealloc [4]blockRef
		brs         []blockRef
	}
	m := make(map[string]*blockRefs, maxSeriesCount)
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
		// Do not intern mb.MetricName, since it leads to increased memory usage.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3692
		metricName := sr.MetricBlockRef.MetricName
		brs := m[string(metricName)]
		if brs == nil {
			brs = &blockRefs{}
			brs.brs = brs.brsPrealloc[:0]
		}
		brs.brs = append(brs.brs, blockRef{
			partRef: br.PartRef(),
			addr:    addr,
		})
		if len(brs.brs) == 1 {
			metricNameStr := string(metricName)
			orderedMetricNames = append(orderedMetricNames, metricNameStr)
			m[metricNameStr] = brs
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
	rss.deadline = deadline
	pts := make([]packedTimeseries, len(orderedMetricNames))
	for i, metricName := range orderedMetricNames {
		pts[i] = packedTimeseries{
			metricName: metricName,
			brs:        m[metricName].brs,
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

func setupTfss(qt *querytracer.Tracer, tr storage.TimeRange, tagFilterss [][]storage.TagFilter, maxMetrics int, deadline searchutils.Deadline) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(tagFilterss))
	for _, tagFilters := range tagFilterss {
		tfs := storage.NewTagFilters()
		for i := range tagFilters {
			tf := &tagFilters[i]
			if string(tf.Key) == "__graphite__" {
				query := tf.Value
				paths, err := vmstorage.SearchGraphitePaths(qt, tr, query, maxMetrics, deadline.Deadline())
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
	re, err := metricsql.CompileRegexp(filter)
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
