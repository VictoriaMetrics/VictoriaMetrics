package netstorage

import (
	"container/heap"
	"flag"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxTagKeysPerSearch   = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned per search")
	maxTagValuesPerSearch = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned per search")
	maxMetricsPerSearch   = flag.Int("search.maxUniqueTimeseries", 300e3, "The maximum number of unique time series each search can scan")
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

	// Marshaled MetricName. Used only for results sorting
	// in app/vmselect/promql
	MetricNameMarshaled []byte
}

func (r *Result) reset() {
	r.MetricName.Reset()
	r.Values = r.Values[:0]
	r.Timestamps = r.Timestamps[:0]
	r.MetricNameMarshaled = r.MetricNameMarshaled[:0]
}

// Results holds results returned from ProcessSearchQuery.
type Results struct {
	tr        storage.TimeRange
	fetchData bool
	deadline  Deadline

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

// RunParallel runs in parallel f for all the results from rss.
//
// f shouldn't hold references to rs after returning.
// workerID is the id of the worker goroutine that calls f.
//
// rss becomes unusable after the call to RunParallel.
func (rss *Results) RunParallel(f func(rs *Result, workerID uint)) error {
	defer func() {
		putTmpBlocksFile(rss.tbf)
		rss.tbf = nil
	}()

	workersCount := 1 + len(rss.packedTimeseries)/32
	if workersCount > gomaxprocs {
		workersCount = gomaxprocs
	}
	if workersCount == 0 {
		logger.Panicf("BUG: workersCount cannot be zero")
	}
	workCh := make(chan *packedTimeseries, workersCount)
	doneCh := make(chan error)

	// Start workers.
	rowsProcessedTotal := uint64(0)
	for i := 0; i < workersCount; i++ {
		go func(workerID uint) {
			rs := getResult()
			defer putResult(rs)
			maxWorkersCount := gomaxprocs / workersCount

			var err error
			rowsProcessed := 0
			for pts := range workCh {
				if time.Until(rss.deadline.Deadline) < 0 {
					err = fmt.Errorf("timeout exceeded during query execution: %s", rss.deadline.String())
					break
				}
				if err = pts.Unpack(rss.tbf, rs, rss.tr, rss.fetchData, maxWorkersCount); err != nil {
					break
				}
				if len(rs.Timestamps) == 0 && rss.fetchData {
					// Skip empty blocks.
					continue
				}
				rowsProcessed += len(rs.Values)
				f(rs, workerID)
			}
			atomic.AddUint64(&rowsProcessedTotal, uint64(rowsProcessed))
			// Drain the remaining work
			for range workCh {
			}
			doneCh <- err
		}(uint(i))
	}

	// Feed workers with work.
	for i := range rss.packedTimeseries {
		workCh <- &rss.packedTimeseries[i]
	}
	seriesProcessedTotal := len(rss.packedTimeseries)
	rss.packedTimeseries = rss.packedTimeseries[:0]
	close(workCh)

	// Wait until workers finish.
	var errors []error
	for i := 0; i < workersCount; i++ {
		if err := <-doneCh; err != nil {
			errors = append(errors, err)
		}
	}
	perQueryRowsProcessed.Update(float64(rowsProcessedTotal))
	perQuerySeriesProcessed.Update(float64(seriesProcessedTotal))
	if len(errors) > 0 {
		// Return just the first error, since other errors
		// is likely duplicate the first error.
		return errors[0]
	}
	return nil
}

var perQueryRowsProcessed = metrics.NewHistogram(`vm_per_query_rows_processed_count`)
var perQuerySeriesProcessed = metrics.NewHistogram(`vm_per_query_series_processed_count`)

var gomaxprocs = runtime.GOMAXPROCS(-1)

type packedTimeseries struct {
	metricName string
	addrs      []tmpBlockAddr
}

// Unpack unpacks pts to dst.
func (pts *packedTimeseries) Unpack(tbf *tmpBlocksFile, dst *Result, tr storage.TimeRange, fetchData bool, maxWorkersCount int) error {
	dst.reset()

	if err := dst.MetricName.Unmarshal(bytesutil.ToUnsafeBytes(pts.metricName)); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %s", pts.metricName, err)
	}

	workersCount := 1 + len(pts.addrs)/32
	if workersCount > maxWorkersCount {
		workersCount = maxWorkersCount
	}
	if workersCount == 0 {
		logger.Panicf("BUG: workersCount cannot be zero")
	}

	sbs := make([]*sortBlock, 0, len(pts.addrs))
	var sbsLock sync.Mutex

	workCh := make(chan tmpBlockAddr, workersCount)
	doneCh := make(chan error)

	// Start workers
	for i := 0; i < workersCount; i++ {
		go func() {
			var err error
			for addr := range workCh {
				sb := getSortBlock()
				if err = sb.unpackFrom(tbf, addr, tr, fetchData); err != nil {
					break
				}

				sbsLock.Lock()
				sbs = append(sbs, sb)
				sbsLock.Unlock()
			}

			// Drain the remaining work
			for range workCh {
			}
			doneCh <- err
		}()
	}

	// Feed workers with work
	for _, addr := range pts.addrs {
		workCh <- addr
	}
	pts.addrs = pts.addrs[:0]
	close(workCh)

	// Wait until workers finish
	var errors []error
	for i := 0; i < workersCount; i++ {
		if err := <-doneCh; err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		// Return the first error only, since other errors are likely the same.
		return errors[0]
	}

	// Merge blocks
	mergeSortBlocks(dst, sbs)
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

func mergeSortBlocks(dst *Result, sbh sortBlocksHeap) {
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

	dst.Timestamps, dst.Values = storage.DeduplicateSamples(dst.Timestamps, dst.Values)
}

type sortBlock struct {
	// b is used as a temporary storage for unpacked rows before they
	// go to Timestamps and Values.
	b storage.Block

	Timestamps []int64
	Values     []float64
	NextIdx    int
}

func (sb *sortBlock) reset() {
	sb.b.Reset()
	sb.Timestamps = sb.Timestamps[:0]
	sb.Values = sb.Values[:0]
	sb.NextIdx = 0
}

func (sb *sortBlock) unpackFrom(tbf *tmpBlocksFile, addr tmpBlockAddr, tr storage.TimeRange, fetchData bool) error {
	tbf.MustReadBlockAt(&sb.b, addr)
	if fetchData {
		if err := sb.b.UnmarshalData(); err != nil {
			return fmt.Errorf("cannot unmarshal block: %s", err)
		}
	}
	timestamps := sb.b.Timestamps()

	// Skip timestamps smaller than tr.MinTimestamp.
	i := 0
	for i < len(timestamps) && timestamps[i] < tr.MinTimestamp {
		i++
	}

	// Skip timestamps bigger than tr.MaxTimestamp.
	j := len(timestamps)
	for j > i && timestamps[j-1] > tr.MaxTimestamp {
		j--
	}
	skippedRows := sb.b.RowsCount() - (j - i)
	metricRowsSkipped.Add(skippedRows)

	// Copy the remaining values.
	if i == j {
		return nil
	}
	values := sb.b.Values()
	sb.Timestamps = append(sb.Timestamps, timestamps[i:j]...)
	sb.Values = decimal.AppendDecimalToFloat(sb.Values, values[i:j], sb.b.Scale())
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
func DeleteSeries(sq *storage.SearchQuery) (int, error) {
	tfss, err := setupTfss(sq.TagFilterss)
	if err != nil {
		return 0, err
	}
	return vmstorage.DeleteMetrics(tfss)
}

// GetLabels returns labels until the given deadline.
func GetLabels(deadline Deadline) ([]string, error) {
	labels, err := vmstorage.SearchTagKeys(*maxTagKeysPerSearch)
	if err != nil {
		return nil, fmt.Errorf("error during labels search: %s", err)
	}

	// Substitute "" with "__name__"
	for i := range labels {
		if labels[i] == "" {
			labels[i] = "__name__"
		}
	}

	// Sort labels like Prometheus does
	sort.Strings(labels)

	return labels, nil
}

// GetLabelValues returns label values for the given labelName
// until the given deadline.
func GetLabelValues(labelName string, deadline Deadline) ([]string, error) {
	if labelName == "__name__" {
		labelName = ""
	}

	// Search for tag values
	labelValues, err := vmstorage.SearchTagValues([]byte(labelName), *maxTagValuesPerSearch)
	if err != nil {
		return nil, fmt.Errorf("error during label values search for labelName=%q: %s", labelName, err)
	}

	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)

	return labelValues, nil
}

// GetLabelEntries returns all the label entries until the given deadline.
func GetLabelEntries(deadline Deadline) ([]storage.TagEntry, error) {
	labelEntries, err := vmstorage.SearchTagEntries(*maxTagKeysPerSearch, *maxTagValuesPerSearch)
	if err != nil {
		return nil, fmt.Errorf("error during label entries request: %s", err)
	}

	// Substitute "" with "__name__"
	for i := range labelEntries {
		e := &labelEntries[i]
		if e.Key == "" {
			e.Key = "__name__"
		}
	}

	// Sort labelEntries by the number of label values in each entry.
	sort.Slice(labelEntries, func(i, j int) bool {
		a, b := labelEntries[i].Values, labelEntries[j].Values
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return labelEntries[i].Key > labelEntries[j].Key
	})

	return labelEntries, nil
}

// GetSeriesCount returns the number of unique series.
func GetSeriesCount(deadline Deadline) (uint64, error) {
	n, err := vmstorage.GetSeriesCount()
	if err != nil {
		return 0, fmt.Errorf("error during series count request: %s", err)
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

// ProcessSearchQuery performs sq on storage nodes until the given deadline.
func ProcessSearchQuery(sq *storage.SearchQuery, fetchData bool, deadline Deadline) (*Results, error) {
	// Setup search.
	tfss, err := setupTfss(sq.TagFilterss)
	if err != nil {
		return nil, err
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}

	vmstorage.WG.Add(1)
	defer vmstorage.WG.Done()

	sr := getStorageSearch()
	defer putStorageSearch(sr)
	sr.Init(vmstorage.Storage, tfss, tr, fetchData, *maxMetricsPerSearch)

	tbf := getTmpBlocksFile()
	m := make(map[string][]tmpBlockAddr)
	blocksRead := 0
	bb := tmpBufPool.Get()
	defer tmpBufPool.Put(bb)
	for sr.NextMetricBlock() {
		blocksRead++
		bb.B = storage.MarshalBlock(bb.B[:0], sr.MetricBlock.Block)
		addr, err := tbf.WriteBlockData(bb.B)
		if err != nil {
			putTmpBlocksFile(tbf)
			return nil, fmt.Errorf("cannot write data block #%d to temporary blocks file: %s", blocksRead, err)
		}
		if time.Until(deadline.Deadline) < 0 {
			putTmpBlocksFile(tbf)
			return nil, fmt.Errorf("timeout exceeded while fetching data block #%d from storage: %s", blocksRead, deadline.String())
		}
		metricName := sr.MetricBlock.MetricName
		m[string(metricName)] = append(m[string(metricName)], addr)
	}
	if err := sr.Error(); err != nil {
		putTmpBlocksFile(tbf)
		return nil, fmt.Errorf("search error after reading %d data blocks: %s", blocksRead, err)
	}
	if err := tbf.Finalize(); err != nil {
		putTmpBlocksFile(tbf)
		return nil, fmt.Errorf("cannot finalize temporary blocks file with %d blocks: %s", blocksRead, err)
	}

	var rss Results
	rss.packedTimeseries = make([]packedTimeseries, len(m))
	rss.tr = tr
	rss.fetchData = fetchData
	rss.deadline = deadline
	rss.tbf = tbf
	i := 0
	for metricName, addrs := range m {
		pts := &rss.packedTimeseries[i]
		i++
		pts.metricName = metricName
		pts.addrs = addrs
	}

	// Sort rss.packedTimeseries by the first addr offset in order
	// to reduce the number of disk seeks during unpacking in RunParallel.
	// In this case tmpBlocksFile must be read almost sequentially.
	sort.Slice(rss.packedTimeseries, func(i, j int) bool {
		pts := rss.packedTimeseries
		return pts[i].addrs[0].offset < pts[j].addrs[0].offset
	})

	return &rss, nil
}

func getResult() *Result {
	v := rsPool.Get()
	if v == nil {
		return &Result{}
	}
	return v.(*Result)
}

func putResult(rs *Result) {
	if len(rs.Values) > 8192 {
		// Do not pool big results, since they may occupy too much memory.
		return
	}
	rs.reset()
	rsPool.Put(rs)
}

var rsPool sync.Pool

func setupTfss(tagFilterss [][]storage.TagFilter) ([]*storage.TagFilters, error) {
	tfss := make([]*storage.TagFilters, 0, len(tagFilterss))
	for _, tagFilters := range tagFilterss {
		tfs := storage.NewTagFilters()
		for i := range tagFilters {
			tf := &tagFilters[i]
			if err := tfs.Add(tf.Key, tf.Value, tf.IsNegative, tf.IsRegexp); err != nil {
				return nil, fmt.Errorf("cannot parse tag filter %s: %s", tf, err)
			}
		}
		tfss = append(tfss, tfs)
	}
	return tfss, nil
}

// Deadline contains deadline with the corresponding timeout for pretty error messages.
type Deadline struct {
	Deadline time.Time

	timeout  time.Duration
	flagHint string
}

// NewDeadline returns deadline for the given timeout.
//
// flagHint must contain a hit for command-line flag, which could be used
// in order to increase timeout.
func NewDeadline(timeout time.Duration, flagHint string) Deadline {
	return Deadline{
		Deadline: time.Now().Add(timeout),
		timeout:  timeout,
		flagHint: flagHint,
	}
}

// String returns human-readable string representation for d.
func (d *Deadline) String() string {
	return fmt.Sprintf("%.3f seconds; the timeout can be adjusted with `%s` command-line flag", d.timeout.Seconds(), d.flagHint)
}
