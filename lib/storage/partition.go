package storage

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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// The maximum size of big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxBigPartSize = 1e12

// The maximum expected number of inmemory parts per partition.
//
// The actual number of inmemory parts may exceed this value if in-memory mergers
// cannot keep up with the rate of creating new in-memory parts.
const maxInmemoryParts = 60

// Default number of parts to merge at once.
//
// This number has been obtained empirically - it gives the lowest possible overhead.
// See appendPartsToMerge tests for details.
const defaultPartsToMerge = 15

// The number of shards for rawRow entries per partition.
//
// Higher number of shards reduces CPU contention and increases the max bandwidth on multi-core systems.
var rawRowsShardsPerPartition = cgroup.AvailableCPUs()

// The interval for flushing buffered rows into parts, so they become visible to search.
const pendingRowsFlushInterval = 2 * time.Second

// The interval for guaranteed flush of recently ingested data from memory to on-disk parts, so they survive process crash.
var dataFlushInterval = 5 * time.Second

// SetDataFlushInterval sets the interval for guaranteed flush of recently ingested data from memory to disk.
//
// The data can be flushed from memory to disk more frequently if it doesn't fit the memory limit.
//
// This function must be called before initializing the storage.
func SetDataFlushInterval(d time.Duration) {
	if d < pendingRowsFlushInterval {
		// There is no sense in setting dataFlushInterval to values smaller than pendingRowsFlushInterval,
		// since pending rows unconditionally remain in memory for up to pendingRowsFlushInterval.
		d = pendingRowsFlushInterval
	}

	dataFlushInterval = d
}

// The maximum number of rawRow items in rawRowsShard.
//
// Limit the maximum shard size to 8Mb, since this gives the lowest CPU usage under high ingestion rate.
const maxRawRowsPerShard = (8 << 20) / int(unsafe.Sizeof(rawRow{}))

// partition represents a partition.
type partition struct {
	activeInmemoryMerges atomic.Int64
	activeSmallMerges    atomic.Int64
	activeBigMerges      atomic.Int64

	inmemoryMergesCount atomic.Uint64
	smallMergesCount    atomic.Uint64
	bigMergesCount      atomic.Uint64

	inmemoryRowsMerged atomic.Uint64
	smallRowsMerged    atomic.Uint64
	bigRowsMerged      atomic.Uint64

	inmemoryRowsDeleted atomic.Uint64
	smallRowsDeleted    atomic.Uint64
	bigRowsDeleted      atomic.Uint64

	isDedupScheduled atomic.Bool

	mergeIdx atomic.Uint64

	// the path to directory with smallParts.
	smallPartsPath string

	// the path to directory with bigParts.
	bigPartsPath string

	// The parent storage.
	s *Storage

	// Name is the name of the partition in the form YYYY_MM.
	name string

	// The time range for the partition. Usually this is a whole month.
	tr TimeRange

	// rawRows contains recently added rows that haven't been converted into parts yet.
	//
	// rawRows are converted into inmemoryParts on every pendingRowsFlushInterval or when rawRows becomes full.
	//
	// rawRows aren't visible for search due to performance reasons.
	rawRows rawRowsShards

	// partsLock protects inmemoryParts, smallParts and bigParts.
	partsLock sync.Mutex

	// Contains inmemory parts with recently ingested data, which are visible for search.
	inmemoryParts []*partWrapper

	// Contains file-based parts with small number of items, which are visible for search.
	smallParts []*partWrapper

	// Contains file-based parts with big number of items, which are visible for search.
	bigParts []*partWrapper

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
}

// partWrapper is a wrapper for the part.
type partWrapper struct {
	// The number of references to the part.
	refCount atomic.Int32

	// The flag, which is set when the part must be deleted after refCount reaches zero.
	// This field should be updated only after partWrapper
	// was removed from the list of active parts.
	mustDrop atomic.Bool

	// The part itself.
	p *part

	// non-nil if the part is inmemoryPart.
	mp *inmemoryPart

	// Whether the part is in merge now.
	isInMerge bool

	// The deadline when in-memory part must be flushed to disk.
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
		putInmemoryPart(pw.mp)
		pw.mp = nil
	}
	pw.p.MustClose()
	pw.p = nil

	if deletePath != "" {
		fs.MustRemoveAll(deletePath)
	}
}

// mustCreatePartition creates new partition for the given timestamp and the given paths
// to small and big partitions.
func mustCreatePartition(timestamp int64, smallPartitionsPath, bigPartitionsPath string, s *Storage) *partition {
	name := timestampToPartitionName(timestamp)
	smallPartsPath := filepath.Join(filepath.Clean(smallPartitionsPath), name)
	bigPartsPath := filepath.Join(filepath.Clean(bigPartitionsPath), name)
	logger.Infof("creating a partition %q with smallPartsPath=%q, bigPartsPath=%q", name, smallPartsPath, bigPartsPath)

	fs.MustMkdirFailIfExist(smallPartsPath)
	fs.MustMkdirFailIfExist(bigPartsPath)

	var tr TimeRange
	tr.fromPartitionTimestamp(timestamp)

	pt := newPartition(name, smallPartsPath, bigPartsPath, tr, s)

	pt.startBackgroundWorkers()

	logger.Infof("partition %q has been created", name)

	return pt
}

func (pt *partition) startBackgroundWorkers() {
	// Start file parts mergers, so they could start merging unmerged parts if needed.
	// There is no need in starting in-memory parts mergers, since there are no in-memory parts yet.
	pt.startSmallPartsMergers()
	pt.startBigPartsMergers()

	pt.startPendingRowsFlusher()
	pt.startInmemoryPartsFlusher()
	pt.startStalePartsRemover()
}

// Drop drops all the data on the storage for the given pt.
//
// The pt must be detached from table before calling pt.Drop.
func (pt *partition) Drop() {
	logger.Infof("dropping partition %q at smallPartsPath=%q, bigPartsPath=%q", pt.name, pt.smallPartsPath, pt.bigPartsPath)

	fs.MustRemoveDirAtomic(pt.smallPartsPath)
	fs.MustRemoveDirAtomic(pt.bigPartsPath)
	logger.Infof("partition %q has been dropped", pt.name)
}

// mustOpenPartition opens the existing partition from the given paths.
func mustOpenPartition(smallPartsPath, bigPartsPath string, s *Storage) *partition {
	smallPartsPath = filepath.Clean(smallPartsPath)
	bigPartsPath = filepath.Clean(bigPartsPath)

	name := filepath.Base(smallPartsPath)
	var tr TimeRange
	if err := tr.fromPartitionName(name); err != nil {
		logger.Panicf("FATAL: cannot obtain partition time range from smallPartsPath %q: %s", smallPartsPath, err)
	}
	if !strings.HasSuffix(bigPartsPath, name) {
		logger.Panicf("FATAL: partition name in bigPartsPath %q doesn't match smallPartsPath %q; want %q", bigPartsPath, smallPartsPath, name)
	}

	partsFile := filepath.Join(smallPartsPath, partsFilename)
	partNamesSmall, partNamesBig := mustReadPartNames(partsFile, smallPartsPath, bigPartsPath)

	smallParts := mustOpenParts(partsFile, smallPartsPath, partNamesSmall)
	bigParts := mustOpenParts(partsFile, bigPartsPath, partNamesBig)

	if !fs.IsPathExist(partsFile) {
		// Create parts.json file if it doesn't exist yet.
		// This should protect from possible carshloops just after the migration from versions below v1.90.0
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4336
		mustWritePartNames(smallParts, bigParts, smallPartsPath)
	}

	pt := newPartition(name, smallPartsPath, bigPartsPath, tr, s)
	pt.smallParts = smallParts
	pt.bigParts = bigParts

	pt.startBackgroundWorkers()

	return pt
}

func newPartition(name, smallPartsPath, bigPartsPath string, tr TimeRange, s *Storage) *partition {
	p := &partition{
		name:           name,
		smallPartsPath: smallPartsPath,
		bigPartsPath:   bigPartsPath,
		tr:             tr,
		s:              s,
		stopCh:         make(chan struct{}),
	}
	p.mergeIdx.Store(uint64(time.Now().UnixNano()))
	p.rawRows.init()
	return p
}

// partitionMetrics contains essential metrics for the partition.
type partitionMetrics struct {
	PendingRows uint64

	IndexBlocksCacheSize         uint64
	IndexBlocksCacheSizeBytes    uint64
	IndexBlocksCacheSizeMaxBytes uint64
	IndexBlocksCacheRequests     uint64
	IndexBlocksCacheMisses       uint64

	InmemorySizeBytes uint64
	SmallSizeBytes    uint64
	BigSizeBytes      uint64

	InmemoryRowsCount uint64
	SmallRowsCount    uint64
	BigRowsCount      uint64

	InmemoryBlocksCount uint64
	SmallBlocksCount    uint64
	BigBlocksCount      uint64

	InmemoryPartsCount uint64
	SmallPartsCount    uint64
	BigPartsCount      uint64

	ActiveInmemoryMerges uint64
	ActiveSmallMerges    uint64
	ActiveBigMerges      uint64

	InmemoryMergesCount uint64
	SmallMergesCount    uint64
	BigMergesCount      uint64

	InmemoryRowsMerged uint64
	SmallRowsMerged    uint64
	BigRowsMerged      uint64

	InmemoryRowsDeleted uint64
	SmallRowsDeleted    uint64
	BigRowsDeleted      uint64

	InmemoryPartsRefCount uint64
	SmallPartsRefCount    uint64
	BigPartsRefCount      uint64

	ScheduledDownsamplingPartitions     uint64
	ScheduledDownsamplingPartitionsSize uint64
}

// TotalRowsCount returns total number of rows in tm.
func (pm *partitionMetrics) TotalRowsCount() uint64 {
	return pm.PendingRows + pm.InmemoryRowsCount + pm.SmallRowsCount + pm.BigRowsCount
}

// UpdateMetrics updates m with metrics from pt.
func (pt *partition) UpdateMetrics(m *partitionMetrics) {
	m.PendingRows += uint64(pt.rawRows.Len())

	pt.partsLock.Lock()

	isDedupScheduled := pt.isDedupScheduled.Load()
	if isDedupScheduled {
		m.ScheduledDownsamplingPartitions++
	}

	for _, pw := range pt.inmemoryParts {
		p := pw.p
		m.InmemoryRowsCount += p.ph.RowsCount
		m.InmemoryBlocksCount += p.ph.BlocksCount
		m.InmemorySizeBytes += p.size
		m.InmemoryPartsRefCount += uint64(pw.refCount.Load())
		if isDedupScheduled {
			m.ScheduledDownsamplingPartitionsSize += p.size
		}
	}
	for _, pw := range pt.smallParts {
		p := pw.p
		m.SmallRowsCount += p.ph.RowsCount
		m.SmallBlocksCount += p.ph.BlocksCount
		m.SmallSizeBytes += p.size
		m.SmallPartsRefCount += uint64(pw.refCount.Load())
		if isDedupScheduled {
			m.ScheduledDownsamplingPartitionsSize += p.size
		}
	}
	for _, pw := range pt.bigParts {
		p := pw.p
		m.BigRowsCount += p.ph.RowsCount
		m.BigBlocksCount += p.ph.BlocksCount
		m.BigSizeBytes += p.size
		m.BigPartsRefCount += uint64(pw.refCount.Load())
		if isDedupScheduled {
			m.ScheduledDownsamplingPartitionsSize += p.size
		}
	}

	m.InmemoryPartsCount += uint64(len(pt.inmemoryParts))
	m.SmallPartsCount += uint64(len(pt.smallParts))
	m.BigPartsCount += uint64(len(pt.bigParts))

	pt.partsLock.Unlock()

	m.IndexBlocksCacheSize = uint64(ibCache.Len())
	m.IndexBlocksCacheSizeBytes = uint64(ibCache.SizeBytes())
	m.IndexBlocksCacheSizeMaxBytes = uint64(ibCache.SizeMaxBytes())
	m.IndexBlocksCacheRequests = ibCache.Requests()
	m.IndexBlocksCacheMisses = ibCache.Misses()

	m.ActiveInmemoryMerges += uint64(pt.activeInmemoryMerges.Load())
	m.ActiveSmallMerges += uint64(pt.activeSmallMerges.Load())
	m.ActiveBigMerges += uint64(pt.activeBigMerges.Load())

	m.InmemoryMergesCount += pt.inmemoryMergesCount.Load()
	m.SmallMergesCount += pt.smallMergesCount.Load()
	m.BigMergesCount += pt.bigMergesCount.Load()

	m.InmemoryRowsMerged += pt.inmemoryRowsMerged.Load()
	m.SmallRowsMerged += pt.smallRowsMerged.Load()
	m.BigRowsMerged += pt.bigRowsMerged.Load()

	m.InmemoryRowsDeleted += pt.inmemoryRowsDeleted.Load()
	m.SmallRowsDeleted += pt.smallRowsDeleted.Load()
	m.BigRowsDeleted += pt.bigRowsDeleted.Load()
}

// AddRows adds the given rows to the partition pt.
//
// All the rows must fit the partition by timestamp range
// and must have valid PrecisionBits.
func (pt *partition) AddRows(rows []rawRow) {
	if len(rows) == 0 {
		return
	}

	if isDebug {
		// Validate all the rows.
		for i := range rows {
			r := &rows[i]
			if !pt.HasTimestamp(r.Timestamp) {
				logger.Panicf("BUG: row %+v has Timestamp outside partition %q range %+v", r, pt.smallPartsPath, &pt.tr)
			}
			if err := encoding.CheckPrecisionBits(r.PrecisionBits); err != nil {
				logger.Panicf("BUG: row %+v has invalid PrecisionBits: %s", r, err)
			}
		}
	}

	pt.rawRows.addRows(pt, rows)
}

var isDebug = false

type rawRowsShards struct {
	flushDeadlineMs atomic.Int64

	shardIdx atomic.Uint32

	// Shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawRowsShard

	rowssToFlushLock sync.Mutex
	rowssToFlush     [][]rawRow
}

func (rrss *rawRowsShards) init() {
	rrss.shards = make([]rawRowsShard, rawRowsShardsPerPartition)
}

func (rrss *rawRowsShards) addRows(pt *partition, rows []rawRow) {
	shards := rrss.shards
	shardsLen := uint32(len(shards))
	for len(rows) > 0 {
		n := rrss.shardIdx.Add(1)
		idx := n % shardsLen
		tailRows, rowsToFlush := shards[idx].addRows(rows)
		rrss.addRowsToFlush(pt, rowsToFlush)
		rows = tailRows
	}
}

func (rrss *rawRowsShards) addRowsToFlush(pt *partition, rowsToFlush []rawRow) {
	if len(rowsToFlush) == 0 {
		return
	}

	var rowssToMerge [][]rawRow

	rrss.rowssToFlushLock.Lock()
	if len(rrss.rowssToFlush) == 0 {
		rrss.updateFlushDeadline()
	}
	rrss.rowssToFlush = append(rrss.rowssToFlush, rowsToFlush)
	if len(rrss.rowssToFlush) >= defaultPartsToMerge {
		rowssToMerge = rrss.rowssToFlush
		rrss.rowssToFlush = nil
	}
	rrss.rowssToFlushLock.Unlock()

	pt.flushRowssToInmemoryParts(rowssToMerge)
}

func (rrss *rawRowsShards) Len() int {
	n := 0
	for i := range rrss.shards[:] {
		n += rrss.shards[i].Len()
	}

	rrss.rowssToFlushLock.Lock()
	for _, rows := range rrss.rowssToFlush {
		n += len(rows)
	}
	rrss.rowssToFlushLock.Unlock()

	return n
}

func (rrss *rawRowsShards) updateFlushDeadline() {
	rrss.flushDeadlineMs.Store(time.Now().Add(pendingRowsFlushInterval).UnixMilli())
}

type rawRowsShardNopad struct {
	flushDeadlineMs atomic.Int64

	mu   sync.Mutex
	rows []rawRow
}

type rawRowsShard struct {
	rawRowsShardNopad

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(rawRowsShardNopad{})%atomicutil.CacheLineSize]byte
}

func (rrs *rawRowsShard) Len() int {
	rrs.mu.Lock()
	n := len(rrs.rows)
	rrs.mu.Unlock()
	return n
}

func (rrs *rawRowsShard) addRows(rows []rawRow) ([]rawRow, []rawRow) {
	var rowsToFlush []rawRow

	rrs.mu.Lock()
	if cap(rrs.rows) == 0 {
		rrs.rows = newRawRows()
	}
	if len(rrs.rows) == 0 {
		rrs.updateFlushDeadline()
	}
	n := copy(rrs.rows[len(rrs.rows):cap(rrs.rows)], rows)
	rrs.rows = rrs.rows[:len(rrs.rows)+n]
	rows = rows[n:]
	if len(rows) > 0 {
		rowsToFlush = rrs.rows
		rrs.rows = newRawRows()
		rrs.updateFlushDeadline()
		n = copy(rrs.rows[:cap(rrs.rows)], rows)
		rrs.rows = rrs.rows[:n]
		rows = rows[n:]
	}
	rrs.mu.Unlock()

	return rows, rowsToFlush
}

func newRawRows() []rawRow {
	return make([]rawRow, 0, maxRawRowsPerShard)
}

func (pt *partition) flushRowssToInmemoryParts(rowss [][]rawRow) {
	if len(rowss) == 0 {
		return
	}

	// Convert rowss into in-memory parts.
	var pwsLock sync.Mutex
	pws := make([]*partWrapper, 0, len(rowss))
	wg := getWaitGroup()
	for _, rows := range rowss {
		wg.Add(1)
		inmemoryPartsConcurrencyCh <- struct{}{}
		go func(rowsChunk []rawRow) {
			defer func() {
				<-inmemoryPartsConcurrencyCh
				wg.Done()
			}()

			pw := pt.createInmemoryPart(rowsChunk)
			if pw != nil {
				pwsLock.Lock()
				pws = append(pws, pw)
				pwsLock.Unlock()
			}
		}(rows)
	}
	wg.Wait()
	putWaitGroup(wg)

	// Merge pws into a single in-memory part.
	maxPartSize := getMaxInmemoryPartSize()
	for len(pws) > 1 {
		pws = pt.mustMergeInmemoryParts(pws)

		pwsRemaining := pws[:0]
		for _, pw := range pws {
			if pw.p.size >= maxPartSize {
				pt.addToInmemoryParts(pw)
			} else {
				pwsRemaining = append(pwsRemaining, pw)
			}
		}
		pws = pwsRemaining
	}
	if len(pws) == 1 {
		pt.addToInmemoryParts(pws[0])
	}
}

func (pt *partition) addToInmemoryParts(pw *partWrapper) {
	pt.partsLock.Lock()
	pt.inmemoryParts = append(pt.inmemoryParts, pw)
	pt.startInmemoryPartsMergerLocked()
	pt.partsLock.Unlock()
}

func (pt *partition) NotifyReadWriteMode() {
	pt.startInmemoryPartsMergers()
	pt.startSmallPartsMergers()
	pt.startBigPartsMergers()
}

func (pt *partition) inmemoryPartsMerger() {
	for {
		if pt.s.isReadOnly.Load() {
			return
		}
		maxOutBytes := pt.getMaxBigPartSize()

		pt.partsLock.Lock()
		pws := getPartsToMerge(pt.inmemoryParts, maxOutBytes)
		pt.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		inmemoryPartsConcurrencyCh <- struct{}{}
		err := pt.mergeParts(pws, pt.stopCh, false, false)
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
		logger.Panicf("FATAL: unrecoverable error when merging inmemory parts in partition %q: %s", pt.name, err)
	}
}

func (pt *partition) smallPartsMerger() {
	for {
		if pt.s.isReadOnly.Load() {
			return
		}
		maxOutBytes := pt.getMaxBigPartSize()

		pt.partsLock.Lock()
		pws := getPartsToMerge(pt.smallParts, maxOutBytes)
		pt.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		smallPartsConcurrencyCh <- struct{}{}
		err := pt.mergeParts(pws, pt.stopCh, false, false)
		<-smallPartsConcurrencyCh

		if err == nil {
			// Try merging additional parts.
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// Nothing to do - finish the merger.
			return
		}
		// Unexpected error.
		logger.Panicf("FATAL: unrecoverable error when merging small parts at %q: %s", pt.smallPartsPath, err)
	}
}

func (pt *partition) bigPartsMerger() {
	for {
		if pt.s.isReadOnly.Load() {
			return
		}
		maxOutBytes := pt.getMaxBigPartSize()

		pt.partsLock.Lock()
		pws := getPartsToMerge(pt.bigParts, maxOutBytes)
		pt.partsLock.Unlock()

		if len(pws) == 0 {
			// Nothing to merge
			return
		}

		bigPartsConcurrencyCh <- struct{}{}
		err := pt.mergeParts(pws, pt.stopCh, false, false)
		<-bigPartsConcurrencyCh

		if err == nil {
			// Try merging additional parts.
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// Nothing to do - finish the merger.
			return
		}
		// Unexpected error.
		logger.Panicf("FATAL: unrecoverable error when merging big parts at %q: %s", pt.bigPartsPath, err)
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

func (pt *partition) mustMergeInmemoryParts(pws []*partWrapper) []*partWrapper {
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

			pw := pt.mustMergeInmemoryPartsFinal(pwsChunk)
			if pw == nil {
				return
			}

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

// mustMergeInmemoryPartsFinal merges the given in-memory part wrappers (pws) into a single new in-memory part wrapper.
// It panics if the input slice pws is empty (though the caller should prevent this).
// Returns nil if the merge results in an empty part (e.g., due to retention filters removing all data).
// Otherwise, returns the wrapper for the merged part.
func (pt *partition) mustMergeInmemoryPartsFinal(pws []*partWrapper) *partWrapper {
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

	// determine flushToDiskDeadline before performing the actual merge,
	// in order to guarantee the correct deadline, since the merge may take significant amounts of time.
	flushToDiskDeadline := getFlushToDiskDeadline(pws)

	// Prepare blockStreamWriter for destination part.
	srcRowsCount := uint64(0)
	srcBlocksCount := uint64(0)
	for _, bsr := range bsrs {
		srcRowsCount += bsr.ph.RowsCount
		srcBlocksCount += bsr.ph.BlocksCount
	}
	rowsPerBlock := float64(srcRowsCount) / float64(srcBlocksCount)
	compressLevel := getCompressLevel(rowsPerBlock)
	bsw := getBlockStreamWriter()
	mpDst := getInmemoryPart()
	bsw.MustInitFromInmemoryPart(mpDst, compressLevel)

	// Merge parts.
	// The merge shouldn't be interrupted by stopCh, so use nil stopCh.
	ph, err := pt.mergePartsInternal("", bsw, bsrs, partInmemory, nil, time.Now().UnixMilli(), false)
	putBlockStreamWriter(bsw)
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	if err != nil {
		logger.Panicf("FATAL: cannot merge inmemoryBlocks: %s", err)
	}

	// The resulting part is empty, no need to create a part wrapper
	if ph.BlocksCount == 0 {
		putInmemoryPart(mpDst)
		return nil
	}

	mpDst.ph = *ph
	return newPartWrapperFromInmemoryPart(mpDst, flushToDiskDeadline)
}

func (pt *partition) createInmemoryPart(rows []rawRow) *partWrapper {
	if len(rows) == 0 {
		return nil
	}
	mp := getInmemoryPart()
	mp.InitFromRows(rows)

	// Make sure the part may be added.
	if mp.ph.MinTimestamp > mp.ph.MaxTimestamp {
		logger.Panicf("BUG: the part %q cannot be added to partition %q because its MinTimestamp exceeds MaxTimestamp; %d vs %d",
			&mp.ph, pt.smallPartsPath, mp.ph.MinTimestamp, mp.ph.MaxTimestamp)
	}
	if mp.ph.MinTimestamp < pt.tr.MinTimestamp {
		logger.Panicf("BUG: the part %q cannot be added to partition %q because of too small MinTimestamp; got %d; want at least %d",
			&mp.ph, pt.smallPartsPath, mp.ph.MinTimestamp, pt.tr.MinTimestamp)
	}
	if mp.ph.MaxTimestamp > pt.tr.MaxTimestamp {
		logger.Panicf("BUG: the part %q cannot be added to partition %q because of too big MaxTimestamp; got %d; want at least %d",
			&mp.ph, pt.smallPartsPath, mp.ph.MaxTimestamp, pt.tr.MaxTimestamp)
	}
	flushToDiskDeadline := time.Now().Add(dataFlushInterval)
	return newPartWrapperFromInmemoryPart(mp, flushToDiskDeadline)
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

// HasTimestamp returns true if the pt contains the given timestamp.
func (pt *partition) HasTimestamp(timestamp int64) bool {
	return timestamp >= pt.tr.MinTimestamp && timestamp <= pt.tr.MaxTimestamp
}

// GetParts appends parts snapshot to dst and returns it.
//
// The appended parts must be released with PutParts.
func (pt *partition) GetParts(dst []*partWrapper, addInMemory bool) []*partWrapper {
	pt.partsLock.Lock()
	if addInMemory {
		incRefForParts(pt.inmemoryParts)
		dst = append(dst, pt.inmemoryParts...)
	}
	incRefForParts(pt.smallParts)
	dst = append(dst, pt.smallParts...)
	incRefForParts(pt.bigParts)
	dst = append(dst, pt.bigParts...)
	pt.partsLock.Unlock()

	return dst
}

// PutParts releases the given pws obtained via GetParts.
func (pt *partition) PutParts(pws []*partWrapper) {
	for _, pw := range pws {
		pw.decRef()
	}
}

func incRefForParts(pws []*partWrapper) {
	for _, pw := range pws {
		pw.incRef()
	}
}

// MustClose closes the pt, so the app may safely exit.
//
// The pt must be detached from table before calling pt.MustClose.
func (pt *partition) MustClose() {
	// Notify the background workers to stop.
	// The pt.partsLock is acquired in order to guarantee that pt.wg.Add() isn't called
	// after pt.stopCh is closed and pt.wg.Wait() is called below.
	pt.partsLock.Lock()
	close(pt.stopCh)
	pt.partsLock.Unlock()

	// Wait for background workers to stop.
	pt.wg.Wait()

	// Flush the remaining in-memory rows to files.
	pt.flushInmemoryRowsToFiles()

	// Remove references from inmemoryParts, smallParts and bigParts, so they may be eventually closed
	// after all the searches are done.
	pt.partsLock.Lock()

	if n := pt.rawRows.Len(); n > 0 {
		logger.Panicf("BUG: raw rows must be empty at this stage; got %d rows", n)
	}

	if n := len(pt.inmemoryParts); n > 0 {
		logger.Panicf("BUG: in-memory parts must be empty at this stage; got %d parts", n)
	}
	pt.inmemoryParts = nil

	smallParts := pt.smallParts
	pt.smallParts = nil

	bigParts := pt.bigParts
	pt.bigParts = nil

	pt.partsLock.Unlock()

	for _, pw := range smallParts {
		pw.decRef()
	}
	for _, pw := range bigParts {
		pw.decRef()
	}
}

func (pt *partition) startInmemoryPartsMergers() {
	pt.partsLock.Lock()
	for i := 0; i < cap(inmemoryPartsConcurrencyCh); i++ {
		pt.startInmemoryPartsMergerLocked()
	}
	pt.partsLock.Unlock()
}

func (pt *partition) startInmemoryPartsMergerLocked() {
	select {
	case <-pt.stopCh:
		return
	default:
	}
	pt.wg.Add(1)
	go func() {
		pt.inmemoryPartsMerger()
		pt.wg.Done()
	}()
}

func (pt *partition) startSmallPartsMergers() {
	pt.partsLock.Lock()
	for i := 0; i < cap(smallPartsConcurrencyCh); i++ {
		pt.startSmallPartsMergerLocked()
	}
	pt.partsLock.Unlock()
}

func (pt *partition) startSmallPartsMergerLocked() {
	select {
	case <-pt.stopCh:
		return
	default:
	}
	pt.wg.Add(1)
	go func() {
		pt.smallPartsMerger()
		pt.wg.Done()
	}()
}

func (pt *partition) startBigPartsMergers() {
	pt.partsLock.Lock()
	for i := 0; i < cap(bigPartsConcurrencyCh); i++ {
		pt.startBigPartsMergerLocked()
	}
	pt.partsLock.Unlock()
}

func (pt *partition) startBigPartsMergerLocked() {
	select {
	case <-pt.stopCh:
		return
	default:
	}
	pt.wg.Add(1)
	go func() {
		pt.bigPartsMerger()
		pt.wg.Done()
	}()
}

func (pt *partition) startPendingRowsFlusher() {
	pt.wg.Add(1)
	go func() {
		pt.pendingRowsFlusher()
		pt.wg.Done()
	}()
}

func (pt *partition) startInmemoryPartsFlusher() {
	pt.wg.Add(1)
	go func() {
		pt.inmemoryPartsFlusher()
		pt.wg.Done()
	}()
}

func (pt *partition) startStalePartsRemover() {
	pt.wg.Add(1)
	go func() {
		pt.stalePartsRemover()
		pt.wg.Done()
	}()
}

var (
	inmemoryPartsConcurrencyCh = make(chan struct{}, getInmemoryPartsConcurrency())
	smallPartsConcurrencyCh    = make(chan struct{}, getSmallPartsConcurrency())
	bigPartsConcurrencyCh      = make(chan struct{}, getBigPartsConcurrency())
)

func getInmemoryPartsConcurrency() int {
	// The concurrency for processing in-memory parts must equal to the number of CPU cores,
	// since these operations are CPU-bound.
	return cgroup.AvailableCPUs()
}

func getSmallPartsConcurrency() int {
	n := cgroup.AvailableCPUs()
	if n < 4 {
		// Allow at least 4 concurrent workers for small parts on systems
		// with less than 4 CPU cores in order to be able to make smaller part merges
		// when bigger part merges are in progress.
		return 4
	}
	return n
}

func getBigPartsConcurrency() int {
	n := cgroup.AvailableCPUs()
	if n < 4 {
		// Allow at least 4 concurrent workers for big parts on systems
		// with less than 4 CPU cores in order to be able to make smaller part merges
		// when bigger part merges are in progress.
		return 4
	}
	return n
}

func (pt *partition) inmemoryPartsFlusher() {
	// Do not add jitter to d in order to guarantee the flush interval
	d := dataFlushInterval
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pt.flushInmemoryPartsToFiles(false)
		}
	}
}

func (pt *partition) pendingRowsFlusher() {
	// Do not add jitter to d in order to guarantee the flush interval
	d := pendingRowsFlushInterval
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pt.flushPendingRows(false)
		}
	}
}

func (pt *partition) flushPendingRows(isFinal bool) {
	pt.rawRows.flush(pt, isFinal)
}

func (pt *partition) flushInmemoryRowsToFiles() {
	pt.flushPendingRows(true)
	pt.flushInmemoryPartsToFiles(true)
}

func (pt *partition) flushInmemoryPartsToFiles(isFinal bool) {
	currentTime := time.Now()
	var pws []*partWrapper

	pt.partsLock.Lock()
	for _, pw := range pt.inmemoryParts {
		if !pw.isInMerge && (isFinal || pw.flushToDiskDeadline.Before(currentTime)) {
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	pt.partsLock.Unlock()

	if err := pt.mergePartsToFiles(pws, nil, inmemoryPartsConcurrencyCh, false); err != nil {
		logger.Panicf("FATAL: cannot merge in-memory parts: %s", err)
	}
}

func (rrss *rawRowsShards) flush(pt *partition, isFinal bool) {
	var dst [][]rawRow

	currentTimeMs := time.Now().UnixMilli()
	flushDeadlineMs := rrss.flushDeadlineMs.Load()
	if isFinal || currentTimeMs >= flushDeadlineMs {
		rrss.rowssToFlushLock.Lock()
		dst = rrss.rowssToFlush
		rrss.rowssToFlush = nil
		rrss.rowssToFlushLock.Unlock()
	}

	for i := range rrss.shards {
		dst = rrss.shards[i].appendRawRowsToFlush(dst, currentTimeMs, isFinal)
	}

	pt.flushRowssToInmemoryParts(dst)
}

func (rrs *rawRowsShard) appendRawRowsToFlush(dst [][]rawRow, currentTimeMs int64, isFinal bool) [][]rawRow {
	flushDeadlineMs := rrs.flushDeadlineMs.Load()
	if !isFinal && currentTimeMs < flushDeadlineMs {
		// Fast path - nothing to flush
		return dst
	}

	// Slow path - move rrs.rows to dst.
	rrs.mu.Lock()
	dst = appendRawRowss(dst, rrs.rows)
	rrs.rows = rrs.rows[:0]
	rrs.mu.Unlock()

	return dst
}

func (rrs *rawRowsShard) updateFlushDeadline() {
	rrs.flushDeadlineMs.Store(time.Now().Add(pendingRowsFlushInterval).UnixMilli())
}

func appendRawRowss(dst [][]rawRow, src []rawRow) [][]rawRow {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		dst = append(dst, newRawRows())
	}
	prows := &dst[len(dst)-1]
	n := copy((*prows)[len(*prows):cap(*prows)], src)
	*prows = (*prows)[:len(*prows)+n]
	src = src[n:]
	for len(src) > 0 {
		rows := newRawRows()
		n := copy(rows[:cap(rows)], src)
		rows = rows[:len(rows)+n]
		src = src[n:]
		dst = append(dst, rows)
	}
	return dst
}

func (pt *partition) mergePartsToFiles(pws []*partWrapper, stopCh <-chan struct{}, concurrencyCh chan struct{}, useSparseCache bool) error {
	pwsLen := len(pws)

	var errGlobal error
	var errGlobalLock sync.Mutex
	wg := getWaitGroup()
	for len(pws) > 0 {
		pwsToMerge, pwsRemaining := getPartsForOptimalMerge(pws)
		wg.Add(1)
		concurrencyCh <- struct{}{}
		go func(pwsChunk []*partWrapper) {
			defer func() {
				<-concurrencyCh
				wg.Done()
			}()

			if err := pt.mergeParts(pwsChunk, stopCh, true, useSparseCache); err != nil && !errors.Is(err, errForciblyStopped) {
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
		return fmt.Errorf("cannot merge %d parts optimally: %w", pwsLen, errGlobal)
	}
	return nil
}

// ForceMergeAllParts runs merge for all the parts in pt.
func (pt *partition) ForceMergeAllParts(stopCh <-chan struct{}) error {
	pws := pt.getAllPartsForMerge()
	if len(pws) == 0 {
		// Nothing to merge.
		return nil
	}

	// Check whether there is enough disk space for merging pws.
	newPartSize := getPartsSize(pws)
	maxOutBytes := fs.MustGetFreeSpace(pt.bigPartsPath)
	if newPartSize > maxOutBytes {
		freeSpaceNeededBytes := newPartSize - maxOutBytes
		forceMergeLogger.Warnf("cannot initiate force merge for the partition %s; additional space needed: %d bytes", pt.name, freeSpaceNeededBytes)
		pt.releasePartsToMerge(pws)
		return nil
	}

	// If len(pws) == 1, then the merge must run anyway.
	// This allows applying the configured retention, removing the deleted series
	// and performing de-duplication if needed.
	if err := pt.mergePartsToFiles(pws, stopCh, bigPartsConcurrencyCh, true); err != nil {
		return fmt.Errorf("cannot force merge %d parts from partition %q: %w", len(pws), pt.name, err)
	}

	return nil
}

var forceMergeLogger = logger.WithThrottler("forceMerge", time.Minute)

func (pt *partition) getAllPartsForMerge() []*partWrapper {
	var pws []*partWrapper
	pt.partsLock.Lock()
	if !hasActiveMerges(pt.inmemoryParts) && !hasActiveMerges(pt.smallParts) && !hasActiveMerges(pt.bigParts) {
		pws = appendAllPartsForMerge(pws, pt.inmemoryParts)
		pws = appendAllPartsForMerge(pws, pt.smallParts)
		pws = appendAllPartsForMerge(pws, pt.bigParts)
	}
	pt.partsLock.Unlock()
	return pws
}

func appendAllPartsForMerge(dst, src []*partWrapper) []*partWrapper {
	for _, pw := range src {
		if pw.isInMerge {
			logger.Panicf("BUG: part %q is already in merge", pw.p.path)
		}
		pw.isInMerge = true
		dst = append(dst, pw)
	}
	return dst
}

func hasActiveMerges(pws []*partWrapper) bool {
	for _, pw := range pws {
		if pw.isInMerge {
			return true
		}
	}
	return false
}

func getMaxInmemoryPartSize() uint64 {
	// Allocate 10% of allowed memory for in-memory parts.
	n := uint64(0.1 * float64(memory.Allowed()) / maxInmemoryParts)
	if n < 1e6 {
		n = 1e6
	}
	return n
}

func (pt *partition) getMaxSmallPartSize() uint64 {
	// Small parts are cached in the OS page cache,
	// so limit their size by the remaining free RAM.
	mem := memory.Remaining()
	n := uint64(mem) / defaultPartsToMerge
	if n < 10e6 {
		n = 10e6
	}
	// Make sure the output part fits available disk space for small parts.
	sizeLimit := getMaxOutBytes(pt.smallPartsPath, cap(smallPartsConcurrencyCh))
	if n > sizeLimit {
		n = sizeLimit
	}
	return n
}

func (pt *partition) getMaxBigPartSize() uint64 {
	// Always use 4 workers for big merges due to historical reasons.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4915#issuecomment-1733922830
	workersCount := 4
	return getMaxOutBytes(pt.bigPartsPath, workersCount)
}

func getMaxOutBytes(path string, workersCount int) uint64 {
	n := fs.MustGetFreeSpace(path)
	// Do not subtract freeDiskSpaceLimitBytes from n before calculating the maxOutBytes,
	// since this will result in sub-optimal merges - e.g. many small parts will be left unmerged.

	// Divide free space by the max number of concurrent merges.
	maxOutBytes := n / uint64(workersCount)
	if maxOutBytes > maxBigPartSize {
		maxOutBytes = maxBigPartSize
	}
	return maxOutBytes
}

func assertIsInMerge(pws []*partWrapper) {
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge unexpectedly set to false")
		}
	}
}

func (pt *partition) releasePartsToMerge(pws []*partWrapper) {
	pt.partsLock.Lock()
	for _, pw := range pws {
		if !pw.isInMerge {
			logger.Panicf("BUG: missing isInMerge flag on the part %q", pw.p.path)
		}
		pw.isInMerge = false
	}
	pt.partsLock.Unlock()
}

func (pt *partition) isFinalDedupNeeded() bool {
	dedupInterval := GetDedupInterval()

	pws := pt.GetParts(nil, false)
	minDedupInterval := getMinDedupInterval(pws)
	pt.PutParts(pws)

	return dedupInterval > minDedupInterval
}

func getMinDedupInterval(pws []*partWrapper) int64 {
	if len(pws) == 0 {
		return 0
	}
	dMin := pws[0].p.ph.MinDedupInterval
	for _, pw := range pws[1:] {
		d := pw.p.ph.MinDedupInterval
		if d < dMin {
			dMin = d
		}
	}
	return dMin
}

// mergeParts merges pws to a single resulting part.
//
// It is expected that pws contains at least a single part.
//
// Merging is immediately stopped if stopCh is closed.
//
// if isFinal is set, then the resulting part will be saved to disk.
// If at least a single source part at pws is stored on disk, then the resulting part
// will be stored to disk.
//
// All the parts inside pws must have isInMerge field set to true.
// The isInMerge field inside pws parts is set to false before returning from the function.
func (pt *partition) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal, useSparseCache bool) error {
	if len(pws) == 0 {
		logger.Panicf("BUG: empty pws cannot be passed to mergeParts()")
	}

	assertIsInMerge(pws)
	defer pt.releasePartsToMerge(pws)

	startTime := time.Now()

	// Initialize destination paths.
	dstPartType := pt.getDstPartType(pws, isFinal)
	mergeIdx := pt.nextMergeIdx()
	dstPartPath := pt.getDstPartPath(dstPartType, mergeIdx)

	if !isDedupEnabled() && isFinal && len(pws) == 1 && pws[0].mp != nil {
		// Fast path: flush a single in-memory part to disk.
		mp := pws[0].mp
		mp.MustStoreToDisk(dstPartPath)
		pwNew := pt.openCreatedPart(&mp.ph, pws, nil, dstPartPath)
		pt.swapSrcWithDstParts(pws, pwNew, dstPartType)
		return nil
	}

	// Prepare BlockStreamReaders for source parts.
	bsrs := mustOpenBlockStreamReaders(pws)

	// Prepare BlockStreamWriter for destination part.
	srcSize := uint64(0)
	srcRowsCount := uint64(0)
	srcBlocksCount := uint64(0)
	for _, pw := range pws {
		srcSize += pw.p.size
		srcRowsCount += pw.p.ph.RowsCount
		srcBlocksCount += pw.p.ph.BlocksCount
	}
	rowsPerBlock := float64(srcRowsCount) / float64(srcBlocksCount)
	compressLevel := getCompressLevel(rowsPerBlock)
	currentTimestamp := startTime.UnixMilli()
	bsw := getBlockStreamWriter()
	var mpNew *inmemoryPart
	if dstPartType == partInmemory {
		mpNew = getInmemoryPart()
		bsw.MustInitFromInmemoryPart(mpNew, compressLevel)
	} else {
		if dstPartPath == "" {
			logger.Panicf("BUG: dstPartPath must be non-empty")
		}
		nocache := dstPartType == partBig
		bsw.MustInitFromFilePart(dstPartPath, nocache, compressLevel)
	}

	ph, err := pt.mergePartsInternal(dstPartPath, bsw, bsrs, dstPartType, stopCh, currentTimestamp, useSparseCache)
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
	pwNew := pt.openCreatedPart(ph, pws, mpNew, dstPartPath)

	dstRowsCount := uint64(0)
	dstBlocksCount := uint64(0)
	dstSize := uint64(0)
	if pwNew != nil {
		pDst := pwNew.p
		dstRowsCount = pDst.ph.RowsCount
		dstBlocksCount = pDst.ph.BlocksCount
		dstSize = pDst.size
	}

	pt.swapSrcWithDstParts(pws, pwNew, dstPartType)

	d := time.Since(startTime)
	if d <= 30*time.Second {
		return nil
	}

	// Log stats for long merges.
	durationSecs := d.Seconds()
	rowsPerSec := int(float64(srcRowsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d rows, %d blocks, %d bytes) into (1 part, %d rows, %d blocks, %d bytes) in %.3f seconds at %d rows/sec to %q",
		len(pws), srcRowsCount, srcBlocksCount, srcSize, dstRowsCount, dstBlocksCount, dstSize, durationSecs, rowsPerSec, dstPartPath)

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
	partSmall    = partType(1)
	partBig      = partType(2)
)

func (pt *partition) getDstPartType(pws []*partWrapper, isFinal bool) partType {
	dstPartSize := getPartsSize(pws)
	if dstPartSize > pt.getMaxSmallPartSize() {
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

func (pt *partition) getDstPartPath(dstPartType partType, mergeIdx uint64) string {
	ptPath := ""
	switch dstPartType {
	case partSmall:
		ptPath = pt.smallPartsPath
	case partBig:
		ptPath = pt.bigPartsPath
	case partInmemory:
		ptPath = pt.smallPartsPath
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}
	dstPartPath := ""
	if dstPartType != partInmemory {
		dstPartPath = filepath.Join(ptPath, fmt.Sprintf("%016X", mergeIdx))
	}
	return dstPartPath
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

func (pt *partition) mergePartsInternal(dstPartPath string, bsw *blockStreamWriter, bsrs []*blockStreamReader, dstPartType partType, stopCh <-chan struct{}, currentTimestamp int64, useSparseCache bool) (*partHeader, error) {
	var ph partHeader
	var rowsMerged *atomic.Uint64
	var rowsDeleted *atomic.Uint64
	var mergesCount *atomic.Uint64
	var activeMerges *atomic.Int64
	switch dstPartType {
	case partInmemory:
		rowsMerged = &pt.inmemoryRowsMerged
		rowsDeleted = &pt.inmemoryRowsDeleted
		mergesCount = &pt.inmemoryMergesCount
		activeMerges = &pt.activeInmemoryMerges
	case partSmall:
		rowsMerged = &pt.smallRowsMerged
		rowsDeleted = &pt.smallRowsDeleted
		mergesCount = &pt.smallMergesCount
		activeMerges = &pt.activeSmallMerges
	case partBig:
		rowsMerged = &pt.bigRowsMerged
		rowsDeleted = &pt.bigRowsDeleted
		mergesCount = &pt.bigMergesCount
		activeMerges = &pt.activeBigMerges
	default:
		logger.Panicf("BUG: unknown partType=%d", dstPartType)
	}
	retentionDeadline := currentTimestamp - pt.s.retentionMsecs
	activeMerges.Add(1)
	dmis := pt.s.getDeletedMetricIDs()
	err := mergeBlockStreams(&ph, bsw, bsrs, stopCh, dmis, retentionDeadline, rowsMerged, rowsDeleted, useSparseCache)
	activeMerges.Add(-1)
	mergesCount.Add(1)
	if err != nil {
		return nil, fmt.Errorf("cannot merge %d parts to %s: %w", len(bsrs), dstPartPath, err)
	}
	if dstPartPath != "" {
		ph.MinDedupInterval = GetDedupInterval()
		ph.MustWriteMetadata(dstPartPath)
	}
	return &ph, nil
}

func (pt *partition) openCreatedPart(ph *partHeader, pws []*partWrapper, mpNew *inmemoryPart, dstPartPath string) *partWrapper {
	// Open the created part.
	if ph.RowsCount == 0 {
		// The created part is empty. Remove it
		if mpNew == nil {
			fs.MustRemoveAll(dstPartPath)
		}
		return nil
	}
	if mpNew != nil {
		// Open the created part from memory.
		flushToDiskDeadline := getFlushToDiskDeadline(pws)
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

func (pt *partition) swapSrcWithDstParts(pws []*partWrapper, pwNew *partWrapper, dstPartType partType) {
	// Atomically unregister old parts and add new part to pt.
	m := makeMapFromPartWrappers(pws)

	removedInmemoryParts := 0
	removedSmallParts := 0
	removedBigParts := 0

	pt.partsLock.Lock()

	pt.inmemoryParts, removedInmemoryParts = removeParts(pt.inmemoryParts, m)
	pt.smallParts, removedSmallParts = removeParts(pt.smallParts, m)
	pt.bigParts, removedBigParts = removeParts(pt.bigParts, m)
	if pwNew != nil {
		switch dstPartType {
		case partInmemory:
			pt.inmemoryParts = append(pt.inmemoryParts, pwNew)
			pt.startInmemoryPartsMergerLocked()
		case partSmall:
			pt.smallParts = append(pt.smallParts, pwNew)
			pt.startSmallPartsMergerLocked()
		case partBig:
			pt.bigParts = append(pt.bigParts, pwNew)
			pt.startBigPartsMergerLocked()
		default:
			logger.Panicf("BUG: unknown partType=%d", dstPartType)
		}
	}

	// Atomically store the updated list of file-based parts on disk.
	// This must be performed under partsLock in order to prevent from races
	// when multiple concurrently running goroutines update the list.
	if removedSmallParts > 0 || removedBigParts > 0 || pwNew != nil && (dstPartType == partSmall || dstPartType == partBig) {
		mustWritePartNames(pt.smallParts, pt.bigParts, pt.smallPartsPath)
	}

	pt.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedSmallParts + removedBigParts
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

func getCompressLevel(rowsPerBlock float64) int {
	// See https://github.com/facebook/zstd/releases/tag/v1.3.4 about negative compression levels.
	if rowsPerBlock <= 10 {
		return -5
	}
	if rowsPerBlock <= 50 {
		return -2
	}
	if rowsPerBlock <= 200 {
		return -1
	}
	if rowsPerBlock <= 500 {
		return 1
	}
	if rowsPerBlock <= 1000 {
		return 2
	}
	return 3
}

func (pt *partition) nextMergeIdx() uint64 {
	return pt.mergeIdx.Add(1)
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

func (pt *partition) stalePartsRemover() {
	d := timeutil.AddJitterToDuration(7 * time.Minute)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pt.removeStaleParts()
		}
	}
}

func (pt *partition) removeStaleParts() {
	startTime := time.Now()
	retentionDeadline := timestampFromTime(startTime) - pt.s.retentionMsecs

	var pws []*partWrapper
	pt.partsLock.Lock()
	for _, pw := range pt.inmemoryParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			pt.inmemoryRowsDeleted.Add(pw.p.ph.RowsCount)
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	for _, pw := range pt.smallParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			pt.smallRowsDeleted.Add(pw.p.ph.RowsCount)
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	for _, pw := range pt.bigParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			pt.bigRowsDeleted.Add(pw.p.ph.RowsCount)
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	pt.partsLock.Unlock()

	pt.swapSrcWithDstParts(pws, nil, partSmall)
}

// getPartsToMerge returns optimal parts to merge from pws.
//
// The summary size of the returned parts must be smaller than maxOutBytes.
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
			logger.Panicf("BUG: partWrapper.isInMerge cannot be set")
		}
		pw.isInMerge = true
	}

	return pwsToMerge
}

// getPartsForOptimalMerge returns parts from pws for optimal merge, plus the remaining parts.
//
// the pws items are replaced by nil after the call. This is needed for helping Go GC to reclaim the referenced items.
func getPartsForOptimalMerge(pws []*partWrapper) ([]*partWrapper, []*partWrapper) {
	pwsToMerge := appendPartsToMerge(nil, pws, defaultPartsToMerge, 1<<64-1)
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
	// This should reduce N for O(N^2) algorithm below.
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
			outSize := getPartsSize(a)
			if outSize > maxOutBytes {
				// There is no need in verifying remaining parts with bigger sizes.
				break
			}
			m := float64(outSize) / float64(a[len(a)-1].p.size)
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
	// Sort src parts by size and backwards timestamp.
	// This should improve adjanced points' locality in the merged parts.
	sort.Slice(pws, func(i, j int) bool {
		a := pws[i].p
		b := pws[j].p
		if a.size == b.size {
			return a.ph.MinTimestamp > b.ph.MinTimestamp
		}
		return a.size < b.size
	})
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

func mustOpenParts(partsFile, path string, partNames []string) []*partWrapper {
	// The path can be missing after restoring from backup, so create it if needed.
	fs.MustMkdirIfNotExist(path)
	fs.MustRemoveTemporaryDirs(path)

	// Remove txn and tmp directories, which may be left after the upgrade
	// to v1.90.0 and newer versions.
	fs.MustRemoveAll(filepath.Join(path, "txn"))
	fs.MustRemoveAll(filepath.Join(path, "tmp"))

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
				"ensure %q contents is not corrupted; remove %q to rebuild its content from the list of existing parts",
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
			logger.Infof("deleting %q because it isn't listed in %q; this is the expected case after unclean shutdown", deletePath, partsFile)
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
			p: p,
		}
		pw.incRef()
		pws = append(pws, pw)
	}

	return pws
}

// MustCreateSnapshotAt creates pt snapshot at the given smallPath and bigPath dirs.
//
// Snapshot is created using linux hard links, so it is usually created very quickly.
func (pt *partition) MustCreateSnapshotAt(smallPath, bigPath string) {
	logger.Infof("creating partition snapshot of %q and %q...", pt.smallPartsPath, pt.bigPartsPath)
	startTime := time.Now()

	// Flush inmemory data to disk.
	pt.flushInmemoryRowsToFiles()

	pt.partsLock.Lock()
	incRefForParts(pt.smallParts)
	pwsSmall := append([]*partWrapper{}, pt.smallParts...)
	incRefForParts(pt.bigParts)
	pwsBig := append([]*partWrapper{}, pt.bigParts...)
	pt.partsLock.Unlock()

	defer func() {
		pt.PutParts(pwsSmall)
		pt.PutParts(pwsBig)
	}()

	fs.MustMkdirFailIfExist(smallPath)
	fs.MustMkdirFailIfExist(bigPath)

	// Create a file with part names at smallPath
	mustWritePartNames(pwsSmall, pwsBig, smallPath)

	pt.mustCreateSnapshot(pt.smallPartsPath, smallPath, pwsSmall)
	pt.mustCreateSnapshot(pt.bigPartsPath, bigPath, pwsBig)

	logger.Infof("created partition snapshot of %q and %q at %q and %q in %.3f seconds",
		pt.smallPartsPath, pt.bigPartsPath, smallPath, bigPath, time.Since(startTime).Seconds())
}

// mustCreateSnapshot creates a snapshot from srcDir to dstDir.
func (pt *partition) mustCreateSnapshot(srcDir, dstDir string, pws []*partWrapper) {
	// Make hardlinks for pws at dstDir
	for _, pw := range pws {
		srcPartPath := pw.p.path
		dstPartPath := filepath.Join(dstDir, filepath.Base(srcPartPath))
		fs.MustHardLinkFiles(srcPartPath, dstPartPath)
	}

	// Copy the appliedRetentionFilename to dstDir.
	// This file can be created by VictoriaMetrics Enterprise.
	// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention-filters .
	// Do not make hard link to this file, since it can be modified over time.
	srcPath := filepath.Join(srcDir, appliedRetentionFilename)
	if fs.IsPathExist(srcPath) {
		dstPath := filepath.Join(dstDir, filepath.Base(srcPath))
		fs.MustCopyFile(srcPath, dstPath)
	}

	fs.MustSyncPath(dstDir)
	parentDir := filepath.Dir(dstDir)
	fs.MustSyncPath(parentDir)
}

type partNamesJSON struct {
	Small []string
	Big   []string
}

func mustWritePartNames(pwsSmall, pwsBig []*partWrapper, dstDir string) {
	partNamesSmall := getPartNames(pwsSmall)
	partNamesBig := getPartNames(pwsBig)
	partNames := &partNamesJSON{
		Small: partNamesSmall,
		Big:   partNamesBig,
	}
	data, err := json.Marshal(partNames)
	if err != nil {
		logger.Panicf("BUG: cannot marshal partNames to JSON: %s", err)
	}
	partsFile := filepath.Join(dstDir, partsFilename)
	fs.MustWriteAtomic(partsFile, data, true)
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

func mustReadPartNames(partsFile, smallPartsPath, bigPartsPath string) ([]string, []string) {
	if fs.IsPathExist(partsFile) {
		data, err := os.ReadFile(partsFile)
		if err != nil {
			logger.Panicf("FATAL: cannot read %q: %s", partsFile, err)
		}
		var partNames partNamesJSON
		if err := json.Unmarshal(data, &partNames); err != nil {
			logger.Panicf("FATAL: cannot parse %q: %s", partsFile, err)
		}
		return partNames.Small, partNames.Big
	}
	// The partsFile is missing. This is the upgrade from versions previous to v1.90.0.
	// Read part names from smallPartsPath and bigPartsPath directories
	partNamesSmall := mustReadPartNamesFromDir(smallPartsPath)
	partNamesBig := mustReadPartNamesFromDir(bigPartsPath)
	return partNamesSmall, partNamesBig
}

func mustReadPartNamesFromDir(srcDir string) []string {
	if !fs.IsPathExist(srcDir) {
		return nil
	}
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

func isSpecialDir(name string) bool {
	return name == "tmp" || name == "txn" || name == snapshotsDirname || fs.IsScheduledForRemoval(name)
}
