package storage

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
)

const (
	retention31Days = 31 * 24 * time.Hour
	retentionMax    = 100 * 12 * retention31Days
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

	// legacyIDBPrev and legacyIDBCurr contain the legacy previous and current
	// IndexDBs respectively if they existed on filesystem before partition
	// index was introduced. Otherwise, these fields will contain nil pointers.
	// The fields will also contain nil pointers once the corresponding IndexDBs
	// will become outside the retention period.
	//
	// The support of legacy IndexDBs is required to provide forward
	// compatibility with partition index.
	legacyIDBPrev atomic.Pointer[indexDB]
	legacyIDBCurr atomic.Pointer[indexDB]

	// legacyIDBLock prevents accidental removal of legacy indexDBs by
	// retentionWatcher while they are in use by some storage operation(s).
	legacyIDBLock sync.Mutex

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

	// dateMetricIDCache is (indexDB.id, Date, MetricID) cache.
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
	// This is used inside searchMetricNameWithCache() and getTSIDsFromMetricIDs()
	// for detecting permanently missing metricID->metricName/TSID entries.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5959
	missingMetricIDsLock          sync.Mutex
	missingMetricIDs              map[uint64]uint64
	missingMetricIDsResetDeadline uint64

	// isReadOnly is set to true when the storage is in read-only mode.
	isReadOnly atomic.Bool

	metricsTracker *metricnamestats.Tracker
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

// OpenOptions optional args for MustOpenStorage
type OpenOptions struct {
	Retention             time.Duration
	MaxHourlySeries       int
	MaxDailySeries        int
	DisablePerDayIndex    bool
	TrackMetricNamesStats bool
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
	s := &Storage{
		path:           path,
		cachePath:      filepath.Join(path, cacheDirname),
		retentionMsecs: retention.Milliseconds(),
		stopCh:         make(chan struct{}),
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
	if opts.MaxHourlySeries > 0 {
		s.hourlySeriesLimiter = bloomfilter.NewLimiter(opts.MaxHourlySeries, time.Hour)
	}
	if opts.MaxDailySeries > 0 {
		s.dailySeriesLimiter = bloomfilter.NewLimiter(opts.MaxDailySeries, 24*time.Hour)
	}

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache("metricName_tsid", getTSIDCacheSize())
	s.metricIDCache = s.mustLoadCache("metricID_tsid", mem/16)
	s.metricNameCache = s.mustLoadCache("metricID_metricName", getMetricNamesCacheSize())
	s.dateMetricIDCache = newDateMetricIDCache()

	if opts.TrackMetricNamesStats {
		mnt := metricnamestats.MustLoadFrom(filepath.Join(s.cachePath, "metric_usage_tracker"), uint64(getMetricNamesStatsCacheSize()))
		s.metricsTracker = mnt
		if mnt.IsEmpty() {
			// metric names tracker performs attempt to track timeseries during ingestion only at tsid cache miss.
			// It allows to do not decrease storage performance.
			logger.Infof("resetting tsidCache in order to properly track metric names stats usage")
			s.tsidCache.Reset()
		}
	}

	// Load metadata
	metadataDir := filepath.Join(path, metadataDirname)
	isEmptyDB := !fs.IsPathExist(filepath.Join(path, indexdbDirname))
	fs.MustMkdirIfNotExist(metadataDir)
	s.minTimestampForCompositeIndex = mustGetMinTimestampForCompositeIndex(metadataDir, isEmptyDB)

	s.disablePerDayIndex = opts.DisablePerDayIndex

	legacyIDBPath := filepath.Join(path, indexdbDirname)
	// Do not create legacy IndexDB snapshots dir if it does not exist.
	if path := filepath.Join(legacyIDBPath, snapshotsDirname); fs.IsPathExist(path) {
		// Cleanup the legacy IndexDB snapshots dir only if it exists.
		fs.MustRemoveTemporaryDirs(path)
	}
	legacyIDBPrev, legacyIDBCurr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	s.legacyIDBPrev.Store(legacyIDBPrev)
	s.legacyIDBCurr.Store(legacyIDBCurr)
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
	// partition indexDB.
	legacyDeletedMetricIDSet := &uint64set.Set{}
	if legacyIDBPrev != nil {
		legacyDeletedMetricIDSet.Union(legacyIDBPrev.getDeletedMetricIDs())
	}
	if legacyIDBCurr != nil {
		legacyDeletedMetricIDSet.Union(legacyIDBCurr.getDeletedMetricIDs())
	}
	legacyDeletedMetricIDs := legacyDeletedMetricIDSet.AppendTo(nil)
	ptws := tb.GetPartitions(nil)
	for _, ptw := range ptws {
		ptw.pt.idb.updateDeletedMetricIDs(legacyDeletedMetricIDs)
	}
	tb.PutPartitions(ptws)

	// Load nextDayMetricIDs cache after the data table is opened since it
	// requires the table to operate properly.
	// Load prevHourMetricIDs, currHourMetricIDs, and nextDayMetricIDs caches
	// after the data table is opened since they require the partition index to
	// operate properly.
	hour := fasttime.UnixHour()
	hmCurr := s.mustLoadHourMetricIDs(hour, "curr_hour_metric_ids_v2")
	hmPrev := s.mustLoadHourMetricIDs(hour-1, "prev_hour_metric_ids_v2")
	s.currHourMetricIDs.Store(hmCurr)
	s.prevHourMetricIDs.Store(hmPrev)
	date := fasttime.UnixDate()
	nextDayMetricIDs := s.mustLoadNextDayMetricIDs(date)
	s.nextDayMetricIDs.Store(nextDayMetricIDs)
	s.pendingNextDayMetricIDs = &uint64set.Set{}

	s.startCurrHourMetricIDsUpdater()
	s.startNextDayMetricIDsUpdater()
	s.startLegacyRetentionWatcher()

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
	// Do not flush legacy IndexDBs since they are read-only.

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

	// TODO(@rtm0): Extract into Storage.createLegacyIndexDBSnapshot() and move
	// to storage_legacy.go.
	legacyIDBPrev, legacyIDBCurr := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBPrev, legacyIDBCurr)
	if legacyIDBPrev != nil || legacyIDBCurr != nil {
		idbSnapshot := filepath.Join(srcDir, indexdbDirname, snapshotsDirname, snapshotName)
		if legacyIDBPrev != nil {
			prevSnapshot := filepath.Join(idbSnapshot, legacyIDBPrev.name)
			legacyIDBPrev.tb.LegacyMustCreateSnapshotAt(prevSnapshot)
		}
		if legacyIDBCurr != nil {
			currSnapshot := filepath.Join(idbSnapshot, legacyIDBCurr.name)
			legacyIDBCurr.tb.LegacyMustCreateSnapshotAt(currSnapshot)
		}
		dstIdbDir := filepath.Join(dstDir, indexdbDirname)
		fs.MustSymlinkRelative(idbSnapshot, dstIdbDir)
	}

	fs.MustSyncPath(dstDir)

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
	fs.MustRemoveDirAtomic(idbPath)
	fs.MustRemoveDirAtomic(snapshotPath)

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

	DateMetricIDCacheSize        uint64
	DateMetricIDCacheSizeBytes   uint64
	DateMetricIDCacheSyncsCount  uint64
	DateMetricIDCacheResetsCount uint64

	HourMetricIDCacheSize      uint64
	HourMetricIDCacheSizeBytes uint64

	NextDayMetricIDCacheSize      uint64
	NextDayMetricIDCacheSizeBytes uint64

	NextRetentionSeconds uint64

	MetricNamesUsageTrackerSize         uint64
	MetricNamesUsageTrackerSizeBytes    uint64
	MetricNamesUsageTrackerSizeMaxBytes uint64

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

	m.DateMetricIDCacheSize += uint64(s.dateMetricIDCache.EntriesCount())
	m.DateMetricIDCacheSizeBytes += uint64(s.dateMetricIDCache.SizeBytes())
	m.DateMetricIDCacheSyncsCount += s.dateMetricIDCache.syncsCount.Load()
	m.DateMetricIDCacheResetsCount += s.dateMetricIDCache.resetsCount.Load()

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

	var tm metricnamestats.TrackerMetrics
	s.metricsTracker.UpdateMetrics(&tm)
	m.MetricNamesUsageTrackerSizeBytes = tm.CurrentSizeBytes
	m.MetricNamesUsageTrackerSize = tm.CurrentItemsCount
	m.MetricNamesUsageTrackerSizeMaxBytes = tm.MaxSizeBytes

	d := s.legacyNextRetentionSeconds()
	if d < 0 {
		d = 0
	}
	m.NextRetentionSeconds = uint64(d)

	s.tb.UpdateMetrics(&m.TableMetrics)

	legacyIDBPrev, legacyIDBCurr := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBPrev, legacyIDBCurr)
	if legacyIDBPrev != nil {
		legacyIDBPrev.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}
	if legacyIDBCurr != nil {
		legacyIDBCurr.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}

}

// TODO(@rtm0): Move to storage_legacy.go
func (s *Storage) legacyNextRetentionSeconds() int64 {
	return s.legacyNextRotationTimestamp.Load() - int64(fasttime.UnixTimestamp())
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
	// NotifyReadWriteMode() is not called for legacy idbs becase they are
	// opened in read-only mode and must remain so throughout the process life.
}

// TODO(@rtm0): Move to storage_legacy.go
func (s *Storage) startLegacyRetentionWatcher() {
	if !s.hasLegacyIndexDBs() {
		return
	}
	s.legacyRetentionWatcherWG.Add(1)
	go func() {
		s.legacyRetentionWatcher()
		s.legacyRetentionWatcherWG.Done()
	}()
}

// TODO(@rtm0): Move to storage_legacy.go
func (s *Storage) legacyRetentionWatcher() {
	for {
		d := s.legacyNextRetentionSeconds()
		select {
		case <-s.stopCh:
			return
		case currentTime := <-time.After(time.Second * time.Duration(d)):
			s.legacyMustRotateIndexDB(currentTime)
			if !s.hasLegacyIndexDBs() {
				return
			}
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
	s.mustSaveCache(s.tsidCache, "metricName_tsid")
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

	legacyIDBPrev, legacyIDBCurr := s.legacyIDBPrev.Load(), s.legacyIDBCurr.Load()
	if legacyIDBPrev != nil {
		legacyIDBPrev.MustClose()
	}
	if legacyIDBCurr != nil {
		legacyIDBCurr.MustClose()
	}

	// Save caches.
	s.mustSaveCache(s.tsidCache, "metricName_tsid")
	s.tsidCache.Stop()
	s.mustSaveCache(s.metricIDCache, "metricID_tsid")
	s.metricIDCache.Stop()
	s.mustSaveCache(s.metricNameCache, "metricID_metricName")
	s.metricNameCache.Stop()

	hmCurr := s.currHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmCurr, "curr_hour_metric_ids_v2")
	hmPrev := s.prevHourMetricIDs.Load()
	s.mustSaveHourMetricIDs(hmPrev, "prev_hour_metric_ids_v2")

	nextDayMetricIDs := s.nextDayMetricIDs.Load()
	s.mustSaveNextDayMetricIDs(nextDayMetricIDs)

	s.metricsTracker.MustClose()
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

func (s *Storage) mustLoadNextDayMetricIDs(date uint64) *byDateMetricIDEntry {
	ts := int64(date) * msecPerDay
	idb := s.tb.MustGetIndexDB(ts)
	defer s.tb.PutIndexDB(idb)

	e := &byDateMetricIDEntry{
		k: dateKey{
			idbID: idb.id,
			date:  date,
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
	idbIDLoaded := encoding.UnmarshalUint64(src)
	src = src[8:]
	if idbIDLoaded != idb.id {
		logger.Infof("discarding %s, since it contains data for indexDB from previous month; got %d; want %d", path, idbIDLoaded, idb.id)
		return e
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
	dst = encoding.MarshalUint64(dst, e.k.idbID)
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

// LegacySetRetentionTimezoneOffset sets the offset, which is used for
// calculating the time for legacy indexdb rotation.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574
//
// TODO(@rtm0): Move to storage_legacy.go
func LegacySetRetentionTimezoneOffset(offset time.Duration) {
	legacyRetentionTimezoneOffsetSecs = int64(offset.Seconds())
}

// TODO(@rtm0): Move to storage_legacy.go
var legacyRetentionTimezoneOffsetSecs int64

// TODO(@rtm0): Move to storage_legacy.go
func legacyNextRetentionDeadlineSeconds(atSecs, retentionSecs, offsetSecs int64) int64 {
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

// searchAndMerge concurrently performs a search operation on all partition
// IndexDBs that overlap with the given time range and optionally legacy current
// and previous IndexDBs. The individual search results are then merged (merge function applied
// only if search covers more than one index partition).
//
// The function creates a child query tracer for each search function call and
// closes it once the search() returns. Thus, implementations of search func
// must not close the query tracer that they receive.
func searchAndMerge[T any](qt *querytracer.Tracer, s *Storage, tr TimeRange, search func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (T, error), merge func([]T) T) (T, error) {
	qt = qt.NewChild("search indexDBs: timeRange=%v", &tr)
	defer qt.Done()

	var idbs []*indexDB

	ptIDBs := s.tb.GetIndexDBs(tr)
	defer s.tb.PutIndexDBs(ptIDBs)
	idbs = append(idbs, ptIDBs...)

	legacyIDBPrev, legacyIDBCurr := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBPrev, legacyIDBCurr)
	if legacyIDBPrev != nil {
		idbs = append(idbs, legacyIDBPrev)
	}
	if legacyIDBCurr != nil {
		idbs = append(idbs, legacyIDBCurr)
	}

	if len(idbs) == 0 {
		qt.Printf("no indexDBs found")
		var zeroValue T
		return zeroValue, nil
	}

	// It is faster to process one indexDB without spawning goroutines.
	if len(idbs) == 1 {
		idb := idbs[0]
		searchTR := s.adjustTimeRange(tr, idb.tr)
		qtChild := qt.NewChild("search indexDB %s: timeRange=%v", idb.name, &searchTR)
		defer qtChild.Done()
		data, err := search(qtChild, idb, searchTR)
		if err != nil {
			var zeroValue T
			return zeroValue, err
		}
		return data, nil
	}

	qtSearch := qt.NewChild("search %d indexDBs in parallel", len(idbs))
	var wg sync.WaitGroup
	data := make([]T, len(idbs))
	errs := make([]error, len(idbs))
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

// mergeUniq combines the values of several slices into once slice, duplicate
// values are ignored.
func mergeUniq[T cmp.Ordered](data [][]T) []T {
	maxLength := 0
	for _, s := range data {
		if len(s) > maxLength {
			maxLength += len(s)
		}
	}
	if maxLength == 0 {
		return []T{}
	}

	all := make([]T, 0, maxLength)
	seen := make(map[T]struct{}, maxLength)
	for _, s := range data {
		for _, v := range s {
			if _, ok := seen[v]; ok {
				continue
			}
			all = append(all, v)
			seen[v] = struct{}{}
		}
	}
	return all
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
	metricNames, err := searchAndMerge(qt, s, tr, search, mergeUniq)

	qt.Donef("found %d metric names", len(metricNames))
	return metricNames, err
}

// ErrDeadlineExceeded is returned when the request times out.
var ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")

// DeleteSeries marks as deleted all series matching the given tfss and
// updates or resets all caches where the corresponding TSIDs and MetricIDs may
// be stored.
//
// If the number of the series exceeds maxMetrics, no series will be deleted and
// an error will be returned. Otherwise, the function returns the number of
// metrics deleted.
//
// If legacy indexDBs are present, the method will also delete the metricIDs
// from them. However, because legacy indexDBs are read-only, no background
// merges will be performed.
func (s *Storage) DeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (int, error) {
	qt = qt.NewChild("delete series: filters=%s, maxMetrics=%d", tfss, maxMetrics)
	defer qt.Done()

	if len(tfss) == 0 {
		return 0, nil
	}

	// Not deleting in parallel because the deletion operation is rare.

	deletedMetricIDs := &uint64set.Set{}
	legacyDMIs, err := s.legacyDeleteSeries(qt, tfss, maxMetrics)
	if err != nil {
		return 0, err
	}
	deletedMetricIDs.AddMulti(legacyDMIs)

	// Get all partition IndexDBs.
	idbs := s.tb.GetIndexDBs(TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	})
	defer s.tb.PutIndexDBs(idbs)

	for _, idb := range idbs {
		qt.Printf("start deleting from %s partition indexDB", idb.name)
		if len(legacyDMIs) > 0 {
			idb.updateDeletedMetricIDs(legacyDMIs)
		}
		dmis, err := idb.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return 0, err
		}
		deletedMetricIDs.AddMulti(dmis)
		qt.Printf("deleted %d metricIDs from %s partition indexDB", len(dmis), idb.name)
	}

	// Do not reset MetricID->MetricName cache, since it must be used only
	// after filtering out deleted metricIDs.

	n := deletedMetricIDs.Len()
	qt.Donef("deleted %d unique metricIDs", n)
	return n, nil
}

// SearchLabelNames searches for label names matching the given tfss on tr.
func (s *Storage) SearchLabelNames(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label names: filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", tfss, &tr, maxLabelNames, maxMetrics)

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchLabelNames(qt, accountID, projectID, tfss, tr, maxLabelNames, maxMetrics, deadline)
	}
	labelNames, err := searchAndMerge(qt, s, tr, search, mergeUniq)
	qt.Donef("found %d label names", len(labelNames))
	return labelNames, err
}

// SearchLabelValues searches for label values for the given labelName, filters and tr.
func (s *Storage) SearchLabelValues(qt *querytracer.Tracer, accountID, projectID uint32, labelName string, tfss []*TagFilters, tr TimeRange, maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label values: labelName=%q, filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", labelName, tfss, &tr, maxLabelValues, maxMetrics)

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchLabelValues(qt, accountID, projectID, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
	}
	labelValues, err := searchAndMerge(qt, s, tr, search, mergeUniq)
	qt.Donef("found %d label values", len(labelValues))
	return labelValues, err
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
func (s *Storage) SearchTagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int, deadline uint64) ([]string, error) {
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes, deadline)
	}
	return searchAndMerge(qt, s, tr, search, mergeUniq)
}

// SearchGraphitePaths returns all the matching paths for the given graphite
// query on the given tr.
func (s *Storage) SearchGraphitePaths(qt *querytracer.Tracer, accountID, projectID uint32, tr TimeRange, query []byte, maxPaths int, deadline uint64) ([]string, error) {
	query = replaceAlternateRegexpsWithGraphiteWildcards(query)

	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchGraphitePaths(qt, accountID, projectID, tr, nil, query, maxPaths, deadline)
	}
	return searchAndMerge(qt, s, tr, search, mergeUniq)
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
func (s *Storage) GetSeriesCount(accountID, projectID uint32, deadline uint64) (uint64, error) {
	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: time.Now().UnixMilli(),
	}
	search := func(_ *querytracer.Tracer, idb *indexDB, _ TimeRange) (uint64, error) {
		return idb.GetSeriesCount(accountID, projectID, deadline)
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

// SearchTenants returns list of registered tenants on the given tr.
func (s *Storage) SearchTenants(qt *querytracer.Tracer, tr TimeRange, deadline uint64) ([]string, error) {
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) ([]string, error) {
		return idb.SearchTenants(qt, tr, deadline)
	}
	return searchAndMerge(qt, s, tr, search, mergeUniq)
}

// GetTSDBStatus returns TSDB status data for /api/v1/status/tsdb
//
// The method does not provide status for legacy IDBs because merging partition
// indexDB and legacy indexDB statuses is non-trivial and not many users use
// this status for historical data.
func (s *Storage) GetTSDBStatus(qt *querytracer.Tracer, accountID, projectID uint32, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	idbs := s.tb.GetIndexDBs(TimeRange{
		MinTimestamp: int64(date) * msecPerDay,
		MaxTimestamp: int64(date+1)*msecPerDay - 1,
	})
	defer s.tb.PutIndexDBs(idbs)

	if len(idbs) == 0 {
		return &TSDBStatus{}, nil
	}
	if s.disablePerDayIndex {
		date = globalIndexDate
	}

	res, err := idbs[0].GetTSDBStatus(qt, accountID, projectID, tfss, date, focusLabel, topN, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if s.metricsTracker != nil && len(res.SeriesCountByMetricName) > 0 {
		// for performance reason always check if metricsTracker is configured
		names := make([]string, len(res.SeriesCountByMetricName))
		for idx, mns := range res.SeriesCountByMetricName {
			names[idx] = mns.Name
		}
		res.SeriesQueryStatsByMetricName = s.metricsTracker.GetStatRecordsForNames(accountID, projectID, names)
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

func (s *Storage) date(millis int64) uint64 {
	if s.disablePerDayIndex {
		return globalIndexDate
	}
	return uint64(millis) / msecPerDay
}

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
	if searchTR.MinTimestamp > idbTR.MinTimestamp {
		tr.MinTimestamp = searchTR.MinTimestamp
	}
	if searchTR.MaxTimestamp < idbTR.MaxTimestamp {
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

	var idb *indexDB
	var is *indexSearch
	var deletedMetricIDs *uint64set.Set
	var firstWarn error
	for i := range mrs {
		mr := &mrs[i]
		date := uint64(mr.Timestamp) / msecPerDay

		if idb == nil || !idb.HasTimestamp(mr.Timestamp) {
			if idb != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutIndexDB(idb)
			}
			idb = s.tb.MustGetIndexDB(mr.Timestamp)
			is = idb.getIndexSearch(0, 0, noDeadline)
			deletedMetricIDs = idb.getDeletedMetricIDs()
		}

		if s.getTSIDFromCache(&lTSID, mr.MetricNameRaw) && !deletedMetricIDs.Has(lTSID.TSID.MetricID) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been found in cache and isn't deleted.
			// If the TSID is deleted, we re-register time series.
			// Eventually, the deleted TSID will be removed from the cache.{
			if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				continue
			}
			if !is.hasMetricID(lTSID.TSID.MetricID, lTSID.TSID.AccountID, lTSID.TSID.ProjectID) {
				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()
				idb.createGlobalIndexes(&lTSID.TSID, mn)
			}
			if !s.dateMetricIDCache.Has(idb.id, date, lTSID.TSID.MetricID) {
				if !is.hasDateMetricID(date, lTSID.TSID.MetricID, lTSID.TSID.AccountID, lTSID.TSID.ProjectID) {
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
				s.dateMetricIDCache.Set(idb.id, date, lTSID.TSID.MetricID)
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
			s.invalidRawMetricNames.Add(1)
			continue
		}
		mn.sortTags()
		metricNameBuf = mn.Marshal(metricNameBuf[:0])

		if is.getTSIDByMetricName(&lTSID.TSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				continue
			}

			s.putSeriesToCache(mr.MetricNameRaw, &lTSID, idb.id, date)
			continue
		}

		// Slowest path - there is no TSID in indexdb for the given mr.MetricNameRaw. Create it.
		generateTSID(&lTSID.TSID, mn)

		if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			continue
		}

		// Schedule creating TSID indexes instead of creating them synchronously.
		// This should keep stable the ingestion rate when new time series are ingested.
		createAllIndexesForMetricName(idb, mn, &lTSID.TSID, date)
		s.putSeriesToCache(mr.MetricNameRaw, &lTSID, idb.id, date)
		newSeriesCount++
	}
	if idb != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		s.tb.PutIndexDB(idb)
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
	hmPrev := s.prevHourMetricIDs.Load()
	hmCurr := s.currHourMetricIDs.Load()
	var pendingHourEntries []pendingHourMetricIDEntry
	addToPendingHourEntries := func(hour uint64, accountID, projectID uint32, metricID uint64) {
		if hour == hmCurr.hour && !hmCurr.m.Has(metricID) {
			e := pendingHourMetricIDEntry{
				AccountID: accountID,
				ProjectID: projectID,
				MetricID:  metricID,
			}
			pendingHourEntries = append(pendingHourEntries, e)
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
		date := s.date(r.Timestamp)
		hour := uint64(r.Timestamp) / msecPerHour

		if idb == nil || !idb.HasTimestamp(r.Timestamp) {
			if idb != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutIndexDB(idb)
			}
			idb = s.tb.MustGetIndexDB(r.Timestamp)
			is = idb.getIndexSearch(0, 0, noDeadline)
			deletedMetricIDs = idb.getDeletedMetricIDs()
		}

		// Search for TSID for the given mr.MetricNameRaw and store it at r.TSID.
		if string(mr.MetricNameRaw) == string(prevMetricNameRaw) {
			// Fast path - the current mr contains the same metric name as the previous mr, so it contains the same TSID.
			// This path should trigger on bulk imports when many rows contain the same MetricNameRaw.

			if !is.hasMetricID(prevTSID.MetricID, prevTSID.AccountID, prevTSID.ProjectID) {
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

		if s.getTSIDFromCache(&lTSID, mr.MetricNameRaw) && !deletedMetricIDs.Has(lTSID.TSID.MetricID) {
			// Fast path - the TSID for the given mr.MetricNameRaw has been found in cache and isn't deleted.
			// If the TSID is deleted, we re-register time series.
			// Eventually, the deleted TSID will be removed from the cache.
			if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip row, since it exceeds cardinality limit
				j--
				continue
			}

			if !is.hasMetricID(lTSID.TSID.MetricID, lTSID.TSID.AccountID, lTSID.TSID.ProjectID) {
				if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
					if firstWarn == nil {
						firstWarn = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %w", mr.MetricNameRaw, err)
					}
					j--
					s.invalidRawMetricNames.Add(1)
					continue
				}
				mn.sortTags()
				idb.createGlobalIndexes(&lTSID.TSID, mn)
			}

			r.TSID = lTSID.TSID
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			addToPendingHourEntries(hour, lTSID.TSID.AccountID, lTSID.TSID.ProjectID, lTSID.TSID.MetricID)

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
		s.metricsTracker.RegisterIngestRequest(mn.AccountID, mn.ProjectID, mn.MetricGroup)

		// Search for TSID for the given mr.MetricNameRaw in the indexdb.
		if is.getTSIDByMetricName(&lTSID.TSID, metricNameBuf, date) {
			// Slower path - the TSID has been found in indexdb.

			if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
				// Skip the row, since it exceeds the configured cardinality limit.
				j--
				continue
			}

			s.putSeriesToCache(mr.MetricNameRaw, &lTSID, idb.id, date)

			r.TSID = lTSID.TSID
			prevTSID = lTSID.TSID
			prevMetricNameRaw = mr.MetricNameRaw

			addToPendingHourEntries(hour, lTSID.TSID.AccountID, lTSID.TSID.ProjectID, lTSID.TSID.MetricID)
			continue
		}

		// Slowest path - the TSID for the given mr.MetricNameRaw isn't found in indexdb. Create it.
		generateTSID(&lTSID.TSID, mn)

		if !s.registerSeriesCardinality(lTSID.TSID.MetricID, mr.MetricNameRaw) {
			// Skip the row, since it exceeds the configured cardinality limit.
			j--
			continue
		}

		createAllIndexesForMetricName(idb, mn, &lTSID.TSID, date)
		s.putSeriesToCache(mr.MetricNameRaw, &lTSID, idb.id, date)
		newSeriesCount++

		r.TSID = lTSID.TSID
		prevTSID = r.TSID
		prevMetricNameRaw = mr.MetricNameRaw

		addToPendingHourEntries(hour, lTSID.TSID.AccountID, lTSID.TSID.ProjectID, lTSID.TSID.MetricID)

		if logNewSeries {
			logger.Infof("new series created: %s", mn.String())
		}
	}
	if idb != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		s.tb.PutIndexDB(idb)
	}

	s.slowRowInserts.Add(slowInsertsCount)
	s.newTimeseriesCreated.Add(newSeriesCount)
	s.timeseriesRepopulated.Add(seriesRepopulated)

	dstMrs = dstMrs[:j]
	rows = rows[:j]

	if len(pendingHourEntries) > 0 {
		s.pendingHourEntriesLock.Lock()
		s.pendingHourEntries = append(s.pendingHourEntries, pendingHourEntries...)
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

// SetLogNewSeries updates new series logging.
//
// This function must be called before any calling any storage functions.
func SetLogNewSeries(ok bool) {
	logNewSeries = ok
}

var logNewSeries = false

func createAllIndexesForMetricName(db *indexDB, mn *MetricName, tsid *TSID, date uint64) {
	db.createGlobalIndexes(tsid, mn)
	db.createPerDayIndexes(date, tsid, mn)
}

func (s *Storage) putSeriesToCache(metricNameRaw []byte, lTSID *legacyTSID, idbID, date uint64) {
	// Store the TSID indexdb into cache, so future rows for that TSID are
	// ingested via fast path.
	s.putTSIDToCache(lTSID, metricNameRaw)

	// Register the (indexDB.id, date, metricID) entry in the cache,
	// so next time the entry is found there instead of searching for it in the
	// indexdb.
	s.dateMetricIDCache.Set(idbID, date, lTSID.TSID.MetricID)
}

func (s *Storage) registerSeriesCardinality(metricID uint64, metricNameRaw []byte) bool {
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
// during the last hour before that parition becomes the current one. This is
// needed in order to reduce spikes in CPU and disk IO usage just after the
// switch. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1401.
func (s *Storage) prefillNextIndexDB(rows []rawRow, mrs []*MetricRow) error {
	now := time.Unix(int64(fasttime.UnixTimestamp()), 0)
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	d := nextMonth.Sub(now).Seconds()
	if d >= 3600 {
		// Fast path: nothing to pre-fill because it is too early.
		// The pre-fill is started during the last hour before the indexdb rotation.
		return nil
	}

	// Slower path: less than hour left for the next indexdb rotation.
	// Pre-populate idbNext with the increasing probability until the rotation.
	// The probability increases from 0% to 100% proportioinally to d=[3600 .. 0].
	pMin := float64(d) / 3600

	idbNext := s.tb.MustGetIndexDB(nextMonth.UnixMilli())
	defer s.tb.PutIndexDB(idbNext)
	isNext := idbNext.getIndexSearch(0, 0, noDeadline)
	defer idbNext.putIndexSearch(isNext)

	var firstError error
	var lTSID legacyTSID
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
		date := s.date(r.Timestamp)
		metricID := r.TSID.MetricID
		if s.dateMetricIDCache.Has(idbNext.id, date, metricID) {
			// Indexes are already pre-filled.
			continue
		}

		// Check whether the given (date, metricID) is already present in idbNext.
		if isNext.hasDateMetricID(date, metricID, r.TSID.AccountID, r.TSID.ProjectID) {
			// Indexes are already pre-filled at idbNext.
			//
			// Register the (indexDB.id, date, metricID) entry in the cache,
			// so next time the entry is found there instead of searching for it in the indexdb.
			s.dateMetricIDCache.Set(idbNext.id, date, metricID)
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
		s.putSeriesToCache(metricNameRaw, &lTSID, idbNext.id, date)
		timeseriesPreCreated++
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

	var idb *indexDB

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
		if hour == hmCurr.hour {
			// The row belongs to the current hour. Check for the current hour cache.
			if hmCurr.m.Has(metricID) {
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
			if hmPrev.idbID == hmCurr.idbID && date == hmPrevDate && hmPrev.m.Has(metricID) {
				// The metricID is already registered for the current day on the previous hour.
				continue
			}
		}

		if idb == nil || !idb.HasTimestamp(r.Timestamp) {
			if idb != nil {
				s.tb.PutIndexDB(idb)
			}
			idb = s.tb.MustGetIndexDB(r.Timestamp)
		}

		// Slower path: check global cache for (indexDB.id, date, metricID) entry.
		if s.dateMetricIDCache.Has(idb.id, date, metricID) {
			continue
		}
		// Slow path: store the (date, metricID) entry in the indexDB.
		pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
			date: date,
			tsid: &r.TSID,
			mr:   mrs[i],
		})
	}
	if idb != nil {
		s.tb.PutIndexDB(idb)
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

	var firstError error
	dateMetricIDsForCache := make(map[uint64][]dateMetricID)
	mn := GetMetricName()
	var is *indexSearch
	for _, dmid := range pendingDateMetricIDs {
		date := dmid.date
		metricID := dmid.tsid.MetricID
		timestamp := int64(date) * msecPerDay
		if idb == nil || !idb.HasTimestamp(timestamp) {
			if idb != nil {
				if is != nil {
					idb.putIndexSearch(is)
				}
				s.tb.PutIndexDB(idb)
			}
			idb = s.tb.MustGetIndexDB(timestamp)
			is = idb.getIndexSearch(0, 0, noDeadline)
		}

		if !is.hasDateMetricID(date, metricID, dmid.tsid.AccountID, dmid.tsid.ProjectID) {
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

		dateMetricIDsForCache[idb.id] = append(dateMetricIDsForCache[idb.id], dateMetricID{
			date:     date,
			metricID: metricID,
		})
	}
	if idb != nil {
		if is != nil {
			idb.putIndexSearch(is)
		}
		s.tb.PutIndexDB(idb)
	}

	PutMetricName(mn)
	// The (date, metricID) entries must be added to cache only after they have been successfully added to indexDB.
	for idbID, dateMetricIDs := range dateMetricIDsForCache {
		s.dateMetricIDCache.Store(idbID, dateMetricIDs)
	}
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
	syncsCount  atomic.Uint64
	resetsCount atomic.Uint64

	// Contains immutable map
	byDate atomic.Pointer[byDateMetricIDMap]

	// Contains mutable map protected by mu
	byDateMutable *byDateMetricIDMap

	// Contains the number of slow accesses to byDateMutable.
	// Is used for deciding when to merge byDateMutable to byDate.
	// Protected by mu.
	slowHits int

	mu sync.Mutex
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
	dmc.slowHits = 0

	dmc.resetsCount.Add(1)
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

func (dmc *dateMetricIDCache) Has(idbID, date, metricID uint64) bool {
	byDate := dmc.byDate.Load()
	v := byDate.get(idbID, date)
	if v.Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}

	// Slow path. Acquire the lock and search the immutable map again and then
	// also search the mutable map.
	return dmc.hasSlow(idbID, date, metricID)
}

func (dmc *dateMetricIDCache) hasSlow(idbID, date, metricID uint64) bool {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	// First, check immutable map again because the entry may have been moved to
	// the immutable map by the time the caller acquires the lock.
	byDate := dmc.byDate.Load()
	v := byDate.get(idbID, date)
	if v.Has(metricID) {
		return true
	}

	// Then check immutable map.
	vMutable := dmc.byDateMutable.get(idbID, date)
	ok := vMutable.Has(metricID)
	if ok {
		dmc.slowHits++
		if dmc.slowHits > (v.Len()+vMutable.Len())/2 {
			// It is cheaper to merge byDateMutable into byDate than to pay inter-cpu sync costs when accessing vMutable.
			dmc.syncLocked()
			dmc.slowHits = 0
		}
	}
	return ok
}

type dateMetricID struct {
	date     uint64
	metricID uint64
}

func (dmc *dateMetricIDCache) Store(idbID uint64, dmids []dateMetricID) {
	var prevDate uint64
	metricIDs := make([]uint64, 0, len(dmids))
	dmc.mu.Lock()
	for _, dmid := range dmids {
		if prevDate == dmid.date {
			metricIDs = append(metricIDs, dmid.metricID)
			continue
		}
		if len(metricIDs) > 0 {
			v := dmc.byDateMutable.getOrCreate(idbID, prevDate)
			v.AddMulti(metricIDs)
		}
		metricIDs = append(metricIDs[:0], dmid.metricID)
		prevDate = dmid.date
	}
	if len(metricIDs) > 0 {
		v := dmc.byDateMutable.getOrCreate(idbID, prevDate)
		v.AddMulti(metricIDs)
	}
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) Set(idbID, date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(idbID, date)
	v.Add(metricID)
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) syncLocked() {
	if len(dmc.byDateMutable.m) == 0 {
		// Nothing to sync.
		return
	}

	// Merge data from byDate into byDateMutable and then atomically replace byDate with the merged data.
	byDate := dmc.byDate.Load()
	byDateMutable := dmc.byDateMutable
	byDateMutable.hotEntry.Store(&byDateMetricIDEntry{})

	keepDatesMap := make(map[uint64]struct{}, len(byDateMutable.m))
	for k, e := range byDateMutable.m {
		keepDatesMap[k.date] = struct{}{}
		v := byDate.get(k.idbID, k.date)
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
	}

	// Copy entries from byDate, which are missing in byDateMutable
	allDatesMap := make(map[uint64]struct{}, len(byDate.m))
	for k, e := range byDate.m {
		allDatesMap[k.date] = struct{}{}
		v := byDateMutable.get(k.idbID, k.date)
		if v != nil {
			continue
		}
		byDateMutable.m[k] = e
	}

	if len(byDateMutable.m) > 2 {
		// Keep only entries for the last two dates from allDatesMap plus all the entries for byDateMutable.
		dates := make([]uint64, 0, len(allDatesMap))
		for date := range allDatesMap {
			dates = append(dates, date)
		}
		sort.Slice(dates, func(i, j int) bool {
			return dates[i] < dates[j]
		})
		if len(dates) > 2 {
			dates = dates[len(dates)-2:]
		}
		for _, date := range dates {
			keepDatesMap[date] = struct{}{}
		}
		for k := range byDateMutable.m {
			if _, ok := keepDatesMap[k.date]; !ok {
				delete(byDateMutable.m, k)
			}
		}
	}

	// Atomically replace byDate with byDateMutable
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()

	dmc.syncsCount.Add(1)

	if dmc.SizeBytes() > uint64(memory.Allowed())/256 {
		dmc.resetLocked()
	}
}

type byDateMetricIDMap struct {
	hotEntry atomic.Pointer[byDateMetricIDEntry]
	m        map[dateKey]*byDateMetricIDEntry
}

type dateKey struct {
	idbID uint64
	date  uint64
}

func newByDateMetricIDMap() *byDateMetricIDMap {
	dmm := &byDateMetricIDMap{
		m: make(map[dateKey]*byDateMetricIDEntry),
	}
	dmm.hotEntry.Store(&byDateMetricIDEntry{})
	return dmm
}

func (dmm *byDateMetricIDMap) get(idbID, date uint64) *uint64set.Set {
	hotEntry := dmm.hotEntry.Load()
	if hotEntry.k.idbID == idbID && hotEntry.k.date == date {
		// Fast path
		return &hotEntry.v
	}
	// Slow path
	k := dateKey{
		idbID: idbID,
		date:  date,
	}
	e := dmm.m[k]
	if e == nil {
		return nil
	}
	dmm.hotEntry.Store(e)
	return &e.v
}

func (dmm *byDateMetricIDMap) getOrCreate(idbID, date uint64) *uint64set.Set {
	v := dmm.get(idbID, date)
	if v != nil {
		return v
	}
	k := dateKey{
		idbID: idbID,
		date:  date,
	}
	e := &byDateMetricIDEntry{
		k: k,
	}
	dmm.m[k] = e
	return &e.v
}

type byDateMetricIDEntry struct {
	k dateKey
	v uint64set.Set
}

func (s *Storage) updateNextDayMetricIDs(date uint64) {
	ts := int64(date) * msecPerDay
	idb := s.tb.MustGetIndexDB(ts)
	defer s.tb.PutIndexDB(idb)

	e := s.nextDayMetricIDs.Load()
	s.pendingNextDayMetricIDsLock.Lock()
	pendingMetricIDs := s.pendingNextDayMetricIDs
	s.pendingNextDayMetricIDs = &uint64set.Set{}
	s.pendingNextDayMetricIDsLock.Unlock()
	if pendingMetricIDs.Len() == 0 && e.k.idbID == idb.id && e.k.date == date {
		// Fast path: nothing to update.
		return
	}

	// Slow path: union pendingMetricIDs with e.v
	if e.k.idbID == idb.id && e.k.date == date {
		pendingMetricIDs.Union(&e.v)
	} else {
		// Do not add pendingMetricIDs from the previous day to the current day,
		// since this may result in missing registration of the metricIDs in the per-day inverted index.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3309
		pendingMetricIDs = &uint64set.Set{}
	}
	k := dateKey{
		idbID: idb.id,
		date:  date,
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
	idbID := hm.idbID
	var m *uint64set.Set
	var byTenant map[accountProjectKey]*uint64set.Set
	if hm.hour == hour {
		m = hm.m.Clone()
		byTenant = make(map[accountProjectKey]*uint64set.Set, len(hm.byTenant))
		for k, e := range hm.byTenant {
			byTenant[k] = e.Clone()
		}
	} else {
		idbID = s.tb.MustGetIndexDBIDByHour(hour)
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
		idbID:    idbID,
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
	idbID    uint64
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

func (s *Storage) putTSIDToCache(tsid *legacyTSID, metricName []byte) {
	buf := (*[unsafe.Sizeof(*tsid)]byte)(unsafe.Pointer(tsid))[:]
	s.tsidCache.Set(metricName, buf)
}

// TODO(@rtm0): Move to storage_legacy.go
func (s *Storage) mustOpenLegacyIndexDBTables(path string) (prev, curr *indexDB) {
	if !fs.IsPathExist(path) {
		return nil, nil
	}

	fs.MustRemoveTemporaryDirs(path)

	// Search for the two most recent tables: prev and curr.

	// Placing the regexp inside the func in order to keep legacy code close to
	// each other and because this function is called only once on startup.
	indexDBTableNameRegexp := regexp.MustCompile("^[0-9A-F]{16}$")
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

	if len(tableNames) > 3 {
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
	if len(tableNames) == 3 {
		// Also remove next idb.
		pathToRemove := filepath.Join(path, tableNames[2])
		logger.Infof("removing next indexdb dir %q...", pathToRemove)
		fs.MustRemoveAll(pathToRemove)
		logger.Infof("removed next indexdb dir %q", pathToRemove)
		fs.MustSyncPath(path)
		tableNames = tableNames[:2]
	}

	numIDBs := len(tableNames)

	if numIDBs > 1 {
		currPath := filepath.Join(path, tableNames[1])
		curr = mustOpenLegacyIndexDBReadOnly(currPath, s)
	}

	if numIDBs > 0 {
		prevPath := filepath.Join(path, tableNames[0])
		prev = mustOpenLegacyIndexDBReadOnly(prevPath, s)
	}

	return prev, curr
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

// GetMetricNamesStats returns metric names usage stats with give limit and lte predicate
func (s *Storage) GetMetricNamesStats(_ *querytracer.Tracer, tt *TenantToken, limit, le int, matchPattern string) MetricNamesStatsResponse {
	if tt != nil {
		return s.metricsTracker.GetStatsForTenant(tt.AccountID, tt.ProjectID, limit, le, matchPattern)
	}
	res := s.metricsTracker.GetStats(limit, le, matchPattern)
	res.DeduplicateMergeRecords()
	return res
}

// ResetMetricNamesStats resets state for metric names usage tracker
func (s *Storage) ResetMetricNamesStats(_ *querytracer.Tracer) {
	s.metricsTracker.Reset(s.tsidCache.Reset)
}
