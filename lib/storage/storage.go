package storage

import (
	"fmt"
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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/fastcache"
	"golang.org/x/sys/unix"
)

const maxRetentionMonths = 12 * 100

// Storage represents TSDB storage.
type Storage struct {
	path            string
	cachePath       string
	retentionMonths int

	// lock file for excluse access to the storage on the given path.
	flockF *os.File

	idbCurr atomic.Value

	tb *table

	// tsidCache is MetricName -> TSID cache.
	tsidCache *fastcache.Cache

	// metricIDCache is MetricID -> TSID cache.
	metricIDCache *fastcache.Cache

	// metricNameCache is MetricID -> MetricName cache.
	metricNameCache *fastcache.Cache

	// dateMetricIDCache is (Date, MetricID) cache.
	dateMetricIDCache *fastcache.Cache

	stop               chan struct{}
	retentionWatcherWG sync.WaitGroup
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
		return nil, fmt.Errorf("cannot determine absolute path for %q: %s", path, err)
	}

	s := &Storage{
		path:            path,
		cachePath:       path + "/cache",
		retentionMonths: retentionMonths,

		stop: make(chan struct{}),
	}

	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, fmt.Errorf("cannot create a directory for the storage at %q: %s", path, err)
	}
	snapshotsPath := path + "/snapshots"
	if err := fs.MkdirAllIfNotExist(snapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %s", snapshotsPath, err)
	}

	flockFile := path + "/flock.lock"
	flockF, err := os.Create(flockFile)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %s", flockFile, err)
	}
	if err := unix.Flock(int(flockF.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %s", flockFile, err)
	}
	s.flockF = flockF

	// Load caches.
	mem := memory.Allowed()
	s.tsidCache = s.mustLoadCache("MetricName->TSID", "metricName_tsid", mem/3)
	s.metricIDCache = s.mustLoadCache("MetricID->TSID", "metricID_tsid", mem/16)
	s.metricNameCache = s.mustLoadCache("MetricID->MetricName", "metricID_metricName", mem/8)
	s.dateMetricIDCache = s.mustLoadCache("Date->MetricID", "date_metricID", mem/32)

	// Load indexdb
	idbPath := path + "/indexdb"
	idbSnapshotsPath := idbPath + "/snapshots"
	if err := fs.MkdirAllIfNotExist(idbSnapshotsPath); err != nil {
		return nil, fmt.Errorf("cannot create %q: %s", idbSnapshotsPath, err)
	}
	idbCurr, idbPrev, err := openIndexDBTables(idbPath, s.metricIDCache, s.metricNameCache)
	if err != nil {
		return nil, fmt.Errorf("cannot open indexdb tables at %q: %s", idbPath, err)
	}
	idbCurr.SetExtDB(idbPrev)
	s.idbCurr.Store(idbCurr)

	// Load data
	tablePath := path + "/data"
	tb, err := openTable(tablePath, retentionMonths, s.getDeletedMetricIDs)
	if err != nil {
		s.idb().MustClose()
		return nil, fmt.Errorf("cannot open table at %q: %s", tablePath, err)
	}
	s.tb = tb

	s.startRetentionWatcher()

	return s, nil
}

// debugFlush flushes recently added storage data, so it becomes visible to search.
func (s *Storage) debugFlush() {
	s.tb.flushRawRows()
	s.idb().tb.DebugFlush()
}

func (s *Storage) getDeletedMetricIDs() map[uint64]struct{} {
	return s.idb().getDeletedMetricIDs()
}

// CreateSnapshot creates snapshot for s and returns the snapshot name.
func (s *Storage) CreateSnapshot() (string, error) {
	logger.Infof("creating Storage snapshot for %q...", s.path)
	startTime := time.Now()

	snapshotName := fmt.Sprintf("%s-%08X", time.Now().UTC().Format("20060102150405"), nextSnapshotIdx())
	srcDir := s.path
	dstDir := fmt.Sprintf("%s/snapshots/%s", srcDir, snapshotName)
	if err := fs.MkdirAllFailIfExist(dstDir); err != nil {
		return "", fmt.Errorf("cannot create dir %q: %s", dstDir, err)
	}
	dstDataDir := dstDir + "/data"
	if err := fs.MkdirAllFailIfExist(dstDataDir); err != nil {
		return "", fmt.Errorf("cannot create dir %q: %s", dstDataDir, err)
	}

	smallDir, bigDir, err := s.tb.CreateSnapshot(snapshotName)
	if err != nil {
		return "", fmt.Errorf("cannot create table snapshot: %s", err)
	}
	dstSmallDir := dstDataDir + "/small"
	if err := fs.SymlinkRelative(smallDir, dstSmallDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %s", smallDir, dstSmallDir, err)
	}
	dstBigDir := dstDataDir + "/big"
	if err := fs.SymlinkRelative(bigDir, dstBigDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %s", bigDir, dstBigDir, err)
	}
	fs.SyncPath(dstDataDir)

	idbSnapshot := fmt.Sprintf("%s/indexdb/snapshots/%s", s.path, snapshotName)
	idb := s.idb()
	currSnapshot := idbSnapshot + "/" + idb.name
	if err := idb.tb.CreateSnapshotAt(currSnapshot); err != nil {
		return "", fmt.Errorf("cannot create curr indexDB snapshot: %s", err)
	}
	ok := idb.doExtDB(func(extDB *indexDB) {
		prevSnapshot := idbSnapshot + "/" + extDB.name
		err = extDB.tb.CreateSnapshotAt(prevSnapshot)
	})
	if ok && err != nil {
		return "", fmt.Errorf("cannot create prev indexDB snapshot: %s", err)
	}
	dstIdbDir := dstDir + "/indexdb"
	if err := fs.SymlinkRelative(idbSnapshot, dstIdbDir); err != nil {
		return "", fmt.Errorf("cannot create symlink from %q to %q: %s", idbSnapshot, dstIdbDir, err)
	}

	fs.SyncPath(dstDir)
	fs.SyncPath(srcDir + "/snapshots")

	logger.Infof("created Storage snapshot for %q at %q in %s", srcDir, dstDir, time.Since(startTime))
	return snapshotName, nil
}

var snapshotNameRegexp = regexp.MustCompile("^[0-9]{14}-[0-9A-Fa-f]+$")

// ListSnapshots returns sorted list of existing snapshots for s.
func (s *Storage) ListSnapshots() ([]string, error) {
	snapshotsPath := s.path + "/snapshots"
	d, err := os.Open(snapshotsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %s", snapshotsPath, err)
	}
	defer fs.MustClose(d)

	fnames, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("cannot read contents of %q: %s", snapshotsPath, err)
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
	fs.MustRemoveAllSynced(idbPath)
	fs.MustRemoveAllSynced(snapshotPath)

	logger.Infof("deleted snapshot %q in %s", snapshotPath, time.Since(startTime))

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
	TSIDCacheSize       uint64
	TSIDCacheBytesSize  uint64
	TSIDCacheRequests   uint64
	TSIDCacheMisses     uint64
	TSIDCacheCollisions uint64

	MetricIDCacheSize       uint64
	MetricIDCacheBytesSize  uint64
	MetricIDCacheRequests   uint64
	MetricIDCacheMisses     uint64
	MetricIDCacheCollisions uint64

	MetricNameCacheSize       uint64
	MetricNameCacheBytesSize  uint64
	MetricNameCacheRequests   uint64
	MetricNameCacheMisses     uint64
	MetricNameCacheCollisions uint64

	DateMetricIDCacheSize       uint64
	DateMetricIDCacheBytesSize  uint64
	DateMetricIDCacheRequests   uint64
	DateMetricIDCacheMisses     uint64
	DateMetricIDCacheCollisions uint64

	IndexDBMetrics IndexDBMetrics
	TableMetrics   TableMetrics
}

// Reset resets m.
func (m *Metrics) Reset() {
	*m = Metrics{}
}

// UpdateMetrics updates m with metrics from s.
func (s *Storage) UpdateMetrics(m *Metrics) {
	var cs fastcache.Stats
	s.tsidCache.UpdateStats(&cs)
	m.TSIDCacheSize += cs.EntriesCount
	m.TSIDCacheBytesSize += cs.BytesSize
	m.TSIDCacheRequests += cs.GetCalls
	m.TSIDCacheMisses += cs.Misses
	m.TSIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricIDCache.UpdateStats(&cs)
	m.MetricIDCacheSize += cs.EntriesCount
	m.MetricIDCacheBytesSize += cs.BytesSize
	m.MetricIDCacheRequests += cs.GetCalls
	m.MetricIDCacheMisses += cs.Misses
	m.MetricIDCacheCollisions += cs.Collisions

	cs.Reset()
	s.metricNameCache.UpdateStats(&cs)
	m.MetricNameCacheSize += cs.EntriesCount
	m.MetricNameCacheBytesSize += cs.BytesSize
	m.MetricNameCacheRequests += cs.GetCalls
	m.MetricNameCacheMisses += cs.Misses
	m.MetricNameCacheCollisions += cs.Collisions

	cs.Reset()
	s.dateMetricIDCache.UpdateStats(&cs)
	m.DateMetricIDCacheSize += cs.EntriesCount
	m.DateMetricIDCacheBytesSize += cs.BytesSize
	m.DateMetricIDCacheRequests += cs.GetCalls
	m.DateMetricIDCacheMisses += cs.Misses
	m.DateMetricIDCacheCollisions += cs.Collisions

	s.idb().UpdateMetrics(&m.IndexDBMetrics)
	s.tb.UpdateMetrics(&m.TableMetrics)
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

func (s *Storage) mustRotateIndexDB() {
	// Create new indexdb table.
	newTableName := nextIndexDBTableName()
	idbNewPath := s.path + "/indexdb/" + newTableName
	idbNew, err := openIndexDB(idbNewPath, s.metricIDCache, s.metricNameCache)
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
	fs.SyncPath(s.path)

	// Flush tsidCache, so idbNew can be populated with fresh data.
	s.tsidCache.Reset()

	// Flush dateMetricIDCache, so idbNew can be populated with fresh data.
	s.dateMetricIDCache.Reset()

	// Do not flush metricIDCache and metricNameCache, since all the metricIDs
	// from prev idb remain valid after the rotation.
}

// MustClose closes the storage.
func (s *Storage) MustClose() {
	close(s.stop)

	s.retentionWatcherWG.Wait()

	s.tb.MustClose()
	s.idb().MustClose()

	// Save caches.
	s.mustSaveCache(s.tsidCache, "MetricName->TSID", "metricName_tsid")
	s.mustSaveCache(s.metricIDCache, "MetricID->TSID", "metricID_tsid")
	s.mustSaveCache(s.metricNameCache, "MetricID->MetricName", "metricID_metricName")
	s.mustSaveCache(s.dateMetricIDCache, "Date->MetricID", "date_metricID")

	// Release lock file.
	if err := s.flockF.Close(); err != nil {
		logger.Panicf("FATAL: cannot close lock file %q: %s", s.flockF.Name(), err)
	}
}

func (s *Storage) mustLoadCache(info, name string, bytesSize int) *fastcache.Cache {
	path := s.cachePath + "/" + name
	logger.Infof("loading %s cache from %q...", info, path)
	startTime := time.Now()
	c := fastcache.LoadFromFileOrNew(path, bytesSize)
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	logger.Infof("loaded %s cache from %q in %s; entriesCount: %d; bytesSize: %d",
		info, path, time.Since(startTime), cs.EntriesCount, cs.BytesSize)
	return c
}

func (s *Storage) mustSaveCache(c *fastcache.Cache, info, name string) {
	gomaxprocs := runtime.GOMAXPROCS(-1)
	path := s.cachePath + "/" + name
	logger.Infof("saving %s cache to %q...", info, path)
	startTime := time.Now()
	if err := c.SaveToFileConcurrent(path, gomaxprocs); err != nil {
		logger.Panicf("FATAL: cannot save %s cache to %q: %s", info, path, err)
	}
	var cs fastcache.Stats
	c.UpdateStats(&cs)
	c.Reset()
	logger.Infof("saved %s cache to %q in %s; entriesCount: %d; bytesSize: %d",
		info, path, time.Since(startTime), cs.EntriesCount, cs.BytesSize)
}

func nextRetentionDuration(retentionMonths int) time.Duration {
	t := time.Now().UTC()
	n := t.Year()*12 + int(t.Month()) - 1 + retentionMonths
	n -= n % retentionMonths
	y := n / 12
	m := time.Month((n % 12) + 1)
	deadline := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	return deadline.Sub(t)
}

// searchTSIDs returns TSIDs for the given tfss and the given tr.
func (s *Storage) searchTSIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]TSID, error) {
	// Do not cache tfss -> tsids here, since the caching is performed
	// on idb level.
	tsids, err := s.idb().searchTSIDs(tfss, tr, maxMetrics)
	if err != nil {
		return nil, fmt.Errorf("error when searching tsids for tfss %q: %s", tfss, err)
	}
	return tsids, nil
}

// DeleteMetrics deletes all the metrics matching the given tfss.
//
// Returns the number of metrics deleted.
func (s *Storage) DeleteMetrics(tfss []*TagFilters) (int, error) {
	deletedCount, err := s.idb().DeleteTSIDs(tfss)
	if err != nil {
		return deletedCount, fmt.Errorf("cannot delete tsids: %s", err)
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
func (s *Storage) searchMetricName(dst []byte, metricID uint64, accountID, projectID uint32) ([]byte, error) {
	return s.idb().searchMetricName(dst, metricID, accountID, projectID)
}

// SearchTagKeys searches for tag keys for the given (accountID, projectID).
func (s *Storage) SearchTagKeys(accountID, projectID uint32, maxTagKeys int) ([]string, error) {
	return s.idb().SearchTagKeys(accountID, projectID, maxTagKeys)
}

// SearchTagValues searches for tag values for the given tagKey in (accountID, projectID).
func (s *Storage) SearchTagValues(accountID, projectID uint32, tagKey []byte, maxTagValues int) ([]string, error) {
	return s.idb().SearchTagValues(accountID, projectID, tagKey, maxTagValues)
}

// GetSeriesCount returns the approximate number of unique time series for the given (accountID, projectID).
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (s *Storage) GetSeriesCount(accountID, projectID uint32) (uint64, error) {
	return s.idb().GetSeriesCount(accountID, projectID)
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
	return MarshalMetricRow(dst, mr.MetricNameRaw, mr.Timestamp, mr.Value)
}

// MarshalMetricRow marshals MetricRow data to dst and returns the result.
func MarshalMetricRow(dst []byte, metricNameRaw []byte, timestamp int64, value float64) []byte {
	dst = encoding.MarshalBytes(dst, metricNameRaw)
	dst = encoding.MarshalUint64(dst, uint64(timestamp))
	dst = encoding.MarshalUint64(dst, math.Float64bits(value))
	return dst
}

// Unmarshal unmarshals mr from src and returns the remaining tail from src.
func (mr *MetricRow) Unmarshal(src []byte) ([]byte, error) {
	tail, metricNameRaw, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricName: %s", err)
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
	t := time.NewTimer(addRowsTimeout)
	select {
	case addRowsConcurrencyCh <- struct{}{}:
		t.Stop()
		defer func() { <-addRowsConcurrencyCh }()
	case <-t.C:
		return fmt.Errorf("Cannot add %d rows to storage in %s, since it is overloaded with %d concurrent writers. Add more CPUs or reduce load",
			len(mrs), addRowsTimeout, cap(addRowsConcurrencyCh))
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
	var errors []error
	var is *indexSearch
	var mn *MetricName
	var kb *bytesutil.ByteBuffer

	idb := s.idb()
	dmis := idb.getDeletedMetricIDs()
	rowsLen := len(rows)
	if n := rowsLen + len(mrs) - cap(rows); n > 0 {
		rows = append(rows[:cap(rows)], make([]rawRow, n)...)
	}
	rows = rows[:rowsLen+len(mrs)]
	j := 0
	for i := range mrs {
		mr := &mrs[i]
		if math.IsNaN(mr.Value) {
			// Just skip NaNs, since the underlying encoding
			// doesn't know how to work with them.
			continue
		}
		r := &rows[rowsLen+j]
		j++
		r.Timestamp = mr.Timestamp
		r.Value = mr.Value
		r.PrecisionBits = precisionBits
		if s.getTSIDFromCache(&r.TSID, mr.MetricNameRaw) {
			if len(dmis) == 0 {
				// Fast path - the TSID for the given MetricName has been found in cache and isn't deleted.
				continue
			}
			if _, deleted := dmis[r.TSID.MetricID]; !deleted {
				// Fast path - the TSID for the given MetricName has been found in cache and isn't deleted.
				continue
			}
		}

		// Slow path - the TSID is missing in the cache. Search for it in the index.
		if is == nil {
			is = idb.getIndexSearch()
			mn = GetMetricName()
			kb = kbPool.Get()
		}
		if err := mn.unmarshalRaw(mr.MetricNameRaw); err != nil {
			// Do not stop adding rows on error - just skip invalid row.
			// This guarantees that invalid rows don't prevent
			// from adding valid rows into the storage.
			err = fmt.Errorf("cannot unmarshal MetricNameRaw %q: %s", mr.MetricNameRaw, err)
			errors = append(errors, err)
			j--
			continue
		}
		mn.sortTags()
		kb.B = mn.Marshal(kb.B[:0])
		if err := is.GetOrCreateTSIDByName(&r.TSID, kb.B); err != nil {
			// Do not stop adding rows on error - just skip invalid row.
			// This guarantees that invalid rows don't prevent
			// from adding valid rows into the storage.
			err = fmt.Errorf("cannot obtain TSID for MetricName %q: %s", kb.B, err)
			errors = append(errors, err)
			j--
			continue
		}
		s.putTSIDToCache(&r.TSID, mr.MetricNameRaw)
	}
	if is != nil {
		kbPool.Put(kb)
		PutMetricName(mn)
		idb.putIndexSearch(is)
	}
	rows = rows[:rowsLen+j]

	if err := s.tb.AddRows(rows); err != nil {
		err = fmt.Errorf("cannot add rows to table: %s", err)
		errors = append(errors, err)
	}
	errors = s.updateDateMetricIDCache(rows, errors)
	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		return rows, fmt.Errorf("errors occured during rows addition: %s", errors[0])
	}
	return rows, nil
}

func (s *Storage) updateDateMetricIDCache(rows []rawRow, errors []error) []error {
	var date uint64
	var prevTimestamp int64
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	kb.B = bytesutil.Resize(kb.B, 16)
	keyBuf := kb.B
	a := (*[2]uint64)(unsafe.Pointer(&keyBuf[0]))
	idb := s.idb()
	for i := range rows {
		r := &rows[i]
		if r.Timestamp != prevTimestamp {
			date = uint64(r.Timestamp) / msecPerDay
			prevTimestamp = r.Timestamp
		}
		metricID := r.TSID.MetricID
		a[0] = date
		a[1] = metricID
		if s.dateMetricIDCache.Has(keyBuf) {
			// Fast path: the (date, metricID) entry is in the cache.
			continue
		}

		// Slow path: store the entry in the cache and in the indexDB.
		// It is OK if the (date, metricID) entry is added multiple times to db
		// by concurrent goroutines.
		s.dateMetricIDCache.Set(keyBuf, nil)
		if err := idb.storeDateMetricID(date, metricID, r.TSID.AccountID, r.TSID.ProjectID); err != nil {
			errors = append(errors, err)
			continue
		}
	}
	return errors
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

func openIndexDBTables(path string, metricIDCache, metricNameCache *fastcache.Cache) (curr, prev *indexDB, err error) {
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return nil, nil, fmt.Errorf("cannot create directory %q: %s", path, err)
	}

	d, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open directory: %s", err)
	}
	defer fs.MustClose(d)

	// Search for the two most recent tables - the last one is active,
	// the previous one contains backup data.
	fis, err := d.Readdir(-1)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read directory: %s", err)
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
		if err := os.RemoveAll(pathToRemove); err != nil {
			return nil, nil, fmt.Errorf("cannot remove obsolete indexdb dir %q: %s", pathToRemove, err)
		}
		logger.Infof("removed obsolete indexdb dir %q", pathToRemove)
	}

	// Persist changes on the file system.
	fs.SyncPath(path)

	// Open the last two tables.
	currPath := path + "/" + tableNames[len(tableNames)-1]

	curr, err = openIndexDB(currPath, metricIDCache, metricNameCache)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open curr indexdb table at %q: %s", currPath, err)
	}
	prevPath := path + "/" + tableNames[len(tableNames)-2]
	prev, err = openIndexDB(prevPath, metricIDCache, metricNameCache)
	if err != nil {
		curr.MustClose()
		return nil, nil, fmt.Errorf("cannot open prev indexdb table at %q: %s", prevPath, err)
	}

	return curr, prev, nil
}

var indexDBTableNameRegexp = regexp.MustCompile("^[0-9A-F]{16}$")

func nextIndexDBTableName() string {
	n := atomic.AddUint64(&indexDBTableIdx, 1)
	return fmt.Sprintf("%016X", n)
}

var indexDBTableIdx = uint64(time.Now().UnixNano())
