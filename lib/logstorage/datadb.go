package logstorage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

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

// The maximum number of inmemory parts in the partition.
//
// If the number of inmemory parts reaches this value, then assisted merge runs during data ingestion.
const maxInmemoryPartsPerPartition = 20

// datadb represents a database with log data
type datadb struct {
	// mergeIdx is used for generating unique directory names for parts
	mergeIdx uint64

	inmemoryMergesTotal  uint64
	inmemoryActiveMerges uint64
	fileMergesTotal      uint64
	fileActiveMerges     uint64

	// pt is the partition the datadb belongs to
	pt *partition

	// path is the path to the directory with log data
	path string

	// flushInterval is interval for flushing the inmemory parts to disk
	flushInterval time.Duration

	// inmemoryParts contains a list of inmemory parts
	inmemoryParts []*partWrapper

	// fileParts contains a list of file-based parts
	fileParts []*partWrapper

	// partsLock protects parts from concurrent access
	partsLock sync.Mutex

	// wg is used for determining when background workers stop
	wg sync.WaitGroup

	// stopCh is used for notifying background workers to stop
	stopCh chan struct{}

	// mergeDoneCond is used for pace-limiting the data ingestion rate
	mergeDoneCond *sync.Cond

	// inmemoryPartsFlushersCount is the number of currently running in-memory parts flushers
	//
	// This variable must be accessed under partsLock.
	inmemoryPartsFlushersCount int

	// mergeWorkersCount is the number of currently running merge workers
	//
	// This variable must be accessed under partsLock.
	mergeWorkersCount int
}

// partWrapper is a wrapper for opened part.
type partWrapper struct {
	// refCount is the number of references to p.
	//
	// When the number of references reaches zero, then p is closed.
	refCount int32

	// The flag, which is set when the part must be deleted after refCount reaches zero.
	mustBeDeleted uint32

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
	atomic.AddInt32(&pw.refCount, 1)
}

func (pw *partWrapper) decRef() {
	n := atomic.AddInt32(&pw.refCount, -1)
	if n > 0 {
		return
	}

	deletePath := ""
	if pw.mp == nil {
		if atomic.LoadUint32(&pw.mustBeDeleted) != 0 {
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
	mustWritePartNames(path, []string{})
}

// mustOpenDatadb opens datadb at the given path with the given flushInterval for in-memory data.
func mustOpenDatadb(pt *partition, path string, flushInterval time.Duration) *datadb {
	// Remove temporary directories, which may be left after unclean shutdown.
	fs.MustRemoveTemporaryDirs(path)

	partNames := mustReadPartNames(path)
	mustRemoveUnusedDirs(path, partNames)

	pws := make([]*partWrapper, len(partNames))
	for i, partName := range partNames {
		partPath := filepath.Join(path, partName)
		p := mustOpenFilePart(pt, partPath)
		pws[i] = newPartWrapper(p, nil, time.Time{})
	}

	ddb := &datadb{
		pt:            pt,
		mergeIdx:      uint64(time.Now().UnixNano()),
		flushInterval: flushInterval,
		path:          path,
		fileParts:     pws,
		stopCh:        make(chan struct{}),
	}
	ddb.mergeDoneCond = sync.NewCond(&ddb.partsLock)

	// Start merge workers in the hope they'll merge the remaining parts
	ddb.partsLock.Lock()
	n := getMergeWorkersCount()
	for i := 0; i < n; i++ {
		ddb.startMergeWorkerLocked()
	}
	ddb.partsLock.Unlock()

	return ddb
}

// startInmemoryPartsFlusherLocked starts flusher for in-memory parts to disk.
//
// This function must be called under partsLock.
func (ddb *datadb) startInmemoryPartsFlusherLocked() {
	if ddb.inmemoryPartsFlushersCount >= 1 {
		return
	}
	ddb.inmemoryPartsFlushersCount++
	ddb.wg.Add(1)
	go func() {
		ddb.flushInmemoryParts()
		ddb.wg.Done()
	}()
}

func (ddb *datadb) flushInmemoryParts() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		ddb.partsLock.Lock()
		pws := make([]*partWrapper, 0, len(ddb.inmemoryParts))
		pws = appendNotInMergePartsLocked(pws, ddb.inmemoryParts)
		currentTime := time.Now()
		partsToFlush := pws[:0]
		for _, pw := range pws {
			if pw.flushDeadline.Before(currentTime) {
				partsToFlush = append(partsToFlush, pw)
			}
		}
		setInMergeLocked(partsToFlush)
		if len(pws) == 0 {
			ddb.inmemoryPartsFlushersCount--
		}
		ddb.partsLock.Unlock()

		if len(pws) == 0 {
			// There are no in-memory parts, so stop the flusher.
			return
		}
		ddb.mustMergePartsFinal(partsToFlush)

		select {
		case <-ddb.stopCh:
			return
		case <-ticker.C:
		}
	}
}

// startMergeWorkerLocked starts a merge worker.
//
// This function must be called under locked partsLock.
func (ddb *datadb) startMergeWorkerLocked() {
	if ddb.mergeWorkersCount >= getMergeWorkersCount() {
		return
	}
	ddb.mergeWorkersCount++
	ddb.wg.Add(1)
	go func() {
		globalMergeLimitCh <- struct{}{}
		ddb.mustMergeExistingParts()
		<-globalMergeLimitCh
		ddb.wg.Done()
	}()
}

// globalMergeLimitCh limits the number of concurrent merges across all the partitions
var globalMergeLimitCh = make(chan struct{}, getMergeWorkersCount())

func getMergeWorkersCount() int {
	n := cgroup.AvailableCPUs()
	if n < 4 {
		// Use bigger number of workers on systems with small number of CPU cores,
		// since a single worker may become busy for long time when merging big parts.
		// Then the remaining workers may continue performing merges
		// for newly added small parts.
		return 4
	}
	return n
}

func (ddb *datadb) mustMergeExistingParts() {
	for !needStop(ddb.stopCh) {
		maxOutBytes := ddb.availableDiskSpace()

		ddb.partsLock.Lock()
		parts := make([]*partWrapper, 0, len(ddb.inmemoryParts)+len(ddb.fileParts))
		parts = appendNotInMergePartsLocked(parts, ddb.inmemoryParts)
		parts = appendNotInMergePartsLocked(parts, ddb.fileParts)
		pws := appendPartsToMerge(nil, parts, maxOutBytes)
		setInMergeLocked(pws)
		if len(pws) == 0 {
			ddb.mergeWorkersCount--
		}
		ddb.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge at the moment.
			return
		}

		partsSize := getCompressedSize(pws)
		if !ddb.reserveDiskSpace(partsSize) {
			// There is no free disk space for the merge,
			// because concurrent merge workers already reserved the disk space.
			// Try again with smaller maxOutBytes.
			ddb.releasePartsToMerge(pws)
			continue
		}
		ddb.mustMergeParts(pws, false)
		ddb.releaseDiskSpace(partsSize)
	}
}

// appendNotInMergePartsLocked appends src parts with isInMerge=false to dst and returns the result.
//
// This function must be called under partsLock.
func appendNotInMergePartsLocked(dst, src []*partWrapper) []*partWrapper {
	for _, pw := range src {
		if !pw.isInMerge {
			dst = append(dst, pw)
		}
	}
	return dst
}

// setInMergeLocked sets isInMerge flag for pws.
//
// This function must be called under partsLock.
func setInMergeLocked(pws []*partWrapper) {
	for _, pw := range pws {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge unexpectedly set to true")
		}
		pw.isInMerge = true
	}
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
// if isFinal is set, then the resulting part will be saved to disk.
//
// All the parts inside pws must have isInMerge field set to true.
func (ddb *datadb) mustMergeParts(pws []*partWrapper, isFinal bool) {
	if len(pws) == 0 {
		// Nothing to merge.
		return
	}
	assertIsInMerge(pws)

	startTime := time.Now()

	dstPartType := ddb.getDstPartType(pws, isFinal)
	if dstPartType == partInmemory {
		atomic.AddUint64(&ddb.inmemoryMergesTotal, 1)
		atomic.AddUint64(&ddb.inmemoryActiveMerges, 1)
		defer atomic.AddUint64(&ddb.inmemoryActiveMerges, ^uint64(0))
	} else {
		atomic.AddUint64(&ddb.fileMergesTotal, 1)
		atomic.AddUint64(&ddb.fileActiveMerges, 1)
		defer atomic.AddUint64(&ddb.fileActiveMerges, ^uint64(0))
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
		srcSize += pw.p.ph.CompressedSizeBytes
		srcRowsCount += pw.p.ph.RowsCount
		srcBlocksCount += pw.p.ph.BlocksCount
	}
	bsw := getBlockStreamWriter()
	var mpNew *inmemoryPart
	if dstPartType == partInmemory {
		mpNew = getInmemoryPart()
		bsw.MustInitForInmemoryPart(mpNew)
	} else {
		nocache := !shouldUsePageCacheForPartSize(srcSize)
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
		ddb.releasePartsToMerge(pws)
		ddb.mergeDoneCond.Broadcast()
		// Remove incomplete destination part
		if dstPartType == partFile {
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
	if d <= 30*time.Second {
		return
	}

	// Log stats for long merges.
	durationSecs := d.Seconds()
	rowsPerSec := int(float64(srcRowsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d rows, %d blocks, %d bytes) into (1 part, %d rows, %d blocks, %d bytes) in %.3f seconds at %d rows/sec to %q",
		len(pws), srcRowsCount, srcBlocksCount, srcSize, dstRowsCount, dstBlocksCount, dstSize, durationSecs, rowsPerSec, dstPartPath)
}

func (ddb *datadb) nextMergeIdx() uint64 {
	return atomic.AddUint64(&ddb.mergeIdx, 1)
}

type partType int

var (
	partInmemory = partType(0)
	partFile     = partType(1)
)

func (ddb *datadb) getDstPartType(pws []*partWrapper, isFinal bool) partType {
	if isFinal {
		return partFile
	}
	dstPartSize := getCompressedSize(pws)
	if dstPartSize > getMaxInmemoryPartSize() {
		return partFile
	}
	if !areAllInmemoryParts(pws) {
		// If at least a single source part is located in file,
		// then the destination part must be in file for durability reasons.
		return partFile
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
	if len(lr.streamIDs) == 0 {
		return
	}

	mp := getInmemoryPart()
	mp.mustInitFromRows(lr)
	p := mustOpenInmemoryPart(ddb.pt, mp)

	flushDeadline := time.Now().Add(ddb.flushInterval)
	pw := newPartWrapper(p, mp, flushDeadline)

	ddb.partsLock.Lock()
	ddb.inmemoryParts = append(ddb.inmemoryParts, pw)
	ddb.startInmemoryPartsFlusherLocked()
	if len(ddb.inmemoryParts) > defaultPartsToMerge {
		ddb.startMergeWorkerLocked()
	}
	for len(ddb.inmemoryParts) > maxInmemoryPartsPerPartition {
		// limit the pace for data ingestion if too many inmemory parts are created
		ddb.mergeDoneCond.Wait()
	}
	ddb.partsLock.Unlock()
}

// DatadbStats contains various stats for datadb.
type DatadbStats struct {
	// InmemoryMergesTotal is the number of inmemory merges performed in the given datadb.
	InmemoryMergesTotal uint64

	// InmemoryActiveMerges is the number of currently active inmemory merges performed by the given datadb.
	InmemoryActiveMerges uint64

	// FileMergesTotal is the number of file merges performed in the given datadb.
	FileMergesTotal uint64

	// FileActiveMerges is the number of currently active file merges performed by the given datadb.
	FileActiveMerges uint64

	// InmemoryRowsCount is the number of rows, which weren't flushed to disk yet.
	InmemoryRowsCount uint64

	// FileRowsCount is the number of rows stored on disk.
	FileRowsCount uint64

	// InmemoryParts is the number of in-memory parts, which weren't flushed to disk yet.
	InmemoryParts uint64

	// FileParts is the number of file-based parts stored on disk.
	FileParts uint64

	// InmemoryBlocks is the number of in-memory blocks, which weren't flushed to disk yet.
	InmemoryBlocks uint64

	// FileBlocks is the number of file-based blocks stored on disk.
	FileBlocks uint64

	// CompressedInmemorySize is the size of compressed data stored in memory.
	CompressedInmemorySize uint64

	// CompressedFileSize is the size of compressed data stored on disk.
	CompressedFileSize uint64

	// UncompressedInmemorySize is the size of uncompressed data stored in memory.
	UncompressedInmemorySize uint64

	// UncompressedFileSize is the size of uncompressed data stored on disk.
	UncompressedFileSize uint64
}

func (s *DatadbStats) reset() {
	*s = DatadbStats{}
}

// RowsCount returns the number of rows stored in datadb.
func (s *DatadbStats) RowsCount() uint64 {
	return s.InmemoryRowsCount + s.FileRowsCount
}

// updateStats updates s with ddb stats
func (ddb *datadb) updateStats(s *DatadbStats) {
	s.InmemoryMergesTotal += atomic.LoadUint64(&ddb.inmemoryMergesTotal)
	s.InmemoryActiveMerges += atomic.LoadUint64(&ddb.inmemoryActiveMerges)
	s.FileMergesTotal += atomic.LoadUint64(&ddb.fileMergesTotal)
	s.FileActiveMerges += atomic.LoadUint64(&ddb.fileActiveMerges)

	ddb.partsLock.Lock()

	s.InmemoryRowsCount += getRowsCount(ddb.inmemoryParts)
	s.FileRowsCount += getRowsCount(ddb.fileParts)

	s.InmemoryParts += uint64(len(ddb.inmemoryParts))
	s.FileParts += uint64(len(ddb.fileParts))

	s.InmemoryBlocks += getBlocksCount(ddb.inmemoryParts)
	s.FileBlocks += getBlocksCount(ddb.fileParts)

	s.CompressedInmemorySize += getCompressedSize(ddb.inmemoryParts)
	s.CompressedFileSize += getCompressedSize(ddb.fileParts)

	s.UncompressedInmemorySize += getUncompressedSize(ddb.inmemoryParts)
	s.UncompressedFileSize += getUncompressedSize(ddb.fileParts)

	ddb.partsLock.Unlock()
}

// debugFlush() makes sure that the recently ingested data is availalbe for search.
func (ddb *datadb) debugFlush() {
	// Nothing to do, since all the ingested data is available for search via ddb.inmemoryParts.
}

func (ddb *datadb) mustMergePartsFinal(pws []*partWrapper) {
	assertIsInMerge(pws)

	var pwsChunk []*partWrapper
	for len(pws) > 0 {
		pwsChunk = appendPartsToMerge(pwsChunk[:0], pws, (1<<64)-1)
		if len(pwsChunk) == 0 {
			pwsChunk = append(pwsChunk[:0], pws...)
		}
		ddb.mustMergeParts(pwsChunk, true)

		partsToRemove := partsToMap(pwsChunk)
		removedParts := 0
		pws, removedParts = removeParts(pws, partsToRemove)
		if removedParts != len(pwsChunk) {
			logger.Panicf("BUG: unexpected number of parts removed; got %d; want %d", removedParts, len(pwsChunk))
		}
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

func (ddb *datadb) swapSrcWithDstParts(pws []*partWrapper, pwNew *partWrapper, dstPartType partType) {
	// Atomically unregister old parts and add new part to pt.
	partsToRemove := partsToMap(pws)
	removedInmemoryParts := 0
	removedFileParts := 0

	ddb.partsLock.Lock()

	ddb.inmemoryParts, removedInmemoryParts = removeParts(ddb.inmemoryParts, partsToRemove)
	ddb.fileParts, removedFileParts = removeParts(ddb.fileParts, partsToRemove)
	if pwNew != nil {
		switch dstPartType {
		case partInmemory:
			ddb.inmemoryParts = append(ddb.inmemoryParts, pwNew)
			ddb.startInmemoryPartsFlusherLocked()
		case partFile:
			ddb.fileParts = append(ddb.fileParts, pwNew)
		default:
			logger.Panicf("BUG: unknown partType=%d", dstPartType)
		}
		if len(ddb.inmemoryParts)+len(ddb.fileParts) > defaultPartsToMerge {
			ddb.startMergeWorkerLocked()
		}
	}

	// Atomically store the updated list of file-based parts on disk.
	// This must be performed under partsLock in order to prevent from races
	// when multiple concurrently running goroutines update the list.
	if removedFileParts > 0 || pwNew != nil && dstPartType == partFile {
		partNames := getPartNames(ddb.fileParts)
		mustWritePartNames(ddb.path, partNames)
	}

	ddb.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedFileParts
	if removedParts != len(partsToRemove) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(partsToRemove))
	}

	// Mark old parts as must be deleted and decrement reference count,
	// so they are eventually closed and deleted.
	for _, pw := range pws {
		atomic.StoreUint32(&pw.mustBeDeleted, 1)
		pw.decRef()
	}

	ddb.mergeDoneCond.Broadcast()
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

func (ddb *datadb) availableDiskSpace() uint64 {
	available := fs.MustGetFreeSpace(ddb.path)
	reserved := atomic.LoadUint64(&reservedDiskSpace)
	if available < reserved {
		return 0
	}
	return available - reserved
}

func (ddb *datadb) reserveDiskSpace(n uint64) bool {
	available := fs.MustGetFreeSpace(ddb.path)
	reserved := atomic.AddUint64(&reservedDiskSpace, n)
	if available > reserved {
		return true
	}
	ddb.releaseDiskSpace(n)
	return false
}

func (ddb *datadb) releaseDiskSpace(n uint64) {
	atomic.AddUint64(&reservedDiskSpace, -n)
}

// reservedDiskSpace tracks global reserved disk space for currently executed
// background merges across all the partitions.
//
// It should allow avoiding background merges when there is no free disk space.
var reservedDiskSpace uint64

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
	// Stop background workers
	close(ddb.stopCh)
	ddb.wg.Wait()

	// flush in-memory data to disk
	pws := append([]*partWrapper{}, ddb.inmemoryParts...)
	setInMergeLocked(pws)
	ddb.mustMergePartsFinal(pws)

	// There is no need in using ddb.partsLock here, since nobody should acces ddb now.
	for _, pw := range ddb.inmemoryParts {
		pw.decRef()
		if pw.refCount != 0 {
			logger.Panicf("BUG: there are %d references to inmemoryPart", pw.refCount)
		}
	}
	ddb.inmemoryParts = nil

	for _, pw := range ddb.fileParts {
		pw.decRef()
		if pw.refCount != 0 {
			logger.Panicf("BUG: ther are %d references to filePart", pw.refCount)
		}
	}
	ddb.fileParts = nil

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

func mustWritePartNames(path string, partNames []string) {
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

func shouldUsePageCacheForPartSize(size uint64) bool {
	mem := memory.Remaining() / defaultPartsToMerge
	return size <= uint64(mem)
}
