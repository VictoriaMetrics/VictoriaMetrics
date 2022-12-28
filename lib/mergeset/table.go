package mergeset

import (
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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storagepacelimiter"
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

	snapshotLock sync.RWMutex

	flockF *os.File

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

	return tailItems
}

type partWrapper struct {
	p *part

	mp *inmemoryPart

	refCount uint64

	isInMerge bool

	// The deadline when the in-memory part must be flushed to disk.
	flushToDiskDeadline time.Time
}

func (pw *partWrapper) incRef() {
	atomic.AddUint64(&pw.refCount, 1)
}

func (pw *partWrapper) decRef() {
	n := atomic.AddUint64(&pw.refCount, ^uint64(0))
	if int64(n) < 0 {
		logger.Panicf("BUG: pw.refCount must be bigger than 0; got %d", int64(n))
	}
	if n > 0 {
		return
	}

	if pw.mp != nil {
		// Do not return pw.mp to pool via putInmemoryPart(),
		// since pw.mp size may be too big compared to other entries stored in the pool.
		// This may result in increased memory usage because of high fragmentation.
		pw.mp = nil
	}
	pw.p.MustClose()
	pw.p = nil
}

// OpenTable opens a table on the given path.
//
// Optional flushCallback is called every time new data batch is flushed
// to the underlying storage and becomes visible to search.
//
// Optional prepareBlock is called during merge before flushing the prepared block
// to persistent storage.
//
// The table is created if it doesn't exist yet.
func OpenTable(path string, flushCallback func(), prepareBlock PrepareBlockCallback, isReadOnly *uint32) (*Table, error) {
	path = filepath.Clean(path)
	logger.Infof("opening table %q...", path)
	startTime := time.Now()

	// Create a directory for the table if it doesn't exist yet.
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create directory %q: %w", path, err)
	}

	// Protect from concurrent opens.
	flockF, err := fs.CreateFlockFile(path)
	if err != nil {
		return nil, err
	}

	// Open table parts.
	pws, err := openParts(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open table parts at %q: %w", path, err)
	}

	tb := &Table{
		path:          path,
		flushCallback: flushCallback,
		prepareBlock:  prepareBlock,
		isReadOnly:    isReadOnly,
		fileParts:     pws,
		mergeIdx:      uint64(time.Now().UnixNano()),
		flockF:        flockF,
		stopCh:        make(chan struct{}),
	}
	tb.rawItems.init()
	tb.startBackgroundWorkers()

	var m TableMetrics
	tb.UpdateMetrics(&m)
	logger.Infof("table %q has been opened in %.3f seconds; partsCount: %d; blocksCount: %d, itemsCount: %d; sizeBytes: %d",
		path, time.Since(startTime).Seconds(), m.FilePartsCount, m.FileBlocksCount, m.FileItemsCount, m.FileSizeBytes)

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

	return tb, nil
}

func (tb *Table) startBackgroundWorkers() {
	tb.startMergeWorkers()
	tb.startInmemoryPartsFlusher()
	tb.startPendingItemsFlusher()
}

// MustClose closes the table.
func (tb *Table) MustClose() {
	close(tb.stopCh)

	logger.Infof("waiting for background workers to stop on %q...", tb.path)
	startTime := time.Now()
	tb.wg.Wait()
	logger.Infof("background workers stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), tb.path)

	logger.Infof("flushing inmemory parts to files on %q...", tb.path)
	startTime = time.Now()
	tb.flushInmemoryItems()
	logger.Infof("inmemory parts have been successfully flushed to files in %.3f seconds at %q", time.Since(startTime).Seconds(), tb.path)

	logger.Infof("waiting for flush callback worker to stop on %q...", tb.path)
	startTime = time.Now()
	tb.flushCallbackWorkerWG.Wait()
	logger.Infof("flush callback worker stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), tb.path)

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

	// Release flockF
	if err := tb.flockF.Close(); err != nil {
		logger.Panicf("FATAL:cannot close %q: %s", tb.flockF.Name(), err)
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
		m.PartsRefCount += atomic.LoadUint64(&pw.refCount)
	}

	m.FilePartsCount += uint64(len(tb.fileParts))
	for _, pw := range tb.fileParts {
		p := pw.p
		m.FileBlocksCount += p.ph.blocksCount
		m.FileItemsCount += p.ph.itemsCount
		m.FileSizeBytes += p.size
		m.PartsRefCount += atomic.LoadUint64(&pw.refCount)
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

// DebugFlush flushes all the added items to the storage, so they become visible to search.
//
// This function is only for debugging and testing.
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
	for {
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
		if !isFinal {
			return
		}
		tb.partsLock.Lock()
		n := len(tb.inmemoryParts)
		tb.partsLock.Unlock()
		if n == 0 {
			// All the in-memory parts were flushed to disk.
			return
		}
		// Some parts weren't flushed to disk because they were being merged.
		// Sleep for a while and try flushing them again.
		time.Sleep(10 * time.Millisecond)
	}
}

func (riss *rawItemsShards) flush(tb *Table, dst []*inmemoryBlock, isFinal bool) []*inmemoryBlock {
	tb.rawItemsPendingFlushesWG.Add(1)
	defer tb.rawItemsPendingFlushesWG.Done()

	for i := range riss.shards {
		dst = riss.shards[i].appendBlocksToFlush(dst, tb, isFinal)
	}
	tb.flushBlocksToParts(dst, isFinal)
	return dst
}

func (ris *rawItemsShard) appendBlocksToFlush(dst []*inmemoryBlock, tb *Table, isFinal bool) []*inmemoryBlock {
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
	tb.partsLock.Unlock()

	flushConcurrencyCh <- struct{}{}
	tb.assistedMergeForInmemoryParts()
	tb.assistedMergeForFileParts()
	<-flushConcurrencyCh

	if tb.flushCallback != nil {
		if isFinal {
			tb.flushCallback()
		} else {
			atomic.CompareAndSwapUint32(&tb.needFlushCallbackCall, 0, 1)
		}
	}
}

var flushConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

func needAssistedMerge(pws []*partWrapper, maxParts int) bool {
	if len(pws) < maxParts {
		return false
	}
	return getNotInMergePartsCount(pws) >= defaultPartsToMerge
}

func (tb *Table) assistedMergeForInmemoryParts() {
	for {
		tb.partsLock.Lock()
		needMerge := needAssistedMerge(tb.inmemoryParts, maxInmemoryParts)
		tb.partsLock.Unlock()
		if !needMerge {
			return
		}

		// Prioritize assisted merges over searches.
		storagepacelimiter.Search.Inc()
		atomic.AddUint64(&tb.inmemoryAssistedMerges, 1)
		err := tb.mergeInmemoryParts()
		storagepacelimiter.Search.Dec()
		if err == nil {
			continue
		}
		if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) {
			return
		}
		logger.Panicf("FATAL: cannot assist with merging inmemory parts: %s", err)
	}
}

func (tb *Table) assistedMergeForFileParts() {
	for {
		tb.partsLock.Lock()
		needMerge := needAssistedMerge(tb.fileParts, maxFileParts)
		tb.partsLock.Unlock()
		if !needMerge {
			return
		}

		// Prioritize assisted merges over searches.
		storagepacelimiter.Search.Inc()
		atomic.AddUint64(&tb.fileAssistedMerges, 1)
		err := tb.mergeExistingParts(false)
		storagepacelimiter.Search.Dec()
		if err == nil {
			continue
		}
		if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) || errors.Is(err, errReadOnlyMode) {
			return
		}
		logger.Panicf("FATAL: cannot assist with merging file parts: %s", err)
	}
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
		bsr.InitFromInmemoryBlock(ib)
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
	bsw.InitFromInmemoryPart(mpDst, compressLevel)

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
	// Start a merge worker per available CPU core.
	// The actual number of concurrent merges is limited inside mergeWorker() below.
	workersCount := cgroup.AvailableCPUs()
	for i := 0; i < workersCount; i++ {
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

const (
	minMergeSleepTime = 10 * time.Millisecond
	maxMergeSleepTime = 10 * time.Second
)

func (tb *Table) mergeWorker() {
	sleepTime := minMergeSleepTime
	var lastMergeTime uint64
	isFinal := false
	t := time.NewTimer(sleepTime)
	for {
		// Limit the number of concurrent calls to mergeExistingParts, since the total number of merge workers
		// across tables may exceed the the cap(mergeWorkersLimitCh).
		mergeWorkersLimitCh <- struct{}{}
		err := tb.mergeExistingParts(isFinal)
		<-mergeWorkersLimitCh
		if err == nil {
			// Try merging additional parts.
			sleepTime = minMergeSleepTime
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

		// Nothing to merge. Sleep for a while and try again.
		sleepTime *= 2
		if sleepTime > maxMergeSleepTime {
			sleepTime = maxMergeSleepTime
		}
		select {
		case <-tb.stopCh:
			return
		case <-t.C:
			t.Reset(sleepTime)
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
func (tb *Table) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal bool) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
	}
	defer tb.releasePartsToMerge(pws)

	startTime := time.Now()

	// Initialize destination paths.
	dstPartType := getDstPartType(pws, isFinal)
	tmpPartPath, mergeIdx := tb.getDstPartPaths(dstPartType)

	if isFinal && len(pws) == 1 && pws[0].mp != nil {
		// Fast path: flush a single in-memory part to disk.
		mp := pws[0].mp
		if tmpPartPath == "" {
			logger.Panicf("BUG: tmpPartPath must be non-empty")
		}
		if err := mp.StoreToDisk(tmpPartPath); err != nil {
			return fmt.Errorf("cannot store in-memory part to %q: %w", tmpPartPath, err)
		}
		pwNew, err := tb.openCreatedPart(&mp.ph, pws, nil, tmpPartPath, mergeIdx)
		if err != nil {
			return fmt.Errorf("cannot atomically register the created part: %w", err)
		}
		tb.swapSrcWithDstParts(pws, pwNew, dstPartType)
		return nil
	}

	// Prepare BlockStreamReaders for source parts.
	bsrs, err := openBlockStreamReaders(pws)
	if err != nil {
		return err
	}
	closeBlockStreamReaders := func() {
		for _, bsr := range bsrs {
			putBlockStreamReader(bsr)
		}
		bsrs = nil
	}

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
		bsw.InitFromInmemoryPart(mpNew, compressLevel)
	} else {
		if tmpPartPath == "" {
			logger.Panicf("BUG: tmpPartPath must be non-empty")
		}
		nocache := srcItemsCount > maxItemsPerCachedPart()
		if err := bsw.InitFromFilePart(tmpPartPath, nocache, compressLevel); err != nil {
			closeBlockStreamReaders()
			return fmt.Errorf("cannot create destination part at %q: %w", tmpPartPath, err)
		}
	}

	// Merge source parts to destination part.
	ph, err := tb.mergePartsInternal(tmpPartPath, bsw, bsrs, dstPartType, stopCh)
	putBlockStreamWriter(bsw)
	closeBlockStreamReaders()
	if err != nil {
		return fmt.Errorf("cannot merge %d parts: %w", len(pws), err)
	}
	if mpNew != nil {
		// Update partHeader for destination inmemory part after the merge.
		mpNew.ph = *ph
	}

	// Atomically move the created part from tmpPartPath to its destination
	// and swap the source parts with the newly created part.
	pwNew, err := tb.openCreatedPart(ph, pws, mpNew, tmpPartPath, mergeIdx)
	if err != nil {
		return fmt.Errorf("cannot atomically register the created part: %w", err)
	}
	tb.swapSrcWithDstParts(pws, pwNew, dstPartType)

	d := time.Since(startTime)
	if d <= 30*time.Second {
		return nil
	}

	// Log stats for long merges.
	dstItemsCount := uint64(0)
	dstBlocksCount := uint64(0)
	dstSize := uint64(0)
	dstPartPath := ""
	if pwNew != nil {
		pDst := pwNew.p
		dstItemsCount = pDst.ph.itemsCount
		dstBlocksCount = pDst.ph.blocksCount
		dstSize = pDst.size
		dstPartPath = pDst.path
	}
	durationSecs := d.Seconds()
	itemsPerSec := int(float64(srcItemsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d items, %d blocks, %d bytes) into (1 part, %d items, %d blocks, %d bytes) in %.3f seconds at %d items/sec to %q",
		len(pws), srcItemsCount, srcBlocksCount, srcSize, dstItemsCount, dstBlocksCount, dstSize, durationSecs, itemsPerSec, dstPartPath)

	return nil
}

func getFlushToDiskDeadline(pws []*partWrapper) time.Time {
	d := pws[0].flushToDiskDeadline
	for _, pw := range pws[1:] {
		if pw.flushToDiskDeadline.Before(d) {
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

func (tb *Table) getDstPartPaths(dstPartType partType) (string, uint64) {
	tmpPartPath := ""
	mergeIdx := tb.nextMergeIdx()
	switch dstPartType {
	case partInmemory:
	case partFile:
		tmpPartPath = fmt.Sprintf("%s/tmp/%016X", tb.path, mergeIdx)
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}
	return tmpPartPath, mergeIdx
}

func openBlockStreamReaders(pws []*partWrapper) ([]*blockStreamReader, error) {
	bsrs := make([]*blockStreamReader, 0, len(pws))
	for _, pw := range pws {
		bsr := getBlockStreamReader()
		if pw.mp != nil {
			bsr.InitFromInmemoryPart(pw.mp)
		} else {
			if err := bsr.InitFromFilePart(pw.p.path); err != nil {
				for _, bsr := range bsrs {
					putBlockStreamReader(bsr)
				}
				return nil, fmt.Errorf("cannot open source part for merging: %w", err)
			}
		}
		bsrs = append(bsrs, bsr)
	}
	return bsrs, nil
}

func (tb *Table) mergePartsInternal(tmpPartPath string, bsw *blockStreamWriter, bsrs []*blockStreamReader, dstPartType partType, stopCh <-chan struct{}) (*partHeader, error) {
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
		return nil, fmt.Errorf("cannot merge parts to %q: %w", tmpPartPath, err)
	}
	if tmpPartPath != "" {
		if err := ph.WriteMetadata(tmpPartPath); err != nil {
			return nil, fmt.Errorf("cannot write metadata to destination part %q: %w", tmpPartPath, err)
		}
	}
	return &ph, nil
}

func (tb *Table) openCreatedPart(ph *partHeader, pws []*partWrapper, mpNew *inmemoryPart, tmpPartPath string, mergeIdx uint64) (*partWrapper, error) {
	dstPartPath := ""
	if mpNew == nil || !areAllInmemoryParts(pws) {
		// Either source or destination parts are located on disk.
		// Create a transaction for atomic deleting of old parts and moving new part to its destination on disk.
		var bb bytesutil.ByteBuffer
		for _, pw := range pws {
			if pw.mp == nil {
				fmt.Fprintf(&bb, "%s\n", pw.p.path)
			}
		}
		dstPartPath = ph.Path(tb.path, mergeIdx)
		fmt.Fprintf(&bb, "%s -> %s\n", tmpPartPath, dstPartPath)
		txnPath := fmt.Sprintf("%s/txn/%016X", tb.path, mergeIdx)
		if err := fs.WriteFileAtomically(txnPath, bb.B, false); err != nil {
			return nil, fmt.Errorf("cannot create transaction file %q: %w", txnPath, err)
		}

		// Run the created transaction.
		if err := runTransaction(&tb.snapshotLock, tb.path, txnPath); err != nil {
			return nil, fmt.Errorf("cannot execute transaction %q: %w", txnPath, err)
		}
	}
	// Open the created part.
	if mpNew != nil {
		// Open the created part from memory.
		flushToDiskDeadline := getFlushToDiskDeadline(pws)
		pwNew := newPartWrapperFromInmemoryPart(mpNew, flushToDiskDeadline)
		return pwNew, nil
	}
	// Open the created part from disk.
	pNew, err := openFilePart(dstPartPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open merged part %q: %w", dstPartPath, err)
	}
	pwNew := &partWrapper{
		p:        pNew,
		refCount: 1,
	}
	return pwNew, nil
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
	if pwNew != nil {
		switch dstPartType {
		case partInmemory:
			tb.inmemoryParts = append(tb.inmemoryParts, pwNew)
		case partFile:
			tb.fileParts = append(tb.fileParts, pwNew)
		default:
			logger.Panicf("BUG: unknown partType=%d", dstPartType)
		}
	}
	tb.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedFileParts
	if removedParts != len(m) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(m))
	}

	// Remove references from old parts.
	for _, pw := range pws {
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

var mergeWorkersLimitCh = make(chan struct{}, cgroup.AvailableCPUs())

func openParts(path string) ([]*partWrapper, error) {
	// The path can be missing after restoring from backup, so create it if needed.
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, err
	}
	fs.MustRemoveTemporaryDirs(path)
	d, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open difrectory: %w", err)
	}
	defer fs.MustClose(d)

	// Run remaining transactions and cleanup /txn and /tmp directories.
	// Snapshots cannot be created yet, so use fakeSnapshotLock.
	var fakeSnapshotLock sync.RWMutex
	if err := runTransactions(&fakeSnapshotLock, path); err != nil {
		return nil, fmt.Errorf("cannot run transactions: %w", err)
	}

	txnDir := path + "/txn"
	fs.MustRemoveDirAtomic(txnDir)
	if err := fs.MkdirAllFailIfExist(txnDir); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", txnDir, err)
	}

	tmpDir := path + "/tmp"
	fs.MustRemoveDirAtomic(tmpDir)
	if err := fs.MkdirAllFailIfExist(tmpDir); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", tmpDir, err)
	}

	fs.MustSyncPath(path)

	// Open parts.
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}
	var pws []*partWrapper
	for _, fi := range fis {
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories.
			continue
		}
		fn := fi.Name()
		if isSpecialDir(fn) {
			// Skip special dirs.
			continue
		}
		partPath := path + "/" + fn
		if fs.IsEmptyDir(partPath) {
			// Remove empty directory, which can be left after unclean shutdown on NFS.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1142
			fs.MustRemoveDirAtomic(partPath)
			continue
		}
		p, err := openFilePart(partPath)
		if err != nil {
			mustCloseParts(pws)
			return nil, fmt.Errorf("cannot open part %q: %w", partPath, err)
		}
		pw := &partWrapper{
			p:        p,
			refCount: 1,
		}
		pws = append(pws, pw)
	}

	return pws, nil
}

func mustCloseParts(pws []*partWrapper) {
	for _, pw := range pws {
		if pw.refCount != 1 {
			logger.Panicf("BUG: unexpected refCount when closing part %q: %d; want 1", pw.p.path, pw.refCount)
		}
		pw.p.MustClose()
	}
}

// CreateSnapshotAt creates tb snapshot in the given dstDir.
//
// Snapshot is created using linux hard links, so it is usually created
// very quickly.
func (tb *Table) CreateSnapshotAt(dstDir string) error {
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
	if strings.HasPrefix(dstDir, srcDir+"/") {
		return fmt.Errorf("cannot create snapshot %q inside the data dir %q", dstDir, srcDir)
	}

	// Flush inmemory items to disk.
	tb.flushInmemoryItems()

	// The snapshot must be created under the lock in order to prevent from
	// concurrent modifications via runTransaction.
	tb.snapshotLock.Lock()
	defer tb.snapshotLock.Unlock()

	if err := fs.MkdirAllFailIfExist(dstDir); err != nil {
		return fmt.Errorf("cannot create snapshot dir %q: %w", dstDir, err)
	}

	d, err := os.Open(srcDir)
	if err != nil {
		return fmt.Errorf("cannot open difrectory: %w", err)
	}
	defer fs.MustClose(d)

	fis, err := d.Readdir(-1)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}
	for _, fi := range fis {
		fn := fi.Name()
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories.
			continue
		}
		if isSpecialDir(fn) {
			// Skip special dirs.
			continue
		}
		srcPartPath := srcDir + "/" + fn
		dstPartPath := dstDir + "/" + fn
		if err := fs.HardLinkFiles(srcPartPath, dstPartPath); err != nil {
			return fmt.Errorf("cannot create hard links from %q to %q: %w", srcPartPath, dstPartPath, err)
		}
	}

	fs.MustSyncPath(dstDir)
	parentDir := filepath.Dir(dstDir)
	fs.MustSyncPath(parentDir)

	logger.Infof("created Table snapshot of %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	return nil
}

func runTransactions(txnLock *sync.RWMutex, path string) error {
	// Wait until all the previous pending transaction deletions are finished.
	pendingTxnDeletionsWG.Wait()

	// Make sure all the current transaction deletions are finished before exiting.
	defer pendingTxnDeletionsWG.Wait()

	txnDir := path + "/txn"
	d, err := os.Open(txnDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot open transaction dir: %w", err)
	}
	defer fs.MustClose(d)

	fis, err := d.Readdir(-1)
	if err != nil {
		return fmt.Errorf("cannot read directory %q: %w", d.Name(), err)
	}

	// Sort transaction files by id, since transactions must be ordered.
	sort.Slice(fis, func(i, j int) bool {
		return fis[i].Name() < fis[j].Name()
	})

	for _, fi := range fis {
		fn := fi.Name()
		if fs.IsTemporaryFileName(fn) {
			// Skip temporary files, which could be left after unclean shutdown.
			continue
		}
		txnPath := txnDir + "/" + fn
		if err := runTransaction(txnLock, path, txnPath); err != nil {
			return fmt.Errorf("cannot run transaction from %q: %w", txnPath, err)
		}
	}
	return nil
}

func runTransaction(txnLock *sync.RWMutex, pathPrefix, txnPath string) error {
	// The transaction must run under read lock in order to provide
	// consistent snapshots with Table.CreateSnapshot().
	txnLock.RLock()
	defer txnLock.RUnlock()

	data, err := os.ReadFile(txnPath)
	if err != nil {
		return fmt.Errorf("cannot read transaction file: %w", err)
	}
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	paths := strings.Split(string(data), "\n")

	if len(paths) == 0 {
		return fmt.Errorf("empty transaction")
	}
	rmPaths := paths[:len(paths)-1]
	mvPaths := strings.Split(paths[len(paths)-1], " -> ")
	if len(mvPaths) != 2 {
		return fmt.Errorf("invalid last line in the transaction file: got %q; must contain `srcPath -> dstPath`", paths[len(paths)-1])
	}

	// Remove old paths. It is OK if certain paths don't exist.
	for _, path := range rmPaths {
		path, err := validatePath(pathPrefix, path)
		if err != nil {
			return fmt.Errorf("invalid path to remove: %w", err)
		}
		fs.MustRemoveDirAtomic(path)
	}

	// Move the new part to new directory.
	srcPath := mvPaths[0]
	dstPath := mvPaths[1]
	srcPath, err = validatePath(pathPrefix, srcPath)
	if err != nil {
		return fmt.Errorf("invalid source path to rename: %w", err)
	}
	dstPath, err = validatePath(pathPrefix, dstPath)
	if err != nil {
		return fmt.Errorf("invalid destination path to rename: %w", err)
	}
	if fs.IsPathExist(srcPath) {
		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("cannot rename %q to %q: %w", srcPath, dstPath, err)
		}
	} else if !fs.IsPathExist(dstPath) {
		// Emit info message for the expected condition after unclean shutdown on NFS disk.
		// The dstPath part may be missing because it could be already merged into bigger part
		// while old source parts for the current txn weren't still deleted due to NFS locks.
		logger.Infof("cannot find both source and destination paths: %q -> %q; this may be the case after unclean shutdown (OOM, `kill -9`, hard reset) on NFS disk",
			srcPath, dstPath)
	}

	// Flush pathPrefix directory metadata to the underying storage.
	fs.MustSyncPath(pathPrefix)

	pendingTxnDeletionsWG.Add(1)
	go func() {
		defer pendingTxnDeletionsWG.Done()
		if err := os.Remove(txnPath); err != nil {
			logger.Errorf("cannot remove transaction file %q: %s", txnPath, err)
		}
	}()

	return nil
}

var pendingTxnDeletionsWG syncwg.WaitGroup

func validatePath(pathPrefix, path string) (string, error) {
	var err error

	pathPrefix, err = filepath.Abs(pathPrefix)
	if err != nil {
		return path, fmt.Errorf("cannot determine absolute path for pathPrefix=%q: %w", pathPrefix, err)
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return path, fmt.Errorf("cannot determine absolute path for %q: %w", path, err)
	}
	if !strings.HasPrefix(path, pathPrefix+"/") {
		return path, fmt.Errorf("invalid path %q; must start with %q", path, pathPrefix+"/")
	}
	return path, nil
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
	return name == "tmp" || name == "txn" || name == "snapshots" || name == "cache"
}
