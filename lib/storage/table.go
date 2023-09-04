package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// table represents a single table with time series data.
type table struct {
	path                string
	smallPartitionsPath string
	bigPartitionsPath   string

	s *Storage

	ptws     []*partitionWrapper
	ptwsLock sync.Mutex

	stop chan struct{}

	retentionWatcherWG  sync.WaitGroup
	finalDedupWatcherWG sync.WaitGroup
}

// partitionWrapper provides refcounting mechanism for the partition.
type partitionWrapper struct {
	// Atomic counters must be at the top of struct for proper 8-byte alignment on 32-bit archs.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212

	refCount uint64

	// The partition must be dropped if mustDrop > 0
	mustDrop uint64

	pt *partition
}

func (ptw *partitionWrapper) incRef() {
	atomic.AddUint64(&ptw.refCount, 1)
}

func (ptw *partitionWrapper) decRef() {
	n := atomic.AddUint64(&ptw.refCount, ^uint64(0))
	if int64(n) < 0 {
		logger.Panicf("BUG: pts.refCount must be positive; got %d", int64(n))
	}
	if n > 0 {
		return
	}

	// refCount is zero. Close the partition.
	ptw.pt.MustClose()

	if atomic.LoadUint64(&ptw.mustDrop) == 0 {
		ptw.pt = nil
		return
	}

	// ptw.mustDrop > 0. Drop the partition.
	ptw.pt.Drop()
	ptw.pt = nil
}

func (ptw *partitionWrapper) scheduleToDrop() {
	atomic.AddUint64(&ptw.mustDrop, 1)
}

// mustOpenTable opens a table on the given path.
//
// The table is created if it doesn't exist.
func mustOpenTable(path string, s *Storage) *table {
	path = filepath.Clean(path)

	// Create a directory for the table if it doesn't exist yet.
	fs.MustMkdirIfNotExist(path)

	// Create directories for small and big partitions if they don't exist yet.
	smallPartitionsPath := filepath.Join(path, smallDirname)
	fs.MustMkdirIfNotExist(smallPartitionsPath)
	fs.MustRemoveTemporaryDirs(smallPartitionsPath)

	smallSnapshotsPath := filepath.Join(smallPartitionsPath, snapshotsDirname)
	fs.MustMkdirIfNotExist(smallSnapshotsPath)
	fs.MustRemoveTemporaryDirs(smallSnapshotsPath)

	bigPartitionsPath := filepath.Join(path, bigDirname)
	fs.MustMkdirIfNotExist(bigPartitionsPath)
	fs.MustRemoveTemporaryDirs(bigPartitionsPath)

	bigSnapshotsPath := filepath.Join(bigPartitionsPath, snapshotsDirname)
	fs.MustMkdirIfNotExist(bigSnapshotsPath)
	fs.MustRemoveTemporaryDirs(bigSnapshotsPath)

	// Open partitions.
	pts := mustOpenPartitions(smallPartitionsPath, bigPartitionsPath, s)

	tb := &table{
		path:                path,
		smallPartitionsPath: smallPartitionsPath,
		bigPartitionsPath:   bigPartitionsPath,
		s:                   s,

		stop: make(chan struct{}),
	}
	for _, pt := range pts {
		tb.addPartitionNolock(pt)
	}
	tb.startRetentionWatcher()
	tb.startFinalDedupWatcher()
	return tb
}

// CreateSnapshot creates tb snapshot and returns paths to small and big parts of it.
// If deadline is reached before snapshot is created error is returned.
// If any error occurs during snapshot created data is not removed.
func (tb *table) CreateSnapshot(snapshotName string, deadline uint64) (string, string, error) {
	logger.Infof("creating table snapshot of %q...", tb.path)
	startTime := time.Now()

	ptws := tb.GetPartitions(nil)
	defer tb.PutPartitions(ptws)

	dstSmallDir := filepath.Join(tb.path, smallDirname, snapshotsDirname, snapshotName)
	fs.MustMkdirFailIfExist(dstSmallDir)

	dstBigDir := filepath.Join(tb.path, bigDirname, snapshotsDirname, snapshotName)
	fs.MustMkdirFailIfExist(dstBigDir)

	for _, ptw := range ptws {
		if deadline > 0 && fasttime.UnixTimestamp() > deadline {
			fs.MustRemoveAll(dstSmallDir)
			fs.MustRemoveAll(dstBigDir)
			return "", "", fmt.Errorf("cannot create snapshot for %q: timeout exceeded", tb.path)
		}

		smallPath := filepath.Join(dstSmallDir, ptw.pt.name)
		bigPath := filepath.Join(dstBigDir, ptw.pt.name)
		ptw.pt.MustCreateSnapshotAt(smallPath, bigPath)
	}

	fs.MustSyncPath(dstSmallDir)
	fs.MustSyncPath(dstBigDir)
	fs.MustSyncPath(filepath.Dir(dstSmallDir))
	fs.MustSyncPath(filepath.Dir(dstBigDir))

	logger.Infof("created table snapshot for %q at (%q, %q) in %.3f seconds", tb.path, dstSmallDir, dstBigDir, time.Since(startTime).Seconds())
	return dstSmallDir, dstBigDir, nil
}

// MustDeleteSnapshot deletes snapshot with the given snapshotName.
func (tb *table) MustDeleteSnapshot(snapshotName string) {
	smallDir := filepath.Join(tb.path, smallDirname, snapshotsDirname, snapshotName)
	fs.MustRemoveDirAtomic(smallDir)
	bigDir := filepath.Join(tb.path, bigDirname, snapshotsDirname, snapshotName)
	fs.MustRemoveDirAtomic(bigDir)
}

func (tb *table) addPartitionNolock(pt *partition) {
	ptw := &partitionWrapper{
		pt:       pt,
		refCount: 1,
	}
	tb.ptws = append(tb.ptws, ptw)
}

// MustClose closes the table.
// It is expected that all the pending searches on the table are finished before calling MustClose.
func (tb *table) MustClose() {
	close(tb.stop)
	tb.retentionWatcherWG.Wait()
	tb.finalDedupWatcherWG.Wait()

	tb.ptwsLock.Lock()
	ptws := tb.ptws
	tb.ptws = nil
	tb.ptwsLock.Unlock()

	for _, ptw := range ptws {
		if n := atomic.LoadUint64(&ptw.refCount); n != 1 {
			logger.Panicf("BUG: unexpected refCount=%d when closing the partition; probably there are pending searches", n)
		}
		ptw.decRef()
	}
}

// flushPendingRows flushes all the pending raw rows, so they become visible to search.
//
// This function is for debug purposes only.
func (tb *table) flushPendingRows() {
	ptws := tb.GetPartitions(nil)
	defer tb.PutPartitions(ptws)

	var rows []rawRow
	for _, ptw := range ptws {
		rows = ptw.pt.flushPendingRows(rows[:0], true)
	}
}

// TableMetrics contains essential metrics for the table.
type TableMetrics struct {
	partitionMetrics

	PartitionsRefCount uint64
}

// UpdateMetrics updates m with metrics from tb.
func (tb *table) UpdateMetrics(m *TableMetrics) {
	tb.ptwsLock.Lock()
	for _, ptw := range tb.ptws {
		ptw.pt.UpdateMetrics(&m.partitionMetrics)
		m.PartitionsRefCount += atomic.LoadUint64(&ptw.refCount)
	}
	tb.ptwsLock.Unlock()
}

// ForceMergePartitions force-merges partitions in tb with names starting from the given partitionNamePrefix.
//
// Partitions are merged sequentially in order to reduce load on the system.
func (tb *table) ForceMergePartitions(partitionNamePrefix string) error {
	ptws := tb.GetPartitions(nil)
	defer tb.PutPartitions(ptws)
	for _, ptw := range ptws {
		if !strings.HasPrefix(ptw.pt.name, partitionNamePrefix) {
			continue
		}
		logger.Infof("starting forced merge for partition %q", ptw.pt.name)
		startTime := time.Now()
		if err := ptw.pt.ForceMergeAllParts(); err != nil {
			return fmt.Errorf("cannot complete forced merge for partition %q: %w", ptw.pt.name, err)
		}
		logger.Infof("forced merge for partition %q has been finished in %.3f seconds", ptw.pt.name, time.Since(startTime).Seconds())
	}
	return nil
}

// MustAddRows adds the given rows to the table tb.
func (tb *table) MustAddRows(rows []rawRow) {
	if len(rows) == 0 {
		return
	}

	// Verify whether all the rows may be added to a single partition.
	ptwsX := getPartitionWrappers()
	defer putPartitionWrappers(ptwsX)

	ptwsX.a = tb.GetPartitions(ptwsX.a[:0])
	ptws := ptwsX.a
	for i, ptw := range ptws {
		singlePt := true
		for j := range rows {
			if !ptw.pt.HasTimestamp(rows[j].Timestamp) {
				singlePt = false
				break
			}
		}
		if !singlePt {
			continue
		}

		if i != 0 {
			// Move the partition with the matching rows to the front of tb.ptws,
			// so it will be detected faster next time.
			tb.ptwsLock.Lock()
			for j := range tb.ptws {
				if ptw == tb.ptws[j] {
					tb.ptws[0], tb.ptws[j] = tb.ptws[j], tb.ptws[0]
					break
				}
			}
			tb.ptwsLock.Unlock()
		}

		// Fast path - add all the rows into the ptw.
		ptw.pt.AddRows(rows)
		tb.PutPartitions(ptws)
		return
	}

	// Slower path - split rows into per-partition buckets.
	ptBuckets := make(map[*partitionWrapper][]rawRow)
	var missingRows []rawRow
	for i := range rows {
		r := &rows[i]
		ptFound := false
		for _, ptw := range ptws {
			if ptw.pt.HasTimestamp(r.Timestamp) {
				ptBuckets[ptw] = append(ptBuckets[ptw], *r)
				ptFound = true
				break
			}
		}
		if !ptFound {
			missingRows = append(missingRows, *r)
		}
	}

	for ptw, ptRows := range ptBuckets {
		ptw.pt.AddRows(ptRows)
	}
	tb.PutPartitions(ptws)
	if len(missingRows) == 0 {
		return
	}

	// The slowest path - there are rows that don't fit any existing partition.
	// Create new partitions for these rows.
	// Do this under tb.ptwsLock.
	minTimestamp, maxTimestamp := tb.getMinMaxTimestamps()
	tb.ptwsLock.Lock()
	for i := range missingRows {
		r := &missingRows[i]

		if r.Timestamp < minTimestamp || r.Timestamp > maxTimestamp {
			// Silently skip row outside retention, since it should be deleted anyway.
			continue
		}

		// Make sure the partition for the r hasn't been added by another goroutines.
		ptFound := false
		for _, ptw := range tb.ptws {
			if ptw.pt.HasTimestamp(r.Timestamp) {
				ptFound = true
				ptw.pt.AddRows(missingRows[i : i+1])
				break
			}
		}
		if ptFound {
			continue
		}

		pt := mustCreatePartition(r.Timestamp, tb.smallPartitionsPath, tb.bigPartitionsPath, tb.s)
		pt.AddRows(missingRows[i : i+1])
		tb.addPartitionNolock(pt)
	}
	tb.ptwsLock.Unlock()
}

func (tb *table) getMinMaxTimestamps() (int64, int64) {
	now := int64(fasttime.UnixTimestamp() * 1000)
	minTimestamp := now - tb.s.retentionMsecs
	maxTimestamp := now + 2*24*3600*1000 // allow max +2 days from now due to timezones shit :)
	if minTimestamp < 0 {
		// Negative timestamps aren't supported by the storage.
		minTimestamp = 0
	}
	if maxTimestamp < 0 {
		maxTimestamp = (1 << 63) - 1
	}
	return minTimestamp, maxTimestamp
}

func (tb *table) startRetentionWatcher() {
	tb.retentionWatcherWG.Add(1)
	go func() {
		tb.retentionWatcher()
		tb.retentionWatcherWG.Done()
	}()
}

func (tb *table) retentionWatcher() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stop:
			return
		case <-ticker.C:
		}

		minTimestamp := int64(fasttime.UnixTimestamp()*1000) - tb.s.retentionMsecs
		var ptwsDrop []*partitionWrapper
		tb.ptwsLock.Lock()
		dst := tb.ptws[:0]
		for _, ptw := range tb.ptws {
			if ptw.pt.tr.MaxTimestamp < minTimestamp {
				ptwsDrop = append(ptwsDrop, ptw)
			} else {
				dst = append(dst, ptw)
			}
		}
		tb.ptws = dst
		tb.ptwsLock.Unlock()

		if len(ptwsDrop) == 0 {
			continue
		}

		// There are partitions to drop. Drop them.

		// Remove table references from partitions, so they will be eventually
		// closed and dropped after all the pending searches are done.
		for _, ptw := range ptwsDrop {
			ptw.scheduleToDrop()
			ptw.decRef()
		}
	}
}

func (tb *table) startFinalDedupWatcher() {
	tb.finalDedupWatcherWG.Add(1)
	go func() {
		tb.finalDedupWatcher()
		tb.finalDedupWatcherWG.Done()
	}()
}

func (tb *table) finalDedupWatcher() {
	if !isDedupEnabled() {
		// Deduplication is disabled.
		return
	}
	f := func() {
		ptws := tb.GetPartitions(nil)
		defer tb.PutPartitions(ptws)
		timestamp := timestampFromTime(time.Now())
		currentPartitionName := timestampToPartitionName(timestamp)
		for _, ptw := range ptws {
			if ptw.pt.name == currentPartitionName || !ptw.pt.isFinalDedupNeeded() {
				// Do not run final dedup for the current month.
				continue
			}
			if err := ptw.pt.runFinalDedup(); err != nil {
				logger.Errorf("cannot run final dedup for partition %s: %s", ptw.pt.name, err)
				continue
			}
		}
	}
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-tb.stop:
			return
		case <-t.C:
			f()
		}
	}
}

// GetPartitions appends tb's partitions snapshot to dst and returns the result.
//
// The returned partitions must be passed to PutPartitions
// when they no longer needed.
func (tb *table) GetPartitions(dst []*partitionWrapper) []*partitionWrapper {
	tb.ptwsLock.Lock()
	for _, ptw := range tb.ptws {
		ptw.incRef()
		dst = append(dst, ptw)
	}
	tb.ptwsLock.Unlock()

	return dst
}

// PutPartitions deregisters ptws obtained via GetPartitions.
func (tb *table) PutPartitions(ptws []*partitionWrapper) {
	for _, ptw := range ptws {
		ptw.decRef()
	}
}

func mustOpenPartitions(smallPartitionsPath, bigPartitionsPath string, s *Storage) []*partition {
	// Certain partition directories in either `big` or `small` dir may be missing
	// after restoring from backup. So populate partition names from both dirs.
	ptNames := make(map[string]bool)
	mustPopulatePartitionNames(smallPartitionsPath, ptNames)
	mustPopulatePartitionNames(bigPartitionsPath, ptNames)
	var pts []*partition
	for ptName := range ptNames {
		smallPartsPath := filepath.Join(smallPartitionsPath, ptName)
		bigPartsPath := filepath.Join(bigPartitionsPath, ptName)
		pt := mustOpenPartition(smallPartsPath, bigPartsPath, s)
		pts = append(pts, pt)
	}
	return pts
}

func mustPopulatePartitionNames(partitionsPath string, ptNames map[string]bool) {
	des := fs.MustReadDir(partitionsPath)
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories
			continue
		}
		ptName := de.Name()
		if ptName == snapshotsDirname {
			// Skip directory with snapshots
			continue
		}
		ptNames[ptName] = true
	}
}

type partitionWrappers struct {
	a []*partitionWrapper
}

func getPartitionWrappers() *partitionWrappers {
	v := ptwsPool.Get()
	if v == nil {
		return &partitionWrappers{}
	}
	return v.(*partitionWrappers)
}

func putPartitionWrappers(ptwsX *partitionWrappers) {
	ptwsX.a = ptwsX.a[:0]
	ptwsPool.Put(ptwsX)
}

var ptwsPool sync.Pool
