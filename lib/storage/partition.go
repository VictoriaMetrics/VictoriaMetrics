package storage

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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storagepacelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
)

// The maximum size of big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxBigPartSize = 1e12

// The maximum number of inmemory parts in the partition.
//
// If the number of inmemory parts reaches this value, then assisted merge runs during data ingestion.
const maxInmemoryPartsPerPartition = 32

// The maximum number of small parts in the partition.
//
// If the number of small parts reaches this value, then assisted merge runs during data ingestion.
const maxSmallPartsPerPartition = 64

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
var rawRowsShardsPerPartition = (cgroup.AvailableCPUs() + 1) / 2

// The interval for flushing bufferred rows into parts, so they become visible to search.
const pendingRowsFlushInterval = time.Second

// The interval for guaranteed flush of recently ingested data from memory to on-disk parts,
// so they survive process crash.
var dataFlushInterval = 5 * time.Second

// SetDataFlushInterval sets the interval for guaranteed flush of recently ingested data from memory to disk.
//
// The data can be flushed from memory to disk more frequently if it doesn't fit the memory limit.
//
// This function must be called before initializing the storage.
func SetDataFlushInterval(d time.Duration) {
	if d > pendingRowsFlushInterval {
		dataFlushInterval = d
		mergeset.SetDataFlushInterval(d)
	}
}

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

// partition represents a partition.
type partition struct {
	// Put atomic counters to the top of struct, so they are aligned to 8 bytes on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	activeInmemoryMerges uint64
	activeSmallMerges    uint64
	activeBigMerges      uint64

	inmemoryMergesCount uint64
	smallMergesCount    uint64
	bigMergesCount      uint64

	inmemoryRowsMerged uint64
	smallRowsMerged    uint64
	bigRowsMerged      uint64

	inmemoryRowsDeleted uint64
	smallRowsDeleted    uint64
	bigRowsDeleted      uint64

	inmemoryAssistedMerges uint64
	smallAssistedMerges    uint64

	mergeNeedFreeDiskSpace uint64

	mergeIdx uint64

	smallPartsPath string
	bigPartsPath   string

	// The parent storage.
	s *Storage

	// Name is the name of the partition in the form YYYY_MM.
	name string

	// The time range for the partition. Usually this is a whole month.
	tr TimeRange

	// rawRows contains recently added rows that haven't been converted into parts yet.
	// rawRows are periodically converted into inmemroyParts.
	// rawRows aren't used in search for performance reasons.
	rawRows rawRowsShards

	// partsLock protects inmemoryParts, smallParts and bigParts.
	partsLock sync.Mutex

	// Contains inmemory parts with recently ingested data.
	// It must be merged into either smallParts or bigParts to become visible to search.
	inmemoryParts []*partWrapper

	// Contains file-based parts with small number of items.
	smallParts []*partWrapper

	// Contains file-based parts with big number of items.
	bigParts []*partWrapper

	snapshotLock sync.RWMutex

	stopCh chan struct{}

	wg sync.WaitGroup
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

	// The deadline when in-memory part must be flushed to disk.
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
		putInmemoryPart(pw.mp)
		pw.mp = nil
	}
	pw.p.MustClose()
	pw.p = nil
}

// createPartition creates new partition for the given timestamp and the given paths
// to small and big partitions.
func createPartition(timestamp int64, smallPartitionsPath, bigPartitionsPath string, s *Storage) (*partition, error) {
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

	pt := newPartition(name, smallPartsPath, bigPartsPath, s)
	pt.tr.fromPartitionTimestamp(timestamp)
	pt.startBackgroundWorkers()

	logger.Infof("partition %q has been created", name)

	return pt, nil
}

func (pt *partition) startBackgroundWorkers() {
	pt.startMergeWorkers()
	pt.startInmemoryPartsFlusher()
	pt.startPendingRowsFlusher()
	pt.startStalePartsRemover()
}

// Drop drops all the data on the storage for the given pt.
//
// The pt must be detached from table before calling pt.Drop.
func (pt *partition) Drop() {
	logger.Infof("dropping partition %q at smallPartsPath=%q, bigPartsPath=%q", pt.name, pt.smallPartsPath, pt.bigPartsPath)
	// Wait until all the pending transaction deletions are finished before removing partition directories.
	pendingTxnDeletionsWG.Wait()

	fs.MustRemoveDirAtomic(pt.smallPartsPath)
	fs.MustRemoveDirAtomic(pt.bigPartsPath)
	logger.Infof("partition %q has been dropped", pt.name)
}

// openPartition opens the existing partition from the given paths.
func openPartition(smallPartsPath, bigPartsPath string, s *Storage) (*partition, error) {
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

	pt := newPartition(name, smallPartsPath, bigPartsPath, s)
	pt.smallParts = smallParts
	pt.bigParts = bigParts
	if err := pt.tr.fromPartitionName(name); err != nil {
		return nil, fmt.Errorf("cannot obtain partition time range from smallPartsPath %q: %w", smallPartsPath, err)
	}
	pt.startBackgroundWorkers()

	return pt, nil
}

func newPartition(name, smallPartsPath, bigPartsPath string, s *Storage) *partition {
	p := &partition{
		name:           name,
		smallPartsPath: smallPartsPath,
		bigPartsPath:   bigPartsPath,

		s: s,

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

	InmemoryAssistedMerges uint64
	SmallAssistedMerges    uint64

	MergeNeedFreeDiskSpace uint64
}

// TotalRowsCount returns total number of rows in tm.
func (pm *partitionMetrics) TotalRowsCount() uint64 {
	return pm.PendingRows + pm.InmemoryRowsCount + pm.SmallRowsCount + pm.BigRowsCount
}

// UpdateMetrics updates m with metrics from pt.
func (pt *partition) UpdateMetrics(m *partitionMetrics) {
	m.PendingRows += uint64(pt.rawRows.Len())

	pt.partsLock.Lock()

	for _, pw := range pt.inmemoryParts {
		p := pw.p
		m.InmemoryRowsCount += p.ph.RowsCount
		m.InmemoryBlocksCount += p.ph.BlocksCount
		m.InmemorySizeBytes += p.size
		m.InmemoryPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}
	for _, pw := range pt.smallParts {
		p := pw.p
		m.SmallRowsCount += p.ph.RowsCount
		m.SmallBlocksCount += p.ph.BlocksCount
		m.SmallSizeBytes += p.size
		m.SmallPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}
	for _, pw := range pt.bigParts {
		p := pw.p
		m.BigRowsCount += p.ph.RowsCount
		m.BigBlocksCount += p.ph.BlocksCount
		m.BigSizeBytes += p.size
		m.BigPartsRefCount += atomic.LoadUint64(&pw.refCount)
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

	m.ActiveInmemoryMerges += atomic.LoadUint64(&pt.activeInmemoryMerges)
	m.ActiveSmallMerges += atomic.LoadUint64(&pt.activeSmallMerges)
	m.ActiveBigMerges += atomic.LoadUint64(&pt.activeBigMerges)

	m.InmemoryMergesCount += atomic.LoadUint64(&pt.inmemoryMergesCount)
	m.SmallMergesCount += atomic.LoadUint64(&pt.smallMergesCount)
	m.BigMergesCount += atomic.LoadUint64(&pt.bigMergesCount)

	m.InmemoryRowsMerged += atomic.LoadUint64(&pt.inmemoryRowsMerged)
	m.SmallRowsMerged += atomic.LoadUint64(&pt.smallRowsMerged)
	m.BigRowsMerged += atomic.LoadUint64(&pt.bigRowsMerged)

	m.InmemoryRowsDeleted += atomic.LoadUint64(&pt.inmemoryRowsDeleted)
	m.SmallRowsDeleted += atomic.LoadUint64(&pt.smallRowsDeleted)
	m.BigRowsDeleted += atomic.LoadUint64(&pt.bigRowsDeleted)

	m.InmemoryAssistedMerges += atomic.LoadUint64(&pt.inmemoryAssistedMerges)
	m.SmallAssistedMerges += atomic.LoadUint64(&pt.smallAssistedMerges)

	m.MergeNeedFreeDiskSpace += atomic.LoadUint64(&pt.mergeNeedFreeDiskSpace)
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
	shards := rrss.shards
	shardsLen := uint32(len(shards))
	for len(rows) > 0 {
		n := atomic.AddUint32(&rrss.shardIdx, 1)
		idx := n % shardsLen
		rows = shards[idx].addRows(pt, rows)
	}
}

func (rrss *rawRowsShards) Len() int {
	n := 0
	for i := range rrss.shards[:] {
		n += rrss.shards[i].Len()
	}
	return n
}

type rawRowsShardNopad struct {
	// Put lastFlushTime to the top in order to avoid unaligned memory access on 32-bit architectures
	lastFlushTime uint64

	mu   sync.Mutex
	rows []rawRow
}

type rawRowsShard struct {
	rawRowsShardNopad

	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(rawRowsShardNopad{})%128]byte
}

func (rrs *rawRowsShard) Len() int {
	rrs.mu.Lock()
	n := len(rrs.rows)
	rrs.mu.Unlock()
	return n
}

func (rrs *rawRowsShard) addRows(pt *partition, rows []rawRow) []rawRow {
	var rrb *rawRowsBlock

	rrs.mu.Lock()
	if cap(rrs.rows) == 0 {
		rrs.rows = newRawRowsBlock()
	}
	n := copy(rrs.rows[len(rrs.rows):cap(rrs.rows)], rows)
	rrs.rows = rrs.rows[:len(rrs.rows)+n]
	rows = rows[n:]
	if len(rows) > 0 {
		rrb = getRawRowsBlock()
		rrb.rows, rrs.rows = rrs.rows, rrb.rows
		n = copy(rrs.rows[:cap(rrs.rows)], rows)
		rrs.rows = rrs.rows[:n]
		rows = rows[n:]
		atomic.StoreUint64(&rrs.lastFlushTime, fasttime.UnixTimestamp())
	}
	rrs.mu.Unlock()

	if rrb != nil {
		pt.flushRowsToParts(rrb.rows)
		putRawRowsBlock(rrb)
	}

	return rows
}

type rawRowsBlock struct {
	rows []rawRow
}

func newRawRowsBlock() []rawRow {
	n := getMaxRawRowsPerShard()
	return make([]rawRow, 0, n)
}

func getRawRowsBlock() *rawRowsBlock {
	v := rawRowsBlockPool.Get()
	if v == nil {
		return &rawRowsBlock{
			rows: newRawRowsBlock(),
		}
	}
	return v.(*rawRowsBlock)
}

func putRawRowsBlock(rrb *rawRowsBlock) {
	rrb.rows = rrb.rows[:0]
	rawRowsBlockPool.Put(rrb)
}

var rawRowsBlockPool sync.Pool

func (pt *partition) flushRowsToParts(rows []rawRow) {
	if len(rows) == 0 {
		return
	}
	maxRows := getMaxRawRowsPerShard()
	var pwsLock sync.Mutex
	pws := make([]*partWrapper, 0, (len(rows)+maxRows-1)/maxRows)
	wg := getWaitGroup()
	for len(rows) > 0 {
		n := maxRows
		if n > len(rows) {
			n = len(rows)
		}
		wg.Add(1)
		flushConcurrencyCh <- struct{}{}
		go func(rowsChunk []rawRow) {
			defer func() {
				<-flushConcurrencyCh
				wg.Done()
			}()
			pw := pt.createInmemoryPart(rowsChunk)
			if pw == nil {
				return
			}
			pwsLock.Lock()
			pws = append(pws, pw)
			pwsLock.Unlock()
		}(rows[:n])
		rows = rows[n:]
	}
	wg.Wait()
	putWaitGroup(wg)

	pt.partsLock.Lock()
	pt.inmemoryParts = append(pt.inmemoryParts, pws...)
	pt.partsLock.Unlock()

	flushConcurrencyCh <- struct{}{}
	pt.assistedMergeForInmemoryParts()
	pt.assistedMergeForSmallParts()
	<-flushConcurrencyCh
	// There is no need in assisted merges for small and big parts,
	// since the bottleneck is possible only at inmemory parts.
}

var flushConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

func (pt *partition) assistedMergeForInmemoryParts() {
	for {
		pt.partsLock.Lock()
		ok := getNotInMergePartsCount(pt.inmemoryParts) < maxInmemoryPartsPerPartition
		pt.partsLock.Unlock()
		if ok {
			return
		}

		// There are too many unmerged inmemory parts.
		// This usually means that the app cannot keep up with the data ingestion rate.
		// Assist with mering inmemory parts.
		// Prioritize assisted merges over searches.
		storagepacelimiter.Search.Inc()
		atomic.AddUint64(&pt.inmemoryAssistedMerges, 1)
		err := pt.mergeInmemoryParts()
		storagepacelimiter.Search.Dec()
		if err == nil {
			continue
		}
		if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) {
			return
		}
		logger.Panicf("FATAL: cannot merge inmemory parts: %s", err)
	}
}

func (pt *partition) assistedMergeForSmallParts() {
	for {
		pt.partsLock.Lock()
		ok := getNotInMergePartsCount(pt.smallParts) < maxSmallPartsPerPartition
		pt.partsLock.Unlock()
		if ok {
			return
		}

		// There are too many unmerged small parts.
		// This usually means that the app cannot keep up with the data ingestion rate.
		// Assist with mering small parts.
		// Prioritize assisted merges over searches.
		storagepacelimiter.Search.Inc()
		atomic.AddUint64(&pt.smallAssistedMerges, 1)
		err := pt.mergeExistingParts(false)
		storagepacelimiter.Search.Dec()
		if err == nil {
			continue
		}
		if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) || errors.Is(err, errReadOnlyMode) {
			return
		}
		logger.Panicf("FATAL: cannot merge small parts: %s", err)
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
	p, err := mp.NewPart()
	if err != nil {
		logger.Panicf("BUG: cannot create part from %q: %s", &mp.ph, err)
	}
	pw := &partWrapper{
		p:                   p,
		mp:                  mp,
		refCount:            1,
		flushToDiskDeadline: flushToDiskDeadline,
	}
	return pw
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
	for _, pw := range pt.inmemoryParts {
		pw.incRef()
	}
	dst = append(dst, pt.inmemoryParts...)
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

	logger.Infof("waiting for service workers to stop on %q...", pt.smallPartsPath)
	startTime := time.Now()
	pt.wg.Wait()
	logger.Infof("service workers stopped in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	logger.Infof("flushing inmemory parts to files on %q...", pt.smallPartsPath)
	startTime = time.Now()
	pt.flushInmemoryRows()
	logger.Infof("inmemory parts have been flushed to files in %.3f seconds on %q", time.Since(startTime).Seconds(), pt.smallPartsPath)

	// Remove references from inmemoryParts, smallParts and bigParts, so they may be eventually closed
	// after all the searches are done.
	pt.partsLock.Lock()
	inmemoryParts := pt.inmemoryParts
	smallParts := pt.smallParts
	bigParts := pt.bigParts
	pt.inmemoryParts = nil
	pt.smallParts = nil
	pt.bigParts = nil
	pt.partsLock.Unlock()

	for _, pw := range inmemoryParts {
		pw.decRef()
	}
	for _, pw := range smallParts {
		pw.decRef()
	}
	for _, pw := range bigParts {
		pw.decRef()
	}
}

func (pt *partition) startInmemoryPartsFlusher() {
	pt.wg.Add(1)
	go func() {
		pt.inmemoryPartsFlusher()
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

func (pt *partition) inmemoryPartsFlusher() {
	ticker := time.NewTicker(dataFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			pt.flushInmemoryParts(false)
		}
	}
}

func (pt *partition) pendingRowsFlusher() {
	ticker := time.NewTicker(pendingRowsFlushInterval)
	defer ticker.Stop()
	var rows []rawRow
	for {
		select {
		case <-pt.stopCh:
			return
		case <-ticker.C:
			rows = pt.flushPendingRows(rows[:0], false)
		}
	}
}

func (pt *partition) flushPendingRows(dst []rawRow, isFinal bool) []rawRow {
	return pt.rawRows.flush(pt, dst, isFinal)
}

func (pt *partition) flushInmemoryRows() {
	pt.rawRows.flush(pt, nil, true)
	pt.flushInmemoryParts(true)
}

func (pt *partition) flushInmemoryParts(isFinal bool) {
	for {
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

		if err := pt.mergePartsOptimal(pws, nil); err != nil {
			logger.Panicf("FATAL: cannot merge in-memory parts: %s", err)
		}
		if !isFinal {
			return
		}
		pt.partsLock.Lock()
		n := len(pt.inmemoryParts)
		pt.partsLock.Unlock()
		if n == 0 {
			// All the in-memory parts were flushed to disk.
			return
		}
		// Some parts weren't flushed to disk because they were being merged.
		// Sleep for a while and try flushing them again.
		time.Sleep(10 * time.Millisecond)
	}
}

func (rrss *rawRowsShards) flush(pt *partition, dst []rawRow, isFinal bool) []rawRow {
	for i := range rrss.shards {
		dst = rrss.shards[i].appendRawRowsToFlush(dst, pt, isFinal)
	}
	pt.flushRowsToParts(dst)
	return dst
}

func (rrs *rawRowsShard) appendRawRowsToFlush(dst []rawRow, pt *partition, isFinal bool) []rawRow {
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(pendingRowsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}
	lastFlushTime := atomic.LoadUint64(&rrs.lastFlushTime)
	if !isFinal && currentTime < lastFlushTime+uint64(flushSeconds) {
		// Fast path - nothing to flush
		return dst
	}
	// Slow path - move rrs.rows to dst.
	rrs.mu.Lock()
	dst = append(dst, rrs.rows...)
	rrs.rows = rrs.rows[:0]
	atomic.StoreUint64(&rrs.lastFlushTime, currentTime)
	rrs.mu.Unlock()
	return dst
}

func (pt *partition) mergePartsOptimal(pws []*partWrapper, stopCh <-chan struct{}) error {
	sortPartsForOptimalMerge(pws)
	for len(pws) > 0 {
		n := defaultPartsToMerge
		if n > len(pws) {
			n = len(pws)
		}
		pwsChunk := pws[:n]
		pws = pws[n:]
		err := pt.mergeParts(pwsChunk, stopCh, true)
		if err == nil {
			continue
		}
		pt.releasePartsToMerge(pws)
		if errors.Is(err, errForciblyStopped) {
			return nil
		}
		return fmt.Errorf("cannot merge parts optimally: %w", err)
	}
	return nil
}

// ForceMergeAllParts runs merge for all the parts in pt.
func (pt *partition) ForceMergeAllParts() error {
	pws := pt.getAllPartsForMerge()
	if len(pws) == 0 {
		// Nothing to merge.
		return nil
	}
	for {
		// Check whether there is enough disk space for merging pws.
		newPartSize := getPartsSize(pws)
		maxOutBytes := fs.MustGetFreeSpace(pt.bigPartsPath)
		if newPartSize > maxOutBytes {
			freeSpaceNeededBytes := newPartSize - maxOutBytes
			forceMergeLogger.Warnf("cannot initiate force merge for the partition %s; additional space needed: %d bytes", pt.name, freeSpaceNeededBytes)
			return nil
		}

		// If len(pws) == 1, then the merge must run anyway.
		// This allows applying the configured retention, removing the deleted series
		// and performing de-duplication if needed.
		if err := pt.mergePartsOptimal(pws, pt.stopCh); err != nil {
			return fmt.Errorf("cannot force merge %d parts from partition %q: %w", len(pws), pt.name, err)
		}
		pws = pt.getAllPartsForMerge()
		if len(pws) <= 1 {
			return nil
		}
	}
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

var mergeWorkersLimitCh = make(chan struct{}, getDefaultMergeConcurrency(16))

var bigMergeWorkersLimitCh = make(chan struct{}, getDefaultMergeConcurrency(4))

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
	bigMergeWorkersLimitCh = make(chan struct{}, n)
}

// SetMergeWorkersCount sets the maximum number of concurrent mergers for parts.
//
// The function must be called before opening or creating any storage.
func SetMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	mergeWorkersLimitCh = make(chan struct{}, n)
}

func (pt *partition) startMergeWorkers() {
	// Start a merge worker per available CPU core.
	// The actual number of concurrent merges is limited inside mergeWorker() below.
	workersCount := cgroup.AvailableCPUs()
	for i := 0; i < workersCount; i++ {
		pt.wg.Add(1)
		go func() {
			pt.mergeWorker()
			pt.wg.Done()
		}()
	}
}

const (
	minMergeSleepTime = 10 * time.Millisecond
	maxMergeSleepTime = 10 * time.Second
)

func (pt *partition) mergeWorker() {
	sleepTime := minMergeSleepTime
	var lastMergeTime uint64
	isFinal := false
	t := time.NewTimer(sleepTime)
	for {
		// Limit the number of concurrent calls to mergeExistingParts, since the total number of merge workers
		// across partitions may exceed the the cap(mergeWorkersLimitCh).
		mergeWorkersLimitCh <- struct{}{}
		err := pt.mergeExistingParts(isFinal)
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
			logger.Panicf("FATAL: unrecoverable error when merging parts in the partition (%q, %q): %s", pt.smallPartsPath, pt.bigPartsPath, err)
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
			return
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
	mergeset.SetFinalMergeDelay(delay)
}

func getMaxInmemoryPartSize() uint64 {
	// Allocate 10% of allowed memory for in-memory parts.
	n := uint64(0.1 * float64(memory.Allowed()) / maxInmemoryPartsPerPartition)
	if n < 1e6 {
		n = 1e6
	}
	return n
}

func (pt *partition) getMaxSmallPartSize() uint64 {
	// Small parts are cached in the OS page cache,
	// so limit their size by the remaining free RAM.
	mem := memory.Remaining()
	// It is expected no more than defaultPartsToMerge/2 parts exist
	// in the OS page cache before they are merged into bigger part.
	// Half of the remaining RAM must be left for lib/mergeset parts,
	// so the maxItems is calculated using the below code:
	n := uint64(mem) / defaultPartsToMerge
	if n < 10e6 {
		n = 10e6
	}
	// Make sure the output part fits available disk space for small parts.
	sizeLimit := getMaxOutBytes(pt.smallPartsPath, cap(mergeWorkersLimitCh))
	if n > sizeLimit {
		n = sizeLimit
	}
	return n
}

func (pt *partition) getMaxBigPartSize() uint64 {
	return getMaxOutBytes(pt.bigPartsPath, cap(bigMergeWorkersLimitCh))
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

func (pt *partition) canBackgroundMerge() bool {
	return atomic.LoadUint32(&pt.s.isReadOnly) == 0
}

var errReadOnlyMode = fmt.Errorf("storage is in readonly mode")

func (pt *partition) mergeInmemoryParts() error {
	maxOutBytes := pt.getMaxBigPartSize()

	pt.partsLock.Lock()
	pws, needFreeSpace := getPartsToMerge(pt.inmemoryParts, maxOutBytes, false)
	pt.partsLock.Unlock()

	atomicSetBool(&pt.mergeNeedFreeDiskSpace, needFreeSpace)
	return pt.mergeParts(pws, pt.stopCh, false)
}

func (pt *partition) mergeExistingParts(isFinal bool) error {
	if !pt.canBackgroundMerge() {
		// Do not perform merge in read-only mode, since this may result in disk space shortage.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2603
		return errReadOnlyMode
	}
	maxOutBytes := pt.getMaxBigPartSize()

	pt.partsLock.Lock()
	dst := make([]*partWrapper, 0, len(pt.inmemoryParts)+len(pt.smallParts)+len(pt.bigParts))
	dst = append(dst, pt.inmemoryParts...)
	dst = append(dst, pt.smallParts...)
	dst = append(dst, pt.bigParts...)
	pws, needFreeSpace := getPartsToMerge(dst, maxOutBytes, isFinal)
	pt.partsLock.Unlock()

	atomicSetBool(&pt.mergeNeedFreeDiskSpace, needFreeSpace)
	return pt.mergeParts(pws, pt.stopCh, isFinal)
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
	dedupInterval := GetDedupInterval(pt.tr.MaxTimestamp)
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

// mergeParts merges pws to a single resulting part.
//
// Merging is immediately stopped if stopCh is closed.
//
// if isFinal is set, then the resulting part will be saved to disk.
//
// All the parts inside pws must have isInMerge field set to true.
func (pt *partition) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal bool) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
	}
	defer pt.releasePartsToMerge(pws)

	startTime := time.Now()

	// Initialize destination paths.
	dstPartType := pt.getDstPartType(pws, isFinal)
	ptPath, tmpPartPath, mergeIdx := pt.getDstPartPaths(dstPartType)

	if dstPartType == partBig {
		bigMergeWorkersLimitCh <- struct{}{}
		defer func() {
			<-bigMergeWorkersLimitCh
		}()
	}

	if isFinal && len(pws) == 1 && pws[0].mp != nil {
		// Fast path: flush a single in-memory part to disk.
		mp := pws[0].mp
		if tmpPartPath == "" {
			logger.Panicf("BUG: tmpPartPath must be non-empty")
		}
		if err := mp.StoreToDisk(tmpPartPath); err != nil {
			return fmt.Errorf("cannot store in-memory part to %q: %w", tmpPartPath, err)
		}
		pwNew, err := pt.openCreatedPart(&mp.ph, pws, nil, ptPath, tmpPartPath, mergeIdx)
		if err != nil {
			return fmt.Errorf("cannot atomically register the created part: %w", err)
		}
		pt.swapSrcWithDstParts(pws, pwNew, dstPartType)
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
	srcRowsCount := uint64(0)
	srcBlocksCount := uint64(0)
	for _, pw := range pws {
		srcSize += pw.p.size
		srcRowsCount += pw.p.ph.RowsCount
		srcBlocksCount += pw.p.ph.BlocksCount
	}
	rowsPerBlock := float64(srcRowsCount) / float64(srcBlocksCount)
	compressLevel := getCompressLevel(rowsPerBlock)
	bsw := getBlockStreamWriter()
	var mpNew *inmemoryPart
	if dstPartType == partInmemory {
		mpNew = getInmemoryPart()
		bsw.InitFromInmemoryPart(mpNew, compressLevel)
	} else {
		if tmpPartPath == "" {
			logger.Panicf("BUG: tmpPartPath must be non-empty")
		}
		nocache := dstPartType == partBig
		if err := bsw.InitFromFilePart(tmpPartPath, nocache, compressLevel); err != nil {
			closeBlockStreamReaders()
			return fmt.Errorf("cannot create destination part at %q: %w", tmpPartPath, err)
		}
	}

	// Merge source parts to destination part.
	ph, err := pt.mergePartsInternal(tmpPartPath, bsw, bsrs, dstPartType, stopCh)
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
	pwNew, err := pt.openCreatedPart(ph, pws, mpNew, ptPath, tmpPartPath, mergeIdx)
	if err != nil {
		return fmt.Errorf("cannot atomically register the created part: %w", err)
	}
	pt.swapSrcWithDstParts(pws, pwNew, dstPartType)

	d := time.Since(startTime)
	if d <= 30*time.Second {
		return nil
	}

	// Log stats for long merges.
	dstRowsCount := uint64(0)
	dstBlocksCount := uint64(0)
	dstSize := uint64(0)
	dstPartPath := ""
	if pwNew != nil {
		pDst := pwNew.p
		dstRowsCount = pDst.ph.RowsCount
		dstBlocksCount = pDst.ph.BlocksCount
		dstSize = pDst.size
		dstPartPath = pDst.String()
	}
	durationSecs := d.Seconds()
	rowsPerSec := int(float64(srcRowsCount) / durationSecs)
	logger.Infof("merged (%d parts, %d rows, %d blocks, %d bytes) into (1 part, %d rows, %d blocks, %d bytes) in %.3f seconds at %d rows/sec to %q",
		len(pws), srcRowsCount, srcBlocksCount, srcSize, dstRowsCount, dstBlocksCount, dstSize, durationSecs, rowsPerSec, dstPartPath)

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

func (pt *partition) getDstPartPaths(dstPartType partType) (string, string, uint64) {
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
	ptPath = filepath.Clean(ptPath)
	mergeIdx := pt.nextMergeIdx()
	tmpPartPath := ""
	if dstPartType != partInmemory {
		tmpPartPath = fmt.Sprintf("%s/tmp/%016X", ptPath, mergeIdx)
	}
	return ptPath, tmpPartPath, mergeIdx
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

func (pt *partition) mergePartsInternal(tmpPartPath string, bsw *blockStreamWriter, bsrs []*blockStreamReader, dstPartType partType, stopCh <-chan struct{}) (*partHeader, error) {
	var ph partHeader
	var rowsMerged *uint64
	var rowsDeleted *uint64
	var mergesCount *uint64
	var activeMerges *uint64
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
	retentionDeadline := timestampFromTime(time.Now()) - pt.s.retentionMsecs
	atomic.AddUint64(activeMerges, 1)
	err := mergeBlockStreams(&ph, bsw, bsrs, stopCh, pt.s, retentionDeadline, rowsMerged, rowsDeleted)
	atomic.AddUint64(activeMerges, ^uint64(0))
	atomic.AddUint64(mergesCount, 1)
	if err != nil {
		return nil, fmt.Errorf("cannot merge parts to %q: %w", tmpPartPath, err)
	}
	if tmpPartPath != "" {
		ph.MinDedupInterval = GetDedupInterval(ph.MaxTimestamp)
		if err := ph.writeMinDedupInterval(tmpPartPath); err != nil {
			return nil, fmt.Errorf("cannot store min dedup interval: %w", err)
		}
	}
	return &ph, nil
}

func (pt *partition) openCreatedPart(ph *partHeader, pws []*partWrapper, mpNew *inmemoryPart, ptPath, tmpPartPath string, mergeIdx uint64) (*partWrapper, error) {
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
		if ph.RowsCount > 0 {
			// The destination part may have no rows if they are deleted during the merge.
			dstPartPath = ph.Path(ptPath, mergeIdx)
		}
		fmt.Fprintf(&bb, "%s -> %s\n", tmpPartPath, dstPartPath)
		txnPath := fmt.Sprintf("%s/txn/%016X", ptPath, mergeIdx)
		if err := fs.WriteFileAtomically(txnPath, bb.B, false); err != nil {
			return nil, fmt.Errorf("cannot create transaction file %q: %w", txnPath, err)
		}

		// Run the created transaction.
		if err := runTransaction(&pt.snapshotLock, pt.smallPartsPath, pt.bigPartsPath, txnPath); err != nil {
			return nil, fmt.Errorf("cannot execute transaction %q: %w", txnPath, err)
		}
	}
	// Open the created part.
	if ph.RowsCount == 0 {
		// The created part is empty.
		return nil, nil
	}
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

func (pt *partition) swapSrcWithDstParts(pws []*partWrapper, pwNew *partWrapper, dstPartType partType) {
	// Atomically unregister old parts and add new part to pt.
	m := make(map[*partWrapper]bool, len(pws))
	for _, pw := range pws {
		m[pw] = true
	}
	if len(m) != len(pws) {
		logger.Panicf("BUG: %d duplicate parts found when merging %d parts", len(pws)-len(m), len(pws))
	}
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
		case partSmall:
			pt.smallParts = append(pt.smallParts, pwNew)
		case partBig:
			pt.bigParts = append(pt.bigParts, pwNew)
		default:
			logger.Panicf("BUG: unknown partType=%d", dstPartType)
		}
	}
	pt.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedSmallParts + removedBigParts
	if removedParts != len(m) {
		logger.Panicf("BUG: unexpected number of parts removed; got %d, want %d", removedParts, len(m))
	}

	// Remove partition references from old parts.
	for _, pw := range pws {
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
	if rowsPerBlock <= 2000 {
		return 3
	}
	if rowsPerBlock <= 4000 {
		return 4
	}
	return 5
}

func (pt *partition) nextMergeIdx() uint64 {
	return atomic.AddUint64(&pt.mergeIdx, 1)
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

func (pt *partition) startStalePartsRemover() {
	pt.wg.Add(1)
	go func() {
		pt.stalePartsRemover()
		pt.wg.Done()
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
	retentionDeadline := timestampFromTime(startTime) - pt.s.retentionMsecs

	pt.partsLock.Lock()
	for _, pw := range pt.inmemoryParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.inmemoryRowsDeleted, pw.p.ph.RowsCount)
			m[pw] = true
		}
	}
	for _, pw := range pt.smallParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.smallRowsDeleted, pw.p.ph.RowsCount)
			m[pw] = true
		}
	}
	for _, pw := range pt.bigParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.bigRowsDeleted, pw.p.ph.RowsCount)
			m[pw] = true
		}
	}
	removedInmemoryParts := 0
	removedSmallParts := 0
	removedBigParts := 0
	if len(m) > 0 {
		pt.inmemoryParts, removedInmemoryParts = removeParts(pt.inmemoryParts, m)
		pt.smallParts, removedSmallParts = removeParts(pt.smallParts, m)
		pt.bigParts, removedBigParts = removeParts(pt.bigParts, m)
	}
	pt.partsLock.Unlock()

	removedParts := removedInmemoryParts + removedSmallParts + removedBigParts
	if removedParts != len(m) {
		logger.Panicf("BUG: unexpected number of stale parts removed; got %d, want %d", removedParts, len(m))
	}

	// Physically remove stale parts under snapshotLock in order to provide
	// consistent snapshots with table.CreateSnapshot().
	pt.snapshotLock.RLock()
	for pw := range m {
		if pw.mp == nil {
			logger.Infof("removing part %q, since its data is out of the configured retention (%d secs)", pw.p.path, pt.s.retentionMsecs/1000)
			fs.MustRemoveDirAtomic(pw.p.path)
		}
	}
	// There is no need in calling fs.MustSyncPath() on pt.smallPartsPath and pt.bigPartsPath,
	// since they should be automatically called inside fs.MustRemoveDirAtomic().
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
	fs.MustRemoveTemporaryDirs(path)
	d, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open partition directory: %w", err)
	}
	defer fs.MustClose(d)

	// Run remaining transactions and cleanup /txn and /tmp directories.
	// Snapshots cannot be created yet, so use fakeSnapshotLock.
	var fakeSnapshotLock sync.RWMutex
	if err := runTransactions(&fakeSnapshotLock, pathPrefix1, pathPrefix2, path); err != nil {
		return nil, fmt.Errorf("cannot run transactions from %q: %w", path, err)
	}

	txnDir := path + "/txn"
	fs.MustRemoveDirAtomic(txnDir)
	tmpDir := path + "/tmp"
	fs.MustRemoveDirAtomic(tmpDir)
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
		if fn == "snapshots" {
			// "snapshots" dir is skipped for backwards compatibility. Now it is unused.
			continue
		}
		if fn == "tmp" || fn == "txn" {
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
	pt.flushInmemoryRows()

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
		return fmt.Errorf("cannot open partition difrectory: %w", err)
	}
	defer fs.MustClose(d)

	fis, err := d.Readdir(-1)
	if err != nil {
		return fmt.Errorf("cannot read partition directory: %w", err)
	}
	for _, fi := range fis {
		fn := fi.Name()
		if !fs.IsDirOrSymlink(fi) {
			if fn == "appliedRetention.txt" {
				// Copy the appliedRetention.txt file to dstDir.
				// This file can be created by VictoriaMetrics enterprise.
				// See https://docs.victoriametrics.com/#retention-filters .
				// Do not make hard link to this file, since it can be modified over time.
				srcPath := srcDir + "/" + fn
				dstPath := dstDir + "/" + fn
				if err := fs.CopyFile(srcPath, dstPath); err != nil {
					return fmt.Errorf("cannot copy %q to %q: %w", srcPath, dstPath, err)
				}
			}
			// Skip non-directories.
			continue
		}
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
		return fmt.Errorf("cannot open transaction directory: %w", err)
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
		path, err := validatePath(pathPrefix1, pathPrefix2, path)
		if err != nil {
			return fmt.Errorf("invalid path to remove: %w", err)
		}
		fs.MustRemoveDirAtomic(path)
	}

	// Move the new part to new directory.
	srcPath := mvPaths[0]
	dstPath := mvPaths[1]
	if len(srcPath) > 0 {
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
				logger.Infof("cannot find both source and destination paths: %q -> %q; this may be the case after unclean shutdown "+
					"(OOM, `kill -9`, hard reset) on NFS disk", srcPath, dstPath)
			}
		} else {
			// Just remove srcPath.
			fs.MustRemoveDirAtomic(srcPath)
		}
	}

	// Flush pathPrefix* directory metadata to the underying storage,
	// so the moved files become visible there.
	fs.MustSyncPath(pathPrefix1)
	fs.MustSyncPath(pathPrefix2)

	pendingTxnDeletionsWG.Add(1)
	go func() {
		defer pendingTxnDeletionsWG.Done()

		// There is no need in calling fs.MustSyncPath for pathPrefix* after parts' removal,
		// since it is already called by fs.MustRemoveDirAtomic.

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
