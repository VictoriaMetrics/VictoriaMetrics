package netstorage

import (
	"container/heap"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/cespare/xxhash/v2"
)

var (
	replicationFactor = flag.Int("replicationFactor", 1, "How many copies of every time series is available on the provided -storageNode nodes. "+
		"vmselect continues returning full responses when up to replicationFactor-1 vmstorage nodes are temporarily unavailable during querying. "+
		"See also -search.skipSlowReplicas")
	skipSlowReplicas = flag.Bool("search.skipSlowReplicas", false, "Whether to skip -replicationFactor - 1 slowest vmstorage nodes during querying. "+
		"Enabling this setting may improve query speed, but it could also lead to incomplete results if some queried data has less than -replicationFactor "+
		"copies at vmstorage nodes. Consider enabling this setting only if all the queried data contains -replicationFactor copies in the cluster")
	maxSamplesPerSeries  = flag.Int("search.maxSamplesPerSeries", 30e6, "The maximum number of raw samples a single query can scan per each time series. See also -search.maxSamplesPerQuery")
	maxSamplesPerQuery   = flag.Int("search.maxSamplesPerQuery", 1e9, "The maximum number of raw samples a single query can process across all time series. This protects from heavy queries, which select unexpectedly high number of raw samples. See also -search.maxSamplesPerSeries")
	vmstorageDialTimeout = flag.Duration("vmstorageDialTimeout", 3*time.Second, "Timeout for establishing RPC connections from vmselect to vmstorage. "+
		"See also -vmstorageUserTimeout")
	vmstorageUserTimeout = flag.Duration("vmstorageUserTimeout", 3*time.Second, "Network timeout for RPC connections from vmselect to vmstorage (Linux only). "+
		"Lower values reduce the maximum query durations when some vmstorage nodes become unavailable because of networking issues. "+
		"Read more about TCP_USER_TIMEOUT at https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/ . "+
		"See also -vmstorageDialTimeout")
	maxWorkersPerQuery = flag.Int("search.maxWorkersPerQuery", defaultMaxWorkersPerQuery, "The maximum number of CPU cores a single query can use. "+
		"The default value should work good for most cases. "+
		"The flag can be set to lower values for improving performance of big number of concurrently executed queries. "+
		"The flag can be set to bigger values for improving performance of heavy queries, which scan big number of time series (>10K) and/or big number of samples (>100M). "+
		"There is no sense in setting this flag to values bigger than the number of CPU cores available on the system")
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

	tbfs []*tmpBlocksFile

	packedTimeseries []packedTimeseries
}

// Len returns the number of results in rss.
func (rss *Results) Len() int {
	return len(rss.packedTimeseries)
}

// Cancel cancels rss work.
func (rss *Results) Cancel() {
	rss.closeTmpBlockFiles()
}

func (rss *Results) closeTmpBlockFiles() {
	closeTmpBlockFiles(rss.tbfs)
	rss.tbfs = nil
}

func closeTmpBlockFiles(tbfs []*tmpBlocksFile) {
	for _, tbf := range tbfs {
		putTmpBlocksFile(tbf)
	}
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
	if err := tsw.pts.Unpack(r, rss.tbfs, rss.tr); err != nil {
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
			// Do not call runtime.Gosched() here in order to give a chance
			// the real owner of the work to complete it, since it consumes additional CPU
			// and slows down the code on systems with big number of CPU cores.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3966#issuecomment-1483208419

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

// MaxWorkers returns the maximum number of concurrent goroutines, which can be used by RunParallel()
func MaxWorkers() int {
	n := *maxWorkersPerQuery
	if n <= 0 {
		return defaultMaxWorkersPerQuery
	}
	if n > gomaxprocs {
		// There is no sense in running more than gomaxprocs CPU-bound concurrent workers,
		// since this may worsen the query performance.
		n = gomaxprocs
	}
	return n
}

var gomaxprocs = cgroup.AvailableCPUs()

var defaultMaxWorkersPerQuery = func() int {
	// maxWorkersLimit is the maximum number of CPU cores, which can be used in parallel
	// for processing an average query, without significant impact on inter-CPU communications.
	const maxWorkersLimit = 32

	n := gomaxprocs
	if n > maxWorkersLimit {
		n = maxWorkersLimit
	}
	return n
}()

// RunParallel runs f in parallel for all the results from rss.
//
// f shouldn't hold references to rs after returning.
// workerID is the id of the worker goroutine that calls f. The workerID is in the range [0..MaxWorkers()-1].
// Data processing is immediately stopped if f returns non-nil error.
//
// rss becomes unusable after the call to RunParallel.
func (rss *Results) RunParallel(qt *querytracer.Tracer, f func(rs *Result, workerID uint) error) error {
	qt = qt.NewChild("parallel process of fetched data")
	defer rss.closeTmpBlockFiles()

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
	addrs      []tmpBlockAddr
}

type unpackWork struct {
	tbfs []*tmpBlocksFile
	addr tmpBlockAddr
	tr   storage.TimeRange
	sb   *sortBlock
	err  error
}

func (upw *unpackWork) reset() {
	upw.tbfs = nil
	upw.addr = tmpBlockAddr{}
	upw.tr = storage.TimeRange{}
	upw.sb = nil
	upw.err = nil
}

func (upw *unpackWork) unpack(tmpBlock *storage.Block) {
	sb := getSortBlock()
	if err := sb.unpackFrom(tmpBlock, upw.tbfs, upw.addr, upw.tr); err != nil {
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
			// Do not call runtime.Gosched() here in order to give a chance
			// the real owner of the work to complete it, since it consumes additional CPU
			// and slows down the code on systems with big number of CPU cores.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3966#issuecomment-1483208419

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
func (pts *packedTimeseries) Unpack(dst *Result, tbfs []*tmpBlocksFile, tr storage.TimeRange) error {
	dst.reset()
	if err := dst.MetricName.Unmarshal(bytesutil.ToUnsafeBytes(pts.metricName)); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %w", pts.metricName, err)
	}
	sbh := getSortBlocksHeap()
	var err error
	sbh.sbs, err = pts.unpackTo(sbh.sbs[:0], tbfs, tr)
	pts.addrs = pts.addrs[:0]
	if err != nil {
		putSortBlocksHeap(sbh)
		return err
	}
	dedupInterval := storage.GetDedupInterval()
	mergeSortBlocks(dst, sbh, dedupInterval)
	putSortBlocksHeap(sbh)
	return nil
}

func (pts *packedTimeseries) unpackTo(dst []*sortBlock, tbfs []*tmpBlocksFile, tr storage.TimeRange) ([]*sortBlock, error) {
	upwsLen := len(pts.addrs)
	if upwsLen == 0 {
		// Nothing to do
		return nil, nil
	}
	initUnpackWork := func(upw *unpackWork, addr tmpBlockAddr) {
		upw.tbfs = tbfs
		upw.addr = addr
		upw.tr = tr
	}
	if gomaxprocs == 1 || upwsLen <= 1000 {
		// It is faster to unpack all the data in the current goroutine.
		upw := getUnpackWork()
		samples := 0
		tmpBlock := getTmpStorageBlock()
		var err error
		for _, addr := range pts.addrs {
			initUnpackWork(upw, addr)
			upw.unpack(tmpBlock)
			if upw.err != nil {
				return dst, upw.err
			}
			samples += len(upw.sb.Timestamps)
			if *maxSamplesPerSeries > 0 && samples > *maxSamplesPerSeries {
				putSortBlock(upw.sb)
				err = &limitExceededErr{
					err: fmt.Errorf("cannot process more than %d samples per series; either increase -search.maxSamplesPerSeries "+
						"or reduce time range for the query", *maxSamplesPerSeries),
				}
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
	for i, addr := range pts.addrs {
		upw := getUnpackWork()
		initUnpackWork(upw, addr)
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

func (sb *sortBlock) unpackFrom(tmpBlock *storage.Block, tbfs []*tmpBlocksFile, addr tmpBlockAddr, tr storage.TimeRange) error {
	tmpBlock.Reset()
	tbfs[addr.tbfIdx].MustReadBlockAt(tmpBlock, addr)
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

// RegisterMetricNames registers metric names from mrs in the storage.
func RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline searchutils.Deadline) error {
	qt = qt.NewChild("register metric names")
	defer qt.Done()
	sns := getStorageNodes()
	// Split mrs among available vmstorage nodes.
	mrsPerNode := make([][]storage.MetricRow, len(sns))
	for _, mr := range mrs {
		idx := 0
		if len(sns) > 1 {
			// There is no need in using the same hash as for time series distribution in vminsert,
			// since RegisterMetricNames is used only in Graphite Tags API.
			h := xxhash.Sum64(mr.MetricNameRaw)
			idx = int(h % uint64(len(sns)))
		}
		mrsPerNode[idx] = append(mrsPerNode[idx], mr)
	}

	// Push mrs to storage nodes in parallel.
	snr := startStorageNodesRequest(qt, sns, true, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.registerMetricNamesRequests.Inc()
		err := sn.registerMetricNames(qt, mrsPerNode[workerID], deadline)
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
func DeleteSeries(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline) (int, error) {
	qt = qt.NewChild("delete series: %s", sq)
	defer qt.Done()
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		deletedCount int
		err          error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, true, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.deleteSeriesRequests.Inc()
		deletedCount, err := sn.deleteSeries(qt, requestData, deadline)
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

// LabelNames returns label names matching the given sq until the given deadline.
func LabelNames(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, maxLabelNames int, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("get labels: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelNames []string
		err        error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.labelNamesRequests.Inc()
		labelNames, err := sn.getLabelNames(qt, requestData, maxLabelNames, deadline)
		if err != nil {
			sn.labelNamesErrors.Inc()
			err = fmt.Errorf("cannot get labels from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			labelNames: labelNames,
			err:        err,
		}
	})

	// Collect results
	var labelNames []string
	isPartial, err := snr.collectResults(partialLabelNamesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		labelNames = append(labelNames, nr.labelNames...)
		return nil
	})
	qt.Printf("get %d non-duplicated labels", len(labelNames))
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch labels from vmstorage nodes: %w", err)
	}

	// Deduplicate labels
	labelNames = deduplicateStrings(labelNames)
	qt.Printf("get %d unique labels after de-duplication", len(labelNames))
	if maxLabelNames > 0 && maxLabelNames < len(labelNames) {
		labelNames = labelNames[:maxLabelNames]
	}
	// Sort labelNames like Prometheus does
	sort.Strings(labelNames)
	qt.Printf("sort %d labels", len(labelNames))
	return labelNames, isPartial, nil
}

// GraphiteTags returns Graphite tags until the given deadline.
func GraphiteTags(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool, filter string, limit int, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("get graphite tags: filter=%s, limit=%d", filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	sq := storage.NewSearchQuery(accountID, projectID, 0, 0, nil, 0)
	labels, isPartial, err := LabelNames(qt, denyPartialResponse, sq, 0, deadline)
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

// LabelValues returns label values matching the given labelName and sq until the given deadline.
func LabelValues(qt *querytracer.Tracer, denyPartialResponse bool, labelName string, sq *storage.SearchQuery, maxLabelValues int, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("get values for label %s: %s", labelName, sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelValues []string
		err         error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.labelValuesRequests.Inc()
		labelValues, err := sn.getLabelValues(qt, labelName, requestData, maxLabelValues, deadline)
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
	qt.Printf("get %d non-duplicated label values", len(labelValues))
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch label values from vmstorage nodes: %w", err)
	}

	// Deduplicate label values
	labelValues = deduplicateStrings(labelValues)
	qt.Printf("get %d unique label values after de-duplication", len(labelValues))
	// Sort labelValues like Prometheus does
	if maxLabelValues > 0 && maxLabelValues < len(labelValues) {
		labelValues = labelValues[:maxLabelValues]
	}
	sort.Strings(labelValues)
	qt.Printf("sort %d label values", len(labelValues))
	return labelValues, isPartial, nil
}

// Tenants returns tenants until the given deadline.
func Tenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	qt = qt.NewChild("get tenants on timeRange=%s", &tr)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		tenants []string
		err     error
	}
	sns := getStorageNodes()
	// Deny partial responses when obtaining the list of tenants, since partial tenants have little sense.
	snr := startStorageNodesRequest(qt, sns, true, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.tenantsRequests.Inc()
		tenants, err := sn.getTenants(qt, tr, deadline)
		if err != nil {
			sn.tenantsErrors.Inc()
			err = fmt.Errorf("cannot get tenants from vmstorage %s: %w", sn.connPool.Addr(), err)
		}
		return &nodeResult{
			tenants: tenants,
			err:     err,
		}
	})

	// Collect results
	var tenants []string
	_, err := snr.collectResults(partialLabelValuesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		tenants = append(tenants, nr.tenants...)
		return nil
	})
	qt.Printf("get %d non-duplicated tenants", len(tenants))
	if err != nil {
		return nil, fmt.Errorf("cannot fetch tenants from vmstorage nodes: %w", err)
	}

	// Deduplicate tenants
	tenants = deduplicateStrings(tenants)
	qt.Printf("get %d unique tenants after de-duplication", len(tenants))

	sort.Strings(tenants)
	qt.Printf("sort %d tenants", len(tenants))
	return tenants, nil
}

// GraphiteTagValues returns tag values for the given tagName until the given deadline.
func GraphiteTagValues(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool, tagName, filter string, limit int, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("get graphite tag values for tagName=%s, filter=%s, limit=%d", tagName, filter, limit)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	if tagName == "name" {
		tagName = ""
	}
	sq := storage.NewSearchQuery(accountID, projectID, 0, 0, nil, 0)
	tagValues, isPartial, err := LabelValues(qt, denyPartialResponse, tagName, sq, 0, deadline)
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

// TagValueSuffixes returns tag value suffixes for the given tagKey and the given tagValuePrefix.
//
// It can be used for implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func TagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool, tr storage.TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, maxSuffixes int, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("get tag value suffixes for tagKey=%s, tagValuePrefix=%s, maxSuffixes=%d, timeRange=%s", tagKey, tagValuePrefix, maxSuffixes, &tr)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		suffixes []string
		err      error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.tagValueSuffixesRequests.Inc()
		suffixes, err := sn.getTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
		if err != nil {
			sn.tagValueSuffixesErrors.Inc()
			err = fmt.Errorf("cannot get tag value suffixes for timeRange=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c from vmstorage %s: %w",
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

// TSDBStatus returns tsdb status according to https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It accepts arbitrary filters on time series in sq.
func TSDBStatus(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, focusLabel string, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, bool, error) {
	qt = qt.NewChild("get tsdb stats: %s, focusLabel=%q, topN=%d", sq, focusLabel, topN)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		status *storage.TSDBStatus
		err    error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.tsdbStatusRequests.Inc()
		status, err := sn.getTSDBStatus(qt, requestData, focusLabel, topN, deadline)
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
	totalSeries := uint64(0)
	totalLabelValuePairs := uint64(0)
	seriesCountByMetricName := make(map[string]uint64)
	seriesCountByLabelName := make(map[string]uint64)
	seriesCountByFocusLabelValue := make(map[string]uint64)
	seriesCountByLabelValuePair := make(map[string]uint64)
	labelValueCountByLabelName := make(map[string]uint64)
	for _, st := range statuses {
		totalSeries += st.TotalSeries
		totalLabelValuePairs += st.TotalLabelValuePairs
		for _, e := range st.SeriesCountByMetricName {
			seriesCountByMetricName[e.Name] += e.Count
		}
		for _, e := range st.SeriesCountByLabelName {
			seriesCountByLabelName[e.Name] += e.Count
		}
		for _, e := range st.SeriesCountByFocusLabelValue {
			seriesCountByFocusLabelValue[e.Name] += e.Count
		}
		for _, e := range st.SeriesCountByLabelValuePair {
			seriesCountByLabelValuePair[e.Name] += e.Count
		}
		for _, e := range st.LabelValueCountByLabelName {
			// The same label values may exist in multiple vmstorage nodes.
			// So select the maximum label values count in order to get the value close to reality.
			if e.Count > labelValueCountByLabelName[e.Name] {
				labelValueCountByLabelName[e.Name] = e.Count
			}
		}
	}
	return &storage.TSDBStatus{
		TotalSeries:                  totalSeries,
		TotalLabelValuePairs:         totalLabelValuePairs,
		SeriesCountByMetricName:      toTopHeapEntries(seriesCountByMetricName, topN),
		SeriesCountByLabelName:       toTopHeapEntries(seriesCountByLabelName, topN),
		SeriesCountByFocusLabelValue: toTopHeapEntries(seriesCountByFocusLabelValue, topN),
		SeriesCountByLabelValuePair:  toTopHeapEntries(seriesCountByLabelValuePair, topN),
		LabelValueCountByLabelName:   toTopHeapEntries(labelValueCountByLabelName, topN),
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

// SeriesCount returns the number of unique series.
func SeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool, deadline searchutils.Deadline) (uint64, bool, error) {
	qt = qt.NewChild("get series count")
	defer qt.Done()
	if deadline.Exceeded() {
		return 0, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		n   uint64
		err error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.seriesCountRequests.Inc()
		n, err := sn.getSeriesCount(qt, accountID, projectID, deadline)
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
	tbfs                []*tmpBlocksFile
	ms                  []map[string]*blockAddrs
	orderedMetricNamess [][]string
}

type blockAddrs struct {
	addrsPrealloc [4]tmpBlockAddr
	addrs         []tmpBlockAddr
}

func newBlockAddrs() *blockAddrs {
	ba := &blockAddrs{}
	ba.addrs = ba.addrsPrealloc[:0]
	return ba
}

func newTmpBlocksFileWrapper(sns []*storageNode) *tmpBlocksFileWrapper {
	n := len(sns)
	tbfs := make([]*tmpBlocksFile, n)
	for i := range tbfs {
		tbfs[i] = getTmpBlocksFile()
	}
	ms := make([]map[string]*blockAddrs, n)
	for i := range ms {
		ms[i] = make(map[string]*blockAddrs)
	}
	return &tmpBlocksFileWrapper{
		tbfs:                tbfs,
		ms:                  ms,
		orderedMetricNamess: make([][]string, n),
	}
}

func (tbfw *tmpBlocksFileWrapper) RegisterAndWriteBlock(mb *storage.MetricBlock, workerID uint) error {
	bb := tmpBufPool.Get()
	bb.B = storage.MarshalBlock(bb.B[:0], &mb.Block)
	addr, err := tbfw.tbfs[workerID].WriteBlockData(bb.B, workerID)
	tmpBufPool.Put(bb)
	if err != nil {
		return err
	}
	// Do not intern mb.MetricName, since it leads to increased memory usage.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3692
	metricName := mb.MetricName
	m := tbfw.ms[workerID]
	addrs := m[string(metricName)]
	if addrs == nil {
		addrs = newBlockAddrs()
	}
	addrs.addrs = append(addrs.addrs, addr)
	if len(addrs.addrs) == 1 {
		// An optimization for big number of time series with long names: store only a single copy of metricNameStr
		// in both tbfw.orderedMetricNamess and tbfw.ms.
		orderedMetricNames := tbfw.orderedMetricNamess[workerID]
		metricNameStr := string(metricName)
		orderedMetricNames = append(orderedMetricNames, metricNameStr)
		m[metricNameStr] = addrs
		tbfw.orderedMetricNamess[workerID] = orderedMetricNames
	}
	return nil
}

func (tbfw *tmpBlocksFileWrapper) Finalize() ([]string, map[string]*blockAddrs, uint64, error) {
	var bytesTotal uint64
	for i, tbf := range tbfw.tbfs {
		if err := tbf.Finalize(); err != nil {
			closeTmpBlockFiles(tbfw.tbfs)
			return nil, nil, 0, fmt.Errorf("cannot finalize temporary blocks file with %d series: %w", len(tbfw.ms[i]), err)
		}
		bytesTotal += tbf.Len()
	}
	orderedMetricNames := tbfw.orderedMetricNamess[0]
	addrsByMetricName := tbfw.ms[0]
	for i, m := range tbfw.ms[1:] {
		for _, metricName := range tbfw.orderedMetricNamess[i+1] {
			dstAddrs, ok := addrsByMetricName[metricName]
			if !ok {
				orderedMetricNames = append(orderedMetricNames, metricName)
				dstAddrs = newBlockAddrs()
				addrsByMetricName[metricName] = dstAddrs
			}
			dstAddrs.addrs = append(dstAddrs.addrs, m[metricName].addrs...)
		}
	}
	return orderedMetricNames, addrsByMetricName, bytesTotal, nil
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
func ExportBlocks(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutils.Deadline,
	f func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange, workerID uint) error) error {
	qt = qt.NewChild("export blocks: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return fmt.Errorf("timeout exceeded before starting data export: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	sns := getStorageNodes()
	blocksRead := newPerNodeCounter(sns)
	samples := newPerNodeCounter(sns)
	processBlock := func(mb *storage.MetricBlock, workerID uint) error {
		mn := metricNamePool.Get().(*storage.MetricName)
		if err := mn.Unmarshal(mb.MetricName); err != nil {
			return fmt.Errorf("cannot unmarshal metricName: %w", err)
		}
		if err := f(mn, &mb.Block, tr, workerID); err != nil {
			return err
		}
		mn.Reset()
		metricNamePool.Put(mn)
		blocksRead.Add(workerID, 1)
		samples.Add(workerID, uint64(mb.Block.RowsCount()))
		return nil
	}
	_, err := processBlocks(qt, sns, true, sq, processBlock, deadline)
	qt.Printf("export blocks=%d, samples=%d, err=%v", blocksRead.GetTotal(), samples.GetTotal(), err)
	if err != nil {
		return fmt.Errorf("error occured during export: %w", err)
	}
	return nil
}

// SearchMetricNames returns all the metric names matching sq until the given deadline.
//
// The returned metric names must be unmarshaled via storage.MetricName.UnmarshalString().
func SearchMetricNames(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutils.Deadline) ([]string, bool, error) {
	qt = qt.NewChild("fetch metric names: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting to search metric names: %s", deadline.String())
	}
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		metricNames []string
		err         error
	}
	sns := getStorageNodes()
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.searchMetricNamesRequests.Inc()
		metricNames, err := sn.processSearchMetricNames(qt, requestData, deadline)
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
	metricNamesMap := make(map[string]struct{})
	isPartial, err := snr.collectResults(partialSearchMetricNamesResults, func(result interface{}) error {
		nr := result.(*nodeResult)
		if nr.err != nil {
			return nr.err
		}
		for _, metricName := range nr.metricNames {
			metricNamesMap[metricName] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, isPartial, fmt.Errorf("cannot fetch metric names from vmstorage nodes: %w", err)
	}

	metricNames := make([]string, 0, len(metricNamesMap))
	for metricName := range metricNamesMap {
		metricNames = append(metricNames, metricName)
	}
	sort.Strings(metricNames)
	qt.Printf("sort %d metric names", len(metricNames))
	return metricNames, isPartial, nil
}

// limitExceededErr error generated by vmselect
// on checking complexity limits during processing responses
// from storage nodes.
type limitExceededErr struct {
	err error
}

// Error satisfies error interface
func (e limitExceededErr) Error() string { return e.err.Error() }

// ProcessSearchQuery performs sq until the given deadline.
//
// Results.RunParallel or Results.Cancel must be called on the returned Results.
func ProcessSearchQuery(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutils.Deadline) (*Results, bool, error) {
	qt = qt.NewChild("fetch matching series: %s", sq)
	defer qt.Done()
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}

	// Setup search.
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	sns := getStorageNodes()
	tbfw := newTmpBlocksFileWrapper(sns)
	blocksRead := newPerNodeCounter(sns)
	samples := newPerNodeCounter(sns)
	maxSamplesPerWorker := uint64(*maxSamplesPerQuery) / uint64(len(sns))
	processBlock := func(mb *storage.MetricBlock, workerID uint) error {
		blocksRead.Add(workerID, 1)
		n := samples.Add(workerID, uint64(mb.Block.RowsCount()))
		if *maxSamplesPerQuery > 0 && n > maxSamplesPerWorker && samples.GetTotal() > uint64(*maxSamplesPerQuery) {
			return &limitExceededErr{
				err: fmt.Errorf("cannot select more than -search.maxSamplesPerQuery=%d samples; possible solutions: "+
					"to increase the -search.maxSamplesPerQuery; to reduce time range for the query; "+
					"to use more specific label filters in order to select lower number of series", *maxSamplesPerQuery),
			}
		}
		if err := tbfw.RegisterAndWriteBlock(mb, workerID); err != nil {
			return fmt.Errorf("cannot write MetricBlock to temporary blocks file: %w", err)
		}
		return nil
	}
	isPartial, err := processBlocks(qt, sns, denyPartialResponse, sq, processBlock, deadline)
	if err != nil {
		closeTmpBlockFiles(tbfw.tbfs)
		return nil, false, fmt.Errorf("error occured during search: %w", err)
	}
	orderedMetricNames, addrsByMetricName, bytesTotal, err := tbfw.Finalize()
	if err != nil {
		return nil, false, fmt.Errorf("cannot finalize temporary blocks files: %w", err)
	}
	qt.Printf("fetch unique series=%d, blocks=%d, samples=%d, bytes=%d", len(addrsByMetricName), blocksRead.GetTotal(), samples.GetTotal(), bytesTotal)

	var rss Results
	rss.tr = tr
	rss.deadline = deadline
	rss.tbfs = tbfw.tbfs
	pts := make([]packedTimeseries, len(orderedMetricNames))
	for i, metricName := range orderedMetricNames {
		pts[i] = packedTimeseries{
			metricName: metricName,
			addrs:      addrsByMetricName[metricName].addrs,
		}
	}
	rss.packedTimeseries = pts
	return &rss, isPartial, nil
}

// ProcessBlocks calls processBlock per each block matching the given sq.
func ProcessBlocks(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery,
	processBlock func(mb *storage.MetricBlock, workerID uint) error, deadline searchutils.Deadline) (bool, error) {
	sns := getStorageNodes()
	return processBlocks(qt, sns, denyPartialResponse, sq, processBlock, deadline)
}

func processBlocks(qt *querytracer.Tracer, sns []*storageNode, denyPartialResponse bool, sq *storage.SearchQuery,
	processBlock func(mb *storage.MetricBlock, workerID uint) error, deadline searchutils.Deadline) (bool, error) {
	requestData := sq.Marshal(nil)

	// Make sure that processBlock is no longer called after the exit from processBlocks() function.
	// Use per-worker WaitGroup instead of a shared WaitGroup in order to avoid inter-CPU contention,
	// which may significantly slow down the rate of processBlock calls on multi-CPU systems.
	type wgStruct struct {
		// mu prevents from calling processBlock when stop is set to true
		mu sync.Mutex

		// wg is used for waiting until currently executed processBlock calls are finished.
		wg sync.WaitGroup

		// stop must be set to true when no more processBlocks calls should be made.
		stop bool
	}
	type wgWithPadding struct {
		wgStruct
		// The padding prevents false sharing on widespread platforms with
		// 128 mod (cache line size) = 0 .
		_ [128 - unsafe.Sizeof(wgStruct{})%128]byte
	}
	wgs := make([]wgWithPadding, len(sns))
	f := func(mb *storage.MetricBlock, workerID uint) error {
		muwg := &wgs[workerID]
		muwg.mu.Lock()
		if muwg.stop {
			muwg.mu.Unlock()
			return nil
		}
		muwg.wg.Add(1)
		muwg.mu.Unlock()
		err := processBlock(mb, workerID)
		muwg.wg.Done()
		return err
	}

	// Send the query to all the storage nodes in parallel.
	snr := startStorageNodesRequest(qt, sns, denyPartialResponse, func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{} {
		sn.searchRequests.Inc()
		err := sn.processSearchQuery(qt, requestData, f, workerID, deadline)
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
	// Make sure that processBlock is no longer called after the exit from processBlocks() function.
	for i := range wgs {
		muwg := &wgs[i]
		muwg.mu.Lock()
		muwg.stop = true
		muwg.mu.Unlock()
	}
	for i := range wgs {
		wgs[i].wg.Wait()
	}
	if err != nil {
		return isPartial, fmt.Errorf("cannot fetch query results from vmstorage nodes: %w", err)
	}
	return isPartial, nil
}

type storageNodesRequest struct {
	denyPartialResponse bool
	resultsCh           chan rpcResult
	qts                 map[*querytracer.Tracer]struct{}
	sns                 []*storageNode
}

type rpcResult struct {
	data interface{}
	qt   *querytracer.Tracer
}

func startStorageNodesRequest(qt *querytracer.Tracer, sns []*storageNode, denyPartialResponse bool,
	f func(qt *querytracer.Tracer, workerID uint, sn *storageNode) interface{}) *storageNodesRequest {
	resultsCh := make(chan rpcResult, len(sns))
	qts := make(map[*querytracer.Tracer]struct{}, len(sns))
	for idx, sn := range sns {
		qtChild := qt.NewChild("rpc at vmstorage %s", sn.connPool.Addr())
		qts[qtChild] = struct{}{}
		go func(workerID uint, sn *storageNode) {
			data := f(qtChild, workerID, sn)
			resultsCh <- rpcResult{
				data: data,
				qt:   qtChild,
			}
		}(uint(idx), sn)
	}
	return &storageNodesRequest{
		denyPartialResponse: denyPartialResponse,
		resultsCh:           resultsCh,
		qts:                 qts,
		sns:                 sns,
	}
}

func (snr *storageNodesRequest) finishQueryTracers(msg string) {
	for qt := range snr.qts {
		snr.finishQueryTracer(qt, msg)
	}
}

func (snr *storageNodesRequest) finishQueryTracer(qt *querytracer.Tracer, msg string) {
	if msg == "" {
		qt.Done()
	} else {
		qt.Donef("%s", msg)
	}
	delete(snr.qts, qt)
}

func (snr *storageNodesRequest) collectAllResults(f func(result interface{}) error) error {
	sns := snr.sns
	for i := 0; i < len(sns); i++ {
		result := <-snr.resultsCh
		if err := f(result.data); err != nil {
			snr.finishQueryTracer(result.qt, fmt.Sprintf("error: %s", err))
			// Immediately return the error to the caller without waiting for responses from other vmstorage nodes -
			// they will be processed in brackground.
			snr.finishQueryTracers("cancel request because of error in other vmstorage nodes")
			return err
		}
		snr.finishQueryTracer(result.qt, "")
	}
	return nil
}

func (snr *storageNodesRequest) collectResults(partialResultsCounter *metrics.Counter, f func(result interface{}) error) (bool, error) {
	var errsPartial []error
	resultsCollected := 0
	sns := snr.sns
	for i := 0; i < len(sns); i++ {
		// There is no need in timer here, since all the goroutines executing the f function
		// passed to startStorageNodesRequest must be finished until the deadline.
		result := <-snr.resultsCh
		if err := f(result.data); err != nil {
			snr.finishQueryTracer(result.qt, fmt.Sprintf("error: %s", err))
			var er *errRemote
			if errors.As(err, &er) {
				// Immediately return the error reported by vmstorage to the caller,
				// since such errors usually mean misconfiguration at vmstorage.
				// The misconfiguration must be known by the caller, so it is fixed ASAP.
				snr.finishQueryTracers("cancel request because of error in other vmstorage nodes")
				return false, err
			}
			var limitErr *limitExceededErr
			if errors.As(err, &limitErr) {
				// Immediately return the error, since complexity limits are already exceeded,
				// and we don't need to process the rest of results.
				snr.finishQueryTracers("cancel request because query complexity limit was exceeded")
				return false, err
			}
			errsPartial = append(errsPartial, err)
			if snr.denyPartialResponse && len(errsPartial) >= *replicationFactor {
				// Return the error to the caller if partial responses are denied
				// and the number of partial responses reach -replicationFactor,
				// since this means that the response is partial.
				snr.finishQueryTracers("cancel request because partial responses are denied and some vmstorage nodes failed to return response")

				// Returns 503 status code for partial response, so the caller could retry it if needed.
				err = &httpserver.ErrorWithStatusCode{
					Err:        err,
					StatusCode: http.StatusServiceUnavailable,
				}
				return false, err
			}
			continue
		}
		snr.finishQueryTracer(result.qt, "")
		resultsCollected++
		if *skipSlowReplicas && resultsCollected > len(sns)-*replicationFactor {
			// There is no need in waiting for the remaining results,
			// because the collected results contain all the data according to the given -replicationFactor.
			// This should speed up responses when a part of vmstorage nodes are slow and/or temporarily unavailable.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711
			//
			// It is expected that cap(snr.resultsCh) == len(sns), otherwise goroutine leak is possible.
			snr.finishQueryTracers(fmt.Sprintf("cancel request because -search.skipSlowReplicas is set and %d out of %d nodes already returned response "+
				"according to -replicationFactor=%d", resultsCollected, len(sns), *replicationFactor))
			return false, nil
		}
	}
	if len(errsPartial) < *replicationFactor {
		// Assume that the result is full if the the number of failing vmstorage nodes
		// is smaller than the -replicationFactor.
		return false, nil
	}
	if len(errsPartial) == len(sns) {
		// All the vmstorage nodes returned error.
		// Return only the first error, since it has no sense in returning all errors.
		// Returns 503 status code for partial response, so the caller could retry it if needed.
		err := &httpserver.ErrorWithStatusCode{
			Err:        errsPartial[0],
			StatusCode: http.StatusServiceUnavailable,
		}
		return false, err
	}
	// Return partial results.
	// This allows gracefully degrade vmselect in the case
	// if a part of vmstorage nodes are temporarily unavailable.
	partialResultsCounter.Inc()
	// Do not return the error, since it may spam logs on busy vmselect
	// serving high amounts of requests.
	partialErrorsLogger.Warnf("%d out of %d vmstorage nodes were unavailable during the query; a sample error: %s", len(errsPartial), len(sns), errsPartial[0])
	return true, nil
}

var partialErrorsLogger = logger.WithThrottler("partialErrors", 10*time.Second)

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

	// The number of requests to labelNames.
	labelNamesRequests *metrics.Counter

	// The number of errors during requests to labelNames.
	labelNamesErrors *metrics.Counter

	// The number of requests to labelValues.
	labelValuesRequests *metrics.Counter

	// The number of errors during requests to labelValuesOnTimeRange.
	labelValuesErrors *metrics.Counter

	// The number of requests to tagValueSuffixes.
	tagValueSuffixesRequests *metrics.Counter

	// The number of errors during requests to tagValueSuffixes.
	tagValueSuffixesErrors *metrics.Counter

	// The number of requests to tsdb status.
	tsdbStatusRequests *metrics.Counter

	// The number of errors during requests to tsdb status.
	tsdbStatusErrors *metrics.Counter

	// The number of requests to seriesCount.
	seriesCountRequests *metrics.Counter

	// The number of errors during requests to seriesCount.
	seriesCountErrors *metrics.Counter

	// The number of searchMetricNames requests to storageNode.
	searchMetricNamesRequests *metrics.Counter

	// The number of searchMetricNames errors to storageNode.
	searchMetricNamesErrors *metrics.Counter

	// The number of search requests to storageNode.
	searchRequests *metrics.Counter

	// The number of search request errors to storageNode.
	searchErrors *metrics.Counter

	// The number of metric blocks read.
	metricBlocksRead *metrics.Counter

	// The number of read metric rows.
	metricRowsRead *metrics.Counter

	// The number of list tenants requests to storageNode.
	tenantsRequests *metrics.Counter

	// The number of list tenants errors to storageNode.
	tenantsErrors *metrics.Counter
}

func (sn *storageNode) registerMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline searchutils.Deadline) error {
	if len(mrs) == 0 {
		return nil
	}
	f := func(bc *handshake.BufferedConn) error {
		return sn.registerMetricNamesOnConn(bc, mrs)
	}
	return sn.execOnConnWithPossibleRetry(qt, "registerMetricNames_v3", f, deadline)
}

func (sn *storageNode) deleteSeries(qt *querytracer.Tracer, requestData []byte, deadline searchutils.Deadline) (int, error) {
	var deletedCount int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.deleteSeriesOnConn(bc, requestData)
		if err != nil {
			return err
		}
		deletedCount = n
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "deleteSeries_v5", f, deadline); err != nil {
		return 0, err
	}
	return deletedCount, nil
}

func (sn *storageNode) getLabelNames(qt *querytracer.Tracer, requestData []byte, maxLabelNames int, deadline searchutils.Deadline) ([]string, error) {
	var labels []string
	f := func(bc *handshake.BufferedConn) error {
		ls, err := sn.getLabelNamesOnConn(bc, requestData, maxLabelNames)
		if err != nil {
			return err
		}
		labels = ls
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "labelNames_v5", f, deadline); err != nil {
		return nil, err
	}
	return labels, nil
}

func (sn *storageNode) getLabelValues(qt *querytracer.Tracer, labelName string, requestData []byte, maxLabelValues int, deadline searchutils.Deadline) ([]string, error) {
	var labelValues []string
	f := func(bc *handshake.BufferedConn) error {
		lvs, err := sn.getLabelValuesOnConn(bc, labelName, requestData, maxLabelValues)
		if err != nil {
			return err
		}
		labelValues = lvs
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "labelValues_v5", f, deadline); err != nil {
		return nil, err
	}
	return labelValues, nil
}

func (sn *storageNode) getTenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	var tenants []string
	f := func(bc *handshake.BufferedConn) error {
		result, err := sn.getTenantsOnConn(bc, tr)
		if err != nil {
			return err
		}
		tenants = result
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "tenants_v1", f, deadline); err != nil {
		return nil, err
	}
	return tenants, nil
}

func (sn *storageNode) getTagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, maxSuffixes int, deadline searchutils.Deadline) ([]string, error) {
	var suffixes []string
	f := func(bc *handshake.BufferedConn) error {
		ss, err := sn.getTagValueSuffixesOnConn(bc, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes)
		if err != nil {
			return err
		}
		suffixes = ss
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "tagValueSuffixes_v4", f, deadline); err != nil {
		return nil, err
	}
	return suffixes, nil
}

func (sn *storageNode) getTSDBStatus(qt *querytracer.Tracer, requestData []byte, focusLabel string, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, error) {
	var status *storage.TSDBStatus
	f := func(bc *handshake.BufferedConn) error {
		st, err := sn.getTSDBStatusOnConn(bc, requestData, focusLabel, topN)
		if err != nil {
			return err
		}
		status = st
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "tsdbStatus_v5", f, deadline); err != nil {
		return nil, err
	}
	return status, nil
}

func (sn *storageNode) getSeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline searchutils.Deadline) (uint64, error) {
	var n uint64
	f := func(bc *handshake.BufferedConn) error {
		nn, err := sn.getSeriesCountOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		n = nn
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "seriesCount_v4", f, deadline); err != nil {
		return 0, err
	}
	return n, nil
}

func (sn *storageNode) processSearchMetricNames(qt *querytracer.Tracer, requestData []byte, deadline searchutils.Deadline) ([]string, error) {
	var metricNames []string
	f := func(bc *handshake.BufferedConn) error {
		mns, err := sn.processSearchMetricNamesOnConn(bc, requestData)
		if err != nil {
			return err
		}
		metricNames = mns
		return nil
	}
	if err := sn.execOnConnWithPossibleRetry(qt, "searchMetricNames_v3", f, deadline); err != nil {
		return nil, err
	}
	return metricNames, nil
}

func (sn *storageNode) processSearchQuery(qt *querytracer.Tracer, requestData []byte, processBlock func(mb *storage.MetricBlock, workerID uint) error,
	workerID uint, deadline searchutils.Deadline) error {
	f := func(bc *handshake.BufferedConn) error {
		return sn.processSearchQueryOnConn(bc, requestData, processBlock, workerID)
	}
	return sn.execOnConnWithPossibleRetry(qt, "search_v7", f, deadline)
}

func (sn *storageNode) execOnConnWithPossibleRetry(qt *querytracer.Tracer, funcName string, f func(bc *handshake.BufferedConn) error, deadline searchutils.Deadline) error {
	qtChild := qt.NewChild("rpc call %s()", funcName)
	err := sn.execOnConn(qtChild, funcName, f, deadline)
	defer qtChild.Done()
	if err == nil {
		return nil
	}
	var er *errRemote
	var ne net.Error
	if errors.As(err, &er) || errors.As(err, &ne) && ne.Timeout() || deadline.Exceeded() {
		// There is no sense in repeating the query on the following errors:
		//
		//   - induced by vmstorage (errRemote)
		//   - network timeout errors
		//   - request deadline exceeded errors
		return err
	}
	// Repeat the query in the hope the error was temporary.
	qtRetry := qtChild.NewChild("retry rpc call %s() after error", funcName)
	err = sn.execOnConn(qtRetry, funcName, f, deadline)
	qtRetry.Done()
	return err
}

func (sn *storageNode) execOnConn(qt *querytracer.Tracer, funcName string, f func(bc *handshake.BufferedConn) error, deadline searchutils.Deadline) error {
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
	if err := writeBytes(bc, []byte(funcName)); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send funcName=%q to the server: %w", funcName, err)
	}

	// Send query trace flag
	traceEnabled := qt.Enabled()
	if err := writeBool(bc, traceEnabled); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send traceEnabled=%v for funcName=%q to the server: %w", traceEnabled, funcName, err)
	}
	// Send the remaining timeout instead of deadline to remote server, since it may have different time.
	timeoutSecs := uint32(timeout.Seconds() + 1)
	if err := writeUint32(bc, timeoutSecs); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send timeout=%d for funcName=%q to the server: %w", timeout, funcName, err)
	}
	// Execute the rpc function.
	if err := f(bc); err != nil {
		remoteAddr := bc.RemoteAddr()
		var er *errRemote
		if errors.As(err, &er) {
			// Remote error. The connection may be re-used. Return it to the pool.
			_ = readTrace(qt, bc)
			sn.connPool.Put(bc)
		} else {
			// Local error.
			// Close the connection instead of returning it to the pool,
			// since it may be broken.
			_ = bc.Close()
		}
		if deadline.Exceeded() || errors.Is(err, os.ErrDeadlineExceeded) {
			return fmt.Errorf("cannot execute funcName=%q on vmstorage %q with timeout %s: %w", funcName, remoteAddr, deadline.String(), err)
		}
		return fmt.Errorf("cannot execute funcName=%q on vmstorage %q: %w", funcName, remoteAddr, err)
	}

	// Read trace from the response
	if err := readTrace(qt, bc); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return err
	}
	// Return the connection back to the pool, assuming it is healthy.
	sn.connPool.Put(bc)
	return nil
}

func readTrace(qt *querytracer.Tracer, bc *handshake.BufferedConn) error {
	bb := traceJSONBufPool.Get()
	var err error
	bb.B, err = readBytes(bb.B[:0], bc, maxTraceJSONSize)
	if err != nil {
		return fmt.Errorf("cannot read trace from the server: %w", err)
	}
	if err := qt.AddJSON(bb.B); err != nil {
		return fmt.Errorf("cannot parse trace read from the server: %w", err)
	}
	traceJSONBufPool.Put(bb)
	return nil
}

var traceJSONBufPool bytesutil.ByteBufferPool

const maxTraceJSONSize = 1024 * 1024

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

func (sn *storageNode) deleteSeriesOnConn(bc *handshake.BufferedConn, requestData []byte) (int, error) {
	// Send the request to sn
	if err := writeBytes(bc, requestData); err != nil {
		return 0, fmt.Errorf("cannot send deleteSeries request to conn: %w", err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush deleteSeries request to conn: %w", err)
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

const maxLabelNameSize = 16 * 1024 * 1024

func (sn *storageNode) getLabelNamesOnConn(bc *handshake.BufferedConn, requestData []byte, maxLabelNames int) ([]string, error) {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return nil, fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := writeLimit(bc, maxLabelNames); err != nil {
		return nil, fmt.Errorf("cannot write maxLabelNames=%d: %w", maxLabelNames, err)
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
		buf, err = readBytes(buf[:0], bc, maxLabelNameSize)
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
const maxTenantValueSize = 16 * 1024 * 1024 // TODO: calc 'uint32:uint32'

func (sn *storageNode) getLabelValuesOnConn(bc *handshake.BufferedConn, labelName string, requestData []byte, maxLabelValues int) ([]string, error) {
	// Send the request to sn.
	if err := writeBytes(bc, []byte(labelName)); err != nil {
		return nil, fmt.Errorf("cannot send labelName=%q to conn: %w", labelName, err)
	}
	if err := writeBytes(bc, requestData); err != nil {
		return nil, fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := writeLimit(bc, maxLabelValues); err != nil {
		return nil, fmt.Errorf("cannot write maxLabelValues=%d: %w", maxLabelValues, err)
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

func (sn *storageNode) getTenantsOnConn(bc *handshake.BufferedConn, tr storage.TimeRange) ([]string, error) {
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
	var tenants []string
	for {
		var err error
		buf, err = readBytes(buf[:0], bc, maxTenantValueSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read tenant #%d: %w", len(tenants), err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return tenants, nil
		}
		tenants = append(tenants, string(buf))
	}
}

func (sn *storageNode) getTagValueSuffixesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32,
	tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int) ([]string, error) {
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
	if err := writeLimit(bc, maxSuffixes); err != nil {
		return nil, fmt.Errorf("cannot send maxSuffixes=%d to conn: %w", maxSuffixes, err)
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

func (sn *storageNode) getTSDBStatusOnConn(bc *handshake.BufferedConn, requestData []byte, focusLabel string, topN int) (*storage.TSDBStatus, error) {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return nil, fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := writeBytes(bc, []byte(focusLabel)); err != nil {
		return nil, fmt.Errorf("cannot write focusLabel=%q: %w", focusLabel, err)
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
	return readTSDBStatus(bc)
}

func readTSDBStatus(bc *handshake.BufferedConn) (*storage.TSDBStatus, error) {
	totalSeries, err := readUint64(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read totalSeries: %w", err)
	}
	totalLabelValuePairs, err := readUint64(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read totalLabelValuePairs: %w", err)
	}
	seriesCountByMetricName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByMetricName: %w", err)
	}
	seriesCountByLabelName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByLabelName: %w", err)
	}
	seriesCountByFocusLabelValue, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByFocusLabelValue: %w", err)
	}
	seriesCountByLabelValuePair, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByLabelValuePair: %w", err)
	}
	labelValueCountByLabelName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read labelValueCountByLabelName: %w", err)
	}
	status := &storage.TSDBStatus{
		TotalSeries:                  totalSeries,
		TotalLabelValuePairs:         totalLabelValuePairs,
		SeriesCountByMetricName:      seriesCountByMetricName,
		SeriesCountByLabelName:       seriesCountByLabelName,
		SeriesCountByFocusLabelValue: seriesCountByFocusLabelValue,
		SeriesCountByLabelValuePair:  seriesCountByLabelValuePair,
		LabelValueCountByLabelName:   labelValueCountByLabelName,
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
		buf, err = readBytes(buf[:0], bc, maxLabelNameSize)
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

func (sn *storageNode) processSearchMetricNamesOnConn(bc *handshake.BufferedConn, requestData []byte) ([]string, error) {
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
	metricNames := make([]string, metricNamesCount)
	for i := int64(0); i < int64(metricNamesCount); i++ {
		buf, err = readBytes(buf[:0], bc, maxMetricNameSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read metricName #%d: %w", i+1, err)
		}
		metricNames[i] = string(buf)
	}
	return metricNames, nil
}

const maxMetricNameSize = 64 * 1024

func (sn *storageNode) processSearchQueryOnConn(bc *handshake.BufferedConn, requestData []byte,
	processBlock func(mb *storage.MetricBlock, workerID uint) error, workerID uint) error {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return fmt.Errorf("cannot write requestData: %w", err)
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
			return fmt.Errorf("cannot unmarshal MetricBlock #%d from %d bytes: %w", blocksRead, len(buf), err)
		}
		if len(tail) != 0 {
			return fmt.Errorf("non-empty tail after unmarshaling MetricBlock #%d: (len=%d) %q", blocksRead, len(tail), tail)
		}
		blocksRead++
		sn.metricBlocksRead.Inc()
		sn.metricRowsRead.Add(mb.Block.RowsCount())
		if err := processBlock(&mb, workerID); err != nil {
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

func writeLimit(bc *handshake.BufferedConn, limit int) error {
	if limit < 0 {
		limit = 0
	}
	if limit > 1<<31-1 {
		limit = 1<<31 - 1
	}
	limitU32 := uint32(limit)
	if err := writeUint32(bc, limitU32); err != nil {
		return fmt.Errorf("cannot write limit=%d to conn: %w", limitU32, err)
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
	buf = bytesutil.ResizeNoCopyMayOverallocate(buf, 8)
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read %d bytes with data size: %w; read only %d bytes", len(buf), err, n)
	}
	dataSize := encoding.UnmarshalUint64(buf)
	if dataSize > uint64(maxDataSize) {
		return buf, fmt.Errorf("too big data size: %d; it mustn't exceed %d bytes", dataSize, maxDataSize)
	}
	buf = bytesutil.ResizeNoCopyMayOverallocate(buf, int(dataSize))
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

type storageNodesBucket struct {
	ms  *metrics.Set
	sns []*storageNode
}

var storageNodes atomic.Pointer[storageNodesBucket]

func getStorageNodesBucket() *storageNodesBucket {
	return storageNodes.Load()
}

func setStorageNodesBucket(snb *storageNodesBucket) {
	storageNodes.Store(snb)
}

func getStorageNodes() []*storageNode {
	snb := getStorageNodesBucket()
	return snb.sns
}

// Init initializes storage nodes' connections to the given addrs.
//
// MustStop must be called when the initialized connections are no longer needed.
func Init(addrs []string) {
	snb := initStorageNodes(addrs)
	setStorageNodesBucket(snb)
}

// MustStop gracefully stops netstorage.
func MustStop() {
	snb := getStorageNodesBucket()
	mustStopStorageNodes(snb)
}

func initStorageNodes(addrs []string) *storageNodesBucket {
	if len(addrs) == 0 {
		logger.Panicf("BUG: addrs must be non-empty")
	}
	var snsLock sync.Mutex
	sns := make([]*storageNode, 0, len(addrs))
	var wg sync.WaitGroup
	ms := metrics.NewSet()
	// initialize connections to storage nodes in parallel in order speed up the initialization
	// for big number of storage nodes.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4364
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			sn := newStorageNode(ms, addr)
			snsLock.Lock()
			sns = append(sns, sn)
			snsLock.Unlock()
		}(addr)
	}
	wg.Wait()
	metrics.RegisterSet(ms)
	return &storageNodesBucket{
		sns: sns,
		ms:  ms,
	}
}

func newStorageNode(ms *metrics.Set, addr string) *storageNode {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// Automatically add missing port.
		addr += ":8401"
	}
	// There is no need in requests compression, since vmselect requests are usually very small.
	connPool := netutil.NewConnPool(ms, "vmselect", addr, handshake.VMSelectClient, 0, *vmstorageDialTimeout, *vmstorageUserTimeout)

	sn := &storageNode{
		connPool: connPool,

		concurrentQueries: ms.NewCounter(fmt.Sprintf(`vm_concurrent_queries{name="vmselect", addr=%q}`, addr)),

		registerMetricNamesRequests: ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="registerMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		registerMetricNamesErrors:   ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="registerMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		deleteSeriesRequests:        ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		deleteSeriesErrors:          ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		labelNamesRequests:          ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		labelNamesErrors:            ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		labelValuesRequests:         ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		labelValuesErrors:           ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tagValueSuffixesRequests:    ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tagValueSuffixesErrors:      ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tsdbStatusRequests:          ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tsdbStatusErrors:            ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		seriesCountRequests:         ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		seriesCountErrors:           ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		searchMetricNamesRequests:   ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="searchMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		searchMetricNamesErrors:     ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="searchMetricNames", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		searchRequests:              ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		searchErrors:                ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tenantsRequests:             ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tenants", type="rpcClient", name="vmselect", addr=%q}`, addr)),
		tenantsErrors:               ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tenants", type="rpcClient", name="vmselect", addr=%q}`, addr)),

		metricBlocksRead: ms.NewCounter(fmt.Sprintf(`vm_metric_blocks_read_total{name="vmselect", addr=%q}`, addr)),
		metricRowsRead:   ms.NewCounter(fmt.Sprintf(`vm_metric_rows_read_total{name="vmselect", addr=%q}`, addr)),
	}
	return sn
}

func mustStopStorageNodes(snb *storageNodesBucket) {
	for _, sn := range snb.sns {
		sn.connPool.MustStop()
	}
	metrics.UnregisterSet(snb.ms)
	snb.ms.UnregisterAllMetrics()
}

var (
	partialLabelNamesResults        = metrics.NewCounter(`vm_partial_results_total{action="labelNames", name="vmselect"}`)
	partialLabelValuesResults       = metrics.NewCounter(`vm_partial_results_total{action="labelValues", name="vmselect"}`)
	partialTagValueSuffixesResults  = metrics.NewCounter(`vm_partial_results_total{action="tagValueSuffixes", name="vmselect"}`)
	partialTSDBStatusResults        = metrics.NewCounter(`vm_partial_results_total{action="tsdbStatus", name="vmselect"}`)
	partialSeriesCountResults       = metrics.NewCounter(`vm_partial_results_total{action="seriesCount", name="vmselect"}`)
	partialSearchMetricNamesResults = metrics.NewCounter(`vm_partial_results_total{action="searchMetricNames", name="vmselect"}`)
	partialSearchResults            = metrics.NewCounter(`vm_partial_results_total{action="search", name="vmselect"}`)
)

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

type uint64WithPadding struct {
	n uint64
	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(uint64(0))%128]byte
}

type perNodeCounter struct {
	ns []uint64WithPadding
}

func newPerNodeCounter(sns []*storageNode) *perNodeCounter {
	return &perNodeCounter{
		ns: make([]uint64WithPadding, len(sns)),
	}
}

func (pnc *perNodeCounter) Add(nodeIdx uint, n uint64) uint64 {
	return atomic.AddUint64(&pnc.ns[nodeIdx].n, n)
}

func (pnc *perNodeCounter) GetTotal() uint64 {
	var total uint64
	for _, n := range pnc.ns {
		total += n.n
	}
	return total
}
