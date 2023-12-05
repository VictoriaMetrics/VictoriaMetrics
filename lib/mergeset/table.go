package mergeset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
)

// maxInmemoryParts is the maximum number of inmemory parts in the table.
//
// This number may be reached when the insertion pace outreaches merger pace.
// If this number is reached, then assisted merges are performed
// during data ingestion.
const maxInmemoryParts = 30

// maxFileParts is the maximum number of file parts in the table.
//
// This number may be reached when the insertion pace outreaches merger pace.
// If this number is reached, then assisted merges are performed
// during data ingestion.
const maxFileParts = 64

// Default number of parts to merge at once.
//
// This number has been obtained empirically - it gives the lowest possible overhead.
// See appendPartsToMerge tests for details.
const defaultPartsToMerge = 15

// The final number of parts to merge at once.
//
// It must be smaller than defaultPartsToMerge.
// Lower value improves select performance at the cost of increased
// write amplification.
const finalPartsToMerge = 2

// maxPartSize is the maximum part size in bytes.
//
// This number should be limited by the amount of time required to merge parts of this summary size.
// The required time shouldn't exceed a day.
const maxPartSize = 400e9

// The interval for flushing buffered data to parts, so it becomes visible to search.
const pendingItemsFlushInterval = time.Second

// The interval for guaranteed flush of recently ingested data from memory to on-disk parts,
// so they survive process crash.
var dataFlushInterval = 5 * time.Second

// SetDataFlushInterval sets the interval for guaranteed flush of recently ingested data from memory to disk.
//
// The data can be flushed from memory to disk more frequently if it doesn't fit the memory limit.
//
// This function must be called before initializing the indexdb.
func SetDataFlushInterval(d time.Duration) {
	if d > pendingItemsFlushInterval {
		dataFlushInterval = d
	}
}

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
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	activeInmemoryMerges uint64
	activeFileMerges     uint64

	inmemoryMergesCount uint64
	fileMergesCount     uint64

	inmemoryItemsMerged uint64
	fileItemsMerged     uint64

	inmemoryAssistedMerges uint64
	fileAssistedMerges     uint64

	itemsAdded          uint64
	itemsAddedSizeBytes uint64

	mergeIdx uint64

	path string

	flushCallback         func()
	flushCallbackWorkerWG sync.WaitGroup
	needFlushCallbackCall uint32

	prepareBlock PrepareBlockCallback
	isReadOnly   *uint32

	// rawItems contains recently added items that haven't been converted to parts yet.
	//
	// rawItems aren't used in search for performance reasons
	rawItems rawItemsShards

	// partsLock protects inmemoryParts and fileParts.
	partsLock sync.Mutex

	// inmemoryParts contains inmemory parts.
	inmemoryParts []*partWrapper

	// fileParts contains file-backed parts.
	fileParts []*partWrapper

	// This channel is used for signaling the background mergers that there are parts,
	// which may need to be merged.
	needMergeCh chan struct{}

	stopCh chan struct{}

	wg sync.WaitGroup

	// Use syncwg instead of sync, since Add/Wait may be called from concurrent goroutines.
	rawItemsPendingFlushesWG syncwg.WaitGroup
}

type rawItemsShards struct {
	shardIdx uint32

	// shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawItemsShard
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
	return (cpus*multiplier + 1) / 2
}()

const maxBlocksPerShard = 256

func (riss *rawItemsShards) init() {
	riss.shards = make([]rawItemsShard, rawItemsShardsPerTable)
}

func (riss *rawItemsShards) addItems(tb *Table, items [][]byte) {
	shards := riss.shards
	shardsLen := uint32(len(shards))
	for len(items) > 0 {
		n := atomic.AddUint32(&riss.shardIdx, 1)
		idx := n % shardsLen
		items = shards[idx].addItems(tb, items)
	}
}

func (riss *rawItemsShards) Len() int {
	n := 0
	for i := range riss.shards {
		n += riss.shards[i].Len()
	}
	return n
}

type rawItemsShardNopad struct {
	// Put lastFlushTime to the top in order to avoid unaligned memory access on 32-bit architectures
	lastFlushTime uint64

	mu  sync.Mutex
	ibs []*inmemoryBlock
}

type rawItemsShard struct {
	rawItemsShardNopad

	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(rawItemsShardNopad{})%128]byte
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

func (ris *rawItemsShard) addItems(tb *Table, items [][]byte) [][]byte {
	var ibsToFlush []*inmemoryBlock
	var tailItems [][]byte

	ris.mu.Lock()
	ibs := ris.ibs
	if len(ibs) == 0 {
		ib := getInmemoryBlock()
		ibs = append(ibs, ib)
		ris.ibs = ibs
	}
	ib := ibs[len(ibs)-1]
	for i, item := range items {
		if ib.Add(item) {
			continue
		}
		if len(ibs) >= maxBlocksPerShard {
			ibsToFlush = ibs
			ibs = make([]*inmemoryBlock, 0, maxBlocksPerShard)
			tailItems = items[i:]
			atomic.StoreUint64(&ris.lastFlushTime, fasttime.UnixTimestamp())
			break
		}
		ib = getInmemoryBlock()
		if ib.Add(item) {
			ibs = append(ibs, ib)
			continue
		}
		putInmemoryBlock(ib)
		logger.Panicf("BUG: cannot insert too big item into an empty inmemoryBlock len(item)=%d; the caller should be responsible for avoiding too big items", len(item))
	}
	ris.ibs = ibs
	ris.mu.Unlock()

	tb.flushBlocksToParts(ibsToFlush, false)

	if len(ibsToFlush) > 0 {
		// Run assisted merges if needed.
		flushConcurrencyCh <- struct{}{}
		tb.assistedMergeForInmemoryParts()
		tb.assistedMergeForFileParts()
		<-flushConcurrencyCh
	}

	return tailItems
}

type partWrapper struct {
	p *part

	mp *inmemoryPart

	refCount uint32

	// mustBeDeleted marks partWrapper for deletion.
	// This field should be updated only after partWrapper
	// was removed from the list of active parts.
	mustBeDeleted uint32

	isInMerge bool

	// The deadline when the in-memory part must be flushed to disk.
	flushToDiskDeadline time.Time
}

func (pw *partWrapper) incRef() {
	atomic.AddUint32(&pw.refCount, 1)
}

func (pw *partWrapper) decRef() {
	n := atomic.AddUint32(&pw.refCount, ^uint32(0))
	if int32(n) < 0 {
		logger.Panicf("BUG: pw.refCount must be bigger than 0; got %d", int32(n))
	}
	if n > 0 {
		return
	}

	deletePath := ""
	if pw.mp == nil && atomic.LoadUint32(&pw.mustBeDeleted) != 0 {
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
		fs.MustRemoveAll(deletePath)
	}
}

// MustOpenTable opens a table on the given path.
//
// Optional flushCallback is called every time new data batch is flushed
// to the underlying storage and becomes visible to search.
//
// Optional prepareBlock is called during merge before flushing the prepared block
// to persistent storage.
//
// The table is created if it doesn't exist yet.
func MustOpenTable(path string, flushCallback func(), prepareBlock PrepareBlockCallback, isReadOnly *uint32) *Table {
	path = filepath.Clean(path)

	// Create a directory for the table if it doesn't exist yet.
	fs.MustMkdirIfNotExist(path)

	// Open table parts.
	pws := mustOpenParts(path)

	tb := &Table{
		path:          path,
		flushCallback: flushCallback,
		prepareBlock:  prepareBlock,
		isReadOnly:    isReadOnly,
		fileParts:     pws,
		mergeIdx:      uint64(time.Now().UnixNano()),
		needMergeCh:   make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}
	tb.rawItems.init()
	tb.startBackgroundWorkers()

	// Wake up a single background merger, so it could start merging parts if needed.
	tb.notifyBackgroundMergers()

	if flushCallback != nil {
		tb.flushCallbackWorkerWG.Add(1)
		go func() {
			// call flushCallback once per 10 seconds in order to improve the effectiveness of caches,
			// which are reset by the flushCallback.
			tc := time.NewTicker(10 * time.Second)
			for {
				select {
				case <-tb.stopCh:
					tb.flushCallback()
					tb.flushCallbackWorkerWG.Done()
					return
				case <-tc.C:
					if atomic.CompareAndSwapUint32(&tb.needFlushCallbackCall, 1, 0) {
						tb.flushCallback()
					}
				}
			}
		}()
	}

	return tb
}

func (tb *Table) startBackgroundWorkers() {
	tb.startMergeWorkers()
	tb.startInmemoryPartsFlusher()
	tb.startPendingItemsFlusher()
}

// MustClose closes the table.
func (tb *Table) MustClose() {
	close(tb.stopCh)

	// Waiting for background workers to stop
	tb.wg.Wait()

	tb.flushInmemoryItems()
	tb.flushCallbackWorkerWG.Wait()

	// Remove references to parts from the tb, so they may be eventually closed after all the searches are done.
	tb.partsLock.Lock()
	inmemoryParts := tb.inmemoryParts
	fileParts := tb.fileParts
	tb.inmemoryParts = nil
	tb.fileParts = nil
	tb.partsLock.Unlock()

	for _, pw := range inmemoryParts {
		pw.decRef()
	}
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

	InmemoryAssistedMerges uint64
	FileAssistedMerges     uint64

	ItemsAdded          uint64
	ItemsAddedSizeBytes uint64

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

	IndexBlocksCacheSize         uint64
	IndexBlocksCacheSizeBytes    uint64
	IndexBlocksCacheSizeMaxBytes uint64
	IndexBlocksCacheRequests     uint64
	IndexBlocksCacheMisses       uint64

	PartsRefCount uint64
}

// TotalItemsCount returns the total number of items in the table.
func (tm *TableMetrics) TotalItemsCount() uint64 {
	return tm.InmemoryItemsCount + tm.FileItemsCount
}

// UpdateMetrics updates m with metrics from tb.
func (tb *Table) UpdateMetrics(m *TableMetrics) {
	m.ActiveInmemoryMerges += atomic.LoadUint64(&tb.activeInmemoryMerges)
	m.ActiveFileMerges += atomic.LoadUint64(&tb.activeFileMerges)

	m.InmemoryMergesCount += atomic.LoadUint64(&tb.inmemoryMergesCount)
	m.FileMergesCount += atomic.LoadUint64(&tb.fileMergesCount)

	m.InmemoryItemsMerged += atomic.LoadUint64(&tb.inmemoryItemsMerged)
	m.FileItemsMerged += atomic.LoadUint64(&tb.fileItemsMerged)

	m.InmemoryAssistedMerges += atomic.LoadUint64(&tb.inmemoryAssistedMerges)
	m.FileAssistedMerges += atomic.LoadUint64(&tb.fileAssistedMerges)

	m.ItemsAdded += atomic.LoadUint64(&tb.itemsAdded)
	m.ItemsAddedSizeBytes += atomic.LoadUint64(&tb.itemsAddedSizeBytes)

	m.PendingItems += uint64(tb.rawItems.Len())

	tb.partsLock.Lock()

	m.InmemoryPartsCount += uint64(len(tb.inmemoryParts))
	for _, pw := range tb.inmemoryParts {
		p := pw.p
		m.InmemoryBlocksCount += p.ph.blocksCount
		m.InmemoryItemsCount += p.ph.itemsCount
		m.InmemorySizeBytes += p.size
		m.PartsRefCount += uint64(atomic.LoadUint32(&pw.refCount))
	}

	m.FilePartsCount += uint64(len(tb.fileParts))
	for _, pw := range tb.fileParts {
		p := pw.p
		m.FileBlocksCount += p.ph.blocksCount
		m.FileItemsCount += p.ph.itemsCount
		m.FileSizeBytes += p.size
		m.PartsRefCount += uint64(atomic.LoadUint32(&pw.refCount))
	}
	tb.partsLock.Unlock()

	m.DataBlocksCacheSize = uint64(ibCache.Len())
	m.DataBlocksCacheSizeBytes = uint64(ibCache.SizeBytes())
	m.DataBlocksCacheSizeMaxBytes = uint64(ibCache.SizeMaxBytes())
	m.DataBlocksCacheRequests = ibCache.Requests()
	m.DataBlocksCacheMisses = ibCache.Misses()

	m.IndexBlocksCacheSize = uint64(idxbCache.Len())
	m.IndexBlocksCacheSizeBytes = uint64(idxbCache.SizeBytes())
	m.IndexBlocksCacheSizeMaxBytes = uint64(idxbCache.SizeMaxBytes())
	m.IndexBlocksCacheRequests = idxbCache.Requests()
	m.IndexBlocksCacheMisses = idxbCache.Misses()
}

// AddItems adds the given items to the tb.
//
// The function panics when items contains an item with length exceeding maxInmemoryBlockSize.
// It is caller's responsibility to make sure there are no too long items.
func (tb *Table) AddItems(items [][]byte) {
	tb.rawItems.addItems(tb, items)
	atomic.AddUint64(&tb.itemsAdded, uint64(len(items)))
	n := 0
	for _, item := range items {
		n += len(item)
	}
	atomic.AddUint64(&tb.itemsAddedSizeBytes, uint64(n))
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

func (tb *Table) mergePartsOptimal(pws []*partWrapper) error {
	sortPartsForOptimalMerge(pws)
	for len(pws) > 0 {
		n := defaultPartsToMerge
		if n > len(pws) {
			n = len(pws)
		}
		pwsChunk := pws[:n]
		pws = pws[n:]
		err := tb.mergeParts(pwsChunk, nil, true)
		if err == nil {
			continue
		}
		tb.releasePartsToMerge(pws)
		return fmt.Errorf("cannot optimally merge %d parts: %w", n, err)
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
	tb.flushPendingItems(nil, true)

	// Wait for background flushers to finish.
	tb.rawItemsPendingFlushesWG.Wait()
}

func (tb *Table) startInmemoryPartsFlusher() {
	tb.wg.Add(1)
	go func() {
		tb.inmemoryPartsFlusher()
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

func (tb *Table) inmemoryPartsFlusher() {
	ticker := time.NewTicker(dataFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			tb.flushInmemoryParts(false)
		}
	}
}

func (tb *Table) pendingItemsFlusher() {
	ticker := time.NewTicker(pendingItemsFlushInterval)
	defer ticker.Stop()
	var ibs []*inmemoryBlock
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			ibs = tb.flushPendingItems(ibs[:0], false)
			for i := range ibs {
				ibs[i] = nil
			}
		}
	}
}

func (tb *Table) flushPendingItems(dst []*inmemoryBlock, isFinal bool) []*inmemoryBlock {
	return tb.rawItems.flush(tb, dst, isFinal)
}

func (tb *Table) flushInmemoryItems() {
	tb.rawItems.flush(tb, nil, true)
	tb.flushInmemoryParts(true)
}

func (tb *Table) flushInmemoryParts(isFinal bool) {
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

	if err := tb.mergePartsOptimal(pws); err != nil {
		logger.Panicf("FATAL: cannot merge in-memory parts: %s", err)
	}
}

func (riss *rawItemsShards) flush(tb *Table, dst []*inmemoryBlock, isFinal bool) []*inmemoryBlock {
	tb.rawItemsPendingFlushesWG.Add(1)
	for i := range riss.shards {
		dst = riss.shards[i].appendBlocksToFlush(dst, isFinal)
	}
	tb.flushBlocksToParts(dst, isFinal)
	tb.rawItemsPendingFlushesWG.Done()
	return dst
}

func (ris *rawItemsShard) appendBlocksToFlush(dst []*inmemoryBlock, isFinal bool) []*inmemoryBlock {
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(pendingItemsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	lastFlushTime := atomic.LoadUint64(&ris.lastFlushTime)
	if !isFinal && currentTime < lastFlushTime+uint64(flushSeconds) {
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
	atomic.StoreUint64(&ris.lastFlushTime, currentTime)
	ris.mu.Unlock()
	return dst
}

func (tb *Table) flushBlocksToParts(ibs []*inmemoryBlock, isFinal bool) {
	if len(ibs) == 0 {
		return
	}
	var pwsLock sync.Mutex
	pws := make([]*partWrapper, 0, (len(ibs)+defaultPartsToMerge-1)/defaultPartsToMerge)
	wg := getWaitGroup()
	for len(ibs) > 0 {
		n := defaultPartsToMerge
		if n > len(ibs) {
			n = len(ibs)
		}
		wg.Add(1)
		flushConcurrencyCh <- struct{}{}
		go func(ibsChunk []*inmemoryBlock) {
			defer func() {
				<-flushConcurrencyCh
				wg.Done()
			}()
			pw := tb.createInmemoryPart(ibsChunk)
			if pw == nil {
				return
			}
			pwsLock.Lock()
			pws = append(pws, pw)
			pwsLock.Unlock()
		}(ibs[:n])
		ibs = ibs[n:]
	}
	wg.Wait()
	putWaitGroup(wg)

	tb.partsLock.Lock()
	tb.inmemoryParts = append(tb.inmemoryParts, pws...)
	for range pws {
		if !tb.notifyBackgroundMergers() {
			break
		}
	}
	tb.partsLock.Unlock()

	if tb.flushCallback != nil {
		if isFinal {
			tb.flushCallback()
		} else {
			atomic.CompareAndSwapUint32(&tb.needFlushCallbackCall, 0, 1)
		}
	}
}

func (tb *Table) notifyBackgroundMergers() bool {
	select {
	case tb.needMergeCh <- struct{}{}:
		return true
	default:
		return false
	}
}

var flushConcurrencyLimit = func() int {
	n := cgroup.AvailableCPUs()
	if n < 2 {
		// Allow at least 2 concurrent flushers on systems with a single CPU core
		// in order to guarantee that in-memory data flushes and background merges can be continued
		// when a single flusher is busy with the long merge.
		n = 2
	}
	return n
}()

var flushConcurrencyCh = make(chan struct{}, flushConcurrencyLimit)

func needAssistedMerge(pws []*partWrapper, maxParts int) bool {
	if len(pws) < maxParts {
		return false
	}
	return getNotInMergePartsCount(pws) >= defaultPartsToMerge
}

func (tb *Table) assistedMergeForInmemoryParts() {
	tb.partsLock.Lock()
	needMerge := needAssistedMerge(tb.inmemoryParts, maxInmemoryParts)
	tb.partsLock.Unlock()
	if !needMerge {
		return
	}

	atomic.AddUint64(&tb.inmemoryAssistedMerges, 1)
	err := tb.mergeInmemoryParts()
	if err == nil {
		return
	}
	if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) {
		return
	}
	logger.Panicf("FATAL: cannot assist with merging inmemory parts: %s", err)
}

func (tb *Table) assistedMergeForFileParts() {
	tb.partsLock.Lock()
	needMerge := needAssistedMerge(tb.fileParts, maxFileParts)
	tb.partsLock.Unlock()
	if !needMerge {
		return
	}

	atomic.AddUint64(&tb.fileAssistedMerges, 1)
	err := tb.mergeExistingParts(false)
	if err == nil {
		return
	}
	if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) || errors.Is(err, errReadOnlyMode) {
		return
	}
	logger.Panicf("FATAL: cannot assist with merging file parts: %s", err)
}

func getNotInMergePartsCount(pws []*partWrapper) int {
	n := 0
	for _, pw := range pws {
		if !pw.isInMerge {
			n++
		}
	}
	return n
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

func (tb *Table) createInmemoryPart(ibs []*inmemoryBlock) *partWrapper {
	outItemsCount := uint64(0)
	for _, ib := range ibs {
		outItemsCount += uint64(ib.Len())
	}

	// Prepare blockStreamReaders for source blocks.
	bsrs := make([]*blockStreamReader, 0, len(ibs))
	for _, ib := range ibs {
		if len(ib.items) == 0 {
			continue
		}
		bsr := getBlockStreamReader()
		bsr.MustInitFromInmemoryBlock(ib)
		putInmemoryBlock(ib)
		bsrs = append(bsrs, bsr)
	}
	if len(bsrs) == 0 {
		return nil
	}
	flushToDiskDeadline := time.Now().Add(dataFlushInterval)
	if len(bsrs) == 1 {
		// Nothing to merge. Just return a single inmemory part.
		bsr := bsrs[0]
		mp := &inmemoryPart{}
		mp.Init(&bsr.Block)
		putBlockStreamReader(bsr)
		return newPartWrapperFromInmemoryPart(mp, flushToDiskDeadline)
	}

	// Prepare blockStreamWriter for destination part.
	compressLevel := getCompressLevel(outItemsCount)
	bsw := getBlockStreamWriter()
	mpDst := &inmemoryPart{}
	bsw.MustInitFromInmemoryPart(mpDst, compressLevel)

	// Merge parts.
	// The merge shouldn't be interrupted by stopCh,
	// since it may be final after stopCh is closed.
	atomic.AddUint64(&tb.activeInmemoryMerges, 1)
	err := mergeBlockStreams(&mpDst.ph, bsw, bsrs, tb.prepareBlock, nil, &tb.inmemoryItemsMerged)
	atomic.AddUint64(&tb.activeInmemoryMerges, ^uint64(0))
	atomic.AddUint64(&tb.inmemoryMergesCount, 1)
	if err != nil {
		logger.Panicf("FATAL: cannot merge inmemoryBlocks: %s", err)
	}
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	return newPartWrapperFromInmemoryPart(mpDst, flushToDiskDeadline)
}

func newPartWrapperFromInmemoryPart(mp *inmemoryPart, flushToDiskDeadline time.Time) *partWrapper {
	p := mp.NewPart()
	return &partWrapper{
		p:                   p,
		mp:                  mp,
		refCount:            1,
		flushToDiskDeadline: flushToDiskDeadline,
	}
}

func (tb *Table) startMergeWorkers() {
	// The actual number of concurrent merges is limited inside mergeWorker() below.
	for i := 0; i < cap(mergeWorkersLimitCh); i++ {
		tb.wg.Add(1)
		go func() {
			tb.mergeWorker()
			tb.wg.Done()
		}()
	}
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
	// Divide free space by the max number of concurrent merges.
	maxOutBytes := n / uint64(cap(mergeWorkersLimitCh))
	if maxOutBytes > maxPartSize {
		maxOutBytes = maxPartSize
	}
	return maxOutBytes
}

func (tb *Table) canBackgroundMerge() bool {
	return atomic.LoadUint32(tb.isReadOnly) == 0
}

var errReadOnlyMode = fmt.Errorf("storage is in readonly mode")

func (tb *Table) mergeInmemoryParts() error {
	maxOutBytes := tb.getMaxFilePartSize()

	tb.partsLock.Lock()
	pws := getPartsToMerge(tb.inmemoryParts, maxOutBytes, false)
	tb.partsLock.Unlock()

	return tb.mergeParts(pws, tb.stopCh, false)
}

func (tb *Table) mergeExistingParts(isFinal bool) error {
	if !tb.canBackgroundMerge() {
		// Do not perform background merge in read-only mode
		// in order to prevent from disk space shortage.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2603
		return errReadOnlyMode
	}
	maxOutBytes := tb.getMaxFilePartSize()

	tb.partsLock.Lock()
	dst := make([]*partWrapper, 0, len(tb.inmemoryParts)+len(tb.fileParts))
	dst = append(dst, tb.inmemoryParts...)
	dst = append(dst, tb.fileParts...)
	pws := getPartsToMerge(dst, maxOutBytes, isFinal)
	tb.partsLock.Unlock()

	return tb.mergeParts(pws, tb.stopCh, isFinal)
}

func (tb *Table) mergeWorker() {
	var lastMergeTime uint64
	isFinal := false
	for {
		// Limit the number of concurrent calls to mergeExistingParts, since the total number of merge workers
		// across tables may exceed the the cap(mergeWorkersLimitCh).
		mergeWorkersLimitCh <- struct{}{}
		err := tb.mergeExistingParts(isFinal)
		<-mergeWorkersLimitCh
		if err == nil {
			// Try merging additional parts.
			lastMergeTime = fasttime.UnixTimestamp()
			isFinal = false
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// The merger has been stopped.
			return
		}
		if !errors.Is(err, errNothingToMerge) && !errors.Is(err, errReadOnlyMode) {
			// Unexpected error.
			logger.Panicf("FATAL: unrecoverable error when merging inmemory parts in %q: %s", tb.path, err)
		}
		if finalMergeDelaySeconds > 0 && fasttime.UnixTimestamp()-lastMergeTime > finalMergeDelaySeconds {
			// We have free time for merging into bigger parts.
			// This should improve select performance.
			lastMergeTime = fasttime.UnixTimestamp()
			isFinal = true
			continue
		}

		// Nothing to merge. Wait for the notification of new merge.
		select {
		case <-tb.stopCh:
			return
		case <-tb.needMergeCh:
		}
	}
}

// Disable final merge by default, since it may lead to high disk IO and CPU usage
// after some inactivity time.
var finalMergeDelaySeconds = uint64(0)

// SetFinalMergeDelay sets the delay before doing final merge for Table without newly ingested data.
//
// This function may be called only before Table initialization.
func SetFinalMergeDelay(delay time.Duration) {
	if delay <= 0 {
		return
	}
	finalMergeDelaySeconds = uint64(delay.Seconds() + 1)
}

var errNothingToMerge = fmt.Errorf("nothing to merge")

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
// Merging is immediately stopped if stopCh is closed.
//
// If isFinal is set, then the resulting part will be stored to disk.
//
// All the parts inside pws must have isInMerge field set to true.
// The isInMerge field inside pws parts is set to false before returning from the function.
func (tb *Table) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal bool) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
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
		fs.MustSyncPath(dstPartPath)
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

func getFlushToDiskDeadline(pws []*partWrapper) time.Time {
	d := time.Now().Add(dataFlushInterval)
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
	var itemsMerged *uint64
	var mergesCount *uint64
	var activeMerges *uint64
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
	atomic.AddUint64(activeMerges, 1)
	err := mergeBlockStreams(&ph, bsw, bsrs, tb.prepareBlock, stopCh, itemsMerged)
	atomic.AddUint64(activeMerges, ^uint64(0))
	atomic.AddUint64(mergesCount, 1)
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
		flushToDiskDeadline := getFlushToDiskDeadline(pws)
		pwNew := newPartWrapperFromInmemoryPart(mpNew, flushToDiskDeadline)
		return pwNew
	}
	// Open the created part from disk.
	pNew := mustOpenFilePart(dstPartPath)
	pwNew := &partWrapper{
		p:        pNew,
		refCount: 1,
	}
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
	m := make(map[*partWrapper]bool, len(pws))
	for _, pw := range pws {
		m[pw] = true
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found when merging %d parts", len(pws)-len(m), len(pws))
	}
	removedInmemoryParts := 0
	removedFileParts := 0

	tb.partsLock.Lock()

	tb.inmemoryParts, removedInmemoryParts = removeParts(tb.inmemoryParts, m)
	tb.fileParts, removedFileParts = removeParts(tb.fileParts, m)
	switch dstPartType {
	case partInmemory:
		tb.inmemoryParts = append(tb.inmemoryParts, pwNew)
	case partFile:
		tb.fileParts = append(tb.fileParts, pwNew)
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}
	tb.notifyBackgroundMergers()

	// Atomically store the updated list of file-based parts on disk.
	// This must be performed under partsLock in order to prevent from races
	// when multiple concurrently running goroutines update the list.
	if removedFileParts > 0 || dstPartType == partFile {
		mustWritePartNames(tb.fileParts, tb.path)
	}

	tb.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedFileParts
	if removedParts != len(m) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(m))
	}

	// Mark old parts as must be deleted and decrement reference count,
	// so they are eventually closed and deleted.
	for _, pw := range pws {
		atomic.StoreUint32(&pw.mustBeDeleted, 1)
		pw.decRef()
	}
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
	if itemsCount <= 1<<28 {
		return 3
	}
	return 4
}

func (tb *Table) nextMergeIdx() uint64 {
	return atomic.AddUint64(&tb.mergeIdx, 1)
}

var mergeWorkersLimitCh = make(chan struct{}, getWorkersCount())

func getWorkersCount() int {
	n := cgroup.AvailableCPUs()
	if n < 4 {
		// Allow at least 4 merge workers on systems with small CPUs count
		// in order to guarantee that background merges can be continued
		// when multiple workers are busy with big merges.
		n = 4
	}
	return n
}

func mustOpenParts(path string) []*partWrapper {
	// The path can be missing after restoring from backup, so create it if needed.
	fs.MustMkdirIfNotExist(path)
	fs.MustRemoveTemporaryDirs(path)

	// Remove txn and tmp directories, which may be left after the upgrade
	// to v1.90.0 and newer versions.
	fs.MustRemoveAll(filepath.Join(path, "txn"))
	fs.MustRemoveAll(filepath.Join(path, "tmp"))

	partNames := mustReadPartNames(path)

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
			partsFile := filepath.Join(path, partsFilename)
			logger.Panicf("FATAL: part %q is listed in %q, but is missing on disk; "+
				"ensure %q contents is not corrupted; remove %q to rebuild its' content from the list of existing parts",
				partPath, partsFile, partsFile, partsFile)
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
			fs.MustRemoveAll(deletePath)
		}
	}
	fs.MustSyncPath(path)

	// Open parts
	var pws []*partWrapper
	for _, partName := range partNames {
		partPath := filepath.Join(path, partName)
		p := mustOpenFilePart(partPath)
		pw := &partWrapper{
			p:        p,
			refCount: 1,
		}
		pws = append(pws, pw)
	}
	partNamesPath := filepath.Join(path, partsFilename)
	if !fs.IsPathExist(partNamesPath) {
		// Create parts.json file if it doesn't exist yet.
		// This should protect from possible carshloops just after the migration from versions below v1.90.0
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4336
		mustWritePartNames(pws, path)
	}

	return pws
}

// CreateSnapshotAt creates tb snapshot in the given dstDir.
//
// Snapshot is created using linux hard links, so it is usually created very quickly.
//
// If deadline is reached before snapshot is created error is returned.
//
// The caller is responsible for data removal at dstDir on unsuccessful snapshot creation.
func (tb *Table) CreateSnapshotAt(dstDir string, deadline uint64) error {
	logger.Infof("creating Table snapshot of %q...", tb.path)
	startTime := time.Now()

	var err error
	srcDir := tb.path
	srcDir, err = filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("cannot obtain absolute dir for %q: %w", srcDir, err)
	}
	dstDir, err = filepath.Abs(dstDir)
	if err != nil {
		return fmt.Errorf("cannot obtain absolute dir for %q: %w", dstDir, err)
	}
	if strings.HasPrefix(dstDir, srcDir+string(filepath.Separator)) {
		return fmt.Errorf("cannot create snapshot %q inside the data dir %q", dstDir, srcDir)
	}

	// Flush inmemory items to disk.
	tb.flushInmemoryItems()

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
		if deadline > 0 && fasttime.UnixTimestamp() > deadline {
			return fmt.Errorf("cannot create snapshot for %q: timeout exceeded", tb.path)
		}
		srcPartPath := pw.p.path
		dstPartPath := filepath.Join(dstDir, filepath.Base(srcPartPath))
		fs.MustHardLinkFiles(srcPartPath, dstPartPath)
	}

	fs.MustSyncPath(dstDir)
	parentDir := filepath.Dir(dstDir)
	fs.MustSyncPath(parentDir)

	logger.Infof("created Table snapshot of %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	return nil
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
	partNamesPath := filepath.Join(dstDir, partsFilename)
	fs.MustWriteAtomic(partNamesPath, data, true)
}

func mustReadPartNames(srcDir string) []string {
	partNamesPath := filepath.Join(srcDir, partsFilename)
	if fs.IsPathExist(partNamesPath) {
		data, err := os.ReadFile(partNamesPath)
		if err != nil {
			logger.Panicf("FATAL: cannot read %s file: %s", partsFilename, err)
		}
		var partNames []string
		if err := json.Unmarshal(data, &partNames); err != nil {
			logger.Panicf("FATAL: cannot parse %s: %s", partNamesPath, err)
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
// if isFinal is set, then merge harder.
//
// The summary size of the returned parts must be smaller than the maxOutBytes.
func getPartsToMerge(pws []*partWrapper, maxOutBytes uint64, isFinal bool) []*partWrapper {
	pwsRemaining := make([]*partWrapper, 0, len(pws))
	for _, pw := range pws {
		if !pw.isInMerge {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}
	maxPartsToMerge := defaultPartsToMerge
	var dst []*partWrapper
	if isFinal {
		for len(dst) == 0 && maxPartsToMerge >= finalPartsToMerge {
			dst = appendPartsToMerge(dst[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
			maxPartsToMerge--
		}
	} else {
		dst = appendPartsToMerge(dst[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
	}
	for _, pw := range dst {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge is already set")
		}
		pw.isInMerge = true
	}
	return dst
}

// minMergeMultiplier is the minimum multiplier for the size of the output part
// compared to the size of the maximum input part for the merge.
//
// Higher value reduces write amplification (disk write IO induced by the merge),
// while increases the number of unmerged parts.
// The 1.7 is good enough for production workloads.
const minMergeMultiplier = 1.7

// appendPartsToMerge finds optimal parts to merge from src, appends
// them to dst and returns the result.
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

func removeParts(pws []*partWrapper, partsToRemove map[*partWrapper]bool) ([]*partWrapper, int) {
	dst := pws[:0]
	for _, pw := range pws {
		if !partsToRemove[pw] {
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
	return name == "tmp" || name == "txn" || name == "snapshots" || name == "cache" || fs.IsScheduledForRemoval(name)
}
