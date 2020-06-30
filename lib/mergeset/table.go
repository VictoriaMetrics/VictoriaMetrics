package mergeset

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
)

// These are global counters for cache requests and misses for parts
// which were already merged into another parts.
var (
	historicalDataBlockCacheRequests uint64
	historicalDataBlockCacheMisses   uint64

	historicalIndexBlockCacheRequests uint64
	historicalIndexBlockCacheMisses   uint64
)

// maxParts is the maximum number of parts in the table.
//
// This number may be reached when the insertion pace outreaches merger pace.
const maxParts = 512

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

// maxItemsPerPart is the absolute maximum number of items per part.
//
// This number should be limited by the amount of time required to merge
// such number of items. The required time shouldn't exceed a day.
//
// TODO: adjust this number using production stats.
const maxItemsPerPart = 100e9

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

// The interval for flushing (converting) recent raw items into parts,
// so they become visible to search.
const rawItemsFlushInterval = time.Second

// Table represents mergeset table.
type Table struct {
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	activeMerges   uint64
	mergesCount    uint64
	itemsMerged    uint64
	assistedMerges uint64

	mergeIdx uint64

	path string

	flushCallback func()

	prepareBlock PrepareBlockCallback

	partsLock sync.Mutex
	parts     []*partWrapper

	rawItemsBlocks        []*inmemoryBlock
	rawItemsLock          sync.Mutex
	rawItemsLastFlushTime uint64

	snapshotLock sync.RWMutex

	flockF *os.File

	stopCh chan struct{}

	// Use syncwg instead of sync, since Add/Wait may be called from concurrent goroutines.
	partMergersWG syncwg.WaitGroup

	rawItemsFlusherWG sync.WaitGroup

	convertersWG sync.WaitGroup

	// Use syncwg instead of sync, since Add/Wait may be called from concurrent goroutines.
	rawItemsPendingFlushesWG syncwg.WaitGroup
}

type partWrapper struct {
	p *part

	mp *inmemoryPart

	refCount uint64

	isInMerge bool
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
		putInmemoryPart(pw.mp)
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
func OpenTable(path string, flushCallback func(), prepareBlock PrepareBlockCallback) (*Table, error) {
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
		parts:         pws,
		mergeIdx:      uint64(time.Now().UnixNano()),
		flockF:        flockF,
		stopCh:        make(chan struct{}),
	}
	tb.startPartMergers()
	tb.startRawItemsFlusher()

	var m TableMetrics
	tb.UpdateMetrics(&m)
	logger.Infof("table %q has been opened in %.3f seconds; partsCount: %d; blocksCount: %d, itemsCount: %d; sizeBytes: %d",
		path, time.Since(startTime).Seconds(), m.PartsCount, m.BlocksCount, m.ItemsCount, m.SizeBytes)

	tb.convertersWG.Add(1)
	go func() {
		tb.convertToV1280()
		tb.convertersWG.Done()
	}()

	return tb, nil
}

// MustClose closes the table.
func (tb *Table) MustClose() {
	close(tb.stopCh)

	logger.Infof("waiting for raw items flusher to stop on %q...", tb.path)
	startTime := time.Now()
	tb.rawItemsFlusherWG.Wait()
	logger.Infof("raw items flusher stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), tb.path)

	logger.Infof("waiting for converters to stop on %q...", tb.path)
	startTime = time.Now()
	tb.convertersWG.Wait()
	logger.Infof("converters stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), tb.path)

	logger.Infof("waiting for part mergers to stop on %q...", tb.path)
	startTime = time.Now()
	tb.partMergersWG.Wait()
	logger.Infof("part mergers stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), tb.path)

	logger.Infof("flushing inmemory parts to files on %q...", tb.path)
	startTime = time.Now()

	// Flush raw items the last time before exit.
	tb.flushRawItems(true)

	// Flush inmemory parts to disk.
	var pws []*partWrapper
	tb.partsLock.Lock()
	for _, pw := range tb.parts {
		if pw.mp == nil {
			continue
		}
		if pw.isInMerge {
			logger.Panicf("BUG: the inmemory part %s mustn't be in merge after stopping parts merger in %q", &pw.mp.ph, tb.path)
		}
		pw.isInMerge = true
		pws = append(pws, pw)
	}
	tb.partsLock.Unlock()

	if err := tb.mergePartsOptimal(pws, nil); err != nil {
		logger.Panicf("FATAL: cannot flush inmemory parts to files in %q: %s", tb.path, err)
	}
	logger.Infof("%d inmemory parts have been flushed to files in %.3f seconds on %q", len(pws), time.Since(startTime).Seconds(), tb.path)

	// Remove references to parts from the tb, so they may be eventually closed
	// after all the searches are done.
	tb.partsLock.Lock()
	parts := tb.parts
	tb.parts = nil
	tb.partsLock.Unlock()

	for _, pw := range parts {
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
	ActiveMerges   uint64
	MergesCount    uint64
	ItemsMerged    uint64
	AssistedMerges uint64

	PendingItems uint64

	PartsCount uint64

	BlocksCount uint64
	ItemsCount  uint64
	SizeBytes   uint64

	DataBlocksCacheSize     uint64
	DataBlocksCacheRequests uint64
	DataBlocksCacheMisses   uint64

	IndexBlocksCacheSize     uint64
	IndexBlocksCacheRequests uint64
	IndexBlocksCacheMisses   uint64

	PartsRefCount uint64
}

// UpdateMetrics updates m with metrics from tb.
func (tb *Table) UpdateMetrics(m *TableMetrics) {
	m.ActiveMerges += atomic.LoadUint64(&tb.activeMerges)
	m.MergesCount += atomic.LoadUint64(&tb.mergesCount)
	m.ItemsMerged += atomic.LoadUint64(&tb.itemsMerged)
	m.AssistedMerges += atomic.LoadUint64(&tb.assistedMerges)

	tb.rawItemsLock.Lock()
	for _, ib := range tb.rawItemsBlocks {
		m.PendingItems += uint64(len(ib.items))
	}
	tb.rawItemsLock.Unlock()

	tb.partsLock.Lock()
	m.PartsCount += uint64(len(tb.parts))
	for _, pw := range tb.parts {
		p := pw.p

		m.BlocksCount += p.ph.blocksCount
		m.ItemsCount += p.ph.itemsCount
		m.SizeBytes += p.size

		m.DataBlocksCacheSize += p.ibCache.Len()
		m.DataBlocksCacheRequests += p.ibCache.Requests()
		m.DataBlocksCacheMisses += p.ibCache.Misses()

		m.IndexBlocksCacheSize += p.idxbCache.Len()
		m.IndexBlocksCacheRequests += p.idxbCache.Requests()
		m.IndexBlocksCacheMisses += p.idxbCache.Misses()

		m.PartsRefCount += atomic.LoadUint64(&pw.refCount)
	}
	tb.partsLock.Unlock()

	m.DataBlocksCacheRequests += atomic.LoadUint64(&historicalDataBlockCacheRequests)
	m.DataBlocksCacheMisses += atomic.LoadUint64(&historicalDataBlockCacheMisses)

	m.IndexBlocksCacheRequests += atomic.LoadUint64(&historicalIndexBlockCacheRequests)
	m.IndexBlocksCacheMisses += atomic.LoadUint64(&historicalIndexBlockCacheMisses)
}

// AddItems adds the given items to the tb.
func (tb *Table) AddItems(items [][]byte) error {
	var err error
	var blocksToMerge []*inmemoryBlock

	tb.rawItemsLock.Lock()
	if len(tb.rawItemsBlocks) == 0 {
		ib := getInmemoryBlock()
		tb.rawItemsBlocks = append(tb.rawItemsBlocks, ib)
	}
	ib := tb.rawItemsBlocks[len(tb.rawItemsBlocks)-1]
	for _, item := range items {
		if !ib.Add(item) {
			ib = getInmemoryBlock()
			if !ib.Add(item) {
				putInmemoryBlock(ib)
				err = fmt.Errorf("cannot insert an item %q into an empty inmemoryBlock on %q; it looks like the item is too large? len(item)=%d",
					item, tb.path, len(item))
				break
			}
			tb.rawItemsBlocks = append(tb.rawItemsBlocks, ib)
		}
	}
	if len(tb.rawItemsBlocks) >= 1024 {
		blocksToMerge = tb.rawItemsBlocks
		tb.rawItemsBlocks = nil
		tb.rawItemsLastFlushTime = fasttime.UnixTimestamp()
	}
	tb.rawItemsLock.Unlock()

	if blocksToMerge == nil {
		// Fast path.
		return err
	}

	// Slow path: merge blocksToMerge.
	tb.mergeRawItemsBlocks(blocksToMerge)
	return err
}

// getParts appends parts snapshot to dst and returns it.
//
// The appended parts must be released with putParts.
func (tb *Table) getParts(dst []*partWrapper) []*partWrapper {
	tb.partsLock.Lock()
	for _, pw := range tb.parts {
		pw.incRef()
	}
	dst = append(dst, tb.parts...)
	tb.partsLock.Unlock()

	return dst
}

// putParts releases the given pws obtained via getParts.
func (tb *Table) putParts(pws []*partWrapper) {
	for _, pw := range pws {
		pw.decRef()
	}
}

func (tb *Table) startRawItemsFlusher() {
	tb.rawItemsFlusherWG.Add(1)
	go func() {
		tb.rawItemsFlusher()
		tb.rawItemsFlusherWG.Done()
	}()
}

func (tb *Table) rawItemsFlusher() {
	ticker := time.NewTicker(rawItemsFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			tb.flushRawItems(false)
		}
	}
}

const convertToV1280FileName = "converted-to-v1.28.0"

func (tb *Table) convertToV1280() {
	// Convert tag->metricID rows into tag->metricIDs rows when upgrading to v1.28.0+.
	flagFilePath := tb.path + "/" + convertToV1280FileName
	if fs.IsPathExist(flagFilePath) {
		// The conversion has been already performed.
		return
	}

	getAllPartsForMerge := func() []*partWrapper {
		var pws []*partWrapper
		tb.partsLock.Lock()
		for _, pw := range tb.parts {
			if pw.isInMerge {
				continue
			}
			pw.isInMerge = true
			pws = append(pws, pw)
		}
		tb.partsLock.Unlock()
		return pws
	}
	pws := getAllPartsForMerge()
	if len(pws) > 0 {
		logger.Infof("started round 1 of background conversion of %q to v1.28.0 format; merge %d parts", tb.path, len(pws))
		startTime := time.Now()
		if err := tb.mergePartsOptimal(pws, tb.stopCh); err != nil {
			logger.Errorf("failed round 1 of background conversion of %q to v1.28.0 format: %s", tb.path, err)
			return
		}
		logger.Infof("finished round 1 of background conversion of %q to v1.28.0 format in %.3f seconds", tb.path, time.Since(startTime).Seconds())

		// The second round is needed in order to merge small blocks
		// with tag->metricIDs rows left after the first round.
		pws = getAllPartsForMerge()
		logger.Infof("started round 2 of background conversion of %q to v1.28.0 format; merge %d parts", tb.path, len(pws))
		startTime = time.Now()
		if len(pws) > 0 {
			if err := tb.mergePartsOptimal(pws, tb.stopCh); err != nil {
				logger.Errorf("failed round 2 of background conversion of %q to v1.28.0 format: %s", tb.path, err)
				return
			}
		}
		logger.Infof("finished round 2 of background conversion of %q to v1.28.0 format in %.3f seconds", tb.path, time.Since(startTime).Seconds())
	}

	if err := fs.WriteFileAtomically(flagFilePath, []byte("ok")); err != nil {
		logger.Panicf("FATAL: cannot create %q: %s", flagFilePath, err)
	}
}

func (tb *Table) mergePartsOptimal(pws []*partWrapper, stopCh <-chan struct{}) error {
	for len(pws) > defaultPartsToMerge {
		if err := tb.mergeParts(pws[:defaultPartsToMerge], stopCh, false); err != nil {
			return fmt.Errorf("cannot merge %d parts: %w", defaultPartsToMerge, err)
		}
		pws = pws[defaultPartsToMerge:]
	}
	if len(pws) > 0 {
		if err := tb.mergeParts(pws, stopCh, false); err != nil {
			return fmt.Errorf("cannot merge %d parts: %w", len(pws), err)
		}
	}
	return nil
}

// DebugFlush flushes all the added items to the storage,
// so they become visible to search.
//
// This function is only for debugging and testing.
func (tb *Table) DebugFlush() {
	tb.flushRawItems(true)

	// Wait for background flushers to finish.
	tb.rawItemsPendingFlushesWG.Wait()
}

func (tb *Table) flushRawItems(isFinal bool) {
	tb.rawItemsPendingFlushesWG.Add(1)
	defer tb.rawItemsPendingFlushesWG.Done()

	mustFlush := false
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(rawItemsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	var blocksToMerge []*inmemoryBlock

	tb.rawItemsLock.Lock()
	if isFinal || currentTime-tb.rawItemsLastFlushTime > uint64(flushSeconds) {
		mustFlush = true
		blocksToMerge = tb.rawItemsBlocks
		tb.rawItemsBlocks = nil
		tb.rawItemsLastFlushTime = currentTime
	}
	tb.rawItemsLock.Unlock()

	if mustFlush {
		tb.mergeRawItemsBlocks(blocksToMerge)
	}
}

func (tb *Table) mergeRawItemsBlocks(blocksToMerge []*inmemoryBlock) {
	tb.partMergersWG.Add(1)
	defer tb.partMergersWG.Done()

	pws := make([]*partWrapper, 0, (len(blocksToMerge)+defaultPartsToMerge-1)/defaultPartsToMerge)
	for len(blocksToMerge) > 0 {
		n := defaultPartsToMerge
		if n > len(blocksToMerge) {
			n = len(blocksToMerge)
		}
		pw := tb.mergeInmemoryBlocks(blocksToMerge[:n])
		blocksToMerge = blocksToMerge[n:]
		if pw == nil {
			continue
		}
		pw.isInMerge = true
		pws = append(pws, pw)
	}
	if len(pws) > 0 {
		if err := tb.mergeParts(pws, nil, true); err != nil {
			logger.Panicf("FATAL: cannot merge raw parts: %s", err)
		}
		if tb.flushCallback != nil {
			tb.flushCallback()
		}
	}

	for {
		tb.partsLock.Lock()
		ok := len(tb.parts) <= maxParts
		tb.partsLock.Unlock()
		if ok {
			return
		}

		// The added part exceeds maxParts count. Assist with merging other parts.
		err := tb.mergeExistingParts(false)
		if err == nil {
			atomic.AddUint64(&tb.assistedMerges, 1)
			continue
		}
		if err == errNothingToMerge || err == errForciblyStopped {
			return
		}
		logger.Panicf("FATAL: cannot merge small parts: %s", err)
	}
}

func (tb *Table) mergeInmemoryBlocks(blocksToMerge []*inmemoryBlock) *partWrapper {
	// Convert blocksToMerge into inmemoryPart's
	mps := make([]*inmemoryPart, 0, len(blocksToMerge))
	for _, ib := range blocksToMerge {
		if len(ib.items) == 0 {
			continue
		}
		mp := getInmemoryPart()
		mp.Init(ib)
		putInmemoryBlock(ib)
		mps = append(mps, mp)
	}
	if len(mps) == 0 {
		return nil
	}
	if len(mps) == 1 {
		// Nothing to merge. Just return a single inmemory part.
		mp := mps[0]
		p := mp.NewPart()
		return &partWrapper{
			p:        p,
			mp:       mp,
			refCount: 1,
		}
	}
	defer func() {
		// Return source inmemoryParts to pool.
		for _, mp := range mps {
			putInmemoryPart(mp)
		}
	}()

	atomic.AddUint64(&tb.mergesCount, 1)
	atomic.AddUint64(&tb.activeMerges, 1)
	defer atomic.AddUint64(&tb.activeMerges, ^uint64(0))

	// Prepare blockStreamReaders for source parts.
	bsrs := make([]*blockStreamReader, 0, len(mps))
	for _, mp := range mps {
		bsr := getBlockStreamReader()
		bsr.InitFromInmemoryPart(mp)
		bsrs = append(bsrs, bsr)
	}

	// Prepare blockStreamWriter for destination part.
	bsw := getBlockStreamWriter()
	mpDst := getInmemoryPart()
	bsw.InitFromInmemoryPart(mpDst)

	// Merge parts.
	// The merge shouldn't be interrupted by stopCh,
	// since it may be final after stopCh is closed.
	if err := mergeBlockStreams(&mpDst.ph, bsw, bsrs, tb.prepareBlock, nil, &tb.itemsMerged); err != nil {
		logger.Panicf("FATAL: cannot merge inmemoryBlocks: %s", err)
	}
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}

	p := mpDst.NewPart()
	return &partWrapper{
		p:        p,
		mp:       mpDst,
		refCount: 1,
	}
}

func (tb *Table) startPartMergers() {
	for i := 0; i < mergeWorkersCount; i++ {
		tb.partMergersWG.Add(1)
		go func() {
			if err := tb.partMerger(); err != nil {
				logger.Panicf("FATAL: unrecoverable error when merging parts in %q: %s", tb.path, err)
			}
			tb.partMergersWG.Done()
		}()
	}
}

func (tb *Table) mergeExistingParts(isFinal bool) error {
	maxItems := tb.maxOutPartItems()
	if maxItems > maxItemsPerPart {
		maxItems = maxItemsPerPart
	}

	tb.partsLock.Lock()
	pws := getPartsToMerge(tb.parts, maxItems, isFinal)
	tb.partsLock.Unlock()

	return tb.mergeParts(pws, tb.stopCh, false)
}

const (
	minMergeSleepTime = time.Millisecond
	maxMergeSleepTime = time.Second
)

func (tb *Table) partMerger() error {
	sleepTime := minMergeSleepTime
	var lastMergeTime uint64
	isFinal := false
	t := time.NewTimer(sleepTime)
	for {
		err := tb.mergeExistingParts(isFinal)
		if err == nil {
			// Try merging additional parts.
			sleepTime = minMergeSleepTime
			lastMergeTime = fasttime.UnixTimestamp()
			isFinal = false
			continue
		}
		if err == errForciblyStopped {
			// The merger has been stopped.
			return nil
		}
		if err != errNothingToMerge {
			return err
		}
		if fasttime.UnixTimestamp()-lastMergeTime > 30 {
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
			return nil
		case <-t.C:
			t.Reset(sleepTime)
		}
	}
}

var errNothingToMerge = fmt.Errorf("nothing to merge")

func (tb *Table) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isOuterParts bool) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
	}

	atomic.AddUint64(&tb.mergesCount, 1)
	atomic.AddUint64(&tb.activeMerges, 1)
	defer atomic.AddUint64(&tb.activeMerges, ^uint64(0))

	startTime := time.Now()

	defer func() {
		// Remove isInMerge flag from pws.
		tb.partsLock.Lock()
		for _, pw := range pws {
			if !pw.isInMerge {
				logger.Panicf("BUG: missing isInMerge flag on the part %q", pw.p.path)
			}
			pw.isInMerge = false
		}
		tb.partsLock.Unlock()
	}()

	// Prepare blockStreamReaders for source parts.
	bsrs := make([]*blockStreamReader, 0, len(pws))
	defer func() {
		for _, bsr := range bsrs {
			putBlockStreamReader(bsr)
		}
	}()
	for _, pw := range pws {
		bsr := getBlockStreamReader()
		if pw.mp != nil {
			if !isOuterParts {
				logger.Panicf("BUG: inmemory part must be always outer")
			}
			bsr.InitFromInmemoryPart(pw.mp)
		} else {
			if err := bsr.InitFromFilePart(pw.p.path); err != nil {
				return fmt.Errorf("cannot open source part for merging: %w", err)
			}
		}
		bsrs = append(bsrs, bsr)
	}

	outItemsCount := uint64(0)
	outBlocksCount := uint64(0)
	for _, pw := range pws {
		outItemsCount += pw.p.ph.itemsCount
		outBlocksCount += pw.p.ph.blocksCount
	}
	nocache := true
	if outItemsCount < maxItemsPerCachedPart() {
		// Cache small (i.e. recent) output parts in OS file cache,
		// since there is high chance they will be read soon.
		nocache = false
	}

	// Prepare blockStreamWriter for destination part.
	mergeIdx := tb.nextMergeIdx()
	tmpPartPath := fmt.Sprintf("%s/tmp/%016X", tb.path, mergeIdx)
	bsw := getBlockStreamWriter()
	compressLevel := getCompressLevelForPartItems(outItemsCount, outBlocksCount)
	if err := bsw.InitFromFilePart(tmpPartPath, nocache, compressLevel); err != nil {
		return fmt.Errorf("cannot create destination part %q: %w", tmpPartPath, err)
	}

	// Merge parts into a temporary location.
	var ph partHeader
	err := mergeBlockStreams(&ph, bsw, bsrs, tb.prepareBlock, stopCh, &tb.itemsMerged)
	putBlockStreamWriter(bsw)
	if err != nil {
		if err == errForciblyStopped {
			return err
		}
		return fmt.Errorf("error when merging parts to %q: %w", tmpPartPath, err)
	}
	if err := ph.WriteMetadata(tmpPartPath); err != nil {
		return fmt.Errorf("cannot write metadata to destination part %q: %w", tmpPartPath, err)
	}

	// Close bsrs (aka source parts).
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	bsrs = nil

	// Create a transaction for atomic deleting old parts and moving
	// new part to its destination place.
	var bb bytesutil.ByteBuffer
	for _, pw := range pws {
		if pw.mp == nil {
			fmt.Fprintf(&bb, "%s\n", pw.p.path)
		}
	}
	dstPartPath := ph.Path(tb.path, mergeIdx)
	fmt.Fprintf(&bb, "%s -> %s\n", tmpPartPath, dstPartPath)
	txnPath := fmt.Sprintf("%s/txn/%016X", tb.path, mergeIdx)
	if err := fs.WriteFileAtomically(txnPath, bb.B); err != nil {
		return fmt.Errorf("cannot create transaction file %q: %w", txnPath, err)
	}

	// Run the created transaction.
	if err := runTransaction(&tb.snapshotLock, tb.path, txnPath); err != nil {
		return fmt.Errorf("cannot execute transaction %q: %w", txnPath, err)
	}

	// Open the merged part.
	newP, err := openFilePart(dstPartPath)
	if err != nil {
		return fmt.Errorf("cannot open merged part %q: %w", dstPartPath, err)
	}
	newPSize := newP.size
	newPW := &partWrapper{
		p:        newP,
		refCount: 1,
	}

	// Atomically remove old parts and add new part.
	m := make(map[*partWrapper]bool, len(pws))
	for _, pw := range pws {
		m[pw] = true
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found in the merge of %d parts", len(pws)-len(m), len(pws))
	}
	removedParts := 0
	tb.partsLock.Lock()
	tb.parts, removedParts = removeParts(tb.parts, m)
	tb.parts = append(tb.parts, newPW)
	tb.partsLock.Unlock()
	if removedParts != len(m) {
		if !isOuterParts {
			logger.Panicf("BUG: unexpected number of parts removed; got %d; want %d", removedParts, len(m))
		}
		if removedParts != 0 {
			logger.Panicf("BUG: removed non-zero outer parts: %d", removedParts)
		}
	}

	// Remove partition references from old parts.
	for _, pw := range pws {
		pw.decRef()
	}

	d := time.Since(startTime)
	if d > 10*time.Second {
		logger.Infof("merged %d items across %d blocks in %.3f seconds at %d items/sec to %q; sizeBytes: %d",
			outItemsCount, outBlocksCount, d.Seconds(), int(float64(outItemsCount)/d.Seconds()), dstPartPath, newPSize)
	}

	return nil
}

func getCompressLevelForPartItems(itemsCount, blocksCount uint64) int {
	// There is no need in using blocksCount here, since mergeset blocks are usually full.

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

var (
	maxOutPartItemsLock     sync.Mutex
	maxOutPartItemsDeadline uint64
	lastMaxOutPartItems     uint64
)

func (tb *Table) maxOutPartItems() uint64 {
	maxOutPartItemsLock.Lock()
	if maxOutPartItemsDeadline < fasttime.UnixTimestamp() {
		lastMaxOutPartItems = tb.maxOutPartItemsSlow()
		maxOutPartItemsDeadline = fasttime.UnixTimestamp() + 2
	}
	n := lastMaxOutPartItems
	maxOutPartItemsLock.Unlock()
	return n
}

func (tb *Table) maxOutPartItemsSlow() uint64 {
	freeSpace := fs.MustGetFreeSpace(tb.path)

	// Calculate the maximum number of items in the output merge part
	// by dividing the freeSpace by 4 and by the number of concurrent
	// mergeWorkersCount.
	// This assumes each item is compressed into 4 bytes.
	return freeSpace / uint64(mergeWorkersCount) / 4
}

var mergeWorkersCount = func() int {
	return runtime.GOMAXPROCS(-1)
}()

func openParts(path string) ([]*partWrapper, error) {
	// The path can be missing after restoring from backup, so create it if needed.
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, err
	}
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
	fs.MustRemoveAll(txnDir)
	if err := fs.MkdirAllFailIfExist(txnDir); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", txnDir, err)
	}

	tmpDir := path + "/tmp"
	fs.MustRemoveAll(tmpDir)
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
	tb.flushRawItems(true)

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
			switch fn {
			case convertToV1280FileName:
				srcPath := srcDir + "/" + fn
				dstPath := dstDir + "/" + fn
				if err := os.Link(srcPath, dstPath); err != nil {
					return fmt.Errorf("cannot hard link from %q to %q: %w", srcPath, dstPath, err)
				}
			default:
				// Skip other non-directories.
			}
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
		return fmt.Errorf("cannot open %q: %w", txnDir, err)
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
	// The transaction must be run under read lock in order to provide
	// consistent snapshots with Table.CreateSnapshot().
	txnLock.RLock()
	defer txnLock.RUnlock()

	data, err := ioutil.ReadFile(txnPath)
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
	var removeWG sync.WaitGroup
	for _, path := range rmPaths {
		path, err := validatePath(pathPrefix, path)
		if err != nil {
			return fmt.Errorf("invalid path to remove: %w", err)
		}
		removeWG.Add(1)
		fs.MustRemoveAllWithDoneCallback(path, removeWG.Done)
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
		// Remove the transaction file only after all the source paths are deleted.
		// This is required for NFS mounts. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
		removeWG.Wait()
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
// The returned parts will contain less than maxItems items.
func getPartsToMerge(pws []*partWrapper, maxItems uint64, isFinal bool) []*partWrapper {
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
			dst = appendPartsToMerge(dst[:0], pwsRemaining, maxPartsToMerge, maxItems)
			maxPartsToMerge--
		}
	} else {
		dst = appendPartsToMerge(dst[:0], pwsRemaining, maxPartsToMerge, maxItems)
	}
	for _, pw := range dst {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge is already set")
		}
		pw.isInMerge = true
	}
	return dst
}

// appendPartsToMerge finds optimal parts to merge from src, appends
// them to dst and returns the result.
func appendPartsToMerge(dst, src []*partWrapper, maxPartsToMerge int, maxItems uint64) []*partWrapper {
	if len(src) < 2 {
		// There is no need in merging zero or one part :)
		return dst
	}
	if maxPartsToMerge < 2 {
		logger.Panicf("BUG: maxPartsToMerge cannot be smaller than 2; got %d", maxPartsToMerge)
	}

	// Filter out too big parts.
	// This should reduce N for O(n^2) algorithm below.
	maxInPartItems := maxItems / 2
	tmp := make([]*partWrapper, 0, len(src))
	for _, pw := range src {
		if pw.p.ph.itemsCount > maxInPartItems {
			continue
		}
		tmp = append(tmp, pw)
	}
	src = tmp

	// Sort src parts by itemsCount.
	sort.Slice(src, func(i, j int) bool { return src[i].p.ph.itemsCount < src[j].p.ph.itemsCount })

	n := maxPartsToMerge
	if len(src) < n {
		n = len(src)
	}

	// Exhaustive search for parts giving the lowest write amplification
	// when merged.
	var pws []*partWrapper
	maxM := float64(0)
	for i := 2; i <= n; i++ {
		for j := 0; j <= len(src)-i; j++ {
			itemsSum := uint64(0)
			a := src[j : j+i]
			for _, pw := range a {
				itemsSum += pw.p.ph.itemsCount
			}
			if itemsSum > maxItems {
				// There is no sense in checking the remaining bigger parts.
				break
			}
			m := float64(itemsSum) / float64(a[len(a)-1].p.ph.itemsCount)
			if m < maxM {
				continue
			}
			maxM = m
			pws = a
		}
	}

	minM := float64(maxPartsToMerge) / 2
	if minM < 1.7 {
		minM = 1.7
	}
	if maxM < minM {
		// There is no sense in merging parts with too small m.
		return dst
	}
	return append(dst, pws...)
}

func removeParts(pws []*partWrapper, partsToRemove map[*partWrapper]bool) ([]*partWrapper, int) {
	removedParts := 0
	dst := pws[:0]
	for _, pw := range pws {
		if partsToRemove[pw] {
			atomic.AddUint64(&historicalDataBlockCacheRequests, pw.p.ibCache.Requests())
			atomic.AddUint64(&historicalDataBlockCacheMisses, pw.p.ibCache.Misses())
			atomic.AddUint64(&historicalIndexBlockCacheRequests, pw.p.idxbCache.Requests())
			atomic.AddUint64(&historicalIndexBlockCacheMisses, pw.p.idxbCache.Misses())
			removedParts++
			continue
		}
		dst = append(dst, pw)
	}
	return dst, removedParts
}

func isSpecialDir(name string) bool {
	// Snapshots and cache dirs aren't used anymore.
	// Keep them here for backwards compatibility.
	return name == "tmp" || name == "txn" || name == "snapshots" || name == "cache"
}
