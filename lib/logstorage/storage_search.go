package logstorage

import (
	"math"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

// genericSearchOptions contain options used for search.
type genericSearchOptions struct {
	// tenantIDs must contain the list of tenantIDs for the search.
	tenantIDs []TenantID

	// filter is the filter to use for the search
	filter filter

	// resultColumnNames is names of columns to return in the result.
	resultColumnNames []string
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

	// resultColumnNames is names of columns to return in the result
	resultColumnNames []string
}

// RunQuery runs the given q and calls processBlock for results
func (s *Storage) RunQuery(tenantIDs []TenantID, q *Query, stopCh <-chan struct{}, processBlock func(columns []BlockColumn)) {
	resultColumnNames := q.getResultColumnNames()
	so := &genericSearchOptions{
		tenantIDs:         tenantIDs,
		filter:            q.f,
		resultColumnNames: resultColumnNames,
	}
	workersCount := cgroup.AvailableCPUs()
	s.search(workersCount, so, stopCh, func(workerID uint, br *blockResult) {
		brs := getBlockRows()
		cs := brs.cs

		for i, columnName := range resultColumnNames {
			cs = append(cs, BlockColumn{
				Name:   columnName,
				Values: br.getColumnValues(i),
			})
		}
		processBlock(cs)

		brs.cs = cs
		putBlockRows(brs)
	})
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

// The number of blocks to search at once by a single worker
//
// This number must be increased on systems with many CPU cores in order to amortize
// the overhead for passing the blockSearchWork to worker goroutines.
const blockSearchWorksPerBatch = 64

// searchResultFunc must process sr.
//
// The callback is called at the worker with the given workerID.
type searchResultFunc func(workerID uint, br *blockResult)

// search searches for the matching rows according to so.
//
// It calls f for each found matching block.
func (s *Storage) search(workersCount int, so *genericSearchOptions, stopCh <-chan struct{}, processBlockResult searchResultFunc) {
	// Spin up workers
	var wg sync.WaitGroup
	workCh := make(chan []*blockSearchWork, workersCount)
	wg.Add(workersCount)
	for i := 0; i < workersCount; i++ {
		go func(workerID uint) {
			bs := getBlockSearch()
			for bsws := range workCh {
				for _, bsw := range bsws {
					bs.search(bsw)
					if bs.br.RowsCount() > 0 {
						processBlockResult(workerID, &bs.br)
					}
				}
			}
			putBlockSearch(bs)
			wg.Done()
		}(uint(i))
	}

	// Obtain common time filter from so.filter
	tf, f := getCommonTimeFilter(so.filter)

	// Select partitions according to the selected time range
	s.partitionsLock.Lock()
	ptws := s.partitions
	minDay := tf.minTimestamp / nsecPerDay
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= minDay
	})
	ptws = ptws[n:]
	maxDay := tf.maxTimestamp / nsecPerDay
	n = sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day > maxDay
	})
	ptws = ptws[:n]
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	// Obtain common streamFilter from f
	var sf *StreamFilter
	sf, f = getCommonStreamFilter(f)

	// Apply search to matching partitions
	var pws []*partWrapper
	for _, ptw := range ptws {
		pws = ptw.pt.search(pws, tf, sf, f, so, workCh, stopCh)
	}

	// Wait until workers finish their work
	close(workCh)
	wg.Wait()

	// Decrement references to parts
	for _, pw := range pws {
		pw.decRef()
	}

	// Decrement references to partitions
	for _, ptw := range ptws {
		ptw.decRef()
	}
}

func (pt *partition) search(pwsDst []*partWrapper, tf *timeFilter, sf *StreamFilter, f filter, so *genericSearchOptions,
	workCh chan<- []*blockSearchWork, stopCh <-chan struct{},
) []*partWrapper {
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
		tenantIDs:         tenantIDs,
		streamIDs:         streamIDs,
		minTimestamp:      tf.minTimestamp,
		maxTimestamp:      tf.maxTimestamp,
		filter:            f,
		resultColumnNames: so.resultColumnNames,
	}
	return pt.ddb.search(pwsDst, soInternal, workCh, stopCh)
}

func hasStreamFilters(f filter) bool {
	switch t := f.(type) {
	case *andFilter:
		return hasStreamFiltersInList(t.filters)
	case *orFilter:
		return hasStreamFiltersInList(t.filters)
	case *notFilter:
		return hasStreamFilters(t.f)
	case *streamFilter:
		return true
	default:
		return false
	}
}

func hasStreamFiltersInList(filters []filter) bool {
	for _, f := range filters {
		if hasStreamFilters(f) {
			return true
		}
	}
	return false
}

func initStreamFilters(tenantIDs []TenantID, idb *indexdb, f filter) filter {
	switch t := f.(type) {
	case *andFilter:
		return &andFilter{
			filters: initStreamFiltersList(tenantIDs, idb, t.filters),
		}
	case *orFilter:
		return &orFilter{
			filters: initStreamFiltersList(tenantIDs, idb, t.filters),
		}
	case *notFilter:
		return &notFilter{
			f: initStreamFilters(tenantIDs, idb, t.f),
		}
	case *streamFilter:
		return &streamFilter{
			f:         t.f,
			tenantIDs: tenantIDs,
			idb:       idb,
		}
	default:
		return t
	}
}

func initStreamFiltersList(tenantIDs []TenantID, idb *indexdb, filters []filter) []filter {
	result := make([]filter, len(filters))
	for i, f := range filters {
		result[i] = initStreamFilters(tenantIDs, idb, f)
	}
	return result
}

func (ddb *datadb) search(pwsDst []*partWrapper, so *searchOptions, workCh chan<- []*blockSearchWork, stopCh <-chan struct{}) []*partWrapper {
	// Select parts with data for the given time range
	ddb.partsLock.Lock()
	pwsDstLen := len(pwsDst)
	pwsDst = appendPartsInTimeRange(pwsDst, ddb.inmemoryParts, so.minTimestamp, so.maxTimestamp)
	pwsDst = appendPartsInTimeRange(pwsDst, ddb.fileParts, so.minTimestamp, so.maxTimestamp)
	pws := pwsDst[pwsDstLen:]
	for _, pw := range pws {
		pw.incRef()
	}
	ddb.partsLock.Unlock()

	// Apply search to matching parts
	for _, pw := range pws {
		pw.p.search(so, workCh, stopCh)
	}

	return pwsDst
}

func (p *part) search(so *searchOptions, workCh chan<- []*blockSearchWork, stopCh <-chan struct{}) {
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

func (p *part) searchByTenantIDs(so *searchOptions, bhss *blockHeaders, workCh chan<- []*blockSearchWork, stopCh <-chan struct{}) {
	// it is assumed that tenantIDs are sorted
	tenantIDs := so.tenantIDs

	bsws := make([]*blockSearchWork, 0, blockSearchWorksPerBatch)
	scheduleBlockSearch := func(bh *blockHeader) bool {
		// Do not use pool for blockSearchWork, since it is returned back to the pool
		// at another goroutine, which may run on another CPU core.
		// This means that it will be put into another per-CPU pool, which may result
		// in slowdown related to memory synchronization between CPU cores.
		// This slowdown is increased on systems with bigger number of CPU cores.
		bsw := newBlockSearchWork(p, so, bh)
		bsws = append(bsws, bsw)
		if len(bsws) < cap(bsws) {
			return true
		}
		select {
		case <-stopCh:
			return false
		case workCh <- bsws:
			bsws = make([]*blockSearchWork, 0, blockSearchWorksPerBatch)
			return true
		}
	}

	// it is assumed that ibhs are sorted
	ibhs := p.indexBlockHeaders
	for len(ibhs) > 0 && len(tenantIDs) > 0 {
		select {
		case <-stopCh:
			return
		default:
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
	if len(bsws) > 0 {
		workCh <- bsws
	}
}

func (p *part) searchByStreamIDs(so *searchOptions, bhss *blockHeaders, workCh chan<- []*blockSearchWork, stopCh <-chan struct{}) {
	// it is assumed that streamIDs are sorted
	streamIDs := so.streamIDs

	bsws := make([]*blockSearchWork, 0, blockSearchWorksPerBatch)
	scheduleBlockSearch := func(bh *blockHeader) bool {
		// Do not use pool for blockSearchWork, since it is returned back to the pool
		// at another goroutine, which may run on another CPU core.
		// This means that it will be put into another per-CPU pool, which may result
		// in slowdown related to memory synchronization between CPU cores.
		// This slowdown is increased on systems with bigger number of CPU cores.
		bsw := newBlockSearchWork(p, so, bh)
		bsws = append(bsws, bsw)
		if len(bsws) < cap(bsws) {
			return true
		}
		select {
		case <-stopCh:
			return false
		case workCh <- bsws:
			bsws = make([]*blockSearchWork, 0, blockSearchWorksPerBatch)
			return true
		}
	}

	// it is assumed that ibhs are sorted
	ibhs := p.indexBlockHeaders

	for len(ibhs) > 0 && len(streamIDs) > 0 {
		select {
		case <-stopCh:
			return
		default:
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
	if len(bsws) > 0 {
		workCh <- bsws
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
	case *andFilter:
		filters := t.filters
		for i, filter := range filters {
			sf, ok := filter.(*streamFilter)
			if ok && !sf.f.isEmpty() {
				// Remove sf from filters, since it doesn't filter out anything then.
				af := &andFilter{
					filters: append(filters[:i:i], filters[i+1:]...),
				}
				return sf.f, af
			}
		}
	case *streamFilter:
		return t.f, &noopFilter{}
	}
	return nil, f
}

func getCommonTimeFilter(f filter) (*timeFilter, filter) {
	switch t := f.(type) {
	case *andFilter:
		for _, filter := range t.filters {
			tf, ok := filter.(*timeFilter)
			if ok {
				// The tf must remain in af in order to properly filter out rows outside the selected time range
				return tf, f
			}
		}
	case *timeFilter:
		return t, f
	}
	return allTimeFilter, f
}

var allTimeFilter = &timeFilter{
	minTimestamp: math.MinInt64,
	maxTimestamp: math.MaxInt64,
}
