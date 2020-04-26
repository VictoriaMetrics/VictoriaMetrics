package netstorage

import (
	"container/heap"
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
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
	at        *auth.Token
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
				if err = pts.Unpack(rss.tbf, rs, rss.tr, rss.fetchData, rss.at, maxWorkersCount); err != nil {
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
func (pts *packedTimeseries) Unpack(tbf *tmpBlocksFile, dst *Result, tr storage.TimeRange, fetchData bool, at *auth.Token, maxWorkersCount int) error {
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
				if err = sb.unpackFrom(tbf, addr, tr, fetchData, at); err != nil {
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

	timestamps, values := storage.DeduplicateSamples(dst.Timestamps, dst.Values)
	dedups := len(dst.Timestamps) - len(timestamps)
	dedupsDuringSelect.Add(dedups)
	dst.Timestamps = timestamps
	dst.Values = values
}

var dedupsDuringSelect = metrics.NewCounter(`vm_deduplicated_samples_total{type="select"}`)

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

func (sb *sortBlock) unpackFrom(tbf *tmpBlocksFile, addr tmpBlockAddr, tr storage.TimeRange, fetchData bool, at *auth.Token) error {
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

// DeleteSeries deletes time series matching the given sq.
func DeleteSeries(at *auth.Token, sq *storage.SearchQuery, deadline Deadline) (int, error) {
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		deletedCount int
		err          error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.deleteSeriesRequests.Inc()
			deletedCount, err := sn.deleteMetrics(requestData, deadline)
			if err != nil {
				sn.deleteSeriesRequestErrors.Inc()
			}
			resultsCh <- nodeResult{
				deletedCount: deletedCount,
				err:          err,
			}
		}(sn)
	}

	// Collect results
	deletedTotal := 0
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.deleteMetrics must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		deletedTotal += nr.deletedCount
	}
	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		return deletedTotal, fmt.Errorf("error occured during deleting time series: %s", errors[0])
	}
	return deletedTotal, nil
}

// GetLabels returns labels until the given deadline.
func GetLabels(at *auth.Token, deadline Deadline) ([]string, bool, error) {
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labels []string
		err    error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelsRequests.Inc()
			labels, err := sn.getLabels(at.AccountID, at.ProjectID, deadline)
			if err != nil {
				sn.labelsRequestErrors.Inc()
				err = fmt.Errorf("cannot get labels from vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				labels: labels,
				err:    err,
			}
		}(sn)
	}

	// Collect results
	var labels []string
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getLabels must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		labels = append(labels, nr.labels...)
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return nil, true, fmt.Errorf("error occured during fetching labels: %s", errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelsResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching labels: %s", errors[0])
		isPartialResult = true
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

	return labels, isPartialResult, nil
}

// GetLabelValues returns label values for the given labelName
// until the given deadline.
func GetLabelValues(at *auth.Token, labelName string, deadline Deadline) ([]string, bool, error) {
	if labelName == "__name__" {
		labelName = ""
	}

	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelValues []string
		err         error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelValuesRequests.Inc()
			labelValues, err := sn.getLabelValues(at.AccountID, at.ProjectID, labelName, deadline)
			if err != nil {
				sn.labelValuesRequestErrors.Inc()
				err = fmt.Errorf("cannot get label values from vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				labelValues: labelValues,
				err:         err,
			}
		}(sn)
	}

	// Collect results
	var labelValues []string
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getLabelValues must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		labelValues = append(labelValues, nr.labelValues...)
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return nil, true, fmt.Errorf("error occured during fetching label values: %s", errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelValuesResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching label values: %s", errors[0])
		isPartialResult = true
	}

	// Deduplicate label values
	labelValues = deduplicateStrings(labelValues)

	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)

	return labelValues, isPartialResult, nil
}

// GetLabelEntries returns all the label entries for at until the given deadline.
func GetLabelEntries(at *auth.Token, deadline Deadline) ([]storage.TagEntry, bool, error) {
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labelEntries []storage.TagEntry
		err          error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelEntriesRequests.Inc()
			labelEntries, err := sn.getLabelEntries(at.AccountID, at.ProjectID, deadline)
			if err != nil {
				sn.labelEntriesRequestErrors.Inc()
				err = fmt.Errorf("cannot get label entries from vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				labelEntries: labelEntries,
				err:          err,
			}
		}(sn)
	}

	// Collect results
	var labelEntries []storage.TagEntry
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getLabelEntries must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		labelEntries = append(labelEntries, nr.labelEntries...)
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return nil, true, fmt.Errorf("error occured during fetching label entries: %s", errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelEntriesResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching label entries: %s", errors[0])
		isPartialResult = true
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

	return labelEntries, isPartialResult, nil
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
func GetTSDBStatusForDate(at *auth.Token, deadline Deadline, date uint64, topN int) (*storage.TSDBStatus, bool, error) {
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		status *storage.TSDBStatus
		err    error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.tsdbStatusRequests.Inc()
			status, err := sn.getTSDBStatusForDate(at.AccountID, at.ProjectID, date, topN, deadline)
			if err != nil {
				sn.tsdbStatusRequestErrors.Inc()
				err = fmt.Errorf("cannot obtain tsdb status from vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				status: status,
				err:    err,
			}
		}(sn)
	}

	// Collect results.
	var statuses []*storage.TSDBStatus
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getTSDBStatusForDate must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		statuses = append(statuses, nr.status)
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return nil, true, fmt.Errorf("error occured during fetching tsdb stats: %s", errors[0])
		}
		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialTSDBStatusResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching tsdb stats: %s", errors[0])
		isPartialResult = true
	}

	status := mergeTSDBStatuses(statuses, topN)
	return status, isPartialResult, nil
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

// GetSeriesCount returns the number of unique series for the given at.
func GetSeriesCount(at *auth.Token, deadline Deadline) (uint64, bool, error) {
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		n   uint64
		err error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.seriesCountRequests.Inc()
			n, err := sn.getSeriesCount(at.AccountID, at.ProjectID, deadline)
			if err != nil {
				sn.seriesCountRequestErrors.Inc()
				err = fmt.Errorf("cannot get series count from vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				n:   n,
				err: err,
			}
		}(sn)
	}

	// Collect results
	var n uint64
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getSeriesCount must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		n += nr.n
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return 0, true, fmt.Errorf("error occured during fetching series count: %s", errors[0])
		}
		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialSeriesCountResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching series count: %s", errors[0])
		isPartialResult = true
	}

	return n, isPartialResult, nil
}

type tmpBlocksFileWrapper struct {
	mu                 sync.Mutex
	tbf                *tmpBlocksFile
	m                  map[string][]tmpBlockAddr
	orderedMetricNames []string
}

func (tbfw *tmpBlocksFileWrapper) WriteBlock(mb *storage.MetricBlock) error {
	bb := tmpBufPool.Get()
	bb.B = storage.MarshalBlock(bb.B[:0], mb.Block)
	tbfw.mu.Lock()
	addr, err := tbfw.tbf.WriteBlockData(bb.B)
	tmpBufPool.Put(bb)
	if err == nil {
		metricName := mb.MetricName
		addrs := tbfw.m[string(metricName)]
		if len(addrs) == 0 {
			tbfw.orderedMetricNames = append(tbfw.orderedMetricNames, string(metricName))
		}
		tbfw.m[string(metricName)] = append(addrs, addr)
	}
	tbfw.mu.Unlock()
	return err
}

// ProcessSearchQuery performs sq on storage nodes until the given deadline.
func ProcessSearchQuery(at *auth.Token, sq *storage.SearchQuery, fetchData bool, deadline Deadline) (*Results, bool, error) {
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	resultsCh := make(chan error, len(storageNodes))
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	tbfw := &tmpBlocksFileWrapper{
		tbf: getTmpBlocksFile(),
		m:   make(map[string][]tmpBlockAddr),
	}
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.searchRequests.Inc()
			err := sn.processSearchQuery(tbfw, requestData, tr, fetchData, deadline)
			if err != nil {
				sn.searchRequestErrors.Inc()
				err = fmt.Errorf("cannot perform search on vmstorage %s: %s", sn.connPool.Addr(), err)
			}
			resultsCh <- err
		}(sn)
	}

	// Collect results.
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.processSearchQuery must be finished until the deadline.
		err := <-resultsCh
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			putTmpBlocksFile(tbfw.tbf)
			return nil, true, fmt.Errorf("error occured during search: %s", errors[0])
		}

		// Just return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		// Do not log the error, since it may spam logs on busy vmselect
		// serving high amount of requests.
		partialSearchResults.Inc()
		isPartialResult = true
	}
	if err := tbfw.tbf.Finalize(); err != nil {
		putTmpBlocksFile(tbfw.tbf)
		return nil, false, fmt.Errorf("cannot finalize temporary blocks file with %d time series: %s", len(tbfw.m), err)
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
	return &rss, isPartialResult, nil
}

type storageNode struct {
	connPool *netutil.ConnPool

	// The channel for limiting the maximum number of concurrent queries to storageNode.
	concurrentQueriesCh chan struct{}

	// The number of DeleteSeries requests to storageNode.
	deleteSeriesRequests *metrics.Counter

	// The number of DeleteSeries request errors to storageNode.
	deleteSeriesRequestErrors *metrics.Counter

	// The number of requests to labels.
	labelsRequests *metrics.Counter

	// The number of errors during requests to labels.
	labelsRequestErrors *metrics.Counter

	// The number of requests to labelValues.
	labelValuesRequests *metrics.Counter

	// The number of errors during requests to labelValues.
	labelValuesRequestErrors *metrics.Counter

	// The number of requests to labelEntries.
	labelEntriesRequests *metrics.Counter

	// The number of errors during requests to labelEntries.
	labelEntriesRequestErrors *metrics.Counter

	// The number of requests to tsdb status.
	tsdbStatusRequests *metrics.Counter

	// The number of errors during requests to tsdb status.
	tsdbStatusRequestErrors *metrics.Counter

	// The number of requests to seriesCount.
	seriesCountRequests *metrics.Counter

	// The number of errors during requests to seriesCount.
	seriesCountRequestErrors *metrics.Counter

	// The number of search requests to storageNode.
	searchRequests *metrics.Counter

	// The number of search request errors to storageNode.
	searchRequestErrors *metrics.Counter

	// The number of metric blocks read.
	metricBlocksRead *metrics.Counter

	// The number of read metric rows.
	metricRowsRead *metrics.Counter
}

func (sn *storageNode) deleteMetrics(requestData []byte, deadline Deadline) (int, error) {
	var deletedCount int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.deleteMetricsOnConn(bc, requestData)
		if err != nil {
			return err
		}
		deletedCount += n
		return nil
	}
	if err := sn.execOnConn("deleteMetrics_v2", f, deadline); err != nil {
		// Try again before giving up.
		// There is no need in zeroing deletedCount.
		if err = sn.execOnConn("deleteMetrics_v2", f, deadline); err != nil {
			return deletedCount, err
		}
	}
	return deletedCount, nil
}

func (sn *storageNode) getLabels(accountID, projectID uint32, deadline Deadline) ([]string, error) {
	var labels []string
	f := func(bc *handshake.BufferedConn) error {
		ls, err := sn.getLabelsOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		labels = ls
		return nil
	}
	if err := sn.execOnConn("labels", f, deadline); err != nil {
		// Try again before giving up.
		labels = nil
		if err = sn.execOnConn("labels", f, deadline); err != nil {
			return nil, err
		}
	}
	return labels, nil
}

func (sn *storageNode) getLabelValues(accountID, projectID uint32, labelName string, deadline Deadline) ([]string, error) {
	var labelValues []string
	f := func(bc *handshake.BufferedConn) error {
		lvs, err := sn.getLabelValuesOnConn(bc, accountID, projectID, labelName)
		if err != nil {
			return err
		}
		labelValues = lvs
		return nil
	}
	if err := sn.execOnConn("labelValues", f, deadline); err != nil {
		// Try again before giving up.
		labelValues = nil
		if err = sn.execOnConn("labelValues", f, deadline); err != nil {
			return nil, err
		}
	}
	return labelValues, nil
}

func (sn *storageNode) getLabelEntries(accountID, projectID uint32, deadline Deadline) ([]storage.TagEntry, error) {
	var tagEntries []storage.TagEntry
	f := func(bc *handshake.BufferedConn) error {
		tes, err := sn.getLabelEntriesOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		tagEntries = tes
		return nil
	}
	if err := sn.execOnConn("labelEntries", f, deadline); err != nil {
		// Try again before giving up.
		tagEntries = nil
		if err = sn.execOnConn("labelEntries", f, deadline); err != nil {
			return nil, err
		}
	}
	return tagEntries, nil
}

func (sn *storageNode) getTSDBStatusForDate(accountID, projectID uint32, date uint64, topN int, deadline Deadline) (*storage.TSDBStatus, error) {
	var status *storage.TSDBStatus
	f := func(bc *handshake.BufferedConn) error {
		st, err := sn.getTSDBStatusForDateOnConn(bc, accountID, projectID, date, topN)
		if err != nil {
			return err
		}
		status = st
		return nil
	}
	if err := sn.execOnConn("tsdbStatus", f, deadline); err != nil {
		// Try again before giving up.
		status = nil
		if err = sn.execOnConn("tsdbStatus", f, deadline); err != nil {
			return nil, err
		}
	}
	return status, nil
}

func (sn *storageNode) getSeriesCount(accountID, projectID uint32, deadline Deadline) (uint64, error) {
	var n uint64
	f := func(bc *handshake.BufferedConn) error {
		nn, err := sn.getSeriesCountOnConn(bc, accountID, projectID)
		if err != nil {
			return err
		}
		n = nn
		return nil
	}
	if err := sn.execOnConn("seriesCount", f, deadline); err != nil {
		// Try again before giving up.
		n = 0
		if err = sn.execOnConn("seriesCount", f, deadline); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (sn *storageNode) processSearchQuery(tbfw *tmpBlocksFileWrapper, requestData []byte, tr storage.TimeRange, fetchData bool, deadline Deadline) error {
	var blocksRead int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.processSearchQueryOnConn(tbfw, bc, requestData, tr, fetchData)
		if err != nil {
			return err
		}
		blocksRead = n
		return nil
	}
	if err := sn.execOnConn("search_v3", f, deadline); err != nil && blocksRead == 0 {
		// Try again before giving up if zero blocks read on the previous attempt.
		if err = sn.execOnConn("search_v3", f, deadline); err != nil {
			return err
		}
	}
	return nil
}

func (sn *storageNode) execOnConn(rpcName string, f func(bc *handshake.BufferedConn) error, deadline Deadline) error {
	select {
	case sn.concurrentQueriesCh <- struct{}{}:
	default:
		return fmt.Errorf("too many concurrent queries (more than %d)", cap(sn.concurrentQueriesCh))
	}
	defer func() {
		<-sn.concurrentQueriesCh
	}()

	bc, err := sn.connPool.Get()
	if err != nil {
		return fmt.Errorf("cannot obtain connection from a pool: %s", err)
	}
	if err := bc.SetDeadline(deadline.Deadline); err != nil {
		_ = bc.Close()
		logger.Panicf("FATAL: cannot set connection deadline: %s", err)
	}
	if err := writeBytes(bc, []byte(rpcName)); err != nil {
		// Close the connection instead of returning it to the pool,
		// since it may be broken.
		_ = bc.Close()
		return fmt.Errorf("cannot send rpcName=%q to the server: %s", rpcName, err)
	}

	if err := f(bc); err != nil {
		remoteAddr := bc.RemoteAddr()
		if _, ok := err.(*errRemote); ok {
			// Remote error. The connection may be re-used. Return it to the pool.
			sn.connPool.Put(bc)
		} else {
			// Local error.
			// Close the connection instead of returning it to the pool,
			// since it may be broken.
			_ = bc.Close()
		}
		return fmt.Errorf("cannot execute rpcName=%q on vmstorage %q with timeout %s: %s", rpcName, remoteAddr, deadline.String(), err)
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

func (sn *storageNode) deleteMetricsOnConn(bc *handshake.BufferedConn, requestData []byte) (int, error) {
	// Send the request to sn
	if err := writeBytes(bc, requestData); err != nil {
		return 0, fmt.Errorf("cannot send deleteMetrics request to conn: %s", err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush deleteMetrics request to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return 0, &errRemote{msg: string(buf)}
	}

	// Read deletedCount
	deletedCount, err := readUint64(bc)
	if err != nil {
		return 0, fmt.Errorf("cannot read deletedCount value: %s", err)
	}
	return int(deletedCount), nil
}

const maxLabelSize = 16 * 1024 * 1024

func (sn *storageNode) getLabelsOnConn(bc *handshake.BufferedConn, accountID, projectID uint32) ([]string, error) {
	// Send the request to sn.
	if err := writeUint32(bc, accountID); err != nil {
		return nil, fmt.Errorf("cannot send accountID=%d to conn: %s", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return nil, fmt.Errorf("cannot send projectID=%d to conn: %s", projectID, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return nil, &errRemote{msg: string(buf)}
	}

	// Read response
	var labels []string
	for {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read labels: %s", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labels, nil
		}
		labels = append(labels, string(buf))
	}
}

const maxLabelValueSize = 16 * 1024 * 1024

func (sn *storageNode) getLabelValuesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, labelName string) ([]string, error) {
	// Send the request to sn.
	if err := writeUint32(bc, accountID); err != nil {
		return nil, fmt.Errorf("cannot send accountID=%d to conn: %s", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return nil, fmt.Errorf("cannot send projectID=%d to conn: %s", projectID, err)
	}
	if err := writeBytes(bc, []byte(labelName)); err != nil {
		return nil, fmt.Errorf("cannot send labelName=%q to conn: %s", labelName, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush labelName to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return nil, &errRemote{msg: string(buf)}
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
			return nil, buf, fmt.Errorf("cannot read labelValue: %s", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labelValues, buf, nil
		}
		labelValues = append(labelValues, string(buf))
	}
}

func (sn *storageNode) getLabelEntriesOnConn(bc *handshake.BufferedConn, accountID, projectID uint32) ([]storage.TagEntry, error) {
	// Send the request to sn.
	if err := writeUint32(bc, accountID); err != nil {
		return nil, fmt.Errorf("cannot send accountID=%d to conn: %s", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return nil, fmt.Errorf("cannot send projectID=%d to conn: %s", projectID, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush request to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return nil, &errRemote{msg: string(buf)}
	}

	// Read response
	var labelEntries []storage.TagEntry
	for {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read label: %s", err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return labelEntries, nil
		}
		label := string(buf)
		var values []string
		values, buf, err = readLabelValues(buf, bc)
		if err != nil {
			return nil, fmt.Errorf("cannot read values for label %q: %s", label, err)
		}
		labelEntries = append(labelEntries, storage.TagEntry{
			Key:    label,
			Values: values,
		})
	}
}

func (sn *storageNode) getTSDBStatusForDateOnConn(bc *handshake.BufferedConn, accountID, projectID uint32, date uint64, topN int) (*storage.TSDBStatus, error) {
	// Send the request to sn.
	if err := writeUint32(bc, accountID); err != nil {
		return nil, fmt.Errorf("cannot send accountID=%d to conn: %s", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return nil, fmt.Errorf("cannot send projectID=%d to conn: %s", projectID, err)
	}
	// date shouldn't exceed 32 bits, so send it as uint32.
	if err := writeUint32(bc, uint32(date)); err != nil {
		return nil, fmt.Errorf("cannot send date=%d to conn: %s", date, err)
	}
	// topN shouldn't exceed 32 bits, so send it as uint32.
	if err := writeUint32(bc, uint32(topN)); err != nil {
		return nil, fmt.Errorf("cannot send topN=%d to conn: %s", topN, err)
	}
	if err := bc.Flush(); err != nil {
		return nil, fmt.Errorf("cannot flush tsdbStatus args to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return nil, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return nil, &errRemote{msg: string(buf)}
	}

	// Read response
	seriesCountByMetricName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByMetricName: %s", err)
	}
	labelValueCountByLabelName, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read labelValueCountByLabelName: %s", err)
	}
	seriesCountByLabelValuePair, err := readTopHeapEntries(bc)
	if err != nil {
		return nil, fmt.Errorf("cannot read seriesCountByLabelValuePair: %s", err)
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
		return nil, fmt.Errorf("cannot read the number of topHeapEntries: %s", err)
	}
	var a []storage.TopHeapEntry
	var buf []byte
	for i := uint64(0); i < n; i++ {
		buf, err = readBytes(buf[:0], bc, maxLabelSize)
		if err != nil {
			return nil, fmt.Errorf("cannot read label name: %s", err)
		}
		count, err := readUint64(bc)
		if err != nil {
			return nil, fmt.Errorf("cannot read label count: %s", err)
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
	if err := writeUint32(bc, accountID); err != nil {
		return 0, fmt.Errorf("cannot send accountID=%d to conn: %s", accountID, err)
	}
	if err := writeUint32(bc, projectID); err != nil {
		return 0, fmt.Errorf("cannot send projectID=%d to conn: %s", projectID, err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush seriesCount args to conn: %s", err)
	}

	// Read response error.
	buf, err := readBytes(nil, bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return 0, &errRemote{msg: string(buf)}
	}

	// Read response
	n, err := readUint64(bc)
	if err != nil {
		return 0, fmt.Errorf("cannot read series count: %s", err)
	}
	return n, nil
}

// maxMetricBlockSize is the maximum size of serialized MetricBlock.
const maxMetricBlockSize = 1024 * 1024

// maxErrorMessageSize is the maximum size of error message received
// from vmstorage.
const maxErrorMessageSize = 64 * 1024

func (sn *storageNode) processSearchQueryOnConn(tbfw *tmpBlocksFileWrapper, bc *handshake.BufferedConn, requestData []byte, tr storage.TimeRange, fetchData bool) (int, error) {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return 0, fmt.Errorf("cannot write requestData: %s", err)
	}
	if err := writeBool(bc, fetchData); err != nil {
		return 0, fmt.Errorf("cannot write fetchData=%v: %s", fetchData, err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush requestData to conn: %s", err)
	}

	var err error
	var buf []byte

	// Read response error.
	buf, err = readBytes(buf[:0], bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %s", err)
	}
	if len(buf) > 0 {
		return 0, &errRemote{msg: string(buf)}
	}

	// Read response. It may consist of multiple MetricBlocks.
	blocksRead := 0
	for {
		buf, err = readBytes(buf[:0], bc, maxMetricBlockSize)
		if err != nil {
			return blocksRead, fmt.Errorf("cannot read MetricBlock #%d: %s", blocksRead, err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return blocksRead, nil
		}
		var mb storage.MetricBlock
		mb.Block = &storage.Block{}
		tail, err := mb.Unmarshal(buf)
		if err != nil {
			return blocksRead, fmt.Errorf("cannot unmarshal MetricBlock #%d: %s", blocksRead, err)
		}
		if len(tail) != 0 {
			return blocksRead, fmt.Errorf("non-empty tail after unmarshaling MetricBlock #%d: (len=%d) %q", blocksRead, len(tail), tail)
		}
		blocksRead++
		sn.metricBlocksRead.Inc()
		sn.metricRowsRead.Add(mb.Block.RowsCount())
		if err := tbfw.WriteBlock(&mb); err != nil {
			return blocksRead, fmt.Errorf("cannot write MetricBlock #%d to temporary blocks file: %s", blocksRead, err)
		}
	}
}

func writeBytes(bc *handshake.BufferedConn, buf []byte) error {
	sizeBuf := encoding.MarshalUint64(nil, uint64(len(buf)))
	if _, err := bc.Write(sizeBuf); err != nil {
		return err
	}
	if _, err := bc.Write(buf); err != nil {
		return err
	}
	return nil
}

func writeUint32(bc *handshake.BufferedConn, n uint32) error {
	buf := encoding.MarshalUint32(nil, n)
	if _, err := bc.Write(buf); err != nil {
		return err
	}
	return nil
}

func writeBool(bc *handshake.BufferedConn, b bool) error {
	var buf [1]byte
	if b {
		buf[0] = 1
	}
	if _, err := bc.Write(buf[:]); err != nil {
		return err
	}
	return nil
}

func readBytes(buf []byte, bc *handshake.BufferedConn, maxDataSize int) ([]byte, error) {
	buf = bytesutil.Resize(buf, 8)
	if n, err := io.ReadFull(bc, buf); err != nil {
		return buf, fmt.Errorf("cannot read %d bytes with data size: %s; read only %d bytes", len(buf), err, n)
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
		return buf, fmt.Errorf("cannot read data with size %d: %s; read only %d bytes", dataSize, err, n)
	}
	return buf, nil
}

func readUint64(bc *handshake.BufferedConn) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(bc, buf[:]); err != nil {
		return 0, fmt.Errorf("cannot read uint64: %s", err)
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
		sn := &storageNode{
			// There is no need in requests compression, since they are usually very small.
			connPool: netutil.NewConnPool("vmselect", addr, handshake.VMSelectClient, 0),

			concurrentQueriesCh: make(chan struct{}, maxConcurrentQueriesPerStorageNode),

			deleteSeriesRequests:      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			deleteSeriesRequestErrors: metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsRequests:            metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsRequestErrors:       metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesRequests:       metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesRequestErrors:  metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesRequests:      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesRequestErrors: metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusRequests:        metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusRequestErrors:   metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountRequests:       metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountRequestErrors:  metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchRequests:            metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchRequestErrors:       metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			metricBlocksRead:          metrics.NewCounter(fmt.Sprintf(`vm_metric_blocks_read_total{name="vmselect", addr=%q}`, addr)),
			metricRowsRead:            metrics.NewCounter(fmt.Sprintf(`vm_metric_rows_read_total{name="vmselect", addr=%q}`, addr)),
		}
		metrics.NewGauge(fmt.Sprintf(`vm_concurrent_queries{name="vmselect", addr=%q}`, addr), func() float64 {
			return float64(len(sn.concurrentQueriesCh))
		})
		storageNodes = append(storageNodes, sn)
	}
}

// Stop gracefully stops netstorage.
func Stop() {
	// Nothing to do at the moment.
}

var (
	partialLabelsResults       = metrics.NewCounter(`vm_partial_labels_results_total{name="vmselect"}`)
	partialLabelValuesResults  = metrics.NewCounter(`vm_partial_label_values_results_total{name="vmselect"}`)
	partialLabelEntriesResults = metrics.NewCounter(`vm_partial_label_entries_results_total{name="vmselect"}`)
	partialTSDBStatusResults   = metrics.NewCounter(`vm_partial_tsdb_status_results_total{name="vmselect"}`)
	partialSeriesCountResults  = metrics.NewCounter(`vm_partial_series_count_results_total{name="vmselect"}`)
	partialSearchResults       = metrics.NewCounter(`vm_partial_search_results_total{name="vmselect"}`)
)

// The maximum number of concurrent queries per storageNode.
const maxConcurrentQueriesPerStorageNode = 100

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
