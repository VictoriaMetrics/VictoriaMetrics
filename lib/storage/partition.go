package storage

import (
	"errors"
	"fmt"
	"io/ioutil"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storagepacelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

func maxSmallPartSize() uint64 {
	// Small parts are cached in the OS page cache,
	// so limit their size by the remaining free RAM.
	mem := memory.Remaining()
	// It is expected no more than defaultPartsToMerge/2 parts exist
	// in the OS page cache before they are merged into bigger part.
	// Half of the remaining RAM must be left for lib/mergeset parts,
	// so the maxItems is calculated using the below code:
	maxSize := uint64(mem) / defaultPartsToMerge
	if maxSize < 10e6 {
		maxSize = 10e6
	}
	return maxSize
}

// The maximum size of big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxBigPartSize = 1e12

// The maximum number of small parts in the partition.
const maxSmallPartsPerPartition = 256

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
const finalPartsToMerge = 3

// The number of shards for rawRow entries per partition.
//
// Higher number of shards reduces CPU contention and increases the max bandwidth on multi-core systems.
var rawRowsShardsPerPartition = (cgroup.AvailableCPUs() + 3) / 4

// getMaxRawRowsPerShard returns the maximum number of rows that haven't been converted into parts yet.
func getMaxRawRowsPerShard() int {
	maxRawRowsPerPartitionOnce.Do(func() {
		n := memory.Allowed() / rawRowsShardsPerPartition / 256 / int(unsafe.Sizeof(rawRow{}))
		if n < 1e4 {
			n = 1e4
		}
		if n > 500e3 {
			n = 500e3
		}
		maxRawRowsPerPartition = n
	})
	return maxRawRowsPerPartition
}

var (
	maxRawRowsPerPartition     int
	maxRawRowsPerPartitionOnce sync.Once
)

// The interval for flushing (converting) recent raw rows into parts,
// so they become visible to search.
const rawRowsFlushInterval = time.Second

// The interval for flushing inmemory parts to persistent storage,
// so they survive process crash.
const inmemoryPartsFlushInterval = 5 * time.Second

// partition represents a partition.
type partition struct {
	// Put atomic counters to the top of struct, so they are aligned to 8 bytes on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	activeBigMerges   uint64
	activeSmallMerges uint64
	bigMergesCount    uint64
	smallMergesCount  uint64
	bigRowsMerged     uint64
	smallRowsMerged   uint64
	bigRowsDeleted    uint64
	smallRowsDeleted  uint64

	smallAssistedMerges uint64

	smallMergeNeedFreeDiskSpace uint64
	bigMergeNeedFreeDiskSpace   uint64

	mergeIdx uint64

	smallPartsPath string
	bigPartsPath   string

	// The callack that returns deleted metric ids which must be skipped during merge.
	getDeletedMetricIDs func() *uint64set.Set

	// data retention in milliseconds.
	// Used for deleting data outside the retention during background merge.
	retentionMsecs int64

	// Whether the storage is in read-only mode.
	// Background merge is stopped in read-only mode.
	isReadOnly *uint32

	// Name is the name of the partition in the form YYYY_MM.
	name string

	// The time range for the partition. Usually this is a whole month.
	tr TimeRange

	// partsLock protects smallParts and bigParts.
	partsLock sync.Mutex

	// Contains all the inmemoryPart plus file-based parts
	// with small number of items (up to maxRowsCountPerSmallPart).
	smallParts []*partWrapper

	// Contains file-based parts with big number of items.
	bigParts []*partWrapper

	// rawRows contains recently added rows that haven't been converted into parts yet.
	//
	// rawRows aren't used in search for performance reasons.
	rawRows rawRowsShards

	snapshotLock sync.RWMutex

	stopCh chan struct{}

	smallPartsMergerWG     sync.WaitGroup
	bigPartsMergerWG       sync.WaitGroup
	rawRowsFlusherWG       sync.WaitGroup
	inmemoryPartsFlusherWG sync.WaitGroup
	stalePartsRemoverWG    sync.WaitGroup
}

// partWrapper is a wrapper for the part.
type partWrapper struct {
	// Put atomic counters to the top of struct, so they are aligned to 8 bytes on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	// The number of references to the part.
	refCount uint64

	// The part itself.
	p *part

	// non-nil if the part is inmemoryPart.
	mp *inmemoryPart

	// Whether the part is in merge now.
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

// createPartition creates new partition for the given timestamp and the given paths
// to small and big partitions.
func createPartition(timestamp int64, smallPartitionsPath, bigPartitionsPath string,
	getDeletedMetricIDs func() *uint64set.Set, retentionMsecs int64, isReadOnly *uint32) (*partition, error) {
	name := timestampToPartitionName(timestamp)
	smallPartsPath := filepath.Clean(smallPartitionsPath) + "/" + name
	bigPartsPath := filepath.Clean(bigPartitionsPath) + "/" + name
	logger.Infof("creating a partition %q with smallPartsPath=%q, bigPartsPath=%q", name, smallPartsPath, bigPartsPath)

	if err := createPartitionDirs(smallPartsPath); err != nil {
		return nil, fmt.Errorf("cannot create directories for small parts %q: %w", smallPartsPath, err)
	}
	if err := createPartitionDirs(bigPartsPath); err != nil {
		return nil, fmt.Errorf("cannot create directories for big parts %q: %w", bigPartsPath, err)
	}

	pt := newPartition(name, smallPartsPath, bigPartsPath, getDeletedMetricIDs, retentionMsecs, isReadOnly)
	pt.tr.fromPartitionTimestamp(timestamp)
	pt.startMergeWorkers()
	pt.startRawRowsFlusher()
	pt.startInmemoryPartsFlusher()
	pt.startStalePartsRemover()

	logger.Infof("partition %q has been created", name)

	return pt, nil
}

// Drop drops all the data on the storage for the given pt.
//
// The pt must be detached from table before calling pt.Drop.
func (pt *partition) Drop() {
	logger.Infof("dropping partition %q at smallPartsPath=%q, bigPartsPath=%q", pt.name, pt.smallPartsPath, pt.bigPartsPath)
	// Wait until all the pending transaction deletions are finished before removing partition directories.
	pendingTxnDeletionsWG.Wait()

	fs.MustRemoveAll(pt.smallPartsPath)
	fs.MustRemoveAll(pt.bigPartsPath)
	logger.Infof("partition %q has been dropped", pt.name)
}

// openPartition opens the existing partition from the given paths.
func openPartition(smallPartsPath, bigPartsPath string, getDeletedMetricIDs func() *uint64set.Set, retentionMsecs int64, isReadOnly *uint32) (*partition, error) {
	smallPartsPath = filepath.Clean(smallPartsPath)
	bigPartsPath = filepath.Clean(bigPartsPath)

	n := strings.LastIndexByte(smallPartsPath, '/')
	if n < 0 {
		return nil, fmt.Errorf("cannot find partition name from smallPartsPath %q; must be in the form /path/to/smallparts/YYYY_MM", smallPartsPath)
	}
	name := smallPartsPath[n+1:]

	if !strings.HasSuffix(bigPartsPath, "/"+name) {
		return nil, fmt.Errorf("patititon name in bigPartsPath %q doesn't match smallPartsPath %q; want %q", bigPartsPath, smallPartsPath, name)
	}

	smallParts, err := openParts(smallPartsPath, bigPartsPath, smallPartsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open small parts from %q: %w", smallPartsPath, err)
	}
	bigParts, err := openParts(smallPartsPath, bigPartsPath, bigPartsPath)
	if err != nil {
		mustCloseParts(smallParts)
		return nil, fmt.Errorf("cannot open big parts from %q: %w", bigPartsPath, err)
	}

	pt := newPartition(name, smallPartsPath, bigPartsPath, getDeletedMetricIDs, retentionMsecs, isReadOnly)
	pt.smallParts = smallParts
	pt.bigParts = bigParts
	if err := pt.tr.fromPartitionName(name); err != nil {
		return nil, fmt.Errorf("cannot obtain partition time range from smallPartsPath %q: %w", smallPartsPath, err)
	}
	pt.startMergeWorkers()
	pt.startRawRowsFlusher()
	pt.startInmemoryPartsFlusher()
	pt.startStalePartsRemover()

	return pt, nil
}

func newPartition(name, smallPartsPath, bigPartsPath string, getDeletedMetricIDs func() *uint64set.Set, retentionMsecs int64, isReadOnly *uint32) *partition {
	p := &partition{
		name:           name,
		smallPartsPath: smallPartsPath,
		bigPartsPath:   bigPartsPath,

		getDeletedMetricIDs: getDeletedMetricIDs,
		retentionMsecs:      retentionMsecs,
		isReadOnly:          isReadOnly,

		mergeIdx: uint64(time.Now().UnixNano()),
		stopCh:   make(chan struct{}),
	}
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

	BigSizeBytes   uint64
	SmallSizeBytes uint64

	BigRowsCount   uint64
	SmallRowsCount uint64

	BigBlocksCount   uint64
	SmallBlocksCount uint64

	BigPartsCount   uint64
	SmallPartsCount uint64

	ActiveBigMerges   uint64
	ActiveSmallMerges uint64

	BigMergesCount   uint64
	SmallMergesCount uint64

	BigRowsMerged   uint64
	SmallRowsMerged uint64

	BigRowsDeleted   uint64
	SmallRowsDeleted uint64

	BigPartsRefCount   uint64
	SmallPartsRefCount uint64

	SmallAssistedMerges uint64

	SmallMergeNeedFreeDiskSpace uint64
	BigMergeNeedFreeDiskSpace   uint64
}

// UpdateMetrics updates m with metrics from pt.
func (pt *partition) UpdateMetrics(m *partitionMetrics) {
	rawRowsLen := uint64(pt.rawRows.Len())
	m.PendingRows += rawRowsLen
	m.SmallRowsCount += rawRowsLen

	pt.partsLock.Lock()

	for _, pw := range pt.bigParts {
		p := pw.p

		m.BigRowsCount += p.ph.RowsCount
		m.BigBlocksCount += p.ph.BlocksCount
		m.BigSizeBytes += p.size
		m.BigPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}

	for _, pw := range pt.smallParts {
		p := pw.p

		m.SmallRowsCount += p.ph.RowsCount
		m.SmallBlocksCount += p.ph.BlocksCount
		m.SmallSizeBytes += p.size
		m.SmallPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}

	m.BigPartsCount += uint64(len(pt.bigParts))
	m.SmallPartsCount += uint64(len(pt.smallParts))

	pt.partsLock.Unlock()

	m.IndexBlocksCacheSize = uint64(ibCache.Len())
	m.IndexBlocksCacheSizeBytes = uint64(ibCache.SizeBytes())
	m.IndexBlocksCacheSizeMaxBytes = uint64(ibCache.SizeMaxBytes())
	m.IndexBlocksCacheRequests = ibCache.Requests()
	m.IndexBlocksCacheMisses = ibCache.Misses()

	m.ActiveBigMerges += atomic.LoadUint64(&pt.activeBigMerges)
	m.ActiveSmallMerges += atomic.LoadUint64(&pt.activeSmallMerges)

	m.BigMergesCount += atomic.LoadUint64(&pt.bigMergesCount)
	m.SmallMergesCount += atomic.LoadUint64(&pt.smallMergesCount)

	m.BigRowsMerged += atomic.LoadUint64(&pt.bigRowsMerged)
	m.SmallRowsMerged += atomic.LoadUint64(&pt.smallRowsMerged)

	m.BigRowsDeleted += atomic.LoadUint64(&pt.bigRowsDeleted)
	m.SmallRowsDeleted += atomic.LoadUint64(&pt.smallRowsDeleted)

	m.SmallAssistedMerges += atomic.LoadUint64(&pt.smallAssistedMerges)

	m.SmallMergeNeedFreeDiskSpace += atomic.LoadUint64(&pt.smallMergeNeedFreeDiskSpace)
	m.BigMergeNeedFreeDiskSpace += atomic.LoadUint64(&pt.bigMergeNeedFreeDiskSpace)
}

// AddRows adds the given rows to the partition pt.
//
// All the rows must fit the partition by timestamp range
// and must have valid PrecisionBits.
func (pt *partition) AddRows(rows []rawRow) {
	if len(rows) == 0 {
		return
	}

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

	pt.rawRows.addRows(pt, rows)
}

type rawRowsShards struct {
	shardIdx uint32

	// Shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawRowsShard
}

func (rrss *rawRowsShards) init() {
	rrss.shards = make([]rawRowsShard, rawRowsShardsPerPartition)
}

func (rrss *rawRowsShards) addRows(pt *partition, rows []rawRow) {
	n := atomic.AddUint32(&rrss.shardIdx, 1)
	shards := rrss.shards
	idx := n % uint32(len(shards))
	shard := &shards[idx]
	shard.addRows(pt, rows)
}

func (rrss *rawRowsShards) Len() int {
	n := 0
	for i := range rrss.shards[:] {
		n += rrss.shards[i].Len()
	}
	return n
}

type rawRowsShard struct {
	mu            sync.Mutex
	rows          []rawRow
	lastFlushTime uint64
}

func (rrs *rawRowsShard) Len() int {
	rrs.mu.Lock()
	n := len(rrs.rows)
	rrs.mu.Unlock()
	return n
}

func (rrs *rawRowsShard) addRows(pt *partition, rows []rawRow) {
	var rowsToFlush []rawRow

	rrs.mu.Lock()
	if cap(rrs.rows) == 0 {
		n := getMaxRawRowsPerShard()
		rrs.rows = make([]rawRow, 0, n)
	}
	maxRowsCount := cap(rrs.rows)
	capacity := maxRowsCount - len(rrs.rows)
	if capacity >= len(rows) {
		// Fast path - rows fit capacity.
		rrs.rows = append(rrs.rows, rows...)
	} else {
		// Slow path - rows don't fit capacity.
		// Put rrs.rows and rows to rowsToFlush and convert it to a part.
		rowsToFlush = append(rowsToFlush, rrs.rows...)
		rowsToFlush = append(rowsToFlush, rows...)
		rrs.rows = rrs.rows[:0]
		rrs.lastFlushTime = fasttime.UnixTimestamp()
	}
	rrs.mu.Unlock()

	pt.flushRowsToParts(rowsToFlush)
}

func (pt *partition) flushRowsToParts(rows []rawRow) {
	maxRows := getMaxRawRowsPerShard()
	wg := getWaitGroup()
	for len(rows) > 0 {
		n := maxRows
		if n > len(rows) {
			n = len(rows)
		}
		wg.Add(1)
		go func(rowsPart []rawRow) {
			defer wg.Done()
			pt.addRowsPart(rowsPart)
		}(rows[:n])
		rows = rows[n:]
	}
	wg.Wait()
	putWaitGroup(wg)
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

func (pt *partition) addRowsPart(rows []rawRow) {
	if len(rows) == 0 {
		return
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

	p, err := mp.NewPart()
	if err != nil {
		logger.Panicf("BUG: cannot create part from %q: %s", &mp.ph, err)
	}

	pw := &partWrapper{
		p:        p,
		mp:       mp,
		refCount: 1,
	}

	pt.partsLock.Lock()
	pt.smallParts = append(pt.smallParts, pw)
	ok := len(pt.smallParts) <= maxSmallPartsPerPartition
	pt.partsLock.Unlock()
	if ok {
		return
	}

	// The added part exceeds available limit. Help merging parts.
	//
	// Prioritize assisted merges over searches.
	storagepacelimiter.Search.Inc()
	err = pt.mergeSmallParts(false)
	storagepacelimiter.Search.Dec()
	if err == nil {
		atomic.AddUint64(&pt.smallAssistedMerges, 1)
		return
	}
	if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) {
		return
	}
	logger.Panicf("FATAL: cannot merge small parts: %s", err)
}

// HasTimestamp returns true if the pt contains the given timestamp.
func (pt *partition) HasTimestamp(timestamp int64) bool {
	return timestamp >= pt.tr.MinTimestamp && timestamp <= pt.tr.MaxTimestamp
}

// GetParts appends parts snapshot to dst and returns it.
//
// The appended parts must be released with PutParts.
func (pt *partition) GetParts(dst []*partWrapper) []*partWrapper {
	pt.partsLock.Lock()
	for _, pw := range pt.smallParts {
		pw.incRef()
	}
	dst = append(dst, pt.smallParts...)
	for _, pw := range pt.bigParts {
		pw.incRef()
	}
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

// MustClose closes the pt, so the app may safely exit.
//
// The pt must be detached from table before calling pt.MustClose.
func (pt *partition) MustClose() {
	close(pt.stopCh)

	// Wait until all the pending transaction deletions are finished.
	pendingTxnDeletionsWG.Wait()

	logger.Infof("waiting for stale parts remover to stop on %q...", pt.smallPartsPath)
	startTime := time.Now()
	pt.stalePartsRemoverWG.Wait()
	logger.Infof("stale parts remover stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	logger.Infof("waiting for inmemory parts flusher to stop on %q...", pt.smallPartsPath)
	startTime = time.Now()
	pt.inmemoryPartsFlusherWG.Wait()
	logger.Infof("inmemory parts flusher stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	logger.Infof("waiting for raw rows flusher to stop on %q...", pt.smallPartsPath)
	startTime = time.Now()
	pt.rawRowsFlusherWG.Wait()
	logger.Infof("raw rows flusher stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	logger.Infof("waiting for small part mergers to stop on %q...", pt.smallPartsPath)
	startTime = time.Now()
	pt.smallPartsMergerWG.Wait()
	logger.Infof("small part mergers stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	logger.Infof("waiting for big part mergers to stop on %q...", pt.bigPartsPath)
	startTime = time.Now()
	pt.bigPartsMergerWG.Wait()
	logger.Infof("big part mergers stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.bigPartsPath)

	logger.Infof("flushing inmemory parts to files on %q...", pt.smallPartsPath)
	startTime = time.Now()

	// Flush raw rows the last time before exit.
	pt.flushRawRows(true)

	// Flush inmemory parts to disk.
	var pws []*partWrapper
	pt.partsLock.Lock()
	for _, pw := range pt.smallParts {
		if pw.mp == nil {
			continue
		}
		if pw.isInMerge {
			logger.Panicf("BUG: the inmemory part %q mustn't be in merge after stopping small parts merger in the partition %q", &pw.mp.ph, pt.smallPartsPath)
		}
		pw.isInMerge = true
		pws = append(pws, pw)
	}
	pt.partsLock.Unlock()

	if err := pt.mergePartsOptimal(pws, nil); err != nil {
		logger.Panicf("FATAL: cannot flush %d inmemory parts to files on %q: %s", len(pws), pt.smallPartsPath, err)
	}
	logger.Infof("%d inmemory parts have been flushed to files in %.3f seconds on %q", len(pws), time.Since(startTime).Seconds(), pt.smallPartsPath)

	// Remove references to smallParts from the pt, so they may be eventually closed
	// after all the searches are done.
	pt.partsLock.Lock()
	smallParts := pt.smallParts
	pt.smallParts = nil
	pt.partsLock.Unlock()

	for _, pw := range smallParts {
		pw.decRef()
	}

	// Remove references to bigParts from the pt, so they may be eventually closed
	// after all the searches are done.
	pt.partsLock.Lock()
	bigParts := pt.bigParts
	pt.bigParts = nil
	pt.partsLock.Unlock()

	for _, pw := range bigParts {
		pw.decRef()
	}
}

func (pt *partition) startRawRowsFlusher() {
	pt.rawRowsFlusherWG.Add(1)
	go func() {
		pt.rawRowsFlusher()
		pt.rawRowsFlusherWG.Done()
	}()
}

func (pt *partition) rawRowsFlusher() {
	ticker := time.NewTicker(rawRowsFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pt.flushRawRows(false)
		}
	}
}

func (pt *partition) flushRawRows(isFinal bool) {
	pt.rawRows.flush(pt, isFinal)
}

func (rrss *rawRowsShards) flush(pt *partition, isFinal bool) {
	var rowsToFlush []rawRow
	for i := range rrss.shards {
		rowsToFlush = rrss.shards[i].appendRawRowsToFlush(rowsToFlush, pt, isFinal)
	}
	pt.flushRowsToParts(rowsToFlush)
}

func (rrs *rawRowsShard) appendRawRowsToFlush(dst []rawRow, pt *partition, isFinal bool) []rawRow {
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(rawRowsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}

	rrs.mu.Lock()
	if isFinal || currentTime-rrs.lastFlushTime > uint64(flushSeconds) {
		dst = append(dst, rrs.rows...)
		rrs.rows = rrs.rows[:0]
	}
	rrs.mu.Unlock()

	return dst
}

func (pt *partition) startInmemoryPartsFlusher() {
	pt.inmemoryPartsFlusherWG.Add(1)
	go func() {
		pt.inmemoryPartsFlusher()
		pt.inmemoryPartsFlusherWG.Done()
	}()
}

func (pt *partition) inmemoryPartsFlusher() {
	ticker := time.NewTicker(inmemoryPartsFlushInterval)
	defer ticker.Stop()
	var pwsBuf []*partWrapper
	var err error
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pwsBuf, err = pt.flushInmemoryParts(pwsBuf[:0], false)
			if err != nil {
				logger.Panicf("FATAL: cannot flush inmemory parts: %s", err)
			}
		}
	}
}

func (pt *partition) flushInmemoryParts(dstPws []*partWrapper, force bool) ([]*partWrapper, error) {
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(inmemoryPartsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}

	// Inmemory parts may present only in small parts.
	pt.partsLock.Lock()
	for _, pw := range pt.smallParts {
		if pw.mp == nil || pw.isInMerge {
			continue
		}
		if force || currentTime-pw.mp.creationTime >= uint64(flushSeconds) {
			pw.isInMerge = true
			dstPws = append(dstPws, pw)
		}
	}
	pt.partsLock.Unlock()

	if err := pt.mergePartsOptimal(dstPws, nil); err != nil {
		return dstPws, fmt.Errorf("cannot merge %d inmemory parts: %w", len(dstPws), err)
	}
	return dstPws, nil
}

func (pt *partition) mergePartsOptimal(pws []*partWrapper, stopCh <-chan struct{}) error {
	defer func() {
		// Remove isInMerge flag from pws.
		pt.partsLock.Lock()
		for _, pw := range pws {
			// Do not check for pws.isInMerge set to false,
			// since it may be set to false in mergeParts below.
			pw.isInMerge = false
		}
		pt.partsLock.Unlock()
	}()
	for len(pws) > defaultPartsToMerge {
		if err := pt.mergeParts(pws[:defaultPartsToMerge], stopCh); err != nil {
			return fmt.Errorf("cannot merge %d parts: %w", defaultPartsToMerge, err)
		}
		pws = pws[defaultPartsToMerge:]
	}
	if len(pws) == 0 {
		return nil
	}
	if err := pt.mergeParts(pws, stopCh); err != nil {
		return fmt.Errorf("cannot merge %d parts: %w", len(pws), err)
	}
	return nil
}

// ForceMergeAllParts runs merge for all the parts in pt - small and big.
func (pt *partition) ForceMergeAllParts() error {
	var pws []*partWrapper
	pt.partsLock.Lock()
	if !hasActiveMerges(pt.smallParts) && !hasActiveMerges(pt.bigParts) {
		pws = appendAllPartsToMerge(pws, pt.smallParts)
		pws = appendAllPartsToMerge(pws, pt.bigParts)
	}
	pt.partsLock.Unlock()

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
		return nil
	}

	// If len(pws) == 1, then the merge must run anyway. This allows removing the deleted series and performing de-duplication if needed.
	if err := pt.mergePartsOptimal(pws, pt.stopCh); err != nil {
		return fmt.Errorf("cannot force merge %d parts from partition %q: %w", len(pws), pt.name, err)
	}
	return nil
}

var forceMergeLogger = logger.WithThrottler("forceMerge", time.Minute)

func appendAllPartsToMerge(dst, src []*partWrapper) []*partWrapper {
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

var (
	bigMergeWorkersCount   = getDefaultMergeConcurrency(4)
	smallMergeWorkersCount = getDefaultMergeConcurrency(8)
)

func getDefaultMergeConcurrency(max int) int {
	v := (cgroup.AvailableCPUs() + 1) / 2
	if v > max {
		v = max
	}
	return v
}

// SetBigMergeWorkersCount sets the maximum number of concurrent mergers for big blocks.
//
// The function must be called before opening or creating any storage.
func SetBigMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	bigMergeWorkersCount = n
}

// SetSmallMergeWorkersCount sets the maximum number of concurrent mergers for small blocks.
//
// The function must be called before opening or creating any storage.
func SetSmallMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	smallMergeWorkersCount = n
}

func (pt *partition) startMergeWorkers() {
	for i := 0; i < smallMergeWorkersCount; i++ {
		pt.smallPartsMergerWG.Add(1)
		go func() {
			pt.smallPartsMerger()
			pt.smallPartsMergerWG.Done()
		}()
	}
	for i := 0; i < bigMergeWorkersCount; i++ {
		pt.bigPartsMergerWG.Add(1)
		go func() {
			pt.bigPartsMerger()
			pt.bigPartsMergerWG.Done()
		}()
	}
}

func (pt *partition) bigPartsMerger() {
	if err := pt.partsMerger(pt.mergeBigParts); err != nil {
		logger.Panicf("FATAL: unrecoverable error when merging big parts in the partition %q: %s", pt.bigPartsPath, err)
	}
}

func (pt *partition) smallPartsMerger() {
	if err := pt.partsMerger(pt.mergeSmallParts); err != nil {
		logger.Panicf("FATAL: unrecoverable error when merging small parts in the partition %q: %s", pt.smallPartsPath, err)
	}
}

const (
	minMergeSleepTime = 10 * time.Millisecond
	maxMergeSleepTime = 10 * time.Second
)

func (pt *partition) partsMerger(mergerFunc func(isFinal bool) error) error {
	sleepTime := minMergeSleepTime
	var lastMergeTime uint64
	isFinal := false
	t := time.NewTimer(sleepTime)
	for {
		err := mergerFunc(isFinal)
		if err == nil {
			// Try merging additional parts.
			sleepTime = minMergeSleepTime
			lastMergeTime = fasttime.UnixTimestamp()
			isFinal = false
			continue
		}
		if errors.Is(err, errForciblyStopped) {
			// The merger has been stopped.
			return nil
		}
		if !errors.Is(err, errNothingToMerge) {
			return err
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
		case <-pt.stopCh:
			return nil
		case <-t.C:
			t.Reset(sleepTime)
		}
	}
}

// Disable final merge by default, since it may lead to high disk IO and CPU usage
// at the beginning of every month when merging data for the previous month.
var finalMergeDelaySeconds = uint64(0)

// SetFinalMergeDelay sets the delay before doing final merge for partitions without newly ingested data.
//
// This function may be called only before Storage initialization.
func SetFinalMergeDelay(delay time.Duration) {
	if delay <= 0 {
		return
	}
	finalMergeDelaySeconds = uint64(delay.Seconds() + 1)
}

func getMaxOutBytes(path string, workersCount int) uint64 {
	n := fs.MustGetFreeSpace(path)
	// Do not substract freeDiskSpaceLimitBytes from n before calculating the maxOutBytes,
	// since this will result in sub-optimal merges - e.g. many small parts will be left unmerged.

	// Divide free space by the max number concurrent merges.
	maxOutBytes := n / uint64(workersCount)
	if maxOutBytes > maxBigPartSize {
		maxOutBytes = maxBigPartSize
	}
	return maxOutBytes
}

func (pt *partition) canBackgroundMerge() bool {
	return atomic.LoadUint32(pt.isReadOnly) == 0
}

func (pt *partition) mergeBigParts(isFinal bool) error {
	if !pt.canBackgroundMerge() {
		// Do not perform merge in read-only mode, since this may result in disk space shortage.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2603
		return nil
	}
	maxOutBytes := getMaxOutBytes(pt.bigPartsPath, bigMergeWorkersCount)

	pt.partsLock.Lock()
	pws, needFreeSpace := getPartsToMerge(pt.bigParts, maxOutBytes, isFinal)
	pt.partsLock.Unlock()

	atomicSetBool(&pt.bigMergeNeedFreeDiskSpace, needFreeSpace)
	return pt.mergeParts(pws, pt.stopCh)
}

func (pt *partition) mergeSmallParts(isFinal bool) error {
	if !pt.canBackgroundMerge() {
		// Do not perform merge in read-only mode, since this may result in disk space shortage.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2603
		return nil
	}
	// Try merging small parts to a big part at first.
	maxBigPartOutBytes := getMaxOutBytes(pt.bigPartsPath, bigMergeWorkersCount)
	pt.partsLock.Lock()
	pws, needFreeSpace := getPartsToMerge(pt.smallParts, maxBigPartOutBytes, isFinal)
	pt.partsLock.Unlock()
	atomicSetBool(&pt.bigMergeNeedFreeDiskSpace, needFreeSpace)

	outSize := getPartsSize(pws)
	if outSize > maxSmallPartSize() {
		// Merge small parts to a big part.
		return pt.mergeParts(pws, pt.stopCh)
	}

	// Make sure that the output small part fits small parts storage.
	maxSmallPartOutBytes := getMaxOutBytes(pt.smallPartsPath, smallMergeWorkersCount)
	if outSize <= maxSmallPartOutBytes {
		// Merge small parts to a small part.
		return pt.mergeParts(pws, pt.stopCh)
	}

	// The output small part doesn't fit small parts storage. Try merging small parts according to maxSmallPartOutBytes limit.
	pt.releasePartsToMerge(pws)
	pt.partsLock.Lock()
	pws, needFreeSpace = getPartsToMerge(pt.smallParts, maxSmallPartOutBytes, isFinal)
	pt.partsLock.Unlock()
	atomicSetBool(&pt.smallMergeNeedFreeDiskSpace, needFreeSpace)

	return pt.mergeParts(pws, pt.stopCh)
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

var errNothingToMerge = fmt.Errorf("nothing to merge")

func atomicSetBool(p *uint64, b bool) {
	v := uint64(0)
	if b {
		v = 1
	}
	atomic.StoreUint64(p, v)
}

func (pt *partition) runFinalDedup() error {
	requiredDedupInterval, actualDedupInterval := pt.getRequiredDedupInterval()
	if requiredDedupInterval <= actualDedupInterval {
		// Deduplication isn't needed.
		return nil
	}
	t := time.Now()
	logger.Infof("starting final dedup for partition %s using requiredDedupInterval=%d ms, since the partition has smaller actualDedupInterval=%d ms",
		pt.bigPartsPath, requiredDedupInterval, actualDedupInterval)
	if err := pt.ForceMergeAllParts(); err != nil {
		return fmt.Errorf("cannot perform final dedup for partition %s: %w", pt.bigPartsPath, err)
	}
	logger.Infof("final dedup for partition %s has been finished in %.3f seconds", pt.bigPartsPath, time.Since(t).Seconds())
	return nil
}

func (pt *partition) getRequiredDedupInterval() (int64, int64) {
	pws := pt.GetParts(nil)
	defer pt.PutParts(pws)
	dedupInterval := GetDedupInterval()
	minDedupInterval := getMinDedupInterval(pws)
	return dedupInterval, minDedupInterval
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

// mergeParts merges pws.
//
// Merging is immediately stopped if stopCh is closed.
//
// All the parts inside pws must have isInMerge field set to true.
func (pt *partition) mergeParts(pws []*partWrapper, stopCh <-chan struct{}) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
	}
	defer pt.releasePartsToMerge(pws)

	startTime := time.Now()

	// Prepare BlockStreamReaders for source parts.
	bsrs := make([]*blockStreamReader, 0, len(pws))
	defer func() {
		for _, bsr := range bsrs {
			putBlockStreamReader(bsr)
		}
	}()
	for _, pw := range pws {
		bsr := getBlockStreamReader()
		if pw.mp != nil {
			bsr.InitFromInmemoryPart(pw.mp)
		} else {
			if err := bsr.InitFromFilePart(pw.p.path); err != nil {
				return fmt.Errorf("cannot open source part for merging: %w", err)
			}
		}
		bsrs = append(bsrs, bsr)
	}

	outSize := uint64(0)
	outRowsCount := uint64(0)
	outBlocksCount := uint64(0)
	for _, pw := range pws {
		outSize += pw.p.size
		outRowsCount += pw.p.ph.RowsCount
		outBlocksCount += pw.p.ph.BlocksCount
	}
	isBigPart := outSize > maxSmallPartSize()
	nocache := isBigPart

	// Prepare BlockStreamWriter for destination part.
	ptPath := pt.smallPartsPath
	if isBigPart {
		ptPath = pt.bigPartsPath
	}
	ptPath = filepath.Clean(ptPath)
	mergeIdx := pt.nextMergeIdx()
	tmpPartPath := fmt.Sprintf("%s/tmp/%016X", ptPath, mergeIdx)
	bsw := getBlockStreamWriter()
	compressLevel := getCompressLevelForRowsCount(outRowsCount, outBlocksCount)
	if err := bsw.InitFromFilePart(tmpPartPath, nocache, compressLevel); err != nil {
		return fmt.Errorf("cannot create destination part %q: %w", tmpPartPath, err)
	}

	// Merge parts.
	dmis := pt.getDeletedMetricIDs()
	var ph partHeader
	rowsMerged := &pt.smallRowsMerged
	rowsDeleted := &pt.smallRowsDeleted
	if isBigPart {
		rowsMerged = &pt.bigRowsMerged
		rowsDeleted = &pt.bigRowsDeleted
		atomic.AddUint64(&pt.bigMergesCount, 1)
		atomic.AddUint64(&pt.activeBigMerges, 1)
	} else {
		atomic.AddUint64(&pt.smallMergesCount, 1)
		atomic.AddUint64(&pt.activeSmallMerges, 1)
	}
	retentionDeadline := timestampFromTime(startTime) - pt.retentionMsecs
	err := mergeBlockStreams(&ph, bsw, bsrs, stopCh, dmis, retentionDeadline, rowsMerged, rowsDeleted)
	if isBigPart {
		atomic.AddUint64(&pt.activeBigMerges, ^uint64(0))
	} else {
		atomic.AddUint64(&pt.activeSmallMerges, ^uint64(0))
	}
	putBlockStreamWriter(bsw)
	if err != nil {
		return fmt.Errorf("error when merging parts to %q: %w", tmpPartPath, err)
	}

	// Close bsrs.
	for _, bsr := range bsrs {
		putBlockStreamReader(bsr)
	}
	bsrs = nil

	ph.MinDedupInterval = GetDedupInterval()
	if err := ph.writeMinDedupInterval(tmpPartPath); err != nil {
		return fmt.Errorf("cannot store min dedup interval for part %q: %w", tmpPartPath, err)
	}

	// Create a transaction for atomic deleting old parts and moving
	// new part to its destination place.
	var bb bytesutil.ByteBuffer
	for _, pw := range pws {
		if pw.mp == nil {
			fmt.Fprintf(&bb, "%s\n", pw.p.path)
		}
	}
	dstPartPath := ""
	if ph.RowsCount > 0 {
		// The destination part may have no rows if they are deleted
		// during the merge due to dmis.
		dstPartPath = ph.Path(ptPath, mergeIdx)
	}
	fmt.Fprintf(&bb, "%s -> %s\n", tmpPartPath, dstPartPath)
	txnPath := fmt.Sprintf("%s/txn/%016X", ptPath, mergeIdx)
	if err := fs.WriteFileAtomically(txnPath, bb.B); err != nil {
		return fmt.Errorf("cannot create transaction file %q: %w", txnPath, err)
	}

	// Run the created transaction.
	if err := runTransaction(&pt.snapshotLock, pt.smallPartsPath, pt.bigPartsPath, txnPath); err != nil {
		return fmt.Errorf("cannot execute transaction %q: %w", txnPath, err)
	}

	var newPW *partWrapper
	var newPSize uint64
	if len(dstPartPath) > 0 {
		// Open the merged part if it is non-empty.
		newP, err := openFilePart(dstPartPath)
		if err != nil {
			return fmt.Errorf("cannot open merged part %q: %w", dstPartPath, err)
		}
		newPSize = newP.size
		newPW = &partWrapper{
			p:        newP,
			refCount: 1,
		}
	}

	// Atomically remove old parts and add new part.
	m := make(map[*partWrapper]bool, len(pws))
	for _, pw := range pws {
		m[pw] = true
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found in the merge of %d parts", len(pws)-len(m), len(pws))
	}
	removedSmallParts := 0
	removedBigParts := 0
	pt.partsLock.Lock()
	pt.smallParts, removedSmallParts = removeParts(pt.smallParts, m, false)
	pt.bigParts, removedBigParts = removeParts(pt.bigParts, m, true)
	if newPW != nil {
		if isBigPart {
			pt.bigParts = append(pt.bigParts, newPW)
		} else {
			pt.smallParts = append(pt.smallParts, newPW)
		}
	}
	pt.partsLock.Unlock()
	if removedSmallParts+removedBigParts != len(m) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedSmallParts+removedBigParts, len(m))
	}

	// Remove partition references from old parts.
	for _, pw := range pws {
		pw.decRef()
	}

	d := time.Since(startTime)
	if d > 30*time.Second {
		logger.Infof("merged %d rows across %d blocks in %.3f seconds at %d rows/sec to %q; sizeBytes: %d",
			outRowsCount, outBlocksCount, d.Seconds(), int(float64(outRowsCount)/d.Seconds()), dstPartPath, newPSize)
	}

	return nil
}

func getCompressLevelForRowsCount(rowsCount, blocksCount uint64) int {
	avgRowsPerBlock := rowsCount / blocksCount
	// See https://github.com/facebook/zstd/releases/tag/v1.3.4 about negative compression levels.
	if avgRowsPerBlock <= 10 {
		return -5
	}
	if avgRowsPerBlock <= 50 {
		return -2
	}
	if avgRowsPerBlock <= 200 {
		return -1
	}
	if avgRowsPerBlock <= 500 {
		return 1
	}
	if avgRowsPerBlock <= 1000 {
		return 2
	}
	if avgRowsPerBlock <= 2000 {
		return 3
	}
	if avgRowsPerBlock <= 4000 {
		return 4
	}
	return 5
}

func (pt *partition) nextMergeIdx() uint64 {
	return atomic.AddUint64(&pt.mergeIdx, 1)
}

func removeParts(pws []*partWrapper, partsToRemove map[*partWrapper]bool, isBig bool) ([]*partWrapper, int) {
	removedParts := 0
	dst := pws[:0]
	for _, pw := range pws {
		if !partsToRemove[pw] {
			dst = append(dst, pw)
			continue
		}
		removedParts++
	}
	return dst, removedParts
}

func (pt *partition) startStalePartsRemover() {
	pt.stalePartsRemoverWG.Add(1)
	go func() {
		pt.stalePartsRemover()
		pt.stalePartsRemoverWG.Done()
	}()
}

func (pt *partition) stalePartsRemover() {
	ticker := time.NewTicker(7 * time.Minute)
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
	m := make(map[*partWrapper]bool)
	startTime := time.Now()
	retentionDeadline := timestampFromTime(startTime) - pt.retentionMsecs

	pt.partsLock.Lock()
	for _, pw := range pt.bigParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.bigRowsDeleted, pw.p.ph.RowsCount)
			m[pw] = true
		}
	}
	for _, pw := range pt.smallParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.smallRowsDeleted, pw.p.ph.RowsCount)
			m[pw] = true
		}
	}
	removedSmallParts := 0
	removedBigParts := 0
	if len(m) > 0 {
		pt.smallParts, removedSmallParts = removeParts(pt.smallParts, m, false)
		pt.bigParts, removedBigParts = removeParts(pt.bigParts, m, true)
	}
	pt.partsLock.Unlock()

	if removedSmallParts+removedBigParts != len(m) {
		logger.Panicf("BUG: unexpected number of stale parts removed; got %d, want %d", removedSmallParts+removedBigParts, len(m))
	}

	// Physically remove stale parts under snapshotLock in order to provide
	// consistent snapshots with partition.CreateSnapshot().
	pt.snapshotLock.RLock()
	var removeWG sync.WaitGroup
	for pw := range m {
		logger.Infof("removing part %q, since its data is out of the configured retention (%d secs)", pw.p.path, pt.retentionMsecs/1000)
		removeWG.Add(1)
		fs.MustRemoveAllWithDoneCallback(pw.p.path, removeWG.Done)
	}
	removeWG.Wait()
	// There is no need in calling fs.MustSyncPath() on pt.smallPartsPath and pt.bigPartsPath,
	// since they should be automatically called inside fs.MustRemoveAllWithDoneCallback.

	pt.snapshotLock.RUnlock()

	// Remove partition references from removed parts.
	for pw := range m {
		pw.decRef()
	}

}

// getPartsToMerge returns optimal parts to merge from pws.
//
// The summary size of the returned parts must be smaller than maxOutBytes.
// The function returns true if pws contains parts, which cannot be merged because of maxOutBytes limit.
func getPartsToMerge(pws []*partWrapper, maxOutBytes uint64, isFinal bool) ([]*partWrapper, bool) {
	pwsRemaining := make([]*partWrapper, 0, len(pws))
	for _, pw := range pws {
		if !pw.isInMerge {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}
	maxPartsToMerge := defaultPartsToMerge
	var pms []*partWrapper
	needFreeSpace := false
	if isFinal {
		for len(pms) == 0 && maxPartsToMerge >= finalPartsToMerge {
			pms, needFreeSpace = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
			maxPartsToMerge--
		}
	} else {
		pms, needFreeSpace = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
	}
	for _, pw := range pms {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge cannot be set")
		}
		pw.isInMerge = true
	}
	return pms, needFreeSpace
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
// The function returns true if src contains parts, which cannot be merged because of maxOutBytes limit.
func appendPartsToMerge(dst, src []*partWrapper, maxPartsToMerge int, maxOutBytes uint64) ([]*partWrapper, bool) {
	if len(src) < 2 {
		// There is no need in merging zero or one part :)
		return dst, false
	}
	if maxPartsToMerge < 2 {
		logger.Panicf("BUG: maxPartsToMerge cannot be smaller than 2; got %d", maxPartsToMerge)
	}

	// Filter out too big parts.
	// This should reduce N for O(N^2) algorithm below.
	skippedBigParts := 0
	maxInPartBytes := uint64(float64(maxOutBytes) / minMergeMultiplier)
	tmp := make([]*partWrapper, 0, len(src))
	for _, pw := range src {
		if pw.p.size > maxInPartBytes {
			skippedBigParts++
			continue
		}
		tmp = append(tmp, pw)
	}
	src = tmp
	needFreeSpace := skippedBigParts > 1

	// Sort src parts by size and backwards timestamp.
	// This should improve adjanced points' locality in the merged parts.
	sort.Slice(src, func(i, j int) bool {
		a := src[i].p
		b := src[j].p
		if a.size == b.size {
			return a.ph.MinTimestamp > b.ph.MinTimestamp
		}
		return a.size < b.size
	})

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
			outSize := getPartsSize(a)
			if outSize > maxOutBytes {
				needFreeSpace = true
			}
			if a[0].p.size*uint64(len(a)) < a[len(a)-1].p.size {
				// Do not merge parts with too big difference in size,
				// since this results in unbalanced merges.
				continue
			}
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
		return dst, needFreeSpace
	}
	return append(dst, pws...), needFreeSpace
}

func getPartsSize(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.size
	}
	return n
}

func openParts(pathPrefix1, pathPrefix2, path string) ([]*partWrapper, error) {
	// The path can be missing after restoring from backup, so create it if needed.
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, err
	}
	d, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open directory %q: %w", path, err)
	}
	defer fs.MustClose(d)

	// Run remaining transactions and cleanup /txn and /tmp directories.
	// Snapshots cannot be created yet, so use fakeSnapshotLock.
	var fakeSnapshotLock sync.RWMutex
	if err := runTransactions(&fakeSnapshotLock, pathPrefix1, pathPrefix2, path); err != nil {
		return nil, fmt.Errorf("cannot run transactions from %q: %w", path, err)
	}

	txnDir := path + "/txn"
	fs.MustRemoveAll(txnDir)
	tmpDir := path + "/tmp"
	fs.MustRemoveAll(tmpDir)
	if err := createPartitionDirs(path); err != nil {
		return nil, fmt.Errorf("cannot create directories for partition %q: %w", path, err)
	}

	// Open parts.
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %q: %w", d.Name(), err)
	}
	var pws []*partWrapper
	for _, fi := range fis {
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories.
			continue
		}
		fn := fi.Name()
		if fn == "tmp" || fn == "txn" || fn == "snapshots" {
			// "snapshots" dir is skipped for backwards compatibility. Now it is unused.
			// Skip special dirs.
			continue
		}
		partPath := path + "/" + fn
		if fs.IsEmptyDir(partPath) {
			// Remove empty directory, which can be left after unclean shutdown on NFS.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1142
			fs.MustRemoveAll(partPath)
			continue
		}
		startTime := time.Now()
		p, err := openFilePart(partPath)
		if err != nil {
			mustCloseParts(pws)
			return nil, fmt.Errorf("cannot open part %q: %w", partPath, err)
		}
		logger.Infof("opened part %q in %.3f seconds", partPath, time.Since(startTime).Seconds())

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
			logger.Panicf("BUG: unexpected refCount when closing part %q: %d; want 1", &pw.p.ph, pw.refCount)
		}
		pw.p.MustClose()
	}
}

// CreateSnapshotAt creates pt snapshot at the given smallPath and bigPath dirs.
//
// Snapshot is created using linux hard links, so it is usually created
// very quickly.
func (pt *partition) CreateSnapshotAt(smallPath, bigPath string) error {
	logger.Infof("creating partition snapshot of %q and %q...", pt.smallPartsPath, pt.bigPartsPath)
	startTime := time.Now()

	// Flush inmemory data to disk.
	pt.flushRawRows(true)
	if _, err := pt.flushInmemoryParts(nil, true); err != nil {
		return fmt.Errorf("cannot flush inmemory parts: %w", err)
	}

	// The snapshot must be created under the lock in order to prevent from
	// concurrent modifications via runTransaction.
	pt.snapshotLock.Lock()
	defer pt.snapshotLock.Unlock()

	if err := pt.createSnapshot(pt.smallPartsPath, smallPath); err != nil {
		return fmt.Errorf("cannot create snapshot for %q: %w", pt.smallPartsPath, err)
	}
	if err := pt.createSnapshot(pt.bigPartsPath, bigPath); err != nil {
		return fmt.Errorf("cannot create snapshot for %q: %w", pt.bigPartsPath, err)
	}

	logger.Infof("created partition snapshot of %q and %q at %q and %q in %.3f seconds",
		pt.smallPartsPath, pt.bigPartsPath, smallPath, bigPath, time.Since(startTime).Seconds())
	return nil
}

func (pt *partition) createSnapshot(srcDir, dstDir string) error {
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
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories.
			continue
		}
		fn := fi.Name()
		if fn == "tmp" || fn == "txn" {
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
	fs.MustSyncPath(filepath.Dir(dstDir))

	return nil
}

func runTransactions(txnLock *sync.RWMutex, pathPrefix1, pathPrefix2, path string) error {
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

	// Sort transaction files by id.
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
		if err := runTransaction(txnLock, pathPrefix1, pathPrefix2, txnPath); err != nil {
			return fmt.Errorf("cannot run transaction from %q: %w", txnPath, err)
		}
	}
	return nil
}

func runTransaction(txnLock *sync.RWMutex, pathPrefix1, pathPrefix2, txnPath string) error {
	// The transaction must run under read lock in order to provide
	// consistent snapshots with partition.CreateSnapshot().
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
		path, err := validatePath(pathPrefix1, pathPrefix2, path)
		if err != nil {
			return fmt.Errorf("invalid path to remove: %w", err)
		}
		removeWG.Add(1)
		fs.MustRemoveAllWithDoneCallback(path, removeWG.Done)
	}

	// Move the new part to new directory.
	srcPath := mvPaths[0]
	dstPath := mvPaths[1]
	srcPath, err = validatePath(pathPrefix1, pathPrefix2, srcPath)
	if err != nil {
		return fmt.Errorf("invalid source path to rename: %w", err)
	}
	if len(dstPath) > 0 {
		// Move srcPath to dstPath.
		dstPath, err = validatePath(pathPrefix1, pathPrefix2, dstPath)
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
	} else {
		// Just remove srcPath.
		removeWG.Add(1)
		fs.MustRemoveAllWithDoneCallback(srcPath, removeWG.Done)
	}

	// Flush pathPrefix* directory metadata to the underying storage,
	// so the moved files become visible there.
	fs.MustSyncPath(pathPrefix1)
	fs.MustSyncPath(pathPrefix2)

	pendingTxnDeletionsWG.Add(1)
	go func() {
		defer pendingTxnDeletionsWG.Done()
		// Remove the transaction file only after all the source paths are deleted.
		// This is required for NFS mounts. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
		removeWG.Wait()

		// There is no need in calling fs.MustSyncPath for pathPrefix* after parts' removal,
		// since it is already called by fs.MustRemoveAllWithDoneCallback.

		if err := os.Remove(txnPath); err != nil {
			logger.Errorf("cannot remove transaction file %q: %s", txnPath, err)
		}
	}()

	return nil
}

var pendingTxnDeletionsWG syncwg.WaitGroup

func validatePath(pathPrefix1, pathPrefix2, path string) (string, error) {
	var err error

	pathPrefix1, err = filepath.Abs(pathPrefix1)
	if err != nil {
		return path, fmt.Errorf("cannot determine absolute path for pathPrefix1=%q: %w", pathPrefix1, err)
	}
	pathPrefix2, err = filepath.Abs(pathPrefix2)
	if err != nil {
		return path, fmt.Errorf("cannot determine absolute path for pathPrefix2=%q: %w", pathPrefix2, err)
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return path, fmt.Errorf("cannot determine absolute path for %q: %w", path, err)
	}
	if !strings.HasPrefix(path, pathPrefix1+"/") && !strings.HasPrefix(path, pathPrefix2+"/") {
		return path, fmt.Errorf("invalid path %q; must start with either %q or %q", path, pathPrefix1+"/", pathPrefix2+"/")
	}
	return path, nil
}

func createPartitionDirs(path string) error {
	path = filepath.Clean(path)
	txnPath := path + "/txn"
	if err := fs.MkdirAllFailIfExist(txnPath); err != nil {
		return fmt.Errorf("cannot create txn directory %q: %w", txnPath, err)
	}
	tmpPath := path + "/tmp"
	if err := fs.MkdirAllFailIfExist(tmpPath); err != nil {
		return fmt.Errorf("cannot create tmp directory %q: %w", tmpPath, err)
	}
	fs.MustSyncPath(path)
	return nil
}
