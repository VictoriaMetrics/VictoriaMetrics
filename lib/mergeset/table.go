package mergeset

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// maxInmemoryParts is the maximum number of inmemory parts in the table.
//
// This limit allows reducing CPU usage under high ingestion rate.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5212
//
// This number may be reached when the insertion pace outreaches merger pace.
// If this number is reached, then the data ingestion is paused until background
// mergers reduce the number of parts below this number.
const maxInmemoryParts = 30

// Default number of parts to merge at once.
//
// This number has been obtained empirically - it gives the lowest possible overhead.
// See appendPartsToMerge tests for details.
const defaultPartsToMerge = 15

// maxPartSize is the maximum part size in bytes.
//
// This number should be limited by the amount of time required to merge parts of this summary size.
// The required time shouldn't exceed a day.
const maxPartSize = 400e9

// The interval for flushing buffered data to parts, so it becomes visible to search.
const pendingItemsFlushInterval = time.Second

// maxItemsPerCachedPart is the maximum items per created part by the merge,
// which must be cached in the OS page cache.
//
// Such parts are usually frequently accessed, so it is good to cache their
// contents in OS page cache.
func maxItemsPerCachedPart() uint64 {
	mem := memory.Remaining()
	// Production data shows that each item occupies ~4 bytes in the compressed part.
	// It is expected no more than defaultPartsToMerge/2 parts exist
	// in the OS page cache before they are merged into bigger part.
	// Halft of the remaining RAM must be left for lib/storage parts,
	// so the maxItems is calculated using the below code:
	maxItems := uint64(mem) / (4 * defaultPartsToMerge)
	if maxItems < 1e6 {
		maxItems = 1e6
	}
	return maxItems
}

// Table represents mergeset table.
type Table struct {
	activeInmemoryMerges atomic.Int64
	activeFileMerges     atomic.Int64

	inmemoryMergesCount atomic.Uint64
	fileMergesCount     atomic.Uint64

	inmemoryItemsMerged atomic.Uint64
	fileItemsMerged     atomic.Uint64

	itemsAdded          atomic.Uint64
	itemsAddedSizeBytes atomic.Uint64

	inmemoryPartsLimitReachedCount atomic.Uint64

	mergeIdx atomic.Uint64

	path string

	// The interval for guaranteed flush of recently ingested data from memory to on-disk parts so they survive process crash.
	flushInterval time.Duration

	flushCallback         func()
	needFlushCallbackCall atomic.Bool

	prepareBlock PrepareBlockCallback
	isReadOnly   *atomic.Bool

	// rawItems contains recently added items that haven't been converted to parts yet.
	//
	// rawItems are converted to inmemoryParts at least every pendingItemsFlushInterval or when rawItems becomes full.
	//
	// rawItems aren't visible for search due to performance reasons.
	rawItems rawItemsShards

	// partsLock protects inmemoryParts and fileParts.
	partsLock sync.Mutex

	// inmemoryParts contains inmemory parts, which are visible for search.
	inmemoryParts []*partWrapper

	// fileParts contains file-backed parts, which are visible for search.
	fileParts []*partWrapper

	// inmemoryPartsLimitCh limits the number of inmemory parts to maxInmemoryParts
	// in order to prevent from data ingestion slowdown as described at https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5212
	inmemoryPartsLimitCh chan struct{}

	// stopCh is used for notifying all the background workers to stop.
	//
	// It must be closed under partsLock in order to prevent from calling wg.Add()
	// after stopCh is closed.
	stopCh chan struct{}

	// wg is used for waiting for all the background workers to stop.
	//
	// wg.Add() must be called under partsLock after checking whether stopCh isn't closed.
	// This should prevent from calling wg.Add() after stopCh is closed and wg.Wait() is called.
	wg sync.WaitGroup

	// Use syncwg instead of sync, since Add/Wait may be called from concurrent goroutines.
	flushPendingItemsWG syncwg.WaitGroup
}

type rawItemsShards struct {
	flushDeadlineMs atomic.Int64

	shardIdx atomic.Uint32

	// shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawItemsShard

	ibsToFlushLock sync.Mutex
	ibsToFlush     []*inmemoryBlock
}

// The number of shards for rawItems per table.
//
// Higher number of shards reduces CPU contention and increases the max bandwidth on multi-core systems.
var rawItemsShardsPerTable = func() int {
	cpus := cgroup.AvailableCPUs()
	multiplier := cpus
	if multiplier > 16 {
		multiplier = 16
	}
	return cpus * multiplier
}()

var maxBlocksPerShard = 256

func (riss *rawItemsShards) init() {
	riss.shards = make([]rawItemsShard, rawItemsShardsPerTable)
}

func (riss *rawItemsShards) addItems(tb *Table, items [][]byte) {
	shards := riss.shards
	shardsLen := uint32(len(shards))
	for len(items) > 0 {
		n := riss.shardIdx.Add(1)
		idx := n % shardsLen
		tailItems, ibsToFlush := shards[idx].addItems(items)
		riss.addIbsToFlush(tb, ibsToFlush)
		items = tailItems
	}
}

func (riss *rawItemsShards) addIbsToFlush(tb *Table, ibsToFlush []*inmemoryBlock) {
	if len(ibsToFlush) == 0 {
		return
	}

	var ibsToMerge []*inmemoryBlock

	riss.ibsToFlushLock.Lock()
	if len(riss.ibsToFlush) == 0 {
		riss.updateFlushDeadline()
	}
	riss.ibsToFlush = append(riss.ibsToFlush, ibsToFlush...)
	if len(riss.ibsToFlush) >= maxBlocksPerShard*cgroup.AvailableCPUs() {
		ibsToMerge = riss.ibsToFlush
		riss.ibsToFlush = nil
	}
	riss.ibsToFlushLock.Unlock()

	tb.flushBlocksToInmemoryParts(ibsToMerge, false)
}

func (riss *rawItemsShards) Len() int {
	n := 0
	for i := range riss.shards {
		n += riss.shards[i].Len()
	}
	return n
}

func (riss *rawItemsShards) updateFlushDeadline() {
	riss.flushDeadlineMs.Store(time.Now().Add(pendingItemsFlushInterval).UnixMilli())
}

type rawItemsShardNopad struct {
	flushDeadlineMs atomic.Int64

	mu  sync.Mutex
	ibs []*inmemoryBlock
}

type rawItemsShard struct {
	rawItemsShardNopad

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(rawItemsShardNopad{})%atomicutil.CacheLineSize]byte
}

func (ris *rawItemsShard) Len() int {
	ris.mu.Lock()
	n := 0
	for _, ib := range ris.ibs {
		n += len(ib.items)
	}
	ris.mu.Unlock()
	return n
}

func (ris *rawItemsShard) addItems(items [][]byte) ([][]byte, []*inmemoryBlock) {
	var ibsToFlush []*inmemoryBlock
	var tailItems [][]byte

	ris.mu.Lock()
	ibs := ris.ibs
	if len(ibs) == 0 {
		ibs = append(ibs, &inmemoryBlock{})
		ris.updateFlushDeadline()
		ris.ibs = ibs
	}
	ib := ibs[len(ibs)-1]
	for i, item := range items {
		if ib.Add(item) {
			continue
		}
		if len(ibs) >= maxBlocksPerShard {
			ibsToFlush = append(ibsToFlush, ibs...)
			ibs = make([]*inmemoryBlock, 0, maxBlocksPerShard)
			tailItems = items[i:]
			break
		}
		ib = &inmemoryBlock{}
		if ib.Add(item) {
			ibs = append(ibs, ib)
			continue
		}

		// Skip too long item
		itemPrefix := item
		if len(itemPrefix) > 128 {
			itemPrefix = itemPrefix[:128]
		}
		tooLongItemsTotal.Add(1)
		tooLongItemLogger.Errorf("skipping adding too long item to indexdb: len(item)=%d; it shouldn't exceed %d bytes; item prefix=%q", len(item), maxInmemoryBlockSize, itemPrefix)
	}
	ris.ibs = ibs
	ris.mu.Unlock()

	return tailItems, ibsToFlush
}

func (ris *rawItemsShard) updateFlushDeadline() {
	ris.flushDeadlineMs.Store(time.Now().Add(pendingItemsFlushInterval).UnixMilli())
}

var tooLongItemLogger = logger.WithThrottler("tooLongItem", 5*time.Second)

var tooLongItemsTotal atomicutil.Uint64

type partWrapper struct {
	// refCount is the number of references to partWrapper
	refCount atomic.Int32

	// mustDrop marks partWrapper for deletion.
	// This field should be updated only after partWrapper
	// was removed from the list of active parts.
	mustDrop atomic.Bool

	p *part

	mp *inmemoryPart

	isInMerge bool

	// The deadline when the in-memory part must be flushed to disk.
	flushToDiskDeadline time.Time
}

func (pw *partWrapper) incRef() {
	pw.refCount.Add(1)
}

func (pw *partWrapper) decRef() {
	n := pw.refCount.Add(-1)
	if n < 0 {
		logger.Panicf("BUG: pw.refCount must be bigger than 0; got %d", n)
	}
	if n > 0 {
		return
	}

	deletePath := ""
	if pw.mp == nil && pw.mustDrop.Load() {
		deletePath = pw.p.path
	}
	if pw.mp != nil {
		// Do not return pw.mp to pool via putInmemoryPart(),
		// since pw.mp size may be too big compared to other entries stored in the pool.
		// This may result in increased memory usage because of high fragmentation.
		pw.mp = nil
	}
	pw.p.MustClose()
	pw.p = nil

	if deletePath != "" {
		fs.MustRemoveDir(deletePath)
	}
}

// MustOpenTable opens a table on the given path.
//
// The flushInterval is the interval for flushing pending in-memory data to disk.
//
// Optional flushCallback is called every time new data batch is flushed
// to the underlying storage and becomes visible to search.
//
// Optional prepareBlock is called during merge before flushing the prepared block
// to persistent storage.
//
// The table is created if it doesn't exist yet.
func MustOpenTable(path string, flushInterval time.Duration, flushCallback func(), prepareBlock PrepareBlockCallback, isReadOnly *atomic.Bool) *Table {
	path = filepath.Clean(path)

	if flushInterval < pendingItemsFlushInterval {
		// There is no sense in setting flushInterval to values smaller than pendingItemsFlushInterval,
		// since pending rows unconditionally remain in memory for up to pendingItemsFlushInterval.
		flushInterval = pendingItemsFlushInterval
	}

	// Create a directory for the table if it doesn't exist yet.
	fs.MustMkdirIfNotExist(path)

	// Open table parts.
	pws := mustOpenParts(path)

	tb := &Table{
		path:                 path,
		flushInterval:        flushInterval,
		flushCallback:        flushCallback,
		prepareBlock:         prepareBlock,
		isReadOnly:           isReadOnly,
		fileParts:            pws,
		inmemoryPartsLimitCh: make(chan struct{}, maxInmemoryParts),
		stopCh:               make(chan struct{}),
	}
	tb.mergeIdx.Store(uint64(time.Now().UnixNano()))
	tb.rawItems.init()
	tb.startBackgroundWorkers()

	return tb
}

func (tb *Table) startBackgroundWorkers() {
	// Start file parts mergers, so they could start merging unmerged parts if needed.
	// There is no need in starting in-memory parts mergers, since there are no in-memory parts yet.
	tb.startFilePartsMergers()

	tb.startPendingItemsFlusher()
	tb.startInmemoryPartsFlusher()
	tb.startFlushCallbackWorker()
}

func (tb *Table) startInmemoryPartsMergers() {
	tb.partsLock.Lock()
	for i := 0; i < cap(inmemoryPartsConcurrencyCh); i++ {
		tb.startInmemoryPartsMergerLocked()
	}
	tb.partsLock.Unlock()
}

func (tb *Table) startInmemoryPartsMergerLocked() {
	select {
	case <-tb.stopCh:
		return
	default:
	}
	tb.wg.Add(1)
	go func() {
		tb.inmemoryPartsMerger()
		tb.wg.Done()
	}()
}

func (tb *Table) startFilePartsMergers() {
	tb.partsLock.Lock()
	for i := 0; i < cap(filePartsConcurrencyCh); i++ {
		tb.startFilePartsMergerLocked()
	}
	tb.partsLock.Unlock()
}

func (tb *Table) startFilePartsMergerLocked() {
	select {
	case <-tb.stopCh:
		return
	default:
	}
	tb.wg.Add(1)
	go func() {
		tb.filePartsMerger()
		tb.wg.Done()
	}()
}

func (tb *Table) startPendingItemsFlusher() {
	tb.wg.Add(1)
	go func() {
		tb.pendingItemsFlusher()
		tb.wg.Done()
	}()
}

func (tb *Table) startInmemoryPartsFlusher() {
	tb.wg.Add(1)
	go func() {
		tb.inmemoryPartsFlusher()
		tb.wg.Done()
	}()
}

func (tb *Table) startFlushCallbackWorker() {
	if tb.flushCallback == nil {
		return
	}

	tb.wg.Add(1)
	go func() {
		// call flushCallback once per 10 seconds in order to improve the effectiveness of caches,
		// which are reset by the flushCallback.
		d := timeutil.AddJitterToDuration(time.Second * 10)
		tc := time.NewTicker(d)
		for {
			select {
			case <-tb.stopCh:
				tc.Stop()
				tb.flushCallback()
				tb.wg.Done()
				return
			case <-tc.C:
				if tb.needFlushCallbackCall.CompareAndSwap(true, false) {
					tb.flushCallback()
				}
			}
		}
	}()
}

var (
	inmemoryPartsConcurrencyCh = make(chan struct{}, getInmemoryPartsConcurrency())
	filePartsConcurrencyCh     = make(chan struct{}, getFilePartsConcurrency())
)

func getInmemoryPartsConcurrency() int {
	// The concurrency for processing in-memory parts must equal to the number of CPU cores,
	// since these operations are CPU-bound.
	return cgroup.AvailableCPUs()
}

func getFilePartsConcurrency() int {
	n := cgroup.AvailableCPUs()
	if n < 4 {
		// Allow at least 4 concurrent workers for file parts on systems
		// with less than 4 CPU cores in order to be able to make small file merges
		// when big file merges are in progress.
		return 4
	}
	return n
}

// MustClose closes the table.
//
// This func must be called only when there are no goroutines using the the
// table, such as ones that ingest or retrieve index data.
func (tb *Table) MustClose() {
	// Notify background workers to stop.
	// The tb.partsLock is acquired in order to guarantee that tb.wg.Add() isn't called
	// after tb.stopCh is closed and tb.wg.Wait() is called below.
	tb.partsLock.Lock()
	close(tb.stopCh)
	tb.partsLock.Unlock()

	// Wait for background workers to stop.
	tb.wg.Wait()

	// Flush the remaining in-memory items to files.
	tb.flushInmemoryItemsToFiles()

	// Remove references to parts from the tb, so they may be eventually closed after all the searches are done.
	tb.partsLock.Lock()

	if n := tb.rawItems.Len(); n > 0 {
		logger.Panicf("BUG: raw items must be empty at this stage; got %d items", n)
	}

	if n := len(tb.inmemoryParts); n > 0 {
		logger.Panicf("BUG: in-memory parts must be empty at this stage; got %d parts", n)
	}
	tb.inmemoryParts = nil

	fileParts := tb.fileParts
	tb.fileParts = nil

	tb.partsLock.Unlock()

	for _, pw := range fileParts {
		pw.decRef()
	}
}

// Path returns the path to tb on the filesystem.
func (tb *Table) Path() string {
	return tb.path
}

// TableMetrics contains essential metrics for the Table.
type TableMetrics struct {
	ActiveInmemoryMerges uint64
	ActiveFileMerges     uint64

	InmemoryMergesCount uint64
	FileMergesCount     uint64

	InmemoryItemsMerged uint64
	FileItemsMerged     uint64

	ItemsAdded          uint64
	ItemsAddedSizeBytes uint64

	InmemoryPartsLimitReachedCount uint64

	PendingItems uint64

	InmemoryPartsCount uint64
	FilePartsCount     uint64

	InmemoryBlocksCount uint64
	FileBlocksCount     uint64

	InmemoryItemsCount uint64
	FileItemsCount     uint64

	InmemorySizeBytes uint64
	FileSizeBytes     uint64

	DataBlocksCacheSize         uint64
	DataBlocksCacheSizeBytes    uint64
	DataBlocksCacheSizeMaxBytes uint64
	DataBlocksCacheRequests     uint64
	DataBlocksCacheMisses       uint64

	DataBlocksSparseCacheSize         uint64
	DataBlocksSparseCacheSizeBytes    uint64
	DataBlocksSparseCacheSizeMaxBytes uint64
	DataBlocksSparseCacheRequests     uint64
	DataBlocksSparseCacheMisses       uint64

	IndexBlocksCacheSize         uint64
	IndexBlocksCacheSizeBytes    uint64
	IndexBlocksCacheSizeMaxBytes uint64
	IndexBlocksCacheRequests     uint64
	IndexBlocksCacheMisses       uint64

	PartsRefCount uint64

	TooLongItemsDroppedTotal uint64
}

// TotalItemsCount returns the total number of items in the table.
func (tm *TableMetrics) TotalItemsCount() uint64 {
	return tm.InmemoryItemsCount + tm.FileItemsCount
}

// UpdateMetrics updates m with metrics from tb.
func (tb *Table) UpdateMetrics(m *TableMetrics) {
	m.ActiveInmemoryMerges += uint64(tb.activeInmemoryMerges.Load())
	m.ActiveFileMerges += uint64(tb.activeFileMerges.Load())

	m.InmemoryMergesCount += tb.inmemoryMergesCount.Load()
	m.FileMergesCount += tb.fileMergesCount.Load()

	m.InmemoryItemsMerged += tb.inmemoryItemsMerged.Load()
	m.FileItemsMerged += tb.fileItemsMerged.Load()

	m.ItemsAdded += tb.itemsAdded.Load()
	m.ItemsAddedSizeBytes += tb.itemsAddedSizeBytes.Load()

	m.InmemoryPartsLimitReachedCount += tb.inmemoryPartsLimitReachedCount.Load()

	m.PendingItems += uint64(tb.rawItems.Len())

	tb.partsLock.Lock()

	m.InmemoryPartsCount += uint64(len(tb.inmemoryParts))
	for _, pw := range tb.inmemoryParts {
		p := pw.p
		m.InmemoryBlocksCount += p.ph.blocksCount
		m.InmemoryItemsCount += p.ph.itemsCount
		m.InmemorySizeBytes += p.size
		m.PartsRefCount += uint64(pw.refCount.Load())
	}

	m.FilePartsCount += uint64(len(tb.fileParts))
	for _, pw := range tb.fileParts {
		p := pw.p
		m.FileBlocksCount += p.ph.blocksCount
		m.FileItemsCount += p.ph.itemsCount
		m.FileSizeBytes += p.size
		m.PartsRefCount += uint64(pw.refCount.Load())
	}
	tb.partsLock.Unlock()

	m.DataBlocksCacheSize = uint64(ibCache.Len())
	m.DataBlocksCacheSizeBytes = uint64(ibCache.SizeBytes())
	m.DataBlocksCacheSizeMaxBytes = uint64(ibCache.SizeMaxBytes())
	m.DataBlocksCacheRequests = ibCache.Requests()
	m.DataBlocksCacheMisses = ibCache.Misses()

	m.DataBlocksSparseCacheSize = uint64(ibSparseCache.Len())
	m.DataBlocksSparseCacheSizeBytes = uint64(ibSparseCache.SizeBytes())
	m.DataBlocksSparseCacheSizeMaxBytes = uint64(ibSparseCache.SizeMaxBytes())
	m.DataBlocksSparseCacheRequests = ibSparseCache.Requests()
	m.DataBlocksSparseCacheMisses = ibSparseCache.Misses()

	m.IndexBlocksCacheSize = uint64(idxbCache.Len())
	m.IndexBlocksCacheSizeBytes = uint64(idxbCache.SizeBytes())
	m.IndexBlocksCacheSizeMaxBytes = uint64(idxbCache.SizeMaxBytes())
	m.IndexBlocksCacheRequests = idxbCache.Requests()
	m.IndexBlocksCacheMisses = idxbCache.Misses()

	m.TooLongItemsDroppedTotal = tooLongItemsTotal.Load()
}

// AddItems adds the given items to the tb.
//
// The function ignores items with length exceeding maxInmemoryBlockSize.
// It logs the ignored items, so users could notice and fix the issue.
func (tb *Table) AddItems(items [][]byte) {
	tb.rawItems.addItems(tb, items)
	tb.itemsAdded.Add(uint64(len(items)))
	n := 0
	for _, item := range items {
		n += len(item)
	}
	tb.itemsAddedSizeBytes.Add(uint64(n))
}

// getParts appends parts snapshot to dst and returns it.
//
// The appended parts must be released with putParts.
func (tb *Table) getParts(dst []*partWrapper) []*partWrapper {
	tb.partsLock.Lock()
	for _, pw := range tb.inmemoryParts {
		pw.incRef()
	}
	for _, pw := range tb.fileParts {
		pw.incRef()
	}
	dst = append(dst, tb.inmemoryParts...)
	dst = append(dst, tb.fileParts...)
	tb.partsLock.Unlock()

	return dst
}

// putParts releases the given pws obtained via getParts.
func (tb *Table) putParts(pws []*partWrapper) {
	for _, pw := range pws {
		pw.decRef()
	}
}

func (tb *Table) mergeInmemoryPartsToFiles(pws []*partWrapper) error {
	pwsLen := len(pws)

	var errGlobal error
	var errGlobalLock sync.Mutex
	wg := getWaitGroup()
	for len(pws) > 0 {
		pwsToMerge, pwsRemaining := getPartsForOptimalMerge(pws)
		wg.Add(1)
		inmemoryPartsConcurrencyCh <- struct{}{}
		go func(pwsChunk []*partWrapper) {
			defer func() {
				<-inmemoryPartsConcurrencyCh
				wg.Done()
			}()

			if err := tb.mergeParts(pwsChunk, nil, true); err != nil {
				// There is no need for errors.Is(err, errForciblyStopped) check here, since stopCh=nil is passed to mergeParts.
				errGlobalLock.Lock()
				if errGlobal == nil {
					errGlobal = err
				}
				errGlobalLock.Unlock()
			}
		}(pwsToMerge)
		pws = pwsRemaining
	}
	wg.Wait()
	putWaitGroup(wg)

	if errGlobal != nil {
		return fmt.Errorf("cannot optimally merge %d parts: %w", pwsLen, errGlobal)
	}
	return nil
}

// DebugFlush makes sure all the recently added data is visible to search.
//
// Note: this function doesn't store all the in-memory data to disk - it just converts
// recently added items to searchable parts, which can be stored either in memory
// (if they are quite small) or to persistent disk.
//
// This function is for debugging and testing purposes only,
// since it may slow down data ingestion when used frequently.
func (tb *Table) DebugFlush() {
	tb.flushPendingItems(true)

	// Wait for background flushers to finish.
	tb.flushPendingItemsWG.Wait()
}

func (tb *Table) pendingItemsFlusher() {
	// do not add jitter in order to guarantee flush interval
	d := pendingItemsFlushInterval
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			tb.flushPendingItems(false)
		}
	}
}

func (tb *Table) inmemoryPartsFlusher() {
	// do not add jitter in order to guarantee flush interval
	ticker := time.NewTicker(tb.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			tb.flushInmemoryPartsToFiles(false)
		}
	}
}

func (tb *Table) flushPendingItems(isFinal bool) {
	tb.flushPendingItemsWG.Add(1)
	tb.rawItems.flush(tb, isFinal)
	tb.flushPendingItemsWG.Done()
}

func (tb *Table) flushInmemoryItemsToFiles() {
	tb.flushPendingItems(true)
	tb.flushInmemoryPartsToFiles(true)
}

func (tb *Table) flushInmemoryPartsToFiles(isFinal bool) {
	currentTime := time.Now()
	var pws []*partWrapper

	tb.partsLock.Lock()
	for _, pw := range tb.inmemoryParts {
		if !pw.isInMerge && (isFinal || pw.flushToDiskDeadline.Before(currentTime)) {
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	tb.partsLock.Unlock()

	if err := tb.mergeInmemoryPartsToFiles(pws); err != nil {
		logger.Panicf("FATAL: cannot merge in-memory parts to files: %s", err)
	}
}

func (riss *rawItemsShards) flush(tb *Table, isFinal bool) {
	var dst []*inmemoryBlock

	currentTimeMs := time.Now().UnixMilli()
	flushDeadlineMs := riss.flushDeadlineMs.Load()
	if isFinal || currentTimeMs >= flushDeadlineMs {
		riss.ibsToFlushLock.Lock()
		dst = riss.ibsToFlush
		riss.ibsToFlush = nil
		riss.ibsToFlushLock.Unlock()
	}

	for i := range riss.shards {
		dst = riss.shards[i].appendBlocksToFlush(dst, currentTimeMs, isFinal)
	}

	tb.flushBlocksToInmemoryParts(dst, isFinal)
}

func (ris *rawItemsShard) appendBlocksToFlush(dst []*inmemoryBlock, currentTimeMs int64, isFinal bool) []*inmemoryBlock {
	flushDeadlineMs := ris.flushDeadlineMs.Load()
	if !isFinal && currentTimeMs < flushDeadlineMs {
		// Fast path - nothing to flush
		return dst
	}

	// Slow path - move ris.ibs to dst
	ris.mu.Lock()
	ibs := ris.ibs
	dst = append(dst, ibs...)
	for i := range ibs {
		ibs[i] = nil
	}
	ris.ibs = ibs[:0]
	ris.mu.Unlock()

	return dst
}

func (tb *Table) flushBlocksToInmemoryParts(ibs []*inmemoryBlock, isFinal bool) {
	if len(ibs) == 0 {
		return
	}

	// Merge ibs into in-memory parts.
	var pwsLock sync.Mutex
	pws := make([]*partWrapper, 0, (len(ibs)+defaultPartsToMerge-1)/defaultPartsToMerge)
	wg := getWaitGroup()
	for len(ibs) > 0 {
		n := defaultPartsToMerge
		if n > len(ibs) {
			n = len(ibs)
		}
		wg.Add(1)
		inmemoryPartsConcurrencyCh <- struct{}{}
		go func(ibsChunk []*inmemoryBlock) {
			defer func() {
				<-inmemoryPartsConcurrencyCh
				wg.Done()
			}()

			if pw := tb.createInmemoryPart(ibsChunk); pw != nil {
				pwsLock.Lock()
				pws = append(pws, pw)
				pwsLock.Unlock()
			}
			// Clear references to ibsChunk items, so they may be reclaimed faster by Go GC.
			for i := range ibsChunk {
				ibsChunk[i] = nil
			}
		}(ibs[:n])
		ibs = ibs[n:]
	}
	wg.Wait()
	putWaitGroup(wg)

	// Merge pws into a single in-memory part.
	maxPartSize := getMaxInmemoryPartSize()
	for len(pws) > 1 {
		pws = tb.mustMergeInmemoryParts(pws)

		pwsRemaining := pws[:0]
		for _, pw := range pws {
			if pw.p.size >= maxPartSize {
				tb.addToInmemoryParts(pw, isFinal)
			} else {
				pwsRemaining = append(pwsRemaining, pw)
			}
		}
		pws = pwsRemaining
	}
	if len(pws) == 1 {
		tb.addToInmemoryParts(pws[0], isFinal)
	}
}

func (tb *Table) addToInmemoryParts(pw *partWrapper, isFinal bool) {
	// Wait until the number of in-memory parts goes below maxInmemoryParts.
	// This prevents from excess CPU usage during search in tb under high ingestion rate to tb.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5212
	select {
	case tb.inmemoryPartsLimitCh <- struct{}{}:
	default:
		tb.inmemoryPartsLimitReachedCount.Add(1)
		select {
		case tb.inmemoryPartsLimitCh <- struct{}{}:
		case <-tb.stopCh:
		}
	}

	tb.partsLock.Lock()
	tb.inmemoryParts = append(tb.inmemoryParts, pw)
	tb.startInmemoryPartsMergerLocked()
	tb.partsLock.Unlock()

	if tb.flushCallback != nil {
		if isFinal {
			tb.flushCallback()
		} else {
			// Use Load in front of CompareAndSwap in order to avoid slow inter-CPU synchronization
			// at fast path when needFlushCallbackCall is already set to true.
			if !tb.needFlushCallbackCall.Load() {
				tb.needFlushCallbackCall.CompareAndSwap(false, true)
			}
		}
	}
}

func getWaitGroup() *sync.WaitGroup {
	v := wgPool.Get()
	if v == nil {
		return &sync.WaitGroup{}
	}
	return v.(*sync.WaitGroup)
}

func putWaitGroup(wg *sync.WaitGroup) {
	wgPool.Put(wg)
}

var wgPool sync.Pool

func (tb *Table) mustMergeInmemoryParts(pws []*partWrapper) []*partWrapper {
	var pwsResult []*partWrapper
	var pwsResultLock sync.Mutex
	wg := getWaitGroup()
	for len(pws) > 0 {
		pwsToMerge, pwsRemaining := getPartsForOptimalMerge(pws)
		wg.Add(1)
		inmemoryPartsConcurrencyCh <- struct{}{}
		go func(pwsChunk []*partWrapper) {
			defer func() {
				<-inmemoryPartsConcurrencyCh
				wg.Done()
			}()

			pw := tb.mustMergeInmemoryPartsFinal(pwsChunk)

			pwsResultLock.Lock()
			pwsResult = append(pwsResult, pw)
			pwsResultLock.Unlock()
		}(pwsToMerge)
		pws = pwsRemaining
	}
	wg.Wait()
	putWaitGroup(wg)

	return pwsResult
}

func (tb *Table) mustMergeInmemoryPartsFinal(pws []*partWrapper) *partWrapper {
	if len(pws) == 0 {
		logger.Panicf("BUG: pws must contain at least a single item")
	}
	if len(pws) == 1 {
		// Nothing to merge
		return pws[0]
	}

	bsrs := make([]*blockStreamReader, 0, len(pws))
	for _, pw := range pws {
		if pw.mp == nil {
			logger.Panicf("BUG: unexpected file part")
		}
		bsr := getBlockStreamReader()
		bsr.MustInitFromInmemoryPart(pw.mp)
		bsrs = append(bsrs, bsr)
	}

	flushToDiskDeadline := getFlushToDiskDeadline(pws, tb.flushInterval)
	return tb.mustMergeIntoInmemoryPart(bsrs, flushToDiskDeadline)
}

func (tb *Table) createInmemoryPart(ibs []*inmemoryBlock) *partWrapper {
	// Prepare blockStreamReaders for source blocks.
	bsrs := make([]*blockStreamReader, 0, len(ibs))
	for _, ib := range ibs {
		if len(ib.items) == 0 {
			continue
		}
		bsr := getBlockStreamReader()
		bsr.MustInitFromInmemoryBlock(ib)
		bsrs = append(bsrs, bsr)
	}
	if len(bsrs) == 0 {
		return nil
	}

	flushToDiskDeadline := time.Now().Add(tb.flushInterval)
	if len(bsrs) == 1 {
		// Nothing to merge. Just return a single inmemory part.
		bsr := bsrs[0]
		mp := &inmemoryPart{}
		mp.Init(&bsr.Block)
		putBlockStreamReader(bsr)
		return newPartWrapperFromInmemoryPart(mp, flushToDiskDeadline)
	}

	return tb.mustMergeIntoInmemoryPart(bsrs, flushToDiskDeadline)
}

func (tb *Table) mustMergeIntoInmemoryPart(bsrs []*blockStreamReader, flushToDiskDeadline time.Time) *partWrapper {
	// Prepare blockStreamWriter for destination part.
	outItemsCount := uint64(0)
	for _, bsr := range bsrs {
		outItemsCount += bsr.ph.itemsCount
	}
	compressLevel := getCompressLevel(outItemsCount)
	bsw := getBlockStreamWriter()
	mpDst := &inmemoryPart{}
	bsw.MustInitFromInmemoryPart(mpDst, compressLevel)

	// Merge parts.
	// The merge shouldn't be interrupted by stopCh, so use nil stopCh.
	ph, err := tb.mergePartsInternal("", bsw, bsrs, partInmemory, nil)
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	if err != nil {
		logger.Panicf("FATAL: cannot merge inmemoryBlocks: %s", err)
	}
	mpDst.ph = *ph

	return newPartWrapperFromInmemoryPart(mpDst, flushToDiskDeadline)
}

func newPartWrapperFromInmemoryPart(mp *inmemoryPart, flushToDiskDeadline time.Time) *partWrapper {
	p := mp.NewPart()
	pw := &partWrapper{
		p:                   p,
		mp:                  mp,
		flushToDiskDeadline: flushToDiskDeadline,
	}
	pw.incRef()
	return pw
}

func getMaxInmemoryPartSize() uint64 {
	// Allow up to 5% of memory for in-memory parts.
	n := uint64(0.05 * float64(memory.Allowed()) / maxInmemoryParts)
	if n < 1e6 {
		n = 1e6
	}
	return n
}

func (tb *Table) getMaxFilePartSize() uint64 {
	n := fs.MustGetFreeSpace(tb.path)
	// Divide free space by the max number of concurrent merges for file parts.
	maxOutBytes := n / uint64(cap(filePartsConcurrencyCh))
	if maxOutBytes > maxPartSize {
		maxOutBytes = maxPartSize
	}
	return maxOutBytes
}

// NotifyReadWriteMode notifies tb that it may be switched from read-only mode to read-write mode.
func (tb *Table) NotifyReadWriteMode() {
	tb.startInmemoryPartsMergers()
	tb.startFilePartsMergers()
}

func (tb *Table) inmemoryPartsMerger() {
	for {
		if tb.isReadOnly.Load() {
			return
		}
		maxOutBytes := tb.getMaxFilePartSize()

		tb.partsLock.Lock()
		pws := getPartsToMerge(tb.inmemoryParts, maxOutBytes)
		tb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		inmemoryPartsConcurrencyCh <- struct{}{}
		err := tb.mergeParts(pws, tb.stopCh, false)
		<-inmemoryPartsConcurrencyCh

		if err == nil {
			// Try merging additional parts.
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// Nothing to do - finish the merger.
			return
		}
		// Unexpected error.
		logger.Panicf("FATAL: unrecoverable error when merging inmemory parts in %q: %s", tb.path, err)
	}
}

func (tb *Table) filePartsMerger() {
	for {
		if tb.isReadOnly.Load() {
			return
		}
		maxOutBytes := tb.getMaxFilePartSize()

		tb.partsLock.Lock()
		pws := getPartsToMerge(tb.fileParts, maxOutBytes)
		tb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		filePartsConcurrencyCh <- struct{}{}
		err := tb.mergeParts(pws, tb.stopCh, false)
		<-filePartsConcurrencyCh

		if err == nil {
			// Try merging additional parts.
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// The merger has been stopped.
			return
		}
		// Unexpected error.
		logger.Panicf("FATAL: unrecoverable error when merging file parts in %q: %s", tb.path, err)
	}
}

func assertIsInMerge(pws []*partWrapper) {
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge unexpectedly set to false")
		}
	}
}

func (tb *Table) releasePartsToMerge(pws []*partWrapper) {
	tb.partsLock.Lock()
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: missing isInMerge flag on the part %q", pw.p.path)
		}
		pw.isInMerge = false
	}
	tb.partsLock.Unlock()
}

// mergeParts merges pws to a single resulting part.
//
// It is expected that pws contains at least a single part.
//
// Merging is immediately stopped if stopCh is closed.
//
// If isFinal is set, then the resulting part will be stored to disk.
// If at least a single source part at pws is stored on disk, then the resulting part
// will be stored to disk.
//
// All the parts inside pws must have isInMerge field set to true.
// The isInMerge field inside pws parts is set to false before returning from the function.
func (tb *Table) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal bool) error {
	if len(pws) == 0 {
		logger.Panicf("BUG: empty pws cannot be passed to mergeParts()")
	}

	assertIsInMerge(pws)
	defer tb.releasePartsToMerge(pws)

	startTime := time.Now()

	// Initialize destination paths.
	dstPartType := getDstPartType(pws, isFinal)
	mergeIdx := tb.nextMergeIdx()
	dstPartPath := ""
	if dstPartType == partFile {
		dstPartPath = filepath.Join(tb.path, fmt.Sprintf("%016X", mergeIdx))
	}

	if isFinal && len(pws) == 1 && pws[0].mp != nil {
		// Fast path: flush a single in-memory part to disk.
		mp := pws[0].mp
		mp.MustStoreToDisk(dstPartPath)
		pwNew := tb.openCreatedPart(pws, nil, dstPartPath)
		tb.swapSrcWithDstParts(pws, pwNew, dstPartType)
		return nil
	}

	// Prepare BlockStreamReaders for source parts.
	bsrs := mustOpenBlockStreamReaders(pws)

	// Prepare BlockStreamWriter for destination part.
	srcSize := uint64(0)
	srcItemsCount := uint64(0)
	srcBlocksCount := uint64(0)
	for _, pw := range pws {
		srcSize += pw.p.size
		srcItemsCount += pw.p.ph.itemsCount
		srcBlocksCount += pw.p.ph.blocksCount
	}
	compressLevel := getCompressLevel(srcItemsCount)
	bsw := getBlockStreamWriter()
	var mpNew *inmemoryPart
	if dstPartType == partInmemory {
		mpNew = &inmemoryPart{}
		bsw.MustInitFromInmemoryPart(mpNew, compressLevel)
	} else {
		nocache := srcItemsCount > maxItemsPerCachedPart()
		bsw.MustInitFromFilePart(dstPartPath, nocache, compressLevel)
	}

	// Merge source parts to destination part.
	ph, err := tb.mergePartsInternal(dstPartPath, bsw, bsrs, dstPartType, stopCh)
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	if err != nil {
		return err
	}
	if mpNew != nil {
		// Update partHeader for destination inmemory part after the merge.
		mpNew.ph = *ph
	} else {
		// Make sure the created part directory listing is synced.
		fs.MustSyncPathAndParentDir(dstPartPath)
	}

	// Atomically swap the source parts with the newly created part.
	pwNew := tb.openCreatedPart(pws, mpNew, dstPartPath)
	pDst := pwNew.p
	dstItemsCount := pDst.ph.itemsCount
	dstBlocksCount := pDst.ph.blocksCount
	dstSize := pDst.size

	tb.swapSrcWithDstParts(pws, pwNew, dstPartType)

	d := time.Since(startTime)
	if d <= 30*time.Second {
		return nil
	}

	// Log stats for long merges.
	durationSecs := d.Seconds()
	itemsPerSec := int(float64(srcItemsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d items, %d blocks, %d bytes) into (1 part, %d items, %d blocks, %d bytes) in %.3f seconds at %d items/sec to %q",
		len(pws), srcItemsCount, srcBlocksCount, srcSize, dstItemsCount, dstBlocksCount, dstSize, durationSecs, itemsPerSec, dstPartPath)

	return nil
}

func getFlushToDiskDeadline(pws []*partWrapper, flushInterval time.Duration) time.Time {
	d := time.Now().Add(flushInterval)
	for _, pw := range pws {
		if pw.mp != nil && pw.flushToDiskDeadline.Before(d) {
			d = pw.flushToDiskDeadline
		}
	}
	return d
}

type partType int

var (
	partInmemory = partType(0)
	partFile     = partType(1)
)

func getDstPartType(pws []*partWrapper, isFinal bool) partType {
	dstPartSize := getPartsSize(pws)
	if isFinal || dstPartSize > getMaxInmemoryPartSize() {
		return partFile
	}
	if !areAllInmemoryParts(pws) {
		// If at least a single source part is located in file,
		// then the destination part must be in file for durability reasons.
		return partFile
	}
	return partInmemory
}

func mustOpenBlockStreamReaders(pws []*partWrapper) []*blockStreamReader {
	bsrs := make([]*blockStreamReader, 0, len(pws))
	for _, pw := range pws {
		bsr := getBlockStreamReader()
		if pw.mp != nil {
			bsr.MustInitFromInmemoryPart(pw.mp)
		} else {
			bsr.MustInitFromFilePart(pw.p.path)
		}
		bsrs = append(bsrs, bsr)
	}
	return bsrs
}

func (tb *Table) mergePartsInternal(dstPartPath string, bsw *blockStreamWriter, bsrs []*blockStreamReader, dstPartType partType, stopCh <-chan struct{}) (*partHeader, error) {
	var ph partHeader
	var itemsMerged *atomic.Uint64
	var mergesCount *atomic.Uint64
	var activeMerges *atomic.Int64
	switch dstPartType {
	case partInmemory:
		itemsMerged = &tb.inmemoryItemsMerged
		mergesCount = &tb.inmemoryMergesCount
		activeMerges = &tb.activeInmemoryMerges
	case partFile:
		itemsMerged = &tb.fileItemsMerged
		mergesCount = &tb.fileMergesCount
		activeMerges = &tb.activeFileMerges
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}
	activeMerges.Add(1)
	err := mergeBlockStreams(&ph, bsw, bsrs, tb.prepareBlock, stopCh, itemsMerged)
	activeMerges.Add(-1)
	mergesCount.Add(1)
	if err != nil {
		return nil, fmt.Errorf("cannot merge %d parts to %s: %w", len(bsrs), dstPartPath, err)
	}
	if dstPartPath != "" {
		ph.MustWriteMetadata(dstPartPath)
	}
	return &ph, nil
}

func (tb *Table) openCreatedPart(pws []*partWrapper, mpNew *inmemoryPart, dstPartPath string) *partWrapper {
	// Open the created part.
	if mpNew != nil {
		// Open the created part from memory.
		flushToDiskDeadline := getFlushToDiskDeadline(pws, tb.flushInterval)
		pwNew := newPartWrapperFromInmemoryPart(mpNew, flushToDiskDeadline)
		return pwNew
	}
	// Open the created part from disk.
	pNew := mustOpenFilePart(dstPartPath)
	pwNew := &partWrapper{
		p: pNew,
	}
	pwNew.incRef()
	return pwNew
}

func areAllInmemoryParts(pws []*partWrapper) bool {
	for _, pw := range pws {
		if pw.mp == nil {
			return false
		}
	}
	return true
}

func (tb *Table) swapSrcWithDstParts(pws []*partWrapper, pwNew *partWrapper, dstPartType partType) {
	// Atomically unregister old parts and add new part to tb.
	m := makeMapFromPartWrappers(pws)

	removedInmemoryParts := 0
	removedFileParts := 0

	tb.partsLock.Lock()

	tb.inmemoryParts, removedInmemoryParts = removeParts(tb.inmemoryParts, m)
	tb.fileParts, removedFileParts = removeParts(tb.fileParts, m)
	switch dstPartType {
	case partInmemory:
		tb.inmemoryParts = append(tb.inmemoryParts, pwNew)
		tb.startInmemoryPartsMergerLocked()
	case partFile:
		tb.fileParts = append(tb.fileParts, pwNew)
		tb.startFilePartsMergerLocked()
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}

	// Atomically store the updated list of file-based parts on disk.
	// This must be performed under partsLock in order to prevent from races
	// when multiple concurrently running goroutines update the list.
	if removedFileParts > 0 || dstPartType == partFile {
		mustWritePartNames(tb.fileParts, tb.path)
	}

	tb.partsLock.Unlock()

	// Update inmemoryPartsLimitCh accordingly to the number of the remaining in-memory parts.
	for i := 0; i < removedInmemoryParts; i++ {
		select {
		case <-tb.inmemoryPartsLimitCh:
		case <-tb.stopCh:
		}
	}
	if dstPartType == partInmemory {
		select {
		case tb.inmemoryPartsLimitCh <- struct{}{}:
		case <-tb.stopCh:
		}
	}

	removedParts := removedInmemoryParts + removedFileParts
	if removedParts != len(m) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(m))
	}

	// Mark old parts as must be deleted and decrement reference count,
	// so they are eventually closed and deleted.
	for _, pw := range pws {
		pw.mustDrop.Store(true)
		pw.decRef()
	}
}

func makeMapFromPartWrappers(pws []*partWrapper) map[*partWrapper]struct{} {
	m := make(map[*partWrapper]struct{}, len(pws))
	for _, pw := range pws {
		m[pw] = struct{}{}
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found in %d source parts", len(pws)-len(m), len(pws))
	}
	return m
}

func getPartsSize(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.size
	}
	return n
}

func getCompressLevel(itemsCount uint64) int {
	if itemsCount <= 1<<16 {
		// -5 is the minimum supported compression for zstd.
		// See https://github.com/facebook/zstd/releases/tag/v1.3.4
		return -5
	}
	if itemsCount <= 1<<17 {
		return -4
	}
	if itemsCount <= 1<<18 {
		return -3
	}
	if itemsCount <= 1<<19 {
		return -2
	}
	if itemsCount <= 1<<20 {
		return -1
	}
	if itemsCount <= 1<<22 {
		return 1
	}
	if itemsCount <= 1<<25 {
		return 2
	}
	return 3
}

func (tb *Table) nextMergeIdx() uint64 {
	return tb.mergeIdx.Add(1)
}

func mustOpenParts(path string) []*partWrapper {
	// The path can be missing after restoring from backup, so create it if needed.
	fs.MustMkdirIfNotExist(path)

	// Remove txn and tmp directories, which may be left after the upgrade
	// to v1.90.0 and newer versions.
	fs.MustRemoveDir(filepath.Join(path, "txn"))
	fs.MustRemoveDir(filepath.Join(path, "tmp"))

	partsFile := filepath.Join(path, partsFilename)
	partNames := mustReadPartNames(partsFile, path)

	// Remove dirs missing in partNames. These dirs may be left after unclean shutdown
	// or after the update from versions prior to v1.90.0.
	des := fs.MustReadDir(path)
	m := make(map[string]struct{}, len(partNames))
	for _, partName := range partNames {
		// Make sure the partName exists on disk.
		// If it is missing, then manual action from the user is needed,
		// since this is unexpected state, which cannot occur under normal operation,
		// including unclean shutdown.
		partPath := filepath.Join(path, partName)
		if !fs.IsPathExist(partPath) {
			logger.Panicf("FATAL: part %q is listed in %q, but is missing on disk; "+
				"ensure %q contents is not corrupted; remove %q from %q in order to restore access to the remaining data",
				partPath, partsFile, partsFile, partPath, partsFile)
		}

		m[partName] = struct{}{}
	}
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories.
			continue
		}
		fn := de.Name()
		if _, ok := m[fn]; !ok {
			deletePath := filepath.Join(path, fn)
			logger.Infof("deleting %q because it isn't listed in %q; this is the expected case after unclean shutdown", deletePath, partsFile)
			fs.MustRemoveDir(deletePath)
		}
	}
	fs.MustSyncPath(path)

	// Open parts
	var pws []*partWrapper
	for _, partName := range partNames {
		partPath := filepath.Join(path, partName)
		p := mustOpenFilePart(partPath)
		pw := &partWrapper{
			p: p,
		}
		pw.incRef()
		pws = append(pws, pw)
	}
	if !fs.IsPathExist(partsFile) {
		// Create parts.json file if it doesn't exist yet.
		// This should protect from possible carshloops just after the migration from versions below v1.90.0
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4336
		mustWritePartNames(pws, path)
	}

	return pws
}

// MustCreateSnapshotAt creates tb snapshot in the given dstDir.
//
// Snapshot is created using linux hard links, so it is usually created very quickly.
//
// The method panics in case of any error. The input it accepts is provided by
// the storage (not by a user) and the error indicates either a bug in storage
// or a problem with the underlying file system (such as insufficient
// permissions).
func (tb *Table) MustCreateSnapshotAt(dstDir string) {
	logger.Infof("creating Table snapshot of %q...", tb.path)
	startTime := time.Now()

	var err error
	srcDir := tb.path
	srcDir, err = filepath.Abs(srcDir)
	if err != nil {
		logger.Panicf("FATAL: cannot obtain absolute dir for %q: %w", srcDir, err)
	}
	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		logger.Panicf("FATAL: cannot obtain absolute dir for %q: %w", dstDir, err)
	}
	prefix := srcDir + string(filepath.Separator)
	if strings.HasPrefix(dstDir, prefix) {
		logger.Panicf("BUG: cannot create snapshot %q inside the data dir %q", dstDir, srcDir)
	}

	// Flush inmemory items to disk.
	tb.flushInmemoryItemsToFiles()

	fs.MustMkdirFailIfExist(dstDir)

	pws := tb.getParts(nil)
	defer tb.putParts(pws)

	// Create a file with part names at dstDir
	mustWritePartNames(pws, dstDir)

	// Make hardlinks for pws at dstDir
	for _, pw := range pws {
		if pw.mp != nil {
			// Skip in-memory parts
			continue
		}
		srcPartPath := pw.p.path
		dstPartPath := filepath.Join(dstDir, filepath.Base(srcPartPath))
		fs.MustHardLinkFiles(srcPartPath, dstPartPath)
	}

	fs.MustSyncPath(dstDir)
	parentDir := filepath.Dir(dstDir)
	fs.MustSyncPath(parentDir)

	logger.Infof("created Table snapshot of %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
}

func mustWritePartNames(pws []*partWrapper, dstDir string) {
	partNames := make([]string, 0, len(pws))
	for _, pw := range pws {
		if pw.mp != nil {
			// Skip in-memory parts
			continue
		}
		partName := filepath.Base(pw.p.path)
		partNames = append(partNames, partName)
	}
	sort.Strings(partNames)
	data, err := json.Marshal(partNames)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partNames to JSON: %s", err)
	}
	partsFile := filepath.Join(dstDir, partsFilename)
	fs.MustWriteAtomic(partsFile, data, true)
}

func mustReadPartNames(partsFile, srcDir string) []string {
	if fs.IsPathExist(partsFile) {
		data, err := os.ReadFile(partsFile)
		if err != nil {
			logger.Panicf("FATAL: cannot read %q: %s", partsFile, err)
		}
		var partNames []string
		if err := json.Unmarshal(data, &partNames); err != nil {
			logger.Panicf("FATAL: cannot parse %q: %s", partsFile, err)
		}
		return partNames
	}
	// The partsFilename is missing. This is the upgrade from versions previous to v1.90.0.
	// Read part names from directories under srcDir
	des := fs.MustReadDir(srcDir)
	var partNames []string
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories.
			continue
		}
		partName := de.Name()
		if isSpecialDir(partName) {
			// Skip special dirs.
			continue
		}
		partNames = append(partNames, partName)
	}
	return partNames
}

// getPartsToMerge returns optimal parts to merge from pws.
//
// The summary size of the returned parts must be smaller than the maxOutBytes.
func getPartsToMerge(pws []*partWrapper, maxOutBytes uint64) []*partWrapper {
	pwsRemaining := make([]*partWrapper, 0, len(pws))
	for _, pw := range pws {
		if !pw.isInMerge {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}

	pwsToMerge := appendPartsToMerge(nil, pwsRemaining, defaultPartsToMerge, maxOutBytes)

	for _, pw := range pwsToMerge {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge unexpectedly set to true")
		}
		pw.isInMerge = true
	}

	return pwsToMerge
}

// getPartsForOptimalMerge returns parts from pws for optimal merge, plus the remaining parts.
//
// the pws items are replaced by nil after the call. This is needed for helping Go GC to reclaim the referenced items.
func getPartsForOptimalMerge(pws []*partWrapper) ([]*partWrapper, []*partWrapper) {
	pwsToMerge := appendPartsToMerge(nil, pws, defaultPartsToMerge, math.MaxUint64)
	if len(pwsToMerge) == 0 {
		return pws, nil
	}

	m := makeMapFromPartWrappers(pwsToMerge)
	pwsRemaining := make([]*partWrapper, 0, len(pws)-len(pwsToMerge))
	for _, pw := range pws {
		if _, ok := m[pw]; !ok {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}

	// Clear references to pws items, so they could be reclaimed faster by Go GC.
	for i := range pws {
		pws[i] = nil
	}

	return pwsToMerge, pwsRemaining
}

// minMergeMultiplier is the minimum multiplier for the size of the output part
// compared to the size of the maximum input part for the merge.
//
// Higher value reduces write amplification (disk write IO induced by the merge),
// while increases the number of unmerged parts.
// The 1.7 is good enough for production workloads.
const minMergeMultiplier = 1.7

// appendPartsToMerge finds optimal parts to merge from src, appends them to dst and returns the result.
func appendPartsToMerge(dst, src []*partWrapper, maxPartsToMerge int, maxOutBytes uint64) []*partWrapper {
	if len(src) < 2 {
		// There is no need in merging zero or one part :)
		return dst
	}
	if maxPartsToMerge < 2 {
		logger.Panicf("BUG: maxPartsToMerge cannot be smaller than 2; got %d", maxPartsToMerge)
	}

	// Filter out too big parts.
	// This should reduce N for O(n^2) algorithm below.
	maxInPartBytes := uint64(float64(maxOutBytes) / minMergeMultiplier)
	tmp := make([]*partWrapper, 0, len(src))
	for _, pw := range src {
		if pw.p.size > maxInPartBytes {
			continue
		}
		tmp = append(tmp, pw)
	}
	src = tmp

	sortPartsForOptimalMerge(src)

	maxSrcParts := maxPartsToMerge
	if maxSrcParts > len(src) {
		maxSrcParts = len(src)
	}
	minSrcParts := (maxSrcParts + 1) / 2
	if minSrcParts < 2 {
		minSrcParts = 2
	}

	// Exhaustive search for parts giving the lowest write amplification when merged.
	var pws []*partWrapper
	maxM := float64(0)
	for i := minSrcParts; i <= maxSrcParts; i++ {
		for j := 0; j <= len(src)-i; j++ {
			a := src[j : j+i]
			if a[0].p.size*uint64(len(a)) < a[len(a)-1].p.size {
				// Do not merge parts with too big difference in size,
				// since this results in unbalanced merges.
				continue
			}
			outBytes := uint64(0)
			for _, pw := range a {
				outBytes += pw.p.size
			}
			if outBytes > maxOutBytes {
				// There is no sense in checking the remaining bigger parts.
				break
			}
			m := float64(outBytes) / float64(a[len(a)-1].p.size)
			if m < maxM {
				continue
			}
			maxM = m
			pws = a
		}
	}

	minM := float64(maxPartsToMerge) / 2
	if minM < minMergeMultiplier {
		minM = minMergeMultiplier
	}
	if maxM < minM {
		// There is no sense in merging parts with too small m,
		// since this leads to high disk write IO.
		return dst
	}
	return append(dst, pws...)
}

func sortPartsForOptimalMerge(pws []*partWrapper) {
	// Sort src parts by size.
	sort.Slice(pws, func(i, j int) bool {
		return pws[i].p.size < pws[j].p.size
	})
}

func removeParts(pws []*partWrapper, partsToRemove map[*partWrapper]struct{}) ([]*partWrapper, int) {
	dst := pws[:0]
	for _, pw := range pws {
		if _, ok := partsToRemove[pw]; !ok {
			dst = append(dst, pw)
		}
	}
	for i := len(dst); i < len(pws); i++ {
		pws[i] = nil
	}
	return dst, len(pws) - len(dst)
}

func isSpecialDir(name string) bool {
	// Snapshots and cache dirs aren't used anymore.
	// Keep them here for backwards compatibility.
	return name == "tmp" || name == "txn" || name == "snapshots" || name == "cache"
}
