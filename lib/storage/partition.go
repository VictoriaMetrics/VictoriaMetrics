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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
)

// The maximum size of big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxBigPartSize = 1e12

// The maximum number of inmemory parts in the partition.
//
// If the number of inmemory parts reaches this value, then assisted merge runs during data ingestion.
const maxInmemoryPartsPerPartition = 20

// The maximum number of small parts in the partition.
//
// If the number of small parts reaches this value, then assisted merge runs during data ingestion.
const maxSmallPartsPerPartition = 30

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

// The interval for flushing buffered rows into parts, so they become visible to search.
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
	inmemoryParts []*partWrapper

	// Contains file-based parts with small number of items.
	smallParts []*partWrapper

	// Contains file-based parts with big number of items.
	bigParts []*partWrapper

	// This channel is used for signaling the background mergers that there are parts,
	// which may need to be merged.
	needMergeCh chan struct{}

	stopCh chan struct{}

	wg sync.WaitGroup
}

// partWrapper is a wrapper for the part.
type partWrapper struct {
	// The number of references to the part.
	refCount uint32

	// The flag, which is set when the part must be deleted after refCount reaches zero.
	// This field should be updated only after partWrapper
	// was removed from the list of active parts.
	mustBeDeleted uint32

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

	pt := newPartition(name, smallPartsPath, bigPartsPath, s)
	pt.tr.fromPartitionTimestamp(timestamp)
	pt.startBackgroundWorkers()

	logger.Infof("partition %q has been created", name)

	return pt
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

	fs.MustRemoveDirAtomic(pt.smallPartsPath)
	fs.MustRemoveDirAtomic(pt.bigPartsPath)
	logger.Infof("partition %q has been dropped", pt.name)
}

// mustOpenPartition opens the existing partition from the given paths.
func mustOpenPartition(smallPartsPath, bigPartsPath string, s *Storage) *partition {
	smallPartsPath = filepath.Clean(smallPartsPath)
	bigPartsPath = filepath.Clean(bigPartsPath)

	name := filepath.Base(smallPartsPath)
	if !strings.HasSuffix(bigPartsPath, name) {
		logger.Panicf("FATAL: partition name in bigPartsPath %q doesn't match smallPartsPath %q; want %q", bigPartsPath, smallPartsPath, name)
	}

	partNamesSmall, partNamesBig := mustReadPartNames(smallPartsPath, bigPartsPath)

	smallParts := mustOpenParts(smallPartsPath, partNamesSmall)
	bigParts := mustOpenParts(bigPartsPath, partNamesBig)

	partNamesPath := filepath.Join(smallPartsPath, partsFilename)
	if !fs.IsPathExist(partNamesPath) {
		// Create parts.json file if it doesn't exist yet.
		// This should protect from possible carshloops just after the migration from versions below v1.90.0
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4336
		mustWritePartNames(smallParts, bigParts, smallPartsPath)
	}

	pt := newPartition(name, smallPartsPath, bigPartsPath, s)
	pt.smallParts = smallParts
	pt.bigParts = bigParts
	if err := pt.tr.fromPartitionName(name); err != nil {
		logger.Panicf("FATAL: cannot obtain partition time range from smallPartsPath %q: %s", smallPartsPath, err)
	}
	pt.startBackgroundWorkers()

	// Wake up a single background merger, so it could start merging parts if needed.
	pt.notifyBackgroundMergers()

	return pt
}

func newPartition(name, smallPartsPath, bigPartsPath string, s *Storage) *partition {
	p := &partition{
		name:           name,
		smallPartsPath: smallPartsPath,
		bigPartsPath:   bigPartsPath,

		s: s,

		mergeIdx:    uint64(time.Now().UnixNano()),
		needMergeCh: make(chan struct{}, cgroup.AvailableCPUs()),

		stopCh: make(chan struct{}),
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
		m.InmemoryPartsRefCount += uint64(atomic.LoadUint32(&pw.refCount))
	}
	for _, pw := range pt.smallParts {
		p := pw.p
		m.SmallRowsCount += p.ph.RowsCount
		m.SmallBlocksCount += p.ph.BlocksCount
		m.SmallSizeBytes += p.size
		m.SmallPartsRefCount += uint64(atomic.LoadUint32(&pw.refCount))
	}
	for _, pw := range pt.bigParts {
		p := pw.p
		m.BigRowsCount += p.ph.RowsCount
		m.BigBlocksCount += p.ph.BlocksCount
		m.BigSizeBytes += p.size
		m.BigPartsRefCount += uint64(atomic.LoadUint32(&pw.refCount))
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
		rrs.rows = newRawRows()
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

		// Run assisted merges if needed.
		flushConcurrencyCh <- struct{}{}
		pt.assistedMergeForInmemoryParts()
		pt.assistedMergeForSmallParts()
		// There is no need in assisted merges for big parts,
		// since the bottleneck is possible only at inmemory and small parts.
		<-flushConcurrencyCh
	}

	return rows
}

type rawRowsBlock struct {
	rows []rawRow
}

func newRawRows() []rawRow {
	n := getMaxRawRowsPerShard()
	return make([]rawRow, 0, n)
}

func getRawRowsBlock() *rawRowsBlock {
	v := rawRowsBlockPool.Get()
	if v == nil {
		return &rawRowsBlock{
			rows: newRawRows(),
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
	for range pws {
		if !pt.notifyBackgroundMergers() {
			break
		}
	}
	pt.partsLock.Unlock()
}

func (pt *partition) notifyBackgroundMergers() bool {
	select {
	case pt.needMergeCh <- struct{}{}:
		return true
	default:
		return false
	}
}

var flushConcurrencyLimit = func() int {
	n := cgroup.AvailableCPUs()
	if n < 3 {
		// Allow at least 3 concurrent flushers on systems with a single CPU core
		// in order to guarantee that in-memory data flushes and background merges can be continued
		// when a single flusher is busy with the long merge of big parts,
		// while another flusher is busy with the long merge of small parts.
		n = 3
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

func (pt *partition) assistedMergeForInmemoryParts() {
	pt.partsLock.Lock()
	needMerge := needAssistedMerge(pt.inmemoryParts, maxInmemoryPartsPerPartition)
	pt.partsLock.Unlock()
	if !needMerge {
		return
	}

	atomic.AddUint64(&pt.inmemoryAssistedMerges, 1)
	err := pt.mergeInmemoryParts()
	if err == nil {
		return
	}
	if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) {
		return
	}
	logger.Panicf("FATAL: cannot merge inmemory parts: %s", err)
}

func (pt *partition) assistedMergeForSmallParts() {
	pt.partsLock.Lock()
	needMerge := needAssistedMerge(pt.smallParts, maxSmallPartsPerPartition)
	pt.partsLock.Unlock()
	if !needMerge {
		return
	}

	atomic.AddUint64(&pt.smallAssistedMerges, 1)
	err := pt.mergeExistingParts(false)
	if err == nil {
		return
	}
	if errors.Is(err, errNothingToMerge) || errors.Is(err, errForciblyStopped) || errors.Is(err, errReadOnlyMode) {
		return
	}
	logger.Panicf("FATAL: cannot merge small parts: %s", err)
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
	p := mp.NewPart()
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
	close(pt.stopCh)

	// Waiting for service workers to stop
	pt.wg.Wait()

	pt.flushInmemoryRows()

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
}

func (rrss *rawRowsShards) flush(pt *partition, dst []rawRow, isFinal bool) []rawRow {
	for i := range rrss.shards {
		dst = rrss.shards[i].appendRawRowsToFlush(dst, isFinal)
	}
	pt.flushRowsToParts(dst)
	return dst
}

func (rrs *rawRowsShard) appendRawRowsToFlush(dst []rawRow, isFinal bool) []rawRow {
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
	if err := pt.mergePartsOptimal(pws, pt.stopCh); err != nil {
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

var mergeWorkersLimitCh = make(chan struct{}, getDefaultMergeConcurrency(16))

func getDefaultMergeConcurrency(max int) int {
	v := (cgroup.AvailableCPUs() + 1) / 2
	if v > max {
		v = max
	}
	return adjustMergeWorkersLimit(v)
}

// SetMergeWorkersCount sets the maximum number of concurrent mergers for parts.
//
// The function must be called before opening or creating any storage.
func SetMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	n = adjustMergeWorkersLimit(n)
	mergeWorkersLimitCh = make(chan struct{}, n)
}

func adjustMergeWorkersLimit(n int) int {
	if n < 4 {
		// Allow at least 4 merge workers on systems with small CPUs count
		// in order to guarantee that background merges can be continued
		// when multiple workers are busy with big merges.
		n = 4
	}
	return n
}

func (pt *partition) startMergeWorkers() {
	// The actual number of concurrent merges is limited inside mergeWorker() below.
	for i := 0; i < cap(mergeWorkersLimitCh); i++ {
		pt.wg.Add(1)
		go func() {
			pt.mergeWorker()
			pt.wg.Done()
		}()
	}
}

func (pt *partition) mergeWorker() {
	var lastMergeTime uint64
	isFinal := false
	for {
		// Limit the number of concurrent calls to mergeExistingParts, since the total number of merge workers
		// across partitions may exceed the the cap(mergeWorkersLimitCh).
		mergeWorkersLimitCh <- struct{}{}
		err := pt.mergeExistingParts(isFinal)
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
			logger.Panicf("FATAL: unrecoverable error when merging parts in the partition (%q, %q): %s", pt.smallPartsPath, pt.bigPartsPath, err)
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
		case <-pt.stopCh:
			return
		case <-pt.needMergeCh:
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

func (pt *partition) canBackgroundMerge() bool {
	return atomic.LoadUint32(&pt.s.isReadOnly) == 0
}

var errReadOnlyMode = fmt.Errorf("storage is in readonly mode")

func (pt *partition) mergeInmemoryParts() error {
	maxOutBytes := pt.getMaxBigPartSize()

	pt.partsLock.Lock()
	pws := getPartsToMerge(pt.inmemoryParts, maxOutBytes, false)
	pt.partsLock.Unlock()

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
	pws := getPartsToMerge(dst, maxOutBytes, isFinal)
	pt.partsLock.Unlock()

	return pt.mergeParts(pws, pt.stopCh, isFinal)
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

var errNothingToMerge = fmt.Errorf("nothing to merge")

func (pt *partition) runFinalDedup() error {
	requiredDedupInterval, actualDedupInterval := pt.getRequiredDedupInterval()
	t := time.Now()
	logger.Infof("starting final dedup for partition %s using requiredDedupInterval=%d ms, since the partition has smaller actualDedupInterval=%d ms",
		pt.bigPartsPath, requiredDedupInterval, actualDedupInterval)
	if err := pt.ForceMergeAllParts(); err != nil {
		return fmt.Errorf("cannot perform final dedup for partition %s: %w", pt.bigPartsPath, err)
	}
	logger.Infof("final dedup for partition %s has been finished in %.3f seconds", pt.bigPartsPath, time.Since(t).Seconds())
	return nil
}

func (pt *partition) isFinalDedupNeeded() bool {
	requiredDedupInterval, actualDedupInterval := pt.getRequiredDedupInterval()
	return requiredDedupInterval > actualDedupInterval
}

func (pt *partition) getRequiredDedupInterval() (int64, int64) {
	pws := pt.GetParts(nil, false)
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

// mergeParts merges pws to a single resulting part.
//
// Merging is immediately stopped if stopCh is closed.
//
// if isFinal is set, then the resulting part will be saved to disk.
//
// All the parts inside pws must have isInMerge field set to true.
// The isInMerge field inside pws parts is set to false before returning from the function.
func (pt *partition) mergeParts(pws []*partWrapper, stopCh <-chan struct{}, isFinal bool) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
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

	// Merge source parts to destination part.
	ph, err := pt.mergePartsInternal(dstPartPath, bsw, bsrs, dstPartType, stopCh)
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

func (pt *partition) mergePartsInternal(dstPartPath string, bsw *blockStreamWriter, bsrs []*blockStreamReader, dstPartType partType, stopCh <-chan struct{}) (*partHeader, error) {
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
		pt.notifyBackgroundMergers()
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
		atomic.StoreUint32(&pw.mustBeDeleted, 1)
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
	startTime := time.Now()
	retentionDeadline := timestampFromTime(startTime) - pt.s.retentionMsecs

	var pws []*partWrapper
	pt.partsLock.Lock()
	for _, pw := range pt.inmemoryParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.inmemoryRowsDeleted, pw.p.ph.RowsCount)
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	for _, pw := range pt.smallParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.smallRowsDeleted, pw.p.ph.RowsCount)
			pw.isInMerge = true
			pws = append(pws, pw)
		}
	}
	for _, pw := range pt.bigParts {
		if !pw.isInMerge && pw.p.ph.MaxTimestamp < retentionDeadline {
			atomic.AddUint64(&pt.bigRowsDeleted, pw.p.ph.RowsCount)
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
func getPartsToMerge(pws []*partWrapper, maxOutBytes uint64, isFinal bool) []*partWrapper {
	pwsRemaining := make([]*partWrapper, 0, len(pws))
	for _, pw := range pws {
		if !pw.isInMerge {
			pwsRemaining = append(pwsRemaining, pw)
		}
	}
	maxPartsToMerge := defaultPartsToMerge
	var pms []*partWrapper
	if isFinal {
		for len(pms) == 0 && maxPartsToMerge >= finalPartsToMerge {
			pms = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
			maxPartsToMerge--
		}
	} else {
		pms = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxOutBytes)
	}
	for _, pw := range pms {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge cannot be set")
		}
		pw.isInMerge = true
	}
	return pms
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

func getPartsSize(pws []*partWrapper) uint64 {
	n := uint64(0)
	for _, pw := range pws {
		n += pw.p.size
	}
	return n
}

func mustOpenParts(path string, partNames []string) []*partWrapper {
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

	return pws
}

// MustCreateSnapshotAt creates pt snapshot at the given smallPath and bigPath dirs.
//
// Snapshot is created using linux hard links, so it is usually created very quickly.
func (pt *partition) MustCreateSnapshotAt(smallPath, bigPath string) {
	logger.Infof("creating partition snapshot of %q and %q...", pt.smallPartsPath, pt.bigPartsPath)
	startTime := time.Now()

	// Flush inmemory data to disk.
	pt.flushInmemoryRows()

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
	// This file can be created by VictoriaMetrics enterprise.
	// See https://docs.victoriametrics.com/#retention-filters .
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
	partNamesPath := filepath.Join(dstDir, partsFilename)
	fs.MustWriteAtomic(partNamesPath, data, true)
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

func mustReadPartNames(smallPartsPath, bigPartsPath string) ([]string, []string) {
	partNamesPath := filepath.Join(smallPartsPath, partsFilename)
	if fs.IsPathExist(partNamesPath) {
		data, err := os.ReadFile(partNamesPath)
		if err != nil {
			logger.Panicf("FATAL: cannot read %s file: %s", partsFilename, err)
		}
		var partNames partNamesJSON
		if err := json.Unmarshal(data, &partNames); err != nil {
			logger.Panicf("FATAL: cannot parse %s: %s", partNamesPath, err)
		}
		return partNames.Small, partNames.Big
	}
	// The partsFilename is missing. This is the upgrade from versions previous to v1.90.0.
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
