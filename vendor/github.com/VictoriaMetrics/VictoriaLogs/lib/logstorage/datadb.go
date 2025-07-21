package logstorage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

// The maximum size of big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxBigPartSize = 1e12

// The maximum number of inmemory parts in the partition.
//
// The actual number of inmemory parts may exceed this value if in-memory mergers
// cannot keep up with the rate of creating new in-memory parts.
const maxInmemoryPartsPerPartition = 20

// Default number of parts to merge at once.
//
// This number has been obtained empirically - it gives the lowest possible overhead.
// See appendPartsToMerge tests for details.
const defaultPartsToMerge = 15

// minMergeMultiplier is the minimum multiplier for the size of the output part
// compared to the size of the maximum input part for the merge.
//
// Higher value reduces write amplification (disk write IO induced by the merge),
// while increases the number of unmerged parts.
// The 1.7 is good enough for production workloads.
const minMergeMultiplier = 1.7

// datadb represents a database with log data
type datadb struct {
	// rb is an in-memory buffer for the added rows. It is periodically converted to parts.
	//
	// This buffer amortizes the overhead needed for converting the ingested logs into searchable parts.
	rb rowsBuffer

	// mergeIdx is used for generating unique directory names for parts
	mergeIdx atomic.Uint64

	inmemoryMergesTotal  atomic.Uint64
	inmemoryActiveMerges atomic.Int64

	smallPartMergesTotal  atomic.Uint64
	smallPartActiveMerges atomic.Int64

	bigPartMergesTotal  atomic.Uint64
	bigPartActiveMerges atomic.Int64

	// pt is the partition the datadb belongs to
	pt *partition

	// path is the path to the directory with log data
	path string

	// flushInterval is interval for flushing the inmemory parts to disk
	flushInterval time.Duration

	// inmemoryParts contains a list of inmemory parts
	inmemoryParts []*partWrapper

	// smallParts contains a list of file-based small parts
	smallParts []*partWrapper

	// bigParts contains a list of file-based big parts
	bigParts []*partWrapper

	// partsLock protects parts from concurrent access
	partsLock sync.Mutex

	// wg is used for determining when background workers stop
	//
	// wg.Add() must be called under partsLock after checking whether stopCh isn't closed.
	// This should prevent from calling wg.Add() after stopCh is closed and wg.Wait() is called.
	wg sync.WaitGroup

	// stopCh is used for notifying background workers to stop
	//
	// It must be closed under partsLock in order to prevent from calling wg.Add()
	// after stopCh is closed.
	stopCh chan struct{}
}

// partWrapper is a wrapper for opened part.
type partWrapper struct {
	// refCount is the number of references to p.
	//
	// When the number of references reaches zero, then p is closed.
	refCount atomic.Int32

	// The flag, which is set when the part must be deleted after refCount reaches zero.
	mustDrop atomic.Bool

	// p is an opened part
	p *part

	// mp references inmemory part used for initializing p.
	mp *inmemoryPart

	// isInMerge is set to true if the part takes part in merge.
	isInMerge bool

	// The deadline when in-memory part must be flushed to disk.
	flushDeadline time.Time
}

func (pw *partWrapper) incRef() {
	pw.refCount.Add(1)
}

func (pw *partWrapper) decRef() {
	n := pw.refCount.Add(-1)
	if n > 0 {
		return
	}

	deletePath := ""
	if pw.mp == nil {
		if pw.mustDrop.Load() {
			deletePath = pw.p.path
		}
	} else {
		putInmemoryPart(pw.mp)
		pw.mp = nil
	}

	mustClosePart(pw.p)
	pw.p = nil

	if deletePath != "" {
		fs.MustRemoveAll(deletePath)
	}
}

func mustCreateDatadb(path string) {
	fs.MustMkdirFailIfExist(path)
	mustWritePartNames(path, nil, nil)
}

// mustOpenDatadb opens datadb at the given path with the given flushInterval for in-memory data.
func mustOpenDatadb(pt *partition, path string, flushInterval time.Duration) *datadb {
	partNames := mustReadPartNames(path)
	mustRemoveUnusedDirs(path, partNames)

	var smallParts []*partWrapper
	var bigParts []*partWrapper
	for _, partName := range partNames {
		// Make sure the partName exists on disk.
		// If it is missing, then manual action from the user is needed,
		// since this is unexpected state, which cannot occur under normal operation,
		// including unclean shutdown.
		partPath := filepath.Join(path, partName)
		if !fs.IsPathExist(partPath) {
			partsFile := filepath.Join(path, partsFilename)
			logger.Panicf("FATAL: part %q is listed in %q, but is missing on disk; "+
				"ensure %q contents is not corrupted; remove %q to rebuild its content from the list of existing parts",
				partPath, partsFile, partsFile, partsFile)
		}

		p := mustOpenFilePart(pt, partPath)
		pw := newPartWrapper(p, nil, time.Time{})
		if p.ph.CompressedSizeBytes > getMaxInmemoryPartSize() {
			bigParts = append(bigParts, pw)
		} else {
			smallParts = append(smallParts, pw)
		}
	}

	ddb := &datadb{
		pt:            pt,
		flushInterval: flushInterval,
		path:          path,
		smallParts:    smallParts,
		bigParts:      bigParts,
		stopCh:        make(chan struct{}),
	}
	ddb.rb.init(&ddb.wg, ddb.mustFlushLogRows)
	ddb.mergeIdx.Store(uint64(time.Now().UnixNano()))

	ddb.startBackgroundWorkers()

	return ddb
}

func (ddb *datadb) startBackgroundWorkers() {
	// Start file parts mergers, so they could start merging unmerged parts if needed.
	// There is no need in starting in-memory parts mergers, since there are no in-memory parts yet.
	ddb.startSmallPartsMergers()
	ddb.startBigPartsMergers()

	ddb.startInmemoryPartsFlusher()
}

var (
	inmemoryPartsConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())
	smallPartsConcurrencyCh    = make(chan struct{}, cgroup.AvailableCPUs())
	bigPartsConcurrencyCh      = make(chan struct{}, cgroup.AvailableCPUs())
)

func (ddb *datadb) startSmallPartsMergers() {
	ddb.partsLock.Lock()
	for i := 0; i < cap(smallPartsConcurrencyCh); i++ {
		ddb.startSmallPartsMergerLocked()
	}
	ddb.partsLock.Unlock()
}

func (ddb *datadb) startBigPartsMergers() {
	ddb.partsLock.Lock()
	for i := 0; i < cap(bigPartsConcurrencyCh); i++ {
		ddb.startBigPartsMergerLocked()
	}
	ddb.partsLock.Unlock()
}

func (ddb *datadb) startInmemoryPartsMergerLocked() {
	if needStop(ddb.stopCh) {
		return
	}
	ddb.wg.Add(1)
	go func() {
		ddb.inmemoryPartsMerger()
		ddb.wg.Done()
	}()
}

func (ddb *datadb) startSmallPartsMergerLocked() {
	if needStop(ddb.stopCh) {
		return
	}
	ddb.wg.Add(1)
	go func() {
		ddb.smallPartsMerger()
		ddb.wg.Done()
	}()
}

func (ddb *datadb) startBigPartsMergerLocked() {
	if needStop(ddb.stopCh) {
		return
	}
	ddb.wg.Add(1)
	go func() {
		ddb.bigPartsMerger()
		ddb.wg.Done()
	}()
}

func (ddb *datadb) startInmemoryPartsFlusher() {
	ddb.wg.Add(1)
	go func() {
		ddb.inmemoryPartsFlusher()
		ddb.wg.Done()
	}()
}

func (ddb *datadb) inmemoryPartsFlusher() {
	// Do not add jitter to d in order to guarantee the flush interval
	ticker := time.NewTicker(ddb.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ddb.stopCh:
			return
		case <-ticker.C:
			ddb.mustFlushInmemoryPartsToFiles(false)
		}
	}
}

func (ddb *datadb) mustFlushInmemoryPartsToFiles(isFinal bool) {
	currentTime := time.Now()
	var pws []*partWrapper

	ddb.partsLock.Lock()
	for _, pw := range ddb.inmemoryParts {
		if !pw.isInMerge && (isFinal || pw.flushDeadline.Before(currentTime)) {
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	ddb.partsLock.Unlock()

	ddb.mustMergePartsToFiles(pws)
}

func (ddb *datadb) mustMergePartsToFiles(pws []*partWrapper) {
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

			ddb.mustMergeParts(pwsChunk, true)
		}(pwsToMerge)
		pws = pwsRemaining
	}
	wg.Wait()
	putWaitGroup(wg)
}

// getPartsForOptimalMerge returns parts from pws for optimal merge, plus the remaining parts.
//
// the pws items are replaced by nil after the call. This is needed for helping Go GC to reclaim the referenced items.
func getPartsForOptimalMerge(pws []*partWrapper) ([]*partWrapper, []*partWrapper) {
	pwsToMerge := appendPartsToMerge(nil, pws, math.MaxUint64)
	if len(pwsToMerge) == 0 {
		return pws, nil
	}

	m := partsToMap(pwsToMerge)
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

func (ddb *datadb) inmemoryPartsMerger() {
	for {
		if needStop(ddb.stopCh) {
			return
		}
		maxOutBytes := ddb.getMaxBigPartSize()

		ddb.partsLock.Lock()
		pws := getPartsToMergeLocked(ddb.inmemoryParts, maxOutBytes)
		ddb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		inmemoryPartsConcurrencyCh <- struct{}{}
		ddb.mustMergeParts(pws, false)
		<-inmemoryPartsConcurrencyCh
	}
}

func (ddb *datadb) smallPartsMerger() {
	for {
		if needStop(ddb.stopCh) {
			return
		}
		maxOutBytes := ddb.getMaxBigPartSize()

		ddb.partsLock.Lock()
		pws := getPartsToMergeLocked(ddb.smallParts, maxOutBytes)
		ddb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		smallPartsConcurrencyCh <- struct{}{}
		ddb.mustMergeParts(pws, false)
		<-smallPartsConcurrencyCh
	}
}

func (ddb *datadb) bigPartsMerger() {
	for {
		if needStop(ddb.stopCh) {
			return
		}
		maxOutBytes := ddb.getMaxBigPartSize()

		ddb.partsLock.Lock()
		pws := getPartsToMergeLocked(ddb.bigParts, maxOutBytes)
		ddb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		bigPartsConcurrencyCh <- struct{}{}
		ddb.mustMergeParts(pws, false)
		<-bigPartsConcurrencyCh
	}
}

// getPartsToMergeLocked returns optimal parts to merge from pws.
//
// The summary size of the returned parts must be smaller than maxOutBytes.
func getPartsToMergeLocked(pws []*partWrapper, maxOutBytes uint64) []*partWrapper {
	pwsRemaining := make([]*partWrapper, 0, len(pws))
	for _, pw := range pws {
		if !pw.isInMerge {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}

	pwsToMerge := appendPartsToMerge(nil, pwsRemaining, maxOutBytes)

	for _, pw := range pwsToMerge {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge cannot be set")
		}
		pw.isInMerge = true
	}

	return pwsToMerge
}

func assertIsInMerge(pws []*partWrapper) {
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge unexpectedly set to false")
		}
	}
}

// mustMergeParts merges pws to a single resulting part.
//
// if isFinal is set, then the resulting part is guaranteed to be saved to disk.
// if isFinal is set, then the merge process cannot be interrupted.
// The pws may remain unmerged after returning from the function if there is no enough disk space.
//
// All the parts inside pws must have isInMerge field set to true.
// The isInMerge field inside pws parts is set to false before returning from the function.
func (ddb *datadb) mustMergeParts(pws []*partWrapper, isFinal bool) {
	if len(pws) == 0 {
		// Nothing to merge.
		return
	}

	assertIsInMerge(pws)
	defer ddb.releasePartsToMerge(pws)

	startTime := time.Now()

	dstPartType := ddb.getDstPartType(pws, isFinal)
	if dstPartType != partInmemory {
		// Make sure there is enough disk space for performing the merge
		partsSize := getCompressedSize(pws)
		needReleaseDiskSpace := tryReserveDiskSpace(ddb.path, partsSize)
		if needReleaseDiskSpace {
			defer releaseDiskSpace(partsSize)
		} else {
			if !isFinal {
				// There is no enough disk space for performing the non-final merge.
				return
			}
			// Try performing final merge even if there is no enough disk space
			// in order to persist in-memory data to disk.
			// It is better to crash on out of memory error in this case.
		}
	}

	switch dstPartType {
	case partInmemory:
		ddb.inmemoryMergesTotal.Add(1)
		ddb.inmemoryActiveMerges.Add(1)
		defer ddb.inmemoryActiveMerges.Add(-1)
	case partSmall:
		ddb.smallPartMergesTotal.Add(1)
		ddb.smallPartActiveMerges.Add(1)
		defer ddb.smallPartActiveMerges.Add(-1)
	case partBig:
		ddb.bigPartMergesTotal.Add(1)
		ddb.bigPartActiveMerges.Add(1)
		defer ddb.bigPartActiveMerges.Add(-1)
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}

	// Initialize destination paths.
	mergeIdx := ddb.nextMergeIdx()
	dstPartPath := ddb.getDstPartPath(dstPartType, mergeIdx)

	if isFinal && len(pws) == 1 && pws[0].mp != nil {
		// Fast path: flush a single in-memory part to disk.
		mp := pws[0].mp
		mp.MustStoreToDisk(dstPartPath)
		pwNew := ddb.openCreatedPart(&mp.ph, pws, nil, dstPartPath)
		ddb.swapSrcWithDstParts(pws, pwNew, dstPartType)
		return
	}

	// Prepare blockStreamReaders for source parts.
	bsrs := mustOpenBlockStreamReaders(pws)

	// Prepare BlockStreamWriter for destination part.
	srcSize := uint64(0)
	srcRowsCount := uint64(0)
	srcBlocksCount := uint64(0)
	for _, pw := range pws {
		ph := &pw.p.ph
		srcSize += ph.CompressedSizeBytes
		srcRowsCount += ph.RowsCount
		srcBlocksCount += ph.BlocksCount
	}
	bsw := getBlockStreamWriter()
	var mpNew *inmemoryPart
	if dstPartType == partInmemory {
		mpNew = getInmemoryPart()
		bsw.MustInitForInmemoryPart(mpNew)
	} else {
		nocache := dstPartType == partBig
		bsw.MustInitForFilePart(dstPartPath, nocache)
	}

	// Merge source parts to destination part.
	var ph partHeader
	stopCh := ddb.stopCh
	if isFinal {
		// The final merge shouldn't be stopped even if ddb.stopCh is closed.
		stopCh = nil
	}
	mustMergeBlockStreams(&ph, bsw, bsrs, stopCh)
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}

	// Persist partHeader for destination part after the merge.
	if mpNew != nil {
		mpNew.ph = ph
	} else {
		ph.mustWriteMetadata(dstPartPath)
		// Make sure the created part directory listing is synced.
		fs.MustSyncPath(dstPartPath)
	}
	if needStop(stopCh) {
		// Remove incomplete destination part
		if dstPartType != partInmemory {
			fs.MustRemoveAll(dstPartPath)
		}
		return
	}

	// Atomically swap the source parts with the newly created part.
	pwNew := ddb.openCreatedPart(&ph, pws, mpNew, dstPartPath)

	dstSize := uint64(0)
	dstRowsCount := uint64(0)
	dstBlocksCount := uint64(0)
	if pwNew != nil {
		pDst := pwNew.p
		dstSize = pDst.ph.CompressedSizeBytes
		dstRowsCount = pDst.ph.RowsCount
		dstBlocksCount = pDst.ph.BlocksCount
	}

	ddb.swapSrcWithDstParts(pws, pwNew, dstPartType)

	d := time.Since(startTime)
	if d <= time.Minute {
		return
	}

	// Log stats for long merges.
	durationSecs := d.Seconds()
	rowsPerSec := int(float64(srcRowsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d rows, %d blocks, %d bytes) into (1 part, %d rows, %d blocks, %d bytes) in %.3f seconds at %d rows/sec to %q",
		len(pws), srcRowsCount, srcBlocksCount, srcSize, dstRowsCount, dstBlocksCount, dstSize, durationSecs, rowsPerSec, dstPartPath)
}

func (ddb *datadb) nextMergeIdx() uint64 {
	return ddb.mergeIdx.Add(1)
}

type partType int

var (
	partInmemory = partType(0)
	partSmall    = partType(1)
	partBig      = partType(2)
)

func (ddb *datadb) getDstPartType(pws []*partWrapper, isFinal bool) partType {
	dstPartSize := getCompressedSize(pws)
	if dstPartSize > ddb.getMaxSmallPartSize() {
		return partBig
	}
	if isFinal || dstPartSize > getMaxInmemoryPartSize() {
		return partSmall
	}
	if !areAllInmemoryParts(pws) {
		// If at least a single source part is located in file,
		// then the destination part must be in file for durability reasons.
		return partSmall
	}
	return partInmemory
}

func (ddb *datadb) getDstPartPath(dstPartType partType, mergeIdx uint64) string {
	ptPath := ddb.path
	dstPartPath := ""
	if dstPartType != partInmemory {
		dstPartPath = filepath.Join(ptPath, fmt.Sprintf("%016X", mergeIdx))
	}
	return dstPartPath
}

func (ddb *datadb) openCreatedPart(ph *partHeader, pws []*partWrapper, mpNew *inmemoryPart, dstPartPath string) *partWrapper {
	// Open the created part.
	if ph.RowsCount == 0 {
		// The created part is empty. Remove it
		if mpNew == nil {
			fs.MustRemoveAll(dstPartPath)
		}
		return nil
	}
	var p *part
	var flushDeadline time.Time
	if mpNew != nil {
		// Open the created part from memory.
		p = mustOpenInmemoryPart(ddb.pt, mpNew)
		flushDeadline = ddb.getFlushToDiskDeadline(pws)
	} else {
		// Open the created part from disk.
		p = mustOpenFilePart(ddb.pt, dstPartPath)
	}
	return newPartWrapper(p, mpNew, flushDeadline)
}

func (ddb *datadb) mustAddRows(lr *LogRows) {
	ddb.rb.mustAddRows(lr)
}

type rowsBuffer struct {
	shards  []rowsBufferShard
	nextIdx atomic.Uint64
}

func (rb *rowsBuffer) init(wg *sync.WaitGroup, flushFunc func(lr *logRows)) {
	shards := make([]rowsBufferShard, cgroup.AvailableCPUs())
	for i := range shards {
		shard := &shards[i]
		shard.wg = wg
		shard.flushFunc = flushFunc
	}
	rb.shards = shards
}

type rowsBufferShard struct {
	wg        *sync.WaitGroup
	flushFunc func(lr *logRows)

	mu         sync.Mutex
	lr         *logRows
	flushTimer *time.Timer

	// padding for preventing false sharing
	_ [atomicutil.CacheLineSize]byte
}

func (rb *rowsBuffer) flush() {
	shards := rb.shards
	for i := range shards {
		shard := &shards[i]
		shard.mu.Lock()
		shard.flushLocked()
		shard.mu.Unlock()
	}
}

func (rb *rowsBuffer) mustAddRows(lr *LogRows) {
	if len(lr.streamIDs) == 0 {
		return
	}

	shards := rb.shards
	idx := rb.nextIdx.Add(1) % uint64(len(shards))
	shard := &shards[idx]

	shard.mu.Lock()
	if shard.flushTimer == nil {
		shard.wg.Add(1)
		shard.flushTimer = time.AfterFunc(time.Second, func() {
			defer shard.wg.Done()

			shard.mu.Lock()
			shard.flushLocked()
			shard.mu.Unlock()
		})
	}
	if shard.lr == nil {
		shard.lr = getLogRows()
	}
	shard.lr.mustAddRows(lr)
	if shard.lr.needFlush() {
		shard.flushLocked()
	}
	shard.mu.Unlock()
}

func (shard *rowsBufferShard) flushLocked() {
	if shard.flushTimer != nil {
		if shard.flushTimer.Stop() {
			shard.wg.Done()
		}
		shard.flushTimer = nil
	}

	if shard.lr != nil {
		shard.flushFunc(shard.lr)
		putLogRows(shard.lr)
		shard.lr = nil
	}
}

func (ddb *datadb) mustFlushLogRows(lr *logRows) {
	inmemoryPartsConcurrencyCh <- struct{}{}
	mp := getInmemoryPart()
	mp.mustInitFromRows(lr)
	p := mustOpenInmemoryPart(ddb.pt, mp)
	<-inmemoryPartsConcurrencyCh

	flushDeadline := time.Now().Add(ddb.flushInterval)
	pw := newPartWrapper(p, mp, flushDeadline)

	ddb.partsLock.Lock()
	ddb.inmemoryParts = append(ddb.inmemoryParts, pw)
	ddb.startInmemoryPartsMergerLocked()
	ddb.partsLock.Unlock()
}

// DatadbStats contains various stats for datadb.
type DatadbStats struct {
	// InmemoryMergesTotal is the number of inmemory merges performed in the given datadb.
	InmemoryMergesTotal uint64

	// InmemoryActiveMerges is the number of currently active inmemory merges performed by the given datadb.
	InmemoryActiveMerges uint64

	// SmallPartMergesTotal is the number of small file merges performed in the given datadb.
	SmallPartMergesTotal uint64

	// SmallPartActiveMerges is the number of currently active small file merges performed by the given datadb.
	SmallPartActiveMerges uint64

	// BigPartMergesTotal is the number of big file merges performed in the given datadb.
	BigPartMergesTotal uint64

	// BigPartActiveMerges is the number of currently active big file merges performed by the given datadb.
	BigPartActiveMerges uint64

	// InmemoryRowsCount is the number of rows, which weren't flushed to disk yet.
	InmemoryRowsCount uint64

	// SmallPartRowsCount is the number of rows stored on disk in small parts.
	SmallPartRowsCount uint64

	// BigPartRowsCount is the number of rows stored on disk in big parts.
	BigPartRowsCount uint64

	// InmemoryParts is the number of in-memory parts, which weren't flushed to disk yet.
	InmemoryParts uint64

	// SmallParts is the number of file-based small parts stored on disk.
	SmallParts uint64

	// BigParts is the number of file-based big parts stored on disk.
	BigParts uint64

	// InmemoryBlocks is the number of in-memory blocks, which weren't flushed to disk yet.
	InmemoryBlocks uint64

	// SmallPartBlocks is the number of file-based small blocks stored on disk.
	SmallPartBlocks uint64

	// BigPartBlocks is the number of file-based big blocks stored on disk.
	BigPartBlocks uint64

	// CompressedInmemorySize is the size of compressed data stored in memory.
	CompressedInmemorySize uint64

	// CompressedSmallPartSize is the size of compressed small parts data stored on disk.
	CompressedSmallPartSize uint64

	// CompressedBigPartSize is the size of compressed big data stored on disk.
	CompressedBigPartSize uint64

	// UncompressedInmemorySize is the size of uncompressed data stored in memory.
	UncompressedInmemorySize uint64

	// UncompressedSmallPartSize is the size of uncompressed small data stored on disk.
	UncompressedSmallPartSize uint64

	// UncompressedBigPartSize is the size of uncompressed big data stored on disk.
	UncompressedBigPartSize uint64
}

func (s *DatadbStats) reset() {
	*s = DatadbStats{}
}

// RowsCount returns the number of rows stored in datadb.
func (s *DatadbStats) RowsCount() uint64 {
	return s.InmemoryRowsCount + s.SmallPartRowsCount + s.BigPartRowsCount
}

// updateStats updates s with ddb stats.
func (ddb *datadb) updateStats(s *DatadbStats) {
	s.InmemoryMergesTotal += ddb.inmemoryMergesTotal.Load()
	s.InmemoryActiveMerges += uint64(ddb.inmemoryActiveMerges.Load())
	s.SmallPartMergesTotal += ddb.smallPartMergesTotal.Load()
	s.SmallPartActiveMerges += uint64(ddb.smallPartActiveMerges.Load())
	s.BigPartMergesTotal += ddb.bigPartMergesTotal.Load()
	s.BigPartActiveMerges += uint64(ddb.bigPartActiveMerges.Load())

	ddb.partsLock.Lock()

	s.InmemoryRowsCount += getRowsCount(ddb.inmemoryParts)
	s.SmallPartRowsCount += getRowsCount(ddb.smallParts)
	s.BigPartRowsCount += getRowsCount(ddb.bigParts)

	s.InmemoryParts += uint64(len(ddb.inmemoryParts))
	s.SmallParts += uint64(len(ddb.smallParts))
	s.BigParts += uint64(len(ddb.bigParts))

	s.InmemoryBlocks += getBlocksCount(ddb.inmemoryParts)
	s.SmallPartBlocks += getBlocksCount(ddb.smallParts)
	s.BigPartBlocks += getBlocksCount(ddb.bigParts)

	s.CompressedInmemorySize += getCompressedSize(ddb.inmemoryParts)
	s.CompressedSmallPartSize += getCompressedSize(ddb.smallParts)
	s.CompressedBigPartSize += getCompressedSize(ddb.bigParts)

	s.UncompressedInmemorySize += getUncompressedSize(ddb.inmemoryParts)
	s.UncompressedSmallPartSize += getUncompressedSize(ddb.smallParts)
	s.UncompressedBigPartSize += getUncompressedSize(ddb.bigParts)

	ddb.partsLock.Unlock()
}

// debugFlush() makes sure that the recently ingested data is available for search.
func (ddb *datadb) debugFlush() {
	ddb.rb.flush()
}

func (ddb *datadb) swapSrcWithDstParts(pws []*partWrapper, pwNew *partWrapper, dstPartType partType) {
	// Atomically unregister old parts and add new part to pt.
	partsToRemove := partsToMap(pws)

	removedInmemoryParts := 0
	removedSmallParts := 0
	removedBigParts := 0

	ddb.partsLock.Lock()

	ddb.inmemoryParts, removedInmemoryParts = removeParts(ddb.inmemoryParts, partsToRemove)
	ddb.smallParts, removedSmallParts = removeParts(ddb.smallParts, partsToRemove)
	ddb.bigParts, removedBigParts = removeParts(ddb.bigParts, partsToRemove)

	if pwNew != nil {
		switch dstPartType {
		case partInmemory:
			ddb.inmemoryParts = append(ddb.inmemoryParts, pwNew)
			ddb.startInmemoryPartsMergerLocked()
		case partSmall:
			ddb.smallParts = append(ddb.smallParts, pwNew)
			ddb.startSmallPartsMergerLocked()
		case partBig:
			ddb.bigParts = append(ddb.bigParts, pwNew)
			ddb.startBigPartsMergerLocked()
		default:
			logger.Panicf("BUG: unknown partType=%d", dstPartType)
		}
	}

	// Atomically store the updated list of file-based parts on disk.
	// This must be performed under partsLock in order to prevent from races
	// when multiple concurrently running goroutines update the list.
	if removedSmallParts > 0 || removedBigParts > 0 || pwNew != nil && dstPartType != partInmemory {
		smallPartNames := getPartNames(ddb.smallParts)
		bigPartNames := getPartNames(ddb.bigParts)
		mustWritePartNames(ddb.path, smallPartNames, bigPartNames)
	}

	ddb.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedSmallParts + removedBigParts
	if removedParts != len(partsToRemove) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(partsToRemove))
	}

	// Mark old parts as must be deleted and decrement reference count, so they are eventually closed and deleted.
	for _, pw := range pws {
		pw.mustDrop.Store(true)
		pw.decRef()
	}
}

func partsToMap(pws []*partWrapper) map[*partWrapper]struct{} {
	m := make(map[*partWrapper]struct{}, len(pws))
	for _, pw := range pws {
		m[pw] = struct{}{}
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found out of %d parts", len(pws)-len(m), len(pws))
	}
	return m
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

func newPartWrapper(p *part, mp *inmemoryPart, flushDeadline time.Time) *partWrapper {
	pw := &partWrapper{
		p:  p,
		mp: mp,

		flushDeadline: flushDeadline,
	}

	// Increase reference counter for newly created part - it is decreased when the part
	// is removed from the list of open parts.
	pw.incRef()

	return pw
}

func (ddb *datadb) getFlushToDiskDeadline(pws []*partWrapper) time.Time {
	d := time.Now().Add(ddb.flushInterval)
	for _, pw := range pws {
		if pw.mp != nil && pw.flushDeadline.Before(d) {
			d = pw.flushDeadline
		}
	}
	return d
}

func getMaxInmemoryPartSize() uint64 {
	// Allocate 10% of allowed memory for in-memory parts.
	n := uint64(0.1 * float64(memory.Allowed()) / maxInmemoryPartsPerPartition)
	if n < 1e6 {
		n = 1e6
	}
	return n
}

func areAllInmemoryParts(pws []*partWrapper) bool {
	for _, pw := range pws {
		if pw.mp == nil {
			return false
		}
	}
	return true
}

func (ddb *datadb) releasePartsToMerge(pws []*partWrapper) {
	ddb.partsLock.Lock()
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: missing isInMerge flag on the part %q", pw.p.path)
		}
		pw.isInMerge = false
	}
	ddb.partsLock.Unlock()
}

func (ddb *datadb) getMaxBigPartSize() uint64 {
	return getMaxOutBytes(ddb.path)
}

func (ddb *datadb) getMaxSmallPartSize() uint64 {
	// Small parts are cached in the OS page cache,
	// so limit their size by the remaining free RAM.
	mem := memory.Remaining()
	n := uint64(mem) / defaultPartsToMerge
	if n < 10e6 {
		n = 10e6
	}
	// Make sure the output part fits available disk space for small parts.
	sizeLimit := getMaxOutBytes(ddb.path)
	if n > sizeLimit {
		n = sizeLimit
	}
	return n
}

func getMaxOutBytes(path string) uint64 {
	n := availableDiskSpace(path)
	if n > maxBigPartSize {
		n = maxBigPartSize
	}
	return n
}

func availableDiskSpace(path string) uint64 {
	available := fs.MustGetFreeSpace(path)
	reserved := reservedDiskSpace.Load()
	if available < reserved {
		return 0
	}
	return available - reserved
}

func tryReserveDiskSpace(path string, n uint64) bool {
	available := fs.MustGetFreeSpace(path)
	reserved := reserveDiskSpace(n)
	if available >= reserved {
		return true
	}
	releaseDiskSpace(n)
	return false
}

func reserveDiskSpace(n uint64) uint64 {
	return reservedDiskSpace.Add(n)
}

func releaseDiskSpace(n uint64) {
	reservedDiskSpace.Add(^(n - 1))
}

// reservedDiskSpace tracks global reserved disk space for currently executed
// background merges across all the partitions.
//
// It should allow avoiding background merges when there is no free disk space.
var reservedDiskSpace atomicutil.Uint64

func needStop(stopCh <-chan struct{}) bool {
	select {
	case <-stopCh:
		return true
	default:
		return false
	}
}

// mustCloseDatadb can be called only when nobody accesses ddb.
func mustCloseDatadb(ddb *datadb) {
	// Flush ddb.rb for the last time
	ddb.rb.flush()

	// Notify background workers to stop.
	// Make it under ddb.partsLock in order to prevent from calling ddb.wg.Add()
	// after ddb.stopCh is closed and ddb.wg.Wait() is called.
	ddb.partsLock.Lock()
	close(ddb.stopCh)
	ddb.partsLock.Unlock()

	// Wait for background workers to stop.
	ddb.wg.Wait()

	// flush in-memory data to disk
	ddb.mustFlushInmemoryPartsToFiles(true)
	if len(ddb.inmemoryParts) > 0 {
		logger.Panicf("BUG: the number of in-memory parts must be zero after flushing them to disk; got %d", len(ddb.inmemoryParts))
	}
	ddb.inmemoryParts = nil

	// close small parts
	for _, pw := range ddb.smallParts {
		pw.decRef()
		if n := pw.refCount.Load(); n != 0 {
			logger.Panicf("BUG: there are %d references to smallPart", n)
		}
	}
	ddb.smallParts = nil

	// close big parts
	for _, pw := range ddb.bigParts {
		pw.decRef()
		if n := pw.refCount.Load(); n != 0 {
			logger.Panicf("BUG: there are %d references to bigPart", n)
		}
	}
	ddb.bigParts = nil

	ddb.path = ""
	ddb.pt = nil
}

func getPartNames(pws []*partWrapper) []string {
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
	return partNames
}

func mustWritePartNames(path string, smallPartNames, bigPartNames []string) {
	partNames := append([]string{}, smallPartNames...)
	partNames = append(partNames, bigPartNames...)
	data, err := json.Marshal(partNames)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partNames to JSON: %s", err)
	}
	partNamesPath := filepath.Join(path, partsFilename)
	fs.MustWriteAtomic(partNamesPath, data, true)
}

func mustReadPartNames(path string) []string {
	partNamesPath := filepath.Join(path, partsFilename)
	data, err := os.ReadFile(partNamesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// The parts.json file is missing. This can happen if VictoriaLogs shuts down uncleanly
			// (via OOM crash, a panic, SIGKILL or hardware shutdown) in the middle of creating
			// new per-day partition inside the mustCreatePartition() function.
			// Check if there are any part directories in the datadb directory.
			des := fs.MustReadDir(path)
			var partDirs []string
			for _, de := range des {
				if !fs.IsDirOrSymlink(de) {
					continue
				}
				partDirs = append(partDirs, de.Name())
			}

			if len(partDirs) == 0 {
				logger.Warnf("creating missing %s with empty parts list, since no part directories found in %s", partNamesPath, path)
				mustWritePartNames(path, nil, nil)
				return []string{}
			}

			// Parts exist but parts.json is missing - this is an unexpected state that requires manual intervention
			logger.Panicf("FATAL: cannot read %s: %s; found part directories %v in %s. "+
				"This indicates corruption. Manually remove the %s partition directory to resolve the corruption (the partition data will be lost)",
				partNamesPath, err, partDirs, path, path)
		}
		logger.Panicf("FATAL: cannot read %s: %s", partNamesPath, err)
	}
	var partNames []string
	if err := json.Unmarshal(data, &partNames); err != nil {
		logger.Panicf("FATAL: cannot parse %s: %s", partNamesPath, err)
	}
	return partNames
}

// mustRemoveUnusedDirs removes dirs at path, which are missing in partNames.
//
// These dirs may be left after unclean shutdown.
func mustRemoveUnusedDirs(path string, partNames []string) {
	des := fs.MustReadDir(path)
	m := make(map[string]struct{}, len(partNames))
	for _, partName := range partNames {
		m[partName] = struct{}{}
	}
	removedDirs := 0
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories.
			continue
		}
		fn := de.Name()
		if _, ok := m[fn]; !ok {
			deletePath := filepath.Join(path, fn)
			fs.MustRemoveAll(deletePath)
			removedDirs++
		}
	}
	if removedDirs > 0 {
		fs.MustSyncPath(path)
	}
}

// appendPartsToMerge finds optimal parts to merge from src,
// appends them to dst and returns the result.
func appendPartsToMerge(dst, src []*partWrapper, maxOutBytes uint64) []*partWrapper {
	if len(src) < 2 {
		// There is no need in merging zero or one part :)
		return dst
	}

	// Filter out too big parts.
	// This should reduce N for O(N^2) algorithm below.
	maxInPartBytes := uint64(float64(maxOutBytes) / minMergeMultiplier)
	tmp := make([]*partWrapper, 0, len(src))
	for _, pw := range src {
		if pw.p.ph.CompressedSizeBytes > maxInPartBytes {
			continue
		}
		tmp = append(tmp, pw)
	}
	src = tmp

	sortPartsForOptimalMerge(src)

	maxSrcParts := defaultPartsToMerge
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
			if a[0].p.ph.CompressedSizeBytes*uint64(len(a)) < a[len(a)-1].p.ph.CompressedSizeBytes {
				// Do not merge parts with too big difference in size,
				// since this results in unbalanced merges.
				continue
			}
			outSize := getCompressedSize(a)
			if outSize > maxOutBytes {
				// There is no need in verifying remaining parts with bigger sizes.
				break
			}
			m := float64(outSize) / float64(a[len(a)-1].p.ph.CompressedSizeBytes)
			if m < maxM {
				continue
			}
			maxM = m
			pws = a
		}
	}

	minM := float64(defaultPartsToMerge) / 2
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
	// Sort src parts by size and backwards timestamp.
	// This should improve adjanced points' locality in the merged parts.
	sort.Slice(pws, func(i, j int) bool {
		a := &pws[i].p.ph
		b := &pws[j].p.ph
		if a.CompressedSizeBytes == b.CompressedSizeBytes {
			return a.MinTimestamp > b.MinTimestamp
		}
		return a.CompressedSizeBytes < b.CompressedSizeBytes
	})
}

func getCompressedSize(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.ph.CompressedSizeBytes
	}
	return n
}

func getUncompressedSize(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.ph.UncompressedSizeBytes
	}
	return n
}

func getRowsCount(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.ph.RowsCount
	}
	return n
}

func getBlocksCount(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.ph.BlocksCount
	}
	return n
}

func (ddb *datadb) mustForceMergeAllParts() {
	// Flush inmemory parts to files before forced merge
	ddb.mustFlushInmemoryPartsToFiles(true)

	var pws []*partWrapper

	// Collect all the file parts for forced merge
	ddb.partsLock.Lock()
	pws = appendAllPartsForMergeLocked(pws, ddb.smallParts)
	pws = appendAllPartsForMergeLocked(pws, ddb.bigParts)
	ddb.partsLock.Unlock()

	// If len(pws) == 1, then the merge must run anyway.
	// This allows applying the configured retention, removing the deleted data, etc.

	// Merge pws optimally
	wg := getWaitGroup()
	for len(pws) > 0 {
		pwsToMerge, pwsRemaining := getPartsForOptimalMerge(pws)
		wg.Add(1)
		bigPartsConcurrencyCh <- struct{}{}
		go func(pwsChunk []*partWrapper) {
			defer func() {
				<-bigPartsConcurrencyCh
				wg.Done()
			}()

			ddb.mustMergeParts(pwsChunk, false)
		}(pwsToMerge)
		pws = pwsRemaining
	}
	wg.Wait()
	putWaitGroup(wg)
}

func appendAllPartsForMergeLocked(dst, src []*partWrapper) []*partWrapper {
	for _, pw := range src {
		if !pw.isInMerge {
			pw.isInMerge = true
			dst = append(dst, pw)
		}
	}
	return dst
}
