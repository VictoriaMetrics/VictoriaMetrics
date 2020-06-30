package storage

import (
	"fmt"
	"io/ioutil"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// These are global counters for cache requests and misses for parts
// which were already merged into another parts.
var (
	historicalBigIndexBlocksCacheRequests uint64
	historicalBigIndexBlocksCacheMisses   uint64

	historicalSmallIndexBlocksCacheRequests uint64
	historicalSmallIndexBlocksCacheMisses   uint64
)

func maxRowsPerSmallPart() uint64 {
	// Small parts are cached in the OS page cache,
	// so limit the number of rows for small part by the remaining free RAM.
	mem := memory.Remaining()
	// Production data shows that each row occupies ~1 byte in the compressed part.
	// It is expected no more than defaultPartsToMerge/2 parts exist
	// in the OS page cache before they are merged into bigger part.
	// Half of the remaining RAM must be left for lib/mergeset parts,
	// so the maxItems is calculated using the below code:
	maxRows := uint64(mem) / defaultPartsToMerge
	if maxRows < 10e6 {
		maxRows = 10e6
	}
	return maxRows
}

// The maximum number of rows per big part.
//
// This number limits the maximum time required for building big part.
// This time shouldn't exceed a few days.
const maxRowsPerBigPart = 1e12

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
var rawRowsShardsPerPartition = (runtime.GOMAXPROCS(-1) + 7) / 8

// getMaxRowsPerPartition returns the maximum number of rows that haven't been converted into parts yet.
func getMaxRawRowsPerPartition() int {
	maxRawRowsPerPartitionOnce.Do(func() {
		n := memory.Allowed() / 256 / int(unsafe.Sizeof(rawRow{}))
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

	mergeIdx uint64

	smallPartsPath string
	bigPartsPath   string

	// The callack that returns deleted metric ids which must be skipped during merge.
	getDeletedMetricIDs func() *uint64set.Set

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
func createPartition(timestamp int64, smallPartitionsPath, bigPartitionsPath string, getDeletedMetricIDs func() *uint64set.Set) (*partition, error) {
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

	pt := newPartition(name, smallPartsPath, bigPartsPath, getDeletedMetricIDs)
	pt.tr.fromPartitionTimestamp(timestamp)
	pt.startMergeWorkers()
	pt.startRawRowsFlusher()
	pt.startInmemoryPartsFlusher()

	logger.Infof("partition %q has been created", name)

	return pt, nil
}

// Drop drops all the data on the storage for the given pt.
//
// The pt must be detached from table before calling pt.Drop.
func (pt *partition) Drop() {
	logger.Infof("dropping partition %q at smallPartsPath=%q, bigPartsPath=%q", pt.name, pt.smallPartsPath, pt.bigPartsPath)
	fs.MustRemoveAll(pt.smallPartsPath)
	fs.MustRemoveAll(pt.bigPartsPath)
	logger.Infof("partition %q has been dropped", pt.name)
}

// openPartition opens the existing partition from the given paths.
func openPartition(smallPartsPath, bigPartsPath string, getDeletedMetricIDs func() *uint64set.Set) (*partition, error) {
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

	pt := newPartition(name, smallPartsPath, bigPartsPath, getDeletedMetricIDs)
	pt.smallParts = smallParts
	pt.bigParts = bigParts
	if err := pt.tr.fromPartitionName(name); err != nil {
		return nil, fmt.Errorf("cannot obtain partition time range from smallPartsPath %q: %w", smallPartsPath, err)
	}
	pt.startMergeWorkers()
	pt.startRawRowsFlusher()
	pt.startInmemoryPartsFlusher()

	return pt, nil
}

func newPartition(name, smallPartsPath, bigPartsPath string, getDeletedMetricIDs func() *uint64set.Set) *partition {
	p := &partition{
		name:           name,
		smallPartsPath: smallPartsPath,
		bigPartsPath:   bigPartsPath,

		getDeletedMetricIDs: getDeletedMetricIDs,

		mergeIdx: uint64(time.Now().UnixNano()),
		stopCh:   make(chan struct{}),
	}
	p.rawRows.init()
	return p
}

// partitionMetrics contains essential metrics for the partition.
type partitionMetrics struct {
	PendingRows uint64

	BigIndexBlocksCacheSize     uint64
	BigIndexBlocksCacheRequests uint64
	BigIndexBlocksCacheMisses   uint64

	SmallIndexBlocksCacheSize     uint64
	SmallIndexBlocksCacheRequests uint64
	SmallIndexBlocksCacheMisses   uint64

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
}

// UpdateMetrics updates m with metrics from pt.
func (pt *partition) UpdateMetrics(m *partitionMetrics) {
	rawRowsLen := uint64(pt.rawRows.Len())
	m.PendingRows += rawRowsLen
	m.SmallRowsCount += rawRowsLen

	pt.partsLock.Lock()

	for _, pw := range pt.bigParts {
		p := pw.p

		m.BigIndexBlocksCacheSize += p.ibCache.Len()
		m.BigIndexBlocksCacheRequests += p.ibCache.Requests()
		m.BigIndexBlocksCacheMisses += p.ibCache.Misses()
		m.BigRowsCount += p.ph.RowsCount
		m.BigBlocksCount += p.ph.BlocksCount
		m.BigSizeBytes += p.size
		m.BigPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}

	for _, pw := range pt.smallParts {
		p := pw.p

		m.SmallIndexBlocksCacheSize += p.ibCache.Len()
		m.SmallIndexBlocksCacheRequests += p.ibCache.Requests()
		m.SmallIndexBlocksCacheMisses += p.ibCache.Misses()
		m.SmallRowsCount += p.ph.RowsCount
		m.SmallBlocksCount += p.ph.BlocksCount
		m.SmallSizeBytes += p.size
		m.SmallPartsRefCount += atomic.LoadUint64(&pw.refCount)
	}

	m.BigPartsCount += uint64(len(pt.bigParts))
	m.SmallPartsCount += uint64(len(pt.smallParts))

	pt.partsLock.Unlock()

	m.BigIndexBlocksCacheRequests += atomic.LoadUint64(&historicalBigIndexBlocksCacheRequests)
	m.BigIndexBlocksCacheMisses += atomic.LoadUint64(&historicalBigIndexBlocksCacheMisses)

	m.SmallIndexBlocksCacheRequests += atomic.LoadUint64(&historicalSmallIndexBlocksCacheRequests)
	m.SmallIndexBlocksCacheMisses += atomic.LoadUint64(&historicalSmallIndexBlocksCacheMisses)

	m.ActiveBigMerges += atomic.LoadUint64(&pt.activeBigMerges)
	m.ActiveSmallMerges += atomic.LoadUint64(&pt.activeSmallMerges)

	m.BigMergesCount += atomic.LoadUint64(&pt.bigMergesCount)
	m.SmallMergesCount += atomic.LoadUint64(&pt.smallMergesCount)

	m.BigRowsMerged += atomic.LoadUint64(&pt.bigRowsMerged)
	m.SmallRowsMerged += atomic.LoadUint64(&pt.smallRowsMerged)

	m.BigRowsDeleted += atomic.LoadUint64(&pt.bigRowsDeleted)
	m.SmallRowsDeleted += atomic.LoadUint64(&pt.smallRowsDeleted)

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
	lock     sync.Mutex
	shardIdx int

	// Shards reduce lock contention when adding rows on multi-CPU systems.
	shards []rawRowsShard
}

func (rrs *rawRowsShards) init() {
	rrs.shards = make([]rawRowsShard, rawRowsShardsPerPartition)
}

func (rrs *rawRowsShards) addRows(pt *partition, rows []rawRow) {
	rrs.lock.Lock()
	rrs.shardIdx++
	if rrs.shardIdx >= len(rrs.shards) {
		rrs.shardIdx = 0
	}
	shard := &rrs.shards[rrs.shardIdx]
	rrs.lock.Unlock()

	shard.addRows(pt, rows)
}

func (rrs *rawRowsShards) Len() int {
	n := 0
	for i := range rrs.shards[:] {
		n += rrs.shards[i].Len()
	}
	return n
}

type rawRowsShard struct {
	lock          sync.Mutex
	rows          []rawRow
	lastFlushTime uint64
}

func (rrs *rawRowsShard) Len() int {
	rrs.lock.Lock()
	n := len(rrs.rows)
	rrs.lock.Unlock()
	return n
}

func (rrs *rawRowsShard) addRows(pt *partition, rows []rawRow) {
	var rrss []*rawRows

	rrs.lock.Lock()
	if cap(rrs.rows) == 0 {
		rrs.rows = getRawRowsMaxSize().rows
	}
	maxRowsCount := getMaxRawRowsPerPartition()
	for {
		capacity := maxRowsCount - len(rrs.rows)
		if capacity >= len(rows) {
			// Fast path - rows fit capacity.
			rrs.rows = append(rrs.rows, rows...)
			break
		}

		// Slow path - rows don't fit capacity.
		// Fill rawRows to capacity and convert it to a part.
		rrs.rows = append(rrs.rows, rows[:capacity]...)
		rows = rows[capacity:]
		rr := getRawRowsMaxSize()
		rrs.rows, rr.rows = rr.rows, rrs.rows
		rrss = append(rrss, rr)
		rrs.lastFlushTime = fasttime.UnixTimestamp()
	}
	rrs.lock.Unlock()

	for _, rr := range rrss {
		pt.addRowsPart(rr.rows)
		putRawRows(rr)
	}
}

type rawRows struct {
	rows []rawRow
}

func getRawRowsMaxSize() *rawRows {
	size := getMaxRawRowsPerPartition()
	return getRawRowsWithSize(size)
}

func getRawRowsWithSize(size int) *rawRows {
	p, sizeRounded := getRawRowsPool(size)
	v := p.Get()
	if v == nil {
		return &rawRows{
			rows: make([]rawRow, 0, sizeRounded),
		}
	}
	return v.(*rawRows)
}

func putRawRows(rr *rawRows) {
	rr.rows = rr.rows[:0]
	size := cap(rr.rows)
	p, _ := getRawRowsPool(size)
	p.Put(rr)
}

func getRawRowsPool(size int) (*sync.Pool, int) {
	size--
	if size < 0 {
		size = 0
	}
	bucketIdx := 64 - bits.LeadingZeros64(uint64(size))
	if bucketIdx >= len(rawRowsPools) {
		bucketIdx = len(rawRowsPools) - 1
	}
	p := &rawRowsPools[bucketIdx]
	sizeRounded := 1 << uint(bucketIdx)
	return p, sizeRounded
}

var rawRowsPools [19]sync.Pool

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
	err = pt.mergeSmallParts(false)
	if err == nil {
		atomic.AddUint64(&pt.smallAssistedMerges, 1)
		return
	}
	if err == errNothingToMerge || err == errForciblyStopped {
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

	logger.Infof("waiting for inmemory parts flusher to stop on %q...", pt.smallPartsPath)
	startTime := time.Now()
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

	if err := pt.mergePartsOptimal(pws); err != nil {
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

func (rrs *rawRowsShards) flush(pt *partition, isFinal bool) {
	for i := range rrs.shards[:] {
		rrs.shards[i].flush(pt, isFinal)
	}
}

func (rrs *rawRowsShard) flush(pt *partition, isFinal bool) {
	var rr *rawRows
	currentTime := fasttime.UnixTimestamp()
	flushSeconds := int64(rawRowsFlushInterval.Seconds())
	if flushSeconds <= 0 {
		flushSeconds = 1
	}

	rrs.lock.Lock()
	if isFinal || currentTime-rrs.lastFlushTime > uint64(flushSeconds) {
		rr = getRawRowsMaxSize()
		rrs.rows, rr.rows = rr.rows, rrs.rows
	}
	rrs.lock.Unlock()

	if rr != nil {
		pt.addRowsPart(rr.rows)
		putRawRows(rr)
	}
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

	if err := pt.mergePartsOptimal(dstPws); err != nil {
		return dstPws, fmt.Errorf("cannot merge %d inmemory parts: %w", len(dstPws), err)
	}
	return dstPws, nil
}

func (pt *partition) mergePartsOptimal(pws []*partWrapper) error {
	for len(pws) > defaultPartsToMerge {
		if err := pt.mergeParts(pws[:defaultPartsToMerge], nil); err != nil {
			return fmt.Errorf("cannot merge %d parts: %w", defaultPartsToMerge, err)
		}
		pws = pws[defaultPartsToMerge:]
	}
	if len(pws) > 0 {
		if err := pt.mergeParts(pws, nil); err != nil {
			return fmt.Errorf("cannot merge %d parts: %w", len(pws), err)
		}
	}
	return nil
}

var mergeWorkersCount = func() int {
	n := runtime.GOMAXPROCS(-1) / 2
	if n <= 0 {
		n = 1
	}
	return n
}()

var (
	bigMergeConcurrencyLimitCh   = make(chan struct{}, mergeWorkersCount)
	smallMergeConcurrencyLimitCh = make(chan struct{}, mergeWorkersCount)
)

// SetBigMergeWorkersCount sets the maximum number of concurrent mergers for big blocks.
//
// The function must be called before opening or creating any storage.
func SetBigMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	bigMergeConcurrencyLimitCh = make(chan struct{}, n)
}

// SetSmallMergeWorkersCount sets the maximum number of concurrent mergers for small blocks.
//
// The function must be called before opening or creating any storage.
func SetSmallMergeWorkersCount(n int) {
	if n <= 0 {
		// Do nothing
		return
	}
	smallMergeConcurrencyLimitCh = make(chan struct{}, n)
}

func (pt *partition) startMergeWorkers() {
	for i := 0; i < mergeWorkersCount; i++ {
		pt.smallPartsMergerWG.Add(1)
		go func() {
			pt.smallPartsMerger()
			pt.smallPartsMergerWG.Done()
		}()
	}
	for i := 0; i < mergeWorkersCount; i++ {
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
	minMergeSleepTime = time.Millisecond
	maxMergeSleepTime = time.Second
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
		case <-pt.stopCh:
			return nil
		case <-t.C:
			t.Reset(sleepTime)
		}
	}
}

func maxRowsByPath(path string) uint64 {
	freeSpace := fs.MustGetFreeSpace(path)

	// Calculate the maximum number of rows in the output merge part
	// by dividing the freeSpace by the number of concurrent
	// mergeWorkersCount for big parts.
	// This assumes each row is compressed into 1 byte. Production
	// simulation shows that each row usually occupies up to 0.5 bytes,
	// so this is quite safe assumption.
	maxRows := freeSpace / uint64(mergeWorkersCount)
	if maxRows > maxRowsPerBigPart {
		maxRows = maxRowsPerBigPart
	}
	return maxRows
}

func (pt *partition) mergeBigParts(isFinal bool) error {
	bigMergeConcurrencyLimitCh <- struct{}{}
	defer func() {
		<-bigMergeConcurrencyLimitCh
	}()

	maxRows := maxRowsByPath(pt.bigPartsPath)

	pt.partsLock.Lock()
	pws := getPartsToMerge(pt.bigParts, maxRows, isFinal)
	pt.partsLock.Unlock()

	if len(pws) == 0 {
		return errNothingToMerge
	}

	atomic.AddUint64(&pt.bigMergesCount, 1)
	atomic.AddUint64(&pt.activeBigMerges, 1)
	err := pt.mergeParts(pws, pt.stopCh)
	atomic.AddUint64(&pt.activeBigMerges, ^uint64(0))

	return err
}

func (pt *partition) mergeSmallParts(isFinal bool) error {
	smallMergeConcurrencyLimitCh <- struct{}{}
	defer func() {
		<-smallMergeConcurrencyLimitCh
	}()

	maxRows := maxRowsByPath(pt.smallPartsPath)
	if maxRows > maxRowsPerSmallPart() {
		// The output part may go to big part,
		// so make sure it has enough space.
		maxBigPartRows := maxRowsByPath(pt.bigPartsPath)
		if maxRows > maxBigPartRows {
			maxRows = maxBigPartRows
		}
	}

	pt.partsLock.Lock()
	pws := getPartsToMerge(pt.smallParts, maxRows, isFinal)
	pt.partsLock.Unlock()

	if len(pws) == 0 {
		return errNothingToMerge
	}

	atomic.AddUint64(&pt.smallMergesCount, 1)
	atomic.AddUint64(&pt.activeSmallMerges, 1)
	err := pt.mergeParts(pws, pt.stopCh)
	atomic.AddUint64(&pt.activeSmallMerges, ^uint64(0))

	return err
}

var errNothingToMerge = fmt.Errorf("nothing to merge")

func (pt *partition) mergeParts(pws []*partWrapper, stopCh <-chan struct{}) error {
	if len(pws) == 0 {
		// Nothing to merge.
		return errNothingToMerge
	}

	defer func() {
		// Remove isInMerge flag from pws.
		pt.partsLock.Lock()
		for _, pw := range pws {
			if !pw.isInMerge {
				logger.Panicf("BUG: missing isInMerge flag on the part %q", pw.p.path)
			}
			pw.isInMerge = false
		}
		pt.partsLock.Unlock()
	}()

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

	outRowsCount := uint64(0)
	outBlocksCount := uint64(0)
	for _, pw := range pws {
		outRowsCount += pw.p.ph.RowsCount
		outBlocksCount += pw.p.ph.BlocksCount
	}
	isBigPart := outRowsCount > maxRowsPerSmallPart()
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
	var ph partHeader
	rowsMerged := &pt.smallRowsMerged
	rowsDeleted := &pt.smallRowsDeleted
	if isBigPart {
		rowsMerged = &pt.bigRowsMerged
		rowsDeleted = &pt.bigRowsDeleted
	}
	dmis := pt.getDeletedMetricIDs()
	err := mergeBlockStreams(&ph, bsw, bsrs, stopCh, rowsMerged, dmis, rowsDeleted)
	putBlockStreamWriter(bsw)
	if err != nil {
		if err == errForciblyStopped {
			return err
		}
		return fmt.Errorf("error when merging parts to %q: %w", tmpPartPath, err)
	}

	// Close bsrs.
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
	if d > 10*time.Second {
		logger.Infof("merged %d rows across %d blocks in %.3f seconds at %d rows/sec to %q; sizeBytes: %d",
			outRowsCount, outBlocksCount, d.Seconds(), int(float64(outRowsCount)/d.Seconds()), dstPartPath, newPSize)
	}

	return nil
}

func getCompressLevelForRowsCount(rowsCount, blocksCount uint64) int {
	avgRowsPerBlock := rowsCount / blocksCount
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
		if partsToRemove[pw] {
			requests := pw.p.ibCache.Requests()
			misses := pw.p.ibCache.Misses()
			if isBig {
				atomic.AddUint64(&historicalBigIndexBlocksCacheRequests, requests)
				atomic.AddUint64(&historicalBigIndexBlocksCacheMisses, misses)
			} else {
				atomic.AddUint64(&historicalSmallIndexBlocksCacheRequests, requests)
				atomic.AddUint64(&historicalSmallIndexBlocksCacheMisses, misses)
			}
			removedParts++
			continue
		}
		dst = append(dst, pw)
	}
	return dst, removedParts
}

// getPartsToMerge returns optimal parts to merge from pws.
//
// The returned rows will contain less than maxRows rows.
func getPartsToMerge(pws []*partWrapper, maxRows uint64, isFinal bool) []*partWrapper {
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
			pms = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxRows)
			maxPartsToMerge--
		}
	} else {
		pms = appendPartsToMerge(pms[:0], pwsRemaining, maxPartsToMerge, maxRows)
	}
	for _, pw := range pms {
		if pw.isInMerge {
			logger.Panicf("BUG: partWrapper.isInMerge cannot be set")
		}
		pw.isInMerge = true
	}
	return pms
}

// appendPartsToMerge finds optimal parts to merge from src, appends
// them to dst and returns the result.
func appendPartsToMerge(dst, src []*partWrapper, maxPartsToMerge int, maxRows uint64) []*partWrapper {
	if len(src) < 2 {
		// There is no need in merging zero or one part :)
		return dst
	}
	if maxPartsToMerge < 2 {
		logger.Panicf("BUG: maxPartsToMerge cannot be smaller than 2; got %d", maxPartsToMerge)
	}

	// Filter out too big parts.
	// This should reduce N for O(n^2) algorithm below.
	maxInPartRows := maxRows / 2
	tmp := make([]*partWrapper, 0, len(src))
	for _, pw := range src {
		if pw.p.ph.RowsCount > maxInPartRows {
			continue
		}
		tmp = append(tmp, pw)
	}
	src = tmp

	// Sort src parts by rows count and backwards timestamp.
	// This should improve adjanced points' locality in the merged parts.
	sort.Slice(src, func(i, j int) bool {
		a := &src[i].p.ph
		b := &src[j].p.ph
		if a.RowsCount == b.RowsCount {
			return a.MinTimestamp > b.MinTimestamp
		}
		return a.RowsCount < b.RowsCount
	})

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
			a := src[j : j+i]
			rowsSum := uint64(0)
			for _, pw := range a {
				rowsSum += pw.p.ph.RowsCount
			}
			if rowsSum > maxRows {
				// There is no need in verifying remaining parts with higher number of rows
				break
			}
			m := float64(rowsSum) / float64(a[len(a)-1].p.ph.RowsCount)
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
	// The transaction must be run under read lock in order to provide
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
		fs.MustRemoveAll(srcPath)
	}

	// Flush pathPrefix* directory metadata to the underying storage.
	fs.MustSyncPath(pathPrefix1)
	fs.MustSyncPath(pathPrefix2)

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
