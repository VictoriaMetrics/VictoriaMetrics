package storage

import (
	"bytes"
	"fmt"
	"io"
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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/VictoriaMetrics/metricsql"
)

const (
	retention31Days = 31 * 24 * time.Hour
	retentionMax    = 100 * 12 * retention31Days
)

// Storage represents TSDB storage.
type Storage struct {
	// Atomic counters must go at the top of the structure in order to properly align by 8 bytes on 32-bit archs.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212 .
	tooSmallTimestampRows uint64
	tooBigTimestampRows   uint64

	timeseriesRepopulated  uint64
	timeseriesPreCreated   uint64
	newTimeseriesCreated   uint64
	slowRowInserts         uint64
	slowPerDayIndexInserts uint64
	slowMetricNameLoads    uint64

	hourlySeriesLimitRowsDropped uint64
	dailySeriesLimitRowsDropped  uint64

	// nextRotationTimestamp is a timestamp in seconds of the next indexdb rotation.
	//
	// It is used for gradual pre-population of the idbNext during the last hour before the indexdb rotation.
	// in order to reduce spikes in CPU and disk IO usage just after the rotiation.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401
	nextRotationTimestamp int64

	path           string
	cachePath      string
	retentionMsecs int64

	// lock file for exclusive access to the storage on the given path.
	flockF *os.File

	// idbCurr contains the currently used indexdb.
	idbCurr atomic.Pointer[indexDB]

	// idbNext is the next indexdb, which will become idbCurr at the next rotation.
	//
	// It is started to be gradually pre-populated with the data for active time series during the last hour
	// before nextRotationTimestamp.
	// This reduces spikes in CPU and disk IO usage just after the rotiation.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401
	idbNext atomic.Pointer[indexDB]

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

	// dateMetricIDCache is (generation, Date, MetricID) cache, where generation is the indexdb generation.
	// See generationTSID for details.
	dateMetricIDCache *dateMetricIDCache

	// Fast cache for MetricID values occurred during the current hour.
	currHourMetricIDs atomic.Pointer[hourMetricIDs]

	// Fast cache for MetricID values occurred during the previous hour.
	prevHourMetricIDs atomic.Pointer[hourMetricIDs]

	// Fast cache for pre-populating per-day inverted index for the next day.
	// This is needed in order to remove CPU usage spikes at 00:00 UTC
	// due to creation of per-day inverted index for active time series.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 for details.
	nextDayMetricIDs atomic.Pointer[byDateMetricIDEntry]

	// Pending MetricID values to be added to currHourMetricIDs.
	pendingHourEntriesLock sync.Mutex
	pendingHourEntries     []pendingHourMetricIDEntry

	// Pending MetricIDs to be added to nextDayMetricIDs.
	pendingNextDayMetricIDsLock sync.Mutex
	pendingNextDayMetricIDs     *uint64set.Set

	// prefetchedMetricIDs contains metricIDs for pre-fetched metricNames in the prefetchMetricNames function.
	prefetchedMetricIDs atomic.Pointer[uint64set.Set]

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
	deletedMetricIDs           atomic.Pointer[uint64set.Set]
	deletedMetricIDsUpdateLock sync.Mutex

	isReadOnly uint32
}

type pendingHourMetricIDEntry struct {
	AccountID uint32
	ProjectID uint32
	MetricID  uint64
}

type accountProjectKey struct {
	AccountID uint32
	ProjectID uint32
}

// MustOpenStorage opens storage on the given path with the given retentionMsecs.
func MustOpenStorage(path string, retention time.Duration, maxHourlySeries, maxDailySeries int) *Storage {
	path, err := filepath.Abs(path)
	if err != nil {
		logger.Panicf("FATAL: cannot determine absolute path for %q: %s", path, err)
	}
	if retention <= 0 || retention > retentionMax {
		retention = retentionMax
	}
	s := &Storage{
		path:           path,
		cachePath:      filepath.Join(path, cacheDirname),
		retentionMsecs: retention.Milliseconds(),
		stop:           make(chan struct{}),
	}
	fs.MustMkdirIfNotExist(path)

	// Check whether the cache directory must be removed
	// It is removed if it contains resetCacheOnStartupFilename.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1447 for details.
	if fs.IsPathExist(filepath.Join(s.cachePath, resetCacheOnStartupFilename)) {
		logger.Infof("removing cache directory at %q, since it contains `%s` file...", s.cachePath, resetCacheOnStartupFilename)
		// Do not use fs.MustRemoveAll() here, since the cache directory may be mounted
		// to a separate filesystem. In this case the fs.MustRemoveAll() will fail while
		// trying to remove the mount root.
		fs.RemoveDirContents(s.cachePath)
		logger.Infof("cache directory at %q has been successfully removed", s.cachePath)
	}

	// Protect from concurrent opens.
	s.flockF = fs.MustCreateFlockFile(path)

	// Check whether restore process finished successfully
	restoreLockF := filepath.Join(path, backupnames.RestoreInProgressFilename)
	if fs.IsPathExist(restoreLockF) {
		logger.Panicf("FATAL: incomplete vmrestore run; run vmrestore again or remove lock file %q", restoreLockF)
	}

	// Pre-create snapshots directory if it is missing.
	snapshotsPath := filepath.Join(path, snapshotsDirname)
	fs.MustMkdirIfNotExist(snapshotsPath)
	fs.MustRemoveTemporaryDirs(snapshotsPath)

	// Initialize series cardinality limiter.
	if maxHourlySeries > 0 {
		s.hourlySeriesLimiter = bloomfilter.NewLimiter(maxHourlySeries, time.Hour)
	}
	if maxDailySeries > 0 {
		s.dailySeriesLimiter = bloomfilter.NewLimiter(maxDailySeries, 24*time.Hour)
	}

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache("metricName_tsid", getTSIDCacheSize())
	s.metricIDCache = s.mustLoadCache("metricID_tsid", mem/16)
	s.metricNameCache = s.mustLoadCache("metricID_metricName", mem/10)
	s.dateMetricIDCache = newDateMetricIDCache()

	hour := fasttime.UnixHour()
	hmCurr := s.mustLoadHourMetricIDs(hour, "curr_hour_metric_ids")
	hmPrev := s.mustLoadHourMetricIDs(hour-1, "prev_hour_metric_ids")
	s.currHourMetricIDs.Store(hmCurr)
	s.prevHourMetricIDs.Store(hmPrev)

	s.pendingNextDayMetricIDs = &uint64set.Set{}

	s.prefetchedMetricIDs.Store(&uint64set.Set{})

	// Load metadata
	metadataDir := filepath.Join(path, metadataDirname)
	isEmptyDB := !fs.IsPathExist(filepath.Join(path, indexdbDirname))
	fs.MustMkdirIfNotExist(metadataDir)
	s.minTimestampForCompositeIndex = mustGetMinTimestampForCompositeIndex(metadataDir, isEmptyDB)

	// Load indexdb
	idbPath := filepath.Join(path, indexdbDirname)
	idbSnapshotsPath := filepath.Join(idbPath, snapshotsDirname)
	fs.MustMkdirIfNotExist(idbSnapshotsPath)
	fs.MustRemoveTemporaryDirs(idbSnapshotsPath)
	idbNext, idbCurr, idbPrev := s.mustOpenIndexDBTables(idbPath)

	idbCurr.SetExtDB(idbPrev)
	idbNext.SetExtDB(idbCurr)

	s.idbCurr.Store(idbCurr)
	s.idbNext.Store(idbNext)

	// Initialize nextRotationTimestamp
	nowSecs := int64(fasttime.UnixTimestamp())
	retentionSecs := retention.Milliseconds() / 1000 // not .Seconds() because unnecessary float64 conversion
	nextRotationTimestamp := nextRetentionDeadlineSeconds(nowSecs, retentionSecs, retentionTimezoneOffsetSecs)
	atomic.StoreInt64(&s.nextRotationTimestamp, nextRotationTimestamp)

	// Load nextDayMetricIDs cache
	date := fasttime.UnixDate()
	nextDayMetricIDs := s.mustLoadNextDayMetricIDs(idbCurr.generation, date)
	s.nextDayMetricIDs.Store(nextDayMetricIDs)

	// Load deleted metricIDs from idbCurr and idbPrev
	dmisCurr, err := idbCurr.loadDeletedMetricIDs()
	if err != nil {
		logger.Panicf("FATAL: cannot load deleted metricIDs for the current indexDB at %q: %s", path, err)
	}
	dmisPrev, err := idbPrev.loadDeletedMetricIDs()
	if err != nil {
		logger.Panicf("FATAL: cannot load deleted metricIDs for the previous indexDB at %q: %s", path, err)
	}
	s.setDeletedMetricIDs(dmisCurr)
	s.updateDeletedMetricIDs(dmisPrev)

	// check for free disk space before opening the table
	// to prevent unexpected part merges. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4023
	s.startFreeDiskSpaceWatcher()

	// Load data
	tablePath := filepath.Join(path, dataDirname)
	tb := mustOpenTable(tablePath, s)
	s.tb = tb

	s.startCurrHourMetricIDsUpdater()
	s.startNextDayMetricIDsUpdater()
	s.startRetentionWatcher()

	return s
}

// RetentionMsecs returns retentionMsecs for s.
func (s *Storage) RetentionMsecs() int64 {
	return s.retentionMsecs
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
	return s.deletedMetricIDs.Load()
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

// DebugFlush makes sure all the recently added data is visible to search.
//
// Note: this function doesn't store all the in-memory data to disk - it just converts
// recently added items to searchable parts, which can be stored either in memory
// (if they are quite small) or to persistent disk.
//
// This function is for debugging and testing purposes only,
// since it may slow down data ingestion when used frequently.
func (s *Storage) DebugFlush() {
	s.tb.flushPendingRows()
	idb := s.idb()
	idb.tb.DebugFlush()
	idb.doExtDB(func(extDB *indexDB) {
		extDB.tb.DebugFlush()
	})
}

// CreateSnapshot creates snapshot for s and returns the snapshot name.
func (s *Storage) CreateSnapshot(deadline uint64) (string, error) {
	logger.Infof("creating Storage snapshot for %q...", s.path)
	startTime := time.Now()

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	var dirsToRemoveOnError []string
	defer func() {
		for _, dir := range dirsToRemoveOnError {
			fs.MustRemoveAll(dir)
		}
	}()

	snapshotName := snapshot.NewName()
	srcDir := s.path
	dstDir := filepath.Join(srcDir, snapshotsDirname, snapshotName)
	fs.MustMkdirFailIfExist(dstDir)
	dirsToRemoveOnError = append(dirsToRemoveOnError, dstDir)

	smallDir, bigDir, err := s.tb.CreateSnapshot(snapshotName, deadline)
	if err != nil {
		return "", fmt.Errorf("cannot create table snapshot: %w", err)
	}
	dirsToRemoveOnError = append(dirsToRemoveOnError, smallDir, bigDir)

	dstDataDir := filepath.Join(dstDir, dataDirname)
	fs.MustMkdirFailIfExist(dstDataDir)

	dstSmallDir := filepath.Join(dstDataDir, smallDirname)
	fs.MustSymlinkRelative(smallDir, dstSmallDir)

	dstBigDir := filepath.Join(dstDataDir, bigDirname)
	fs.MustSymlinkRelative(bigDir, dstBigDir)

	fs.MustSyncPath(dstDataDir)

	srcMetadataDir := filepath.Join(srcDir, metadataDirname)
	dstMetadataDir := filepath.Join(dstDir, metadataDirname)
	fs.MustCopyDirectory(srcMetadataDir, dstMetadataDir)

	idbSnapshot := filepath.Join(srcDir, indexdbDirname, snapshotsDirname, snapshotName)
	idb := s.idb()
	currSnapshot := filepath.Join(idbSnapshot, idb.name)
	if err := idb.tb.CreateSnapshotAt(currSnapshot, deadline); err != nil {
		return "", fmt.Errorf("cannot create curr indexDB snapshot: %w", err)
	}
	dirsToRemoveOnError = append(dirsToRemoveOnError, idbSnapshot)

	ok := idb.doExtDB(func(extDB *indexDB) {
		prevSnapshot := filepath.Join(idbSnapshot, extDB.name)
		err = extDB.tb.CreateSnapshotAt(prevSnapshot, deadline)
	})
	if ok && err != nil {
		return "", fmt.Errorf("cannot create prev indexDB snapshot: %w", err)
	}
	dstIdbDir := filepath.Join(dstDir, indexdbDirname)
	fs.MustSymlinkRelative(idbSnapshot, dstIdbDir)

	fs.MustSyncPath(dstDir)

	logger.Infof("created Storage snapshot for %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	dirsToRemoveOnError = nil
	return snapshotName, nil
}

// ListSnapshots returns sorted list of existing snapshots for s.
func (s *Storage) ListSnapshots() ([]string, error) {
	snapshotsPath := filepath.Join(s.path, snapshotsDirname)
	d, err := os.Open(snapshotsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open snapshots directory: %w", err)
	}
	defer fs.MustClose(d)

	fnames, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read snapshots directory at %q: %w", snapshotsPath, err)
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
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)

	logger.Infof("deleting snapshot %q...", snapshotPath)
	startTime := time.Now()

	s.tb.MustDeleteSnapshot(snapshotName)
	idbPath := filepath.Join(s.path, indexdbDirname, snapshotsDirname, snapshotName)
	fs.MustRemoveDirAtomic(idbPath)
	fs.MustRemoveDirAtomic(snapshotPath)

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
	return s.idbCurr.Load()
}

// Metrics contains essential metrics for the Storage.
type Metrics struct {
	RowsAddedTotal    uint64
	DedupsDuringMerge uint64

	TooSmallTimestampRows uint64
	TooBigTimestampRows   uint64

	TimeseriesRepopulated  uint64
	TimeseriesPreCreated   uint64
	NewTimeseriesCreated   uint64
	SlowRowInserts         uint64
	SlowPerDayIndexInserts uint64
	SlowMetricNameLoads    uint64

	HourlySeriesLimitRowsDropped   uint64
	HourlySeriesLimitMaxSeries     uint64
	HourlySeriesLimitCurrentSeries uint64

	DailySeriesLimitRowsDropped   uint64
	DailySeriesLimitMaxSeries     uint64
	DailySeriesLimitCurrentSeries uint64

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

	m.TimeseriesRepopulated += atomic.LoadUint64(&s.timeseriesRepopulated)
	m.TimeseriesPreCreated += atomic.LoadUint64(&s.timeseriesPreCreated)
	m.NewTimeseriesCreated += atomic.LoadUint64(&s.newTimeseriesCreated)
	m.SlowRowInserts += atomic.LoadUint64(&s.slowRowInserts)
	m.SlowPerDayIndexInserts += atomic.LoadUint64(&s.slowPerDayIndexInserts)
	m.SlowMetricNameLoads += atomic.LoadUint64(&s.slowMetricNameLoads)

	if sl := s.hourlySeriesLimiter; sl != nil {
		m.HourlySeriesLimitRowsDropped += atomic.LoadUint64(&s.hourlySeriesLimitRowsDropped)
		m.HourlySeriesLimitMaxSeries += uint64(sl.MaxItems())
		m.HourlySeriesLimitCurrentSeries += uint64(sl.CurrentItems())
	}

	if sl := s.dailySeriesLimiter; sl != nil {
		m.DailySeriesLimitRowsDropped += atomic.LoadUint64(&s.dailySeriesLimitRowsDropped)
		m.DailySeriesLimitMaxSeries += uint64(sl.MaxItems())
		m.DailySeriesLimitCurrentSeries += uint64(sl.CurrentItems())
	}

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

	hmCurr := s.currHourMetricIDs.Load()
	hmPrev := s.prevHourMetricIDs.Load()
	hourMetricIDsLen := hmPrev.m.Len()
	if hmCurr.m.Len() > hourMetricIDsLen {
		hourMetricIDsLen = hmCurr.m.Len()
	}
	m.HourMetricIDCacheSize += uint64(hourMetricIDsLen)
	m.HourMetricIDCacheSizeBytes += hmCurr.m.SizeBytes()
	m.HourMetricIDCacheSizeBytes += hmPrev.m.SizeBytes()

	nextDayMetricIDs := &s.nextDayMetricIDs.Load().v
	m.NextDayMetricIDCacheSize += uint64(nextDayMetricIDs.Len())
	m.NextDayMetricIDCacheSizeBytes += nextDayMetricIDs.SizeBytes()

	prefetchedMetricIDs := s.prefetchedMetricIDs.Load()
	m.PrefetchedMetricIDsSize += uint64(prefetchedMetricIDs.Len())
	m.PrefetchedMetricIDsSizeBytes += uint64(prefetchedMetricIDs.SizeBytes())

	d := s.nextRetentionSeconds()
	if d < 0 {
		d = 0
	}
	m.NextRetentionSeconds = uint64(d)

	s.idb().UpdateMetrics(&m.IndexDBMetrics)
	s.tb.UpdateMetrics(&m.TableMetrics)
}

func (s *Storage) nextRetentionSeconds() int64 {
	return atomic.LoadInt64(&s.nextRotationTimestamp) - int64(fasttime.UnixTimestamp())
}

// SetFreeDiskSpaceLimit sets the minimum free disk space size of current storage path
//
// The function must be called before opening or creating any storage.
func SetFreeDiskSpaceLimit(bytes int64) {
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
			if atomic.CompareAndSwapUint32(&s.isReadOnly, 0, 1) {
				// log notification only on state change
				logger.Warnf("switching the storage at %s to read-only mode, since it has less than -storage.minFreeDiskSpaceBytes=%d of free space: %d bytes left",
					s.path, freeDiskSpaceLimitBytes, freeSpaceBytes)
			}
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
		d := s.nextRetentionSeconds()
		select {
		case <-s.stop:
			return
		case currentTime := <-time.After(time.Second * time.Duration(d)):
			s.mustRotateIndexDB(currentTime)
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
			hour := fasttime.UnixHour()
			s.updateCurrHourMetricIDs(hour)
			return
		case <-ticker.C:
			hour := fasttime.UnixHour()
			s.updateCurrHourMetricIDs(hour)
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
			date := fasttime.UnixDate()
			s.updateNextDayMetricIDs(date)
			return
		case <-ticker.C:
			date := fasttime.UnixDate()
			s.updateNextDayMetricIDs(date)
		}
	}
}

func (s *Storage) mustRotateIndexDB(currentTime time.Time) {
	// Create new indexdb table, which will be used as idbNext
	newTableName := nextIndexDBTableName()
	idbNewPath := filepath.Join(s.path, indexdbDirname, newTableName)
	idbNew := mustOpenIndexDB(idbNewPath, s, &s.isReadOnly)

	// Update nextRotationTimestamp
	nextRotationTimestamp := currentTime.Unix() + s.retentionMsecs/1000
	atomic.StoreInt64(&s.nextRotationTimestamp, nextRotationTimestamp)

	// Set idbNext to idbNew
	idbNext := s.idbNext.Load()
	idbNew.SetExtDB(idbNext)
	s.idbNext.Store(idbNew)

	// Set idbCurr to idbNext
	idbCurr := s.idb()
	s.idbCurr.Store(idbNext)

	// Schedule data removal for idbPrev
	idbCurr.doExtDB(func(extDB *indexDB) {
		extDB.scheduleToDrop()
	})
	idbCurr.SetExtDB(nil)

	// Persist changes on the file system.
	fs.MustSyncPath(s.path)

	// Do not flush tsidCache to avoid read/write path slowdown.
	// The cache is automatically re-populated with new TSID entries
	// with the updated indexdb generation.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401

	// Flush metric id caches for the current and the previous hour,
	// since they may contain entries missing in idbCurr after the rotation.
	// This should prevent from missing data in queries when
	// the following steps are performed for short -retentionPeriod (e.g. 1 day):
	//
	// 1. Add samples for some series between 3-4 UTC. These series are registered in currHourMetricIDs.
	// 2. The indexdb rotation is performed at 4 UTC. currHourMetricIDs is moved to prevHourMetricIDs.
	// 3. Continue adding samples for series from step 1 during time range 4-5 UTC.
	//    These series are already registered in prevHourMetricIDs, so VM doesn't add per-day entries to the current indexdb.
	// 4. Stop adding new samples for these series just before 5 UTC.
	// 5. The next indexdb rotation is performed at 4 UTC next day.
	//    The information about the series added at step 3 disappears from indexdb, since the old indexdb from step 1 is deleted,
	//    while the current indexdb doesn't contain information about the series.
	//    So queries for the last 24 hours stop returning samples added at step 3.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2698
	s.pendingHourEntriesLock.Lock()
	s.pendingHourEntries = nil
	s.pendingHourEntriesLock.Unlock()
	s.currHourMetricIDs.Store(&hourMetricIDs{})
	s.prevHourMetricIDs.Store(&hourMetricIDs{})

	// Do not flush dateMetricIDCache, since it contains entries prefixed with idb generation.

	// There is no need in resetting nextDayMetricIDs, since it contains entries prefixed with idb generation.

	// Do not flush metricIDCache and metricNameCache, since all the metricIDs
	// from prev idb remain valid after the rotation.
}

func (s *Storage) resetAndSaveTSIDCache() {
	// Reset cache and then store the reset cache on disk in order to prevent
	// from inconsistent behaviour after possible unclean shutdown.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347
	s.tsidCache.Reset()
	s.mustSaveCache(s.tsidCache, "metricName_tsid")
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
	s.mustSaveCache(s.tsidCache, "metricName_tsid")
	s.tsidCache.Stop()
	s.mustSaveCache(s.metricIDCache, "metricID_tsid")
	s.metricIDCache.Stop()
	s.mustSaveCache(s.metricNameCache, "metricID_metricName")
	s.metricNameCache.Stop()

	hmCurr := s.currHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmCurr, "curr_hour_metric_ids")
	hmPrev := s.prevHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmPrev, "prev_hour_metric_ids")

	nextDayMetricIDs := s.nextDayMetricIDs.Load()
	s.mustSaveNextDayMetricIDs(nextDayMetricIDs)

	// Release lock file.
	fs.MustClose(s.flockF)
	s.flockF = nil

	// Stop series limiters.
	if sl := s.hourlySeriesLimiter; sl != nil {
		sl.MustStop()
	}
	if sl := s.dailySeriesLimiter; sl != nil {
		sl.MustStop()
	}
}

func (s *Storage) mustLoadNextDayMetricIDs(generation, date uint64) *byDateMetricIDEntry {
	e := &byDateMetricIDEntry{
		k: generationDateKey{
			generation: generation,
			date:       date,
		},
	}
	name := "next_day_metric_ids_v2"
	path := filepath.Join(s.cachePath, name)
	if !fs.IsPathExist(path) {
		return e
	}
	src, err := os.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
	if len(src) < 24 {
		logger.Errorf("discarding %s, since it has broken header; got %d bytes; want %d bytes", path, len(src), 24)
		return e
	}

	// Unmarshal header
	generationLoaded := encoding.UnmarshalUint64(src)
	src = src[8:]
	if generationLoaded != generation {
		logger.Infof("discarding %s, since it contains data for stale generation; got %d; want %d", path, generationLoaded, generation)
	}
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
	return e
}

func (s *Storage) mustLoadHourMetricIDs(hour uint64, name string) *hourMetricIDs {
	hm := &hourMetricIDs{
		hour: hour,
	}
	path := filepath.Join(s.cachePath, name)
	if !fs.IsPathExist(path) {
		return hm
	}
	src, err := os.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
	if len(src) < 16 {
		logger.Errorf("discarding %s, since it has broken header; got %d bytes; want %d bytes", path, len(src), 16)
		return hm
	}

	// Unmarshal header
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
	src = tail

	// Unmarshal hm.byTenant
	if len(src) < 8 {
		logger.Errorf("discarding %s, since it has broken hm.byTenant header; got %d bytes; want %d bytes", path, len(src), 8)
		return hm
	}
	byTenantLen := encoding.UnmarshalUint64(src)
	src = src[8:]
	byTenant := make(map[accountProjectKey]*uint64set.Set, byTenantLen)
	for i := uint64(0); i < byTenantLen; i++ {
		if len(src) < 16 {
			logger.Errorf("discarding %s, since it has broken accountID:projectID prefix; got %d bytes; want %d bytes", path, len(src), 16)
			return hm
		}
		accountID := encoding.UnmarshalUint32(src)
		src = src[4:]
		projectID := encoding.UnmarshalUint32(src)
		src = src[4:]
		mLen := encoding.UnmarshalUint64(src)
		src = src[8:]
		if uint64(len(src)) < 8*mLen {
			logger.Errorf("discarding %s, since it has broken accountID:projectID entry; got %d bytes; want %d bytes", path, len(src), 8*mLen)
			return hm
		}
		m := &uint64set.Set{}
		for j := uint64(0); j < mLen; j++ {
			metricID := encoding.UnmarshalUint64(src)
			src = src[8:]
			m.Add(metricID)
		}
		k := accountProjectKey{
			AccountID: accountID,
			ProjectID: projectID,
		}
		byTenant[k] = m
	}

	hm.m = m
	hm.byTenant = byTenant
	return hm
}

func (s *Storage) mustSaveNextDayMetricIDs(e *byDateMetricIDEntry) {
	name := "next_day_metric_ids_v2"
	path := filepath.Join(s.cachePath, name)
	dst := make([]byte, 0, e.v.Len()*8+16)

	// Marshal header
	dst = encoding.MarshalUint64(dst, e.k.generation)
	dst = encoding.MarshalUint64(dst, e.k.date)

	// Marshal e.v
	dst = marshalUint64Set(dst, &e.v)

	if err := os.WriteFile(path, dst, 0644); err != nil {
		logger.Panicf("FATAL: cannot write %d bytes to %q: %s", len(dst), path, err)
	}
}

func (s *Storage) mustSaveHourMetricIDs(hm *hourMetricIDs, name string) {
	path := filepath.Join(s.cachePath, name)
	dst := make([]byte, 0, hm.m.Len()*8+24)

	// Marshal header
	dst = encoding.MarshalUint64(dst, hm.hour)

	// Marshal hm.m
	dst = marshalUint64Set(dst, hm.m)

	// Marshal hm.byTenant
	var metricIDs []uint64
	dst = encoding.MarshalUint64(dst, uint64(len(hm.byTenant)))
	for k, e := range hm.byTenant {
		dst = encoding.MarshalUint32(dst, k.AccountID)
		dst = encoding.MarshalUint32(dst, k.ProjectID)
		dst = encoding.MarshalUint64(dst, uint64(e.Len()))
		metricIDs = e.AppendTo(metricIDs[:0])
		for _, metricID := range metricIDs {
			dst = encoding.MarshalUint64(dst, metricID)
		}
	}

	if err := os.WriteFile(path, dst, 0644); err != nil {
		logger.Panicf("FATAL: cannot write %d bytes to %q: %s", len(dst), path, err)
	}
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
	path := filepath.Join(metadataDir, "minTimestampForCompositeIndex")
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
	fs.MustWriteAtomic(path, dateBuf, true)
	return minTimestamp
}

func loadMinTimestampForCompositeIndex(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("unexpected length of %q; got %d bytes; want 8 bytes", path, len(data))
	}
	return encoding.UnmarshalInt64(data), nil
}

func (s *Storage) mustLoadCache(name string, sizeBytes int) *workingsetcache.Cache {
	path := filepath.Join(s.cachePath, name)
	return workingsetcache.Load(path, sizeBytes)
}

func (s *Storage) mustSaveCache(c *workingsetcache.Cache, name string) {
	saveCacheLock.Lock()
	defer saveCacheLock.Unlock()

	path := filepath.Join(s.cachePath, name)
	if err := c.Save(path); err != nil {
		logger.Panicf("FATAL: cannot save cache to %q: %s", path, err)
	}
}

// saveCacheLock prevents from data races when multiple concurrent goroutines save the same cache.
var saveCacheLock sync.Mutex

// SetRetentionTimezoneOffset sets the offset, which is used for calculating the time for indexdb rotation.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574
func SetRetentionTimezoneOffset(offset time.Duration) {
	retentionTimezoneOffsetSecs = int64(offset.Seconds())
}

var retentionTimezoneOffsetSecs int64

func nextRetentionDeadlineSeconds(atSecs, retentionSecs, offsetSecs int64) int64 {
	// Round retentionSecs to days. This guarantees that per-day inverted index works as expected
	const secsPerDay = 24 * 3600
	retentionSecs = ((retentionSecs + secsPerDay - 1) / secsPerDay) * secsPerDay

	// Schedule the deadline to +4 hours from the next retention period start
	// because of historical reasons - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/248
	offsetSecs -= 4 * 3600

	// Make sure that offsetSecs doesn't exceed retentionSecs
	offsetSecs %= retentionSecs

	// align the retention deadline to multiples of retentionSecs
	// This makes the deadline independent of atSecs.
	deadline := ((atSecs + offsetSecs + retentionSecs - 1) / retentionSecs) * retentionSecs

	// Apply the provided offsetSecs
	deadline -= offsetSecs

	return deadline
}

// SearchMetricNames returns marshaled metric names matching the given tfss on the given tr.
//
// The marshaled metric names must be unmarshaled via MetricName.UnmarshalString().
func (s *Storage) SearchMetricNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for matching metric names: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()
	metricIDs, err := s.idb().searchMetricIDs(qt, tfss, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if len(metricIDs) == 0 || len(tfss) == 0 {
		return nil, nil
	}
	accountID := tfss[0].accountID
	projectID := tfss[0].projectID
	if err = s.prefetchMetricNames(qt, accountID, projectID, metricIDs, deadline); err != nil {
		return nil, err
	}
	idb := s.idb()
	metricNames := make([]string, 0, len(metricIDs))
	metricNamesSeen := make(map[string]struct{}, len(metricIDs))
	var metricName []byte
	for i, metricID := range metricIDs {
		if i&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(deadline); err != nil {
				return nil, err
			}
		}
		var ok bool
		metricName, ok = idb.searchMetricNameWithCache(metricName[:0], metricID, accountID, projectID)
		if !ok {
			// Skip missing metricName for metricID.
			// It should be automatically fixed. See indexDB.searchMetricName for details.
			continue
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

// prefetchMetricNames pre-fetches metric names for the given metricIDs into metricID->metricName cache.
//
// It is expected that all the metricIDs belong to the same (accountID, projectID)
//
// This should speed-up further searchMetricNameWithCache calls for srcMetricIDs from tsids.
func (s *Storage) prefetchMetricNames(qt *querytracer.Tracer, accountID, projectID uint32, srcMetricIDs []uint64, deadline uint64) error {
	qt = qt.NewChild("prefetch metric names for %d metricIDs", len(srcMetricIDs))
	defer qt.Done()
	if len(srcMetricIDs) == 0 {
		qt.Printf("nothing to prefetch")
		return nil
	}
	var metricIDs uint64Sorter
	prefetchedMetricIDs := s.prefetchedMetricIDs.Load()
	for _, metricID := range srcMetricIDs {
		if prefetchedMetricIDs.Has(metricID) {
			continue
		}
		metricIDs = append(metricIDs, metricID)
	}
	qt.Printf("%d out of %d metric names must be pre-fetched", len(metricIDs), len(srcMetricIDs))
	if len(metricIDs) < 500 {
		// It is cheaper to skip pre-fetching and obtain metricNames inline.
		qt.Printf("skip pre-fetching metric names for low number of metric ids=%d", len(metricIDs))
		return nil
	}
	atomic.AddUint64(&s.slowMetricNameLoads, uint64(len(metricIDs)))

	// Pre-fetch metricIDs.
	sort.Sort(metricIDs)
	var missingMetricIDs []uint64
	var metricName []byte
	var err error
	idb := s.idb()
	is := idb.getIndexSearch(accountID, projectID, deadline)
	defer idb.putIndexSearch(is)
	for loops, metricID := range metricIDs {
		if loops&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		var ok bool
		metricName, ok = is.searchMetricNameWithCache(metricName[:0], metricID)
		if !ok {
			missingMetricIDs = append(missingMetricIDs, metricID)
			continue
		}
	}
	idb.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch(accountID, projectID, deadline)
		defer extDB.putIndexSearch(is)
		for loops, metricID := range missingMetricIDs {
			if loops&paceLimiterSlowIterationsMask == 0 {
				if err = checkSearchDeadlineAndPace(is.deadline); err != nil {
					return
				}
			}
			metricName, _ = is.searchMetricNameWithCache(metricName[:0], metricID)
		}
	})
	if err != nil && err != io.EOF {
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
	// Do not reset MetricName->TSID cache, since it is already reset inside DeleteTSIDs.

	// Do not reset MetricID->MetricName cache, since it must be used only
	// after filtering out deleted metricIDs.

	return deletedCount, nil
}

// SearchLabelNamesWithFiltersOnTimeRange searches for label names matching the given tfss on tr.
func (s *Storage) SearchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*TagFilters, tr TimeRange,
	maxLabelNames, maxMetrics int, deadline uint64,
) ([]string, error) {
	return s.idb().SearchLabelNamesWithFiltersOnTimeRange(qt, accountID, projectID, tfss, tr, maxLabelNames, maxMetrics, deadline)
}

// SearchLabelValuesWithFiltersOnTimeRange searches for label values for the given labelName, filters and tr.
func (s *Storage) SearchLabelValuesWithFiltersOnTimeRange(qt *querytracer.Tracer, accountID, projectID uint32, labelName string, tfss []*TagFilters,
	tr TimeRange, maxLabelValues, maxMetrics int, deadline uint64,
) ([]string, error) {
	return s.idb().SearchLabelValuesWithFiltersOnTimeRange(qt, accountID, projectID, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given tagKey and tagValuePrefix on the given tr.
//
// This allows implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or similar APIs.
//
// If more than maxTagValueSuffixes suffixes is found, then only the first maxTagValueSuffixes suffixes is returned.
func (s *Storage) SearchTagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr TimeRange, tagKey, tagValuePrefix string,
	delimiter byte, maxTagValueSuffixes int, deadline uint64,
) ([]string, error) {
	return s.idb().SearchTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes, deadline)
}

// SearchGraphitePaths returns all the matching paths for the given graphite query on the given tr.
func (s *Storage) SearchGraphitePaths(qt *querytracer.Tracer, accountID, projectID uint32, tr TimeRange, query []byte, maxPaths int, deadline uint64) ([]string, error) {
	query = replaceAlternateRegexpsWithGraphiteWildcards(query)
	return s.searchGraphitePaths(qt, accountID, projectID, tr, nil, query, maxPaths, deadline)
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

func (s *Storage) searchGraphitePaths(qt *querytracer.Tracer, accountID, projectID uint32, tr TimeRange, qHead, qTail []byte, maxPaths int, deadline uint64) ([]string, error) {
	n := bytes.IndexAny(qTail, "*[{")
	if n < 0 {
		// Verify that qHead matches a metric name.
		qHead = append(qHead, qTail...)
		suffixes, err := s.SearchTagValueSuffixes(qt, accountID, projectID, tr, "", bytesutil.ToUnsafeString(qHead), '.', 1, deadline)
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
	suffixes, err := s.SearchTagValueSuffixes(qt, accountID, projectID, tr, "", bytesutil.ToUnsafeString(qHead), '.', maxPaths, deadline)
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
		ps, err := s.searchGraphitePaths(qt, accountID, projectID, tr, qHead, qTail, maxPaths, deadline)
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
	return metricsql.CompileRegexp(reStr)
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

// GetSeriesCount returns the approximate number of unique time series for the given (accountID, projectID).
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (s *Storage) GetSeriesCount(accountID, projectID uint32, deadline uint64) (uint64, error) {
	return s.idb().GetSeriesCount(accountID, projectID, deadline)
}

// SearchTenants returns list of registered tenants on the given tr.
func (s *Storage) SearchTenants(qt *querytracer.Tracer, tr TimeRange, deadline uint64) ([]string, error) {
	return s.idb().SearchTenants(qt, tr, deadline)
}

// GetTSDBStatus returns TSDB status data for /api/v1/status/tsdb
func (s *Storage) GetTSDBStatus(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	return s.idb().GetTSDBStatus(qt, accountID, projectID, tfss, date, focusLabel, topN, maxMetrics, deadline)
}

// MetricRow is a metric to insert into storage.
type MetricRow struct {
	// MetricNameRaw contains raw metric name, which must be decoded
	// with MetricName.UnmarshalRaw.
	MetricNameRaw []byte

	Timestamp int64
	Value     float64
}

// ResetX resets mr after UnmarshalX or after UnmarshalMetricRows
func (mr *MetricRow) ResetX() {
	mr.MetricNameRaw = nil
	mr.Timestamp = 0
	mr.Value = 0
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
	return MarshalMetricRow(dst, mr.MetricNameRaw, mr.Timestamp, mr.Value)
}

// MarshalMetricRow marshals MetricRow data to dst and returns the result.
func MarshalMetricRow(dst []byte, metricNameRaw []byte, timestamp int64, value float64) []byte {
	dst = encoding.MarshalBytes(dst, metricNameRaw)
	dst = encoding.MarshalUint64(dst, uint64(timestamp))
	dst = encoding.MarshalUint64(dst, math.Float64bits(value))
	return dst
}

// UnmarshalMetricRows appends unmarshaled MetricRow items from src to dst and returns the result.
//
// Up to maxRows rows are unmarshaled at once. The remaining byte slice is returned to the caller.
//
// The returned MetricRow items refer to src, so they become invalid as soon as src changes.
func UnmarshalMetricRows(dst []MetricRow, src []byte, maxRows int) ([]MetricRow, []byte, error) {
	for len(src) > 0 && maxRows > 0 {
		if len(dst) < cap(dst) {
			dst = dst[:len(dst)+1]
		} else {
			dst = append(dst, MetricRow{})
		}
		mr := &dst[len(dst)-1]
		tail, err := mr.UnmarshalX(src)
		if err != nil {
			return dst, tail, err
		}
		src = tail
		maxRows--
	}
	return dst, src, nil
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
//
// The caller should limit the number of concurrent AddRows calls to the number
// of available CPU cores in order to limit memory usage.
func (s *Storage) AddRows(mrs []MetricRow, precisionBits uint8) error {
	if len(mrs) == 0 {
		return nil
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

// RegisterMetricNames registers all the metric names from mrs in the indexdb, so they can be queried later.
//
// The the MetricRow.Timestamp is used for registering the metric name at the given day according to the timestamp.
// Th MetricRow.Value field is ignored.
func (s *Storage) RegisterMetricNames(qt *querytracer.Tracer, mrs []MetricRow) {
	qt = qt.NewChild("registering %d series", len(mrs))
	defer qt.Done()
	var metricNameBuf []byte
	var genTSID generationTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	var seriesRepopulated uint64

	idb := s.idb()
	generation := idb.generation
	is := idb.getIndexSearch(0, 0, noDeadline)
	defer idb.putIndexSearch(is)
	var firstWarn error
	for i := range mrs {
		mr := &mrs[i]
		date := uint64(mr.Timestamp) / msecPerDay
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			// Fast path - mr.MetricNameRaw has been already registered in the current idb.
			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				continue
			}
			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.

				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					// Do not stop adding rows on error - just skip invalid row.
					// This guarantees that invalid rows don't prevent
					// from adding valid rows into the storage.
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot umarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					continue
				}
				mn.sortTags()

				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)
				seriesRepopulated++
			}
			continue
		}

		// Slow path - search TSID for the given metricName in indexdb.

		// Construct canonical metric name - it is used below.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			// Do not stop adding rows on error - just skip invalid row.
			// This guarantees that invalid rows don't prevent
			// from adding valid rows into the storage.
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot umarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		if is.getTSIDByMetricName(&genTSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				continue
			}

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				seriesRepopulated++
			}
			s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)
			continue
		}

		// Slowest path - there is no TSID in indexdb for the given mr.MetricNameRaw. Create it.
		generateTSID(&genTSID.TSID, mn)

		if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			continue
		}

		// Schedule creating TSID indexes instead of creating them synchronously.
		// This should keep stable the ingestion rate when new time series are ingested.
		createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
		genTSID.generation = generation
		s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)
	}

	atomic.AddUint64(&s.timeseriesRepopulated, seriesRepopulated)

	// There is no need in pre-filling idbNext here, since RegisterMetricNames() is rarely called.
	// So it is OK to register metric names in blocking manner after indexdb rotation.

	if firstWarn != nil {
		logger.Warnf("cannot create some metrics: %s", firstWarn)
	}
}

func (s *Storage) add(rows []rawRow, dstMrs []*MetricRow, mrs []MetricRow, precisionBits uint8) error {
	idb := s.idb()
	generation := idb.generation
	is := idb.getIndexSearch(0, 0, noDeadline)
	defer idb.putIndexSearch(is)

	mn := GetMetricName()
	defer PutMetricName(mn)

	var (
		// These vars are used for speeding up bulk imports of multiple adjacent rows for the same metricName.
		prevTSID          TSID
		prevMetricNameRaw []byte
	)
	var metricNameBuf []byte

	var slowInsertsCount uint64
	var newSeriesCount uint64
	var seriesRepopulated uint64

	minTimestamp, maxTimestamp := s.tb.getMinMaxTimestamps()

	var genTSID generationTSID

	// Return only the first error, since it has no sense in returning all errors.
	var firstWarn error

	j := 0
	for i := range mrs {
		mr := &mrs[i]
		var isStaleNan bool
		if math.IsNaN(mr.Value) {
			if !decimal.IsStaleNaN(mr.Value) {
				// Skip NaNs other than Prometheus staleness marker, since the underlying encoding
				// doesn't know how to work with them.
				continue
			}
			isStaleNan = true
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

		// Search for TSID for the given mr.MetricNameRaw and store it at r.TSID.
		if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
			// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
			// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.
			r.TSID = prevTSID
			continue
		}
		if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been found in cache and isn't deleted.
			// There is no need in checking whether r.TSID.MetricID is deleted, since tsidCache doesn't
			// contain MetricName->TSID entries for deleted time series.
			// See Storage.DeleteSeries code for details.

			if !s.registerSeriesCardinality(r.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				j--
				continue
			}
			r.TSID = genTSID.TSID
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				date := uint64(r.Timestamp) / msecPerDay

				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					j--
					continue
				}
				mn.sortTags()

				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)
				seriesRepopulated++
				slowInsertsCount++
			}
			continue
		}

		// Slow path - the TSID for the given mr.MetricNameRaw is missing in the cache.
		slowInsertsCount++

		date := uint64(r.Timestamp) / msecPerDay

		// Construct canonical metric name - it is used below.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			j--
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		// Search for TSID for the given mr.MetricNameRaw in the indexdb.
		if is.getTSIDByMetricName(&genTSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				j--
				continue
			}

			if genTSID.generation < generation {
				// The found TSID is from the previous indexdb. Create it in the current indexdb.
				createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
				genTSID.generation = generation
				seriesRepopulated++
			}
			s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)

			r.TSID = genTSID.TSID
			prevTSID = genTSID.TSID
			prevMetricNameRaw = mr.MetricNameRaw
			continue
		}

		// If sample is stale and its TSID wasn't found in cache and in indexdb,
		// then we skip it. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5069
		if isStaleNan {
			j--
			continue
		}

		// Slowest path - the TSID for the given mr.MetricNameRaw isn't found in indexdb. Create it.
		generateTSID(&genTSID.TSID, mn)

		if !s.registerSeriesCardinality(genTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			j--
			continue
		}

		createAllIndexesForMetricName(is, mn, &genTSID.TSID, date)
		genTSID.generation = generation
		s.putSeriesToCache(mr.MetricNameRaw, &genTSID, date)
		newSeriesCount++

		r.TSID = genTSID.TSID
		prevTSID = r.TSID
		prevMetricNameRaw = mr.MetricNameRaw

		if logNewSeries {
			logger.Infof("new series created: %s", mn.String())
		}
	}

	atomic.AddUint64(&s.slowRowInserts, slowInsertsCount)
	atomic.AddUint64(&s.newTimeseriesCreated, newSeriesCount)
	atomic.AddUint64(&s.timeseriesRepopulated, seriesRepopulated)

	dstMrs = dstMrs[:j]
	rows = rows[:j]

	if err := s.prefillNextIndexDB(rows, dstMrs); err != nil {
		if firstWarn == nil {
			firstWarn = err
		}
	}
	if firstWarn != nil {
		storageAddRowsLogger.Warnf("warn occurred during rows addition: %s", firstWarn)
	}

	err := s.updatePerDateData(rows, dstMrs)
	if err != nil {
		err = fmt.Errorf("cannot update per-date data: %w", err)
	} else {
		s.tb.MustAddRows(rows)
	}
	if err != nil {
		return fmt.Errorf("error occurred during rows addition: %w", err)
	}
	return nil
}

var storageAddRowsLogger = logger.WithThrottler("storageAddRows", 5*time.Second)

// SetLogNewSeries updates new series logging.
//
// This function must be called before any calling any storage functions.
func SetLogNewSeries(ok bool) {
	logNewSeries = ok
}

var logNewSeries = false

func createAllIndexesForMetricName(is *indexSearch, mn *MetricName, tsid *TSID, date uint64) {
	is.createGlobalIndexes(tsid, mn)
	is.createPerDayIndexes(date, tsid, mn)
}

func (s *Storage) putSeriesToCache(metricNameRaw []byte, genTSID *generationTSID, date uint64) {
	// Store the TSID for for the current indexdb into cache,
	// so future rows for that TSID are ingested via fast path.
	s.putTSIDToCache(genTSID, metricNameRaw)

	// Register the (generation, date, metricID) entry in the cache,
	// so next time the entry is found there instead of searching for it in the indexdb.
	s.dateMetricIDCache.Set(genTSID.generation, date, genTSID.TSID.MetricID)
}

func (s *Storage) registerSeriesCardinality(metricID uint64, metricNameRaw []byte) bool {
	if sl := s.hourlySeriesLimiter; sl != nil && !sl.Add(metricID) {
		atomic.AddUint64(&s.hourlySeriesLimitRowsDropped, 1)
		logSkippedSeries(metricNameRaw, "-storage.maxHourlySeries", sl.MaxItems())
		return false
	}
	if sl := s.dailySeriesLimiter; sl != nil && !sl.Add(metricID) {
		atomic.AddUint64(&s.dailySeriesLimitRowsDropped, 1)
		logSkippedSeries(metricNameRaw, "-storage.maxDailySeries", sl.MaxItems())
		return false
	}
	return true
}

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

func (s *Storage) prefillNextIndexDB(rows []rawRow, mrs []*MetricRow) error {
	d := s.nextRetentionSeconds()
	if d >= 3600 {
		// Fast path: nothing to pre-fill because it is too early.
		// The pre-fill is started during the last hour before the indexdb rotation.
		return nil
	}

	// Slower path: less than hour left for the next indexdb rotation.
	// Pre-populate idbNext with the increasing probability until the rotation.
	// The probability increases from 0% to 100% proportioinally to d=[3600 .. 0].
	pMin := float64(d) / 3600

	idbNext := s.idbNext.Load()
	generation := idbNext.generation
	isNext := idbNext.getIndexSearch(0, 0, noDeadline)
	defer idbNext.putIndexSearch(isNext)

	var firstError error
	var genTSID generationTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	timeseriesPreCreated := uint64(0)
	for i := range rows {
		r := &rows[i]
		p := float64(uint32(fastHashUint64(r.TSID.MetricID))) / (1 << 32)
		if p < pMin {
			// Fast path: it is too early to pre-fill indexes for the given MetricID.
			continue
		}

		// Check whether the given MetricID is already present in dateMetricIDCache.
		date := uint64(r.Timestamp) / msecPerDay
		metricID := r.TSID.MetricID
		if s.dateMetricIDCache.Has(generation, date, metricID) {
			// Indexes are already pre-filled.
			continue
		}

		// Check whether the given (date, metricID) is already present in idbNext.
		if isNext.hasDateMetricIDNoExtDB(date, metricID, r.TSID.AccountID, r.TSID.ProjectID) {
			// Indexes are already pre-filled at idbNext.
			//
			// Register the (generation, date, metricID) entry in the cache,
			// so next time the entry is found there instead of searching for it in the indexdb.
			s.dateMetricIDCache.Set(generation, date, metricID)
			continue
		}

		// Slow path: pre-fill indexes in idbNext.
		metricNameRaw := mrs[i].MetricNameRaw
		if err := mn.UnmarshalRaw(metricNameRaw); err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", metricNameRaw, err)
			}
			continue
		}
		mn.sortTags()

		createAllIndexesForMetricName(isNext, mn, &r.TSID, date)
		genTSID.TSID = r.TSID
		genTSID.generation = generation
		s.putSeriesToCache(metricNameRaw, &genTSID, date)
		timeseriesPreCreated++
	}
	atomic.AddUint64(&s.timeseriesPreCreated, timeseriesPreCreated)

	return firstError
}

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

	idb := s.idb()
	generation := idb.generation

	hm := s.currHourMetricIDs.Load()
	hmPrev := s.prevHourMetricIDs.Load()
	hmPrevDate := hmPrev.hour / 24
	nextDayMetricIDs := &s.nextDayMetricIDs.Load().v
	ts := fasttime.UnixTimestamp()
	// Start pre-populating the next per-day inverted index during the last hour of the current day.
	// pMin linearly increases from 0 to 1 during the last hour of the day.
	pMin := (float64(ts%(3600*24)) / 3600) - 23
	type pendingDateMetricID struct {
		date uint64
		tsid *TSID
		mr   *MetricRow
	}
	var pendingDateMetricIDs []pendingDateMetricID
	var pendingNextDayMetricIDs []uint64
	var pendingHourEntries []pendingHourMetricIDEntry
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
			// The row belongs to the current hour. Check for the current hour cache.
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
							date: date + 1,
							tsid: &r.TSID,
							mr:   mrs[i],
						})
						pendingNextDayMetricIDs = append(pendingNextDayMetricIDs, metricID)
					}
				}
				continue
			}
			e := pendingHourMetricIDEntry{
				AccountID: r.TSID.AccountID,
				ProjectID: r.TSID.ProjectID,
				MetricID:  metricID,
			}
			pendingHourEntries = append(pendingHourEntries, e)
			if date == hmPrevDate && hmPrev.m.Has(metricID) {
				// The metricID is already registered for the current day on the previous hour.
				continue
			}
		}

		// Slower path: check global cache for (generation, date, metricID) entry.
		if s.dateMetricIDCache.Has(generation, date, metricID) {
			continue
		}
		// Slow path: store the (date, metricID) entry in the indexDB.
		pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
			date: date,
			tsid: &r.TSID,
			mr:   mrs[i],
		})
	}
	if len(pendingNextDayMetricIDs) > 0 {
		s.pendingNextDayMetricIDsLock.Lock()
		s.pendingNextDayMetricIDs.AddMulti(pendingNextDayMetricIDs)
		s.pendingNextDayMetricIDsLock.Unlock()
	}
	if len(pendingHourEntries) > 0 {
		s.pendingHourEntriesLock.Lock()
		s.pendingHourEntries = append(s.pendingHourEntries, pendingHourEntries...)
		s.pendingHourEntriesLock.Unlock()
	}
	if len(pendingDateMetricIDs) == 0 {
		// Fast path - there are no new (date, metricID) entries.
		return nil
	}

	// Slow path - add new (date, metricID) entries to indexDB.

	atomic.AddUint64(&s.slowPerDayIndexInserts, uint64(len(pendingDateMetricIDs)))
	// Sort pendingDateMetricIDs by (accountID, projectID, date, metricID) in order to speed up `is` search in the loop below.
	sort.Slice(pendingDateMetricIDs, func(i, j int) bool {
		a := pendingDateMetricIDs[i]
		b := pendingDateMetricIDs[j]
		if a.tsid.AccountID != b.tsid.AccountID {
			return a.tsid.AccountID < b.tsid.AccountID
		}
		if a.tsid.ProjectID != b.tsid.ProjectID {
			return a.tsid.ProjectID < b.tsid.ProjectID
		}
		if a.date != b.date {
			return a.date < b.date
		}
		return a.tsid.MetricID < b.tsid.MetricID
	})

	is := idb.getIndexSearch(0, 0, noDeadline)
	defer idb.putIndexSearch(is)

	var firstError error
	dateMetricIDsForCache := make([]dateMetricID, 0, len(pendingDateMetricIDs))
	mn := GetMetricName()
	for _, dmid := range pendingDateMetricIDs {
		date := dmid.date
		metricID := dmid.tsid.MetricID
		if !is.hasDateMetricIDNoExtDB(date, metricID, dmid.tsid.AccountID, dmid.tsid.ProjectID) {
			// The (date, metricID) entry is missing in the indexDB. Add it there together with per-day indexes.
			// It is OK if the (date, metricID) entry is added multiple times to indexdb
			// by concurrent goroutines.
			if err := mn.UnmarshalRaw(dmid.mr.MetricNameRaw); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", dmid.mr.MetricNameRaw, err)
				}
				continue
			}
			mn.sortTags()
			is.createPerDayIndexes(date, dmid.tsid, mn)
		}
		dateMetricIDsForCache = append(dateMetricIDsForCache, dateMetricID{
			date:     date,
			metricID: metricID,
		})
	}
	PutMetricName(mn)
	// The (date, metricID) entries must be added to cache only after they have been successfully added to indexDB.
	s.dateMetricIDCache.Store(generation, dateMetricIDsForCache)
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
	byDate atomic.Pointer[byDateMetricIDMap]

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

func (dmc *dateMetricIDCache) resetLocked() {
	// Do not reset syncsCount and resetsCount
	dmc.byDate.Store(newByDateMetricIDMap())
	dmc.byDateMutable = newByDateMetricIDMap()
	dmc.nextSyncDeadline = 10 + fasttime.UnixTimestamp()

	atomic.AddUint64(&dmc.resetsCount, 1)
}

func (dmc *dateMetricIDCache) EntriesCount() int {
	byDate := dmc.byDate.Load()
	n := 0
	for _, e := range byDate.m {
		n += e.v.Len()
	}
	return n
}

func (dmc *dateMetricIDCache) SizeBytes() uint64 {
	byDate := dmc.byDate.Load()
	n := uint64(0)
	for _, e := range byDate.m {
		n += e.v.SizeBytes()
	}
	return n
}

func (dmc *dateMetricIDCache) Has(generation, date, metricID uint64) bool {
	byDate := dmc.byDate.Load()
	v := byDate.get(generation, date)
	if v.Has(metricID) {
		// Fast path.
		// The majority of calls must go here.
		return true
	}

	// Slow path. Check mutable map.
	dmc.mu.Lock()
	v = dmc.byDateMutable.get(generation, date)
	ok := v.Has(metricID)
	dmc.syncLockedIfNeeded()
	dmc.mu.Unlock()

	return ok
}

type dateMetricID struct {
	date     uint64
	metricID uint64
}

func (dmc *dateMetricIDCache) Store(generation uint64, dmids []dateMetricID) {
	var prevDate uint64
	metricIDs := make([]uint64, 0, len(dmids))
	dmc.mu.Lock()
	for _, dmid := range dmids {
		if prevDate == dmid.date {
			metricIDs = append(metricIDs, dmid.metricID)
			continue
		}
		if len(metricIDs) > 0 {
			v := dmc.byDateMutable.getOrCreate(generation, prevDate)
			v.AddMulti(metricIDs)
		}
		metricIDs = append(metricIDs[:0], dmid.metricID)
		prevDate = dmid.date
	}
	if len(metricIDs) > 0 {
		v := dmc.byDateMutable.getOrCreate(generation, prevDate)
		v.AddMulti(metricIDs)
	}
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) Set(generation, date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(generation, date)
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

	// Merge data from byDate into byDateMutable and then atomically replace byDate with the merged data.
	byDate := dmc.byDate.Load()
	byDateMutable := dmc.byDateMutable
	for k, e := range byDateMutable.m {
		v := byDate.get(k.generation, k.date)
		if v == nil {
			// Nothing to merge
			continue
		}
		v = v.Clone()
		v.Union(&e.v)
		dme := &byDateMetricIDEntry{
			k: k,
			v: *v,
		}
		byDateMutable.m[k] = dme
		he := byDateMutable.hotEntry.Load()
		if he.k == k {
			byDateMutable.hotEntry.Store(dme)
		}
	}
	// Copy entries from byDate, which are missing in byDateMutable
	for k, e := range byDate.m {
		v := byDateMutable.get(k.generation, k.date)
		if v != nil {
			continue
		}
		byDateMutable.m[k] = e
	}

	// Atomically replace byDate with byDateMutable
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()

	atomic.AddUint64(&dmc.syncsCount, 1)

	if dmc.SizeBytes() > uint64(memory.Allowed())/256 {
		dmc.resetLocked()
	}
}

type byDateMetricIDMap struct {
	hotEntry atomic.Pointer[byDateMetricIDEntry]
	m        map[generationDateKey]*byDateMetricIDEntry
}

type generationDateKey struct {
	generation uint64
	date       uint64
}

func newByDateMetricIDMap() *byDateMetricIDMap {
	dmm := &byDateMetricIDMap{
		m: make(map[generationDateKey]*byDateMetricIDEntry),
	}
	dmm.hotEntry.Store(&byDateMetricIDEntry{})
	return dmm
}

func (dmm *byDateMetricIDMap) get(generation, date uint64) *uint64set.Set {
	hotEntry := dmm.hotEntry.Load()
	if hotEntry.k.generation == generation && hotEntry.k.date == date {
		// Fast path
		return &hotEntry.v
	}
	// Slow path
	k := generationDateKey{
		generation: generation,
		date:       date,
	}
	e := dmm.m[k]
	if e == nil {
		return nil
	}
	dmm.hotEntry.Store(e)
	return &e.v
}

func (dmm *byDateMetricIDMap) getOrCreate(generation, date uint64) *uint64set.Set {
	v := dmm.get(generation, date)
	if v != nil {
		return v
	}
	k := generationDateKey{
		generation: generation,
		date:       date,
	}
	e := &byDateMetricIDEntry{
		k: k,
	}
	dmm.m[k] = e
	return &e.v
}

type byDateMetricIDEntry struct {
	k generationDateKey
	v uint64set.Set
}

func (s *Storage) updateNextDayMetricIDs(date uint64) {
	generation := s.idb().generation
	e := s.nextDayMetricIDs.Load()
	s.pendingNextDayMetricIDsLock.Lock()
	pendingMetricIDs := s.pendingNextDayMetricIDs
	s.pendingNextDayMetricIDs = &uint64set.Set{}
	s.pendingNextDayMetricIDsLock.Unlock()
	if pendingMetricIDs.Len() == 0 && e.k.generation == generation && e.k.date == date {
		// Fast path: nothing to update.
		return
	}

	// Slow path: union pendingMetricIDs with e.v
	if e.k.generation == generation && e.k.date == date {
		pendingMetricIDs.Union(&e.v)
	} else {
		// Do not add pendingMetricIDs from the previous day to the current day,
		// since this may result in missing registration of the metricIDs in the per-day inverted index.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309
		pendingMetricIDs = &uint64set.Set{}
	}
	k := generationDateKey{
		generation: generation,
		date:       date,
	}
	eNew := &byDateMetricIDEntry{
		k: k,
		v: *pendingMetricIDs,
	}
	s.nextDayMetricIDs.Store(eNew)
}

func (s *Storage) updateCurrHourMetricIDs(hour uint64) {
	hm := s.currHourMetricIDs.Load()
	var newEntries []pendingHourMetricIDEntry
	s.pendingHourEntriesLock.Lock()
	if len(s.pendingHourEntries) < cap(s.pendingHourEntries)/2 {
		// Free up memory occupied by s.pendingHourEntries,
		// since it looks like now it needs much lower amounts of memory.
		newEntries = s.pendingHourEntries
		s.pendingHourEntries = nil
	} else {
		// Copy s.pendingHourEntries to newEntries and re-use s.pendingHourEntries capacity,
		// since its memory usage is at stable state.
		// This should reduce the number of additional memory re-allocations
		// when adding new items to s.pendingHourEntries.
		newEntries = append([]pendingHourMetricIDEntry{}, s.pendingHourEntries...)
		s.pendingHourEntries = s.pendingHourEntries[:0]
	}
	s.pendingHourEntriesLock.Unlock()

	if len(newEntries) == 0 && hm.hour == hour {
		// Fast path: nothing to update.
		return
	}

	// Slow path: hm.m must be updated with non-empty s.pendingHourEntries.
	var m *uint64set.Set
	var byTenant map[accountProjectKey]*uint64set.Set
	if hm.hour == hour {
		m = hm.m.Clone()
		byTenant = make(map[accountProjectKey]*uint64set.Set, len(hm.byTenant))
		for k, e := range hm.byTenant {
			byTenant[k] = e.Clone()
		}
	} else {
		m = &uint64set.Set{}
		byTenant = make(map[accountProjectKey]*uint64set.Set)
	}
	if hm.hour == hour || hour%24 != 0 {
		// Do not add pending metricIDs from the previous hour on the previous day to the current hour,
		// since this may result in missing registration of the metricIDs in the per-day inverted index.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309
		for _, x := range newEntries {
			m.Add(x.MetricID)
			k := accountProjectKey{
				AccountID: x.AccountID,
				ProjectID: x.ProjectID,
			}
			e := byTenant[k]
			if e == nil {
				e = &uint64set.Set{}
				byTenant[k] = e
			}
			e.Add(x.MetricID)
		}
	}

	hmNew := &hourMetricIDs{
		m:        m,
		byTenant: byTenant,
		hour:     hour,
	}
	s.currHourMetricIDs.Store(hmNew)
	if hm.hour != hour {
		s.prevHourMetricIDs.Store(hm)
	}
}

type hourMetricIDs struct {
	m        *uint64set.Set
	byTenant map[accountProjectKey]*uint64set.Set
	hour     uint64
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

func (s *Storage) mustOpenIndexDBTables(path string) (next, curr, prev *indexDB) {
	fs.MustMkdirIfNotExist(path)
	fs.MustRemoveTemporaryDirs(path)

	// Search for the three most recent tables - the prev, curr and next.
	des := fs.MustReadDir(path)
	var tableNames []string
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories.
			continue
		}
		tableName := de.Name()
		if !indexDBTableNameRegexp.MatchString(tableName) {
			// Skip invalid directories.
			continue
		}
		tableNames = append(tableNames, tableName)
	}
	sort.Slice(tableNames, func(i, j int) bool {
		return tableNames[i] < tableNames[j]
	})
	switch len(tableNames) {
	case 0:
		prevName := nextIndexDBTableName()
		currName := nextIndexDBTableName()
		nextName := nextIndexDBTableName()
		tableNames = append(tableNames, prevName, currName, nextName)
	case 1:
		currName := nextIndexDBTableName()
		nextName := nextIndexDBTableName()
		tableNames = append(tableNames, currName, nextName)
	case 2:
		nextName := nextIndexDBTableName()
		tableNames = append(tableNames, nextName)
	default:
		// Remove all the tables except the last three tables.
		for _, tn := range tableNames[:len(tableNames)-3] {
			pathToRemove := filepath.Join(path, tn)
			logger.Infof("removing obsolete indexdb dir %q...", pathToRemove)
			fs.MustRemoveAll(pathToRemove)
			logger.Infof("removed obsolete indexdb dir %q", pathToRemove)
		}
		fs.MustSyncPath(path)

		tableNames = tableNames[len(tableNames)-3:]
	}

	// Open tables
	nextPath := filepath.Join(path, tableNames[2])
	currPath := filepath.Join(path, tableNames[1])
	prevPath := filepath.Join(path, tableNames[0])

	next = mustOpenIndexDB(nextPath, s, &s.isReadOnly)
	curr = mustOpenIndexDB(currPath, s, &s.isReadOnly)
	prev = mustOpenIndexDB(prevPath, s, &s.isReadOnly)

	return next, curr, prev
}

var indexDBTableNameRegexp = regexp.MustCompile("^[0-9A-F]{16}$")

func nextIndexDBTableName() string {
	n := atomic.AddUint64(&indexDBTableIdx, 1)
	return fmt.Sprintf("%016X", n)
}

var indexDBTableIdx = uint64(time.Now().UnixNano())
