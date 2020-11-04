package netstorage

import (
	"container/heap"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
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

var timeseriesWorkCh = make(chan *timeseriesWork, gomaxprocs*16)

type timeseriesWork struct {
	mustStop uint64
	rss      *Results
	pts      *packedTimeseries
	f        func(rs *Result, workerID uint) error
	doneCh   chan error

	rowsProcessed int
}

func init() {
	for i := 0; i < gomaxprocs; i++ {
		go timeseriesWorker(uint(i))
	}
}

func timeseriesWorker(workerID uint) {
	var rs Result
	var rsLastResetTime uint64
	for tsw := range timeseriesWorkCh {
		rss := tsw.rss
		if rss.deadline.Exceeded() {
			tsw.doneCh <- fmt.Errorf("timeout exceeded during query execution: %s", rss.deadline.String())
			continue
		}
		if atomic.LoadUint64(&tsw.mustStop) != 0 {
			tsw.doneCh <- nil
			continue
		}
		if err := tsw.pts.Unpack(rss.tbf, &rs, rss.tr, rss.fetchData, rss.at); err != nil {
			tsw.doneCh <- fmt.Errorf("error during time series unpacking: %w", err)
			continue
		}
		if len(rs.Timestamps) > 0 || !rss.fetchData {
			if err := tsw.f(&rs, workerID); err != nil {
				tsw.doneCh <- err
				continue
			}
		}
		tsw.rowsProcessed = len(rs.Values)
		tsw.doneCh <- nil
		currentTime := fasttime.UnixTimestamp()
		if cap(rs.Values) > 1024*1024 && 4*len(rs.Values) < cap(rs.Values) && currentTime-rsLastResetTime > 10 {
			// Reset rs in order to preseve memory usage after processing big time series with millions of rows.
			rs = Result{}
			rsLastResetTime = currentTime
		}
	}
}

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

	// Feed workers with work.
	tsws := make([]*timeseriesWork, len(rss.packedTimeseries))
	for i := range rss.packedTimeseries {
		tsw := &timeseriesWork{
			rss:    rss,
			pts:    &rss.packedTimeseries[i],
			f:      f,
			doneCh: make(chan error, 1),
		}
		timeseriesWorkCh <- tsw
		tsws[i] = tsw
	}
	seriesProcessedTotal := len(rss.packedTimeseries)
	rss.packedTimeseries = rss.packedTimeseries[:0]

	// Wait until work is complete.
	var firstErr error
	rowsProcessedTotal := 0
	for _, tsw := range tsws {
		if err := <-tsw.doneCh; err != nil && firstErr == nil {
			// Return just the first error, since other errors
			// are likely duplicate the first error.
			firstErr = err
			// Notify all the the tsws that they shouldn't be executed.
			for _, tsw := range tsws {
				atomic.StoreUint64(&tsw.mustStop, 1)
			}
		}
		rowsProcessedTotal += tsw.rowsProcessed
	}

	perQueryRowsProcessed.Update(float64(rowsProcessedTotal))
	perQuerySeriesProcessed.Update(float64(seriesProcessedTotal))
	return firstErr
}

var perQueryRowsProcessed = metrics.NewHistogram(`vm_per_query_rows_processed_count`)
var perQuerySeriesProcessed = metrics.NewHistogram(`vm_per_query_series_processed_count`)

var gomaxprocs = runtime.GOMAXPROCS(-1)

type packedTimeseries struct {
	metricName string
	addrs      []tmpBlockAddr
}

var unpackWorkCh = make(chan *unpackWork, gomaxprocs*128)

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

func init() {
	for i := 0; i < gomaxprocs; i++ {
		go unpackWorker()
	}
}

func unpackWorker() {
	var tmpBlock storage.Block
	for upw := range unpackWorkCh {
		upw.unpack(&tmpBlock)
	}
}

// unpackBatchSize is the maximum number of blocks that may be unpacked at once by a single goroutine.
//
// This batch is needed in order to reduce contention for upackWorkCh in multi-CPU system.
var unpackBatchSize = 8 * runtime.GOMAXPROCS(-1)

// Unpack unpacks pts to dst.
func (pts *packedTimeseries) Unpack(tbf *tmpBlocksFile, dst *Result, tr storage.TimeRange, fetchData bool, at *auth.Token) error {
	dst.reset()
	if err := dst.MetricName.Unmarshal(bytesutil.ToUnsafeBytes(pts.metricName)); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %w", pts.metricName, err)
	}
	if !fetchData {
		// Do not spend resources on data reading and unpacking.
		return nil
	}

	// Feed workers with work
	addrsLen := len(pts.addrs)
	upws := make([]*unpackWork, 0, 1+addrsLen/unpackBatchSize)
	upw := getUnpackWork()
	upw.tbf = tbf
	upw.at = at
	for _, addr := range pts.addrs {
		if len(upw.ws) >= unpackBatchSize {
			unpackWorkCh <- upw
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
	unpackWorkCh <- upw
	upws = append(upws, upw)
	pts.addrs = pts.addrs[:0]

	// Wait until work is complete
	sbs := make([]*sortBlock, 0, addrsLen)
	var firstErr error
	for _, upw := range upws {
		if err := <-upw.doneCh; err != nil && firstErr == nil {
			// Return the first error only, since other errors are likely the same.
			firstErr = err
		}
		if firstErr == nil {
			sbs = append(sbs, upw.sbs...)
		} else {
			for _, sb := range upw.sbs {
				putSortBlock(sb)
			}
		}
		putUnpackWork(upw)
	}
	if firstErr != nil {
		return firstErr
	}
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

// DeleteSeries deletes time series matching the given sq.
func DeleteSeries(at *auth.Token, sq *storage.SearchQuery, deadline searchutils.Deadline) (int, error) {
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
		return deletedTotal, fmt.Errorf("error occured during deleting time series: %w", errors[0])
	}
	return deletedTotal, nil
}

// GetLabelsOnTimeRange returns labels for the given tr until the given deadline.
func GetLabelsOnTimeRange(at *auth.Token, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		labels []string
		err    error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelsOnTimeRangeRequests.Inc()
			labels, err := sn.getLabelsOnTimeRange(at.AccountID, at.ProjectID, tr, deadline)
			if err != nil {
				sn.labelsOnTimeRangeRequestErrors.Inc()
				err = fmt.Errorf("cannot get labels on time range from vmstorage %s: %w", sn.connPool.Addr(), err)
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
		// sn.getLabelsOnTimeRange must be finished until the deadline.
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
			return nil, true, fmt.Errorf("error occured during fetching labels on time range: %w", errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelsOnTimeRangeResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching labels on time range: %s", errors[0])
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

// GetLabels returns labels until the given deadline.
func GetLabels(at *auth.Token, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
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
				err = fmt.Errorf("cannot get labels from vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return nil, true, fmt.Errorf("error occured during fetching labels: %w", errors[0])
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

// GetLabelValuesOnTimeRange returns label values for the given labelName on the given tr
// until the given deadline.
func GetLabelValuesOnTimeRange(at *auth.Token, labelName string, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, bool, error) {
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
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelValuesOnTimeRangeRequests.Inc()
			labelValues, err := sn.getLabelValuesOnTimeRange(at.AccountID, at.ProjectID, labelName, tr, deadline)
			if err != nil {
				sn.labelValuesOnTimeRangeRequestErrors.Inc()
				err = fmt.Errorf("cannot get label values on time range from vmstorage %s: %w", sn.connPool.Addr(), err)
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
		// sn.getLabelValuesOnTimeRange must be finished until the deadline.
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
			return nil, true, fmt.Errorf("error occured during fetching label values on time range: %w", errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelValuesOnTimeRangeResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching label values on time range: %s", errors[0])
		isPartialResult = true
	}

	// Deduplicate label values
	labelValues = deduplicateStrings(labelValues)
	// Sort labelValues like Prometheus does
	sort.Strings(labelValues)
	return labelValues, isPartialResult, nil
}

// GetLabelValues returns label values for the given labelName
// until the given deadline.
func GetLabelValues(at *auth.Token, labelName string, deadline searchutils.Deadline) ([]string, bool, error) {
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
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.labelValuesRequests.Inc()
			labelValues, err := sn.getLabelValues(at.AccountID, at.ProjectID, labelName, deadline)
			if err != nil {
				sn.labelValuesRequestErrors.Inc()
				err = fmt.Errorf("cannot get label values from vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return nil, true, fmt.Errorf("error occured during fetching label values: %w", errors[0])
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

// GetTagValueSuffixes returns tag value suffixes for the given tagKey and the given tagValuePrefix.
//
// It can be used for implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
func GetTagValueSuffixes(at *auth.Token, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, deadline searchutils.Deadline) ([]string, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
	// Send the query to all the storage nodes in parallel.
	type nodeResult struct {
		suffixes []string
		err      error
	}
	resultsCh := make(chan nodeResult, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.tagValueSuffixesRequests.Inc()
			suffixes, err := sn.getTagValueSuffixes(at.AccountID, at.ProjectID, tr, tagKey, tagValuePrefix, delimiter, deadline)
			if err != nil {
				sn.tagValueSuffixesRequestErrors.Inc()
				err = fmt.Errorf("cannot get tag value suffixes for tr=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c from vmstorage %s: %w",
					tr.String(), tagKey, tagValuePrefix, delimiter, sn.connPool.Addr(), err)
			}
			resultsCh <- nodeResult{
				suffixes: suffixes,
				err:      err,
			}
		}(sn)
	}

	// Collect results
	m := make(map[string]struct{})
	var errors []error
	for i := 0; i < len(storageNodes); i++ {
		// There is no need in timer here, since all the goroutines executing
		// sn.getTagValueSuffixes must be finished until the deadline.
		nr := <-resultsCh
		if nr.err != nil {
			errors = append(errors, nr.err)
			continue
		}
		for _, suffix := range nr.suffixes {
			m[suffix] = struct{}{}
		}
	}
	isPartialResult := false
	if len(errors) > 0 {
		if len(errors) == len(storageNodes) {
			// Return only the first error, since it has no sense in returning all errors.
			return nil, true, fmt.Errorf("error occured during fetching tag value suffixes for tr=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c: %w",
				tr.String(), tagKey, tagValuePrefix, delimiter, errors[0])
		}

		// Just log errors and return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		partialLabelEntriesResults.Inc()
		// Log only the first error, since it has no sense in returning all errors.
		logger.Errorf("certain storageNodes are unhealthy when fetching tag value suffixes: %s", errors[0])
		isPartialResult = true
	}
	suffixes := make([]string, 0, len(m))
	for suffix := range m {
		suffixes = append(suffixes, suffix)
	}
	return suffixes, isPartialResult, nil
}

// GetLabelEntries returns all the label entries for at until the given deadline.
func GetLabelEntries(at *auth.Token, deadline searchutils.Deadline) ([]storage.TagEntry, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
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
				err = fmt.Errorf("cannot get label entries from vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return nil, true, fmt.Errorf("error occured during fetching label entries: %w", errors[0])
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
func GetTSDBStatusForDate(at *auth.Token, deadline searchutils.Deadline, date uint64, topN int) (*storage.TSDBStatus, bool, error) {
	if deadline.Exceeded() {
		return nil, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
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
				err = fmt.Errorf("cannot obtain tsdb status from vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return nil, true, fmt.Errorf("error occured during fetching tsdb stats: %w", errors[0])
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
func GetSeriesCount(at *auth.Token, deadline searchutils.Deadline) (uint64, bool, error) {
	if deadline.Exceeded() {
		return 0, false, fmt.Errorf("timeout exceeded before starting the query processing: %s", deadline.String())
	}
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
				err = fmt.Errorf("cannot get series count from vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return 0, true, fmt.Errorf("error occured during fetching series count: %w", errors[0])
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
			// An optimization: avoid memory allocation and copy for already existing metricName key in tbfw.m.
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
// Data processing is immediately stopped if f returns non-nil error.
// It is the responsibility of f to call b.UnmarshalData before reading timestamps and values from the block.
// It is the responsibility of f to filter blocks according to the given tr.
func ExportBlocks(at *auth.Token, sq *storage.SearchQuery, deadline searchutils.Deadline, f func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error) (bool, error) {
	if deadline.Exceeded() {
		return false, fmt.Errorf("timeout exceeded before starting data export: %s", deadline.String())
	}
	tr := storage.TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
	processBlock := func(mb *storage.MetricBlock) error {
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
	isPartialResult, err := processSearchQuery(at, sq, true, processBlock, deadline)
	if err != nil {
		return true, fmt.Errorf("error occured during export: %w", err)
	}
	return isPartialResult, nil
}

// ProcessSearchQuery performs sq until the given deadline.
//
// Results.RunParallel or Results.Cancel must be called on the returned Results.
func ProcessSearchQuery(at *auth.Token, sq *storage.SearchQuery, fetchData bool, deadline searchutils.Deadline) (*Results, bool, error) {
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
	processBlock := func(mb *storage.MetricBlock) error {
		if !fetchData {
			tbfw.RegisterEmptyBlock(mb)
			return nil
		}
		if err := tbfw.RegisterAndWriteBlock(mb); err != nil {
			return fmt.Errorf("cannot write MetricBlock to temporary blocks file: %w", err)
		}
		return nil
	}
	isPartialResult, err := processSearchQuery(at, sq, fetchData, processBlock, deadline)
	if err != nil {
		putTmpBlocksFile(tbfw.tbf)
		return nil, true, fmt.Errorf("error occured during search: %w", err)
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
	return &rss, isPartialResult, nil
}

func processSearchQuery(at *auth.Token, sq *storage.SearchQuery, fetchData bool, processBlock func(mb *storage.MetricBlock) error, deadline searchutils.Deadline) (bool, error) {
	requestData := sq.Marshal(nil)

	// Send the query to all the storage nodes in parallel.
	resultsCh := make(chan error, len(storageNodes))
	for _, sn := range storageNodes {
		go func(sn *storageNode) {
			sn.searchRequests.Inc()
			err := sn.processSearchQuery(requestData, fetchData, processBlock, deadline)
			if err != nil {
				sn.searchRequestErrors.Inc()
				err = fmt.Errorf("cannot perform search on vmstorage %s: %w", sn.connPool.Addr(), err)
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
			return true, errors[0]
		}

		// Just return partial results.
		// This allows gracefully degrade vmselect in the case
		// if certain storageNodes are temporarily unavailable.
		// Do not return the error, since it may spam logs on busy vmselect
		// serving high amount of requests.
		partialSearchResults.Inc()
		isPartialResult = true
	}
	return isPartialResult, nil
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
	labelsOnTimeRangeRequests *metrics.Counter

	// The number of requests to labels.
	labelsRequests *metrics.Counter

	// The number of errors during requests to labels.
	labelsOnTimeRangeRequestErrors *metrics.Counter

	// The number of errors during requests to labels.
	labelsRequestErrors *metrics.Counter

	// The number of requests to labelValuesOnTimeRange.
	labelValuesOnTimeRangeRequests *metrics.Counter

	// The number of requests to labelValues.
	labelValuesRequests *metrics.Counter

	// The number of errors during requests to labelValuesOnTimeRange.
	labelValuesOnTimeRangeRequestErrors *metrics.Counter

	// The number of errors during requests to labelValues.
	labelValuesRequestErrors *metrics.Counter

	// The number of requests to labelEntries.
	labelEntriesRequests *metrics.Counter

	// The number of errors during requests to labelEntries.
	labelEntriesRequestErrors *metrics.Counter

	// The number of requests to tagValueSuffixes.
	tagValueSuffixesRequests *metrics.Counter

	// The number of errors during requests to tagValueSuffixes.
	tagValueSuffixesRequestErrors *metrics.Counter

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

func (sn *storageNode) deleteMetrics(requestData []byte, deadline searchutils.Deadline) (int, error) {
	var deletedCount int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.deleteMetricsOnConn(bc, requestData)
		if err != nil {
			return err
		}
		deletedCount += n
		return nil
	}
	if err := sn.execOnConn("deleteMetrics_v3", f, deadline); err != nil {
		// Try again before giving up.
		// There is no need in zeroing deletedCount.
		if err = sn.execOnConn("deleteMetrics_v3", f, deadline); err != nil {
			return deletedCount, err
		}
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
	if err := sn.execOnConn("labelsOnTimeRange_v1", f, deadline); err != nil {
		// Try again before giving up.
		labels = nil
		if err = sn.execOnConn("labelsOnTimeRange_v1", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("labels_v2", f, deadline); err != nil {
		// Try again before giving up.
		labels = nil
		if err = sn.execOnConn("labels_v2", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("labelValuesOnTimeRange_v1", f, deadline); err != nil {
		// Try again before giving up.
		labelValues = nil
		if err = sn.execOnConn("labelValuesOnTimeRange_v1", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("labelValues_v2", f, deadline); err != nil {
		// Try again before giving up.
		labelValues = nil
		if err = sn.execOnConn("labelValues_v2", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("tagValueSuffixes_v1", f, deadline); err != nil {
		// Try again before giving up.
		suffixes = nil
		if err = sn.execOnConn("tagValueSuffixes_v1", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("labelEntries_v2", f, deadline); err != nil {
		// Try again before giving up.
		tagEntries = nil
		if err = sn.execOnConn("labelEntries_v2", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("tsdbStatus_v2", f, deadline); err != nil {
		// Try again before giving up.
		status = nil
		if err = sn.execOnConn("tsdbStatus_v2", f, deadline); err != nil {
			return nil, err
		}
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
	if err := sn.execOnConn("seriesCount_v2", f, deadline); err != nil {
		// Try again before giving up.
		n = 0
		if err = sn.execOnConn("seriesCount_v2", f, deadline); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (sn *storageNode) processSearchQuery(requestData []byte, fetchData bool, processBlock func(mb *storage.MetricBlock) error, deadline searchutils.Deadline) error {
	var blocksRead int
	f := func(bc *handshake.BufferedConn) error {
		n, err := sn.processSearchQueryOnConn(bc, requestData, fetchData, processBlock)
		if err != nil {
			return err
		}
		blocksRead = n
		return nil
	}
	if err := sn.execOnConn("search_v4", f, deadline); err != nil && blocksRead == 0 {
		// Try again before giving up if zero blocks read on the previous attempt.
		if err = sn.execOnConn("search_v4", f, deadline); err != nil {
			return err
		}
	}
	return nil
}

func (sn *storageNode) execOnConn(rpcName string, f func(bc *handshake.BufferedConn) error, deadline searchutils.Deadline) error {
	select {
	case sn.concurrentQueriesCh <- struct{}{}:
	default:
		return fmt.Errorf("too many concurrent queries (more than %d)", cap(sn.concurrentQueriesCh))
	}
	defer func() {
		<-sn.concurrentQueriesCh
	}()

	d := time.Unix(int64(deadline.Deadline()), 0)
	nowSecs := fasttime.UnixTimestamp()
	currentTime := time.Unix(int64(nowSecs), 0)
	storageTimeout := *searchutils.StorageTimeout
	if storageTimeout > 0 {
		dd := currentTime.Add(storageTimeout)
		if dd.Sub(d) < 0 {
			// Limit the remote deadline to storageTimeout,
			// so slow vmstorage nodes may stop processing the request.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/711 .
			// The local deadline remains the same, so data obtained from
			// the remaining vmstorage nodes could be processed locally.
			d = dd
		}
	}
	timeout := d.Sub(currentTime)
	if timeout <= 0 {
		return fmt.Errorf("request timeout reached: %s or -search.storageTimeout=%s", deadline.String(), storageTimeout.String())
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

func (sn *storageNode) processSearchQueryOnConn(bc *handshake.BufferedConn, requestData []byte, fetchData bool, processBlock func(mb *storage.MetricBlock) error) (int, error) {
	// Send the request to sn.
	if err := writeBytes(bc, requestData); err != nil {
		return 0, fmt.Errorf("cannot write requestData: %w", err)
	}
	if err := writeBool(bc, fetchData); err != nil {
		return 0, fmt.Errorf("cannot write fetchData=%v: %w", fetchData, err)
	}
	if err := bc.Flush(); err != nil {
		return 0, fmt.Errorf("cannot flush requestData to conn: %w", err)
	}

	var err error
	var buf []byte

	// Read response error.
	buf, err = readBytes(buf[:0], bc, maxErrorMessageSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read error message: %w", err)
	}
	if len(buf) > 0 {
		return 0, newErrRemote(buf)
	}

	// Read response. It may consist of multiple MetricBlocks.
	blocksRead := 0
	var mb storage.MetricBlock
	for {
		buf, err = readBytes(buf[:0], bc, maxMetricBlockSize)
		if err != nil {
			return blocksRead, fmt.Errorf("cannot read MetricBlock #%d: %w", blocksRead, err)
		}
		if len(buf) == 0 {
			// Reached the end of the response
			return blocksRead, nil
		}
		tail, err := mb.Unmarshal(buf)
		if err != nil {
			return blocksRead, fmt.Errorf("cannot unmarshal MetricBlock #%d: %w", blocksRead, err)
		}
		if len(tail) != 0 {
			return blocksRead, fmt.Errorf("non-empty tail after unmarshaling MetricBlock #%d: (len=%d) %q", blocksRead, len(tail), tail)
		}
		blocksRead++
		sn.metricBlocksRead.Inc()
		sn.metricRowsRead.Add(mb.Block.RowsCount())
		if err := processBlock(&mb); err != nil {
			return blocksRead, fmt.Errorf("cannot process MetricBlock #%d: %w", blocksRead, err)
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
		sn := &storageNode{
			// There is no need in requests compression, since they are usually very small.
			connPool: netutil.NewConnPool("vmselect", addr, handshake.VMSelectClient, 0),

			concurrentQueriesCh: make(chan struct{}, maxConcurrentQueriesPerStorageNode),

			deleteSeriesRequests:                metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			deleteSeriesRequestErrors:           metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="deleteSeries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsOnTimeRangeRequests:           metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelsOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsRequests:                      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsOnTimeRangeRequestErrors:      metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelsOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelsRequestErrors:                 metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labels", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesOnTimeRangeRequests:      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValuesOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesRequests:                 metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesOnTimeRangeRequestErrors: metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValuesOnTimeRange", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelValuesRequestErrors:            metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValues", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesRequests:                metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			labelEntriesRequestErrors:           metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelEntries", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tagValueSuffixesRequests:            metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tagValueSuffixesRequestErrors:       metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tagValueSuffixes", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusRequests:                  metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			tsdbStatusRequestErrors:             metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatus", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountRequests:                 metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			seriesCountRequestErrors:            metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="seriesCount", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchRequests:                      metrics.NewCounter(fmt.Sprintf(`vm_requests_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			searchRequestErrors:                 metrics.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="search", type="rpcClient", name="vmselect", addr=%q}`, addr)),
			metricBlocksRead:                    metrics.NewCounter(fmt.Sprintf(`vm_metric_blocks_read_total{name="vmselect", addr=%q}`, addr)),
			metricRowsRead:                      metrics.NewCounter(fmt.Sprintf(`vm_metric_rows_read_total{name="vmselect", addr=%q}`, addr)),
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
	partialLabelsOnTimeRangeResults      = metrics.NewCounter(`vm_partial_labels_on_time_range_results_total{name="vmselect"}`)
	partialLabelsResults                 = metrics.NewCounter(`vm_partial_labels_results_total{name="vmselect"}`)
	partialLabelValuesOnTimeRangeResults = metrics.NewCounter(`vm_partial_label_values_on_time_range_results_total{name="vmselect"}`)
	partialLabelValuesResults            = metrics.NewCounter(`vm_partial_label_values_results_total{name="vmselect"}`)
	partialLabelEntriesResults           = metrics.NewCounter(`vm_partial_label_entries_results_total{name="vmselect"}`)
	partialTSDBStatusResults             = metrics.NewCounter(`vm_partial_tsdb_status_results_total{name="vmselect"}`)
	partialSeriesCountResults            = metrics.NewCounter(`vm_partial_series_count_results_total{name="vmselect"}`)
	partialSearchResults                 = metrics.NewCounter(`vm_partial_search_results_total{name="vmselect"}`)
)

// The maximum number of concurrent queries per storageNode.
const maxConcurrentQueriesPerStorageNode = 100
