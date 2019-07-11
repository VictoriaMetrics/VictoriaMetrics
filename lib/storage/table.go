package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

// table represents a single table with time series data.
type table struct {
	path                string
	smallPartitionsPath string
	bigPartitionsPath   string

	getDeletedMetricIDs func() map[uint64]struct{}

	ptws     []*partitionWrapper
	ptwsLock sync.Mutex

	flockF *os.File

	stop chan struct{}

	retentionMilliseconds int64
	retentionWatcherWG    sync.WaitGroup
}

// partitionWrapper provides refcounting mechanism for the partition.
type partitionWrapper struct {
	pt       *partition
	refCount uint64

	// The partition must be dropped if mustDrop > 0
	mustDrop uint64
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

// openTable opens a table on the given path with the given retentionMonths.
//
// The table is created if it doesn't exist.
//
// Data older than the retentionMonths may be dropped at any time.
func openTable(path string, retentionMonths int, getDeletedMetricIDs func() map[uint64]struct{}) (*table, error) {
	path = filepath.Clean(path)

	// Create a directory for the table if it doesn't exist yet.
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create directory for table %q: %s", path, err)
	}

	flockFile := path + "/flock.lock"
	flockF, err := os.Create(flockFile)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %s", flockFile, err)
	}
	if err := unix.Flock(int(flockF.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %s", flockFile, err)
	}

	// Create directories for small and big partitions if they don't exist yet.
	smallPartitionsPath := path + "/small"
	if err := fs.MkdirAllIfNotExist(smallPartitionsPath); err != nil {
		return nil, fmt.Errorf("cannot create directory for small partitions %q: %s", smallPartitionsPath, err)
	}
	smallSnapshotsPath := smallPartitionsPath + "/snapshots"
	if err := fs.MkdirAllIfNotExist(smallSnapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %s", smallSnapshotsPath, err)
	}
	bigPartitionsPath := path + "/big"
	if err := fs.MkdirAllIfNotExist(bigPartitionsPath); err != nil {
		return nil, fmt.Errorf("cannot create directory for big partitions %q: %s", bigPartitionsPath, err)
	}
	bigSnapshotsPath := bigPartitionsPath + "/snapshots"
	if err := fs.MkdirAllIfNotExist(bigSnapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %s", bigSnapshotsPath, err)
	}

	// Open partitions.
	pts, err := openPartitions(smallPartitionsPath, bigPartitionsPath, getDeletedMetricIDs)
	if err != nil {
		return nil, fmt.Errorf("cannot open partitions in the table %q: %s", path, err)
	}

	tb := &table{
		path:                path,
		smallPartitionsPath: smallPartitionsPath,
		bigPartitionsPath:   bigPartitionsPath,
		getDeletedMetricIDs: getDeletedMetricIDs,

		flockF: flockF,

		stop: make(chan struct{}),
	}
	for _, pt := range pts {
		tb.addPartitionNolock(pt)
	}
	if retentionMonths <= 0 || retentionMonths > maxRetentionMonths {
		retentionMonths = maxRetentionMonths
	}
	tb.retentionMilliseconds = int64(retentionMonths) * 31 * 24 * 3600 * 1e3

	tb.startRetentionWatcher()
	return tb, nil
}

// CreateSnapshot creates tb snapshot and returns paths to small and big parts of it.
func (tb *table) CreateSnapshot(snapshotName string) (string, string, error) {
	logger.Infof("creating table snapshot of %q...", tb.path)
	startTime := time.Now()

	ptws := tb.GetPartitions(nil)
	defer tb.PutPartitions(ptws)

	dstSmallDir := fmt.Sprintf("%s/small/snapshots/%s", tb.path, snapshotName)
	if err := fs.MkdirAllFailIfExist(dstSmallDir); err != nil {
		return "", "", fmt.Errorf("cannot create dir %q: %s", dstSmallDir, err)
	}
	dstBigDir := fmt.Sprintf("%s/big/snapshots/%s", tb.path, snapshotName)
	if err := fs.MkdirAllFailIfExist(dstBigDir); err != nil {
		return "", "", fmt.Errorf("cannot create dir %q: %s", dstBigDir, err)
	}

	for _, ptw := range ptws {
		smallPath := dstSmallDir + "/" + ptw.pt.name
		bigPath := dstBigDir + "/" + ptw.pt.name
		if err := ptw.pt.CreateSnapshotAt(smallPath, bigPath); err != nil {
			return "", "", fmt.Errorf("cannot create snapshot for partition %q in %q: %s", ptw.pt.name, tb.path, err)
		}
	}

	fs.MustSyncPath(dstSmallDir)
	fs.MustSyncPath(dstBigDir)
	fs.MustSyncPath(filepath.Dir(dstSmallDir))
	fs.MustSyncPath(filepath.Dir(dstBigDir))

	logger.Infof("created table snapshot for %q at (%q, %q) in %s", tb.path, dstSmallDir, dstBigDir, time.Since(startTime))
	return dstSmallDir, dstBigDir, nil
}

// MustDeleteSnapshot deletes snapshot with the given snapshotName.
func (tb *table) MustDeleteSnapshot(snapshotName string) {
	smallDir := fmt.Sprintf("%s/small/snapshots/%s", tb.path, snapshotName)
	fs.MustRemoveAll(smallDir)
	bigDir := fmt.Sprintf("%s/big/snapshots/%s", tb.path, snapshotName)
	fs.MustRemoveAll(bigDir)
}

func (tb *table) addPartitionNolock(pt *partition) {
	ptw := &partitionWrapper{
		pt:       pt,
		refCount: 1,
	}
	tb.ptws = append(tb.ptws, ptw)
}

// MustClose closes the table.
func (tb *table) MustClose() {
	close(tb.stop)
	tb.retentionWatcherWG.Wait()

	tb.ptwsLock.Lock()
	ptws := tb.ptws
	tb.ptws = nil
	tb.ptwsLock.Unlock()

	// Decrement references to partitions, so they may be eventually closed after
	// pending searches are done.
	for _, ptw := range ptws {
		ptw.decRef()
	}

	// Release exclusive lock on the table.
	if err := tb.flockF.Close(); err != nil {
		logger.Panicf("FATAL: cannot release lock on %q: %s", tb.flockF.Name(), err)
	}
}

// flushRawRows flushes all the pending rows, so they become visible to search.
//
// This function is for debug purposes only.
func (tb *table) flushRawRows() {
	ptws := tb.GetPartitions(nil)
	defer tb.PutPartitions(ptws)

	for _, ptw := range ptws {
		ptw.pt.flushRawRows(nil, true)
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

// AddRows adds the given rows to the table tb.
func (tb *table) AddRows(rows []rawRow) error {
	if len(rows) == 0 {
		return nil
	}

	// Verify whether all the rows may be added to a single partition.
	ptwsX := getPartitionWrappers()
	defer putPartitionWrappers(ptwsX)

	ptwsX.a = tb.GetPartitions(ptwsX.a[:0])
	ptws := ptwsX.a
	for _, ptw := range ptws {
		singlePt := true
		for i := range rows {
			if !ptw.pt.HasTimestamp(rows[i].Timestamp) {
				singlePt = false
				break
			}
		}
		if !singlePt {
			continue
		}

		// Move the partition with the matching rows to the front of tb.ptws,
		// so it will be detected faster next time.
		tb.ptwsLock.Lock()
		for i := range tb.ptws {
			if ptw == tb.ptws[i] {
				tb.ptws[0], tb.ptws[i] = tb.ptws[i], tb.ptws[0]
				break
			}
		}
		tb.ptwsLock.Unlock()

		// Fast path - add all the rows into the ptw.
		ptw.pt.AddRows(rows)
		tb.PutPartitions(ptws)
		return nil
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
		return nil
	}

	// The slowest path - there are rows that don't fit any existing partition.
	// Create new partitions for these rows.
	// Do this under tb.ptwsLock.
	minTimestamp, maxTimestamp := tb.getMinMaxTimestamps()
	tb.ptwsLock.Lock()
	var errors []error
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

		pt, err := createPartition(r.Timestamp, tb.smallPartitionsPath, tb.bigPartitionsPath, tb.getDeletedMetricIDs)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		pt.AddRows(missingRows[i : i+1])
		tb.addPartitionNolock(pt)
	}
	tb.ptwsLock.Unlock()

	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		return fmt.Errorf("errors while adding rows to table %q: %s", tb.path, errors[0])
	}
	return nil
}

func (tb *table) getMinMaxTimestamps() (int64, int64) {
	now := timestampFromTime(time.Now())
	minTimestamp := now - tb.retentionMilliseconds
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
	t := time.NewTimer(time.Minute)
	for {
		select {
		case <-tb.stop:
			return
		case <-t.C:
			t.Reset(time.Minute)
		}

		minTimestamp := timestampFromTime(time.Now()) - tb.retentionMilliseconds
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

		// There are paritions to drop. Drop them.

		// Remove table references from partitions, so they will be eventually
		// closed and dropped after all the pending searches are done.
		for _, ptw := range ptwsDrop {
			ptw.scheduleToDrop()
			ptw.decRef()
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

func openPartitions(smallPartitionsPath, bigPartitionsPath string, getDeletedMetricIDs func() map[uint64]struct{}) ([]*partition, error) {
	smallD, err := os.Open(smallPartitionsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open directory with small partitions %q: %s", smallPartitionsPath, err)
	}
	defer fs.MustClose(smallD)

	fis, err := smallD.Readdir(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory with small partitions %q: %s", smallPartitionsPath, err)
	}
	var pts []*partition
	for _, fi := range fis {
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories
			continue
		}
		ptName := fi.Name()
		if ptName == "snapshots" {
			// Skipe directory with snapshots
			continue
		}
		smallPartsPath := smallPartitionsPath + "/" + ptName
		bigPartsPath := bigPartitionsPath + "/" + ptName
		pt, err := openPartition(smallPartsPath, bigPartsPath, getDeletedMetricIDs)
		if err != nil {
			mustClosePartitions(pts)
			return nil, fmt.Errorf("cannot open partition %q: %s", ptName, err)
		}
		pts = append(pts, pt)
	}
	return pts, nil
}

func mustClosePartitions(pts []*partition) {
	for _, pt := range pts {
		pt.MustClose()
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
