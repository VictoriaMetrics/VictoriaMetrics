package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storagepacelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
)

const (
	msecsPerMonth     = 31 * 24 * 3600 * 1000
	maxRetentionMsecs = 100 * 12 * msecsPerMonth
)

// Storage represents TSDB storage.
type Storage struct {
	// Atomic counters must go at the top of the structure in order to properly align by 8 bytes on 32-bit archs.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212 .
	tooSmallTimestampRows uint64
	tooBigTimestampRows   uint64

	addRowsConcurrencyLimitReached uint64
	addRowsConcurrencyLimitTimeout uint64
	addRowsConcurrencyDroppedRows  uint64

	searchTSIDsConcurrencyLimitReached uint64
	searchTSIDsConcurrencyLimitTimeout uint64

	slowRowInserts         uint64
	slowPerDayIndexInserts uint64
	slowMetricNameLoads    uint64

	hourlySeriesLimitRowsDropped uint64
	dailySeriesLimitRowsDropped  uint64

	path           string
	cachePath      string
	retentionMsecs int64

	// lock file for exclusive access to the storage on the given path.
	flockF *os.File

	idbCurr atomic.Value

	tb *table

	// Series cardinality limiters.
	hourlySeriesLimiter *bloomfilter.Limiter
	dailySeriesLimiter  *bloomfilter.Limiter

	// tsidCache is MetricName -> TSID cache.
	tsidCache *workingsetcache.Cache

	// metricIDCache is MetricID -> TSID cache.
	metricIDCache *workingsetcache.Cache

	// metricNameCache is MetricID -> MetricName cache.
	metricNameCache *workingsetcache.Cache

	// dateMetricIDCache is (Date, MetricID) cache.
	dateMetricIDCache *dateMetricIDCache

	// Fast cache for MetricID values occurred during the current hour.
	currHourMetricIDs atomic.Value

	// Fast cache for MetricID values occurred during the previous hour.
	prevHourMetricIDs atomic.Value

	// Fast cache for pre-populating per-day inverted index for the next day.
	// This is needed in order to remove CPU usage spikes at 00:00 UTC
	// due to creation of per-day inverted index for active time series.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 for details.
	nextDayMetricIDs atomic.Value

	// Pending MetricID values to be added to currHourMetricIDs.
	pendingHourEntriesLock sync.Mutex
	pendingHourEntries     *uint64set.Set

	// Pending MetricIDs to be added to nextDayMetricIDs.
	pendingNextDayMetricIDsLock sync.Mutex
	pendingNextDayMetricIDs     *uint64set.Set

	// prefetchedMetricIDs contains metricIDs for pre-fetched metricNames in the prefetchMetricNames function.
	prefetchedMetricIDs atomic.Value

	// prefetchedMetricIDsDeadline is used for periodic reset of prefetchedMetricIDs in order to limit its size under high rate of creating new series.
	prefetchedMetricIDsDeadline uint64

	// prefetchedMetricIDsLock is used for serializing updates of prefetchedMetricIDs from concurrent goroutines.
	prefetchedMetricIDsLock sync.Mutex

	stop chan struct{}

	currHourMetricIDsUpdaterWG sync.WaitGroup
	nextDayMetricIDsUpdaterWG  sync.WaitGroup
	retentionWatcherWG         sync.WaitGroup
	freeDiskSpaceWatcherWG     sync.WaitGroup

	// The snapshotLock prevents from concurrent creation of snapshots,
	// since this may result in snapshots without recently added data,
	// which may be in the process of flushing to disk by concurrently running
	// snapshot process.
	snapshotLock sync.Mutex

	// The minimum timestamp when composite index search can be used.
	minTimestampForCompositeIndex int64

	// An inmemory set of deleted metricIDs.
	//
	// It is safe to keep the set in memory even for big number of deleted
	// metricIDs, since it usually requires 1 bit per deleted metricID.
	deletedMetricIDs           atomic.Value
	deletedMetricIDsUpdateLock sync.Mutex

	isReadOnly uint32
}

// OpenStorage opens storage on the given path with the given retentionMsecs.
func OpenStorage(path string, retentionMsecs int64, maxHourlySeries, maxDailySeries int) (*Storage, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot determine absolute path for %q: %w", path, err)
	}
	if retentionMsecs <= 0 {
		retentionMsecs = maxRetentionMsecs
	}
	if retentionMsecs > maxRetentionMsecs {
		retentionMsecs = maxRetentionMsecs
	}
	s := &Storage{
		path:           path,
		cachePath:      path + "/cache",
		retentionMsecs: retentionMsecs,
		stop:           make(chan struct{}),
	}
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create a directory for the storage at %q: %w", path, err)
	}

	// Check whether the cache directory must be removed
	// It is removed if it contains reset_cache_on_startup file.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1447 for details.
	if fs.IsPathExist(s.cachePath + "/reset_cache_on_startup") {
		logger.Infof("removing cache directory at %q, since it contains `reset_cache_on_startup` file...", s.cachePath)
		wg := getWaitGroup()
		wg.Add(1)
		fs.MustRemoveAllWithDoneCallback(s.cachePath, wg.Done)
		wg.Wait()
		putWaitGroup(wg)
		logger.Infof("cache directory at %q has been successfully removed", s.cachePath)
	}

	// Protect from concurrent opens.
	flockF, err := fs.CreateFlockFile(path)
	if err != nil {
		return nil, err
	}
	s.flockF = flockF

	// Check whether restore process finished successfully
	restoreLockF := path + "/restore-in-progress"
	if fs.IsPathExist(restoreLockF) {
		return nil, fmt.Errorf("restore lock file exists, incomplete vmrestore run. Run vmrestore again or remove lock file %q", restoreLockF)
	}

	// Pre-create snapshots directory if it is missing.
	snapshotsPath := path + "/snapshots"
	if err := fs.MkdirAllIfNotExist(snapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", snapshotsPath, err)
	}

	// Initialize series cardinality limiter.
	if maxHourlySeries > 0 {
		s.hourlySeriesLimiter = bloomfilter.NewLimiter(maxHourlySeries, time.Hour)
	}
	if maxDailySeries > 0 {
		s.dailySeriesLimiter = bloomfilter.NewLimiter(maxDailySeries, 24*time.Hour)
	}

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache("MetricName->TSID", "metricName_tsid", getTSIDCacheSize())
	s.metricIDCache = s.mustLoadCache("MetricID->TSID", "metricID_tsid", mem/16)
	s.metricNameCache = s.mustLoadCache("MetricID->MetricName", "metricID_metricName", mem/10)
	s.dateMetricIDCache = newDateMetricIDCache()

	hour := fasttime.UnixHour()
	hmCurr := s.mustLoadHourMetricIDs(hour, "curr_hour_metric_ids")
	hmPrev := s.mustLoadHourMetricIDs(hour-1, "prev_hour_metric_ids")
	s.currHourMetricIDs.Store(hmCurr)
	s.prevHourMetricIDs.Store(hmPrev)
	s.pendingHourEntries = &uint64set.Set{}

	date := fasttime.UnixDate()
	nextDayMetricIDs := s.mustLoadNextDayMetricIDs(date)
	s.nextDayMetricIDs.Store(nextDayMetricIDs)
	s.pendingNextDayMetricIDs = &uint64set.Set{}

	s.prefetchedMetricIDs.Store(&uint64set.Set{})

	// Load metadata
	metadataDir := path + "/metadata"
	isEmptyDB := !fs.IsPathExist(path + "/indexdb")
	if err := fs.MkdirAllIfNotExist(metadataDir); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", metadataDir, err)
	}
	s.minTimestampForCompositeIndex = mustGetMinTimestampForCompositeIndex(metadataDir, isEmptyDB)

	// Load indexdb
	idbPath := path + "/indexdb"
	idbSnapshotsPath := idbPath + "/snapshots"
	if err := fs.MkdirAllIfNotExist(idbSnapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", idbSnapshotsPath, err)
	}
	idbCurr, idbPrev, err := s.openIndexDBTables(idbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open indexdb tables at %q: %w", idbPath, err)
	}
	idbCurr.SetExtDB(idbPrev)
	s.idbCurr.Store(idbCurr)

	// Load deleted metricIDs from idbCurr and idbPrev
	dmisCurr, err := idbCurr.loadDeletedMetricIDs()
	if err != nil {
		return nil, fmt.Errorf("cannot load deleted metricIDs for the current indexDB: %w", err)
	}
	dmisPrev, err := idbPrev.loadDeletedMetricIDs()
	if err != nil {
		return nil, fmt.Errorf("cannot load deleted metricIDs for the previous indexDB: %w", err)
	}
	s.setDeletedMetricIDs(dmisCurr)
	s.updateDeletedMetricIDs(dmisPrev)

	// Load data
	tablePath := path + "/data"
	tb, err := openTable(tablePath, s.getDeletedMetricIDs, retentionMsecs, &s.isReadOnly)
	if err != nil {
		s.idb().MustClose()
		return nil, fmt.Errorf("cannot open table at %q: %w", tablePath, err)
	}
	s.tb = tb

	s.startCurrHourMetricIDsUpdater()
	s.startNextDayMetricIDsUpdater()
	s.startRetentionWatcher()
	s.startFreeDiskSpaceWatcher()

	return s, nil
}

var maxTSIDCacheSize int

// SetTSIDCacheSize overrides the default size of storage/tsid cache
func SetTSIDCacheSize(size int) {
	maxTSIDCacheSize = size
}

func getTSIDCacheSize() int {
	if maxTSIDCacheSize <= 0 {
		return int(float64(memory.Allowed()) * 0.37)
	}
	return maxTSIDCacheSize
}

func (s *Storage) getDeletedMetricIDs() *uint64set.Set {
	return s.deletedMetricIDs.Load().(*uint64set.Set)
}

func (s *Storage) setDeletedMetricIDs(dmis *uint64set.Set) {
	s.deletedMetricIDs.Store(dmis)
}

func (s *Storage) updateDeletedMetricIDs(metricIDs *uint64set.Set) {
	s.deletedMetricIDsUpdateLock.Lock()
	dmisOld := s.getDeletedMetricIDs()
	dmisNew := dmisOld.Clone()
	dmisNew.Union(metricIDs)
	s.setDeletedMetricIDs(dmisNew)
	s.deletedMetricIDsUpdateLock.Unlock()
}

// DebugFlush flushes recently added storage data, so it becomes visible to search.
func (s *Storage) DebugFlush() {
	s.tb.flushRawRows()
	s.idb().tb.DebugFlush()
}

// CreateSnapshot creates snapshot for s and returns the snapshot name.
func (s *Storage) CreateSnapshot() (string, error) {
	logger.Infof("creating Storage snapshot for %q...", s.path)
	startTime := time.Now()

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	snapshotName := snapshot.NewName()
	srcDir := s.path
	dstDir := fmt.Sprintf("%s/snapshots/%s", srcDir, snapshotName)
	if err := fs.MkdirAllFailIfExist(dstDir); err != nil {
		return "", fmt.Errorf("cannot create dir %q: %w", dstDir, err)
	}
	dstDataDir := dstDir + "/data"
	if err := fs.MkdirAllFailIfExist(dstDataDir); err != nil {
		return "", fmt.Errorf("cannot create dir %q: %w", dstDataDir, err)
	}

	smallDir, bigDir, err := s.tb.CreateSnapshot(snapshotName)
	if err != nil {
		return "", fmt.Errorf("cannot create table snapshot: %w", err)
	}
	dstSmallDir := dstDataDir + "/small"
	if err := fs.SymlinkRelative(smallDir, dstSmallDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %w", smallDir, dstSmallDir, err)
	}
	dstBigDir := dstDataDir + "/big"
	if err := fs.SymlinkRelative(bigDir, dstBigDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %w", bigDir, dstBigDir, err)
	}
	fs.MustSyncPath(dstDataDir)

	idbSnapshot := fmt.Sprintf("%s/indexdb/snapshots/%s", srcDir, snapshotName)
	idb := s.idb()
	currSnapshot := idbSnapshot + "/" + idb.name
	if err := idb.tb.CreateSnapshotAt(currSnapshot); err != nil {
		return "", fmt.Errorf("cannot create curr indexDB snapshot: %w", err)
	}
	ok := idb.doExtDB(func(extDB *indexDB) {
		prevSnapshot := idbSnapshot + "/" + extDB.name
		err = extDB.tb.CreateSnapshotAt(prevSnapshot)
	})
	if ok && err != nil {
		return "", fmt.Errorf("cannot create prev indexDB snapshot: %w", err)
	}
	dstIdbDir := dstDir + "/indexdb"
	if err := fs.SymlinkRelative(idbSnapshot, dstIdbDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %w", idbSnapshot, dstIdbDir, err)
	}

	srcMetadataDir := srcDir + "/metadata"
	dstMetadataDir := dstDir + "/metadata"
	if err := fs.CopyDirectory(srcMetadataDir, dstMetadataDir); err != nil {
		return "", fmt.Errorf("cannot copy metadata: %w", err)
	}

	fs.MustSyncPath(dstDir)

	logger.Infof("created Storage snapshot for %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	return snapshotName, nil
}

// ListSnapshots returns sorted list of existing snapshots for s.
func (s *Storage) ListSnapshots() ([]string, error) {
	snapshotsPath := s.path + "/snapshots"
	d, err := os.Open(snapshotsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", snapshotsPath, err)
	}
	defer fs.MustClose(d)

	fnames, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read contents of %q: %w", snapshotsPath, err)
	}
	snapshotNames := make([]string, 0, len(fnames))
	for _, fname := range fnames {
		if err := snapshot.Validate(fname); err != nil {
			continue
		}
		snapshotNames = append(snapshotNames, fname)
	}
	sort.Strings(snapshotNames)
	return snapshotNames, nil
}

// DeleteSnapshot deletes the given snapshot.
func (s *Storage) DeleteSnapshot(snapshotName string) error {
	if err := snapshot.Validate(snapshotName); err != nil {
		return fmt.Errorf("invalid snapshotName %q: %w", snapshotName, err)
	}
	snapshotPath := s.path + "/snapshots/" + snapshotName

	logger.Infof("deleting snapshot %q...", snapshotPath)
	startTime := time.Now()

	s.tb.MustDeleteSnapshot(snapshotName)
	idbPath := fmt.Sprintf("%s/indexdb/snapshots/%s", s.path, snapshotName)
	fs.MustRemoveAll(idbPath)
	fs.MustRemoveAll(snapshotPath)

	logger.Infof("deleted snapshot %q in %.3f seconds", snapshotPath, time.Since(startTime).Seconds())

	return nil
}

// DeleteStaleSnapshots deletes snapshot older than given maxAge
func (s *Storage) DeleteStaleSnapshots(maxAge time.Duration) error {
	list, err := s.ListSnapshots()
	if err != nil {
		return err
	}
	expireDeadline := time.Now().UTC().Add(-maxAge)
	for _, snapshotName := range list {
		t, err := snapshot.Time(snapshotName)
		if err != nil {
			return fmt.Errorf("cannot parse snapshot date from %q: %w", snapshotName, err)
		}
		if t.Before(expireDeadline) {
			if err := s.DeleteSnapshot(snapshotName); err != nil {
				return fmt.Errorf("cannot delete snapshot %q: %w", snapshotName, err)
			}
		}
	}
	return nil
}

func (s *Storage) idb() *indexDB {
	return s.idbCurr.Load().(*indexDB)
}

// Metrics contains essential metrics for the Storage.
type Metrics struct {
	RowsAddedTotal    uint64
	DedupsDuringMerge uint64

	TooSmallTimestampRows uint64
	TooBigTimestampRows   uint64

	AddRowsConcurrencyLimitReached uint64
	AddRowsConcurrencyLimitTimeout uint64
	AddRowsConcurrencyDroppedRows  uint64
	AddRowsConcurrencyCapacity     uint64
	AddRowsConcurrencyCurrent      uint64

	SearchTSIDsConcurrencyLimitReached uint64
	SearchTSIDsConcurrencyLimitTimeout uint64
	SearchTSIDsConcurrencyCapacity     uint64
	SearchTSIDsConcurrencyCurrent      uint64

	SearchDelays uint64

	SlowRowInserts         uint64
	SlowPerDayIndexInserts uint64
	SlowMetricNameLoads    uint64

	HourlySeriesLimitRowsDropped uint64
	DailySeriesLimitRowsDropped  uint64

	TimestampsBlocksMerged uint64
	TimestampsBytesSaved   uint64

	TSIDCacheSize         uint64
	TSIDCacheSizeBytes    uint64
	TSIDCacheSizeMaxBytes uint64
	TSIDCacheRequests     uint64
	TSIDCacheMisses       uint64
	TSIDCacheCollisions   uint64

	MetricIDCacheSize         uint64
	MetricIDCacheSizeBytes    uint64
	MetricIDCacheSizeMaxBytes uint64
	MetricIDCacheRequests     uint64
	MetricIDCacheMisses       uint64
	MetricIDCacheCollisions   uint64

	MetricNameCacheSize         uint64
	MetricNameCacheSizeBytes    uint64
	MetricNameCacheSizeMaxBytes uint64
	MetricNameCacheRequests     uint64
	MetricNameCacheMisses       uint64
	MetricNameCacheCollisions   uint64

	DateMetricIDCacheSize        uint64
	DateMetricIDCacheSizeBytes   uint64
	DateMetricIDCacheSyncsCount  uint64
	DateMetricIDCacheResetsCount uint64

	HourMetricIDCacheSize      uint64
	HourMetricIDCacheSizeBytes uint64

	NextDayMetricIDCacheSize      uint64
	NextDayMetricIDCacheSizeBytes uint64

	PrefetchedMetricIDsSize      uint64
	PrefetchedMetricIDsSizeBytes uint64

	NextRetentionSeconds uint64

	IndexDBMetrics IndexDBMetrics
	TableMetrics   TableMetrics
}

// Reset resets m.
func (m *Metrics) Reset() {
	*m = Metrics{}
}

// UpdateMetrics updates m with metrics from s.
func (s *Storage) UpdateMetrics(m *Metrics) {
	m.RowsAddedTotal = atomic.LoadUint64(&rowsAddedTotal)
	m.DedupsDuringMerge = atomic.LoadUint64(&dedupsDuringMerge)

	m.TooSmallTimestampRows += atomic.LoadUint64(&s.tooSmallTimestampRows)
	m.TooBigTimestampRows += atomic.LoadUint64(&s.tooBigTimestampRows)

	m.AddRowsConcurrencyLimitReached += atomic.LoadUint64(&s.addRowsConcurrencyLimitReached)
	m.AddRowsConcurrencyLimitTimeout += atomic.LoadUint64(&s.addRowsConcurrencyLimitTimeout)
	m.AddRowsConcurrencyDroppedRows += atomic.LoadUint64(&s.addRowsConcurrencyDroppedRows)
	m.AddRowsConcurrencyCapacity = uint64(cap(addRowsConcurrencyCh))
	m.AddRowsConcurrencyCurrent = uint64(len(addRowsConcurrencyCh))

	m.SearchTSIDsConcurrencyLimitReached += atomic.LoadUint64(&s.searchTSIDsConcurrencyLimitReached)
	m.SearchTSIDsConcurrencyLimitTimeout += atomic.LoadUint64(&s.searchTSIDsConcurrencyLimitTimeout)
	m.SearchTSIDsConcurrencyCapacity = uint64(cap(searchTSIDsConcurrencyCh))
	m.SearchTSIDsConcurrencyCurrent = uint64(len(searchTSIDsConcurrencyCh))

	m.SearchDelays = storagepacelimiter.Search.DelaysTotal()

	m.SlowRowInserts += atomic.LoadUint64(&s.slowRowInserts)
	m.SlowPerDayIndexInserts += atomic.LoadUint64(&s.slowPerDayIndexInserts)
	m.SlowMetricNameLoads += atomic.LoadUint64(&s.slowMetricNameLoads)

	m.HourlySeriesLimitRowsDropped += atomic.LoadUint64(&s.hourlySeriesLimitRowsDropped)
	m.DailySeriesLimitRowsDropped += atomic.LoadUint64(&s.dailySeriesLimitRowsDropped)

	m.TimestampsBlocksMerged = atomic.LoadUint64(&timestampsBlocksMerged)
	m.TimestampsBytesSaved = atomic.LoadUint64(&timestampsBytesSaved)

	var cs fastcache.Stats
	s.tsidCache.UpdateStats(&cs)
	m.TSIDCacheSize += cs.EntriesCount
	m.TSIDCacheSizeBytes += cs.BytesSize
	m.TSIDCacheSizeMaxBytes += cs.MaxBytesSize
	m.TSIDCacheRequests += cs.GetCalls
	m.TSIDCacheMisses += cs.Misses
	m.TSIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricIDCache.UpdateStats(&cs)
	m.MetricIDCacheSize += cs.EntriesCount
	m.MetricIDCacheSizeBytes += cs.BytesSize
	m.MetricIDCacheSizeMaxBytes += cs.MaxBytesSize
	m.MetricIDCacheRequests += cs.GetCalls
	m.MetricIDCacheMisses += cs.Misses
	m.MetricIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricNameCache.UpdateStats(&cs)
	m.MetricNameCacheSize += cs.EntriesCount
	m.MetricNameCacheSizeBytes += cs.BytesSize
	m.MetricNameCacheSizeMaxBytes += cs.MaxBytesSize
	m.MetricNameCacheRequests += cs.GetCalls
	m.MetricNameCacheMisses += cs.Misses
	m.MetricNameCacheCollisions += cs.Collisions

	m.DateMetricIDCacheSize += uint64(s.dateMetricIDCache.EntriesCount())
	m.DateMetricIDCacheSizeBytes += uint64(s.dateMetricIDCache.SizeBytes())
	m.DateMetricIDCacheSyncsCount += atomic.LoadUint64(&s.dateMetricIDCache.syncsCount)
	m.DateMetricIDCacheResetsCount += atomic.LoadUint64(&s.dateMetricIDCache.resetsCount)

	hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
	hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
	hourMetricIDsLen := hmPrev.m.Len()
	if hmCurr.m.Len() > hourMetricIDsLen {
		hourMetricIDsLen = hmCurr.m.Len()
	}
	m.HourMetricIDCacheSize += uint64(hourMetricIDsLen)
	m.HourMetricIDCacheSizeBytes += hmCurr.m.SizeBytes()
	m.HourMetricIDCacheSizeBytes += hmPrev.m.SizeBytes()

	nextDayMetricIDs := &s.nextDayMetricIDs.Load().(*byDateMetricIDEntry).v
	m.NextDayMetricIDCacheSize += uint64(nextDayMetricIDs.Len())
	m.NextDayMetricIDCacheSizeBytes += nextDayMetricIDs.SizeBytes()

	prefetchedMetricIDs := s.prefetchedMetricIDs.Load().(*uint64set.Set)
	m.PrefetchedMetricIDsSize += uint64(prefetchedMetricIDs.Len())
	m.PrefetchedMetricIDsSizeBytes += uint64(prefetchedMetricIDs.SizeBytes())

	m.NextRetentionSeconds = uint64(nextRetentionDuration(s.retentionMsecs).Seconds())

	s.idb().UpdateMetrics(&m.IndexDBMetrics)
	s.tb.UpdateMetrics(&m.TableMetrics)
}

// SetFreeDiskSpaceLimit sets the minimum free disk space size of current storage path
//
// The function must be called before opening or creating any storage.
func SetFreeDiskSpaceLimit(bytes int) {
	freeDiskSpaceLimitBytes = uint64(bytes)
}

var freeDiskSpaceLimitBytes uint64

// IsReadOnly returns information is storage in read only mode
func (s *Storage) IsReadOnly() bool {
	return atomic.LoadUint32(&s.isReadOnly) == 1
}

func (s *Storage) startFreeDiskSpaceWatcher() {
	f := func() {
		freeSpaceBytes := fs.MustGetFreeSpace(s.path)
		if freeSpaceBytes < freeDiskSpaceLimitBytes {
			// Switch the storage to readonly mode if there is no enough free space left at s.path
			logger.Warnf("switching the storage at %s to read-only mode, since it has less than -storage.minFreeDiskSpaceBytes=%d of free space: %d bytes left",
				s.path, freeDiskSpaceLimitBytes, freeSpaceBytes)
			atomic.StoreUint32(&s.isReadOnly, 1)
			return
		}
		if atomic.CompareAndSwapUint32(&s.isReadOnly, 1, 0) {
			logger.Warnf("enabling writing to the storage at %s, since it has more than -storage.minFreeDiskSpaceBytes=%d of free space: %d bytes left",
				s.path, freeDiskSpaceLimitBytes, freeSpaceBytes)
		}
	}
	f()
	s.freeDiskSpaceWatcherWG.Add(1)
	go func() {
		defer s.freeDiskSpaceWatcherWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				f()
			}
		}
	}()
}

func (s *Storage) startRetentionWatcher() {
	s.retentionWatcherWG.Add(1)
	go func() {
		s.retentionWatcher()
		s.retentionWatcherWG.Done()
	}()
}

func (s *Storage) retentionWatcher() {
	for {
		d := nextRetentionDuration(s.retentionMsecs)
		select {
		case <-s.stop:
			return
		case <-time.After(d):
			s.mustRotateIndexDB()
		}
	}
}

func (s *Storage) startCurrHourMetricIDsUpdater() {
	s.currHourMetricIDsUpdaterWG.Add(1)
	go func() {
		s.currHourMetricIDsUpdater()
		s.currHourMetricIDsUpdaterWG.Done()
	}()
}

func (s *Storage) startNextDayMetricIDsUpdater() {
	s.nextDayMetricIDsUpdaterWG.Add(1)
	go func() {
		s.nextDayMetricIDsUpdater()
		s.nextDayMetricIDsUpdaterWG.Done()
	}()
}

var currHourMetricIDsUpdateInterval = time.Second * 10

func (s *Storage) currHourMetricIDsUpdater() {
	ticker := time.NewTicker(currHourMetricIDsUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.updateCurrHourMetricIDs()
			return
		case <-ticker.C:
			s.updateCurrHourMetricIDs()
		}
	}
}

var nextDayMetricIDsUpdateInterval = time.Second * 11

func (s *Storage) nextDayMetricIDsUpdater() {
	ticker := time.NewTicker(nextDayMetricIDsUpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.updateNextDayMetricIDs()
			return
		case <-ticker.C:
			s.updateNextDayMetricIDs()
		}
	}
}

func (s *Storage) mustRotateIndexDB() {
	// Create new indexdb table.
	newTableName := nextIndexDBTableName()
	idbNewPath := s.path + "/indexdb/" + newTableName
	rotationTimestamp := fasttime.UnixTimestamp()
	idbNew, err := openIndexDB(idbNewPath, s, rotationTimestamp, &s.isReadOnly)
	if err != nil {
		logger.Panicf("FATAL: cannot create new indexDB at %q: %s", idbNewPath, err)
	}

	// Drop extDB
	idbCurr := s.idb()
	idbCurr.doExtDB(func(extDB *indexDB) {
		extDB.scheduleToDrop()
	})
	idbCurr.SetExtDB(nil)

	// Start using idbNew
	idbNew.SetExtDB(idbCurr)
	s.idbCurr.Store(idbNew)

	// Persist changes on the file system.
	fs.MustSyncPath(s.path)

	// Do not flush tsidCache to avoid read/write path slowdown
	// and slowly re-populate new idb with entries from the cache via maybeCreateIndexes().
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401

	// Flush metric id caches for the current and the previous hour,
	// since they may contain entries missing in idbNew.
	// This should prevent from missing data in queries when
	// the following steps are performed for short -retentionPeriod (e.g. 1 day):
	//
	// 1. Add samples for some series between 3-4 UTC. These series are registered in currHourMetricIDs.
	// 2. The indexdb rotation is performed at 4 UTC. currHourMetricIDs is moved to prevHourMetricIDs.
	// 3. Continue adding samples for series from step 1 during time range 4-5 UTC.
	//    These series are already registered in prevHourMetricIDs, so VM doesn't add per-day entries to the current indexdb.
	// 4. Stop adding new samples for these series just before 5 UTC.
	// 5. The next indexdb rotation is performed at 4 UTC next day.
	//    The information about the series from step 5 disappears from indexdb, since the old indexdb from step 1 is deleted,
	//    while the current indexdb doesn't contain information about the series.
	//    So queries for the last 24 hours stop returning samples added at step 3.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2698
	s.pendingHourEntriesLock.Lock()
	s.pendingHourEntries = &uint64set.Set{}
	s.pendingHourEntriesLock.Unlock()
	s.currHourMetricIDs.Store(&hourMetricIDs{})
	s.prevHourMetricIDs.Store(&hourMetricIDs{})

	// Flush dateMetricIDCache, so idbNew can be populated with fresh data.
	s.dateMetricIDCache.Reset()

	// Do not flush metricIDCache and metricNameCache, since all the metricIDs
	// from prev idb remain valid after the rotation.

	// There is no need in resetting nextDayMetricIDs, since it should be automatically reset every day.
}

func (s *Storage) resetAndSaveTSIDCache() {
	// Reset cache and then store the reset cache on disk in order to prevent
	// from inconsistent behaviour after possible unclean shutdown.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347
	s.tsidCache.Reset()
	s.mustSaveCache(s.tsidCache, "MetricName->TSID", "metricName_tsid")
}

// MustClose closes the storage.
//
// It is expected that the s is no longer used during the close.
func (s *Storage) MustClose() {
	close(s.stop)

	s.freeDiskSpaceWatcherWG.Wait()
	s.retentionWatcherWG.Wait()
	s.currHourMetricIDsUpdaterWG.Wait()
	s.nextDayMetricIDsUpdaterWG.Wait()

	s.tb.MustClose()
	s.idb().MustClose()

	// Save caches.
	s.mustSaveCache(s.tsidCache, "MetricName->TSID", "metricName_tsid")
	s.tsidCache.Stop()
	s.mustSaveCache(s.metricIDCache, "MetricID->TSID", "metricID_tsid")
	s.metricIDCache.Stop()
	s.mustSaveCache(s.metricNameCache, "MetricID->MetricName", "metricID_metricName")
	s.metricNameCache.Stop()

	hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
	s.mustSaveHourMetricIDs(hmCurr, "curr_hour_metric_ids")
	hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
	s.mustSaveHourMetricIDs(hmPrev, "prev_hour_metric_ids")

	nextDayMetricIDs := s.nextDayMetricIDs.Load().(*byDateMetricIDEntry)
	s.mustSaveNextDayMetricIDs(nextDayMetricIDs)

	// Release lock file.
	if err := s.flockF.Close(); err != nil {
		logger.Panicf("FATAL: cannot close lock file %q: %s", s.flockF.Name(), err)
	}

	// Stop series limiters.
	if sl := s.hourlySeriesLimiter; sl != nil {
		sl.MustStop()
	}
	if sl := s.dailySeriesLimiter; sl != nil {
		sl.MustStop()
	}
}

func (s *Storage) mustLoadNextDayMetricIDs(date uint64) *byDateMetricIDEntry {
	e := &byDateMetricIDEntry{
		date: date,
	}
	name := "next_day_metric_ids"
	path := s.cachePath + "/" + name
	logger.Infof("loading %s from %q...", name, path)
	startTime := time.Now()
	if !fs.IsPathExist(path) {
		logger.Infof("nothing to load from %q", path)
		return e
	}
	src, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
	srcOrigLen := len(src)
	if len(src) < 16 {
		logger.Errorf("discarding %s, since it has broken header; got %d bytes; want %d bytes", path, len(src), 16)
		return e
	}

	// Unmarshal header
	dateLoaded := encoding.UnmarshalUint64(src)
	src = src[8:]
	if dateLoaded != date {
		logger.Infof("discarding %s, since it contains data for stale date; got %d; want %d", path, dateLoaded, date)
		return e
	}

	// Unmarshal uint64set
	m, tail, err := unmarshalUint64Set(src)
	if err != nil {
		logger.Infof("discarding %s because cannot load uint64set: %s", path, err)
		return e
	}
	if len(tail) > 0 {
		logger.Infof("discarding %s because non-empty tail left; len(tail)=%d", path, len(tail))
		return e
	}
	e.v = *m
	logger.Infof("loaded %s from %q in %.3f seconds; entriesCount: %d; sizeBytes: %d", name, path, time.Since(startTime).Seconds(), m.Len(), srcOrigLen)
	return e
}

func (s *Storage) mustLoadHourMetricIDs(hour uint64, name string) *hourMetricIDs {
	hm := &hourMetricIDs{
		hour: hour,
	}
	path := s.cachePath + "/" + name
	logger.Infof("loading %s from %q...", name, path)
	startTime := time.Now()
	if !fs.IsPathExist(path) {
		logger.Infof("nothing to load from %q", path)
		return hm
	}
	src, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
	srcOrigLen := len(src)
	if len(src) < 24 {
		logger.Errorf("discarding %s, since it has broken header; got %d bytes; want %d bytes", path, len(src), 24)
		return hm
	}

	// Unmarshal header
	isFull := encoding.UnmarshalUint64(src)
	src = src[8:]
	hourLoaded := encoding.UnmarshalUint64(src)
	src = src[8:]
	if hourLoaded != hour {
		logger.Infof("discarding %s, since it contains outdated hour; got %d; want %d", path, hourLoaded, hour)
		return hm
	}

	// Unmarshal uint64set
	m, tail, err := unmarshalUint64Set(src)
	if err != nil {
		logger.Infof("discarding %s because cannot load uint64set: %s", path, err)
		return hm
	}
	if len(tail) > 0 {
		logger.Infof("discarding %s because non-empty tail left; len(tail)=%d", path, len(tail))
		return hm
	}
	hm.m = m
	hm.isFull = isFull != 0
	logger.Infof("loaded %s from %q in %.3f seconds; entriesCount: %d; sizeBytes: %d", name, path, time.Since(startTime).Seconds(), m.Len(), srcOrigLen)
	return hm
}

func (s *Storage) mustSaveNextDayMetricIDs(e *byDateMetricIDEntry) {
	name := "next_day_metric_ids"
	path := s.cachePath + "/" + name
	logger.Infof("saving %s to %q...", name, path)
	startTime := time.Now()
	dst := make([]byte, 0, e.v.Len()*8+16)

	// Marshal header
	dst = encoding.MarshalUint64(dst, e.date)

	// Marshal e.v
	dst = marshalUint64Set(dst, &e.v)

	if err := ioutil.WriteFile(path, dst, 0644); err != nil {
		logger.Panicf("FATAL: cannot write %d bytes to %q: %s", len(dst), path, err)
	}
	logger.Infof("saved %s to %q in %.3f seconds; entriesCount: %d; sizeBytes: %d", name, path, time.Since(startTime).Seconds(), e.v.Len(), len(dst))
}

func (s *Storage) mustSaveHourMetricIDs(hm *hourMetricIDs, name string) {
	path := s.cachePath + "/" + name
	logger.Infof("saving %s to %q...", name, path)
	startTime := time.Now()
	dst := make([]byte, 0, hm.m.Len()*8+24)
	isFull := uint64(0)
	if hm.isFull {
		isFull = 1
	}

	// Marshal header
	dst = encoding.MarshalUint64(dst, isFull)
	dst = encoding.MarshalUint64(dst, hm.hour)

	// Marshal hm.m
	dst = marshalUint64Set(dst, hm.m)

	if err := ioutil.WriteFile(path, dst, 0644); err != nil {
		logger.Panicf("FATAL: cannot write %d bytes to %q: %s", len(dst), path, err)
	}
	logger.Infof("saved %s to %q in %.3f seconds; entriesCount: %d; sizeBytes: %d", name, path, time.Since(startTime).Seconds(), hm.m.Len(), len(dst))
}

func unmarshalUint64Set(src []byte) (*uint64set.Set, []byte, error) {
	mLen := encoding.UnmarshalUint64(src)
	src = src[8:]
	if uint64(len(src)) < 8*mLen {
		return nil, nil, fmt.Errorf("cannot unmarshal uint64set; got %d bytes; want at least %d bytes", len(src), 8*mLen)
	}
	m := &uint64set.Set{}
	for i := uint64(0); i < mLen; i++ {
		metricID := encoding.UnmarshalUint64(src)
		src = src[8:]
		m.Add(metricID)
	}
	return m, src, nil
}

func marshalUint64Set(dst []byte, m *uint64set.Set) []byte {
	dst = encoding.MarshalUint64(dst, uint64(m.Len()))
	m.ForEach(func(part []uint64) bool {
		for _, metricID := range part {
			dst = encoding.MarshalUint64(dst, metricID)
		}
		return true
	})
	return dst
}

func mustGetMinTimestampForCompositeIndex(metadataDir string, isEmptyDB bool) int64 {
	path := metadataDir + "/minTimestampForCompositeIndex"
	minTimestamp, err := loadMinTimestampForCompositeIndex(path)
	if err == nil {
		return minTimestamp
	}
	if !os.IsNotExist(err) {
		logger.Errorf("cannot read minTimestampForCompositeIndex, so trying to re-create it; error: %s", err)
	}
	date := time.Now().UnixNano() / 1e6 / msecPerDay
	if !isEmptyDB {
		// The current and the next day can already contain non-composite indexes,
		// so they cannot be queried with composite indexes.
		date += 2
	} else {
		date = 0
	}
	minTimestamp = date * msecPerDay
	dateBuf := encoding.MarshalInt64(nil, minTimestamp)
	if err := os.RemoveAll(path); err != nil {
		logger.Fatalf("cannot remove a file with minTimestampForCompositeIndex: %s", err)
	}
	if err := fs.WriteFileAtomically(path, dateBuf); err != nil {
		logger.Fatalf("cannot store minTimestampForCompositeIndex: %s", err)
	}
	return minTimestamp
}

func loadMinTimestampForCompositeIndex(path string) (int64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("unexpected length of %q; got %d bytes; want 8 bytes", path, len(data))
	}
	return encoding.UnmarshalInt64(data), nil
}

func (s *Storage) mustLoadCache(info, name string, sizeBytes int) *workingsetcache.Cache {
	path := s.cachePath + "/" + name
	logger.Infof("loading %s cache from %q...", info, path)
	startTime := time.Now()
	c := workingsetcache.Load(path, sizeBytes)
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	logger.Infof("loaded %s cache from %q in %.3f seconds; entriesCount: %d; sizeBytes: %d",
		info, path, time.Since(startTime).Seconds(), cs.EntriesCount, cs.BytesSize)
	return c
}

func (s *Storage) mustSaveCache(c *workingsetcache.Cache, info, name string) {
	saveCacheLock.Lock()
	defer saveCacheLock.Unlock()

	path := s.cachePath + "/" + name
	logger.Infof("saving %s cache to %q...", info, path)
	startTime := time.Now()
	if err := c.Save(path); err != nil {
		logger.Panicf("FATAL: cannot save %s cache to %q: %s", info, path, err)
	}
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	logger.Infof("saved %s cache to %q in %.3f seconds; entriesCount: %d; sizeBytes: %d",
		info, path, time.Since(startTime).Seconds(), cs.EntriesCount, cs.BytesSize)
}

// saveCacheLock prevents from data races when multiple concurrent goroutines save the same cache.
var saveCacheLock sync.Mutex

// SetRetentionTimezoneOffset sets the offset, which is used for calculating the time for indexdb rotation.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574
func SetRetentionTimezoneOffset(offset time.Duration) {
	retentionTimezoneOffsetMsecs = offset.Milliseconds()
}

var retentionTimezoneOffsetMsecs int64

func nextRetentionDuration(retentionMsecs int64) time.Duration {
	// Round retentionMsecs to days. This guarantees that per-day inverted index works as expected.
	retentionMsecs = ((retentionMsecs + msecPerDay - 1) / msecPerDay) * msecPerDay
	t := time.Now().UnixNano() / 1e6
	deadline := ((t + retentionMsecs - 1) / retentionMsecs) * retentionMsecs
	// Schedule the deadline to +4 hours from the next retention period start.
	// This should prevent from possible double deletion of indexdb
	// due to time drift - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/248 .
	deadline += 4 * 3600 * 1000
	// The effect of time zone on retention period is moved out.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574
	deadline -= retentionTimezoneOffsetMsecs
	return time.Duration(deadline-t) * time.Millisecond
}

// SearchMetricNames returns marshaled metric names matching the given tfss on the given tr.
//
// The marshaled metric names must be unmarshaled via MetricName.UnmarshalString().
func (s *Storage) SearchMetricNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for matching metric names: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()
	tsids, err := s.searchTSIDs(qt, tfss, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(tsids) == 0 {
		return nil, nil
	}
	if err = s.prefetchMetricNames(qt, tsids, deadline); err != nil {
		return nil, err
	}
	idb := s.idb()
	metricNames := make([]string, 0, len(tsids))
	metricNamesSeen := make(map[string]struct{}, len(tsids))
	var metricName []byte
	for i := range tsids {
		if i&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(deadline); err != nil {
				return nil, err
			}
		}
		metricID := tsids[i].MetricID
		var err error
		metricName, err = idb.searchMetricNameWithCache(metricName[:0], metricID)
		if err != nil {
			if err == io.EOF {
				// Skip missing metricName for metricID.
				// It should be automatically fixed. See indexDB.searchMetricName for details.
				continue
			}
			return nil, fmt.Errorf("error when searching metricName for metricID=%d: %w", metricID, err)
		}
		if _, ok := metricNamesSeen[string(metricName)]; ok {
			// The given metric name was already seen; skip it
			continue
		}
		metricNames = append(metricNames, string(metricName))
		metricNamesSeen[metricNames[len(metricNames)-1]] = struct{}{}
	}
	qt.Printf("loaded %d metric names", len(metricNames))
	return metricNames, nil
}

// searchTSIDs returns sorted TSIDs for the given tfss and the given tr.
func (s *Storage) searchTSIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]TSID, error) {
	qt = qt.NewChild("search for matching tsids: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()
	// Do not cache tfss -> tsids here, since the caching is performed
	// on idb level.

	// Limit the number of concurrent goroutines that may search TSIDS in the storage.
	// This should prevent from out of memory errors and CPU thrashing when too many
	// goroutines call searchTSIDs.
	select {
	case searchTSIDsConcurrencyCh <- struct{}{}:
	default:
		// Sleep for a while until giving up
		atomic.AddUint64(&s.searchTSIDsConcurrencyLimitReached, 1)
		currentTime := fasttime.UnixTimestamp()
		timeoutSecs := uint64(0)
		if currentTime < deadline {
			timeoutSecs = deadline - currentTime
		}
		timeout := time.Second * time.Duration(timeoutSecs)
		t := timerpool.Get(timeout)
		select {
		case searchTSIDsConcurrencyCh <- struct{}{}:
			qt.Printf("wait in the queue because %d concurrent search requests are already performed", cap(searchTSIDsConcurrencyCh))
			timerpool.Put(t)
		case <-t.C:
			timerpool.Put(t)
			atomic.AddUint64(&s.searchTSIDsConcurrencyLimitTimeout, 1)
			return nil, fmt.Errorf("cannot search for tsids, since more than %d concurrent searches are performed during %.3f secs; add more CPUs or reduce query load",
				cap(searchTSIDsConcurrencyCh), timeout.Seconds())
		}
	}
	tsids, err := s.idb().searchTSIDs(qt, tfss, tr, maxMetrics, deadline)
	<-searchTSIDsConcurrencyCh
	if err != nil {
		return nil, fmt.Errorf("error when searching tsids: %w", err)
	}
	return tsids, nil
}

var (
	// Limit the concurrency for TSID searches to GOMAXPROCS*2, since this operation
	// is CPU bound and sometimes disk IO bound, so there is no sense in running more
	// than GOMAXPROCS*2 concurrent goroutines for TSID searches.
	searchTSIDsConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs()*2)
)

// prefetchMetricNames pre-fetches metric names for the given tsids into metricID->metricName cache.
//
// This should speed-up further searchMetricNameWithCache calls for metricIDs from tsids.
func (s *Storage) prefetchMetricNames(qt *querytracer.Tracer, tsids []TSID, deadline uint64) error {
	qt = qt.NewChild("prefetch metric names for %d tsids", len(tsids))
	defer qt.Done()
	if len(tsids) == 0 {
		qt.Printf("nothing to prefetch")
		return nil
	}
	var metricIDs uint64Sorter
	prefetchedMetricIDs := s.prefetchedMetricIDs.Load().(*uint64set.Set)
	for i := range tsids {
		tsid := &tsids[i]
		metricID := tsid.MetricID
		if prefetchedMetricIDs.Has(metricID) {
			continue
		}
		metricIDs = append(metricIDs, metricID)
	}
	qt.Printf("%d out of %d metric names must be pre-fetched", len(metricIDs), len(tsids))
	if len(metricIDs) < 500 {
		// It is cheaper to skip pre-fetching and obtain metricNames inline.
		qt.Printf("skip pre-fetching metric names for low number of metrid ids=%d", len(metricIDs))
		return nil
	}
	atomic.AddUint64(&s.slowMetricNameLoads, uint64(len(metricIDs)))

	// Pre-fetch metricIDs.
	sort.Sort(metricIDs)
	var missingMetricIDs []uint64
	var metricName []byte
	var err error
	idb := s.idb()
	is := idb.getIndexSearch(deadline)
	defer idb.putIndexSearch(is)
	for loops, metricID := range metricIDs {
		if loops&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		metricName, err = is.searchMetricNameWithCache(metricName[:0], metricID)
		if err != nil {
			if err == io.EOF {
				missingMetricIDs = append(missingMetricIDs, metricID)
				continue
			}
			return fmt.Errorf("error in pre-fetching metricName for metricID=%d: %w", metricID, err)
		}
	}
	idb.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch(deadline)
		defer extDB.putIndexSearch(is)
		for loops, metricID := range missingMetricIDs {
			if loops&paceLimiterSlowIterationsMask == 0 {
				if err = checkSearchDeadlineAndPace(is.deadline); err != nil {
					return
				}
			}
			metricName, err = is.searchMetricNameWithCache(metricName[:0], metricID)
			if err != nil && err != io.EOF {
				err = fmt.Errorf("error in pre-fetching metricName for metricID=%d in extDB: %w", metricID, err)
				return
			}
		}
	})
	if err != nil {
		return err
	}
	qt.Printf("pre-fetch metric names for %d metric ids", len(metricIDs))

	// Store the pre-fetched metricIDs, so they aren't pre-fetched next time.
	s.prefetchedMetricIDsLock.Lock()
	var prefetchedMetricIDsNew *uint64set.Set
	if fasttime.UnixTimestamp() < atomic.LoadUint64(&s.prefetchedMetricIDsDeadline) {
		// Periodically reset the prefetchedMetricIDs in order to limit its size.
		prefetchedMetricIDsNew = &uint64set.Set{}
		atomic.StoreUint64(&s.prefetchedMetricIDsDeadline, fasttime.UnixTimestamp()+73*60)
	} else {
		prefetchedMetricIDsNew = prefetchedMetricIDs.Clone()
	}
	prefetchedMetricIDsNew.AddMulti(metricIDs)
	if prefetchedMetricIDsNew.SizeBytes() > uint64(memory.Allowed())/32 {
		// Reset prefetchedMetricIDsNew if it occupies too much space.
		prefetchedMetricIDsNew = &uint64set.Set{}
	}
	s.prefetchedMetricIDs.Store(prefetchedMetricIDsNew)
	s.prefetchedMetricIDsLock.Unlock()
	qt.Printf("cache metric ids for pre-fetched metric names")
	return nil
}

// ErrDeadlineExceeded is returned when the request times out.
var ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")

// DeleteSeries deletes all the series matching the given tfss.
//
// Returns the number of metrics deleted.
func (s *Storage) DeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters) (int, error) {
	deletedCount, err := s.idb().DeleteTSIDs(qt, tfss)
	if err != nil {
		return deletedCount, fmt.Errorf("cannot delete tsids: %w", err)
	}
	// Do not reset MetricName->TSID cache in order to prevent from adding new data points
	// to deleted time series in Storage.add, since it is already reset inside DeleteTSIDs.

	// Do not reset MetricID->MetricName cache, since it must be used only
	// after filtering out deleted metricIDs.

	return deletedCount, nil
}

// SearchLabelNamesWithFiltersOnTimeRange searches for label names matching the given tfss on tr.
func (s *Storage) SearchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int, deadline uint64) ([]string, error) {
	return s.idb().SearchLabelNamesWithFiltersOnTimeRange(qt, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

// SearchLabelValuesWithFiltersOnTimeRange searches for label values for the given labelName, filters and tr.
func (s *Storage) SearchLabelValuesWithFiltersOnTimeRange(qt *querytracer.Tracer, labelName string, tfss []*TagFilters,
	tr TimeRange, maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	return s.idb().SearchLabelValuesWithFiltersOnTimeRange(qt, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given tagKey and tagValuePrefix on the given tr.
//
// This allows implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or similar APIs.
//
// If more than maxTagValueSuffixes suffixes is found, then only the first maxTagValueSuffixes suffixes is returned.
func (s *Storage) SearchTagValueSuffixes(qt *querytracer.Tracer, tr TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, maxTagValueSuffixes int, deadline uint64) ([]string, error) {
	return s.idb().SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes, deadline)
}

// SearchGraphitePaths returns all the matching paths for the given graphite query on the given tr.
func (s *Storage) SearchGraphitePaths(qt *querytracer.Tracer, tr TimeRange, query []byte, maxPaths int, deadline uint64) ([]string, error) {
	query = replaceAlternateRegexpsWithGraphiteWildcards(query)
	return s.searchGraphitePaths(qt, tr, nil, query, maxPaths, deadline)
}

// replaceAlternateRegexpsWithGraphiteWildcards replaces (foo|..|bar) with {foo,...,bar} in b and returns the new value.
func replaceAlternateRegexpsWithGraphiteWildcards(b []byte) []byte {
	var dst []byte
	for {
		n := bytes.IndexByte(b, '(')
		if n < 0 {
			if len(dst) == 0 {
				// Fast path - b doesn't contain the openining brace.
				return b
			}
			dst = append(dst, b...)
			return dst
		}
		dst = append(dst, b[:n]...)
		b = b[n+1:]
		n = bytes.IndexByte(b, ')')
		if n < 0 {
			dst = append(dst, '(')
			dst = append(dst, b...)
			return dst
		}
		x := b[:n]
		b = b[n+1:]
		if string(x) == ".*" {
			dst = append(dst, '*')
			continue
		}
		dst = append(dst, '{')
		for len(x) > 0 {
			n = bytes.IndexByte(x, '|')
			if n < 0 {
				dst = append(dst, x...)
				break
			}
			dst = append(dst, x[:n]...)
			x = x[n+1:]
			dst = append(dst, ',')
		}
		dst = append(dst, '}')
	}
}

func (s *Storage) searchGraphitePaths(qt *querytracer.Tracer, tr TimeRange, qHead, qTail []byte, maxPaths int, deadline uint64) ([]string, error) {
	n := bytes.IndexAny(qTail, "*[{")
	if n < 0 {
		// Verify that qHead matches a metric name.
		qHead = append(qHead, qTail...)
		suffixes, err := s.SearchTagValueSuffixes(qt, tr, "", bytesutil.ToUnsafeString(qHead), '.', 1, deadline)
		if err != nil {
			return nil, err
		}
		if len(suffixes) == 0 {
			// The query doesn't match anything.
			return nil, nil
		}
		if len(suffixes[0]) > 0 {
			// The query matches a metric name with additional suffix.
			return nil, nil
		}
		return []string{string(qHead)}, nil
	}
	qHead = append(qHead, qTail[:n]...)
	suffixes, err := s.SearchTagValueSuffixes(qt, tr, "", bytesutil.ToUnsafeString(qHead), '.', maxPaths, deadline)
	if err != nil {
		return nil, err
	}
	if len(suffixes) == 0 {
		return nil, nil
	}
	if len(suffixes) >= maxPaths {
		return nil, fmt.Errorf("more than maxPaths=%d suffixes found", maxPaths)
	}
	qNode := qTail[n:]
	qTail = nil
	mustMatchLeafs := true
	if m := bytes.IndexByte(qNode, '.'); m >= 0 {
		qTail = qNode[m+1:]
		qNode = qNode[:m+1]
		mustMatchLeafs = false
	}
	re, err := getRegexpForGraphiteQuery(string(qNode))
	if err != nil {
		return nil, err
	}
	qHeadLen := len(qHead)
	var paths []string
	for _, suffix := range suffixes {
		if len(paths) > maxPaths {
			return nil, fmt.Errorf("more than maxPath=%d paths found", maxPaths)
		}
		if !re.MatchString(suffix) {
			continue
		}
		if mustMatchLeafs {
			qHead = append(qHead[:qHeadLen], suffix...)
			paths = append(paths, string(qHead))
			continue
		}
		qHead = append(qHead[:qHeadLen], suffix...)
		ps, err := s.searchGraphitePaths(qt, tr, qHead, qTail, maxPaths, deadline)
		if err != nil {
			return nil, err
		}
		paths = append(paths, ps...)
	}
	return paths, nil
}

func getRegexpForGraphiteQuery(q string) (*regexp.Regexp, error) {
	parts, tail := getRegexpPartsForGraphiteQuery(q)
	if len(tail) > 0 {
		return nil, fmt.Errorf("unexpected tail left after parsing %q: %q", q, tail)
	}
	reStr := "^" + strings.Join(parts, "") + "$"
	return regexp.Compile(reStr)
}

func getRegexpPartsForGraphiteQuery(q string) ([]string, string) {
	var parts []string
	for {
		n := strings.IndexAny(q, "*{}[,")
		if n < 0 {
			parts = append(parts, regexp.QuoteMeta(q))
			return parts, ""
		}
		parts = append(parts, regexp.QuoteMeta(q[:n]))
		q = q[n:]
		switch q[0] {
		case ',', '}':
			return parts, q
		case '*':
			parts = append(parts, "[^.]*")
			q = q[1:]
		case '{':
			var tmp []string
			for {
				a, tail := getRegexpPartsForGraphiteQuery(q[1:])
				tmp = append(tmp, strings.Join(a, ""))
				if len(tail) == 0 {
					parts = append(parts, regexp.QuoteMeta("{"))
					parts = append(parts, strings.Join(tmp, ","))
					return parts, ""
				}
				if tail[0] == ',' {
					q = tail
					continue
				}
				if tail[0] == '}' {
					if len(tmp) == 1 {
						parts = append(parts, tmp[0])
					} else {
						parts = append(parts, "(?:"+strings.Join(tmp, "|")+")")
					}
					q = tail[1:]
					break
				}
				logger.Panicf("BUG: unexpected first char at tail %q; want `.` or `}`", tail)
			}
		case '[':
			n := strings.IndexByte(q, ']')
			if n < 0 {
				parts = append(parts, regexp.QuoteMeta(q))
				return parts, ""
			}
			parts = append(parts, q[:n+1])
			q = q[n+1:]
		}
	}
}

// GetSeriesCount returns the approximate number of unique time series.
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (s *Storage) GetSeriesCount(deadline uint64) (uint64, error) {
	return s.idb().GetSeriesCount(deadline)
}

// GetTSDBStatus returns TSDB status data for /api/v1/status/tsdb
func (s *Storage) GetTSDBStatus(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	return s.idb().GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

// MetricRow is a metric to insert into storage.
type MetricRow struct {
	// MetricNameRaw contains raw metric name, which must be decoded
	// with MetricName.UnmarshalRaw.
	MetricNameRaw []byte

	Timestamp int64
	Value     float64
}

// CopyFrom copies src to mr.
func (mr *MetricRow) CopyFrom(src *MetricRow) {
	mr.MetricNameRaw = append(mr.MetricNameRaw[:0], src.MetricNameRaw...)
	mr.Timestamp = src.Timestamp
	mr.Value = src.Value
}

// String returns string representation of the mr.
func (mr *MetricRow) String() string {
	metricName := string(mr.MetricNameRaw)
	var mn MetricName
	if err := mn.UnmarshalRaw(mr.MetricNameRaw); err == nil {
		metricName = mn.String()
	}
	return fmt.Sprintf("%s (Timestamp=%d, Value=%f)", metricName, mr.Timestamp, mr.Value)
}

// Marshal appends marshaled mr to dst and returns the result.
func (mr *MetricRow) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, mr.MetricNameRaw)
	dst = encoding.MarshalUint64(dst, uint64(mr.Timestamp))
	dst = encoding.MarshalUint64(dst, math.Float64bits(mr.Value))
	return dst
}

// UnmarshalX unmarshals mr from src and returns the remaining tail from src.
//
// mr refers to src, so it remains valid until src changes.
func (mr *MetricRow) UnmarshalX(src []byte) ([]byte, error) {
	tail, metricNameRaw, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricName: %w", err)
	}
	mr.MetricNameRaw = metricNameRaw

	if len(tail) < 8 {
		return tail, fmt.Errorf("cannot unmarshal Timestamp: want %d bytes; have %d bytes", 8, len(tail))
	}
	timestamp := encoding.UnmarshalUint64(tail)
	mr.Timestamp = int64(timestamp)
	tail = tail[8:]

	if len(tail) < 8 {
		return tail, fmt.Errorf("cannot unmarshal Value: want %d bytes; have %d bytes", 8, len(tail))
	}
	value := encoding.UnmarshalUint64(tail)
	mr.Value = math.Float64frombits(value)
	tail = tail[8:]

	return tail, nil
}

// ForceMergePartitions force-merges partitions in s with names starting from the given partitionNamePrefix.
//
// Partitions are merged sequentially in order to reduce load on the system.
func (s *Storage) ForceMergePartitions(partitionNamePrefix string) error {
	return s.tb.ForceMergePartitions(partitionNamePrefix)
}

var rowsAddedTotal uint64

// AddRows adds the given mrs to s.
func (s *Storage) AddRows(mrs []MetricRow, precisionBits uint8) error {
	if len(mrs) == 0 {
		return nil
	}

	// Limit the number of concurrent goroutines that may add rows to the storage.
	// This should prevent from out of memory errors and CPU thrashing when too many
	// goroutines call AddRows.
	select {
	case addRowsConcurrencyCh <- struct{}{}:
	default:
		// Sleep for a while until giving up
		atomic.AddUint64(&s.addRowsConcurrencyLimitReached, 1)
		t := timerpool.Get(addRowsTimeout)

		// Prioritize data ingestion over concurrent searches.
		storagepacelimiter.Search.Inc()

		select {
		case addRowsConcurrencyCh <- struct{}{}:
			timerpool.Put(t)
			storagepacelimiter.Search.Dec()
		case <-t.C:
			timerpool.Put(t)
			storagepacelimiter.Search.Dec()
			atomic.AddUint64(&s.addRowsConcurrencyLimitTimeout, 1)
			atomic.AddUint64(&s.addRowsConcurrencyDroppedRows, uint64(len(mrs)))
			return fmt.Errorf("cannot add %d rows to storage in %s, since it is overloaded with %d concurrent writers; add more CPUs or reduce load",
				len(mrs), addRowsTimeout, cap(addRowsConcurrencyCh))
		}
	}

	// Add rows to the storage in blocks with limited size in order to reduce memory usage.
	var firstErr error
	ic := getMetricRowsInsertCtx()
	maxBlockLen := len(ic.rrs)
	for len(mrs) > 0 {
		mrsBlock := mrs
		if len(mrs) > maxBlockLen {
			mrsBlock = mrs[:maxBlockLen]
			mrs = mrs[maxBlockLen:]
		} else {
			mrs = nil
		}
		if err := s.add(ic.rrs, ic.tmpMrs, mrsBlock, precisionBits); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		atomic.AddUint64(&rowsAddedTotal, uint64(len(mrsBlock)))
	}
	putMetricRowsInsertCtx(ic)

	<-addRowsConcurrencyCh

	return firstErr
}

type metricRowsInsertCtx struct {
	rrs    []rawRow
	tmpMrs []*MetricRow
}

func getMetricRowsInsertCtx() *metricRowsInsertCtx {
	v := metricRowsInsertCtxPool.Get()
	if v == nil {
		v = &metricRowsInsertCtx{
			rrs:    make([]rawRow, maxMetricRowsPerBlock),
			tmpMrs: make([]*MetricRow, maxMetricRowsPerBlock),
		}
	}
	return v.(*metricRowsInsertCtx)
}

func putMetricRowsInsertCtx(ic *metricRowsInsertCtx) {
	tmpMrs := ic.tmpMrs
	for i := range tmpMrs {
		tmpMrs[i] = nil
	}
	metricRowsInsertCtxPool.Put(ic)
}

var metricRowsInsertCtxPool sync.Pool

const maxMetricRowsPerBlock = 8000

var (
	// Limit the concurrency for data ingestion to GOMAXPROCS, since this operation
	// is CPU bound, so there is no sense in running more than GOMAXPROCS concurrent
	// goroutines on data ingestion path.
	addRowsConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())
	addRowsTimeout       = 30 * time.Second
)

// RegisterMetricNames registers all the metric names from mns in the indexdb, so they can be queried later.
//
// The the MetricRow.Timestamp is used for registering the metric name starting from the given timestamp.
// Th MetricRow.Value field is ignored.
func (s *Storage) RegisterMetricNames(qt *querytracer.Tracer, mrs []MetricRow) error {
	qt = qt.NewChild("registering %d series", len(mrs))
	defer qt.Done()
	var metricName []byte
	var genTSID generationTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	idb := s.idb()
	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)
	for i := range mrs {
		mr := &mrs[i]
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			if err := s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw); err != nil {
				continue
			}
			if genTSID.generation == idb.generation {
				// Fast path - mr.MetricNameRaw has been already registered in the current idb.
				continue
			}
		}
		// Slow path - register mr.MetricNameRaw.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			return fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
		}
		mn.sortTags()
		metricName = mn.Marshal(metricName[:0])
		date := uint64(mr.Timestamp) / msecPerDay
		if err := is.GetOrCreateTSIDByName(&genTSID.TSID, metricName, mr.MetricNameRaw, date); err != nil {
			if errors.Is(err, errSeriesCardinalityExceeded) {
				continue
			}
			return fmt.Errorf("cannot create TSID for metricName %q: %w", metricName, err)
		}
		genTSID.generation = idb.generation
		s.putTSIDToCache(&genTSID, mr.MetricNameRaw)
	}
	return nil
}

func (s *Storage) add(rows []rawRow, dstMrs []*MetricRow, mrs []MetricRow, precisionBits uint8) error {
	idb := s.idb()
	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)
	var (
		// These vars are used for speeding up bulk imports of multiple adjacent rows for the same metricName.
		prevTSID          TSID
		prevMetricNameRaw []byte
	)
	var pmrs *pendingMetricRows
	minTimestamp, maxTimestamp := s.tb.getMinMaxTimestamps()

	var genTSID generationTSID

	// Return only the first error, since it has no sense in returning all errors.
	var firstWarn error
	j := 0
	for i := range mrs {
		mr := &mrs[i]
		if math.IsNaN(mr.Value) {
			if !decimal.IsStaleNaN(mr.Value) {
				// Skip NaNs other than Prometheus staleness marker, since the underlying encoding
				// doesn't know how to work with them.
				continue
			}
		}
		if mr.Timestamp < minTimestamp {
			// Skip rows with too small timestamps outside the retention.
			if firstWarn == nil {
				metricName := getUserReadableMetricName(mr.MetricNameRaw)
				firstWarn = fmt.Errorf("cannot insert row with too small timestamp %d outside the retention; minimum allowed timestamp is %d; "+
					"probably you need updating -retentionPeriod command-line flag; metricName: %s",
					mr.Timestamp, minTimestamp, metricName)
			}
			atomic.AddUint64(&s.tooSmallTimestampRows, 1)
			continue
		}
		if mr.Timestamp > maxTimestamp {
			// Skip rows with too big timestamps significantly exceeding the current time.
			if firstWarn == nil {
				metricName := getUserReadableMetricName(mr.MetricNameRaw)
				firstWarn = fmt.Errorf("cannot insert row with too big timestamp %d exceeding the current time; maximum allowed timestamp is %d; metricName: %s",
					mr.Timestamp, maxTimestamp, metricName)
			}
			atomic.AddUint64(&s.tooBigTimestampRows, 1)
			continue
		}
		dstMrs[j] = mr
		r := &rows[j]
		j++
		r.Timestamp = mr.Timestamp
		r.Value = mr.Value
		r.PrecisionBits = precisionBits
		if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
			// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
			// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.
			r.TSID = prevTSID
			continue
		}
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			if err := s.registerSeriesCardinality(r.TSID.MetricID, mr.MetricNameRaw); err != nil {
				j--
				continue
			}
			r.TSID = genTSID.TSID
			// Fast path - the TSID for the given MetricNameRaw has been found in cache and isn't deleted.
			// There is no need in checking whether r.TSID.MetricID is deleted, since tsidCache doesn't
			// contain MetricName->TSID entries for deleted time series.
			// See Storage.DeleteSeries code for details.
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			if genTSID.generation != idb.generation {
				// The found entry is from the previous cache generation,
				// so attempt to re-populate the current generation with this entry.
				// This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401
				date := uint64(r.Timestamp) / msecPerDay
				created, err := is.maybeCreateIndexes(&genTSID.TSID, mr.MetricNameRaw, date)
				if err != nil {
					return fmt.Errorf("cannot create indexes: %w", err)
				}
				if created {
					genTSID.generation = idb.generation
					s.putTSIDToCache(&genTSID, mr.MetricNameRaw)
				}
			}
			continue
		}

		// Slow path - the TSID is missing in the cache.
		// Postpone its search in the loop below.
		j--
		if pmrs == nil {
			pmrs = getPendingMetricRows()
		}
		if err := pmrs.addRow(mr); err != nil {
			// Do not stop adding rows on error - just skip invalid row.
			// This guarantees that invalid rows don't prevent
			// from adding valid rows into the storage.
			if firstWarn == nil {
				firstWarn = err
			}
			continue
		}
	}
	if pmrs != nil {
		// Sort pendingMetricRows by canonical metric name in order to speed up search via `is` in the loop below.
		pendingMetricRows := pmrs.pmrs
		sort.Slice(pendingMetricRows, func(i, j int) bool {
			return string(pendingMetricRows[i].MetricName) < string(pendingMetricRows[j].MetricName)
		})
		prevMetricNameRaw = nil
		var slowInsertsCount uint64
		for i := range pendingMetricRows {
			pmr := &pendingMetricRows[i]
			mr := pmr.mr
			dstMrs[j] = mr
			r := &rows[j]
			j++
			r.Timestamp = mr.Timestamp
			r.Value = mr.Value
			r.PrecisionBits = precisionBits
			if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
				// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
				// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.
				r.TSID = prevTSID
				continue
			}
			slowInsertsCount++
			date := uint64(r.Timestamp) / msecPerDay
			if err := is.GetOrCreateTSIDByName(&r.TSID, pmr.MetricName, mr.MetricNameRaw, date); err != nil {
				j--
				if errors.Is(err, errSeriesCardinalityExceeded) {
					continue
				}
				// Do not stop adding rows on error - just skip invalid row.
				// This guarantees that invalid rows don't prevent
				// from adding valid rows into the storage.
				if firstWarn == nil {
					firstWarn = fmt.Errorf("cannot obtain or create TSID for MetricName %q: %w", pmr.MetricName, err)
				}
				continue
			}
			genTSID.generation = idb.generation
			genTSID.TSID = r.TSID
			s.putTSIDToCache(&genTSID, mr.MetricNameRaw)

			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw
		}
		putPendingMetricRows(pmrs)
		atomic.AddUint64(&s.slowRowInserts, slowInsertsCount)
	}
	if firstWarn != nil {
		storageAddRowsLogger.Warnf("warn occurred during rows addition: %s", firstWarn)
	}
	dstMrs = dstMrs[:j]
	rows = rows[:j]

	err := s.updatePerDateData(rows, dstMrs)
	if err != nil {
		err = fmt.Errorf("cannot update per-date data: %w", err)
	} else {
		err = s.tb.AddRows(rows)
		if err != nil {
			err = fmt.Errorf("cannot add rows to table: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("error occurred during rows addition: %w", err)
	}
	return nil
}

var storageAddRowsLogger = logger.WithThrottler("storageAddRows", 5*time.Second)

func (s *Storage) registerSeriesCardinality(metricID uint64, metricNameRaw []byte) error {
	if sl := s.hourlySeriesLimiter; sl != nil && !sl.Add(metricID) {
		atomic.AddUint64(&s.hourlySeriesLimitRowsDropped, 1)
		logSkippedSeries(metricNameRaw, "-storage.maxHourlySeries", sl.MaxItems())
		return errSeriesCardinalityExceeded
	}
	if sl := s.dailySeriesLimiter; sl != nil && !sl.Add(metricID) {
		atomic.AddUint64(&s.dailySeriesLimitRowsDropped, 1)
		logSkippedSeries(metricNameRaw, "-storage.maxDailySeries", sl.MaxItems())
		return errSeriesCardinalityExceeded
	}
	return nil
}

var errSeriesCardinalityExceeded = fmt.Errorf("cannot create series because series cardinality limit exceeded")

func logSkippedSeries(metricNameRaw []byte, flagName string, flagValue int) {
	select {
	case <-logSkippedSeriesTicker.C:
		// Do not use logger.WithThrottler() here, since this will result in increased CPU load
		// because of getUserReadableMetricName() calls per each logSkippedSeries call.
		userReadableMetricName := getUserReadableMetricName(metricNameRaw)
		logger.Warnf("skip series %s because %s=%d reached", userReadableMetricName, flagName, flagValue)
	default:
	}
}

var logSkippedSeriesTicker = time.NewTicker(5 * time.Second)

func getUserReadableMetricName(metricNameRaw []byte) string {
	mn := GetMetricName()
	defer PutMetricName(mn)
	if err := mn.UnmarshalRaw(metricNameRaw); err != nil {
		return fmt.Sprintf("cannot unmarshal metricNameRaw %q: %s", metricNameRaw, err)
	}
	return mn.String()
}

type pendingMetricRow struct {
	MetricName []byte
	mr         *MetricRow
}

type pendingMetricRows struct {
	pmrs           []pendingMetricRow
	metricNamesBuf []byte

	lastMetricNameRaw []byte
	lastMetricName    []byte
	mn                MetricName
}

func (pmrs *pendingMetricRows) reset() {
	for _, pmr := range pmrs.pmrs {
		pmr.MetricName = nil
		pmr.mr = nil
	}
	pmrs.pmrs = pmrs.pmrs[:0]
	pmrs.metricNamesBuf = pmrs.metricNamesBuf[:0]
	pmrs.lastMetricNameRaw = nil
	pmrs.lastMetricName = nil
	pmrs.mn.Reset()
}

func (pmrs *pendingMetricRows) addRow(mr *MetricRow) error {
	// Do not spend CPU time on re-calculating canonical metricName during bulk import
	// of many rows for the same metric.
	if string(mr.MetricNameRaw) != string(pmrs.lastMetricNameRaw) {
		if err := pmrs.mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			return fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
		}
		pmrs.mn.sortTags()
		metricNamesBufLen := len(pmrs.metricNamesBuf)
		pmrs.metricNamesBuf = pmrs.mn.Marshal(pmrs.metricNamesBuf)
		pmrs.lastMetricName = pmrs.metricNamesBuf[metricNamesBufLen:]
		pmrs.lastMetricNameRaw = mr.MetricNameRaw
	}
	pmrs.pmrs = append(pmrs.pmrs, pendingMetricRow{
		MetricName: pmrs.lastMetricName,
		mr:         mr,
	})
	return nil
}

func getPendingMetricRows() *pendingMetricRows {
	v := pendingMetricRowsPool.Get()
	if v == nil {
		v = &pendingMetricRows{}
	}
	return v.(*pendingMetricRows)
}

func putPendingMetricRows(pmrs *pendingMetricRows) {
	pmrs.reset()
	pendingMetricRowsPool.Put(pmrs)
}

var pendingMetricRowsPool sync.Pool

func (s *Storage) updatePerDateData(rows []rawRow, mrs []*MetricRow) error {
	var date uint64
	var hour uint64
	var prevTimestamp int64
	var (
		// These vars are used for speeding up bulk imports when multiple adjacent rows
		// contain the same (metricID, date) pairs.
		prevDate     uint64
		prevMetricID uint64
	)
	hm := s.currHourMetricIDs.Load().(*hourMetricIDs)
	hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
	hmPrevDate := hmPrev.hour / 24
	nextDayMetricIDs := &s.nextDayMetricIDs.Load().(*byDateMetricIDEntry).v
	ts := fasttime.UnixTimestamp()
	// Start pre-populating the next per-day inverted index during the last hour of the current day.
	// pMin linearly increases from 0 to 1 during the last hour of the day.
	pMin := (float64(ts%(3600*24)) / 3600) - 23
	type pendingDateMetricID struct {
		date     uint64
		metricID uint64
		mr       *MetricRow
	}
	var pendingDateMetricIDs []pendingDateMetricID
	var pendingNextDayMetricIDs []uint64
	var pendingHourEntries []uint64
	for i := range rows {
		r := &rows[i]
		if r.Timestamp != prevTimestamp {
			date = uint64(r.Timestamp) / msecPerDay
			hour = uint64(r.Timestamp) / msecPerHour
			prevTimestamp = r.Timestamp
		}
		metricID := r.TSID.MetricID
		if metricID == prevMetricID && date == prevDate {
			// Fast path for bulk import of multiple rows with the same (date, metricID) pairs.
			continue
		}
		prevDate = date
		prevMetricID = metricID
		if hour == hm.hour {
			// The r belongs to the current hour. Check for the current hour cache.
			if hm.m.Has(metricID) {
				// Fast path: the metricID is in the current hour cache.
				// This means the metricID has been already added to per-day inverted index.

				// Gradually pre-populate per-day inverted index for the next day during the last hour of the current day.
				// This should reduce CPU usage spike and slowdown at the beginning of the next day
				// when entries for all the active time series must be added to the index.
				// This should address https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 .
				if pMin > 0 {
					p := float64(uint32(fastHashUint64(metricID))) / (1 << 32)
					if p < pMin && !nextDayMetricIDs.Has(metricID) {
						pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
							date:     date + 1,
							metricID: metricID,
							mr:       mrs[i],
						})
						pendingNextDayMetricIDs = append(pendingNextDayMetricIDs, metricID)
					}
				}
				continue
			}
			pendingHourEntries = append(pendingHourEntries, metricID)
			if date == hmPrevDate && hmPrev.m.Has(metricID) {
				// The metricID is already registered for the current day on the previous hour.
				continue
			}
		}

		// Slower path: check global cache for (date, metricID) entry.
		if s.dateMetricIDCache.Has(date, metricID) {
			continue
		}
		// Slow path: store the (date, metricID) entry in the indexDB.
		pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
			date:     date,
			metricID: metricID,
			mr:       mrs[i],
		})
	}
	if len(pendingNextDayMetricIDs) > 0 {
		s.pendingNextDayMetricIDsLock.Lock()
		s.pendingNextDayMetricIDs.AddMulti(pendingNextDayMetricIDs)
		s.pendingNextDayMetricIDsLock.Unlock()
	}
	if len(pendingHourEntries) > 0 {
		s.pendingHourEntriesLock.Lock()
		s.pendingHourEntries.AddMulti(pendingHourEntries)
		s.pendingHourEntriesLock.Unlock()
	}
	if len(pendingDateMetricIDs) == 0 {
		// Fast path - there are no new (date, metricID) entires in rows.
		return nil
	}

	// Slow path - add new (date, metricID) entries to indexDB.

	atomic.AddUint64(&s.slowPerDayIndexInserts, uint64(len(pendingDateMetricIDs)))
	// Sort pendingDateMetricIDs by (date, metricID) in order to speed up `is` search in the loop below.
	sort.Slice(pendingDateMetricIDs, func(i, j int) bool {
		a := pendingDateMetricIDs[i]
		b := pendingDateMetricIDs[j]
		if a.date != b.date {
			return a.date < b.date
		}
		return a.metricID < b.metricID
	})
	idb := s.idb()
	is := idb.getIndexSearch(noDeadline)
	defer idb.putIndexSearch(is)
	var firstError error
	dateMetricIDsForCache := make([]dateMetricID, 0, len(pendingDateMetricIDs))
	mn := GetMetricName()
	for _, dmid := range pendingDateMetricIDs {
		date := dmid.date
		metricID := dmid.metricID
		ok, err := is.hasDateMetricID(date, metricID)
		if err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("error when locating (date=%s, metricID=%d) in database: %w", dateToString(date), metricID, err)
			}
			continue
		}
		if !ok {
			// The (date, metricID) entry is missing in the indexDB. Add it there together with per-day indexes.
			// It is OK if the (date, metricID) entry is added multiple times to db
			// by concurrent goroutines.
			if err := mn.UnmarshalRaw(dmid.mr.MetricNameRaw); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", dmid.mr.MetricNameRaw, err)
				}
				continue
			}
			mn.sortTags()
			if err := is.createPerDayIndexes(date, metricID, mn); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("error when storing per-date inverted index for (date=%s, metricID=%d): %w", dateToString(date), metricID, err)
				}
				continue
			}
		}
		dateMetricIDsForCache = append(dateMetricIDsForCache, dateMetricID{
			date:     date,
			metricID: metricID,
		})
	}
	PutMetricName(mn)
	// The (date, metricID) entries must be added to cache only after they have been successfully added to indexDB.
	s.dateMetricIDCache.Store(dateMetricIDsForCache)
	return firstError
}

func fastHashUint64(x uint64) uint64 {
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	return x * 2685821657736338717
}

// dateMetricIDCache is fast cache for holding (date, metricID) entries.
//
// It should be faster than map[date]*uint64set.Set on multicore systems.
type dateMetricIDCache struct {
	// 64-bit counters must be at the top of the structure to be properly aligned on 32-bit arches.
	syncsCount  uint64
	resetsCount uint64

	// Contains immutable map
	byDate atomic.Value

	// Contains mutable map protected by mu
	byDateMutable    *byDateMetricIDMap
	nextSyncDeadline uint64
	mu               sync.Mutex
}

func newDateMetricIDCache() *dateMetricIDCache {
	var dmc dateMetricIDCache
	dmc.resetLocked()
	return &dmc
}

func (dmc *dateMetricIDCache) Reset() {
	dmc.mu.Lock()
	dmc.resetLocked()
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) resetLocked() {
	// Do not reset syncsCount and resetsCount
	dmc.byDate.Store(newByDateMetricIDMap())
	dmc.byDateMutable = newByDateMetricIDMap()
	dmc.nextSyncDeadline = 10 + fasttime.UnixTimestamp()

	atomic.AddUint64(&dmc.resetsCount, 1)
}

func (dmc *dateMetricIDCache) EntriesCount() int {
	byDate := dmc.byDate.Load().(*byDateMetricIDMap)
	n := 0
	for _, e := range byDate.m {
		n += e.v.Len()
	}
	return n
}

func (dmc *dateMetricIDCache) SizeBytes() uint64 {
	byDate := dmc.byDate.Load().(*byDateMetricIDMap)
	n := uint64(0)
	for _, e := range byDate.m {
		n += e.v.SizeBytes()
	}
	return n
}

func (dmc *dateMetricIDCache) Has(date, metricID uint64) bool {
	byDate := dmc.byDate.Load().(*byDateMetricIDMap)
	v := byDate.get(date)
	if v.Has(metricID) {
		// Fast path.
		// The majority of calls must go here.
		return true
	}

	// Slow path. Check mutable map.
	dmc.mu.Lock()
	v = dmc.byDateMutable.get(date)
	ok := v.Has(metricID)
	dmc.syncLockedIfNeeded()
	dmc.mu.Unlock()

	return ok
}

type dateMetricID struct {
	date     uint64
	metricID uint64
}

func (dmc *dateMetricIDCache) Store(dmids []dateMetricID) {
	var prevDate uint64
	metricIDs := make([]uint64, 0, len(dmids))
	dmc.mu.Lock()
	for _, dmid := range dmids {
		if prevDate == dmid.date {
			metricIDs = append(metricIDs, dmid.metricID)
			continue
		}
		if len(metricIDs) > 0 {
			v := dmc.byDateMutable.getOrCreate(prevDate)
			v.AddMulti(metricIDs)
		}
		metricIDs = append(metricIDs[:0], dmid.metricID)
		prevDate = dmid.date
	}
	if len(metricIDs) > 0 {
		v := dmc.byDateMutable.getOrCreate(prevDate)
		v.AddMulti(metricIDs)
	}
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) Set(date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(date)
	v.Add(metricID)
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) syncLockedIfNeeded() {
	currentTime := fasttime.UnixTimestamp()
	if currentTime >= dmc.nextSyncDeadline {
		dmc.nextSyncDeadline = currentTime + 10
		dmc.syncLocked()
	}
}

func (dmc *dateMetricIDCache) syncLocked() {
	if len(dmc.byDateMutable.m) == 0 {
		// Nothing to sync.
		return
	}
	byDate := dmc.byDate.Load().(*byDateMetricIDMap)
	byDateMutable := dmc.byDateMutable
	for date, e := range byDateMutable.m {
		v := byDate.get(date)
		if v == nil {
			continue
		}
		v = v.Clone()
		v.Union(&e.v)
		byDateMutable.m[date] = &byDateMetricIDEntry{
			date: date,
			v:    *v,
		}
	}
	for date, e := range byDate.m {
		v := byDateMutable.get(date)
		if v != nil {
			continue
		}
		byDateMutable.m[date] = e
	}
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()

	atomic.AddUint64(&dmc.syncsCount, 1)

	if dmc.SizeBytes() > uint64(memory.Allowed())/256 {
		dmc.resetLocked()
	}
}

type byDateMetricIDMap struct {
	hotEntry atomic.Value
	m        map[uint64]*byDateMetricIDEntry
}

func newByDateMetricIDMap() *byDateMetricIDMap {
	dmm := &byDateMetricIDMap{
		m: make(map[uint64]*byDateMetricIDEntry),
	}
	dmm.hotEntry.Store(&byDateMetricIDEntry{})
	return dmm
}

func (dmm *byDateMetricIDMap) get(date uint64) *uint64set.Set {
	hotEntry := dmm.hotEntry.Load().(*byDateMetricIDEntry)
	if hotEntry.date == date {
		// Fast path
		return &hotEntry.v
	}
	// Slow path
	e := dmm.m[date]
	if e == nil {
		return nil
	}
	dmm.hotEntry.Store(e)
	return &e.v
}

func (dmm *byDateMetricIDMap) getOrCreate(date uint64) *uint64set.Set {
	v := dmm.get(date)
	if v != nil {
		return v
	}
	e := &byDateMetricIDEntry{
		date: date,
	}
	dmm.m[date] = e
	return &e.v
}

type byDateMetricIDEntry struct {
	date uint64
	v    uint64set.Set
}

func (s *Storage) updateNextDayMetricIDs() {
	date := fasttime.UnixDate()
	e := s.nextDayMetricIDs.Load().(*byDateMetricIDEntry)
	s.pendingNextDayMetricIDsLock.Lock()
	pendingMetricIDs := s.pendingNextDayMetricIDs
	s.pendingNextDayMetricIDs = &uint64set.Set{}
	s.pendingNextDayMetricIDsLock.Unlock()
	if pendingMetricIDs.Len() == 0 && e.date == date {
		// Fast path: nothing to update.
		return
	}

	// Slow path: union pendingMetricIDs with e.v
	if e.date == date {
		pendingMetricIDs.Union(&e.v)
	}
	eNew := &byDateMetricIDEntry{
		date: date,
		v:    *pendingMetricIDs,
	}
	s.nextDayMetricIDs.Store(eNew)
}

func (s *Storage) updateCurrHourMetricIDs() {
	hm := s.currHourMetricIDs.Load().(*hourMetricIDs)
	s.pendingHourEntriesLock.Lock()
	newMetricIDs := s.pendingHourEntries
	s.pendingHourEntries = &uint64set.Set{}
	s.pendingHourEntriesLock.Unlock()

	hour := fasttime.UnixHour()
	if newMetricIDs.Len() == 0 && hm.hour == hour {
		// Fast path: nothing to update.
		return
	}

	// Slow path: hm.m must be updated with non-empty s.pendingHourEntries.
	var m *uint64set.Set
	isFull := hm.isFull
	if hm.hour == hour {
		m = hm.m.Clone()
	} else {
		m = &uint64set.Set{}
		isFull = true
	}
	m.Union(newMetricIDs)
	hmNew := &hourMetricIDs{
		m:      m,
		hour:   hour,
		isFull: isFull,
	}
	s.currHourMetricIDs.Store(hmNew)
	if hm.hour != hour {
		s.prevHourMetricIDs.Store(hm)
	}
}

type hourMetricIDs struct {
	m      *uint64set.Set
	hour   uint64
	isFull bool
}

type generationTSID struct {
	TSID TSID

	// generation stores the indexdb.generation value to identify to which indexdb belongs this TSID
	generation uint64
}

func (s *Storage) getTSIDFromCache(dst *generationTSID, metricName []byte) bool {
	buf := (*[unsafe.Sizeof(*dst)]byte)(unsafe.Pointer(dst))[:]
	buf = s.tsidCache.Get(buf[:0], metricName)
	return uintptr(len(buf)) == unsafe.Sizeof(*dst)
}

func (s *Storage) putTSIDToCache(tsid *generationTSID, metricName []byte) {
	buf := (*[unsafe.Sizeof(*tsid)]byte)(unsafe.Pointer(tsid))[:]
	s.tsidCache.Set(metricName, buf)
}

func (s *Storage) openIndexDBTables(path string) (curr, prev *indexDB, err error) {
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, nil, fmt.Errorf("cannot create directory %q: %w", path, err)
	}

	d, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open directory: %w", err)
	}
	defer fs.MustClose(d)

	// Search for the two most recent tables - the last one is active,
	// the previous one contains backup data.
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read directory: %w", err)
	}
	var tableNames []string
	for _, fi := range fis {
		if !fs.IsDirOrSymlink(fi) {
			// Skip non-directories.
			continue
		}
		tableName := fi.Name()
		if !indexDBTableNameRegexp.MatchString(tableName) {
			// Skip invalid directories.
			continue
		}
		tableNames = append(tableNames, tableName)
	}
	sort.Slice(tableNames, func(i, j int) bool {
		return tableNames[i] < tableNames[j]
	})
	if len(tableNames) < 2 {
		// Create missing tables
		if len(tableNames) == 0 {
			prevName := nextIndexDBTableName()
			tableNames = append(tableNames, prevName)
		}
		currName := nextIndexDBTableName()
		tableNames = append(tableNames, currName)
	}

	// Invariant: len(tableNames) >= 2

	// Remove all the tables except two last tables.
	for _, tn := range tableNames[:len(tableNames)-2] {
		pathToRemove := path + "/" + tn
		logger.Infof("removing obsolete indexdb dir %q...", pathToRemove)
		fs.MustRemoveAll(pathToRemove)
		logger.Infof("removed obsolete indexdb dir %q", pathToRemove)
	}

	// Persist changes on the file system.
	fs.MustSyncPath(path)

	// Open the last two tables.
	currPath := path + "/" + tableNames[len(tableNames)-1]

	curr, err = openIndexDB(currPath, s, 0, &s.isReadOnly)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open curr indexdb table at %q: %w", currPath, err)
	}
	prevPath := path + "/" + tableNames[len(tableNames)-2]
	prev, err = openIndexDB(prevPath, s, 0, &s.isReadOnly)
	if err != nil {
		curr.MustClose()
		return nil, nil, fmt.Errorf("cannot open prev indexdb table at %q: %w", prevPath, err)
	}

	return curr, prev, nil
}

var indexDBTableNameRegexp = regexp.MustCompile("^[0-9A-F]{16}$")

func nextIndexDBTableName() string {
	n := atomic.AddUint64(&indexDBTableIdx, 1)
	return fmt.Sprintf("%016X", n)
}

var indexDBTableIdx = uint64(time.Now().UnixNano())
