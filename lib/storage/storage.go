package storage

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
)

const maxRetentionMonths = 12 * 100

// Storage represents TSDB storage.
type Storage struct {
	// Atomic counters must go at the top of the structure in order to properly align by 8 bytes on 32-bit archs.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212 .
	tooSmallTimestampRows uint64
	tooBigTimestampRows   uint64

	addRowsConcurrencyLimitReached uint64
	addRowsConcurrencyLimitTimeout uint64
	addRowsConcurrencyDroppedRows  uint64

	slowRowInserts         uint64
	slowPerDayIndexInserts uint64
	slowMetricNameLoads    uint64

	path            string
	cachePath       string
	retentionMonths int

	// lock file for exclusive access to the storage on the given path.
	flockF *os.File

	idbCurr atomic.Value

	tb *table

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

	// metricIDs for pre-fetched metricNames in the prefetchMetricNames function.
	prefetchedMetricIDs atomic.Value

	stop chan struct{}

	currHourMetricIDsUpdaterWG   sync.WaitGroup
	nextDayMetricIDsUpdaterWG    sync.WaitGroup
	retentionWatcherWG           sync.WaitGroup
	prefetchedMetricIDsCleanerWG sync.WaitGroup

	// The snapshotLock prevents from concurrent creation of snapshots,
	// since this may result in snapshots without recently added data,
	// which may be in the process of flushing to disk by concurrently running
	// snapshot process.
	snapshotLock sync.Mutex
}

// OpenStorage opens storage on the given path with the given number of retention months.
func OpenStorage(path string, retentionMonths int) (*Storage, error) {
	if retentionMonths > maxRetentionMonths {
		return nil, fmt.Errorf("too big retentionMonths=%d; cannot exceed %d", retentionMonths, maxRetentionMonths)
	}
	if retentionMonths <= 0 {
		retentionMonths = maxRetentionMonths
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot determine absolute path for %q: %w", path, err)
	}

	s := &Storage{
		path:            path,
		cachePath:       path + "/cache",
		retentionMonths: retentionMonths,

		stop: make(chan struct{}),
	}

	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create a directory for the storage at %q: %w", path, err)
	}
	snapshotsPath := path + "/snapshots"
	if err := fs.MkdirAllIfNotExist(snapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", snapshotsPath, err)
	}

	// Protect from concurrent opens.
	flockF, err := fs.CreateFlockFile(path)
	if err != nil {
		return nil, err
	}
	s.flockF = flockF

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache("MetricName->TSID", "metricName_tsid", mem/3)
	s.metricIDCache = s.mustLoadCache("MetricID->TSID", "metricID_tsid", mem/16)
	s.metricNameCache = s.mustLoadCache("MetricID->MetricName", "metricID_metricName", mem/8)
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

	// Load indexdb
	idbPath := path + "/indexdb"
	idbSnapshotsPath := idbPath + "/snapshots"
	if err := fs.MkdirAllIfNotExist(idbSnapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %w", idbSnapshotsPath, err)
	}
	idbCurr, idbPrev, err := openIndexDBTables(idbPath, s.metricIDCache, s.metricNameCache, &s.currHourMetricIDs, &s.prevHourMetricIDs)
	if err != nil {
		return nil, fmt.Errorf("cannot open indexdb tables at %q: %w", idbPath, err)
	}
	idbCurr.SetExtDB(idbPrev)
	s.idbCurr.Store(idbCurr)

	// Load data
	tablePath := path + "/data"
	tb, err := openTable(tablePath, retentionMonths, s.getDeletedMetricIDs)
	if err != nil {
		s.idb().MustClose()
		return nil, fmt.Errorf("cannot open table at %q: %w", tablePath, err)
	}
	s.tb = tb

	s.startCurrHourMetricIDsUpdater()
	s.startNextDayMetricIDsUpdater()
	s.startRetentionWatcher()
	s.startPrefetchedMetricIDsCleaner()

	return s, nil
}

// debugFlush flushes recently added storage data, so it becomes visible to search.
func (s *Storage) debugFlush() {
	s.tb.flushRawRows()
	s.idb().tb.DebugFlush()
}

func (s *Storage) getDeletedMetricIDs() *uint64set.Set {
	return s.idb().getDeletedMetricIDs()
}

// CreateSnapshot creates snapshot for s and returns the snapshot name.
func (s *Storage) CreateSnapshot() (string, error) {
	logger.Infof("creating Storage snapshot for %q...", s.path)
	startTime := time.Now()

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	snapshotName := fmt.Sprintf("%s-%08X", time.Now().UTC().Format("20060102150405"), nextSnapshotIdx())
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

	idbSnapshot := fmt.Sprintf("%s/indexdb/snapshots/%s", s.path, snapshotName)
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

	fs.MustSyncPath(dstDir)
	fs.MustSyncPath(srcDir + "/snapshots")

	logger.Infof("created Storage snapshot for %q at %q in %.3f seconds", srcDir, dstDir, time.Since(startTime).Seconds())
	return snapshotName, nil
}

var snapshotNameRegexp = regexp.MustCompile("^[0-9]{14}-[0-9A-Fa-f]+$")

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
		if !snapshotNameRegexp.MatchString(fname) {
			continue
		}
		snapshotNames = append(snapshotNames, fname)
	}
	sort.Strings(snapshotNames)
	return snapshotNames, nil
}

// DeleteSnapshot deletes the given snapshot.
func (s *Storage) DeleteSnapshot(snapshotName string) error {
	if !snapshotNameRegexp.MatchString(snapshotName) {
		return fmt.Errorf("invalid snapshotName %q", snapshotName)
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

var snapshotIdx = uint64(time.Now().UnixNano())

func nextSnapshotIdx() uint64 {
	return atomic.AddUint64(&snapshotIdx, 1)
}

func (s *Storage) idb() *indexDB {
	return s.idbCurr.Load().(*indexDB)
}

// Metrics contains essential metrics for the Storage.
type Metrics struct {
	DedupsDuringMerge uint64

	TooSmallTimestampRows uint64
	TooBigTimestampRows   uint64

	AddRowsConcurrencyLimitReached uint64
	AddRowsConcurrencyLimitTimeout uint64
	AddRowsConcurrencyDroppedRows  uint64
	AddRowsConcurrencyCapacity     uint64
	AddRowsConcurrencyCurrent      uint64

	SlowRowInserts         uint64
	SlowPerDayIndexInserts uint64
	SlowMetricNameLoads    uint64

	TSIDCacheSize       uint64
	TSIDCacheSizeBytes  uint64
	TSIDCacheRequests   uint64
	TSIDCacheMisses     uint64
	TSIDCacheCollisions uint64

	MetricIDCacheSize       uint64
	MetricIDCacheSizeBytes  uint64
	MetricIDCacheRequests   uint64
	MetricIDCacheMisses     uint64
	MetricIDCacheCollisions uint64

	MetricNameCacheSize       uint64
	MetricNameCacheSizeBytes  uint64
	MetricNameCacheRequests   uint64
	MetricNameCacheMisses     uint64
	MetricNameCacheCollisions uint64

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

	IndexDBMetrics IndexDBMetrics
	TableMetrics   TableMetrics
}

// Reset resets m.
func (m *Metrics) Reset() {
	*m = Metrics{}
}

// UpdateMetrics updates m with metrics from s.
func (s *Storage) UpdateMetrics(m *Metrics) {
	m.DedupsDuringMerge = atomic.LoadUint64(&dedupsDuringMerge)

	m.TooSmallTimestampRows += atomic.LoadUint64(&s.tooSmallTimestampRows)
	m.TooBigTimestampRows += atomic.LoadUint64(&s.tooBigTimestampRows)

	m.AddRowsConcurrencyLimitReached += atomic.LoadUint64(&s.addRowsConcurrencyLimitReached)
	m.AddRowsConcurrencyLimitTimeout += atomic.LoadUint64(&s.addRowsConcurrencyLimitTimeout)
	m.AddRowsConcurrencyDroppedRows += atomic.LoadUint64(&s.addRowsConcurrencyDroppedRows)
	m.AddRowsConcurrencyCapacity = uint64(cap(addRowsConcurrencyCh))
	m.AddRowsConcurrencyCurrent = uint64(len(addRowsConcurrencyCh))

	m.SlowRowInserts += atomic.LoadUint64(&s.slowRowInserts)
	m.SlowPerDayIndexInserts += atomic.LoadUint64(&s.slowPerDayIndexInserts)
	m.SlowMetricNameLoads += atomic.LoadUint64(&s.slowMetricNameLoads)

	var cs fastcache.Stats
	s.tsidCache.UpdateStats(&cs)
	m.TSIDCacheSize += cs.EntriesCount
	m.TSIDCacheSizeBytes += cs.BytesSize
	m.TSIDCacheRequests += cs.GetCalls
	m.TSIDCacheMisses += cs.Misses
	m.TSIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricIDCache.UpdateStats(&cs)
	m.MetricIDCacheSize += cs.EntriesCount
	m.MetricIDCacheSizeBytes += cs.BytesSize
	m.MetricIDCacheRequests += cs.GetCalls
	m.MetricIDCacheMisses += cs.Misses
	m.MetricIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricNameCache.UpdateStats(&cs)
	m.MetricNameCacheSize += cs.EntriesCount
	m.MetricNameCacheSizeBytes += cs.BytesSize
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

	s.idb().UpdateMetrics(&m.IndexDBMetrics)
	s.tb.UpdateMetrics(&m.TableMetrics)
}

func (s *Storage) startPrefetchedMetricIDsCleaner() {
	s.prefetchedMetricIDsCleanerWG.Add(1)
	go func() {
		s.prefetchedMetricIDsCleaner()
		s.prefetchedMetricIDsCleanerWG.Done()
	}()
}

func (s *Storage) prefetchedMetricIDsCleaner() {
	t := time.NewTicker(7 * time.Minute)
	for {
		select {
		case <-s.stop:
			t.Stop()
			return
		case <-t.C:
			s.prefetchedMetricIDs.Store(&uint64set.Set{})
		}
	}
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
		d := nextRetentionDuration(s.retentionMonths)
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
	idbNew, err := openIndexDB(idbNewPath, s.metricIDCache, s.metricNameCache, &s.currHourMetricIDs, &s.prevHourMetricIDs)
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

	// Flush tsidCache, so idbNew can be populated with fresh data.
	s.tsidCache.Reset()

	// Flush dateMetricIDCache, so idbNew can be populated with fresh data.
	s.dateMetricIDCache.Reset()

	// Do not flush metricIDCache and metricNameCache, since all the metricIDs
	// from prev idb remain valid after the rotation.

	// There is no need in resetting nextDayMetricIDs, since it should be automatically reset every day.
}

// MustClose closes the storage.
func (s *Storage) MustClose() {
	close(s.stop)

	s.retentionWatcherWG.Wait()
	s.currHourMetricIDsUpdaterWG.Wait()
	s.nextDayMetricIDsUpdaterWG.Wait()

	s.tb.MustClose()
	s.idb().MustClose()

	// Save caches.
	s.mustSaveAndStopCache(s.tsidCache, "MetricName->TSID", "metricName_tsid")
	s.mustSaveAndStopCache(s.metricIDCache, "MetricID->TSID", "metricID_tsid")
	s.mustSaveAndStopCache(s.metricNameCache, "MetricID->MetricName", "metricID_metricName")

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

func (s *Storage) mustLoadCache(info, name string, sizeBytes int) *workingsetcache.Cache {
	path := s.cachePath + "/" + name
	logger.Infof("loading %s cache from %q...", info, path)
	startTime := time.Now()
	c := workingsetcache.Load(path, sizeBytes, time.Hour)
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	logger.Infof("loaded %s cache from %q in %.3f seconds; entriesCount: %d; sizeBytes: %d",
		info, path, time.Since(startTime).Seconds(), cs.EntriesCount, cs.BytesSize)
	return c
}

func (s *Storage) mustSaveAndStopCache(c *workingsetcache.Cache, info, name string) {
	path := s.cachePath + "/" + name
	logger.Infof("saving %s cache to %q...", info, path)
	startTime := time.Now()
	if err := c.Save(path); err != nil {
		logger.Panicf("FATAL: cannot save %s cache to %q: %s", info, path, err)
	}
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	c.Stop()
	logger.Infof("saved %s cache to %q in %.3f seconds; entriesCount: %d; sizeBytes: %d",
		info, path, time.Since(startTime).Seconds(), cs.EntriesCount, cs.BytesSize)
}

func nextRetentionDuration(retentionMonths int) time.Duration {
	t := time.Now().UTC()
	n := t.Year()*12 + int(t.Month()) - 1 + retentionMonths
	n -= n % retentionMonths
	y := n / 12
	m := time.Month((n % 12) + 1)
	// Schedule the deadline to +4 hours from the next retention period start.
	// This should prevent from possible double deletion of indexdb
	// due to time drift - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/248 .
	deadline := time.Date(y, m, 1, 4, 0, 0, 0, time.UTC)
	return deadline.Sub(t)
}

// searchTSIDs returns sorted TSIDs for the given tfss and the given tr.
func (s *Storage) searchTSIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]TSID, error) {
	// Do not cache tfss -> tsids here, since the caching is performed
	// on idb level.
	tsids, err := s.idb().searchTSIDs(tfss, tr, maxMetrics)
	if err != nil {
		return nil, fmt.Errorf("error when searching tsids for tfss %q: %w", tfss, err)
	}
	return tsids, nil
}

// prefetchMetricNames pre-fetches metric names for the given tsids into metricID->metricName cache.
//
// This should speed-up further searchMetricName calls for metricIDs from tsids.
func (s *Storage) prefetchMetricNames(tsids []TSID) error {
	var metricIDs uint64Sorter
	prefetchedMetricIDs := s.prefetchedMetricIDs.Load().(*uint64set.Set)
	for i := range tsids {
		metricID := tsids[i].MetricID
		if prefetchedMetricIDs.Has(metricID) {
			continue
		}
		metricIDs = append(metricIDs, metricID)
	}
	if len(metricIDs) < 500 {
		// It is cheaper to skip pre-fetching and obtain metricNames inline.
		return nil
	}
	atomic.AddUint64(&s.slowMetricNameLoads, uint64(len(metricIDs)))

	// Pre-fetch metricIDs.
	sort.Sort(metricIDs)
	var metricName []byte
	var err error
	idb := s.idb()
	is := idb.getIndexSearch()
	defer idb.putIndexSearch(is)
	for _, metricID := range metricIDs {
		metricName, err = is.searchMetricName(metricName[:0], metricID)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error in pre-fetching metricName for metricID=%d: %w", metricID, err)
		}
	}

	// Store the pre-fetched metricIDs, so they aren't pre-fetched next time.
	prefetchedMetricIDsNew := prefetchedMetricIDs.Clone()
	for _, metricID := range metricIDs {
		prefetchedMetricIDsNew.Add(metricID)
	}
	s.prefetchedMetricIDs.Store(prefetchedMetricIDsNew)
	return nil
}

// DeleteMetrics deletes all the metrics matching the given tfss.
//
// Returns the number of metrics deleted.
func (s *Storage) DeleteMetrics(tfss []*TagFilters) (int, error) {
	deletedCount, err := s.idb().DeleteTSIDs(tfss)
	if err != nil {
		return deletedCount, fmt.Errorf("cannot delete tsids: %w", err)
	}
	// Do not reset MetricName -> TSID cache (tsidCache), since the obtained
	// entries must be checked against deleted metricIDs.
	// See Storage.add for details.
	//
	// Do not reset MetricID -> MetricName cache, since it must be used only
	// after filtering out deleted metricIDs.
	return deletedCount, nil
}

// searchMetricName appends metric name for the given metricID to dst
// and returns the result.
func (s *Storage) searchMetricName(dst []byte, metricID uint64) ([]byte, error) {
	return s.idb().searchMetricName(dst, metricID)
}

// SearchTagKeys searches for tag keys
func (s *Storage) SearchTagKeys(maxTagKeys int) ([]string, error) {
	return s.idb().SearchTagKeys(maxTagKeys)
}

// SearchTagValues searches for tag values for the given tagKey
func (s *Storage) SearchTagValues(tagKey []byte, maxTagValues int) ([]string, error) {
	return s.idb().SearchTagValues(tagKey, maxTagValues)
}

// SearchTagEntries returns a list of (tagName -> tagValues)
func (s *Storage) SearchTagEntries(maxTagKeys, maxTagValues int) ([]TagEntry, error) {
	idb := s.idb()
	keys, err := idb.SearchTagKeys(maxTagKeys)
	if err != nil {
		return nil, fmt.Errorf("cannot search tag keys: %w", err)
	}

	// Sort keys for faster seeks below
	sort.Strings(keys)

	tes := make([]TagEntry, len(keys))
	for i, key := range keys {
		values, err := idb.SearchTagValues([]byte(key), maxTagValues)
		if err != nil {
			return nil, fmt.Errorf("cannot search values for tag %q: %w", key, err)
		}
		te := &tes[i]
		te.Key = key
		te.Values = values
	}
	return tes, nil
}

// TagEntry contains (tagName -> tagValues) mapping
type TagEntry struct {
	// Key is tagName
	Key string

	// Values contains all the values for Key.
	Values []string
}

// GetSeriesCount returns the approximate number of unique time series.
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (s *Storage) GetSeriesCount() (uint64, error) {
	return s.idb().GetSeriesCount()
}

// GetTSDBStatusForDate returns TSDB status data for /api/v1/status/tsdb.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
func (s *Storage) GetTSDBStatusForDate(date uint64, topN int) (*TSDBStatus, error) {
	return s.idb().GetTSDBStatusForDate(date, topN)
}

// MetricRow is a metric to insert into storage.
type MetricRow struct {
	// MetricNameRaw contains raw metric name, which must be decoded
	// with MetricName.unmarshalRaw.
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
	if err := mn.unmarshalRaw(mr.MetricNameRaw); err == nil {
		metricName = mn.String()
	}
	return fmt.Sprintf("MetricName=%s, Timestamp=%d, Value=%f\n", metricName, mr.Timestamp, mr.Value)
}

// Marshal appends marshaled mr to dst and returns the result.
func (mr *MetricRow) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, mr.MetricNameRaw)
	dst = encoding.MarshalUint64(dst, uint64(mr.Timestamp))
	dst = encoding.MarshalUint64(dst, math.Float64bits(mr.Value))
	return dst
}

// Unmarshal unmarshals mr from src and returns the remaining tail from src.
func (mr *MetricRow) Unmarshal(src []byte) ([]byte, error) {
	tail, metricNameRaw, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricName: %w", err)
	}
	mr.MetricNameRaw = append(mr.MetricNameRaw[:0], metricNameRaw...)

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

// AddRows adds the given mrs to s.
func (s *Storage) AddRows(mrs []MetricRow, precisionBits uint8) error {
	if len(mrs) == 0 {
		return nil
	}

	// Limit the number of concurrent goroutines that may add rows to the storage.
	// This should prevent from out of memory errors and CPU trashing when too many
	// goroutines call AddRows.
	select {
	case addRowsConcurrencyCh <- struct{}{}:
		defer func() { <-addRowsConcurrencyCh }()
	default:
		// Sleep for a while until giving up
		atomic.AddUint64(&s.addRowsConcurrencyLimitReached, 1)
		t := timerpool.Get(addRowsTimeout)
		select {
		case addRowsConcurrencyCh <- struct{}{}:
			timerpool.Put(t)
			defer func() { <-addRowsConcurrencyCh }()
		case <-t.C:
			timerpool.Put(t)
			atomic.AddUint64(&s.addRowsConcurrencyLimitTimeout, 1)
			atomic.AddUint64(&s.addRowsConcurrencyDroppedRows, uint64(len(mrs)))
			return fmt.Errorf("Cannot add %d rows to storage in %s, since it is overloaded with %d concurrent writers. Add more CPUs or reduce load",
				len(mrs), addRowsTimeout, cap(addRowsConcurrencyCh))
		}
	}

	// Add rows to the storage.
	var err error
	rr := getRawRowsWithSize(len(mrs))
	rr.rows, err = s.add(rr.rows, mrs, precisionBits)
	putRawRows(rr)

	return err
}

var (
	addRowsConcurrencyCh = make(chan struct{}, runtime.GOMAXPROCS(-1)*2)
	addRowsTimeout       = 30 * time.Second
)

func (s *Storage) add(rows []rawRow, mrs []MetricRow, precisionBits uint8) ([]rawRow, error) {
	idb := s.idb()
	dmis := idb.getDeletedMetricIDs()
	rowsLen := len(rows)
	if n := rowsLen + len(mrs) - cap(rows); n > 0 {
		rows = append(rows[:cap(rows)], make([]rawRow, n)...)
	}
	rows = rows[:rowsLen+len(mrs)]
	j := 0
	var (
		// These vars are used for speeding up bulk imports of multiple adjancent rows for the same metricName.
		prevTSID          TSID
		prevMetricNameRaw []byte
	)
	var pmrs *pendingMetricRows
	minTimestamp, maxTimestamp := s.tb.getMinMaxTimestamps()
	// Return only the first error, since it has no sense in returning all errors.
	var firstWarn error
	for i := range mrs {
		mr := &mrs[i]
		if math.IsNaN(mr.Value) {
			// Just skip NaNs, since the underlying encoding
			// doesn't know how to work with them.
			continue
		}
		if mr.Timestamp < minTimestamp {
			// Skip rows with too small timestamps outside the retention.
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot insert row with too small timestamp %d outside the retention; minimum allowed timestamp is %d",
					mr.Timestamp, minTimestamp)
			}
			atomic.AddUint64(&s.tooSmallTimestampRows, 1)
			continue
		}
		if mr.Timestamp > maxTimestamp {
			// Skip rows with too big timestamps significantly exceeding the current time.
			if firstWarn == nil {
				firstWarn = fmt.Errorf("cannot insert row with too big timestamp %d exceeding the current time; maximum allowd timestamp is %d",
					mr.Timestamp, maxTimestamp)
			}
			atomic.AddUint64(&s.tooBigTimestampRows, 1)
			continue
		}
		r := &rows[rowsLen+j]
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
		if s.getTSIDFromCache(&r.TSID, mr.MetricNameRaw) && !dmis.Has(r.TSID.MetricID) {
			// Fast path - the TSID for the given MetricName has been found in cache and isn't deleted.
			prevTSID = r.TSID
			prevMetricNameRaw = mr.MetricNameRaw
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
		atomic.AddUint64(&s.slowRowInserts, uint64(len(pmrs.pmrs)))
		// Sort pendingMetricRows by canonical metric name in order to speed up search via `is` in the loop below.
		pendingMetricRows := pmrs.pmrs
		sort.Slice(pendingMetricRows, func(i, j int) bool {
			return string(pendingMetricRows[i].MetricName) < string(pendingMetricRows[j].MetricName)
		})
		is := idb.getIndexSearch()
		prevMetricNameRaw = nil
		for i := range pendingMetricRows {
			pmr := &pendingMetricRows[i]
			mr := &pmr.mr
			r := &rows[rowsLen+j]
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
			if s.getTSIDFromCache(&r.TSID, mr.MetricNameRaw) && !dmis.Has(r.TSID.MetricID) {
				// Fast path - the TSID for the given MetricName has been found in cache and isn't deleted.
				prevTSID = r.TSID
				prevMetricNameRaw = mr.MetricNameRaw
				continue
			}
			if err := is.GetOrCreateTSIDByName(&r.TSID, pmr.MetricName); err != nil {
				// Do not stop adding rows on error - just skip invalid row.
				// This guarantees that invalid rows don't prevent
				// from adding valid rows into the storage.
				if firstWarn == nil {
					firstWarn = fmt.Errorf("cannot obtain or create TSID for MetricName %q: %w", pmr.MetricName, err)
				}
				j--
				continue
			}
			s.putTSIDToCache(&r.TSID, mr.MetricNameRaw)
		}
		idb.putIndexSearch(is)
		putPendingMetricRows(pmrs)
	}
	if firstWarn != nil {
		logger.Errorf("warn occurred during rows addition: %s", firstWarn)
	}
	rows = rows[:rowsLen+j]

	var firstError error
	if err := s.tb.AddRows(rows); err != nil {
		firstError = fmt.Errorf("cannot add rows to table: %w", err)
	}
	if err := s.updatePerDateData(rows); err != nil && firstError == nil {
		firstError = fmt.Errorf("cannot update per-date data: %w", err)
	}
	if firstError != nil {
		return rows, fmt.Errorf("error occurred during rows addition: %w", firstError)
	}
	return rows, nil
}

type pendingMetricRow struct {
	MetricName []byte
	mr         MetricRow
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
		pmr.mr.MetricNameRaw = nil
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
		if err := pmrs.mn.unmarshalRaw(mr.MetricNameRaw); err != nil {
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
		mr:         *mr,
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

func (s *Storage) updatePerDateData(rows []rawRow) error {
	var date uint64
	var hour uint64
	var prevTimestamp int64
	var (
		// These vars are used for speeding up bulk imports when multiple adjancent rows
		// contain the same (metricID, date) pairs.
		prevDate     uint64
		prevMetricID uint64
	)
	hm := s.currHourMetricIDs.Load().(*hourMetricIDs)
	nextDayMetricIDs := &s.nextDayMetricIDs.Load().(*byDateMetricIDEntry).v
	todayShare16bit := uint64((float64(fasttime.UnixTimestamp()%(3600*24)) / (3600 * 24)) * (1 << 16))
	type pendingDateMetricID struct {
		date     uint64
		metricID uint64
	}
	var pendingDateMetricIDs []pendingDateMetricID
	for i := range rows {
		r := &rows[i]
		if r.Timestamp != prevTimestamp {
			date = uint64(r.Timestamp) / msecPerDay
			hour = uint64(r.Timestamp) / msecPerHour
			prevTimestamp = r.Timestamp
		}
		metricID := r.TSID.MetricID
		if hour == hm.hour {
			// The r belongs to the current hour. Check for the current hour cache.
			if hm.m.Has(metricID) {
				// Fast path: the metricID is in the current hour cache.
				// This means the metricID has been already added to per-day inverted index.

				// Gradually pre-populate per-day inverted index for the next day
				// during the current day.
				// This should reduce CPU usage spike and slowdown at the beginning of the next day
				// when entries for all the active time series must be added to the index.
				// This should address https://github.com/VictoriaMetrics/VictoriaMetrics/issues/430 .
				if todayShare16bit > (metricID&(1<<16-1)) && !nextDayMetricIDs.Has(metricID) {
					pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
						date:     date + 1,
						metricID: metricID,
					})
					s.pendingNextDayMetricIDsLock.Lock()
					s.pendingNextDayMetricIDs.Add(metricID)
					s.pendingNextDayMetricIDsLock.Unlock()
				}
				continue
			}
			s.pendingHourEntriesLock.Lock()
			s.pendingHourEntries.Add(metricID)
			s.pendingHourEntriesLock.Unlock()
		}

		// Slower path: check global cache for (date, metricID) entry.
		if metricID == prevMetricID && date == prevDate {
			// Fast path for bulk import of multiple rows with the same (date, metricID) pairs.
			continue
		}
		prevDate = date
		prevMetricID = metricID

		if !s.dateMetricIDCache.Has(date, metricID) {
			// Slow path: store the (date, metricID) entry in the indexDB.
			// It is OK if the (date, metricID) entry is added multiple times to db
			// by concurrent goroutines.
			pendingDateMetricIDs = append(pendingDateMetricIDs, pendingDateMetricID{
				date:     date,
				metricID: metricID,
			})
		}
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
	is := idb.getIndexSearch()
	defer idb.putIndexSearch(is)
	var firstError error
	prevMetricID = 0
	prevDate = 0
	for _, dateMetricID := range pendingDateMetricIDs {
		date := dateMetricID.date
		metricID := dateMetricID.metricID
		if metricID == prevMetricID && date == prevDate {
			// Fast path for bulk import of multiple rows with the same (date, metricID) pairs.
			continue
		}
		prevDate = date
		prevMetricID = metricID

		if s.dateMetricIDCache.Has(date, metricID) {
			// The metricID has been already added to per-day inverted index.
			continue
		}
		ok, err := is.hasDateMetricID(date, metricID)
		if err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("error when locating (date=%d, metricID=%d) in database: %w", date, metricID, err)
			}
			continue
		}
		if !ok {
			// The (date, metricID) entry is missing in the indexDB. Add it there.
			if err := is.storeDateMetricID(date, metricID); err != nil {
				if firstError == nil {
					firstError = fmt.Errorf("error when storing (date=%d, metricID=%d) in database: %w", date, metricID, err)
				}
				continue
			}
		}
		// The metric must be added to cache only after it has been successfully added to indexDB.
		s.dateMetricIDCache.Set(date, metricID)
	}
	return firstError
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
	byDateMutable *byDateMetricIDMap
	lastSyncTime  uint64
	mu            sync.Mutex
}

func newDateMetricIDCache() *dateMetricIDCache {
	var dmc dateMetricIDCache
	dmc.Reset()
	return &dmc
}

func (dmc *dateMetricIDCache) Reset() {
	dmc.mu.Lock()
	// Do not reset syncsCount and resetsCount
	dmc.byDate.Store(newByDateMetricIDMap())
	dmc.byDateMutable = newByDateMetricIDMap()
	dmc.lastSyncTime = fasttime.UnixTimestamp()
	dmc.mu.Unlock()

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
	currentTime := fasttime.UnixTimestamp()
	dmc.mu.Lock()
	v = dmc.byDateMutable.get(date)
	ok := v.Has(metricID)
	mustSync := false
	if currentTime-dmc.lastSyncTime > 10 {
		mustSync = true
		dmc.lastSyncTime = currentTime
	}
	dmc.mu.Unlock()

	if mustSync {
		dmc.sync()
	}
	return ok
}

func (dmc *dateMetricIDCache) Set(date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(date)
	v.Add(metricID)
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) sync() {
	dmc.mu.Lock()
	byDate := dmc.byDate.Load().(*byDateMetricIDMap)
	for date, e := range dmc.byDateMutable.m {
		v := byDate.get(date)
		e.v.Union(v)
	}
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()
	dmc.mu.Unlock()

	atomic.AddUint64(&dmc.syncsCount, 1)

	if dmc.EntriesCount() > memory.Allowed()/128 {
		dmc.Reset()
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

func (s *Storage) getTSIDFromCache(dst *TSID, metricName []byte) bool {
	buf := (*[unsafe.Sizeof(*dst)]byte)(unsafe.Pointer(dst))[:]
	buf = s.tsidCache.Get(buf[:0], metricName)
	return uintptr(len(buf)) == unsafe.Sizeof(*dst)
}

func (s *Storage) putTSIDToCache(tsid *TSID, metricName []byte) {
	buf := (*[unsafe.Sizeof(*tsid)]byte)(unsafe.Pointer(tsid))[:]
	s.tsidCache.Set(metricName, buf)
}

func openIndexDBTables(path string, metricIDCache, metricNameCache *workingsetcache.Cache, currHourMetricIDs, prevHourMetricIDs *atomic.Value) (curr, prev *indexDB, err error) {
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

	curr, err = openIndexDB(currPath, metricIDCache, metricNameCache, currHourMetricIDs, prevHourMetricIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open curr indexdb table at %q: %w", currPath, err)
	}
	prevPath := path + "/" + tableNames[len(tableNames)-2]
	prev, err = openIndexDB(prevPath, metricIDCache, metricNameCache, currHourMetricIDs, prevHourMetricIDs)
	if err != nil {
		curr.MustClose()
		return nil, nil, fmt.Errorf("cannot open prev indexdb table at %q: %w", prevPath, err)
	}

	// Adjust startDateForPerDayInvertedIndex for the previous index.
	if prev.startDateForPerDayInvertedIndex > curr.startDateForPerDayInvertedIndex {
		prev.startDateForPerDayInvertedIndex = curr.startDateForPerDayInvertedIndex
	}

	return curr, prev, nil
}

var indexDBTableNameRegexp = regexp.MustCompile("^[0-9A-F]{16}$")

func nextIndexDBTableName() string {
	n := atomic.AddUint64(&indexDBTableIdx, 1)
	return fmt.Sprintf("%016X", n)
}

var indexDBTableIdx = uint64(time.Now().UnixNano())
