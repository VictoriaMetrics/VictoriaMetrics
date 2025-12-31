package storage

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/cespare/xxhash/v2"
)

const (
	retention31Days = 31 * 24 * time.Hour
	retentionMax    = 100 * 12 * retention31Days
	idbPrefilStart  = time.Hour
)

// Storage represents TSDB storage.
type Storage struct {
	rowsReceivedTotal atomic.Uint64
	rowsAddedTotal    atomic.Uint64

	tooSmallTimestampRows atomic.Uint64
	tooBigTimestampRows   atomic.Uint64
	invalidRawMetricNames atomic.Uint64

	timeseriesRepopulated  atomic.Uint64
	timeseriesPreCreated   atomic.Uint64
	newTimeseriesCreated   atomic.Uint64
	slowRowInserts         atomic.Uint64
	slowPerDayIndexInserts atomic.Uint64

	hourlySeriesLimitRowsDropped atomic.Uint64
	dailySeriesLimitRowsDropped  atomic.Uint64

	// legacyNextRotationTimestamp is a timestamp in seconds of the next legacy
	// indexdb rotation.
	legacyNextRotationTimestamp atomic.Int64

	path           string
	cachePath      string
	retentionMsecs int64

	// lock file for exclusive access to the storage on the given path.
	flockF *os.File

	// legacyIndexDBs contains the legacy previous and current
	// IndexDBs if they existed on filesystem before partition
	// index was introduced. The pointer is nil if there are no legacy
	// IndexDBs on filesystem.
	//
	// The support of legacy IndexDBs is required to provide forward
	// compatibility with partition index.
	legacyIndexDBs atomic.Pointer[legacyIndexDBs]

	disablePerDayIndex bool

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

	// Fast cache for MetricID values occurred during the current hour.
	currHourMetricIDs atomic.Pointer[hourMetricIDs]

	// Fast cache for MetricID values occurred during the previous hour.
	prevHourMetricIDs atomic.Pointer[hourMetricIDs]

	// Fast cache for pre-populating per-day inverted index for the next day.
	// This is needed in order to remove CPU usage spikes at 00:00 UTC
	// due to creation of per-day inverted index for active time series.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 for details.
	nextDayMetricIDs atomic.Pointer[nextDayMetricIDs]

	// Pending MetricID values to be added to currHourMetricIDs.
	pendingHourEntriesLock sync.Mutex
	pendingHourEntries     *uint64set.Set

	// Pending MetricIDs to be added to nextDayMetricIDs.
	pendingNextDayMetricIDsLock sync.Mutex
	pendingNextDayMetricIDs     *uint64set.Set

	stopCh chan struct{}

	currHourMetricIDsUpdaterWG sync.WaitGroup
	nextDayMetricIDsUpdaterWG  sync.WaitGroup
	legacyRetentionWatcherWG   sync.WaitGroup
	freeDiskSpaceWatcherWG     sync.WaitGroup

	// The snapshotLock prevents from concurrent creation of snapshots,
	// since this may result in snapshots without recently added data,
	// which may be in the process of flushing to disk by concurrently running
	// snapshot process.
	snapshotLock sync.Mutex

	// The minimum timestamp when composite index search can be used.
	minTimestampForCompositeIndex int64

	// missingMetricIDs maps metricID to the deadline in unix timestamp seconds
	// after which all the indexdb entries for the given metricID
	// must be deleted if index entry isn't found by the given metricID.
	// This is used inside searchMetricNameWithCache() and SearchTSIDs()
	// for detecting permanently missing metricID->metricName/TSID entries.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5959
	missingMetricIDsLock          sync.Mutex
	missingMetricIDs              map[uint64]uint64
	missingMetricIDsResetDeadline uint64

	// isReadOnly is set to true when the storage is in read-only mode.
	isReadOnly atomic.Bool

	metricsTracker *metricnamestats.Tracker

	// idbPrefillStartSeconds defines the start time of the idbNext prefill.
	// It helps to spread load in time for index records creation and reduce resource usage.
	idbPrefillStartSeconds int64

	// logNewSeries is used for logging the new series. We will log new series when logNewSeries is true or logNewSeriesUntil is greater than the current time.
	logNewSeries atomic.Bool

	// logNewSeriesUntil is the timestamp until which new series will be logged. We will log new series when logNewSeries is true or logNewSeriesUntil is greater than the current time.
	logNewSeriesUntil atomic.Uint64

	metadataStorage *metricsmetadata.Storage
}

// OpenOptions optional args for MustOpenStorage
type OpenOptions struct {
	Retention             time.Duration
	MaxHourlySeries       int
	MaxDailySeries        int
	DisablePerDayIndex    bool
	TrackMetricNamesStats bool
	IDBPrefillStart       time.Duration
	LogNewSeries          bool
}

// MustOpenStorage opens storage on the given path with the given retentionMsecs.
//
// TODO(@rtm0): Extract legacy IndexDB initialization code into a separate
// method and move it to storage_legacy.go.
func MustOpenStorage(path string, opts OpenOptions) *Storage {
	path, err := filepath.Abs(path)
	if err != nil {
		logger.Panicf("FATAL: cannot determine absolute path for %q: %s", path, err)
	}
	retention := opts.Retention
	if retention <= 0 || retention > retentionMax {
		retention = retentionMax
	}
	idbPrefillStart := opts.IDBPrefillStart
	if idbPrefillStart <= 0 {
		idbPrefillStart = time.Hour
	}
	s := &Storage{
		path:                   path,
		cachePath:              filepath.Join(path, cacheDirname),
		retentionMsecs:         retention.Milliseconds(),
		stopCh:                 make(chan struct{}),
		idbPrefillStartSeconds: idbPrefillStart.Milliseconds() / 1000,
	}
	s.logNewSeries.Store(opts.LogNewSeries)

	fs.MustMkdirIfNotExist(path)

	// Check whether the cache directory must be removed
	// It is removed if it contains resetCacheOnStartupFilename.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1447 for details.
	if fs.IsPathExist(filepath.Join(s.cachePath, resetCacheOnStartupFilename)) {
		logger.Infof("removing cache directory at %q, since it contains `%s` file...", s.cachePath, resetCacheOnStartupFilename)
		// Do not use fs.MustRemoveDir() here, since the cache directory may be mounted
		// to a separate filesystem. In this case the fs.MustRemoveDir() will fail while
		// trying to remove the mount root.
		fs.MustRemoveDirContents(s.cachePath)
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

	// Initialize series cardinality limiter.
	if opts.MaxHourlySeries > 0 {
		s.hourlySeriesLimiter = bloomfilter.NewLimiter(opts.MaxHourlySeries, time.Hour)
	}
	if opts.MaxDailySeries > 0 {
		s.dailySeriesLimiter = bloomfilter.NewLimiter(opts.MaxDailySeries, 24*time.Hour)
	}

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache(tsidCacheFilename, getTSIDCacheSize())
	s.metricIDCache = s.mustLoadCache(metricIDCacheFilename, mem/16)
	s.metricNameCache = s.mustLoadCache(metricNameCacheFilename, getMetricNamesCacheSize())

	if opts.TrackMetricNamesStats {
		mnt := metricnamestats.MustLoadFrom(filepath.Join(s.cachePath, metricNameTrackerFilename), uint64(getMetricNamesStatsCacheSize()))
		s.metricsTracker = mnt
		if mnt.IsEmpty() {
			// metric names tracker performs attempt to track timeseries during ingestion only at tsid cache miss.
			// It allows to do not decrease storage performance.
			logger.Infof("resetting tsidCache in order to properly track metric name usage stats")
			s.tsidCache.Reset()
		}
	}

	s.metadataStorage = metricsmetadata.NewStorage(getMetadataStorageSize())

	// Load metadata
	metadataDir := filepath.Join(path, metadataDirname)
	isEmptyDB := !fs.IsPathExist(filepath.Join(path, indexdbDirname))
	fs.MustMkdirIfNotExist(metadataDir)
	s.minTimestampForCompositeIndex = mustGetMinTimestampForCompositeIndex(metadataDir, isEmptyDB)

	s.disablePerDayIndex = opts.DisablePerDayIndex

	// Load legacy indexDBs.
	legacyIDBPath := filepath.Join(path, indexdbDirname)
	legacyIDBs := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	s.legacyIndexDBs.Store(legacyIDBs)
	// Initialize legacyNextRotationTimestamp
	nowSecs := int64(fasttime.UnixTimestamp())
	retentionSecs := retention.Milliseconds() / 1000 // not .Seconds() because unnecessary float64 conversion
	nextRotationTimestamp := legacyNextRetentionDeadlineSeconds(nowSecs, retentionSecs, legacyRetentionTimezoneOffsetSecs)
	s.legacyNextRotationTimestamp.Store(nextRotationTimestamp)

	// check for free disk space before opening the table
	// to prevent unexpected part merges. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4023
	s.startFreeDiskSpaceWatcher()

	// Load data
	tablePath := filepath.Join(path, dataDirname)
	tb := mustOpenTable(tablePath, s)
	s.tb = tb

	// Add deleted metricIDs from legacy previous and current indexDBs to every
	// partition indexDB. Also add deleted metricIDs from current indexDB to the
	// previous one, because previous may contain the same metrics that wasn't marked as deleted.
	legacyDeletedMetricIDs := &uint64set.Set{}
	idbPrev := legacyIDBs.getIDBPrev()
	if idbPrev != nil {
		legacyDeletedMetricIDs.Union(idbPrev.getDeletedMetricIDs())
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		legacyDeletedMetricIDs.Union(idbCurr.getDeletedMetricIDs())
	}
	if idbPrev != nil {
		idbPrev.setDeletedMetricIDs(legacyDeletedMetricIDs)
	}
	ptws := tb.GetAllPartitions(nil)
	for _, ptw := range ptws {
		ptw.pt.idb.updateDeletedMetricIDs(legacyDeletedMetricIDs)
	}
	tb.PutPartitions(ptws)

	// Load prevHourMetricIDs, currHourMetricIDs, and nextDayMetricIDs caches
	// after the data table is opened since they require the partition index to
	// operate properly.
	hour := fasttime.UnixHour()
	hmCurr := s.mustLoadHourMetricIDs(hour, currHourMetricIDsFilename)
	hmPrev := s.mustLoadHourMetricIDs(hour-1, prevHourMetricIDsFilename)
	s.currHourMetricIDs.Store(hmCurr)
	s.prevHourMetricIDs.Store(hmPrev)
	s.pendingHourEntries = &uint64set.Set{}
	// Load nextDayMetricIDs cache after the data table is opened since it
	// requires the partition index to operate properly.
	date := fasttime.UnixDate()
	nextDayMetricIDs := s.mustLoadNextDayMetricIDs(date)
	s.nextDayMetricIDs.Store(nextDayMetricIDs)
	s.pendingNextDayMetricIDs = &uint64set.Set{}

	s.startCurrHourMetricIDsUpdater()
	s.startNextDayMetricIDsUpdater()
	s.startLegacyRetentionWatcher()

	return s
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

var maxMetricNamesStatsCacheSize int

// SetMetricNamesStatsCacheSize overrides the default size of storage/metricNamesStatsTracker
func SetMetricNamesStatsCacheSize(size int) {
	maxMetricNamesStatsCacheSize = size
}

func getMetricNamesStatsCacheSize() int {
	if maxMetricNamesStatsCacheSize <= 0 {
		return memory.Allowed() / 100
	}
	return maxMetricNamesStatsCacheSize
}

var maxMetricNameCacheSize int

// SetMetricNameCacheSize overrides the default size of storage/metricName cache
func SetMetricNameCacheSize(size int) {
	maxMetricNameCacheSize = size
}

func getMetricNamesCacheSize() int {
	if maxMetricNameCacheSize <= 0 {
		return memory.Allowed() / 10
	}
	return maxMetricNameCacheSize
}

var maxMetadataStorageSize int

// SetMetadataStorageSize overrides the default size of the metadata store
func SetMetadataStorageSize(size int) {
	maxMetadataStorageSize = size
}

func getMetadataStorageSize() int {
	if maxMetadataStorageSize <= 0 {
		return memory.Allowed() / 100
	}
	return maxMetadataStorageSize
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
	s.tb.DebugFlush()

	// Legacy indexDBs do not accept new entries but they continue to accept
	// deletes. I.e. they are not completely read-only and need to be flushed
	// too.
	s.legacyDebugFlush()

	hour := fasttime.UnixHour()
	s.updateCurrHourMetricIDs(hour)
}

// MustCreateSnapshot creates snapshot for s and returns the snapshot name.
//
// The method panics in case of any error since it does not accept any user
// input and therefore the error is not recoverable.
func (s *Storage) MustCreateSnapshot() string {
	logger.Infof("creating Storage snapshot for %q...", s.path)
	startTime := time.Now()

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	snapshotName := snapshotutil.NewName()
	srcDir := s.path
	dstDir := filepath.Join(srcDir, snapshotsDirname, snapshotName)
	fs.MustMkdirFailIfExist(dstDir)

	smallDir, bigDir, indexDBDir := s.tb.MustCreateSnapshot(snapshotName)

	dstDataDir := filepath.Join(dstDir, dataDirname)
	fs.MustMkdirFailIfExist(dstDataDir)

	dstSmallDir := filepath.Join(dstDataDir, smallDirname)
	fs.MustSymlinkRelative(smallDir, dstSmallDir)

	dstBigDir := filepath.Join(dstDataDir, bigDirname)
	fs.MustSymlinkRelative(bigDir, dstBigDir)

	dstIndexDBDir := filepath.Join(dstDataDir, indexdbDirname)
	fs.MustSymlinkRelative(indexDBDir, dstIndexDBDir)

	fs.MustSyncPath(dstDataDir)

	srcMetadataDir := filepath.Join(srcDir, metadataDirname)
	dstMetadataDir := filepath.Join(dstDir, metadataDirname)
	fs.MustCopyDirectory(srcMetadataDir, dstMetadataDir)

	s.legacyCreateSnapshot(snapshotName, srcDir, dstDir)

	fs.MustSyncPathAndParentDir(dstDir)

	logger.Infof("created Storage snapshot for %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	return snapshotName
}

func (s *Storage) mustGetSnapshotsCount() int {
	snapshotNames := s.MustListSnapshots()
	return len(snapshotNames)
}

// MustListSnapshots returns sorted list of existing snapshots for s.
//
// The method panics in case of any error since it does not accept any user
// input and therefore the error is not recoverable.
func (s *Storage) MustListSnapshots() []string {
	snapshotsPath := filepath.Join(s.path, snapshotsDirname)
	d, err := os.Open(snapshotsPath)
	if err != nil {
		logger.Panicf("FATAL: cannot open snapshots directory: %v", err)
	}
	defer fs.MustClose(d)

	fnames, err := d.Readdirnames(-1)
	if err != nil {
		logger.Panicf("FATAL: cannot read snapshots directory at %q: %v", snapshotsPath, err)
	}
	snapshotNames := make([]string, 0, len(fnames))
	for _, fname := range fnames {
		if err := snapshotutil.Validate(fname); err != nil {
			continue
		}
		snapshotNames = append(snapshotNames, fname)
	}
	sort.Strings(snapshotNames)
	return snapshotNames
}

// DeleteSnapshot deletes the given snapshot.
func (s *Storage) DeleteSnapshot(snapshotName string) error {
	if err := snapshotutil.Validate(snapshotName); err != nil {
		return fmt.Errorf("invalid snapshotName %q: %w", snapshotName, err)
	}
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)

	logger.Infof("deleting snapshot %q...", snapshotPath)
	startTime := time.Now()

	s.tb.MustDeleteSnapshot(snapshotName)
	idbPath := filepath.Join(s.path, indexdbDirname, snapshotsDirname, snapshotName)
	fs.MustRemoveDir(idbPath)
	fs.MustRemoveDir(snapshotPath)

	logger.Infof("deleted snapshot %q in %.3f seconds", snapshotPath, time.Since(startTime).Seconds())

	return nil
}

// MustDeleteStaleSnapshots deletes snapshot older than given maxAge
//
// The method panics in case of any error since it is unrelated to the user
// input and indicates a bug in storage or a problem with the underlying file
// system.
func (s *Storage) MustDeleteStaleSnapshots(maxAge time.Duration) {
	list := s.MustListSnapshots()
	expireDeadline := time.Now().UTC().Add(-maxAge)
	for _, snapshotName := range list {
		t, err := snapshotutil.Time(snapshotName)
		if err != nil {
			// Panic because MustListSnapshots() is expected to return valid
			// snapshot names only.
			logger.Panicf("BUG: cannot parse snapshot date from %q: %v", snapshotName, err)
		}
		if t.Before(expireDeadline) {
			if err := s.DeleteSnapshot(snapshotName); err != nil {
				// Panic because MustListSnapshots() is expected to return valid
				// snapshot names only and DeleteSnapshot() fails only if the
				// snapshot name is invalid.
				logger.Panicf("BUG: cannot delete snapshot %q: %v", snapshotName, err)
			}
		}
	}
}

// Metrics contains essential metrics for the Storage.
type Metrics struct {
	RowsReceivedTotal uint64
	RowsAddedTotal    uint64
	DedupsDuringMerge uint64
	SnapshotsCount    uint64

	TooSmallTimestampRows uint64
	TooBigTimestampRows   uint64
	InvalidRawMetricNames uint64

	TimeseriesRepopulated  uint64
	TimeseriesPreCreated   uint64
	NewTimeseriesCreated   uint64
	SlowRowInserts         uint64
	SlowPerDayIndexInserts uint64

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

	HourMetricIDCacheSize      uint64
	HourMetricIDCacheSizeBytes uint64

	NextDayMetricIDCacheSize      uint64
	NextDayMetricIDCacheSizeBytes uint64

	NextRetentionSeconds uint64

	MetricNamesUsageTrackerSize         uint64
	MetricNamesUsageTrackerSizeBytes    uint64
	MetricNamesUsageTrackerSizeMaxBytes uint64

	MetadataStorageItemsCurrent     uint64
	MetadataStorageCurrentSizeBytes uint64
	MetadataStorageMaxSizeBytes     uint64

	DeletedMetricsCount uint64

	TableMetrics TableMetrics
}

// Reset resets m.
func (m *Metrics) Reset() {
	*m = Metrics{}
}

// UpdateMetrics updates m with metrics from s.
func (s *Storage) UpdateMetrics(m *Metrics) {
	m.RowsReceivedTotal += s.rowsReceivedTotal.Load()
	m.RowsAddedTotal += s.rowsAddedTotal.Load()
	m.DedupsDuringMerge = dedupsDuringMerge.Load()
	m.SnapshotsCount += uint64(s.mustGetSnapshotsCount())

	m.TooSmallTimestampRows += s.tooSmallTimestampRows.Load()
	m.TooBigTimestampRows += s.tooBigTimestampRows.Load()
	m.InvalidRawMetricNames += s.invalidRawMetricNames.Load()

	m.TimeseriesRepopulated += s.timeseriesRepopulated.Load()
	m.TimeseriesPreCreated += s.timeseriesPreCreated.Load()
	m.NewTimeseriesCreated += s.newTimeseriesCreated.Load()
	m.SlowRowInserts += s.slowRowInserts.Load()
	m.SlowPerDayIndexInserts += s.slowPerDayIndexInserts.Load()

	if sl := s.hourlySeriesLimiter; sl != nil {
		m.HourlySeriesLimitRowsDropped += s.hourlySeriesLimitRowsDropped.Load()
		m.HourlySeriesLimitMaxSeries += uint64(sl.MaxItems())
		m.HourlySeriesLimitCurrentSeries += uint64(sl.CurrentItems())
	}

	if sl := s.dailySeriesLimiter; sl != nil {
		m.DailySeriesLimitRowsDropped += s.dailySeriesLimitRowsDropped.Load()
		m.DailySeriesLimitMaxSeries += uint64(sl.MaxItems())
		m.DailySeriesLimitCurrentSeries += uint64(sl.CurrentItems())
	}

	m.TimestampsBlocksMerged = timestampsBlocksMerged.Load()
	m.TimestampsBytesSaved = timestampsBytesSaved.Load()

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

	hmCurr := s.currHourMetricIDs.Load()
	hmPrev := s.prevHourMetricIDs.Load()
	hourMetricIDsLen := hmPrev.m.Len()
	if hmCurr.m.Len() > hourMetricIDsLen {
		hourMetricIDsLen = hmCurr.m.Len()
	}
	m.HourMetricIDCacheSize += uint64(hourMetricIDsLen)
	m.HourMetricIDCacheSizeBytes += hmCurr.m.SizeBytes()
	m.HourMetricIDCacheSizeBytes += hmPrev.m.SizeBytes()

	nextDayMetricIDs := &s.nextDayMetricIDs.Load().metricIDs
	m.NextDayMetricIDCacheSize += uint64(nextDayMetricIDs.Len())
	m.NextDayMetricIDCacheSizeBytes += nextDayMetricIDs.SizeBytes()

	var tm metricnamestats.TrackerMetrics
	s.metricsTracker.UpdateMetrics(&tm)
	m.MetricNamesUsageTrackerSizeBytes = tm.CurrentSizeBytes
	m.MetricNamesUsageTrackerSize = tm.CurrentItemsCount
	m.MetricNamesUsageTrackerSizeMaxBytes = tm.MaxSizeBytes

	var mr metricsmetadata.MetadataStorageMetrics
	s.metadataStorage.UpdateMetrics(&mr)
	m.MetadataStorageItemsCurrent = uint64(mr.ItemsCurrent)
	m.MetadataStorageCurrentSizeBytes = mr.CurrentSizeBytes
	m.MetadataStorageMaxSizeBytes = mr.MaxSizeBytes

	d := s.legacyNextRetentionSeconds()
	if d < 0 {
		d = 0
	}
	m.NextRetentionSeconds = uint64(d)

	s.tb.UpdateMetrics(&m.TableMetrics)
	s.legacyUpdateMetrics(m)

	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)
	var dmisCountLegacyPrev, dmisCountLegacyCurr uint64
	if idb := legacyIDBs.getIDBCurr(); idb != nil {
		dmisCountLegacyCurr = uint64(idb.getDeletedMetricIDs().Len())
		m.DeletedMetricsCount += dmisCountLegacyCurr
	}
	if idb := legacyIDBs.getIDBPrev(); idb != nil {
		dmisCountLegacyPrev = uint64(idb.getDeletedMetricIDs().Len())
		// Legacy prev idb also stores a copy of legacy curr idb dmis.
		dmisCountLegacyPrev -= dmisCountLegacyCurr
		m.DeletedMetricsCount += dmisCountLegacyPrev
	}
	for _, ptw := range ptws {
		cnt := uint64(ptw.pt.idb.getDeletedMetricIDs().Len())
		// Each pt idb stores a copy of legacy prev and curr idb dmis.
		cnt -= dmisCountLegacyPrev
		cnt -= dmisCountLegacyCurr
		m.DeletedMetricsCount += cnt
	}
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
	return s.isReadOnly.Load()
}

func (s *Storage) startFreeDiskSpaceWatcher() {
	f := func() {
		freeSpaceBytes := fs.MustGetFreeSpace(s.path)
		if freeSpaceBytes < freeDiskSpaceLimitBytes {
			// Switch the storage to readonly mode if there is no enough free space left at s.path
			//
			// Use Load in front of CompareAndSwap in order to avoid slow inter-CPU synchronization
			// when the storage is already in read-only mode.
			if !s.isReadOnly.Load() && s.isReadOnly.CompareAndSwap(false, true) {
				// log notification only on state change
				logger.Warnf("switching the storage at %s to read-only mode, since it has less than -storage.minFreeDiskSpaceBytes=%d of free space: %d bytes left",
					s.path, freeDiskSpaceLimitBytes, freeSpaceBytes)
			}
			return
		}
		// Use Load in front of CompareAndSwap in order to avoid slow inter-CPU synchronization
		// when the storage isn't in read-only mode.
		if s.isReadOnly.Load() && s.isReadOnly.CompareAndSwap(true, false) {
			s.notifyReadWriteMode()
			logger.Warnf("switching the storage at %s to read-write mode, since it has more than -storage.minFreeDiskSpaceBytes=%d of free space: %d bytes left",
				s.path, freeDiskSpaceLimitBytes, freeSpaceBytes)
		}
	}
	f()
	s.freeDiskSpaceWatcherWG.Add(1)
	go func() {
		defer s.freeDiskSpaceWatcherWG.Done()
		d := timeutil.AddJitterToDuration(time.Second)
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				f()
			}
		}
	}()
}

func (s *Storage) notifyReadWriteMode() {
	s.tb.NotifyReadWriteMode()
	s.legacyNotifyReadWriteMode()
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

func (s *Storage) currHourMetricIDsUpdater() {
	d := timeutil.AddJitterToDuration(time.Second * 10)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			hour := fasttime.UnixHour()
			s.updateCurrHourMetricIDs(hour)
			return
		case <-ticker.C:
			hour := fasttime.UnixHour()
			s.updateCurrHourMetricIDs(hour)
		}
	}
}

func (s *Storage) nextDayMetricIDsUpdater() {
	d := timeutil.AddJitterToDuration(time.Second * 11)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			date := fasttime.UnixDate()
			s.updateNextDayMetricIDs(date)
			return
		case <-ticker.C:
			date := fasttime.UnixDate()
			s.updateNextDayMetricIDs(date)
		}
	}
}

func (s *Storage) resetAndSaveTSIDCache() {
	// Reset cache and then store the reset cache on disk in order to prevent
	// from inconsistent behaviour after possible unclean shutdown.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347
	s.tsidCache.Reset()
	s.mustSaveCache(s.tsidCache, tsidCacheFilename)
}

// MustClose closes the storage.
//
// It is expected that the s is no longer used during the close.
func (s *Storage) MustClose() {
	close(s.stopCh)

	s.freeDiskSpaceWatcherWG.Wait()
	s.legacyRetentionWatcherWG.Wait()
	s.currHourMetricIDsUpdaterWG.Wait()
	s.nextDayMetricIDsUpdaterWG.Wait()

	s.tb.MustClose()

	s.legacyMustCloseIndexDBs()

	// Save caches.
	s.mustSaveCache(s.tsidCache, tsidCacheFilename)
	s.tsidCache.Stop()
	s.mustSaveCache(s.metricIDCache, metricIDCacheFilename)
	s.metricIDCache.Stop()
	s.mustSaveCache(s.metricNameCache, metricNameCacheFilename)
	s.metricNameCache.Stop()

	hmCurr := s.currHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmCurr, currHourMetricIDsFilename)
	hmPrev := s.prevHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmPrev, prevHourMetricIDsFilename)

	nextDayMetricIDs := s.nextDayMetricIDs.Load()
	s.mustSaveNextDayMetricIDs(nextDayMetricIDs)

	s.metricsTracker.MustClose()

	s.metadataStorage.MustClose()

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

func (s *Storage) mustLoadNextDayMetricIDs(date uint64) *nextDayMetricIDs {
	ptw := s.tb.MustGetPartition(int64(date+1) * msecPerDay)
	nextDayIDBID := ptw.pt.idb.id
	s.tb.PutPartition(ptw)
	e := &nextDayMetricIDs{
		// idbID field is used only to cache idb id for the next day to
		// avoid getting it every time a new batch of metric rows is
		// ingested. See updatePerDateData().
		idbID: nextDayIDBID,
		date:  date,
	}
	path := filepath.Join(s.cachePath, nextDayMetricIDsFilename)
	if !fs.IsPathExist(path) {
		return e
	}
	src, err := os.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
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
	e.metricIDs = *m
	return e
}

func (s *Storage) mustLoadHourMetricIDs(hour uint64, name string) *hourMetricIDs {
	hm := &hourMetricIDs{
		hour:  hour,
		idbID: s.tb.MustGetIndexDBIDByHour(hour),
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
	if len(tail) > 0 {
		logger.Infof("discarding %s because non-empty tail left; len(tail)=%d", path, len(tail))
		return hm
	}
	hm.m = m
	return hm
}

func (s *Storage) mustSaveNextDayMetricIDs(e *nextDayMetricIDs) {
	path := filepath.Join(s.cachePath, nextDayMetricIDsFilename)
	dst := make([]byte, 0, e.metricIDs.Len()*8+8)

	// Marshal header
	dst = encoding.MarshalUint64(dst, e.date)

	// Marshal metricIDs
	dst = marshalUint64Set(dst, &e.metricIDs)

	fs.MustWriteSync(path, dst)
}

func (s *Storage) mustSaveHourMetricIDs(hm *hourMetricIDs, name string) {
	path := filepath.Join(s.cachePath, name)
	dst := make([]byte, 0, hm.m.Len()*8+24)

	// Marshal header
	dst = encoding.MarshalUint64(dst, hm.hour)

	// Marshal hm.m
	dst = marshalUint64Set(dst, hm.m)

	fs.MustWriteSync(path, dst)
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
	c.MustSave(path)
}

// saveCacheLock prevents from data races when multiple concurrent goroutines save the same cache.
var saveCacheLock sync.Mutex

func (s *Storage) getMetricNameFromCache(dst []byte, metricID uint64) []byte {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	return s.metricNameCache.Get(dst, key[:])
}

func (s *Storage) putMetricNameToCache(metricID uint64, metricName []byte) {
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	s.metricNameCache.Set(key[:], metricName)
}

// searchAndMerge concurrently performs a search operation on all partition
// IndexDBs that overlap with the given time range and optionally legacy current
// and previous IndexDBs. The individual search results are then merged.
//
// The function creates a child query tracer for each search function call and
// closes it once the search() returns. Thus, implementations of search func
// must not close the query tracer that they receive.
func searchAndMerge[T any](qt *querytracer.Tracer, s *Storage, tr TimeRange, search func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (T, error), merge func([]T) T) (T, error) {
	qt = qt.NewChild("search indexDBs: timeRange=%v", &tr)
	defer qt.Done()

	var idbs []*indexDB

	ptws := s.tb.GetPartitions(tr)
	defer s.tb.PutPartitions(ptws)
	for _, ptw := range ptws {
		idbs = append(idbs, ptw.pt.idb)
	}

	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)
	idbs = legacyIDBs.appendTo(idbs)

	if len(idbs) == 0 {
		qt.Printf("no indexDBs found")
		var zeroValue T
		return zeroValue, nil
	}

	data := make([]T, len(idbs))
	errs := make([]error, len(idbs))

	if len(idbs) == 1 {
		// It is faster to process one indexDB without spawning goroutines.
		idb := idbs[0]
		searchTR := s.adjustTimeRange(tr, idb.tr)
		qtChild := qt.NewChild("search indexDB %s: timeRange=%v", idb.name, &searchTR)
		data[0], errs[0] = search(qtChild, idb, searchTR)
		qtChild.Done()
	} else {
		qtSearch := qt.NewChild("search %d indexDBs in parallel", len(idbs))
		var wg sync.WaitGroup
		for i, idb := range idbs {
			searchTR := s.adjustTimeRange(tr, idb.tr)
			qtChild := qtSearch.NewChild("search indexDB %s: timeRange=%v", idb.name, &searchTR)
			wg.Add(1)
			go func(qt *querytracer.Tracer, i int, idb *indexDB, tr TimeRange) {
				defer wg.Done()
				defer qt.Done()
				data[i], errs[i] = search(qt, idb, tr)
			}(qtChild, i, idb, searchTR)
		}
		wg.Wait()
		qtSearch.Done()
	}

	for _, err := range errs {
		if err != nil {
			var zeroValue T
			return zeroValue, err
		}
	}

	qtMerge := qt.NewChild("merge search results")
	result := merge(data)
	qtMerge.Done()

	return result, nil
}

// searchAndMergeUniq is a specific searchAndMerge operation that is common for
// most index searches. It expects each individual search to return a set of
// strings. The results of all individual searches are then unioned and the
// resulting set is converted into a slice. If result contains more than
// maxResults elements, it is truncated to maxResults.
//
// The final result is not sorted since it must be done by vmselect.
func searchAndMergeUniq(qt *querytracer.Tracer, s *Storage, tr TimeRange, search func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (map[string]struct{}, error), maxResults int) ([]string, error) {
	merge := func(data []map[string]struct{}) map[string]struct{} {
		if len(data) == 0 {
			return nil
		}

		totalLen := 0
		for _, d := range data {
			totalLen += len(d)
		}

		if totalLen > maxResults {
			totalLen = maxResults
		}

		all := make(map[string]struct{}, totalLen)
		for _, d := range data {
			for v := range d {
				if len(all) >= maxResults {
					return all
				}
				all[v] = struct{}{}
			}
		}
		return all
	}

	m, err := searchAndMerge(qt, s, tr, search, merge)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	return res, nil
}

// SearchTSIDs searches the TSIDs that correspond to filters within the given
// time range.
//
// The returned TSIDs are sorted.
//
// The method will fail if the number of found TSIDs exceeds maxMetrics or the
// search has not completed within the specified deadline.
func (s *Storage) SearchTSIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]TSID, error) {
	qt = qt.NewChild("search TSIDs: filters=%s, timeRange=%s, maxMetrics=%d", tfss, &tr, maxMetrics)
	defer qt.Done()

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]TSID, error) {
		return idb.SearchTSIDs(qt, tfss, tr, maxMetrics, deadline)
	}

	merge := func(data [][]TSID) []TSID {
		tsidss := make([][]TSID, 0, len(data))
		for _, d := range data {
			if len(d) > 0 {
				tsidss = append(tsidss, d)
			}
		}
		if len(tsidss) == 0 {
			return nil
		}
		if len(tsidss) == 1 {
			return tsidss[0]
		}
		return mergeSortedTSIDs(tsidss)
	}

	tsids, err := searchAndMerge(qt, s, tr, search, merge)
	if err != nil {
		return nil, err
	}

	return tsids, nil
}

// SearchMetricNames returns marshaled metric names matching the given tfss on
// the given tr.
//
// The marshaled metric names must be unmarshaled via
// MetricName.UnmarshalString().
func (s *Storage) SearchMetricNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search metric names: filters=%s, timeRange=%s, maxMetrics: %d", tfss, &tr, maxMetrics)
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
	}

	merge := func(data [][]string) []string {
		var n int
		for _, d := range data {
			n += len(d)
		}
		seen := make(map[string]struct{}, n)
		all := make([]string, 0, n)
		for _, d := range data {
			for _, v := range d {
				if _, ok := seen[v]; !ok {
					all = append(all, v)
					seen[v] = struct{}{}
				}
			}
		}
		return all
	}
	res, err := searchAndMerge(qt, s, tr, search, merge)
	if err != nil {
		return nil, err
	}
	qt.Donef("found %d metric names", len(res))
	return res, nil
}

// ErrDeadlineExceeded is returned when the request times out.
var ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")

// DeleteSeries marks as deleted all series matching the given tfss and
// resets caches where the corresponding TSIDs and MetricIDs may be stored if
// needed.
//
// If the number of the series exceeds maxMetrics, no series will be deleted and
// an error will be returned. Otherwise, the function returns the number of
// metrics deleted.
//
// If legacy indexDBs are present, the method will also delete the metricIDs
// from them.
func (s *Storage) DeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (int, error) {
	qt = qt.NewChild("delete series: filters=%s, maxMetrics=%d", tfss, maxMetrics)
	defer qt.Done()

	if len(tfss) == 0 {
		return 0, nil
	}

	// Not deleting in parallel because the deletion operation is rare.

	all := &uint64set.Set{}
	legacyDMIs, err := s.legacyDeleteSeries(qt, tfss, maxMetrics)
	if err != nil {
		return 0, err
	}
	all.UnionMayOwn(legacyDMIs)

	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)

	for _, ptw := range ptws {
		idb := ptw.pt.idb
		qt.Printf("start deleting from %s partition indexDB", idb.name)
		if legacyDMIs.Len() > 0 {
			idb.updateDeletedMetricIDs(legacyDMIs)
		}
		dmis, err := idb.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return 0, err
		}
		n := dmis.Len()
		all.UnionMayOwn(dmis)
		qt.Printf("deleted %d metricIDs from %s partition indexDB", n, idb.name)
	}

	n := all.Len()
	qt.Donef("deleted %d unique metricIDs", n)
	return n, nil
}

// SearchLabelNames searches for label names matching the given tfss on tr.
func (s *Storage) SearchLabelNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label names: filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", tfss, &tr, maxLabelNames, maxMetrics)
	defer qt.Done()

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (map[string]struct{}, error) {
		return idb.SearchLabelNames(qt, tfss, tr, maxLabelNames, maxMetrics, deadline)
	}
	res, err := searchAndMergeUniq(qt, s, tr, search, maxLabelNames)
	if err != nil {
		return nil, err
	}
	qt.Printf("found %d label names", len(res))
	return res, nil
}

// SearchLabelValues searches for label values for the given labelName, filters and tr.
func (s *Storage) SearchLabelValues(qt *querytracer.Tracer, labelName string, tfss []*TagFilters, tr TimeRange, maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label values: labelName=%q, filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", labelName, tfss, &tr, maxLabelValues, maxMetrics)
	defer qt.Done()

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (map[string]struct{}, error) {
		return idb.SearchLabelValues(qt, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
	}
	res, err := searchAndMergeUniq(qt, s, tr, search, maxLabelValues)
	if err != nil {
		return nil, err
	}
	qt.Printf("found %d label values", len(res))
	return res, err
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given
// tagKey and tagValuePrefix on the given tr.
//
// This allows implementing
// https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find
// or similar APIs.
//
// If more than maxTagValueSuffixes suffixes is found, then only the first
// maxTagValueSuffixes suffixes is returned.
func (s *Storage) SearchTagValueSuffixes(qt *querytracer.Tracer, tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int, deadline uint64) ([]string, error) {
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (map[string]struct{}, error) {
		return idb.SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes, deadline)
	}
	res, err := searchAndMergeUniq(qt, s, tr, search, maxTagValueSuffixes)
	if err != nil {
		return nil, err
	}
	qt.Printf("found %d tag value suffixes", len(res))
	return res, err
}

// SearchGraphitePaths returns all the matching paths for the given graphite
// query on the given tr.
func (s *Storage) SearchGraphitePaths(qt *querytracer.Tracer, tr TimeRange, query []byte, maxPaths int, deadline uint64) ([]string, error) {
	query = replaceAlternateRegexpsWithGraphiteWildcards(query)
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (map[string]struct{}, error) {
		return idb.SearchGraphitePaths(qt, tr, nil, query, maxPaths, deadline)
	}

	res, err := searchAndMergeUniq(qt, s, tr, search, maxPaths)
	if err != nil {
		return nil, err
	}
	qt.Printf("found %d graphite paths", len(res))
	return res, err
}

// replaceAlternateRegexpsWithGraphiteWildcards replaces (foo|..|bar) with {foo,...,bar} in b and returns the new value.
func replaceAlternateRegexpsWithGraphiteWildcards(b []byte) []byte {
	var dst []byte
	for {
		n := bytes.IndexByte(b, '(')
		if n < 0 {
			if len(dst) == 0 {
				// Fast path - b doesn't contain the opening brace.
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

// GetSeriesCount returns the total number of time series registered in all
// indexDBs. It can return inflated value if the same time series are stored in
// more than one indexDB.
//
// It also includes the deleted series.
func (s *Storage) GetSeriesCount(deadline uint64) (uint64, error) {
	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: time.Now().UnixMilli(),
	}
	search := func(_ *querytracer.Tracer, idb *indexDB, _ TimeRange) (uint64, error) {
		return idb.GetSeriesCount(deadline)
	}
	merge := func(data []uint64) uint64 {
		var total uint64
		for _, cnt := range data {
			total += cnt
		}
		return total
	}
	return searchAndMerge(nil, s, tr, search, merge)
}

// GetTSDBStatus returns TSDB status data for /api/v1/status/tsdb
//
// The method does not provide status for legacy IDBs because merging partition
// indexDB and legacy indexDB statuses is non-trivial and not many users use
// this status for historical data.
func (s *Storage) GetTSDBStatus(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	timestamp := int64(date) * msecPerDay
	ptw := s.tb.GetPartition(timestamp)
	if ptw == nil {
		return &TSDBStatus{}, nil
	}
	defer s.tb.PutPartition(ptw)

	if s.disablePerDayIndex {
		date = globalIndexDate
	}

	res, err := ptw.pt.idb.GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if s.metricsTracker != nil && len(res.SeriesCountByMetricName) > 0 {
		// for performance reason always check if metricsTracker is configured
		names := make([]string, len(res.SeriesCountByMetricName))
		for idx, mns := range res.SeriesCountByMetricName {
			names[idx] = mns.Name
		}
		res.SeriesQueryStatsByMetricName = s.metricsTracker.GetStatRecordsForNames(0, 0, names)
	}
	return res, nil
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
	metricNameRaw, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal MetricName")
	}
	tail := src[nSize:]
	mr.MetricNameRaw = metricNameRaw

	if len(tail) < 8 {
		return tail, fmt.Errorf("cannot unmarshal Timestamp: want %d bytes; have %d bytes", 8, len(tail))
	}
	timestamp := encoding.UnmarshalUint64(tail)
	tail = tail[8:]
	mr.Timestamp = int64(timestamp)

	if len(tail) < 8 {
		return tail, fmt.Errorf("cannot unmarshal Value: want %d bytes; have %d bytes", 8, len(tail))
	}
	value := encoding.UnmarshalUint64(tail)
	tail = tail[8:]
	mr.Value = math.Float64frombits(value)

	return tail, nil
}

// ForceMergePartitions force-merges partitions in s with names starting from the given partitionNamePrefix.
//
// Partitions are merged sequentially in order to reduce load on the system.
func (s *Storage) ForceMergePartitions(partitionNamePrefix string) error {
	return s.tb.ForceMergePartitions(partitionNamePrefix)
}

// AddRows adds the given mrs to s.
//
// The caller should limit the number of concurrent AddRows calls to the number
// of available CPU cores in order to limit memory usage.
func (s *Storage) AddRows(mrs []MetricRow, precisionBits uint8) {
	if len(mrs) == 0 {
		return
	}

	// Add rows to the storage in blocks with limited size in order to reduce memory usage.
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
		rowsAdded := s.add(ic.rrs, ic.tmpMrs, mrsBlock, precisionBits)

		// If the number of received rows is greater than the number of added
		// rows, then some rows have failed to add. Check logs for the first
		// error.
		s.rowsAddedTotal.Add(uint64(rowsAdded))
		s.rowsReceivedTotal.Add(uint64(len(mrsBlock)))
	}
	putMetricRowsInsertCtx(ic)
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

// It has been found empirically, that once the time range is bigger than 40
// days searching using per-day index becomes slower than using global index.
//
// TODO(rtm0): Extract into a flag?
const maxDaysForPerDaySearch = 40

// adjustTimeRange decides whether to use the time range as is or use
// globalIndexTimeRange based on the time range length and -disablePerDayIndex
// flag.
func (s *Storage) adjustTimeRange(searchTR, idbTR TimeRange) TimeRange {
	// If the per day index is disabled, unconditionally search global index.
	if s.disablePerDayIndex {
		return globalIndexTimeRange
	}

	tr := idbTR
	if idbTR.contains(searchTR.MinTimestamp) {
		tr.MinTimestamp = searchTR.MinTimestamp
	}
	if idbTR.contains(searchTR.MaxTimestamp) {
		tr.MaxTimestamp = searchTR.MaxTimestamp
	}

	// For legacy IndexDBs only, partition indexDBs can't span more than a
	// month.
	minDate, maxDate := tr.DateRange()
	if maxDate-minDate > maxDaysForPerDaySearch {
		return globalIndexTimeRange
	}

	// For partition IndexDBs only. If the final time range is still the same as
	// the idb time range, then return globalIndexTimeRange to indicate that we
	// want to search the global index since the entire index db needs to be
	// searched anyway.
	if tr == idbTR {
		return globalIndexTimeRange
	}

	return tr
}

// RegisterMetricNames registers all the metric names from mrs in the indexdb, so they can be queried later.
//
// The the MetricRow.Timestamp is used for registering the metric name at the given day according to the timestamp.
// Th MetricRow.Value field is ignored.
func (s *Storage) RegisterMetricNames(qt *querytracer.Tracer, mrs []MetricRow) {
	qt = qt.NewChild("registering %d series", len(mrs))
	defer qt.Done()
	var metricNameBuf []byte
	var lTSID legacyTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	var newSeriesCount uint64
	var seriesRepopulated uint64

	var ptw *partitionWrapper
	var idb *indexDB
	var is *indexSearch
	var deletedMetricIDs *uint64set.Set

	var firstWarn error
	for i := range mrs {
		mr := &mrs[i]
		if !s.registerSeriesCardinality(mr.MetricNameRaw) {
			// Skip row, since it exceeds cardinality limit
			continue
		}

		date := uint64(mr.Timestamp) / msecPerDay

		if ptw == nil || !ptw.pt.HasTimestamp(mr.Timestamp) {
			if ptw != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutPartition(ptw)
			}
			ptw = s.tb.MustGetPartition(mr.Timestamp)
			idb = ptw.pt.idb
			is = idb.getIndexSearch(noDeadline)
			deletedMetricIDs = idb.getDeletedMetricIDs()
		}

		if s.getTSIDFromCache(&lTSID, mr.MetricNameRaw) && !deletedMetricIDs.Has(lTSID.TSID.MetricID) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been
			// found in cache and isn't deleted. If the TSID is deleted, we
			// re-register time series. Eventually, the deleted TSID will be
			// removed from the cache.

			if !is.hasMetricID(lTSID.TSID.MetricID) {
				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()
				idb.createGlobalIndexes(&lTSID.TSID, mn)
				idb.createPerDayIndexes(date, &lTSID.TSID, mn)
				seriesRepopulated++
			} else if !is.hasDateMetricID(date, lTSID.TSID.MetricID) {
				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()
				idb.createPerDayIndexes(date, &lTSID.TSID, mn)
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
				firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		if is.getTSIDByMetricName(&lTSID.TSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.
			s.storeTSIDToCache(&lTSID, mr.MetricNameRaw)
			continue
		}

		// Slowest path - there is no TSID in indexdb for the given mr.MetricNameRaw. Create it.
		generateTSID(&lTSID.TSID, mn)
		createAllIndexesForMetricName(idb, mn, &lTSID.TSID, date)
		s.storeTSIDToCache(&lTSID, mr.MetricNameRaw)
		newSeriesCount++
	}
	if ptw != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		idb = nil
		s.tb.PutPartition(ptw)
	}

	s.newTimeseriesCreated.Add(newSeriesCount)
	s.timeseriesRepopulated.Add(seriesRepopulated)

	// There is no need in pre-filling idbNext here, since RegisterMetricNames() is rarely called.
	// So it is OK to register metric names in blocking manner after indexdb rotation.

	if firstWarn != nil {
		logger.Warnf("cannot create some metrics: %s", firstWarn)
	}
}

func (s *Storage) add(rows []rawRow, dstMrs []*MetricRow, mrs []MetricRow, precisionBits uint8) int {
	logNewSeries := s.logNewSeries.Load() || s.logNewSeriesUntil.Load() >= fasttime.UnixTimestamp()
	hmPrev := s.prevHourMetricIDs.Load()
	hmCurr := s.currHourMetricIDs.Load()
	var pendingHourEntries []uint64
	addToPendingHourEntries := func(hour, metricID uint64) {
		if hour == hmCurr.hour && !hmCurr.m.Has(metricID) {
			pendingHourEntries = append(pendingHourEntries, metricID)
		}
	}

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

	var lTSID legacyTSID
	var ptw *partitionWrapper
	var idb *indexDB
	var is *indexSearch
	var deletedMetricIDs *uint64set.Set

	// Log only the first error, since it has no sense in logging all errors.
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
			s.tooSmallTimestampRows.Add(1)
			continue
		}
		if mr.Timestamp > maxTimestamp {
			// Skip rows with too big timestamps significantly exceeding the current time.
			if firstWarn == nil {
				metricName := getUserReadableMetricName(mr.MetricNameRaw)
				firstWarn = fmt.Errorf("cannot insert row with too big timestamp %d exceeding the current time; maximum allowed timestamp is %d; metricName: %s",
					mr.Timestamp, maxTimestamp, metricName)
			}
			s.tooBigTimestampRows.Add(1)
			continue
		}
		dstMrs[j] = mr
		r := &rows[j]
		j++
		r.Timestamp = mr.Timestamp
		r.Value = mr.Value
		r.PrecisionBits = precisionBits
		date := uint64(r.Timestamp) / msecPerDay
		hour := uint64(r.Timestamp) / msecPerHour

		if ptw == nil || !ptw.pt.HasTimestamp(r.Timestamp) {
			if ptw != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutPartition(ptw)
			}
			ptw = s.tb.MustGetPartition(r.Timestamp)
			idb = ptw.pt.idb
			is = idb.getIndexSearch(noDeadline)
			deletedMetricIDs = idb.getDeletedMetricIDs()
		}

		// Search for TSID for the given mr.MetricNameRaw and store it at r.TSID.
		if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
			// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
			// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.

			if !is.hasMetricID(prevTSID.MetricID) {
				// The found TSID is not present in the current indexDB (one
				// that corresponds to the timestamp of the current sample).
				// Create it in the current indexdb.

				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					j--
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()
				idb.createGlobalIndexes(&prevTSID, mn)
			}
			r.TSID = prevTSID
			continue
		}
		if !s.registerSeriesCardinality(mr.MetricNameRaw) {
			// Skip row, since it exceeds cardinality limit
			j--
			continue
		}

		// tsidCache may contain TSIDs that were deleted from some indexDBs but
		// are still in use in other indexDBs. Thus, also check if a given TSID
		// was not deleted deom the current indexDB.
		if s.getTSIDFromCache(&lTSID, mr.MetricNameRaw) && !deletedMetricIDs.Has(lTSID.TSID.MetricID) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been found in cache and isn't deleted.

			r.TSID = lTSID.TSID
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			if !is.hasMetricID(lTSID.TSID.MetricID) {
				// The found TSID is from the another partition indexdb. Create it in the current partition indexdb.
				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					j--
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()

				// Only create an entry to the global index.
				// Do not add to tsidCache because it is already there.
				// Do not create an entry in per-day index and do not add to
				// dateMetricIDCache because this will be done in updatePerDateData().
				idb.createGlobalIndexes(&lTSID.TSID, mn)
				seriesRepopulated++
				slowInsertsCount++
			}
			addToPendingHourEntries(hour, lTSID.TSID.MetricID)
			continue
		}

		// Slow path - the TSID for the given mr.MetricNameRaw is missing in the cache.
		slowInsertsCount++

		// Construct canonical metric name - it is used below.
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
			}
			j--
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		// register metric name on tsid cache miss
		// it allows to track metric names since last tsid cache reset
		// and skip index scan to fill metrics tracker
		s.metricsTracker.RegisterIngestRequest(0, 0, mn.MetricGroup)

		// Search for TSID for the given mr.MetricNameRaw in the indexdb.
		if is.getTSIDByMetricName(&lTSID.TSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			s.storeTSIDToCache(&lTSID, mr.MetricNameRaw)

			r.TSID = lTSID.TSID
			prevTSID = lTSID.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			addToPendingHourEntries(hour, lTSID.TSID.MetricID)
			continue
		}

		// Slowest path - the TSID for the given mr.MetricNameRaw isn't found in indexdb. Create it.
		generateTSID(&lTSID.TSID, mn)
		createAllIndexesForMetricName(idb, mn, &lTSID.TSID, date)
		s.storeTSIDToCache(&lTSID, mr.MetricNameRaw)
		newSeriesCount++

		r.TSID = lTSID.TSID
		prevTSID = r.TSID
		prevMetricNameRaw = mr.MetricNameRaw

		addToPendingHourEntries(hour, lTSID.TSID.MetricID)

		if logNewSeries {
			logger.Infof("new series created: %s", mn.String())
		}
	}
	if idb != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		idb = nil
		s.tb.PutPartition(ptw)
	}

	s.slowRowInserts.Add(slowInsertsCount)
	s.newTimeseriesCreated.Add(newSeriesCount)
	s.timeseriesRepopulated.Add(seriesRepopulated)

	dstMrs = dstMrs[:j]
	rows = rows[:j]

	if len(pendingHourEntries) > 0 {
		s.pendingHourEntriesLock.Lock()
		s.pendingHourEntries.AddMulti(pendingHourEntries)
		s.pendingHourEntriesLock.Unlock()
	}

	if err := s.prefillNextIndexDB(rows, dstMrs); err != nil {
		if firstWarn == nil {
			firstWarn = fmt.Errorf("cannot prefill next indexdb: %w", err)
		}
	}

	if err := s.updatePerDateData(rows, dstMrs, hmPrev, hmCurr); err != nil {
		if firstWarn == nil {
			firstWarn = fmt.Errorf("cannot not update per-day index: %w", err)
		}
	}

	if firstWarn != nil {
		storageAddRowsLogger.Warnf("warn occurred during rows addition: %s", firstWarn)
	}

	s.tb.MustAddRows(rows)

	return len(rows)
}

var storageAddRowsLogger = logger.WithThrottler("storageAddRows", 5*time.Second)

// SetLogNewSeriesUntil sets the timestamp until which new series will be logged.
func (s *Storage) SetLogNewSeriesUntil(t uint64) {
	s.logNewSeriesUntil.Store(t)
}

func createAllIndexesForMetricName(db *indexDB, mn *MetricName, tsid *TSID, date uint64) {
	db.createGlobalIndexes(tsid, mn)
	db.createPerDayIndexes(date, tsid, mn)
}

func (s *Storage) registerSeriesCardinality(metricNameRaw []byte) bool {
	if s.hourlySeriesLimiter == nil && s.dailySeriesLimiter == nil {
		return true
	}

	metricID := xxhash.Sum64(metricNameRaw)
	if sl := s.hourlySeriesLimiter; sl != nil && !sl.Add(metricID) {
		s.hourlySeriesLimitRowsDropped.Add(1)
		logSkippedSeries(metricNameRaw, "-storage.maxHourlySeries", sl.MaxItems())
		return false
	}
	if sl := s.dailySeriesLimiter; sl != nil && !sl.Add(metricID) {
		s.dailySeriesLimitRowsDropped.Add(1)
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

// prefillNextIndexDB gradually pre-populates the indexDB of the next partition
// during the last idbPrefillStartSeconds seconds before that partition becomes
// the current one. This is needed in order to reduce spikes in CPU and disk IO
// usage just after the switch.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401.
func (s *Storage) prefillNextIndexDB(rows []rawRow, mrs []*MetricRow) error {
	now := time.Unix(int64(fasttime.UnixTimestamp()), 0).UTC()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	d := nextMonth.Sub(now).Seconds()
	if d >= float64(s.idbPrefillStartSeconds) {
		// Fast path: nothing to pre-fill because it is too early.
		// The pre-fill is started during the last hour before the indexdb rotation.
		return nil
	}

	// Slower path: less than nextPrefillStartSeconds left for the next indexdb rotation.
	// Pre-populate idbNext with the increasing probability until the rotation.
	// The probability increases from 0% to 100% proportionally to d=[nextPrefillStartSeconds .. 0].
	pMin := d / float64(s.idbPrefillStartSeconds)

	ptwNext := s.tb.MustGetPartition(nextMonth.UnixMilli())
	idbNext := ptwNext.pt.idb
	defer s.tb.PutPartition(ptwNext)
	isNext := idbNext.getIndexSearch(noDeadline)
	defer idbNext.putIndexSearch(isNext)

	var firstError error
	var lTSID legacyTSID
	mn := GetMetricName()
	defer PutMetricName(mn)

	// Only prefill index for samples whose timestamp falls within the last
	// idbPrefillStartSeconds of the current month.
	tr := TimeRange{
		MinTimestamp: nextMonth.UnixMilli() - s.idbPrefillStartSeconds*1000,
		MaxTimestamp: nextMonth.UnixMilli() - 1,
	}
	// Use the first date of the next month for prefilling the index.
	date := uint64(nextMonth.UnixMilli()) / msecPerDay

	timeseriesPreCreated := uint64(0)
	for i := range rows {
		r := &rows[i]

		if !tr.contains(r.Timestamp) {
			continue
		}

		p := float64(uint32(fastHashUint64(r.TSID.MetricID))) / (1 << 32)
		if p < pMin {
			// Fast path: it is too early to pre-fill indexes for the given MetricID.
			continue
		}

		// Check whether the given metricID is already present in idbNext.
		metricID := r.TSID.MetricID
		if isNext.hasMetricID(metricID) {
			continue
		}

		// Slow path: pre-fill indexes in idbNext.
		metricNameRaw := mrs[i].MetricNameRaw
		if err := mn.UnmarshalRaw(metricNameRaw); err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", metricNameRaw, err)
			}
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()

		createAllIndexesForMetricName(idbNext, mn, &r.TSID, date)
		lTSID.TSID = r.TSID
		timeseriesPreCreated++

		// Do not put TSID to tsidCache since this has already been done in
		// add().
	}
	s.timeseriesPreCreated.Add(timeseriesPreCreated)

	return firstError
}

func (s *Storage) updatePerDateData(rows []rawRow, mrs []*MetricRow, hmPrev, hmCurr *hourMetricIDs) error {
	if s.disablePerDayIndex {
		return nil
	}

	var date uint64
	var hour uint64
	var prevTimestamp int64
	var (
		// These vars are used for speeding up bulk imports when multiple adjacent rows
		// contain the same (metricID, date) pairs.
		prevDate     uint64
		prevMetricID uint64
	)
	var ptw *partitionWrapper
	var idb *indexDB

	hmPrevDate := hmPrev.hour / 24
	hmCurrDate := hmCurr.hour / 24
	nextDayMetricIDsCache := s.nextDayMetricIDs.Load()
	nextDayIDBID := nextDayMetricIDsCache.idbID
	nextDayMetricIDs := &nextDayMetricIDsCache.metricIDs
	ts := fasttime.UnixTimestamp()
	// Start pre-populating the next per-day inverted index during the last hour of the current day.
	// pMin linearly increases from 0 to 1 during the last hour of the day.
	pMin := (float64(ts%(3600*24)) / 3600) - 23
	currentHour := ts / 3600
	type pendingDateMetricID struct {
		date uint64
		tsid *TSID
		mr   *MetricRow
	}
	var pendingDateMetricIDs []pendingDateMetricID
	var pendingNextDayMetricIDs []uint64
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

		if hmCurr.idbID == nextDayIDBID && pMin > 0 && hour == currentHour {
			// Gradually pre-populate per-day inverted index for the next day during the last hour of the current day.
			// This should reduce CPU usage spike and slowdown at the beginning of the next day
			// when entries for all the active time series must be added to the index.
			// This should address https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 .
			//
			// Do this only if the next day is in the same partition indexDB.
			// If next day is in another partition indexDB, the prefill is
			// handled separately in prefillNextIndexDB.
			// TODO(@rtm0): See if prefillNextIndexDB() logic can be moved here.
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

		if date == hmCurrDate && hmCurr.m.Has(metricID) {
			// Fast path: the metricID is in the current hour cache.
			// This means the metricID has been already added to per-day inverted index.
			continue
		}

		if date == hmPrevDate && hmPrev.m.Has(metricID) {
			// Fast path: the metricID is already registered for its day on the previous hour.
			continue
		}

		// Slower path: check the dateMetricIDCache if the (date, metricID) pair
		// is already present in indexDB.
		if ptw == nil || !ptw.pt.HasTimestamp(r.Timestamp) {
			if ptw != nil {
				s.tb.PutPartition(ptw)
			}
			ptw = s.tb.MustGetPartition(r.Timestamp)
			idb = ptw.pt.idb
		}
		// TODO(@rtm0): indexDB.dateMetricIDCache should not be used directly
		// since its purpose is to optimize is.hasDateMetricID(). See if this
		// function could be changed so that it does not rely on this cache.
		if idb.dateMetricIDCache.Has(date, metricID) {
			continue
		}

		// Slow path: store the (date, metricID) entry in the indexDB.
		pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
			date: date,
			tsid: &r.TSID,
			mr:   mrs[i],
		})
	}
	if ptw != nil {
		s.tb.PutPartition(ptw)
		ptw = nil
		idb = nil
	}

	if len(pendingNextDayMetricIDs) > 0 {
		s.pendingNextDayMetricIDsLock.Lock()
		s.pendingNextDayMetricIDs.AddMulti(pendingNextDayMetricIDs)
		s.pendingNextDayMetricIDsLock.Unlock()
	}
	if len(pendingDateMetricIDs) == 0 {
		// Fast path - there are no new (date, metricID) entries.
		return nil
	}

	// Slow path - add new (date, metricID) entries to indexDB.

	s.slowPerDayIndexInserts.Add(uint64(len(pendingDateMetricIDs)))
	// Sort pendingDateMetricIDs by (date, metricID) in order to speed up `is` search in the loop below.
	sort.Slice(pendingDateMetricIDs, func(i, j int) bool {
		a := pendingDateMetricIDs[i]
		b := pendingDateMetricIDs[j]
		if a.date != b.date {
			return a.date < b.date
		}
		return a.tsid.MetricID < b.tsid.MetricID
	})

	var firstError error
	mn := GetMetricName()
	var is *indexSearch
	for _, dmid := range pendingDateMetricIDs {
		date := dmid.date
		metricID := dmid.tsid.MetricID
		timestamp := int64(date) * msecPerDay
		if ptw == nil || !ptw.pt.HasTimestamp(timestamp) {
			if ptw != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutPartition(ptw)
			}
			ptw = s.tb.MustGetPartition(timestamp)
			idb = ptw.pt.idb
			is = idb.getIndexSearch(noDeadline)
		}

		if !is.hasDateMetricID(date, metricID) {
			// The (date, metricID) entry is missing in the indexDB. Add it there together with per-day index.
			// It is OK if the (date, metricID) entry is added multiple times to indexdb
			// by concurrent goroutines.
			if err := mn.UnmarshalRaw(dmid.mr.MetricNameRaw); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", dmid.mr.MetricNameRaw, err)
				}
				s.invalidRawMetricNames.Add(1)
				continue
			}
			mn.sortTags()
			idb.createPerDayIndexes(date, dmid.tsid, mn)
		}
	}
	if ptw != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		idb = nil
		s.tb.PutPartition(ptw)
	}

	PutMetricName(mn)
	return firstError
}

func fastHashUint64(x uint64) uint64 {
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	return x * 2685821657736338717
}

// nextDayMetricIDs is a cache that holds the metricIDs for the next day.
// The cache is used for improving the performance of data ingestion during
// the last hour of the day when the per-day index is prefilled with the next
// day entries (see updatePerDayData()).
type nextDayMetricIDs struct {
	idbID     uint64
	date      uint64
	metricIDs uint64set.Set
}

func (s *Storage) updateNextDayMetricIDs(date uint64) {
	ptw := s.tb.MustGetPartition(int64(date+1) * msecPerDay)
	nextDayIDBID := ptw.pt.idb.id
	s.tb.PutPartition(ptw)
	e := s.nextDayMetricIDs.Load()
	s.pendingNextDayMetricIDsLock.Lock()
	pendingMetricIDs := s.pendingNextDayMetricIDs
	s.pendingNextDayMetricIDs = &uint64set.Set{}
	s.pendingNextDayMetricIDsLock.Unlock()
	// Not comparing indexDB IDs because different idb ids imply different date.
	if pendingMetricIDs.Len() == 0 && e.date == date {
		// Fast path: nothing to update.
		return
	}

	// Slow path: union pendingMetricIDs with e.metricIDs
	//
	// In partition index, two adjacent dates may correspond to two different
	// indexDBs. For example, 2025-01-31 corresponds to 2025_01 partition
	// indexDB, while 2025-02-01 corresponds to 2025_02 partition indexDB.
	// In order to prefill the next day index correctly, the nextDayMetricIDs
	// cache must contain the entries for one indexDB only and if the nextDay
	// happens to be in a different indexDB the cache needs to be reset. But
	// since different indexDBs imply different dates, it is enough to compare
	// just dates.
	if e.date == date {
		pendingMetricIDs.Union(&e.metricIDs)
	} else {
		// Do not add pendingMetricIDs from the previous day to the current day,
		// since this may result in missing registration of the metricIDs in the per-day inverted index.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309
		pendingMetricIDs = &uint64set.Set{}
	}
	eNew := &nextDayMetricIDs{
		// idbID field is used only to cache idb id for the next day to avoid
		// getting it every time a new batch of metric rows is ingested (see
		// updatePerDateData()).
		idbID:     nextDayIDBID,
		date:      date,
		metricIDs: *pendingMetricIDs,
	}
	s.nextDayMetricIDs.Store(eNew)
}

func (s *Storage) updateCurrHourMetricIDs(hour uint64) {
	hm := s.currHourMetricIDs.Load()
	s.pendingHourEntriesLock.Lock()
	newMetricIDs := s.pendingHourEntries
	s.pendingHourEntries = &uint64set.Set{}
	s.pendingHourEntriesLock.Unlock()

	if newMetricIDs.Len() == 0 && hm.hour == hour {
		// Fast path: nothing to update.
		return
	}

	// Slow path: hm.m must be updated with non-empty s.pendingHourEntries.
	idbID := hm.idbID
	var m *uint64set.Set
	if hm.hour == hour {
		m = hm.m.Clone()
		m.Union(newMetricIDs)
	} else {
		idbID = s.tb.MustGetIndexDBIDByHour(hour)
		m = newMetricIDs
		if hour%24 == 0 {
			// Do not add pending metricIDs from the previous hour to the current hour on the next day,
			// since this may result in missing registration of the metricIDs in the per-day inverted index.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309
			m = &uint64set.Set{}
		}
	}
	hmNew := &hourMetricIDs{
		m:     m,
		hour:  hour,
		idbID: idbID,
	}
	s.currHourMetricIDs.Store(hmNew)
	if hm.hour != hour {
		s.prevHourMetricIDs.Store(hm)
	}
}

type hourMetricIDs struct {
	m     *uint64set.Set
	hour  uint64
	idbID uint64
}

type legacyTSID struct {
	TSID TSID

	// This field used to store the stores the indexdb generation value to
	// identify to which indexdb belongs this TSID. After switching to the
	// partition indexDB this field is not needed anymore, however we still
	// need to preserve it in order to adhere tsidCache data format.
	_ uint64
}

func (s *Storage) getTSIDFromCache(dst *legacyTSID, metricName []byte) bool {
	buf := (*[unsafe.Sizeof(*dst)]byte)(unsafe.Pointer(dst))[:]
	buf = s.tsidCache.Get(buf[:0], metricName)
	return uintptr(len(buf)) == unsafe.Sizeof(*dst)
}

func (s *Storage) storeTSIDToCache(tsid *legacyTSID, metricName []byte) {
	buf := (*[unsafe.Sizeof(*tsid)]byte)(unsafe.Pointer(tsid))[:]
	s.tsidCache.Set(metricName, buf)
}

// wasMetricIDMissingBefore checks if passed metricID was already registered as missing before.
// It returns true if metricID was registered as missing for more than 60s.
//
// This function is called when storage can't find TSID for corresponding metricID.
// There are the following expected cases when this may happen:
//  1. The corresponding metricID -> metricName/tsid entry isn't visible for search yet.
//     The solution is to wait for some time and try the search again.
//     It is OK if newly registered time series isn't visible for search during some time.
//     This should resolve https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5959
//  2. The metricID -> metricName/tsid entry doesn't exist in the indexdb.
//     This is possible after unclean shutdown or after restoring of indexdb from a snapshot.
//     In this case the metricID must be deleted, so new metricID is registered
//     again when new sample for the given metric is ingested next time.
func (s *Storage) wasMetricIDMissingBefore(metricID uint64) bool {
	ct := fasttime.UnixTimestamp()
	s.missingMetricIDsLock.Lock()
	defer s.missingMetricIDsLock.Unlock()

	if ct > s.missingMetricIDsResetDeadline {
		s.missingMetricIDs = nil
		s.missingMetricIDsResetDeadline = ct + 2*60
	}
	deleteDeadline, ok := s.missingMetricIDs[metricID]
	if !ok {
		if s.missingMetricIDs == nil {
			s.missingMetricIDs = make(map[uint64]uint64)
		}
		deleteDeadline = ct + 60
		s.missingMetricIDs[metricID] = deleteDeadline
	}
	return ct > deleteDeadline
}

// MetricNamesStatsResponse contains metric names usage stats API response
type MetricNamesStatsResponse = metricnamestats.StatsResult

// MetricNamesStatsRecord represents record at MetricNamesStatsResponse
type MetricNamesStatsRecord = metricnamestats.StatRecord

// GetMetricNamesStats returns metric names usage stats with given limit and le predicate
func (s *Storage) GetMetricNamesStats(_ *querytracer.Tracer, limit, le int, matchPattern string) MetricNamesStatsResponse {
	return s.metricsTracker.GetStats(limit, le, matchPattern)
}

// ResetMetricNamesStats resets state for metric names usage tracker
func (s *Storage) ResetMetricNamesStats(_ *querytracer.Tracer) {
	s.metricsTracker.Reset(s.tsidCache.Reset)
}

// GetMetadataRows returns time series metric names metadata for the given args
func (s *Storage) GetMetadataRows(qt *querytracer.Tracer, limit int, metricName string) []*metricsmetadata.Row {
	var (
		res []*metricsmetadata.Row
	)

	qt = qt.NewChild("search metrics metadata rows limit=%d,metricName=%q", limit, metricName)
	res = s.metadataStorage.Get(limit, metricName)
	qt.Printf("found %d metadata rows", len(res))
	qt.Done()
	return res
}

// AddMetadataRows writes time series metric names metadata into storage
func (s *Storage) AddMetadataRows(rows []metricsmetadata.Row) {
	s.metadataStorage.Add(rows)
}
