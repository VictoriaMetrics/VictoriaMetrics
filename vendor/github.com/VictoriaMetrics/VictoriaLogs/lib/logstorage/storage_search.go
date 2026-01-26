package logstorage

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// QueryContext is used for execting the query passed to NewQueryContext()
type QueryContext struct {
	// Context is the context for executing the Query.
	Context context.Context

	// QueryStats is query stats, which is updated after Query execution.
	QueryStats *QueryStats

	// TenantIDs is the list of tenant ids to Query.
	TenantIDs []TenantID

	// Query is the query to execute.
	Query *Query

	// AllowPartialResponse indicates whether to allow partial response. This flag is used only in cluster setup when vlselect queries vlstorage nodes.
	AllowPartialResponse bool

	// HiddenFieldsFilters is an optional list of field filters, which must be hidden during query execution.
	//
	// The list may contain full field names and field prefixes ending with *.
	// Prefix match all the fields starting with the given prefix.
	HiddenFieldsFilters []string

	// startTime is creation time for the QueryContext.
	//
	// It is used for calculating query druation.
	startTime time.Time
}

// NewQueryContext returns new context for the given query.
func NewQueryContext(ctx context.Context, qs *QueryStats, tenantIDs []TenantID, q *Query, allowPartialResponse bool, hiddenFieldsFilters []string) *QueryContext {
	startTime := time.Now()
	return newQueryContext(ctx, qs, tenantIDs, q, allowPartialResponse, hiddenFieldsFilters, startTime)
}

// WithQuery returns new QueryContext with the given q, while preserving other fields from qctx.
func (qctx *QueryContext) WithQuery(q *Query) *QueryContext {
	return newQueryContext(qctx.Context, qctx.QueryStats, qctx.TenantIDs, q, qctx.AllowPartialResponse, qctx.HiddenFieldsFilters, qctx.startTime)
}

// WithContext returns new QueryContext with the given ctx, while preserving other fields from qctx.
func (qctx *QueryContext) WithContext(ctx context.Context) *QueryContext {
	return newQueryContext(ctx, qctx.QueryStats, qctx.TenantIDs, qctx.Query, qctx.AllowPartialResponse, qctx.HiddenFieldsFilters, qctx.startTime)
}

// WithContextAndQuery returns new QueryContext with the given ctx and q, while preserving other fields from qctx.
func (qctx *QueryContext) WithContextAndQuery(ctx context.Context, q *Query) *QueryContext {
	return newQueryContext(ctx, qctx.QueryStats, qctx.TenantIDs, q, qctx.AllowPartialResponse, qctx.HiddenFieldsFilters, qctx.startTime)
}

// QueryDurationNsecs returns the duration in nanoseconds since the NewQueryContext call.
func (qctx *QueryContext) QueryDurationNsecs() int64 {
	return time.Since(qctx.startTime).Nanoseconds()
}

func newQueryContext(ctx context.Context, qs *QueryStats, tenantIDs []TenantID, q *Query, allowPartialResponse bool, hiddenFieldsFilters []string, startTime time.Time) *QueryContext {
	if q.opts.allowPartialResponse != nil {
		// query options override other settings for allowPartialResponse.
		allowPartialResponse = *q.opts.allowPartialResponse
	}

	return &QueryContext{
		Context:    ctx,
		QueryStats: qs,
		TenantIDs:  tenantIDs,
		Query:      q,

		AllowPartialResponse: allowPartialResponse,
		HiddenFieldsFilters:  hiddenFieldsFilters,

		startTime: startTime,
	}
}

// storageSearchOptions contain options used for search in the Storage.
//
// This struct must be created via Storage.getSearchOptions() call.
type storageSearchOptions struct {
	// tenantIDs must contain the list of tenantIDs for the search.
	tenantIDs []TenantID

	// streamIDs is an optional sorted list of streamIDs for the search.
	// If it is empty, then the search is performed by tenantIDs
	streamIDs []streamID

	// minTimestamp is the minimum timestamp for the search
	minTimestamp int64

	// maxTimestamp is the maximum timestamp for the search
	maxTimestamp int64

	// sf is an optional stream filter to use for the search before applying the filter
	streamFilter *StreamFilter

	// filter is the filter to use for the search
	//
	// The streamFilter must be applied before applying the filter
	filter filter

	// fieldsFilter is the filter of fields to return in the result
	fieldsFilter *prefixfilter.Filter

	// hiddenFieldsFilter is the filter of fields, which must be hidden during query
	hiddenFieldsFilter *prefixfilter.Filter

	// timeOffset is the offset in nanoseconds, which must be subtracted from the selected the _time values before these values are passed to query pipes.
	timeOffset int64
}

// partitionSearchOptions is search options for the partition.
//
// this struct must be created via partition.getSearchOptions() call.
type partitionSearchOptions struct {
	// Optional sorted list of tenantIDs for the search.
	// If it is empty, then the search is performed by streamIDs
	tenantIDs []TenantID

	// Optional sorted list of streamIDs for the search.
	// If it is empty, then the search is performed by tenantIDs
	streamIDs []streamID

	// minTimestamp is the minimum timestamp for the search
	minTimestamp int64

	// maxTimestamp is the maximum timestamp for the search
	maxTimestamp int64

	// filter is the filter to use for the search
	filter filter

	// fieldsFilter is the filter of fields to return in the result
	fieldsFilter *prefixfilter.Filter

	// hiddenFieldsFilter is the filter of fields, which must be hidden during query
	hiddenFieldsFilter *prefixfilter.Filter
}

func (pso *partitionSearchOptions) matchStreamID(sid *streamID) bool {
	if len(pso.tenantIDs) > 0 {
		return slices.Contains(pso.tenantIDs, sid.tenantID)
	}
	return slices.Contains(pso.streamIDs, *sid)
}

func (pso *partitionSearchOptions) matchTimeRange(minTimestamp, maxTimestamp int64) bool {
	return minTimestamp <= pso.maxTimestamp && maxTimestamp >= pso.minTimestamp
}

// WriteDataBlockFunc must process the db.
//
// WriteDataBlockFunc cannot hold references to db or any of its fields after the function returns.
// If you need BlockColumn names or values after the function returns, copy them using strings.Clone.
type WriteDataBlockFunc func(workerID uint, db *DataBlock)

func (f WriteDataBlockFunc) newBlockResultWriter() writeBlockResultFunc {
	var dbs atomicutil.Slice[DataBlock]
	return func(workerID uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}
		db := dbs.Get(workerID)
		db.initFromBlockResult(br)
		f(workerID, db)
	}
}

// writeBlockResultFunc must process the br.
//
// writeBlockResultFunc cannot hold references to br after returning.
type writeBlockResultFunc func(workerID uint, br *blockResult)

func (f writeBlockResultFunc) newDataBlockWriter() WriteDataBlockFunc {
	var brs atomicutil.Slice[blockResult]
	return func(workerID uint, db *DataBlock) {
		if db.RowsCount() == 0 {
			return
		}
		br := brs.Get(workerID)
		br.initFromDataBlock(db)
		f(workerID, br)
	}
}

// RunQuery runs the given qctx and calls writeBlock for results.
func (s *Storage) RunQuery(qctx *QueryContext, writeBlock WriteDataBlockFunc) error {
	writeBlockResult := writeBlock.newBlockResultWriter()
	return s.runQuery(qctx, writeBlockResult)
}

// runQueryFunc must run the given qctx and pass query results to writeBlock
type runQueryFunc func(qctx *QueryContext, writeBlock writeBlockResultFunc) error

func (s *Storage) runQuery(qctx *QueryContext, writeBlock writeBlockResultFunc) error {
	qNew, err := initSubqueries(qctx, s.runQuery, true)
	if err != nil {
		return err
	}
	q := qNew

	sso := s.getSearchOptions(qctx.TenantIDs, q, qctx.HiddenFieldsFilters)

	search := func(stopCh <-chan struct{}, writeBlockToPipes writeBlockResultFunc) error {
		workersCount := q.GetParallelReaders(s.defaultParallelReaders)
		s.searchParallel(workersCount, sso, qctx.QueryStats, stopCh, writeBlockToPipes)
		return nil
	}

	concurrency := q.GetConcurrency()
	return runPipes(qctx, q.pipes, search, writeBlock, concurrency)
}

func (s *Storage) getSearchOptions(tenantIDs []TenantID, q *Query, hiddenFieldsFilters []string) *storageSearchOptions {
	streamIDs := q.getStreamIDs()
	sort.Slice(streamIDs, func(i, j int) bool {
		return streamIDs[i].less(&streamIDs[j])
	})

	minTimestamp, maxTimestamp := q.GetFilterTimeRange()
	sf, f := getCommonStreamFilter(q.f)
	fieldsFilter := getNeededColumns(q.pipes)

	var hiddenFieldsFilter *prefixfilter.Filter
	if len(hiddenFieldsFilters) > 0 {
		fieldsFilter.AddDenyFilters(hiddenFieldsFilters)
		var hff prefixfilter.Filter
		hff.AddAllowFilters(hiddenFieldsFilters)
		hiddenFieldsFilter = &hff
	}

	return &storageSearchOptions{
		tenantIDs:          tenantIDs,
		streamIDs:          streamIDs,
		minTimestamp:       minTimestamp,
		maxTimestamp:       maxTimestamp,
		streamFilter:       sf,
		filter:             f,
		fieldsFilter:       fieldsFilter,
		hiddenFieldsFilter: hiddenFieldsFilter,
		timeOffset:         -q.opts.timeOffset,
	}
}

// searchFunc must perform search and pass its results to writeBlock.
type searchFunc func(stopCh <-chan struct{}, writeBlock writeBlockResultFunc) error

func runPipes(qctx *QueryContext, pipes []pipe, search searchFunc, writeBlock writeBlockResultFunc, concurrency int) error {
	ctx, topCancel := context.WithCancel(qctx.Context)
	defer topCancel()

	stopCh := ctx.Done()
	if len(pipes) == 0 {
		// Fast path when there are no pipes
		return search(stopCh, writeBlock)
	}

	pp := newNoopPipeProcessor(stopCh, writeBlock)
	cancels := make([]func(), len(pipes))
	pps := make([]pipeProcessor, len(pipes))

	for i := len(pipes) - 1; i >= 0; i-- {
		p := pipes[i]
		ctxChild, cancel := context.WithCancel(ctx)
		pp = p.newPipeProcessor(concurrency, stopCh, cancel, pp)

		cancels[i] = cancel
		pps[i] = pp

		stopCh = ctxChild.Done()
		ctx = ctxChild
	}

	errSearch := search(stopCh, pp.writeBlock)
	if errSearch != nil {
		// Cancel the whole query in order to free up resources occupied by pipes.
		topCancel()
	}

	var errFlush error
	for i, pp := range pps {
		switch t := pp.(type) {
		case *pipeQueryStatsProcessor:
			t.setQueryStats(qctx.QueryStats, qctx.QueryDurationNsecs())
		case *pipeQueryStatsLocalProcessor:
			t.setQueryStats(qctx.QueryStats, qctx.QueryDurationNsecs())
		}

		if err := pp.flush(); err != nil && errFlush == nil {
			// Cancel the whole query in order to free up resources occupied by the remaining pipes.
			topCancel()

			errFlush = err
		}
		cancel := cancels[i]
		cancel()
	}

	if errSearch != nil {
		return errSearch
	}

	return errFlush
}

// GetFieldNames returns field names for the given qctx.
func (s *Storage) GetFieldNames(qctx *QueryContext) ([]ValueWithHits, error) {
	q := qctx.Query

	pipes := append([]pipe{}, q.pipes...)
	pipeStr := "field_names"
	lex := newLexer(pipeStr, q.timestamp)

	p, err := parsePipeFieldNames(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'field_names' pipe at [%s]: %s", pipeStr, err)
	}
	pf := p.(*pipeFieldNames)
	pf.isFirstPipe = len(pipes) == 0

	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pf)

	qNew := q.cloneShallow()
	qNew.pipes = pipes

	qctxNew := qctx.WithQuery(qNew)
	return s.runValuesWithHitsQuery(qctxNew)
}

func getJoinMapGeneric(qctx *QueryContext, runQuery runQueryFunc, byFields []string, prefix string) (map[string][][]Field, error) {
	// TODO: track memory usage

	m := make(map[string][][]Field)
	var mLock sync.Mutex
	writeBlockResult := func(_ uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}

		cs := br.getColumns()
		columnNames := make([]string, len(cs))
		byValuesIdxs := make([]int, len(cs))
		for i := range cs {
			name := strings.Clone(cs[i].name)
			idx := slices.Index(byFields, name)
			if prefix != "" && idx < 0 {
				name = prefix + name
			}
			columnNames[i] = name
			byValuesIdxs[i] = idx
		}

		byValues := make([]string, len(byFields))
		var tmpBuf []byte

		for rowIdx := 0; rowIdx < br.rowsLen; rowIdx++ {
			fields := make([]Field, 0, len(cs))
			clear(byValues)
			for j := range cs {
				name := columnNames[j]
				v := cs[j].getValueAtRow(br, rowIdx)
				if cIdx := byValuesIdxs[j]; cIdx >= 0 {
					byValues[cIdx] = v
					continue
				}
				if v == "" {
					continue
				}
				value := strings.Clone(v)
				fields = append(fields, Field{
					Name:  name,
					Value: value,
				})
			}

			tmpBuf = marshalStrings(tmpBuf[:0], byValues)
			k := string(tmpBuf)

			mLock.Lock()
			m[k] = append(m[k], fields)
			mLock.Unlock()
		}
	}

	if err := runQuery(qctx, writeBlockResult); err != nil {
		return nil, err
	}

	return m, nil
}

func marshalStrings(dst []byte, a []string) []byte {
	for _, v := range a {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}
	return dst
}

func getFieldValuesGeneric(qctx *QueryContext, runQuery runQueryFunc, fieldName string) ([]string, error) {
	// TODO: track memory usage
	q := qctx.Query

	if !isLastPipeUniq(q.pipes) {
		pipes := append([]pipe{}, q.pipes...)
		quotedFieldName := quoteTokenIfNeeded(fieldName)
		pipeStr := fmt.Sprintf("uniq by (%s)", quotedFieldName)
		lex := newLexer(pipeStr, q.timestamp)

		pu, err := parsePipeUniq(lex)
		if err != nil {
			logger.Panicf("BUG: unexpected error when parsing 'uniq' pipe at [%s]: %s", pipeStr, err)
		}
		if !lex.isEnd() {
			logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
		}
		pipes = append(pipes, pu)

		qNew := q.cloneShallow()
		qNew.pipes = pipes

		qctx = qctx.WithQuery(qNew)
	}

	cpusCount := cgroup.AvailableCPUs()
	valuesPerCPU := make([][]string, cpusCount)
	allocatorsPerCPU := make([]chunkedAllocator, cpusCount)
	writeBlockResult := func(workerID uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}

		cs := br.getColumns()
		if len(cs) != 1 {
			logger.Panicf("BUG: expecting one column; got %d columns", len(cs))
		}

		columnValues := cs[0].getValues(br)

		valuesDst := valuesPerCPU[workerID]
		a := allocatorsPerCPU[workerID]
		for i := range columnValues {
			vCopy := a.cloneString(columnValues[i])
			valuesDst = append(valuesDst, vCopy)
		}
		valuesPerCPU[workerID] = valuesDst
	}

	if err := runQuery(qctx, writeBlockResult); err != nil {
		return nil, err
	}

	valuesLen := 0
	for _, values := range valuesPerCPU {
		valuesLen += len(values)
	}
	valuesAll := make([]string, 0, valuesLen)
	for _, values := range valuesPerCPU {
		valuesAll = append(valuesAll, values...)
	}

	return valuesAll, nil
}

func isLastPipeUniq(pipes []pipe) bool {
	if len(pipes) == 0 {
		return false
	}
	_, ok := pipes[len(pipes)-1].(*pipeUniq)
	return ok
}

// GetFieldValues returns unique values with the number of hits for the given fieldName returned by qctx.
//
// If limit > 0, then up to limit unique values are returned.
func (s *Storage) GetFieldValues(qctx *QueryContext, fieldName string, limit uint64) ([]ValueWithHits, error) {
	q := qctx.Query

	pipes := append([]pipe{}, q.pipes...)
	quotedFieldName := quoteTokenIfNeeded(fieldName)
	pipeStr := fmt.Sprintf("field_values %s limit %d", quotedFieldName, limit)
	lex := newLexer(pipeStr, q.timestamp)

	pu, err := parsePipeFieldValues(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'field_values' pipe at [%s]: %s", pipeStr, err)
	}

	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pu)

	qNew := q.cloneShallow()
	qNew.pipes = pipes

	qctxNew := qctx.WithQuery(qNew)
	return s.runValuesWithHitsQuery(qctxNew)
}

// ValueWithHits contains value and hits.
type ValueWithHits struct {
	Value string
	Hits  uint64
}

// Marshal appends marshaled vh to dst and returns the result
func (vh *ValueWithHits) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(vh.Value))
	dst = encoding.MarshalUint64(dst, vh.Hits)
	return dst
}

// UnmarshalInplace unmarshals vh from src and returns the remaining tail.
//
// vh is valid until src is changed.
func (vh *ValueWithHits) UnmarshalInplace(src []byte) ([]byte, error) {
	srcOrig := src

	value, n := encoding.UnmarshalBytes(src)
	if n <= 0 {
		return srcOrig, fmt.Errorf("cannot unmarshal value")
	}
	src = src[n:]
	vh.Value = bytesutil.ToUnsafeString(value)

	if len(src) < 8 {
		return srcOrig, fmt.Errorf("cannot unmarshal hits")
	}
	vh.Hits = encoding.UnmarshalUint64(src)
	src = src[8:]

	return src, nil
}

func toValuesWithHits(m map[string]*uint64) []ValueWithHits {
	results := make([]ValueWithHits, 0, len(m))
	for k, pHits := range m {
		results = append(results, ValueWithHits{
			Value: k,
			Hits:  *pHits,
		})
	}
	sortValuesWithHits(results)
	return results
}

// GetStreamFieldNames returns stream field names for the given qctx.
func (s *Storage) GetStreamFieldNames(qctx *QueryContext) ([]ValueWithHits, error) {
	streams, err := s.GetStreams(qctx, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	m := make(map[string]*uint64)
	forEachStreamField(streams, func(f Field, hits uint64) {
		pHits := m[f.Name]
		if pHits == nil {
			nameCopy := strings.Clone(f.Name)
			hitsLocal := uint64(0)
			pHits = &hitsLocal
			m[nameCopy] = pHits
		}
		*pHits += hits
	})
	names := toValuesWithHits(m)
	return names, nil
}

// GetStreamFieldValues returns stream field values for the given fieldName and the given qctx.
//
// If limit > 0, then up to limit unique values are returned.
func (s *Storage) GetStreamFieldValues(qctx *QueryContext, fieldName string, limit uint64) ([]ValueWithHits, error) {
	streams, err := s.GetStreams(qctx, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	m := make(map[string]*uint64)
	forEachStreamField(streams, func(f Field, hits uint64) {
		if f.Name != fieldName {
			return
		}
		pHits := m[f.Value]
		if pHits == nil {
			valueCopy := strings.Clone(f.Value)
			hitsLocal := uint64(0)
			pHits = &hitsLocal
			m[valueCopy] = pHits
		}
		*pHits += hits
	})
	values := toValuesWithHits(m)
	if limit > 0 && uint64(len(values)) > limit {
		values = values[:limit]
		resetHits(values)
	}
	return values, nil
}

// GetStreams returns streams from qctx results.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreams(qctx *QueryContext, limit uint64) ([]ValueWithHits, error) {
	return s.GetFieldValues(qctx, "_stream", limit)
}

// GetStreamIDs returns stream_id field values from qctx results.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreamIDs(qctx *QueryContext, limit uint64) ([]ValueWithHits, error) {
	return s.GetFieldValues(qctx, "_stream_id", limit)
}

// GetTenantIDs returns tenantIDs for the given start and end.
func (s *Storage) GetTenantIDs(ctx context.Context, start, end int64) ([]TenantID, error) {
	return s.getTenantIDs(ctx, start, end)
}

func (s *Storage) getTenantIDs(ctx context.Context, start, end int64) ([]TenantID, error) {
	workersCount := cgroup.AvailableCPUs()
	stopCh := ctx.Done()

	tenantIDByWorker := make([][]TenantID, workersCount)

	// spin up workers
	var wg sync.WaitGroup
	workCh := make(chan *partition, workersCount)
	for i := 0; i < workersCount; i++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			for pt := range workCh {
				if needStop(stopCh) {
					// The search has been canceled. Just skip all the scheduled work in order to save CPU time.
					continue
				}
				tenantIDs := pt.idb.searchTenants()
				tenantIDByWorker[workerID] = append(tenantIDByWorker[workerID], tenantIDs...)
			}
		}(uint(i))
	}

	// Select partitions according to the selected time range
	s.partitionsLock.Lock()
	ptws := s.partitions
	minDay := start / nsecsPerDay
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= minDay
	})
	ptws = ptws[n:]
	maxDay := end / nsecsPerDay
	n = sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day > maxDay
	})
	ptws = ptws[:n]

	// Copy the selected partitions, so they don't interfere with s.partitions.
	ptws = append([]*partitionWrapper{}, ptws...)

	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	// Schedule concurrent search across matching partitions.
	for _, ptw := range ptws {
		workCh <- ptw.pt
	}

	// Wait until workers finish their work
	close(workCh)
	wg.Wait()

	// Decrement references to partitions
	for _, ptw := range ptws {
		ptw.decRef()
	}

	uniqTenantIDs := make(map[TenantID]struct{})
	for _, tenantIDs := range tenantIDByWorker {
		for _, tenantID := range tenantIDs {
			uniqTenantIDs[tenantID] = struct{}{}
		}
	}

	tenants := make([]TenantID, 0, len(uniqTenantIDs))
	for k := range uniqTenantIDs {
		tenants = append(tenants, k)
	}

	return tenants, nil
}

func (s *Storage) runValuesWithHitsQuery(qctx *QueryContext) ([]ValueWithHits, error) {
	var results []ValueWithHits
	var resultsLock sync.Mutex
	writeBlockResult := func(_ uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}

		cs := br.getColumns()
		if len(cs) != 2 {
			logger.Panicf("BUG: expecting two columns; got %d columns", len(cs))
		}

		columnValues := cs[0].getValues(br)
		columnHits := cs[1].getValues(br)

		valuesWithHits := make([]ValueWithHits, len(columnValues))
		for i := range columnValues {
			x := &valuesWithHits[i]
			hits, _ := tryParseUint64(columnHits[i])
			x.Value = strings.Clone(columnValues[i])
			x.Hits = hits
		}

		resultsLock.Lock()
		results = append(results, valuesWithHits...)
		resultsLock.Unlock()
	}

	err := s.runQuery(qctx, writeBlockResult)
	if err != nil {
		return nil, err
	}
	sortValuesWithHits(results)

	return results, nil
}

func initSubqueries(qctx *QueryContext, runQuery runQueryFunc, keepInSubquery bool) (*Query, error) {
	getFieldValues := func(q *Query, fieldName string) ([]string, error) {
		qctxLocal := qctx.WithQuery(q)
		return getFieldValuesGeneric(qctxLocal, runQuery, fieldName)
	}
	qNew, err := initFilterInValues(qctx.Query, getFieldValues, keepInSubquery)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize `in` subqueries: %w", err)
	}

	getJoinMap := func(q *Query, byFields []string, prefix string) (map[string][][]Field, error) {
		qctxLocal := qctx.WithQuery(q)
		return getJoinMapGeneric(qctxLocal, runQuery, byFields, prefix)
	}
	qNew, err = initJoinMaps(qNew, getJoinMap)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize `join` subqueries: %w", err)
	}

	runUnionQuery := func(ctx context.Context, q *Query, writeBlock writeBlockResultFunc) error {
		qctxLocal := qctx.WithContextAndQuery(ctx, q)
		return runQuery(qctxLocal, writeBlock)
	}
	qNew = initUnionQueries(qNew, runUnionQuery)

	return initStreamContextPipes(qctx, qNew, runQuery)
}

func initStreamContextPipes(qctx *QueryContext, q *Query, runQuery runQueryFunc) (*Query, error) {
	pipes := q.pipes

	if len(pipes) == 0 {
		return q, nil
	}

	for i := 1; i < len(pipes); i++ {
		p := pipes[i]
		if pc, ok := p.(*pipeStreamContext); ok {
			return nil, fmt.Errorf("[%s] pipe must go after [%s] filter; now it goes after the [%s] pipe", pc, q.f, pipes[i-1])
		}
	}

	if pc, ok := pipes[0].(*pipeStreamContext); ok {
		fieldsFilter := getNeededColumns(pipes)

		pipesNew := append([]pipe{}, pipes...)
		pipesNew[0] = pc.withRunQuery(qctx, runQuery, fieldsFilter)
		qNew := q.cloneShallow()
		qNew.pipes = pipesNew
		return qNew, nil
	}

	return q, nil
}

func initFilterInValues(q *Query, getFieldValues getFieldValuesFunc, keepSubquery bool) (*Query, error) {
	if !hasFilterInWithQueryForFilter(q.f) && !hasFilterInWithQueryForPipes(q.pipes) {
		return q, nil
	}

	var cache inValuesCache
	fNew, err := initFilterInValuesForFilter(&cache, q.f, getFieldValues, keepSubquery)
	if err != nil {
		return nil, err
	}
	pipesNew, err := initFilterInValuesForPipes(&cache, q.pipes, getFieldValues, keepSubquery)
	if err != nil {
		return nil, err
	}

	qNew := q.cloneShallow()
	qNew.f = fNew
	qNew.pipes = pipesNew

	return qNew, nil
}

type inValuesCache struct {
	m map[string][]string
}

type runUnionQueryFunc func(ctx context.Context, q *Query, writeBlock writeBlockResultFunc) error

func initUnionQueries(q *Query, runUnionQuery runUnionQueryFunc) *Query {
	if !hasUnionPipes(q.pipes) {
		return q
	}

	pipesNew := make([]pipe, len(q.pipes))
	for i, p := range q.pipes {
		if pu, ok := p.(*pipeUnion); ok {
			p = pu.initUnionQuery(runUnionQuery)
		}
		pipesNew[i] = p
	}

	qNew := q.cloneShallow()
	qNew.pipes = pipesNew

	return qNew
}

func hasUnionPipes(pipes []pipe) bool {
	for _, p := range pipes {
		if _, ok := p.(*pipeUnion); ok {
			return true
		}
	}
	return false
}

type getJoinMapFunc func(q *Query, byFields []string, prefix string) (map[string][][]Field, error)

func initJoinMaps(q *Query, getJoinMap getJoinMapFunc) (*Query, error) {
	if !hasJoinPipes(q.pipes) {
		return q, nil
	}

	pipesNew := make([]pipe, len(q.pipes))
	for i, p := range q.pipes {
		if pj, ok := p.(*pipeJoin); ok {
			pNew, err := pj.initJoinMap(getJoinMap)
			if err != nil {
				return nil, err
			}
			p = pNew
		}
		pipesNew[i] = p
	}

	qNew := q.cloneShallow()
	qNew.pipes = pipesNew

	return qNew, nil
}

func hasJoinPipes(pipes []pipe) bool {
	for _, p := range pipes {
		if _, ok := p.(*pipeJoin); ok {
			return true
		}
	}
	return false
}

func (iff *ifFilter) visitSubqueries(visitFunc func(q *Query)) {
	if iff != nil {
		visitSubqueriesInFilter(iff.f, visitFunc)
	}
}

func (iff *ifFilter) hasFilterInWithQuery() bool {
	if iff == nil {
		return false
	}
	return hasFilterInWithQueryForFilter(iff.f)
}

func hasFilterInWithQueryForFilter(f filter) bool {
	if f == nil {
		return false
	}
	visitFunc := func(f filter) bool {
		switch t := f.(type) {
		case *filterIn:
			return t.values.q != nil
		case *filterContainsAll:
			return t.values.q != nil
		case *filterContainsAny:
			return t.values.q != nil
		case *filterStreamID:
			return t.q != nil
		default:
			return false
		}
	}
	return visitFilterRecursive(f, visitFunc)
}

func hasFilterInWithQueryForPipes(pipes []pipe) bool {
	for _, p := range pipes {
		if p.hasFilterInWithQuery() {
			return true
		}
	}
	return false
}

type getFieldValuesFunc func(q *Query, fieldName string) ([]string, error)

func (iff *ifFilter) initFilterInValues(cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (*ifFilter, error) {
	if iff == nil {
		return nil, nil
	}

	f, err := initFilterInValuesForFilter(cache, iff.f, getFieldValuesFunc, keepSubquery)
	if err != nil {
		return nil, err
	}

	iffNew := *iff
	iffNew.f = f
	return &iffNew, nil
}

func initFilterInValuesForFilter(cache *inValuesCache, f filter, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) (filter, error) {
	if f == nil {
		return nil, nil
	}

	visitFunc := func(f filter) bool {
		switch t := f.(type) {
		case *filterIn:
			return t.values.q != nil
		case *filterContainsAll:
			return t.values.q != nil
		case *filterContainsAny:
			return t.values.q != nil
		case *filterStreamID:
			return t.q != nil
		default:
			return false
		}
	}
	copyFunc := func(f filter) (filter, error) {
		switch t := f.(type) {
		case *filterIn:
			values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValuesFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
			}

			fiNew := &filterIn{
				fieldName: t.fieldName,
			}
			if keepSubquery {
				fiNew.values.q = t.values.q
			}
			fiNew.values.values = values
			return fiNew, nil
		case *filterContainsAll:
			values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValuesFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
			}

			fiNew := &filterContainsAll{
				fieldName: t.fieldName,
			}
			if keepSubquery {
				fiNew.values.q = t.values.q
			}
			fiNew.values.values = values
			return fiNew, nil
		case *filterContainsAny:
			values, err := getValuesForQuery(t.values.q, t.values.qFieldName, cache, getFieldValuesFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
			}

			fiNew := &filterContainsAny{
				fieldName: t.fieldName,
			}
			if keepSubquery {
				fiNew.values.q = t.values.q
			}
			fiNew.values.values = values
			return fiNew, nil
		case *filterStreamID:
			values, err := getValuesForQuery(t.q, t.qFieldName, cache, getFieldValuesFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
			}

			// convert values to streamID list
			streamIDs := make([]streamID, 0, len(values))
			for _, v := range values {
				var sid streamID
				if sid.tryUnmarshalFromString(v) {
					streamIDs = append(streamIDs, sid)
				}
			}

			fsNew := &filterStreamID{
				streamIDs: streamIDs,
			}
			if keepSubquery {
				fsNew.q = t.q
			}
			return fsNew, nil
		default:
			return f, nil
		}
	}
	return copyFilter(f, visitFunc, copyFunc)
}

func getValuesForQuery(q *Query, qFieldName string, cache *inValuesCache, getFieldValuesFunc getFieldValuesFunc) ([]string, error) {
	qStr := q.String()
	values, ok := cache.m[qStr]
	if ok {
		return values, nil
	}

	vs, err := getFieldValuesFunc(q, qFieldName)
	if err != nil {
		return nil, err
	}
	if cache.m == nil {
		cache.m = make(map[string][]string)
	}
	cache.m[qStr] = vs
	return vs, nil
}

func initFilterInValuesForPipes(cache *inValuesCache, pipes []pipe, getFieldValuesFunc getFieldValuesFunc, keepSubquery bool) ([]pipe, error) {
	pipesNew := make([]pipe, len(pipes))
	for i, p := range pipes {
		pNew, err := p.initFilterInValues(cache, getFieldValuesFunc, keepSubquery)
		if err != nil {
			return nil, err
		}
		pipesNew[i] = pNew
	}
	return pipesNew, nil
}

// BlockColumn is a single column of a block of data
type BlockColumn struct {
	// Name is the column name
	Name string

	// Values is column values
	Values []string
}

// DataBlock is a single block of data
type DataBlock struct {
	// Columns represents columns in the data block.
	Columns []BlockColumn
}

// Reset resets db
func (db *DataBlock) Reset() {
	clear(db.Columns)
	db.Columns = db.Columns[:0]
}

// RowsCount returns the number of rows in db.
func (db *DataBlock) RowsCount() int {
	columns := db.Columns
	if len(columns) > 0 {
		return len(columns[0].Values)
	}
	return 0
}

// GetTimestamps appends _time column values from db to dst and returns the result.
//
// It returns false if db doesn't have _time column or this column has invalid timestamps.
func (db *DataBlock) GetTimestamps(dst []int64) ([]int64, bool) {
	c := db.GetColumnByName("_time")
	if c == nil {
		return dst, false
	}
	return tryParseTimestamps(dst, c.Values)
}

// GetColumnByName returns column with the given name from db.
//
// nil is returned if there is no such column.
func (db *DataBlock) GetColumnByName(name string) *BlockColumn {
	columns := db.Columns
	for i := range columns {
		c := &columns[i]
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Marshal appends marshaled db to dst and returns the result.
func (db *DataBlock) Marshal(dst []byte) []byte {
	rowsCount := db.RowsCount()
	dst = encoding.MarshalVarUint64(dst, uint64(rowsCount))

	columns := db.Columns
	dst = encoding.MarshalVarUint64(dst, uint64(len(columns)))
	for i := range columns {
		c := &columns[i]

		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(c.Name))

		values := c.Values
		if len(values) != rowsCount {
			logger.Panicf("BUG: the column %q must contain %d values; got %d values", c.Name, rowsCount, len(values))
		}
		if areConstValues(values) {
			dst = append(dst, valuesTypeConst)
			dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(values[0]))
		} else {
			dst = append(dst, valuesTypeRegular)
			for _, v := range values {
				dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
			}
		}
	}

	return dst
}

const (
	valuesTypeConst   = byte(0)
	valuesTypeRegular = byte(1)
)

// UnmarshalInplace unmarshals db from src and returns the tail
//
// db is valid until src is changed.
// valuesBuf holds all the values in the unmarshaled db.Columns.
func (db *DataBlock) UnmarshalInplace(src []byte, valuesBuf []string) ([]byte, []string, error) {
	srcOrig := src

	db.Reset()

	// Unmarshal the number of rows
	u64, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return srcOrig, valuesBuf, fmt.Errorf("cannot unmarshal the number of rows from len(src)=%d", len(src))
	}
	if u64 > math.MaxInt {
		return srcOrig, valuesBuf, fmt.Errorf("too big number of rows in block: %d; mustn't exceed %v", u64, math.MaxInt)
	}
	rowsCount := int(u64)
	src = src[n:]

	// Unmarshal the number of columns
	columnsLen, n := encoding.UnmarshalVarUint64(src)
	if n <= 0 {
		return srcOrig, valuesBuf, fmt.Errorf("cannot unmarshal the number of columns from len(src)=%d", len(src))
	}
	if columnsLen > math.MaxInt {
		return srcOrig, valuesBuf, fmt.Errorf("too big number of columns in block: %d; mustn't exceed %v", columnsLen, math.MaxInt)
	}
	src = src[n:]

	// Unmarshal columns
	columns := slicesutil.SetLength(db.Columns, int(columnsLen))
	for i := range columns {
		name, n := encoding.UnmarshalBytes(src)
		if n <= 0 {
			return srcOrig, valuesBuf, fmt.Errorf("cannot unmarshal column name from len(src)=%d", len(src))
		}
		src = src[n:]

		valuesBufLen := len(valuesBuf)
		valuesBuf = slicesutil.SetLength(valuesBuf, valuesBufLen+rowsCount)
		valuesBufA := valuesBuf[valuesBufLen:]

		if len(src) == 0 {
			return srcOrig, valuesBuf, fmt.Errorf("missing value type for column %q", name)
		}
		valuesType := src[0]
		src = src[1:]
		switch valuesType {
		case valuesTypeConst:
			v, n := encoding.UnmarshalBytes(src)
			if n <= 0 {
				return srcOrig, valuesBuf, fmt.Errorf("cannot unmarshal const value for column #%d with name %q from len(src)=%d", len(columns), name, len(src))
			}
			src = src[n:]

			value := bytesutil.ToUnsafeString(v)
			for j := 0; j < rowsCount; j++ {
				valuesBufA[j] = value
			}
		case valuesTypeRegular:
			for j := 0; j < rowsCount; j++ {
				v, n := encoding.UnmarshalBytes(src)
				if n <= 0 {
					return srcOrig, valuesBuf, fmt.Errorf("cannot unmarshal value #%d out of %d values for column #%d with name %q from len(src)=%d",
						j, rowsCount, len(columns), name, len(src))
				}
				src = src[n:]

				valuesBufA[j] = bytesutil.ToUnsafeString(v)
			}
		default:
			return srcOrig, valuesBuf, fmt.Errorf("unexpected valuesType=%d", valuesType)
		}

		columns[i] = BlockColumn{
			Name:   bytesutil.ToUnsafeString(name),
			Values: valuesBufA,
		}
	}
	db.Columns = columns

	return src, valuesBuf, nil
}

func (db *DataBlock) initFromBlockResult(br *blockResult) {
	db.Reset()

	cs := br.getColumns()
	for _, c := range cs {
		values := c.getValues(br)
		db.Columns = append(db.Columns, BlockColumn{
			Name:   c.name,
			Values: values,
		})
	}
}

// search searches for the matching rows according to sso.
//
// It uses workersCount parallel workers for the search and calls writeBlock for each matching block.
func (s *Storage) searchParallel(workersCount int, sso *storageSearchOptions, qs *QueryStats, stopCh <-chan struct{}, writeBlock writeBlockResultFunc) {
	// spin up workers
	var wg sync.WaitGroup
	workCh := make(chan *blockSearchWorkBatch, workersCount)
	for workerID := 0; workerID < workersCount; workerID++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()

			qsLocal := &QueryStats{}
			bs := getBlockSearch()
			bm := getBitmap(0)

			for bswb := range workCh {
				bsws := bswb.bsws
				for i := range bsws {
					bsw := &bsws[i]
					if needStop(stopCh) {
						// The search has been canceled. Just skip all the scheduled work in order to save CPU time.
						bsw.reset()
						continue
					}

					rowsProcessed := bsw.bh.rowsCount

					bs.search(qsLocal, bsw, bm)
					if bs.br.rowsLen > 0 {
						if sso.timeOffset != 0 {
							bs.subTimeOffsetToTimestamps(sso.timeOffset)
						}
						writeBlock(workerID, &bs.br)
					}
					bsw.reset()

					qsLocal.BlocksProcessed++
					qsLocal.RowsProcessed += rowsProcessed
					qsLocal.RowsFound += uint64(bs.br.rowsLen)
				}
				bswb.bsws = bswb.bsws[:0]
				putBlockSearchWorkBatch(bswb)
			}

			putBlockSearch(bs)
			putBitmap(bm)
			qs.UpdateAtomic(qsLocal)

		}(uint(workerID))
	}

	// Select partitions according to the selected time range
	ptws, ptwsDecRef := s.getPartitionsForTimeRange(sso.minTimestamp, sso.maxTimestamp)
	defer ptwsDecRef()

	// Schedule concurrent search across matching partitions.
	psfs := make([]partitionSearchFinalizer, len(ptws))
	var wgSearchers sync.WaitGroup
	for i, ptw := range ptws {
		partitionSearchConcurrencyLimitCh <- struct{}{}
		wgSearchers.Add(1)
		go func(idx int, pt *partition) {
			qsLocal := &QueryStats{}

			psfs[idx] = pt.search(sso, qsLocal, workCh, stopCh)

			qs.UpdateAtomic(qsLocal)

			wgSearchers.Done()
			<-partitionSearchConcurrencyLimitCh
		}(i, ptw.pt)
	}
	wgSearchers.Wait()

	// Wait until workers finish their work
	close(workCh)
	wg.Wait()

	// Finalize partition search
	for _, psf := range psfs {
		psf()
	}
}

// getPartitionsForTimeRange returns partitions covered by [minTimestamp, maxTimestamp] time range.
//
// The caller must call ptwsDecRef when the returned partitions are no longer needed.
func (s *Storage) getPartitionsForTimeRange(minTimestamp, maxTimestamp int64) (ptws []*partitionWrapper, ptwsDecRef func()) {
	s.partitionsLock.Lock()

	// s.partitions are sorted by s.day. Use binary search for finding partitions for the given [minTimestamp, maxTimestamp] time range.
	ptwsTmp := s.partitions
	minDay := minTimestamp / nsecsPerDay
	n := sort.Search(len(ptwsTmp), func(i int) bool {
		return ptwsTmp[i].day >= minDay
	})
	ptwsTmp = ptwsTmp[n:]
	maxDay := maxTimestamp / nsecsPerDay
	n = sort.Search(len(ptwsTmp), func(i int) bool {
		return ptwsTmp[i].day > maxDay
	})
	ptwsTmp = ptwsTmp[:n]

	// Copy the selected partitions, so they don't interfere with s.partitions.
	ptws = append([]*partitionWrapper{}, ptwsTmp...)

	for _, ptw := range ptws {
		ptw.incRef()
	}

	s.partitionsLock.Unlock()

	ptwsDecRef = func() {
		for _, ptw := range ptws {
			ptw.decRef()
		}
	}

	return ptws, ptwsDecRef
}

// partitionSearchConcurrencyLimitCh limits the number of concurrent searches in partition.
//
// This is needed for limiting memory usage under high load.
var partitionSearchConcurrencyLimitCh = make(chan struct{}, cgroup.AvailableCPUs())

type partitionSearchFinalizer func()

func (pt *partition) search(sso *storageSearchOptions, qs *QueryStats, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
	if needStop(stopCh) {
		// Do not spend CPU time on search, since it is already stopped.
		return func() {}
	}

	pso := pt.getSearchOptions(sso)
	return pt.ddb.search(pso, qs, workCh, stopCh)
}

func (pt *partition) getSearchOptions(sso *storageSearchOptions) *partitionSearchOptions {
	tenantIDs := sso.tenantIDs
	var streamIDs []streamID

	if sso.streamFilter != nil {
		streamIDs = pt.idb.searchStreamIDs(tenantIDs, sso.streamFilter)
		if len(sso.streamIDs) > 0 {
			streamIDs = intersectStreamIDs(streamIDs, sso.streamIDs)
		}
		tenantIDs = nil
	} else if len(sso.streamIDs) > 0 {
		streamIDs = getStreamIDsForTenantIDs(sso.streamIDs, tenantIDs)
		tenantIDs = nil
	}

	f := sso.filter
	if hasStreamFilters(f) {
		f = initStreamFilters(sso.tenantIDs, pt.idb, f)
	}
	return &partitionSearchOptions{
		tenantIDs:          tenantIDs,
		streamIDs:          streamIDs,
		minTimestamp:       sso.minTimestamp,
		maxTimestamp:       sso.maxTimestamp,
		filter:             f,
		fieldsFilter:       sso.fieldsFilter,
		hiddenFieldsFilter: sso.hiddenFieldsFilter,
	}
}

func intersectStreamIDs(a, b []streamID) []streamID {
	m := make(map[streamID]struct{}, len(b))
	for _, streamID := range b {
		m[streamID] = struct{}{}
	}

	result := make([]streamID, 0, len(a))
	for _, streamID := range a {
		if _, ok := m[streamID]; ok {
			result = append(result, streamID)
		}
	}
	return result
}

func getStreamIDsForTenantIDs(streamIDs []streamID, tenantIDs []TenantID) []streamID {
	m := make(map[TenantID]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		m[tenantID] = struct{}{}
	}

	result := make([]streamID, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		if _, ok := m[streamID.tenantID]; ok {
			result = append(result, streamID)
		}
	}
	return result
}

func hasStreamFilters(f filter) bool {
	visitFunc := func(f filter) bool {
		_, ok := f.(*filterStream)
		return ok
	}
	return visitFilterRecursive(f, visitFunc)
}

func initStreamFilters(tenantIDs []TenantID, idb *indexdb, f filter) filter {
	visitFunc := func(f filter) bool {
		_, ok := f.(*filterStream)
		return ok
	}
	copyFunc := func(f filter) (filter, error) {
		fs := f.(*filterStream)
		fsNew := &filterStream{
			f:         fs.f,
			tenantIDs: tenantIDs,
			idb:       idb,
		}
		return fsNew, nil
	}
	f, err := copyFilter(f, visitFunc, copyFunc)
	if err != nil {
		logger.Panicf("BUG: unexpected error: %s", err)
	}
	return f
}

func (ddb *datadb) search(pso *partitionSearchOptions, qs *QueryStats, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
	// Select parts with data for the given time range
	pws, pwsDecRef := ddb.getPartsForTimeRange(pso.minTimestamp, pso.maxTimestamp)

	// Apply search to matching parts
	for _, pw := range pws {
		pw.p.search(pso, qs, workCh, stopCh)
	}

	return pwsDecRef
}

// getPartsForTimeRange returns ddb parts for the given time range.
//
// The caller must call pwsDecRef on the returned parts when they are no longer needed.
func (ddb *datadb) getPartsForTimeRange(minTimestamp, maxTimestamp int64) (pws []*partWrapper, pwsDecRef func()) {
	ddb.partsLock.Lock()
	pws = appendPartsInTimeRange(nil, ddb.bigParts, minTimestamp, maxTimestamp)
	pws = appendPartsInTimeRange(pws, ddb.smallParts, minTimestamp, maxTimestamp)
	pws = appendPartsInTimeRange(pws, ddb.inmemoryParts, minTimestamp, maxTimestamp)

	for _, pw := range pws {
		pw.incRef()
	}
	ddb.partsLock.Unlock()

	pwsDecRef = func() {
		for _, pw := range pws {
			pw.decRef()
		}
	}

	return pws, pwsDecRef
}

func (p *part) search(pso *partitionSearchOptions, qs *QueryStats, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	bhss := getBlockHeaders()
	if len(pso.tenantIDs) > 0 {
		p.searchByTenantIDs(pso, qs, bhss, workCh, stopCh)
	} else {
		p.searchByStreamIDs(pso, qs, bhss, workCh, stopCh)
	}
	putBlockHeaders(bhss)
}

func (p *part) hasMatchingRows(pso *partitionSearchOptions, stopCh <-chan struct{}) bool {
	var hasMatch atomic.Bool

	// spin up workers
	var wg sync.WaitGroup
	workersCount := cgroup.AvailableCPUs()
	workCh := make(chan *blockSearchWorkBatch, workersCount)
	for workerID := 0; workerID < workersCount; workerID++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()

			qsLocal := &QueryStats{}
			bs := getBlockSearch()
			bm := getBitmap(0)

			for bswb := range workCh {
				bsws := bswb.bsws
				for i := range bsws {
					bsw := &bsws[i]

					if !hasMatch.Load() && !needStop(stopCh) {
						bs.search(qsLocal, bsw, bm)
						if bs.br.rowsLen > 0 {
							hasMatch.Store(true)
						}
					}

					bsw.reset()
				}
				bswb.bsws = bswb.bsws[:0]
				putBlockSearchWorkBatch(bswb)
			}

			putBlockSearch(bs)
			putBitmap(bm)

		}(uint(workerID))
	}

	// execute the search
	var qs QueryStats
	p.search(pso, &qs, workCh, stopCh)

	// Wait until workers finish their work
	close(workCh)
	wg.Wait()

	return hasMatch.Load()
}

func getBlockHeaders() *blockHeaders {
	v := blockHeadersPool.Get()
	if v == nil {
		return &blockHeaders{}
	}
	return v.(*blockHeaders)
}

func putBlockHeaders(bhss *blockHeaders) {
	bhss.reset()
	blockHeadersPool.Put(bhss)
}

var blockHeadersPool sync.Pool

type blockHeaders struct {
	bhs []blockHeader
}

func (bhss *blockHeaders) reset() {
	bhs := bhss.bhs
	for i := range bhs {
		bhs[i].reset()
	}
	bhss.bhs = bhs[:0]
}

func (p *part) searchByTenantIDs(pso *partitionSearchOptions, qs *QueryStats, bhss *blockHeaders, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	// it is assumed that tenantIDs are sorted
	tenantIDs := pso.tenantIDs

	bswb := getBlockSearchWorkBatch()
	scheduleBlockSearch := func(bh *blockHeader) bool {
		if bswb.appendBlockSearchWork(p, pso, bh) {
			return true
		}
		select {
		case <-stopCh:
			return false
		case workCh <- bswb:
			bswb = getBlockSearchWorkBatch()
			return true
		}
	}

	// it is assumed that ibhs are sorted
	ibhs := p.indexBlockHeaders
	for len(ibhs) > 0 && len(tenantIDs) > 0 {
		if needStop(stopCh) {
			return
		}

		// locate tenantID equal or bigger than the tenantID in ibhs[0]
		tenantID := &tenantIDs[0]
		if tenantID.less(&ibhs[0].streamID.tenantID) {
			tenantID = &ibhs[0].streamID.tenantID
			n := sort.Search(len(tenantIDs), func(i int) bool {
				return !tenantIDs[i].less(tenantID)
			})
			if n == len(tenantIDs) {
				tenantIDs = nil
				break
			}
			tenantID = &tenantIDs[n]
			tenantIDs = tenantIDs[n:]
		}

		// locate indexBlockHeader with equal or bigger tenantID than the given tenantID
		n := 0
		if ibhs[0].streamID.tenantID.less(tenantID) {
			n = sort.Search(len(ibhs), func(i int) bool {
				return !ibhs[i].streamID.tenantID.less(tenantID)
			})
			// The end of ibhs[n-1] may contain blocks for the given tenantID, so move it backwards
			n--
		}
		ibh := &ibhs[n]
		ibhs = ibhs[n+1:]

		if pso.minTimestamp > ibh.maxTimestamp || pso.maxTimestamp < ibh.minTimestamp {
			// Skip the ibh, since it doesn't contain entries on the requested time range
			continue
		}

		bhss.bhs = ibh.mustReadBlockHeaders(bhss.bhs[:0], p, qs)

		bhs := bhss.bhs
		for len(bhs) > 0 {
			// search for blocks with the given tenantID
			n = sort.Search(len(bhs), func(i int) bool {
				return !bhs[i].streamID.tenantID.less(tenantID)
			})
			bhs = bhs[n:]
			for len(bhs) > 0 && bhs[0].streamID.tenantID.Equal(tenantID) {
				bh := &bhs[0]
				bhs = bhs[1:]
				th := &bh.timestampsHeader
				if pso.minTimestamp > th.maxTimestamp || pso.maxTimestamp < th.minTimestamp {
					continue
				}
				if !scheduleBlockSearch(bh) {
					return
				}
			}
			if len(bhs) == 0 {
				break
			}

			// search for the next tenantID, which can potentially match tenantID from bhs[0]
			tenantID = &bhs[0].streamID.tenantID
			n = sort.Search(len(tenantIDs), func(i int) bool {
				return !tenantIDs[i].less(tenantID)
			})
			if n == len(tenantIDs) {
				tenantIDs = nil
				break
			}
			tenantID = &tenantIDs[n]
			tenantIDs = tenantIDs[n:]
		}
	}

	// Flush the remaining work
	select {
	case <-stopCh:
	case workCh <- bswb:
	}
}

func (p *part) searchByStreamIDs(pso *partitionSearchOptions, qs *QueryStats, bhss *blockHeaders, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	// it is assumed that streamIDs are sorted
	streamIDs := pso.streamIDs

	bswb := getBlockSearchWorkBatch()
	scheduleBlockSearch := func(bh *blockHeader) bool {
		if bswb.appendBlockSearchWork(p, pso, bh) {
			return true
		}
		select {
		case <-stopCh:
			return false
		case workCh <- bswb:
			bswb = getBlockSearchWorkBatch()
			return true
		}
	}

	// it is assumed that ibhs are sorted
	ibhs := p.indexBlockHeaders

	for len(ibhs) > 0 && len(streamIDs) > 0 {
		if needStop(stopCh) {
			return
		}

		// locate streamID equal or bigger than the streamID in ibhs[0]
		streamID := &streamIDs[0]
		if streamID.less(&ibhs[0].streamID) {
			streamID = &ibhs[0].streamID
			n := sort.Search(len(streamIDs), func(i int) bool {
				return !streamIDs[i].less(streamID)
			})
			if n == len(streamIDs) {
				streamIDs = nil
				break
			}
			streamID = &streamIDs[n]
			streamIDs = streamIDs[n:]
		}

		// locate indexBlockHeader with equal or bigger streamID than the given streamID
		n := 0
		if ibhs[0].streamID.less(streamID) {
			n = sort.Search(len(ibhs), func(i int) bool {
				return !ibhs[i].streamID.less(streamID)
			})
			// The end of ibhs[n-1] may contain blocks for the given streamID, so move it backwards.
			n--
		}
		ibh := &ibhs[n]
		ibhs = ibhs[n+1:]

		if pso.minTimestamp > ibh.maxTimestamp || pso.maxTimestamp < ibh.minTimestamp {
			// Skip the ibh, since it doesn't contain entries on the requested time range
			continue
		}

		bhss.bhs = ibh.mustReadBlockHeaders(bhss.bhs[:0], p, qs)

		bhs := bhss.bhs
		for len(bhs) > 0 {
			// search for blocks with the given streamID
			n = sort.Search(len(bhs), func(i int) bool {
				return !bhs[i].streamID.less(streamID)
			})
			bhs = bhs[n:]
			for len(bhs) > 0 && bhs[0].streamID.equal(streamID) {
				bh := &bhs[0]
				bhs = bhs[1:]
				th := &bh.timestampsHeader
				if pso.minTimestamp > th.maxTimestamp || pso.maxTimestamp < th.minTimestamp {
					continue
				}
				if !scheduleBlockSearch(bh) {
					return
				}
			}
			if len(bhs) == 0 {
				break
			}

			// search for the next streamID, which can potentially match streamID from bhs[0]
			streamID = &bhs[0].streamID
			n = sort.Search(len(streamIDs), func(i int) bool {
				return !streamIDs[i].less(streamID)
			})
			if n == len(streamIDs) {
				streamIDs = nil
				break
			}
			streamID = &streamIDs[n]
			streamIDs = streamIDs[n:]
		}
	}

	// Flush the remaining work
	select {
	case <-stopCh:
	case workCh <- bswb:
	}
}

func appendPartsInTimeRange(dst, src []*partWrapper, minTimestamp, maxTimestamp int64) []*partWrapper {
	for _, pw := range src {
		if maxTimestamp < pw.p.ph.MinTimestamp || minTimestamp > pw.p.ph.MaxTimestamp {
			continue
		}
		dst = append(dst, pw)
	}
	return dst
}

func getCommonStreamFilter(f filter) (*StreamFilter, filter) {
	switch t := f.(type) {
	case *filterAnd:
		filters := t.filters
		for i, filter := range filters {
			sf, ok := filter.(*filterStream)
			if ok && !sf.f.isEmpty() {
				// Remove sf from filters, since it doesn't filter out anything then.
				fa := &filterAnd{
					filters: append(filters[:i:i], filters[i+1:]...),
				}
				return sf.f, fa
			}
		}
	case *filterStream:
		return t.f, &filterNoop{}
	}
	return nil, f
}

func forEachStreamField(streams []ValueWithHits, f func(f Field, hits uint64)) {
	var fields []Field
	for i := range streams {
		var err error
		fields, err = parseStreamFields(fields[:0], streams[i].Value)
		if err != nil {
			continue
		}
		hits := streams[i].Hits
		for j := range fields {
			f(fields[j], hits)
		}
	}
}

func parseStreamFields(dst []Field, s string) ([]Field, error) {
	if len(s) == 0 || s[0] != '{' {
		return dst, fmt.Errorf("missing '{' at the beginning of stream name")
	}
	s = s[1:]
	if len(s) == 0 || s[len(s)-1] != '}' {
		return dst, fmt.Errorf("missing '}' at the end of stream name")
	}
	s = s[:len(s)-1]
	if len(s) == 0 {
		return dst, nil
	}

	for {
		n := strings.Index(s, `="`)
		if n < 0 {
			return dst, fmt.Errorf("cannot find field value in double quotes at [%s]", s)
		}
		name := s[:n]
		s = s[n+1:]

		value, nOffset := tryUnquoteString(s, "")
		if nOffset < 0 {
			return dst, fmt.Errorf("cannot find parse field value in double quotes at [%s]", s)
		}
		s = s[nOffset:]

		dst = append(dst, Field{
			Name:  name,
			Value: value,
		})

		if len(s) == 0 {
			return dst, nil
		}
		if s[0] != ',' {
			return dst, fmt.Errorf("missing ',' after %s=%q", name, value)
		}
		s = s[1:]
	}
}
