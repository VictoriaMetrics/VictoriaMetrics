package logstorage

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// genericSearchOptions contain options used for search.
type genericSearchOptions struct {
	// tenantIDs must contain the list of tenantIDs for the search.
	tenantIDs []TenantID

	// streamIDs is an optional sorted list of streamIDs for the search.
	// If it is empty, then the search is performed by tenantIDs
	streamIDs []streamID

	// minTimestamp is the minimum timestamp for the search
	minTimestamp int64

	// maxTimestamp is the maximum timestamp for the search
	maxTimestamp int64

	// filter is the filter to use for the search
	filter filter

	// neededColumnNames contains names of columns to return in the result
	neededColumnNames []string

	// unneededColumnNames contains names of columns, which mustn't be returned in the result.
	//
	// This list is consulted if needAllColumns=true
	unneededColumnNames []string

	// needAllColumns is set to true when all the columns except of unneededColumnNames must be returned in the result
	needAllColumns bool
}

type searchOptions struct {
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

	// neededColumnNames contains names of columns to return in the result
	neededColumnNames []string

	// unneededColumnNames contains names of columns, which mustn't be returned in the result.
	//
	// This list is consulted when needAllColumns=true.
	unneededColumnNames []string

	// needAllColumns is set to true when all the columns except of unneededColumnNames must be returned in the result
	needAllColumns bool
}

// WriteBlockFunc must write a block with the given timestamps and columns.
//
// WriteBlockFunc cannot hold references to timestamps and columns after returning.
type WriteBlockFunc func(workerID uint, timestamps []int64, columns []BlockColumn)

// RunQuery runs the given q and calls writeBlock for results.
func (s *Storage) RunQuery(ctx context.Context, tenantIDs []TenantID, q *Query, writeBlock WriteBlockFunc) error {
	qNew, err := s.initFilterInValues(ctx, tenantIDs, q)
	if err != nil {
		return err
	}

	writeBlockResult := func(workerID uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}

		brs := getBlockRows()
		csDst := brs.cs

		cs := br.getColumns()
		for _, c := range cs {
			values := c.getValues(br)
			csDst = append(csDst, BlockColumn{
				Name:   c.name,
				Values: values,
			})
		}

		timestamps := br.getTimestamps()
		writeBlock(workerID, timestamps, csDst)

		brs.cs = csDst
		putBlockRows(brs)
	}

	return s.runQuery(ctx, tenantIDs, qNew, writeBlockResult)
}

func (s *Storage) runQuery(ctx context.Context, tenantIDs []TenantID, q *Query, writeBlockResultFunc func(workerID uint, br *blockResult)) error {
	streamIDs := q.getStreamIDs()
	sort.Slice(streamIDs, func(i, j int) bool {
		return streamIDs[i].less(&streamIDs[j])
	})

	minTimestamp, maxTimestamp := q.GetFilterTimeRange()

	neededColumnNames, unneededColumnNames := q.getNeededColumns()
	so := &genericSearchOptions{
		tenantIDs:           tenantIDs,
		streamIDs:           streamIDs,
		minTimestamp:        minTimestamp,
		maxTimestamp:        maxTimestamp,
		filter:              q.f,
		neededColumnNames:   neededColumnNames,
		unneededColumnNames: unneededColumnNames,
		needAllColumns:      slices.Contains(neededColumnNames, "*"),
	}

	workersCount := cgroup.AvailableCPUs()

	ppMain := newDefaultPipeProcessor(writeBlockResultFunc)
	pp := ppMain
	stopCh := ctx.Done()
	cancels := make([]func(), len(q.pipes))
	pps := make([]pipeProcessor, len(q.pipes))

	var errPipe error
	for i := len(q.pipes) - 1; i >= 0; i-- {
		p := q.pipes[i]
		ctxChild, cancel := context.WithCancel(ctx)
		pp = p.newPipeProcessor(workersCount, stopCh, cancel, pp)

		pcp, ok := pp.(*pipeStreamContextProcessor)
		if ok {
			pcp.init(s, neededColumnNames, unneededColumnNames)
			if i > 0 {
				errPipe = fmt.Errorf("[%s] pipe must go after [%s] filter; now it goes after the [%s] pipe", p, q.f, q.pipes[i-1])
			}
		}

		stopCh = ctxChild.Done()
		ctx = ctxChild

		cancels[i] = cancel
		pps[i] = pp
	}

	if errPipe == nil {
		s.search(workersCount, so, stopCh, pp.writeBlock)
	}

	var errFlush error
	for i, pp := range pps {
		if err := pp.flush(); err != nil && errFlush == nil {
			errFlush = err
		}
		cancel := cancels[i]
		cancel()
	}
	if err := ppMain.flush(); err != nil && errFlush == nil {
		errFlush = err
	}

	if errPipe != nil {
		return errPipe
	}

	return errFlush
}

// GetFieldNames returns field names from q results for the given tenantIDs.
func (s *Storage) GetFieldNames(ctx context.Context, tenantIDs []TenantID, q *Query) ([]ValueWithHits, error) {
	pipes := append([]pipe{}, q.pipes...)
	pipeStr := "field_names"
	lex := newLexer(pipeStr)

	pf, err := parsePipeFieldNames(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'field_names' pipe at [%s]: %s", pipeStr, err)
	}
	pf.isFirstPipe = len(pipes) == 0

	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pf)

	q = &Query{
		f:     q.f,
		pipes: pipes,
	}

	return s.runValuesWithHitsQuery(ctx, tenantIDs, q)
}

func (s *Storage) getFieldValuesNoHits(ctx context.Context, tenantIDs []TenantID, q *Query, fieldName string) ([]string, error) {
	pipes := append([]pipe{}, q.pipes...)
	quotedFieldName := quoteTokenIfNeeded(fieldName)
	pipeStr := fmt.Sprintf("uniq by (%s)", quotedFieldName)
	lex := newLexer(pipeStr)

	pu, err := parsePipeUniq(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'uniq' pipe at [%s]: %s", pipeStr, err)
	}

	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pu)

	q = &Query{
		f:     q.f,
		pipes: pipes,
	}

	var values []string
	var valuesLock sync.Mutex
	writeBlockResult := func(_ uint, br *blockResult) {
		if br.rowsLen == 0 {
			return
		}

		cs := br.getColumns()
		if len(cs) != 1 {
			logger.Panicf("BUG: expecting one column; got %d columns", len(cs))
		}

		columnValues := cs[0].getValues(br)

		columnValuesCopy := make([]string, len(columnValues))
		for i := range columnValues {
			columnValuesCopy[i] = strings.Clone(columnValues[i])
		}

		valuesLock.Lock()
		values = append(values, columnValuesCopy...)
		valuesLock.Unlock()
	}

	if err := s.runQuery(ctx, tenantIDs, q, writeBlockResult); err != nil {
		return nil, err
	}

	return values, nil
}

// GetFieldValues returns unique values with the number of hits for the given fieldName returned by q for the given tenantIDs.
//
// If limit > 0, then up to limit unique values are returned.
func (s *Storage) GetFieldValues(ctx context.Context, tenantIDs []TenantID, q *Query, fieldName string, limit uint64) ([]ValueWithHits, error) {
	pipes := append([]pipe{}, q.pipes...)
	quotedFieldName := quoteTokenIfNeeded(fieldName)
	pipeStr := fmt.Sprintf("field_values %s limit %d", quotedFieldName, limit)
	lex := newLexer(pipeStr)

	pu, err := parsePipeFieldValues(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'field_values' pipe at [%s]: %s", pipeStr, err)
	}

	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pu)

	q = &Query{
		f:     q.f,
		pipes: pipes,
	}

	return s.runValuesWithHitsQuery(ctx, tenantIDs, q)
}

// ValueWithHits contains value and hits.
type ValueWithHits struct {
	Value string
	Hits  uint64
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

func sortValuesWithHits(results []ValueWithHits) {
	slices.SortFunc(results, func(a, b ValueWithHits) int {
		if a.Hits == b.Hits {
			if a.Value == b.Value {
				return 0
			}
			if lessString(a.Value, b.Value) {
				return -1
			}
			return 1
		}
		// Sort in descending order of hits
		if a.Hits < b.Hits {
			return 1
		}
		return -1
	})
}

// GetStreamFieldNames returns stream field names from q results for the given tenantIDs.
func (s *Storage) GetStreamFieldNames(ctx context.Context, tenantIDs []TenantID, q *Query) ([]ValueWithHits, error) {
	streams, err := s.GetStreams(ctx, tenantIDs, q, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	m := make(map[string]*uint64)
	forEachStreamField(streams, func(f Field, hits uint64) {
		pHits, ok := m[f.Name]
		if !ok {
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

// GetStreamFieldValues returns stream field values for the given fieldName from q results for the given tenantIDs.
//
// If limit > 9, then up to limit unique values are returned.
func (s *Storage) GetStreamFieldValues(ctx context.Context, tenantIDs []TenantID, q *Query, fieldName string, limit uint64) ([]ValueWithHits, error) {
	streams, err := s.GetStreams(ctx, tenantIDs, q, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	m := make(map[string]*uint64)
	forEachStreamField(streams, func(f Field, hits uint64) {
		if f.Name != fieldName {
			return
		}
		pHits, ok := m[f.Value]
		if !ok {
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
	}
	return values, nil
}

// GetStreams returns streams from q results for the given tenantIDs.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreams(ctx context.Context, tenantIDs []TenantID, q *Query, limit uint64) ([]ValueWithHits, error) {
	return s.GetFieldValues(ctx, tenantIDs, q, "_stream", limit)
}

// GetStreamIDs returns stream_id field values from q results for the given tenantIDs.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreamIDs(ctx context.Context, tenantIDs []TenantID, q *Query, limit uint64) ([]ValueWithHits, error) {
	return s.GetFieldValues(ctx, tenantIDs, q, "_stream_id", limit)
}

func (s *Storage) runValuesWithHitsQuery(ctx context.Context, tenantIDs []TenantID, q *Query) ([]ValueWithHits, error) {
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

	err := s.runQuery(ctx, tenantIDs, q, writeBlockResult)
	if err != nil {
		return nil, err
	}
	sortValuesWithHits(results)

	return results, nil
}

func (s *Storage) initFilterInValues(ctx context.Context, tenantIDs []TenantID, q *Query) (*Query, error) {
	if !hasFilterInWithQueryForFilter(q.f) && !hasFilterInWithQueryForPipes(q.pipes) {
		return q, nil
	}

	getFieldValues := func(q *Query, fieldName string) ([]string, error) {
		return s.getFieldValuesNoHits(ctx, tenantIDs, q, fieldName)
	}
	cache := make(map[string][]string)
	fNew, err := initFilterInValuesForFilter(cache, q.f, getFieldValues)
	if err != nil {
		return nil, err
	}
	pipesNew, err := initFilterInValuesForPipes(cache, q.pipes, getFieldValues)
	if err != nil {
		return nil, err
	}
	qNew := &Query{
		f:     fNew,
		pipes: pipesNew,
	}
	return qNew, nil
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
			return t.needExecuteQuery
		case *filterStreamID:
			return t.needExecuteQuery
		default:
			return false
		}
	}
	return visitFilter(f, visitFunc)
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

func (iff *ifFilter) initFilterInValues(cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) (*ifFilter, error) {
	if iff == nil {
		return nil, nil
	}

	f, err := initFilterInValuesForFilter(cache, iff.f, getFieldValuesFunc)
	if err != nil {
		return nil, err
	}

	iffNew := *iff
	iffNew.f = f
	return &iffNew, nil
}

func initFilterInValuesForFilter(cache map[string][]string, f filter, getFieldValuesFunc getFieldValuesFunc) (filter, error) {
	if f == nil {
		return nil, nil
	}

	visitFunc := func(f filter) bool {
		switch t := f.(type) {
		case *filterIn:
			return t.needExecuteQuery
		case *filterStreamID:
			return t.needExecuteQuery
		default:
			return false
		}
	}
	copyFunc := func(f filter) (filter, error) {
		switch t := f.(type) {
		case *filterIn:
			values, err := getValuesForQuery(t.q, t.qFieldName, cache, getFieldValuesFunc)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", t, err)
			}

			fiNew := &filterIn{
				fieldName: t.fieldName,
				q:         t.q,
				values:    values,
			}
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
				q:         t.q,
			}
			return fsNew, nil
		default:
			return f, nil
		}
	}
	return copyFilter(f, visitFunc, copyFunc)
}

func getValuesForQuery(q *Query, qFieldName string, cache map[string][]string, getFieldValuesFunc getFieldValuesFunc) ([]string, error) {
	qStr := q.String()
	values, ok := cache[qStr]
	if ok {
		return values, nil
	}

	vs, err := getFieldValuesFunc(q, qFieldName)
	if err != nil {
		return nil, err
	}
	cache[qStr] = vs
	return vs, nil
}

func initFilterInValuesForPipes(cache map[string][]string, pipes []pipe, getFieldValuesFunc getFieldValuesFunc) ([]pipe, error) {
	pipesNew := make([]pipe, len(pipes))
	for i, p := range pipes {
		pNew, err := p.initFilterInValues(cache, getFieldValuesFunc)
		if err != nil {
			return nil, err
		}
		pipesNew[i] = pNew
	}
	return pipesNew, nil
}

type blockRows struct {
	cs []BlockColumn
}

func (brs *blockRows) reset() {
	cs := brs.cs
	for i := range cs {
		cs[i].reset()
	}
	brs.cs = cs[:0]
}

func getBlockRows() *blockRows {
	v := blockRowsPool.Get()
	if v == nil {
		return &blockRows{}
	}
	return v.(*blockRows)
}

func putBlockRows(brs *blockRows) {
	brs.reset()
	blockRowsPool.Put(brs)
}

var blockRowsPool sync.Pool

// BlockColumn is a single column of a block of data
type BlockColumn struct {
	// Name is the column name
	Name string

	// Values is column values
	Values []string
}

func (c *BlockColumn) reset() {
	c.Name = ""
	c.Values = nil
}

// searchResultFunc must process sr.
//
// The callback is called at the worker with the given workerID.
type searchResultFunc func(workerID uint, br *blockResult)

// search searches for the matching rows according to so.
//
// It calls processBlockResult for each matching block.
func (s *Storage) search(workersCount int, so *genericSearchOptions, stopCh <-chan struct{}, processBlockResult searchResultFunc) {
	// Spin up workers
	var wgWorkers sync.WaitGroup
	workCh := make(chan *blockSearchWorkBatch, workersCount)
	wgWorkers.Add(workersCount)
	for i := 0; i < workersCount; i++ {
		go func(workerID uint) {
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

					bs.search(bsw, bm)
					if bs.br.rowsLen > 0 {
						processBlockResult(workerID, &bs.br)
					}
					bsw.reset()
				}
				bswb.bsws = bswb.bsws[:0]
				putBlockSearchWorkBatch(bswb)
			}
			putBlockSearch(bs)
			putBitmap(bm)
			wgWorkers.Done()
		}(uint(i))
	}

	// Select partitions according to the selected time range
	s.partitionsLock.Lock()
	ptws := s.partitions
	minDay := so.minTimestamp / nsecsPerDay
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= minDay
	})
	ptws = ptws[n:]
	maxDay := so.maxTimestamp / nsecsPerDay
	n = sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day > maxDay
	})
	ptws = ptws[:n]
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	// Obtain common filterStream from f
	sf, f := getCommonStreamFilter(so.filter)

	// Schedule concurrent search across matching partitions.
	psfs := make([]partitionSearchFinalizer, len(ptws))
	var wgSearchers sync.WaitGroup
	for i, ptw := range ptws {
		partitionSearchConcurrencyLimitCh <- struct{}{}
		wgSearchers.Add(1)
		go func(idx int, pt *partition) {
			psfs[idx] = pt.search(sf, f, so, workCh, stopCh)
			wgSearchers.Done()
			<-partitionSearchConcurrencyLimitCh
		}(i, ptw.pt)
	}
	wgSearchers.Wait()

	// Wait until workers finish their work
	close(workCh)
	wgWorkers.Wait()

	// Finalize partition search
	for _, psf := range psfs {
		psf()
	}

	// Decrement references to partitions
	for _, ptw := range ptws {
		ptw.decRef()
	}
}

// partitionSearchConcurrencyLimitCh limits the number of concurrent searches in partition.
//
// This is needed for limiting memory usage under high load.
var partitionSearchConcurrencyLimitCh = make(chan struct{}, cgroup.AvailableCPUs())

type partitionSearchFinalizer func()

func (pt *partition) search(sf *StreamFilter, f filter, so *genericSearchOptions, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
	if needStop(stopCh) {
		// Do not spend CPU time on search, since it is already stopped.
		return func() {}
	}

	tenantIDs := so.tenantIDs
	var streamIDs []streamID
	if sf != nil {
		streamIDs = pt.idb.searchStreamIDs(tenantIDs, sf)
		if len(so.streamIDs) > 0 {
			streamIDs = intersectStreamIDs(streamIDs, so.streamIDs)
		}
		tenantIDs = nil
	} else if len(so.streamIDs) > 0 {
		streamIDs = getStreamIDsForTenantIDs(so.streamIDs, tenantIDs)
		tenantIDs = nil
	}
	if hasStreamFilters(f) {
		f = initStreamFilters(tenantIDs, pt.idb, f)
	}
	soInternal := &searchOptions{
		tenantIDs:           tenantIDs,
		streamIDs:           streamIDs,
		minTimestamp:        so.minTimestamp,
		maxTimestamp:        so.maxTimestamp,
		filter:              f,
		neededColumnNames:   so.neededColumnNames,
		unneededColumnNames: so.unneededColumnNames,
		needAllColumns:      so.needAllColumns,
	}
	return pt.ddb.search(soInternal, workCh, stopCh)
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
	return visitFilter(f, visitFunc)
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

func (ddb *datadb) search(so *searchOptions, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
	// Select parts with data for the given time range
	ddb.partsLock.Lock()
	pws := appendPartsInTimeRange(nil, ddb.bigParts, so.minTimestamp, so.maxTimestamp)
	pws = appendPartsInTimeRange(pws, ddb.smallParts, so.minTimestamp, so.maxTimestamp)
	pws = appendPartsInTimeRange(pws, ddb.inmemoryParts, so.minTimestamp, so.maxTimestamp)

	// Increase references to the searched parts, so they aren't deleted during search.
	// References to the searched parts must be decremented by calling the returned partitionSearchFinalizer.
	for _, pw := range pws {
		pw.incRef()
	}
	ddb.partsLock.Unlock()

	// Apply search to matching parts
	for _, pw := range pws {
		pw.p.search(so, workCh, stopCh)
	}

	return func() {
		for _, pw := range pws {
			pw.decRef()
		}
	}
}

func (p *part) search(so *searchOptions, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	bhss := getBlockHeaders()
	if len(so.tenantIDs) > 0 {
		p.searchByTenantIDs(so, bhss, workCh, stopCh)
	} else {
		p.searchByStreamIDs(so, bhss, workCh, stopCh)
	}
	putBlockHeaders(bhss)
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

func (p *part) searchByTenantIDs(so *searchOptions, bhss *blockHeaders, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	// it is assumed that tenantIDs are sorted
	tenantIDs := so.tenantIDs

	bswb := getBlockSearchWorkBatch()
	scheduleBlockSearch := func(bh *blockHeader) bool {
		if bswb.appendBlockSearchWork(p, so, bh) {
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

		if so.minTimestamp > ibh.maxTimestamp || so.maxTimestamp < ibh.minTimestamp {
			// Skip the ibh, since it doesn't contain entries on the requested time range
			continue
		}

		bhss.bhs = ibh.mustReadBlockHeaders(bhss.bhs[:0], p)

		bhs := bhss.bhs
		for len(bhs) > 0 {
			// search for blocks with the given tenantID
			n = sort.Search(len(bhs), func(i int) bool {
				return !bhs[i].streamID.tenantID.less(tenantID)
			})
			bhs = bhs[n:]
			for len(bhs) > 0 && bhs[0].streamID.tenantID.equal(tenantID) {
				bh := &bhs[0]
				bhs = bhs[1:]
				th := &bh.timestampsHeader
				if so.minTimestamp > th.maxTimestamp || so.maxTimestamp < th.minTimestamp {
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

func (p *part) searchByStreamIDs(so *searchOptions, bhss *blockHeaders, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) {
	// it is assumed that streamIDs are sorted
	streamIDs := so.streamIDs

	bswb := getBlockSearchWorkBatch()
	scheduleBlockSearch := func(bh *blockHeader) bool {
		if bswb.appendBlockSearchWork(p, so, bh) {
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

		if so.minTimestamp > ibh.maxTimestamp || so.maxTimestamp < ibh.minTimestamp {
			// Skip the ibh, since it doesn't contain entries on the requested time range
			continue
		}

		bhss.bhs = ibh.mustReadBlockHeaders(bhss.bhs[:0], p)

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
				if so.minTimestamp > th.maxTimestamp || so.maxTimestamp < th.minTimestamp {
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
