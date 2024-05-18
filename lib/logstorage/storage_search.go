package logstorage

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
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
	return s.runQuery(ctx, tenantIDs, qNew, writeBlock)
}

func (s *Storage) runQuery(ctx context.Context, tenantIDs []TenantID, q *Query, writeBlock WriteBlockFunc) error {
	neededColumnNames, unneededColumnNames := q.getNeededColumns()
	so := &genericSearchOptions{
		tenantIDs:           tenantIDs,
		filter:              q.f,
		neededColumnNames:   neededColumnNames,
		unneededColumnNames: unneededColumnNames,
		needAllColumns:      slices.Contains(neededColumnNames, "*"),
	}

	pp := newDefaultPipeProcessor(func(workerID uint, br *blockResult) {
		brs := getBlockRows()
		csDst := brs.cs

		for _, c := range br.getColumns() {
			values := c.getValues(br)
			csDst = append(csDst, BlockColumn{
				Name:   c.name,
				Values: values,
			})
		}
		writeBlock(workerID, br.timestamps, csDst)

		brs.cs = csDst
		putBlockRows(brs)
	})

	workersCount := cgroup.AvailableCPUs()

	ppMain := pp
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

// GetUniqueFieldValues returns unique values for the given fieldName returned by q for the given tenantIDs.
//
// If limit > 0, then up to limit unique values are returned. The values are returned in arbitrary order because of performance reasons.
// The caller may sort the returned values if needed.
func (s *Storage) GetUniqueFieldValues(ctx context.Context, tenantIDs []TenantID, q *Query, fieldName string, limit uint64) ([]string, error) {
	// add 'uniq fieldName' to the end of q.pipes
	if !endsWithPipeUniqSingleField(q.pipes, fieldName) {
		pipes := append([]pipe{}, q.pipes...)
		pipes = append(pipes, &pipeUniq{
			byFields: []string{fieldName},
			limit:    limit,
		})
		q = &Query{
			f:     q.f,
			pipes: pipes,
		}
	}

	var values []string
	var valuesLock sync.Mutex
	writeBlock := func(workerID uint, timestamps []int64, columns []BlockColumn) {
		if len(columns) != 1 {
			logger.Panicf("BUG: expecting only a single column; got %d columns", len(columns))
		}
		valuesLock.Lock()
		values = append(values, columns[0].Values...)
		valuesLock.Unlock()
	}

	err := s.runQuery(ctx, tenantIDs, q, writeBlock)
	if err != nil {
		return nil, err
	}

	return values, nil
}

func endsWithPipeUniqSingleField(pipes []pipe, fieldName string) bool {
	if len(pipes) == 0 {
		return false
	}
	pu, ok := pipes[len(pipes)-1].(*pipeUniq)
	if !ok {
		return false
	}
	return len(pu.byFields) == 1 && pu.byFields[0] == fieldName
}

func (s *Storage) initFilterInValues(ctx context.Context, tenantIDs []TenantID, q *Query) (*Query, error) {
	if !hasFilterInWithQueryForFilter(q.f) && !hasFilterInWithQueryForPipes(q.pipes) {
		return q, nil
	}

	getUniqueValues := func(q *Query, fieldName string) ([]string, error) {
		return s.GetUniqueFieldValues(ctx, tenantIDs, q, fieldName, 0)
	}
	cache := make(map[string][]string)
	fNew, err := initFilterInValuesForFilter(cache, q.f, getUniqueValues)
	if err != nil {
		return nil, err
	}
	pipesNew, err := initFilterInValuesForPipes(cache, q.pipes, getUniqueValues)
	if err != nil {
		return nil, err
	}
	qNew := &Query{
		f:     fNew,
		pipes: pipesNew,
	}
	return qNew, nil
}

func hasFilterInWithQueryForFilter(f filter) bool {
	visitFunc := func(f filter) bool {
		fi, ok := f.(*filterIn)
		return ok && fi.needExecuteQuery
	}
	return visitFilter(f, visitFunc)
}

func hasFilterInWithQueryForPipes(pipes []pipe) bool {
	for _, p := range pipes {
		ps, ok := p.(*pipeStats)
		if !ok {
			continue
		}
		for _, f := range ps.funcs {
			if f.iff != nil && hasFilterInWithQueryForFilter(f.iff) {
				return true
			}
		}
	}
	return false
}

type getUniqueValuesFunc func(q *Query, fieldName string) ([]string, error)

func initFilterInValuesForFilter(cache map[string][]string, f filter, getUniqueValuesFunc getUniqueValuesFunc) (filter, error) {
	visitFunc := func(f filter) bool {
		fi, ok := f.(*filterIn)
		return ok && fi.needExecuteQuery
	}
	copyFunc := func(f filter) (filter, error) {
		fi := f.(*filterIn)

		qStr := fi.q.String()
		values, ok := cache[qStr]
		if !ok {
			vs, err := getUniqueValuesFunc(fi.q, fi.qFieldName)
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

func initFilterInValuesForPipes(cache map[string][]string, pipes []pipe, getUniqueValuesFunc getUniqueValuesFunc) ([]pipe, error) {
	pipesNew := make([]pipe, len(pipes))
	for i, p := range pipes {
		switch t := p.(type) {
		case *pipeStats:
			funcsNew := make([]pipeStatsFunc, len(t.funcs))
			for j, f := range t.funcs {
				if f.iff != nil {
					fNew, err := initFilterInValuesForFilter(cache, f.iff, getUniqueValuesFunc)
					if err != nil {
						return nil, err
					}
					f.iff = fNew
				}
				funcsNew[j] = f
			}
			pipesNew[i] = &pipeStats{
				byFields: t.byFields,
				funcs:    funcsNew,
			}
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

	// Obtain common time filter from so.filter
	ft, f := getCommonFilterTime(so.filter)

	// Select partitions according to the selected time range
	s.partitionsLock.Lock()
	ptws := s.partitions
	minDay := ft.minTimestamp / nsecPerDay
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= minDay
	})
	ptws = ptws[n:]
	maxDay := ft.maxTimestamp / nsecPerDay
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
			psfs[idx] = pt.search(ft, sf, f, so, workCh, stopCh)
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

func (pt *partition) search(ft *filterTime, sf *StreamFilter, f filter, so *genericSearchOptions, workCh chan<- *blockSearchWorkBatch, stopCh <-chan struct{}) partitionSearchFinalizer {
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
		minTimestamp:        ft.minTimestamp,
		maxTimestamp:        ft.maxTimestamp,
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

func getCommonFilterTime(f filter) (*filterTime, filter) {
	switch t := f.(type) {
	case *filterAnd:
		for _, filter := range t.filters {
			ft, ok := filter.(*filterTime)
			if ok {
				// The ft must remain in t.filters order to properly filter out rows outside the selected time range
				return ft, f
			}
		}
	case *filterTime:
		return t, f
	}
	return allFilterTime, f
}

var allFilterTime = &filterTime{
	minTimestamp: math.MinInt64,
	maxTimestamp: math.MaxInt64,
}
