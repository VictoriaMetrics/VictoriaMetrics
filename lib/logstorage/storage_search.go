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
		if len(br.timestamps) == 0 {
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
		writeBlock(workerID, br.timestamps, csDst)

		brs.cs = csDst
		putBlockRows(brs)
	}

	return s.runQuery(ctx, tenantIDs, qNew, writeBlockResult)
}

func (s *Storage) runQuery(ctx context.Context, tenantIDs []TenantID, q *Query, writeBlockResultFunc func(workerID uint, br *blockResult)) error {
	neededColumnNames, unneededColumnNames := q.getNeededColumns()
	so := &genericSearchOptions{
		tenantIDs:           tenantIDs,
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
	for i := len(q.pipes) - 1; i >= 0; i-- {
		p := q.pipes[i]
		ctxChild, cancel := context.WithCancel(ctx)
		pp = p.newPipeProcessor(workersCount, stopCh, cancel, pp)
		stopCh = ctxChild.Done()
		ctx = ctxChild

		cancels[i] = cancel
		pps[i] = pp
	}

	s.search(workersCount, so, stopCh, pp.writeBlock)

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
	return errFlush
}

// GetFieldNames returns field names from q results for the given tenantIDs.
func (s *Storage) GetFieldNames(ctx context.Context, tenantIDs []TenantID, q *Query) ([]string, error) {
	pipes := append([]pipe{}, q.pipes...)
	pipeStr := "field_names as names | sort by (names)"
	lex := newLexer(pipeStr)

	pf, err := parsePipeFieldNames(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'field_names' pipe at [%s]: %s", pipeStr, err)
	}
	pf.isFirstPipe = len(pipes) == 0

	if !lex.isKeyword("|") {
		logger.Panicf("BUG: unexpected token after 'field_names' pipe at [%s]: %q", pipeStr, lex.token)
	}
	lex.nextToken()

	ps, err := parsePipeSort(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'sort' pipe at [%s]: %s", pipeStr, err)
	}
	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pf, ps)

	q = &Query{
		f:     q.f,
		pipes: pipes,
	}

	return s.runSingleColumnQuery(ctx, tenantIDs, q)
}

// GetFieldValues returns unique values for the given fieldName returned by q for the given tenantIDs.
//
// If limit > 0, then up to limit unique values are returned.
func (s *Storage) GetFieldValues(ctx context.Context, tenantIDs []TenantID, q *Query, fieldName string, limit uint64) ([]string, error) {
	pipes := append([]pipe{}, q.pipes...)
	quotedFieldName := quoteTokenIfNeeded(fieldName)
	pipeStr := fmt.Sprintf("uniq by (%s) limit %d | sort by (%s)", quotedFieldName, limit, quotedFieldName)
	lex := newLexer(pipeStr)

	pu, err := parsePipeUniq(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'uniq' pipe at [%s]: %s", pipeStr, err)
	}

	if !lex.isKeyword("|") {
		logger.Panicf("BUG: unexpected token after 'uniq' pipe at [%s]: %q", pipeStr, lex.token)
	}
	lex.nextToken()

	ps, err := parsePipeSort(lex)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing 'sort' pipe at [%s]: %s", pipeStr, err)
	}
	if !lex.isEnd() {
		logger.Panicf("BUG: unexpected tail left after parsing pipes [%s]: %q", pipeStr, lex.s)
	}

	pipes = append(pipes, pu, ps)

	q = &Query{
		f:     q.f,
		pipes: pipes,
	}

	return s.runSingleColumnQuery(ctx, tenantIDs, q)
}

// GetStreamLabelNames returns stream label names from q results for the given tenantIDs.
func (s *Storage) GetStreamLabelNames(ctx context.Context, tenantIDs []TenantID, q *Query) ([]string, error) {
	streams, err := s.GetStreams(ctx, tenantIDs, q, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	var names []string
	m := make(map[string]struct{})
	forEachStreamLabel(streams, func(label Field) {
		if _, ok := m[label.Name]; !ok {
			nameCopy := strings.Clone(label.Name)
			names = append(names, nameCopy)
			m[nameCopy] = struct{}{}
		}
	})
	sortStrings(names)

	return names, nil
}

// GetStreamLabelValues returns stream label values for the given labelName from q results for the given tenantIDs.
//
// If limit > 9, then up to limit unique label values are returned.
func (s *Storage) GetStreamLabelValues(ctx context.Context, tenantIDs []TenantID, q *Query, labelName string, limit uint64) ([]string, error) {
	streams, err := s.GetStreams(ctx, tenantIDs, q, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	var values []string
	m := make(map[string]struct{})
	forEachStreamLabel(streams, func(label Field) {
		if label.Name != labelName {
			return
		}
		if _, ok := m[label.Value]; !ok {
			valueCopy := strings.Clone(label.Value)
			values = append(values, valueCopy)
			m[valueCopy] = struct{}{}
		}
	})
	if uint64(len(values)) > limit {
		values = values[:limit]
	}
	sortStrings(values)

	return values, nil
}

// GetStreams returns streams from q results for the given tenantIDs.
//
// If limit > 0, then up to limit unique streams are returned.
func (s *Storage) GetStreams(ctx context.Context, tenantIDs []TenantID, q *Query, limit uint64) ([]string, error) {
	return s.GetFieldValues(ctx, tenantIDs, q, "_stream", limit)
}

func (s *Storage) runSingleColumnQuery(ctx context.Context, tenantIDs []TenantID, q *Query) ([]string, error) {
	var values []string
	var valuesLock sync.Mutex
	writeBlockResult := func(_ uint, br *blockResult) {
		if len(br.timestamps) == 0 {
			return
		}

		cs := br.getColumns()
		if len(cs) != 1 {
			logger.Panicf("BUG: expecting only a single column; got %d columns", len(cs))
		}
		columnValues := cs[0].getValues(br)

		columnValuesCopy := make([]string, len(columnValues))
		for i, v := range columnValues {
			columnValuesCopy[i] = strings.Clone(v)
		}

		valuesLock.Lock()
		values = append(values, columnValuesCopy...)
		valuesLock.Unlock()
	}

	err := s.runQuery(ctx, tenantIDs, q, writeBlockResult)
	if err != nil {
		return nil, err
	}

	return values, nil
}

func (s *Storage) initFilterInValues(ctx context.Context, tenantIDs []TenantID, q *Query) (*Query, error) {
	if !hasFilterInWithQueryForFilter(q.f) && !hasFilterInWithQueryForPipes(q.pipes) {
		return q, nil
	}

	getFieldValues := func(q *Query, fieldName string) ([]string, error) {
		return s.GetFieldValues(ctx, tenantIDs, q, fieldName, 0)
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
		fi, ok := f.(*filterIn)
		return ok && fi.needExecuteQuery
	}
	return visitFilter(f, visitFunc)
}

func hasFilterInWithQueryForPipes(pipes []pipe) bool {
	for _, p := range pipes {
		switch t := p.(type) {
		case *pipeStats:
			for _, f := range t.funcs {
				if f.iff.hasFilterInWithQuery() {
					return true
				}
			}
		case *pipeExtract:
			if t.iff.hasFilterInWithQuery() {
				return true
			}
		case *pipeUnpackJSON:
			if t.iff.hasFilterInWithQuery() {
				return true
			}
		case *pipeUnpackLogfmt:
			if t.iff.hasFilterInWithQuery() {
				return true
			}
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
		fi, ok := f.(*filterIn)
		return ok && fi.needExecuteQuery
	}
	copyFunc := func(f filter) (filter, error) {
		fi := f.(*filterIn)

		qStr := fi.q.String()
		values, ok := cache[qStr]
		if !ok {
			vs, err := getFieldValuesFunc(fi.q, fi.qFieldName)
			if err != nil {
				return nil, fmt.Errorf("cannot obtain unique values for %s: %w", fi, err)
			}
			cache[qStr] = vs
			values = vs
		}

		fiNew := &filterIn{
			fieldName: fi.fieldName,
			q:         fi.q,
			values:    values,
		}
		return fiNew, nil
	}
	return copyFilter(f, visitFunc, copyFunc)
}

func initFilterInValuesForPipes(cache map[string][]string, pipes []pipe, getFieldValuesFunc getFieldValuesFunc) ([]pipe, error) {
	pipesNew := make([]pipe, len(pipes))
	for i, p := range pipes {
		switch t := p.(type) {
		case *pipeStats:
			funcsNew := make([]pipeStatsFunc, len(t.funcs))
			for j, f := range t.funcs {
				iffNew, err := f.iff.initFilterInValues(cache, getFieldValuesFunc)
				if err != nil {
					return nil, err
				}
				f.iff = iffNew
				funcsNew[j] = f
			}
			pipesNew[i] = &pipeStats{
				byFields: t.byFields,
				funcs:    funcsNew,
			}
		case *pipeExtract:
			iffNew, err := t.iff.initFilterInValues(cache, getFieldValuesFunc)
			if err != nil {
				return nil, err
			}
			pe := *t
			pe.iff = iffNew
			pipesNew[i] = &pe
		case *pipeUnpackJSON:
			iffNew, err := t.iff.initFilterInValues(cache, getFieldValuesFunc)
			if err != nil {
				return nil, err
			}
			pu := *t
			pu.iff = iffNew
			pipesNew[i] = &pu
		case *pipeUnpackLogfmt:
			iffNew, err := t.iff.initFilterInValues(cache, getFieldValuesFunc)
			if err != nil {
				return nil, err
			}
			pu := *t
			pu.iff = iffNew
			pipesNew[i] = &pu
		default:
			pipesNew[i] = p
		}
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
					if len(bs.br.timestamps) > 0 {
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

	// Obtain time range from so.filter
	f := so.filter
	minTimestamp, maxTimestamp := getFilterTimeRange(f)

	// Select partitions according to the selected time range
	s.partitionsLock.Lock()
	ptws := s.partitions
	minDay := minTimestamp / nsecPerDay
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= minDay
	})
	ptws = ptws[n:]
	maxDay := maxTimestamp / nsecPerDay
	n = sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day > maxDay
	})
	ptws = ptws[:n]
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	// Obtain common filterStream from f
	var sf *StreamFilter
	sf, f = getCommonStreamFilter(f)

	// Schedule concurrent search across matching partitions.
	psfs := make([]partitionSearchFinalizer, len(ptws))
	var wgSearchers sync.WaitGroup
	for i, ptw := range ptws {
		partitionSearchConcurrencyLimitCh <- struct{}{}
		wgSearchers.Add(1)
		go func(idx int, pt *partition) {
			psfs[idx] = pt.search(minTimestamp, maxTimestamp, sf, f, so, workCh, stopCh)
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

func (pt *partition) search(minTimestamp, maxTimestamp int64, sf *StreamFilter, f filter, so *genericSearchOptions, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
	if needStop(stopCh) {
		// Do not spend CPU time on search, since it is already stopped.
		return func() {}
	}

	tenantIDs := so.tenantIDs
	var streamIDs []streamID
	if sf != nil {
		streamIDs = pt.idb.searchStreamIDs(tenantIDs, sf)
		tenantIDs = nil
	}
	if hasStreamFilters(f) {
		f = initStreamFilters(tenantIDs, pt.idb, f)
	}
	soInternal := &searchOptions{
		tenantIDs:           tenantIDs,
		streamIDs:           streamIDs,
		minTimestamp:        minTimestamp,
		maxTimestamp:        maxTimestamp,
		filter:              f,
		neededColumnNames:   so.neededColumnNames,
		unneededColumnNames: so.unneededColumnNames,
		needAllColumns:      so.needAllColumns,
	}
	return pt.ddb.search(soInternal, workCh, stopCh)
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

func getFilterTimeRange(f filter) (int64, int64) {
	switch t := f.(type) {
	case *filterAnd:
		minTimestamp := int64(math.MinInt64)
		maxTimestamp := int64(math.MaxInt64)
		for _, filter := range t.filters {
			ft, ok := filter.(*filterTime)
			if ok {
				if ft.minTimestamp > minTimestamp {
					minTimestamp = ft.minTimestamp
				}
				if ft.maxTimestamp < maxTimestamp {
					maxTimestamp = ft.maxTimestamp
				}
			}
		}
		return minTimestamp, maxTimestamp
	case *filterTime:
		return t.minTimestamp, t.maxTimestamp
	}
	return math.MinInt64, math.MaxInt64
}

func forEachStreamLabel(streams []string, f func(label Field)) {
	var labels []Field
	for _, stream := range streams {
		var err error
		labels, err = parseStreamLabels(labels[:0], stream)
		if err != nil {
			continue
		}
		for i := range labels {
			f(labels[i])
		}
	}
}

func parseStreamLabels(dst []Field, s string) ([]Field, error) {
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
			return dst, fmt.Errorf("cannot find label value in double quotes at [%s]", s)
		}
		name := s[:n]
		s = s[n+1:]

		value, nOffset := tryUnquoteString(s)
		if nOffset < 0 {
			return dst, fmt.Errorf("cannot find parse label value in double quotes at [%s]", s)
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
