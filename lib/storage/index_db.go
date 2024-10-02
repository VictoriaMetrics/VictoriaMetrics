package storage

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/cespare/xxhash/v2"
)

const (
	// Prefix for MetricName->TSID entries.
	//
	// This index was substituted with nsPrefixDateMetricNameToTSID,
	// since the MetricName->TSID index may require big amounts of memory for indexdb/dataBlocks cache
	// when it grows big on the configured retention under high churn rate
	// (e.g. when new time series are constantly registered).
	//
	// It is much more efficient from memory usage PoV to query per-day MetricName->TSID index
	// (aka nsPrefixDateMetricNameToTSID) when the TSID must be obtained for the given MetricName
	// during data ingestion under high churn rate and big retention.
	//
	// nsPrefixMetricNameToTSID = 0

	// Prefix for Tag->MetricID entries.
	nsPrefixTagToMetricIDs = 1

	// Prefix for MetricID->TSID entries.
	nsPrefixMetricIDToTSID = 2

	// Prefix for MetricID->MetricName entries.
	nsPrefixMetricIDToMetricName = 3

	// Prefix for deleted MetricID entries.
	nsPrefixDeletedMetricID = 4

	// Prefix for Date->MetricID entries.
	nsPrefixDateToMetricID = 5

	// Prefix for (Date,Tag)->MetricID entries.
	nsPrefixDateTagToMetricIDs = 6

	// Prefix for (Date,MetricName)->TSID entries.
	nsPrefixDateMetricNameToTSID = 7
)

// indexDB represents an index db.
type indexDB struct {
	// The number of references to indexDB struct.
	refCount atomic.Int32

	// if the mustDrop is set to true, then the indexDB must be dropped after refCount reaches zero.
	mustDrop atomic.Bool

	// The number of missing MetricID -> TSID entries.
	// High rate for this value means corrupted indexDB.
	missingTSIDsForMetricID atomic.Uint64

	// The number of calls for date range searches.
	dateRangeSearchCalls atomic.Uint64

	// The number of hits for date range searches.
	dateRangeSearchHits atomic.Uint64

	// The number of calls for global search.
	globalSearchCalls atomic.Uint64

	// missingMetricNamesForMetricID is a counter of missing MetricID -> MetricName entries.
	// High rate may mean corrupted indexDB due to unclean shutdown.
	// The db must be automatically recovered after that.
	missingMetricNamesForMetricID atomic.Uint64

	// minMissingTimestamp is the minimum timestamp, which is missing in the given indexDB.
	//
	// This field is used at containsTimeRange() function only for the previous indexDB,
	// since this indexDB is readonly.
	// This field cannot be used for the current indexDB, since it may receive data
	// with bigger timestamps at any time.
	minMissingTimestamp atomic.Int64

	// generation identifies the index generation ID
	// and is used for syncing items from different indexDBs
	generation uint64

	name string
	tb   *mergeset.Table

	extDB     *indexDB
	extDBLock sync.Mutex

	// Cache for fast TagFilters -> MetricIDs lookup.
	tagFiltersToMetricIDsCache *workingsetcache.Cache

	// The parent storage.
	s *Storage

	// Cache for (date, tagFilter) -> loopsCount, which is used for reducing
	// the amount of work when matching a set of filters.
	loopsPerDateTagFilterCache *workingsetcache.Cache

	indexSearchPool sync.Pool
}

var maxTagFiltersCacheSize int

// SetTagFiltersCacheSize overrides the default size of tagFiltersToMetricIDsCache
func SetTagFiltersCacheSize(size int) {
	maxTagFiltersCacheSize = size
}

func getTagFiltersCacheSize() int {
	if maxTagFiltersCacheSize <= 0 {
		return int(float64(memory.Allowed()) / 32)
	}
	return maxTagFiltersCacheSize
}

// mustOpenIndexDB opens index db from the given path.
//
// The last segment of the path should contain unique hex value which
// will be then used as indexDB.generation
func mustOpenIndexDB(path string, s *Storage, isReadOnly *atomic.Bool) *indexDB {
	if s == nil {
		logger.Panicf("BUG: Storage must be nin-nil")
	}

	name := filepath.Base(path)
	gen, err := strconv.ParseUint(name, 16, 64)
	if err != nil {
		logger.Panicf("FATAL: cannot parse indexdb path %q: %s", path, err)
	}

	tb := mergeset.MustOpenTable(path, invalidateTagFiltersCache, mergeTagToMetricIDsRows, isReadOnly)

	// Do not persist tagFiltersToMetricIDsCache in files, since it is very volatile because of tagFiltersKeyGen.
	mem := memory.Allowed()
	tagFiltersCacheSize := getTagFiltersCacheSize()

	db := &indexDB{
		generation: gen,
		tb:         tb,
		name:       name,

		tagFiltersToMetricIDsCache: workingsetcache.New(tagFiltersCacheSize),
		s:                          s,
		loopsPerDateTagFilterCache: workingsetcache.New(mem / 128),
	}
	db.incRef()
	return db
}

const noDeadline = 1<<64 - 1

// IndexDBMetrics contains essential metrics for indexDB.
type IndexDBMetrics struct {
	TagFiltersToMetricIDsCacheSize         uint64
	TagFiltersToMetricIDsCacheSizeBytes    uint64
	TagFiltersToMetricIDsCacheSizeMaxBytes uint64
	TagFiltersToMetricIDsCacheRequests     uint64
	TagFiltersToMetricIDsCacheMisses       uint64

	DeletedMetricsCount uint64

	IndexDBRefCount uint64

	MissingTSIDsForMetricID uint64

	RecentHourMetricIDsSearchCalls uint64
	RecentHourMetricIDsSearchHits  uint64

	DateRangeSearchCalls uint64
	DateRangeSearchHits  uint64
	GlobalSearchCalls    uint64

	MissingMetricNamesForMetricID uint64

	IndexBlocksWithMetricIDsProcessed      uint64
	IndexBlocksWithMetricIDsIncorrectOrder uint64

	MinTimestampForCompositeIndex     uint64
	CompositeFilterSuccessConversions uint64
	CompositeFilterMissingConversions uint64

	mergeset.TableMetrics
}

func (db *indexDB) scheduleToDrop() {
	db.mustDrop.Store(true)
}

// UpdateMetrics updates m with metrics from the db.
func (db *indexDB) UpdateMetrics(m *IndexDBMetrics) {
	// global index metrics
	m.DeletedMetricsCount += uint64(db.s.getDeletedMetricIDs().Len())

	m.IndexBlocksWithMetricIDsProcessed = indexBlocksWithMetricIDsProcessed.Load()
	m.IndexBlocksWithMetricIDsIncorrectOrder = indexBlocksWithMetricIDsIncorrectOrder.Load()

	m.MinTimestampForCompositeIndex = uint64(db.s.minTimestampForCompositeIndex)
	m.CompositeFilterSuccessConversions = compositeFilterSuccessConversions.Load()
	m.CompositeFilterMissingConversions = compositeFilterMissingConversions.Load()

	var cs fastcache.Stats

	cs.Reset()
	db.tagFiltersToMetricIDsCache.UpdateStats(&cs)
	m.TagFiltersToMetricIDsCacheSize += cs.EntriesCount
	m.TagFiltersToMetricIDsCacheSizeBytes += cs.BytesSize
	m.TagFiltersToMetricIDsCacheSizeMaxBytes += cs.MaxBytesSize
	m.TagFiltersToMetricIDsCacheRequests += cs.GetCalls
	m.TagFiltersToMetricIDsCacheMisses += cs.Misses

	m.IndexDBRefCount += uint64(db.refCount.Load())

	// this shouldn't increase the MissingTSIDsForMetricID value,
	// as we only count it as missingTSIDs if it can't be found in both the current and previous indexdb.
	m.MissingTSIDsForMetricID += db.missingTSIDsForMetricID.Load()

	m.DateRangeSearchCalls += db.dateRangeSearchCalls.Load()
	m.DateRangeSearchHits += db.dateRangeSearchHits.Load()
	m.GlobalSearchCalls += db.globalSearchCalls.Load()

	m.MissingMetricNamesForMetricID += db.missingMetricNamesForMetricID.Load()

	db.tb.UpdateMetrics(&m.TableMetrics)
	db.doExtDB(func(extDB *indexDB) {
		extDB.tb.UpdateMetrics(&m.TableMetrics)

		cs.Reset()
		extDB.tagFiltersToMetricIDsCache.UpdateStats(&cs)
		m.TagFiltersToMetricIDsCacheSize += cs.EntriesCount
		m.TagFiltersToMetricIDsCacheSizeBytes += cs.BytesSize
		m.TagFiltersToMetricIDsCacheSizeMaxBytes += cs.MaxBytesSize
		m.TagFiltersToMetricIDsCacheRequests += cs.GetCalls
		m.TagFiltersToMetricIDsCacheMisses += cs.Misses

		m.IndexDBRefCount += uint64(extDB.refCount.Load())
		m.MissingTSIDsForMetricID += extDB.missingTSIDsForMetricID.Load()

		m.DateRangeSearchCalls += extDB.dateRangeSearchCalls.Load()
		m.DateRangeSearchHits += extDB.dateRangeSearchHits.Load()
		m.GlobalSearchCalls += extDB.globalSearchCalls.Load()

		m.MissingMetricNamesForMetricID += extDB.missingMetricNamesForMetricID.Load()
	})
}

// doExtDB calls f for non-nil db.extDB.
//
// f isn't called if db.extDB is nil.
func (db *indexDB) doExtDB(f func(extDB *indexDB)) {
	db.extDBLock.Lock()
	extDB := db.extDB
	if extDB != nil {
		extDB.incRef()
	}
	db.extDBLock.Unlock()
	if extDB != nil {
		f(extDB)
		extDB.decRef()
	}
}

// hasExtDB returns true if db.extDB != nil
func (db *indexDB) hasExtDB() bool {
	db.extDBLock.Lock()
	ok := db.extDB != nil
	db.extDBLock.Unlock()
	return ok
}

// SetExtDB sets external db to search.
//
// It decrements refCount for the previous extDB.
func (db *indexDB) SetExtDB(extDB *indexDB) {
	db.extDBLock.Lock()
	prevExtDB := db.extDB
	db.extDB = extDB
	db.extDBLock.Unlock()

	if prevExtDB != nil {
		prevExtDB.decRef()
	}
}

// MustClose closes db.
func (db *indexDB) MustClose() {
	db.decRef()
}

func (db *indexDB) incRef() {
	db.refCount.Add(1)
}

func (db *indexDB) decRef() {
	n := db.refCount.Add(-1)
	if n < 0 {
		logger.Panicf("BUG: negative refCount: %d", n)
	}
	if n > 0 {
		return
	}

	tbPath := db.tb.Path()
	db.tb.MustClose()
	db.SetExtDB(nil)

	// Free space occupied by caches owned by db.
	db.tagFiltersToMetricIDsCache.Stop()
	db.loopsPerDateTagFilterCache.Stop()

	db.tagFiltersToMetricIDsCache = nil
	db.s = nil
	db.loopsPerDateTagFilterCache = nil

	if !db.mustDrop.Load() {
		return
	}

	logger.Infof("dropping indexDB %q", tbPath)
	fs.MustRemoveDirAtomic(tbPath)
	logger.Infof("indexDB %q has been dropped", tbPath)
}

var tagBufPool bytesutil.ByteBufferPool

func (db *indexDB) getMetricIDsFromTagFiltersCache(qt *querytracer.Tracer, key []byte) ([]uint64, bool) {
	qt = qt.NewChild("search for metricIDs in tag filters cache")
	defer qt.Done()
	buf := tagBufPool.Get()
	defer tagBufPool.Put(buf)
	buf.B = db.tagFiltersToMetricIDsCache.GetBig(buf.B[:0], key)
	if len(buf.B) == 0 {
		qt.Printf("cache miss")
		return nil, false
	}
	qt.Printf("found metricIDs with size: %d bytes", len(buf.B))
	metricIDs := mustUnmarshalMetricIDs(nil, buf.B)
	qt.Printf("unmarshaled %d metricIDs", len(metricIDs))
	return metricIDs, true
}

func (db *indexDB) putMetricIDsToTagFiltersCache(qt *querytracer.Tracer, metricIDs []uint64, key []byte) {
	qt = qt.NewChild("put %d metricIDs in cache", len(metricIDs))
	defer qt.Done()
	buf := tagBufPool.Get()
	buf.B = marshalMetricIDs(buf.B, metricIDs)
	qt.Printf("marshaled %d metricIDs into %d bytes", len(metricIDs), len(buf.B))
	db.tagFiltersToMetricIDsCache.SetBig(key, buf.B)
	qt.Printf("stored %d metricIDs into cache", len(metricIDs))
	tagBufPool.Put(buf)
}

func (db *indexDB) getFromMetricIDCache(dst *TSID, metricID uint64) error {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	buf := (*[unsafe.Sizeof(*dst)]byte)(unsafe.Pointer(dst))
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	tmp := db.s.metricIDCache.Get(buf[:0], key[:])
	if len(tmp) == 0 {
		// The TSID for the given metricID wasn't found in the cache.
		return io.EOF
	}
	if &tmp[0] != &buf[0] || len(tmp) != len(buf) {
		return fmt.Errorf("corrupted MetricID->TSID cache: unexpected size for metricID=%d value; got %d bytes; want %d bytes", metricID, len(tmp), len(buf))
	}
	return nil
}

func (db *indexDB) putToMetricIDCache(metricID uint64, tsid *TSID) {
	buf := (*[unsafe.Sizeof(*tsid)]byte)(unsafe.Pointer(tsid))
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	db.s.metricIDCache.Set(key[:], buf[:])
}

func (db *indexDB) getMetricNameFromCache(dst []byte, metricID uint64) []byte {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	return db.s.metricNameCache.Get(dst, key[:])
}

func (db *indexDB) putMetricNameToCache(metricID uint64, metricName []byte) {
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	db.s.metricNameCache.Set(key[:], metricName)
}

func marshalTagFiltersKey(dst []byte, tfss []*TagFilters, tr TimeRange, versioned bool) []byte {
	// There is no need in versioning the tagFilters key, since the tagFiltersToMetricIDsCache
	// isn't persisted to disk (it is very volatile because of tagFiltersKeyGen).
	prefix := ^uint64(0)
	if versioned {
		prefix = tagFiltersKeyGen.Load()
	}
	// Round start and end times to per-day granularity according to per-day inverted index.
	startDate := uint64(tr.MinTimestamp) / msecPerDay
	endDate := uint64(tr.MaxTimestamp-1) / msecPerDay
	dst = encoding.MarshalUint64(dst, prefix)
	dst = encoding.MarshalUint64(dst, startDate)
	dst = encoding.MarshalUint64(dst, endDate)
	for _, tfs := range tfss {
		dst = append(dst, 0) // separator between tfs groups.
		for i := range tfs.tfs {
			dst = tfs.tfs[i].Marshal(dst)
		}
	}
	return dst
}

func invalidateTagFiltersCache() {
	// This function must be fast, since it is called each time new timeseries is added.
	tagFiltersKeyGen.Add(1)
}

var tagFiltersKeyGen atomic.Uint64

func marshalMetricIDs(dst []byte, metricIDs []uint64) []byte {
	if len(metricIDs) == 0 {
		// Add one zero byte to indicate an empty metricID list and skip
		// compression to save CPU cycles.
		//
		// An empty slice passed to ztsd won't be compressed and therefore
		// nothing will be added to dst and if dst is empty the record won't be
		// added to the cache. As the result, the search for a given filter will
		// be performed again and again. This may lead to cases like this:
		// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7009
		return append(dst, 0)
	}

	// Compress metricIDs, so they occupy less space in the cache.
	//
	// The srcBuf is a []byte cast of metricIDs.
	srcBuf := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(metricIDs))), 8*len(metricIDs))

	dst = encoding.CompressZSTDLevel(dst, srcBuf, 1)
	return dst
}

func mustUnmarshalMetricIDs(dst []uint64, src []byte) []uint64 {
	if len(src) == 1 && src[0] == 0 {
		// One zero byte indicates an empty metricID list.
		// See marshalMetricIDs().
		return dst
	}

	// Decompress src into dstBuf.
	//
	// dstBuf is a []byte cast of dst.
	dstBuf := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(dst))), 8*cap(dst))
	dstBuf = dstBuf[:8*len(dst)]
	dstBufLen := len(dstBuf)
	var err error
	dstBuf, err = encoding.DecompressZSTD(dstBuf, src)
	if err != nil {
		logger.Panicf("FATAL: cannot decompress metricIDs: %s", err)
	}
	if (len(dstBuf)-dstBufLen)%8 != 0 {
		logger.Panicf("FATAL: cannot unmarshal metricIDs from buffer of %d bytes; the buffer length must divide by 8", len(dstBuf)-dstBufLen)
	}

	// Convert dstBuf back to dst
	dst = unsafe.Slice((*uint64)(unsafe.Pointer(unsafe.SliceData(dstBuf))), cap(dstBuf)/8)
	dst = dst[:len(dstBuf)/8]

	return dst
}

// getTSIDByMetricName fills the dst with TSID for the given metricName at the given date.
//
// It returns false if the given metricName isn't found in the indexdb.
func (is *indexSearch) getTSIDByMetricName(dst *generationTSID, metricName []byte, date uint64) bool {
	if is.getTSIDByMetricNameNoExtDB(&dst.TSID, metricName, date) {
		// Fast path - the TSID is found in the current indexdb.
		dst.generation = is.db.generation
		return true
	}

	// Slow path - search for the TSID in the previous indexdb
	ok := false
	deadline := is.deadline
	is.db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch(deadline)
		ok = is.getTSIDByMetricNameNoExtDB(&dst.TSID, metricName, date)
		extDB.putIndexSearch(is)
		if ok {
			dst.generation = extDB.generation
		}
	})
	return ok
}

type indexSearch struct {
	db *indexDB
	ts mergeset.TableSearch
	kb bytesutil.ByteBuffer
	mp tagToMetricIDsRowParser

	// deadline in unix timestamp seconds for the given search.
	deadline uint64
}

func (db *indexDB) getIndexSearch(deadline uint64) *indexSearch {
	v := db.indexSearchPool.Get()
	if v == nil {
		v = &indexSearch{
			db: db,
		}
	}
	is := v.(*indexSearch)
	is.ts.Init(db.tb)
	is.deadline = deadline
	return is
}

func (db *indexDB) putIndexSearch(is *indexSearch) {
	is.ts.MustClose()
	is.kb.Reset()
	is.mp.Reset()
	is.deadline = 0

	db.indexSearchPool.Put(is)
}

func generateTSID(dst *TSID, mn *MetricName) {
	dst.MetricGroupID = xxhash.Sum64(mn.MetricGroup)
	// Assume that the job-like metric is put at mn.Tags[0], while instance-like metric is put at mn.Tags[1]
	// This assumption is true because mn.Tags must be sorted with mn.sortTags() before calling generateTSID() function.
	// This allows grouping data blocks for the same (job, instance) close to each other on disk.
	// This reduces disk seeks and disk read IO when data blocks are read from disk for the same job and/or instance.
	// For example, data blocks for time series matching `process_resident_memory_bytes{job="vmstorage"}` are physically adjacent on disk.
	if len(mn.Tags) > 0 {
		dst.JobID = uint32(xxhash.Sum64(mn.Tags[0].Value))
	}
	if len(mn.Tags) > 1 {
		dst.InstanceID = uint32(xxhash.Sum64(mn.Tags[1].Value))
	}
	dst.MetricID = generateUniqueMetricID()
}

func (is *indexSearch) createGlobalIndexes(tsid *TSID, mn *MetricName) {
	ii := getIndexItems()
	defer putIndexItems(ii)

	// Create metricID -> metricName entry.
	ii.B = marshalCommonPrefix(ii.B, nsPrefixMetricIDToMetricName)
	ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
	ii.B = mn.Marshal(ii.B)
	ii.Next()

	// Create metricID -> TSID entry.
	ii.B = marshalCommonPrefix(ii.B, nsPrefixMetricIDToTSID)
	ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
	ii.B = tsid.Marshal(ii.B)
	ii.Next()

	// Create tag -> metricID entries for every tag in mn.
	kb := kbPool.Get()
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs)
	ii.registerTagIndexes(kb.B, mn, tsid.MetricID)
	kbPool.Put(kb)

	is.db.tb.AddItems(ii.Items)
}

type indexItems struct {
	B     []byte
	Items [][]byte

	start int
}

func (ii *indexItems) reset() {
	ii.B = ii.B[:0]
	ii.Items = ii.Items[:0]
	ii.start = 0
}

func (ii *indexItems) Next() {
	ii.Items = append(ii.Items, ii.B[ii.start:])
	ii.start = len(ii.B)
}

func getIndexItems() *indexItems {
	v := indexItemsPool.Get()
	if v == nil {
		return &indexItems{}
	}
	return v.(*indexItems)
}

func putIndexItems(ii *indexItems) {
	ii.reset()
	indexItemsPool.Put(ii)
}

var indexItemsPool sync.Pool

// SearchLabelNamesWithFiltersOnTimeRange returns all the label names, which match the given tfss on the given tr.
func (db *indexDB) SearchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label names: filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", tfss, &tr, maxLabelNames, maxMetrics)
	defer qt.Done()

	lns := make(map[string]struct{})
	qtChild := qt.NewChild("search for label names in the current indexdb")
	is := db.getIndexSearch(deadline)
	err := is.searchLabelNamesWithFiltersOnTimeRange(qtChild, lns, tfss, tr, maxLabelNames, maxMetrics)
	db.putIndexSearch(is)
	qtChild.Donef("found %d label names", len(lns))
	if err != nil {
		return nil, err
	}

	db.doExtDB(func(extDB *indexDB) {
		qtChild := qt.NewChild("search for label names in the previous indexdb")
		lnsLen := len(lns)
		is := extDB.getIndexSearch(deadline)
		err = is.searchLabelNamesWithFiltersOnTimeRange(qtChild, lns, tfss, tr, maxLabelNames, maxMetrics)
		extDB.putIndexSearch(is)
		qtChild.Donef("found %d additional label names", len(lns)-lnsLen)
	})
	if err != nil {
		return nil, err
	}

	labelNames := make([]string, 0, len(lns))
	for labelName := range lns {
		labelNames = append(labelNames, labelName)
	}
	// Do not sort label names, since they must be sorted by vmselect.
	qt.Printf("found %d label names in the current and the previous indexdb", len(labelNames))
	return labelNames, nil
}

func (is *indexSearch) searchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, lns map[string]struct{}, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int) error {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp-1) / msecPerDay
	if maxDate == 0 || minDate > maxDate || maxDate-minDate > maxDaysForPerDaySearch {
		qtChild := qt.NewChild("search for label names in global index: filters=%s", tfss)
		err := is.searchLabelNamesWithFiltersOnDate(qtChild, lns, tfss, 0, maxLabelNames, maxMetrics)
		qtChild.Done()
		return err
	}
	var mu sync.Mutex
	wg := getWaitGroup()
	var errGlobal error
	qt = qt.NewChild("parallel search for label names: filters=%s, timeRange=%s", tfss, &tr)
	for date := minDate; date <= maxDate; date++ {
		wg.Add(1)
		qtChild := qt.NewChild("search for label names: filters=%s, date=%s", tfss, dateToString(date))
		go func(date uint64) {
			defer func() {
				qtChild.Done()
				wg.Done()
			}()
			lnsLocal := make(map[string]struct{})
			isLocal := is.db.getIndexSearch(is.deadline)
			err := isLocal.searchLabelNamesWithFiltersOnDate(qtChild, lnsLocal, tfss, date, maxLabelNames, maxMetrics)
			is.db.putIndexSearch(isLocal)
			mu.Lock()
			defer mu.Unlock()
			if errGlobal != nil {
				return
			}
			if err != nil {
				errGlobal = err
				return
			}
			if len(lns) >= maxLabelNames {
				return
			}
			for k := range lnsLocal {
				lns[k] = struct{}{}
			}
		}(date)
	}
	wg.Wait()
	putWaitGroup(wg)
	qt.Done()
	return errGlobal
}

func (is *indexSearch) searchLabelNamesWithFiltersOnDate(qt *querytracer.Tracer, lns map[string]struct{}, tfss []*TagFilters, date uint64, maxLabelNames, maxMetrics int) error {
	filter, err := is.searchMetricIDsWithFiltersOnDate(qt, tfss, date, maxMetrics)
	if err != nil {
		return err
	}
	if filter != nil && filter.Len() <= 100e3 {
		// It is faster to obtain label names by metricIDs from the filter
		// instead of scanning the inverted index for the matching filters.
		// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978
		metricIDs := filter.AppendTo(nil)
		qt.Printf("sort %d metricIDs", len(metricIDs))
		is.getLabelNamesForMetricIDs(qt, metricIDs, lns, maxLabelNames)
		return nil
	}

	var prevLabelName []byte
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	dmis := is.db.s.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	underscoreNameSeen := false
	nsPrefixExpected := byte(nsPrefixDateTagToMetricIDs)
	if date == 0 {
		nsPrefixExpected = nsPrefixTagToMetricIDs
	}

	hasCompositeLabelName := false
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	if name := getCommonMetricNameForTagFilterss(tfss); len(name) > 0 {
		compositeLabelName := marshalCompositeTagKey(nil, name, nil)
		kb.B = marshalTagValue(kb.B, compositeLabelName)
		// Drop trailing tagSeparator
		kb.B = kb.B[:len(kb.B)-1]
		hasCompositeLabelName = true
	}
	prefix := append([]byte{}, kb.B...)

	ts.Seek(prefix)
	for len(lns) < maxLabelNames && ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item, nsPrefixExpected); err != nil {
			return err
		}
		if mp.GetMatchingSeriesCount(filter, dmis) == 0 {
			continue
		}
		labelName := mp.Tag.Key
		if len(labelName) == 0 || hasCompositeLabelName {
			underscoreNameSeen = true
		}
		if (!hasCompositeLabelName && isArtificialTagKey(labelName)) || string(labelName) == string(prevLabelName) {
			// Search for the next tag key.
			// The last char in kb.B must be tagSeparatorChar.
			// Just increment it in order to jump to the next tag key.
			kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
			if !hasCompositeLabelName && len(labelName) > 0 && labelName[0] == compositeTagKeyPrefix {
				// skip composite tag entries
				kb.B = append(kb.B, compositeTagKeyPrefix)
			} else {
				kb.B = marshalTagValue(kb.B, labelName)
			}
			kb.B[len(kb.B)-1]++
			ts.Seek(kb.B)
			continue
		}
		if !hasCompositeLabelName {
			lns[string(labelName)] = struct{}{}
		} else {
			_, key, err := unmarshalCompositeTagKey(labelName)
			if err != nil {
				return fmt.Errorf("cannot unmarshal composite tag key: %s", err)
			}
			lns[string(key)] = struct{}{}
		}
		prevLabelName = append(prevLabelName[:0], labelName...)
	}
	if underscoreNameSeen {
		lns["__name__"] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error during search for prefix %q: %w", prefix, err)
	}
	return nil
}

func (is *indexSearch) getLabelNamesForMetricIDs(qt *querytracer.Tracer, metricIDs []uint64, lns map[string]struct{}, maxLabelNames int) {
	if len(metricIDs) > 0 {
		lns["__name__"] = struct{}{}
	}

	dmis := is.db.s.getDeletedMetricIDs()

	var mn MetricName
	foundLabelNames := 0
	var buf []byte
	for _, metricID := range metricIDs {
		if dmis.Has(metricID) {
			// skip deleted IDs from result
			continue
		}
		var ok bool
		buf, ok = is.searchMetricNameWithCache(buf[:0], metricID)
		if !ok {
			// It is likely the metricID->metricName entry didn't propagate to inverted index yet.
			// Skip this metricID for now.
			continue
		}
		if err := mn.Unmarshal(buf); err != nil {
			logger.Panicf("FATAL: cannot unmarshal metricName %q: %s", buf, err)
		}
		for _, tag := range mn.Tags {
			if _, ok := lns[string(tag.Key)]; !ok {
				foundLabelNames++
				lns[string(tag.Key)] = struct{}{}
				if len(lns) >= maxLabelNames {
					qt.Printf("hit the limit on the number of unique label names: %d", maxLabelNames)
					return
				}
			}
		}
	}
	qt.Printf("get %d distinct label names from %d metricIDs", foundLabelNames, len(metricIDs))
}

// SearchLabelValuesWithFiltersOnTimeRange returns label values for the given labelName, tfss and tr.
func (db *indexDB) SearchLabelValuesWithFiltersOnTimeRange(qt *querytracer.Tracer, labelName string, tfss []*TagFilters, tr TimeRange,
	maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search for label values: labelName=%q, filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", labelName, tfss, &tr, maxLabelValues, maxMetrics)
	defer qt.Done()

	lvs := make(map[string]struct{})
	qtChild := qt.NewChild("search for label values in the current indexdb")
	is := db.getIndexSearch(deadline)
	err := is.searchLabelValuesWithFiltersOnTimeRange(qtChild, lvs, labelName, tfss, tr, maxLabelValues, maxMetrics)
	db.putIndexSearch(is)
	qtChild.Donef("found %d label values", len(lvs))
	if err != nil {
		return nil, err
	}
	db.doExtDB(func(extDB *indexDB) {
		qtChild := qt.NewChild("search for label values in the previous indexdb")
		lvsLen := len(lvs)
		is := extDB.getIndexSearch(deadline)
		err = is.searchLabelValuesWithFiltersOnTimeRange(qtChild, lvs, labelName, tfss, tr, maxLabelValues, maxMetrics)
		extDB.putIndexSearch(is)
		qtChild.Donef("found %d additional label values", len(lvs)-lvsLen)
	})
	if err != nil {
		return nil, err
	}

	labelValues := make([]string, 0, len(lvs))
	for labelValue := range lvs {
		if len(labelValue) == 0 {
			// Skip empty values, since they have no any meaning.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
			continue
		}
		labelValues = append(labelValues, labelValue)
	}
	// Do not sort labelValues, since they must be sorted by vmselect.
	qt.Printf("found %d label values in the current and the previous indexdb", len(labelValues))
	return labelValues, nil
}

func (is *indexSearch) searchLabelValuesWithFiltersOnTimeRange(qt *querytracer.Tracer, lvs map[string]struct{}, labelName string, tfss []*TagFilters,
	tr TimeRange, maxLabelValues, maxMetrics int) error {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp-1) / msecPerDay
	if maxDate == 0 || minDate > maxDate || maxDate-minDate > maxDaysForPerDaySearch {
		qtChild := qt.NewChild("search for label values in global index: labelName=%q, filters=%s", labelName, tfss)
		err := is.searchLabelValuesWithFiltersOnDate(qtChild, lvs, labelName, tfss, 0, maxLabelValues, maxMetrics)
		qtChild.Done()
		return err
	}
	var mu sync.Mutex
	wg := getWaitGroup()
	var errGlobal error
	qt = qt.NewChild("parallel search for label values: labelName=%q, filters=%s, timeRange=%s", labelName, tfss, &tr)
	for date := minDate; date <= maxDate; date++ {
		wg.Add(1)
		qtChild := qt.NewChild("search for label names: filters=%s, date=%s", tfss, dateToString(date))
		go func(date uint64) {
			defer func() {
				qtChild.Done()
				wg.Done()
			}()
			lvsLocal := make(map[string]struct{})
			isLocal := is.db.getIndexSearch(is.deadline)
			err := isLocal.searchLabelValuesWithFiltersOnDate(qtChild, lvsLocal, labelName, tfss, date, maxLabelValues, maxMetrics)
			is.db.putIndexSearch(isLocal)
			mu.Lock()
			defer mu.Unlock()
			if errGlobal != nil {
				return
			}
			if err != nil {
				errGlobal = err
				return
			}
			if len(lvs) >= maxLabelValues {
				return
			}
			for v := range lvsLocal {
				lvs[v] = struct{}{}
			}
		}(date)
	}
	wg.Wait()
	putWaitGroup(wg)
	qt.Done()
	return errGlobal
}

func (is *indexSearch) searchLabelValuesWithFiltersOnDate(qt *querytracer.Tracer, lvs map[string]struct{}, labelName string, tfss []*TagFilters,
	date uint64, maxLabelValues, maxMetrics int) error {
	filter, err := is.searchMetricIDsWithFiltersOnDate(qt, tfss, date, maxMetrics)
	if err != nil {
		return err
	}
	if filter != nil && filter.Len() <= 100e3 {
		// It is faster to obtain label values by metricIDs from the filter
		// instead of scanning the inverted index for the matching filters.
		// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978
		metricIDs := filter.AppendTo(nil)
		qt.Printf("sort %d metricIDs", len(metricIDs))
		is.getLabelValuesForMetricIDs(qt, lvs, labelName, metricIDs, maxLabelValues)
		return nil
	}
	if labelName == "__name__" {
		// __name__ label is encoded as empty string in indexdb.
		labelName = ""
	}

	labelNameBytes := bytesutil.ToUnsafeBytes(labelName)
	if name := getCommonMetricNameForTagFilterss(tfss); len(name) > 0 && labelName != "" {
		labelNameBytes = marshalCompositeTagKey(nil, name, labelNameBytes)
	}

	var prevLabelValue []byte
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	dmis := is.db.s.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	nsPrefixExpected := byte(nsPrefixDateTagToMetricIDs)
	if date == 0 {
		nsPrefixExpected = nsPrefixTagToMetricIDs
	}
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	kb.B = marshalTagValue(kb.B, labelNameBytes)
	prefix := append([]byte{}, kb.B...)
	ts.Seek(prefix)
	for len(lvs) < maxLabelValues && ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item, nsPrefixExpected); err != nil {
			return err
		}
		if mp.GetMatchingSeriesCount(filter, dmis) == 0 {
			continue
		}
		labelValue := mp.Tag.Value
		if string(labelValue) == string(prevLabelValue) {
			// Search for the next tag value.
			// The last char in kb.B must be tagSeparatorChar.
			// Just increment it in order to jump to the next tag value.
			kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
			kb.B = marshalTagValue(kb.B, labelNameBytes)
			kb.B = marshalTagValue(kb.B, labelValue)
			kb.B[len(kb.B)-1]++
			ts.Seek(kb.B)
			continue
		}
		lvs[string(labelValue)] = struct{}{}
		prevLabelValue = append(prevLabelValue[:0], labelValue...)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag name prefix %q: %w", prefix, err)
	}
	return nil
}

func (is *indexSearch) getLabelValuesForMetricIDs(qt *querytracer.Tracer, lvs map[string]struct{}, labelName string, metricIDs []uint64, maxLabelValues int) {
	if labelName == "" {
		labelName = "__name__"
	}

	dmis := is.db.s.getDeletedMetricIDs()

	var mn MetricName
	foundLabelValues := 0
	var buf []byte
	for _, metricID := range metricIDs {
		if dmis.Has(metricID) {
			// skip deleted IDs from result
			continue
		}
		var ok bool
		buf, ok = is.searchMetricNameWithCache(buf[:0], metricID)
		if !ok {
			// It is likely the metricID->metricName entry didn't propagate to inverted index yet.
			// Skip this metricID for now.
			continue
		}
		if err := mn.Unmarshal(buf); err != nil {
			logger.Panicf("FATAL: cannot unmarshal metricName %q: %s", buf, err)
		}
		tagValue := mn.GetTagValue(labelName)
		if _, ok := lvs[string(tagValue)]; !ok {
			foundLabelValues++
			lvs[string(tagValue)] = struct{}{}
			if len(lvs) >= maxLabelValues {
				qt.Printf("hit the limit on the number of unique label values for label %q: %d", labelName, maxLabelValues)
				return
			}
		}
	}
	qt.Printf("get %d distinct values for label %q from %d metricIDs", foundLabelValues, labelName, len(metricIDs))
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given tagKey and tagValuePrefix on the given tr.
//
// This allows implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or similar APIs.
//
// If it returns maxTagValueSuffixes suffixes, then it is likely more than maxTagValueSuffixes suffixes is found.
func (db *indexDB) SearchTagValueSuffixes(qt *querytracer.Tracer, tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search tag value suffixes for timeRange=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c, maxTagValueSuffixes=%d",
		&tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	defer qt.Done()

	// TODO: cache results?

	tvss := make(map[string]struct{})
	is := db.getIndexSearch(deadline)
	err := is.searchTagValueSuffixesForTimeRange(tvss, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}
	if len(tvss) < maxTagValueSuffixes {
		db.doExtDB(func(extDB *indexDB) {
			is := extDB.getIndexSearch(deadline)
			qtChild := qt.NewChild("search tag value suffixes in the previous indexdb")
			err = is.searchTagValueSuffixesForTimeRange(tvss, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
			qtChild.Done()
			extDB.putIndexSearch(is)
		})
		if err != nil {
			return nil, err
		}
	}

	suffixes := make([]string, 0, len(tvss))
	for suffix := range tvss {
		// Do not skip empty suffixes, since they may represent leaf tag values.
		suffixes = append(suffixes, suffix)
	}
	if len(suffixes) > maxTagValueSuffixes {
		suffixes = suffixes[:maxTagValueSuffixes]
	}
	// Do not sort suffixes, since they must be sorted by vmselect.
	qt.Printf("found %d suffixes", len(suffixes))
	return suffixes, nil
}

func (is *indexSearch) searchTagValueSuffixesForTimeRange(tvss map[string]struct{}, tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) error {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp-1) / msecPerDay
	if minDate > maxDate || maxDate-minDate > maxDaysForPerDaySearch {
		return is.searchTagValueSuffixesAll(tvss, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	}
	// Query over multiple days in parallel.
	wg := getWaitGroup()
	var errGlobal error
	var mu sync.Mutex // protects tvss + errGlobal from concurrent access below.
	for minDate <= maxDate {
		wg.Add(1)
		go func(date uint64) {
			defer wg.Done()
			tvssLocal := make(map[string]struct{})
			isLocal := is.db.getIndexSearch(is.deadline)
			err := isLocal.searchTagValueSuffixesForDate(tvssLocal, date, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
			is.db.putIndexSearch(isLocal)
			mu.Lock()
			defer mu.Unlock()
			if errGlobal != nil {
				return
			}
			if err != nil {
				errGlobal = err
				return
			}
			if len(tvss) > maxTagValueSuffixes {
				return
			}
			for k := range tvssLocal {
				tvss[k] = struct{}{}
			}
		}(minDate)
		minDate++
	}
	wg.Wait()
	putWaitGroup(wg)
	return errGlobal
}

func (is *indexSearch) searchTagValueSuffixesAll(tvss map[string]struct{}, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) error {
	kb := &is.kb
	nsPrefix := byte(nsPrefixTagToMetricIDs)
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefix)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagKey))
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagValuePrefix))
	kb.B = kb.B[:len(kb.B)-1] // remove tagSeparatorChar from the end of kb.B
	prefix := append([]byte(nil), kb.B...)
	return is.searchTagValueSuffixesForPrefix(tvss, nsPrefix, prefix, len(tagValuePrefix), delimiter, maxTagValueSuffixes)
}

func (is *indexSearch) searchTagValueSuffixesForDate(tvss map[string]struct{}, date uint64, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) error {
	nsPrefix := byte(nsPrefixDateTagToMetricIDs)
	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefix)
	kb.B = encoding.MarshalUint64(kb.B, date)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagKey))
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagValuePrefix))
	kb.B = kb.B[:len(kb.B)-1] // remove tagSeparatorChar from the end of kb.B
	prefix := append([]byte(nil), kb.B...)
	return is.searchTagValueSuffixesForPrefix(tvss, nsPrefix, prefix, len(tagValuePrefix), delimiter, maxTagValueSuffixes)
}

func (is *indexSearch) searchTagValueSuffixesForPrefix(tvss map[string]struct{}, nsPrefix byte, prefix []byte, tagValuePrefixLen int, delimiter byte, maxTagValueSuffixes int) error {
	kb := &is.kb
	ts := &is.ts
	mp := &is.mp
	dmis := is.db.s.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	ts.Seek(prefix)
	for len(tvss) < maxTagValueSuffixes && ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item, nsPrefix); err != nil {
			return err
		}
		if mp.GetMatchingSeriesCount(nil, dmis) == 0 {
			continue
		}
		tagValue := mp.Tag.Value
		suffix := tagValue[tagValuePrefixLen:]
		n := bytes.IndexByte(suffix, delimiter)
		if n < 0 {
			// Found leaf tag value that doesn't have delimiters after the given tagValuePrefix.
			tvss[string(suffix)] = struct{}{}
			continue
		}
		// Found non-leaf tag value. Extract suffix that end with the given delimiter.
		suffix = suffix[:n+1]
		tvss[string(suffix)] = struct{}{}
		if suffix[len(suffix)-1] == 255 {
			continue
		}
		// Search for the next suffix
		suffix[len(suffix)-1]++
		kb.B = append(kb.B[:0], prefix...)
		kb.B = marshalTagValue(kb.B, suffix)
		kb.B = kb.B[:len(kb.B)-1] // remove tagSeparatorChar
		ts.Seek(kb.B)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag value sufixes for prefix %q: %w", prefix, err)
	}
	return nil
}

// GetSeriesCount returns the approximate number of unique timeseries in the db.
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (db *indexDB) GetSeriesCount(deadline uint64) (uint64, error) {
	is := db.getIndexSearch(deadline)
	n, err := is.getSeriesCount()
	db.putIndexSearch(is)
	if err != nil {
		return 0, err
	}

	var nExt uint64
	db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch(deadline)
		nExt, err = is.getSeriesCount()
		extDB.putIndexSearch(is)
	})
	if err != nil {
		return 0, fmt.Errorf("error when searching in extDB: %w", err)
	}
	return n + nExt, nil
}

func (is *indexSearch) getSeriesCount() (uint64, error) {
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	loopsPaceLimiter := 0
	var metricIDsLen uint64
	// Extract the number of series from ((__name__=value): metricIDs) rows
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs)
	kb.B = marshalTagValue(kb.B, nil)
	ts.Seek(kb.B)
	for ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return 0, err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, kb.B) {
			break
		}
		tail := item[len(kb.B):]
		n := bytes.IndexByte(tail, tagSeparatorChar)
		if n < 0 {
			return 0, fmt.Errorf("invalid tag->metricIDs line %q: cannot find tagSeparatorChar %d", item, tagSeparatorChar)
		}
		tail = tail[n+1:]
		if err := mp.InitOnlyTail(item, tail); err != nil {
			return 0, err
		}
		// Take into account deleted timeseries too.
		// It is OK if series can be counted multiple times in rare cases -
		// the returned number is an estimation.
		metricIDsLen += uint64(mp.MetricIDsLen())
	}
	if err := ts.Error(); err != nil {
		return 0, fmt.Errorf("error when counting unique timeseries: %w", err)
	}
	return metricIDsLen, nil
}

// GetTSDBStatus returns topN entries for tsdb status for the given tfss, date and focusLabel.
func (db *indexDB) GetTSDBStatus(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	qtChild := qt.NewChild("collect tsdb stats in the current indexdb")

	is := db.getIndexSearch(deadline)
	status, err := is.getTSDBStatus(qtChild, tfss, date, focusLabel, topN, maxMetrics)
	qtChild.Done()
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}
	if status.hasEntries() {
		return status, nil
	}
	db.doExtDB(func(extDB *indexDB) {
		qtChild := qt.NewChild("collect tsdb stats in the previous indexdb")
		is := extDB.getIndexSearch(deadline)
		status, err = is.getTSDBStatus(qtChild, tfss, date, focusLabel, topN, maxMetrics)
		qtChild.Done()
		extDB.putIndexSearch(is)
	})
	if err != nil {
		return nil, fmt.Errorf("error when obtaining TSDB status from extDB: %w", err)
	}
	return status, nil
}

// getTSDBStatus returns topN entries for tsdb status for the given tfss, date and focusLabel.
func (is *indexSearch) getTSDBStatus(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int) (*TSDBStatus, error) {
	filter, err := is.searchMetricIDsWithFiltersOnDate(qt, tfss, date, maxMetrics)
	if err != nil {
		return nil, err
	}
	if filter != nil && filter.Len() == 0 {
		qt.Printf("no matching series for filter=%s", tfss)
		return &TSDBStatus{}, nil
	}
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	dmis := is.db.s.getDeletedMetricIDs()
	thSeriesCountByMetricName := newTopHeap(topN)
	thSeriesCountByLabelName := newTopHeap(topN)
	thSeriesCountByFocusLabelValue := newTopHeap(topN)
	thSeriesCountByLabelValuePair := newTopHeap(topN)
	thLabelValueCountByLabelName := newTopHeap(topN)
	var tmp, prevLabelName, prevLabelValuePair []byte
	var labelValueCountByLabelName, seriesCountByLabelValuePair uint64
	var totalSeries, labelSeries, totalLabelValuePairs uint64
	nameEqualBytes := []byte("__name__=")
	focusLabelEqualBytes := []byte(focusLabel + "=")

	loopsPaceLimiter := 0
	nsPrefixExpected := byte(nsPrefixDateTagToMetricIDs)
	if date == 0 {
		nsPrefixExpected = nsPrefixTagToMetricIDs
	}
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	prefix := append([]byte{}, kb.B...)
	ts.Seek(prefix)
	for ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return nil, err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item, nsPrefixExpected); err != nil {
			return nil, err
		}
		matchingSeriesCount := mp.GetMatchingSeriesCount(filter, dmis)
		if matchingSeriesCount == 0 {
			// Skip rows without matching metricIDs.
			continue
		}
		tmp = append(tmp[:0], mp.Tag.Key...)
		labelName := tmp
		if isArtificialTagKey(labelName) {
			// Skip artificially created tag keys.
			kb.B = append(kb.B[:0], prefix...)
			if len(labelName) > 0 && labelName[0] == compositeTagKeyPrefix {
				kb.B = append(kb.B, compositeTagKeyPrefix)
			} else {
				kb.B = marshalTagValue(kb.B, labelName)
			}
			kb.B[len(kb.B)-1]++
			ts.Seek(kb.B)
			continue
		}
		if len(labelName) == 0 {
			labelName = append(labelName, "__name__"...)
			tmp = labelName
		}
		if string(labelName) == "__name__" {
			totalSeries += uint64(matchingSeriesCount)
		}
		tmp = append(tmp, '=')
		tmp = append(tmp, mp.Tag.Value...)
		labelValuePair := tmp
		if len(prevLabelName) == 0 {
			prevLabelName = append(prevLabelName[:0], labelName...)
		}
		if string(labelName) != string(prevLabelName) {
			thLabelValueCountByLabelName.push(prevLabelName, labelValueCountByLabelName)
			thSeriesCountByLabelName.push(prevLabelName, labelSeries)
			labelSeries = 0
			labelValueCountByLabelName = 0
			prevLabelName = append(prevLabelName[:0], labelName...)
		}
		if len(prevLabelValuePair) == 0 {
			prevLabelValuePair = append(prevLabelValuePair[:0], labelValuePair...)
			labelValueCountByLabelName++
		}
		if string(labelValuePair) != string(prevLabelValuePair) {
			thSeriesCountByLabelValuePair.push(prevLabelValuePair, seriesCountByLabelValuePair)
			if bytes.HasPrefix(prevLabelValuePair, nameEqualBytes) {
				thSeriesCountByMetricName.push(prevLabelValuePair[len(nameEqualBytes):], seriesCountByLabelValuePair)
			}
			if bytes.HasPrefix(prevLabelValuePair, focusLabelEqualBytes) {
				thSeriesCountByFocusLabelValue.push(prevLabelValuePair[len(focusLabelEqualBytes):], seriesCountByLabelValuePair)
			}
			seriesCountByLabelValuePair = 0
			labelValueCountByLabelName++
			prevLabelValuePair = append(prevLabelValuePair[:0], labelValuePair...)
		}
		// It is OK if series can be counted multiple times in rare cases -
		// the returned number is an estimation.
		labelSeries += uint64(matchingSeriesCount)
		seriesCountByLabelValuePair += uint64(matchingSeriesCount)
		totalLabelValuePairs += uint64(matchingSeriesCount)
	}
	if err := ts.Error(); err != nil {
		return nil, fmt.Errorf("error when counting time series by metric names: %w", err)
	}
	thLabelValueCountByLabelName.push(prevLabelName, labelValueCountByLabelName)
	thSeriesCountByLabelName.push(prevLabelName, labelSeries)
	thSeriesCountByLabelValuePair.push(prevLabelValuePair, seriesCountByLabelValuePair)
	if bytes.HasPrefix(prevLabelValuePair, nameEqualBytes) {
		thSeriesCountByMetricName.push(prevLabelValuePair[len(nameEqualBytes):], seriesCountByLabelValuePair)
	}
	if bytes.HasPrefix(prevLabelValuePair, focusLabelEqualBytes) {
		thSeriesCountByFocusLabelValue.push(prevLabelValuePair[len(focusLabelEqualBytes):], seriesCountByLabelValuePair)
	}
	status := &TSDBStatus{
		TotalSeries:                  totalSeries,
		TotalLabelValuePairs:         totalLabelValuePairs,
		SeriesCountByMetricName:      thSeriesCountByMetricName.getSortedResult(),
		SeriesCountByLabelName:       thSeriesCountByLabelName.getSortedResult(),
		SeriesCountByFocusLabelValue: thSeriesCountByFocusLabelValue.getSortedResult(),
		SeriesCountByLabelValuePair:  thSeriesCountByLabelValuePair.getSortedResult(),
		LabelValueCountByLabelName:   thLabelValueCountByLabelName.getSortedResult(),
	}
	return status, nil
}

// TSDBStatus contains TSDB status data for /api/v1/status/tsdb.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
type TSDBStatus struct {
	TotalSeries                  uint64
	TotalLabelValuePairs         uint64
	SeriesCountByMetricName      []TopHeapEntry
	SeriesCountByLabelName       []TopHeapEntry
	SeriesCountByFocusLabelValue []TopHeapEntry
	SeriesCountByLabelValuePair  []TopHeapEntry
	LabelValueCountByLabelName   []TopHeapEntry
}

func (status *TSDBStatus) hasEntries() bool {
	return len(status.SeriesCountByLabelValuePair) > 0
}

// topHeap maintains a heap of topHeapEntries with the maximum TopHeapEntry.n values.
type topHeap struct {
	topN int
	a    []TopHeapEntry
}

// newTopHeap returns topHeap for topN items.
func newTopHeap(topN int) *topHeap {
	return &topHeap{
		topN: topN,
	}
}

// TopHeapEntry represents an entry from `top heap` used in stats.
type TopHeapEntry struct {
	Name  string
	Count uint64
}

func (th *topHeap) push(name []byte, count uint64) {
	if count == 0 {
		return
	}
	if len(th.a) < th.topN {
		th.a = append(th.a, TopHeapEntry{
			Name:  string(name),
			Count: count,
		})
		heap.Fix(th, len(th.a)-1)
		return
	}
	if count <= th.a[0].Count {
		return
	}
	th.a[0] = TopHeapEntry{
		Name:  string(name),
		Count: count,
	}
	heap.Fix(th, 0)
}

func (th *topHeap) getSortedResult() []TopHeapEntry {
	result := append([]TopHeapEntry{}, th.a...)
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Name < b.Name
	})
	return result
}

// heap.Interface implementation for topHeap.

func (th *topHeap) Len() int {
	return len(th.a)
}

func (th *topHeap) Less(i, j int) bool {
	a := th.a
	return a[i].Count < a[j].Count
}

func (th *topHeap) Swap(i, j int) {
	a := th.a
	a[j], a[i] = a[i], a[j]
}

func (th *topHeap) Push(_ any) {
	panic(fmt.Errorf("BUG: Push shouldn't be called"))
}

func (th *topHeap) Pop() any {
	panic(fmt.Errorf("BUG: Pop shouldn't be called"))
}

// searchMetricNameWithCache appends metric name for the given metricID to dst
// and returns the result.
func (db *indexDB) searchMetricNameWithCache(dst []byte, metricID uint64) ([]byte, bool) {
	metricName := db.getMetricNameFromCache(dst, metricID)
	if len(metricName) > len(dst) {
		return metricName, true
	}

	is := db.getIndexSearch(noDeadline)
	var ok bool
	dst, ok = is.searchMetricName(dst, metricID)
	db.putIndexSearch(is)
	if ok {
		// There is no need in verifying whether the given metricID is deleted,
		// since the filtering must be performed before calling this func.
		db.putMetricNameToCache(metricID, dst)
		return dst, true
	}

	// Try searching in the external indexDB.
	db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch(noDeadline)
		dst, ok = is.searchMetricName(dst, metricID)
		extDB.putIndexSearch(is)
		if ok {
			// There is no need in verifying whether the given metricID is deleted,
			// since the filtering must be performed before calling this func.
			extDB.putMetricNameToCache(metricID, dst)
		}
	})
	if ok {
		return dst, true
	}

	if db.s.wasMetricIDMissingBefore(metricID) {
		// Cannot find the MetricName for the given metricID for the last 60 seconds.
		// It is likely the indexDB contains incomplete set of metricID -> metricName entries
		// after unclean shutdown or after restoring from a snapshot.
		// Mark the metricID as deleted, so it is created again when new sample
		// for the given time series is ingested next time.
		db.missingMetricNamesForMetricID.Add(1)
		db.deleteMetricIDs([]uint64{metricID})
	}

	return dst, false
}

// DeleteTSIDs marks as deleted all the TSIDs matching the given tfss and
// updates or resets all caches where TSIDs and the corresponding MetricIDs may
// be stored.
//
// If the number of the series exceeds maxMetrics, no series will be deleted and
// an error will be returned. Otherwise, the funciton returns the number of
// series deleted.
func (db *indexDB) DeleteTSIDs(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (int, error) {
	qt = qt.NewChild("deleting series for %s", tfss)
	defer qt.Done()
	if len(tfss) == 0 {
		return 0, nil
	}

	// Obtain metricIDs to delete.
	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: (1 << 63) - 1,
	}
	is := db.getIndexSearch(noDeadline)
	metricIDs, err := is.searchMetricIDs(qt, tfss, tr, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return 0, err
	}
	db.deleteMetricIDs(metricIDs)

	// Delete TSIDs in the extDB.
	deletedCount := len(metricIDs)
	db.doExtDB(func(extDB *indexDB) {
		var n int
		qtChild := qt.NewChild("deleting series from the previos indexdb")
		n, err = extDB.DeleteTSIDs(qtChild, tfss, maxMetrics)
		qtChild.Donef("deleted %d series", n)
		deletedCount += n
	})
	if err != nil {
		return deletedCount, fmt.Errorf("cannot delete tsids in extDB: %w", err)
	}
	return deletedCount, nil
}

func (db *indexDB) deleteMetricIDs(metricIDs []uint64) {
	if len(metricIDs) == 0 {
		// Nothing to delete
		return
	}

	// atomically add deleted metricIDs to an inmemory map.
	dmis := &uint64set.Set{}
	dmis.AddMulti(metricIDs)
	db.s.updateDeletedMetricIDs(dmis)

	// Reset TagFilters -> TSIDS cache, since it may contain deleted TSIDs.
	invalidateTagFiltersCache()

	// Reset MetricName -> TSID cache, since it may contain deleted TSIDs.
	db.s.resetAndSaveTSIDCache()

	// Store the metricIDs as deleted.
	// Make this after updating the deletedMetricIDs and resetting caches
	// in order to exclude the possibility of the inconsistent state when the deleted metricIDs
	// remain available in the tsidCache after unclean shutdown.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347
	items := getIndexItems()
	for _, metricID := range metricIDs {
		items.B = append(items.B, nsPrefixDeletedMetricID)
		items.B = encoding.MarshalUint64(items.B, metricID)
		items.Next()
	}
	db.tb.AddItems(items.Items)
	putIndexItems(items)
}

func (db *indexDB) loadDeletedMetricIDs() (*uint64set.Set, error) {
	is := db.getIndexSearch(noDeadline)
	dmis, err := is.loadDeletedMetricIDs()
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}
	return dmis, nil
}

func (is *indexSearch) loadDeletedMetricIDs() (*uint64set.Set, error) {
	dmis := &uint64set.Set{}
	ts := &is.ts
	kb := &is.kb
	kb.B = append(kb.B[:0], nsPrefixDeletedMetricID)
	ts.Seek(kb.B)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, kb.B) {
			break
		}
		item = item[len(kb.B):]
		if len(item) != 8 {
			return nil, fmt.Errorf("unexpected item len; got %d bytes; want %d bytes", len(item), 8)
		}
		metricID := encoding.UnmarshalUint64(item)
		dmis.Add(metricID)
	}
	if err := ts.Error(); err != nil {
		return nil, err
	}
	return dmis, nil
}

// searchMetricIDs returns metricIDs for the given tfss and tr.
//
// The returned metricIDs are sorted.
func (db *indexDB) searchMetricIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]uint64, error) {
	qt = qt.NewChild("search for matching metricIDs: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()

	if len(tfss) == 0 {
		return nil, nil
	}

	qtChild := qt.NewChild("search for metricIDs in the current indexdb")
	tfKeyBuf := tagFiltersKeyBufPool.Get()
	defer tagFiltersKeyBufPool.Put(tfKeyBuf)

	tfKeyBuf.B = marshalTagFiltersKey(tfKeyBuf.B[:0], tfss, tr, true)
	metricIDs, ok := db.getMetricIDsFromTagFiltersCache(qtChild, tfKeyBuf.B)
	if ok {
		// Fast path - metricIDs found in the cache
		qtChild.Done()
		return metricIDs, nil
	}

	// Slow path - search for metricIDs in the db and extDB.
	is := db.getIndexSearch(deadline)
	localMetricIDs, err := is.searchMetricIDs(qtChild, tfss, tr, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return nil, fmt.Errorf("error when searching for metricIDs in the current indexdb: %w", err)
	}
	qtChild.Done()

	var extMetricIDs []uint64
	db.doExtDB(func(extDB *indexDB) {
		qtChild := qt.NewChild("search for metricIDs in the previous indexdb")
		defer qtChild.Done()

		tfKeyExtBuf := tagFiltersKeyBufPool.Get()
		defer tagFiltersKeyBufPool.Put(tfKeyExtBuf)

		// Data in extDB cannot be changed, so use unversioned keys for tag cache.
		tfKeyExtBuf.B = marshalTagFiltersKey(tfKeyExtBuf.B[:0], tfss, tr, false)
		metricIDs, ok := extDB.getMetricIDsFromTagFiltersCache(qtChild, tfKeyExtBuf.B)
		if ok {
			extMetricIDs = metricIDs
			return
		}
		is := extDB.getIndexSearch(deadline)
		extMetricIDs, err = is.searchMetricIDs(qtChild, tfss, tr, maxMetrics)
		extDB.putIndexSearch(is)
		extDB.putMetricIDsToTagFiltersCache(qtChild, extMetricIDs, tfKeyExtBuf.B)
	})
	if err != nil {
		return nil, fmt.Errorf("error when searching for metricIDs in the previous indexdb: %w", err)
	}

	// Merge localMetricIDs with extMetricIDs.
	metricIDs = mergeSortedMetricIDs(localMetricIDs, extMetricIDs)
	qt.Printf("merge %d metricIDs from the current indexdb with %d metricIDs from the previous indexdb; result: %d metricIDs",
		len(localMetricIDs), len(extMetricIDs), len(metricIDs))

	// Store metricIDs in the cache.
	db.putMetricIDsToTagFiltersCache(qt, metricIDs, tfKeyBuf.B)

	return metricIDs, nil
}

func mergeSortedMetricIDs(a, b []uint64) []uint64 {
	if len(b) == 0 {
		return a
	}
	i := 0
	j := 0
	result := make([]uint64, 0, len(a)+len(b))
	for {
		next := b[j]
		start := i
		for i < len(a) && a[i] <= next {
			i++
		}
		result = append(result, a[start:i]...)
		if len(result) > 0 {
			last := result[len(result)-1]
			for j < len(b) && b[j] == last {
				j++
			}
		}
		if i == len(a) {
			return append(result, b[j:]...)
		}
		a, b = b, a
		i, j = j, i
	}
}

func (db *indexDB) getTSIDsFromMetricIDs(qt *querytracer.Tracer, metricIDs []uint64, deadline uint64) ([]TSID, error) {
	qt = qt.NewChild("obtain tsids from %d metricIDs", len(metricIDs))
	defer qt.Done()

	if len(metricIDs) == 0 {
		return nil, nil
	}

	// Search for TSIDs in the current indexdb
	tsids := make([]TSID, len(metricIDs))
	var extMetricIDs []uint64
	i := 0
	err := func() error {
		is := db.getIndexSearch(deadline)
		defer db.putIndexSearch(is)
		for loopsPaceLimiter, metricID := range metricIDs {
			if loopsPaceLimiter&paceLimiterSlowIterationsMask == 0 {
				if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
					return err
				}
			}
			// Try obtaining TSIDs from MetricID->TSID cache. This is much faster
			// than scanning the mergeset if it contains a lot of metricIDs.
			tsid := &tsids[i]
			err := is.db.getFromMetricIDCache(tsid, metricID)
			if err == nil {
				// Fast path - the tsid for metricID is found in cache.
				i++
				continue
			}
			if err != io.EOF {
				return err
			}
			if !is.getTSIDByMetricID(tsid, metricID) {
				// Postpone searching for the missing metricID in the extDB.
				extMetricIDs = append(extMetricIDs, metricID)
				continue
			}
			is.db.putToMetricIDCache(metricID, tsid)
			i++
		}
		return nil
	}()
	if err != nil {
		return nil, fmt.Errorf("error when searching for TISDs by metricIDs in the current indexdb: %w", err)
	}
	tsidsFound := i
	qt.Printf("found %d tsids for %d metricIDs in the current indexdb", tsidsFound, len(metricIDs))

	var metricIDsToDelete []uint64
	if len(extMetricIDs) > 0 {
		// Search for extMetricIDs in the previous indexdb (aka extDB)
		db.doExtDB(func(extDB *indexDB) {
			is := extDB.getIndexSearch(deadline)
			defer extDB.putIndexSearch(is)
			for loopsPaceLimiter, metricID := range extMetricIDs {
				if loopsPaceLimiter&paceLimiterSlowIterationsMask == 0 {
					if err = checkSearchDeadlineAndPace(is.deadline); err != nil {
						return
					}
				}
				// There is no need in searching for TSIDs in MetricID->TSID cache, since
				// this has been already done in the loop above (the MetricID->TSID cache is global).
				tsid := &tsids[i]
				if !is.getTSIDByMetricID(tsid, metricID) {
					// Cannot find TSID for the given metricID.
					// This may be the case on incomplete indexDB
					// due to snapshot or due to un-flushed entries.
					// Mark the metricID as deleted, so it is created again when new sample
					// for the given time series is ingested next time.
					if is.db.s.wasMetricIDMissingBefore(metricID) {
						is.db.missingTSIDsForMetricID.Add(1)
						metricIDsToDelete = append(metricIDsToDelete, metricID)
					}
					continue
				}
				is.db.putToMetricIDCache(metricID, tsid)
				i++
			}
		})
		if err != nil {
			return nil, fmt.Errorf("error when searching for TSIDs by metricIDs in the previous indexdb: %w", err)
		}
		qt.Printf("found %d tsids for %d metricIDs in the previous indexdb", i-tsidsFound, len(extMetricIDs))
	}

	tsids = tsids[:i]
	qt.Printf("load %d tsids for %d metricIDs from both current and previous indexdb", len(tsids), len(metricIDs))

	if len(metricIDsToDelete) > 0 {
		db.deleteMetricIDs(metricIDsToDelete)
	}

	// Sort the found tsids, since they must be passed to TSID search
	// in the sorted order.
	sort.Slice(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) })
	qt.Printf("sort %d tsids", len(tsids))
	return tsids, nil
}

var tagFiltersKeyBufPool bytesutil.ByteBufferPool

func (is *indexSearch) getTSIDByMetricNameNoExtDB(dst *TSID, metricName []byte, date uint64) bool {
	dmis := is.db.s.getDeletedMetricIDs()
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateMetricNameToTSID)
	kb.B = encoding.MarshalUint64(kb.B, date)
	kb.B = append(kb.B, metricName...)
	kb.B = append(kb.B, kvSeparatorChar)
	ts.Seek(kb.B)
	for ts.NextItem() {
		if !bytes.HasPrefix(ts.Item, kb.B) {
			// Nothing found.
			return false
		}
		v := ts.Item[len(kb.B):]
		tail, err := dst.Unmarshal(v)
		if err != nil {
			logger.Panicf("FATAL: cannot unmarshal TSID: %s", err)
		}
		if len(tail) > 0 {
			logger.Panicf("FATAL: unexpected non-empty tail left after unmarshaling TSID: %X", tail)
		}
		if dmis.Has(dst.MetricID) {
			// The dst is deleted. Continue searching.
			continue
		}
		// Found valid dst.
		return true
	}
	if err := ts.Error(); err != nil {
		logger.Panicf("FATAL: error when searching TSID by metricName; searchPrefix %q: %s", kb.B, err)
	}
	// Nothing found
	return false
}

func (is *indexSearch) searchMetricNameWithCache(dst []byte, metricID uint64) ([]byte, bool) {
	metricName := is.db.getMetricNameFromCache(dst, metricID)
	if len(metricName) > len(dst) {
		return metricName, true
	}
	var ok bool
	dst, ok = is.searchMetricName(dst, metricID)
	if ok {
		// There is no need in verifying whether the given metricID is deleted,
		// since the filtering must be performed before calling this func.
		is.db.putMetricNameToCache(metricID, dst)
		return dst, true
	}
	return dst, false
}

func (is *indexSearch) searchMetricName(dst []byte, metricID uint64) ([]byte, bool) {
	ts := &is.ts
	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToMetricName)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return dst, false
		}
		logger.Panicf("FATAL: error when searching metricName by metricID; searchPrefix %q: %s", kb.B, err)
	}
	v := ts.Item[len(kb.B):]
	dst = append(dst, v...)
	return dst, true
}

func (is *indexSearch) containsTimeRange(tr TimeRange) bool {
	db := is.db

	if db.hasExtDB() {
		// The db corresponds to the current indexDB, which is used for storing index data for newly registered time series.
		// This means that it may contain data for the given tr with probability close to 100%.
		return true
	}

	// The db corresponds to the previous indexDB, which is readonly.
	// So it is safe caching the minimum timestamp, which isn't covered by the db.
	minMissingTimestamp := db.minMissingTimestamp.Load()
	if minMissingTimestamp != 0 && tr.MinTimestamp >= minMissingTimestamp {
		return false
	}

	if is.containsTimeRangeSlow(tr) {
		return true
	}

	db.minMissingTimestamp.CompareAndSwap(minMissingTimestamp, tr.MinTimestamp)
	return false
}

func (is *indexSearch) containsTimeRangeSlow(tr TimeRange) bool {
	ts := &is.ts
	kb := &is.kb

	// Verify whether the tr.MinTimestamp is included into `ts` or is smaller than the minimum date stored in `ts`.
	// Do not check whether tr.MaxTimestamp is included into `ts` or is bigger than the max date stored in `ts` for performance reasons.
	// This means that containsTimeRangeSlow() can return true if `tr` is located below the min date stored in `ts`.
	// This is OK, since this case isn't encountered too much in practice.
	// The main practical case allows skipping searching in prev indexdb (`ts`) when `tr`
	// is located above the max date stored there.
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID)
	prefix := kb.B
	kb.B = encoding.MarshalUint64(kb.B, minDate)
	ts.Seek(kb.B)
	if !ts.NextItem() {
		if err := ts.Error(); err != nil {
			logger.Panicf("FATAL: error when searching for minDate=%d, prefix %q: %w", minDate, kb.B, err)
		}
		return false
	}
	if !bytes.HasPrefix(ts.Item, prefix) {
		// minDate exceeds max date from ts.
		return false
	}
	return true
}

func (is *indexSearch) getTSIDByMetricID(dst *TSID, metricID uint64) bool {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	ts := &is.ts
	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return false
		}
		logger.Panicf("FATAL: error when searching TSID by metricID=%d; searchPrefix %q: %s", metricID, kb.B, err)
	}
	v := ts.Item[len(kb.B):]
	tail, err := dst.Unmarshal(v)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal the found TSID=%X for metricID=%d: %s", v, metricID, err)
	}
	if len(tail) > 0 {
		logger.Panicf("FATAL: unexpected non-zero tail left after unmarshaling TSID for metricID=%d: %X", metricID, tail)
	}
	return true
}

// updateMetricIDsByMetricNameMatch matches metricName values for the given srcMetricIDs against tfs
// and adds matching metrics to metricIDs.
func (is *indexSearch) updateMetricIDsByMetricNameMatch(qt *querytracer.Tracer, metricIDs, srcMetricIDs *uint64set.Set, tfs []*tagFilter) error {
	qt = qt.NewChild("filter out %d metric ids with filters=%s", srcMetricIDs.Len(), tfs)
	defer qt.Done()

	// sort srcMetricIDs in order to speed up Seek below.
	sortedMetricIDs := srcMetricIDs.AppendTo(nil)
	qt.Printf("sort %d metric ids", len(sortedMetricIDs))

	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs)
	tfs = removeCompositeTagFilters(tfs, kb.B)

	metricName := kbPool.Get()
	defer kbPool.Put(metricName)
	mn := GetMetricName()
	defer PutMetricName(mn)
	for loopsPaceLimiter, metricID := range sortedMetricIDs {
		if loopsPaceLimiter&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		var ok bool
		metricName.B, ok = is.searchMetricNameWithCache(metricName.B[:0], metricID)
		if !ok {
			// It is likely the metricID->metricName entry didn't propagate to inverted index yet.
			// Skip this metricID for now.
			continue
		}
		if err := mn.Unmarshal(metricName.B); err != nil {
			logger.Panicf("FATAL: cannot unmarshal metricName %q: %s", metricName.B, err)
		}

		// Match the mn against tfs.
		ok, err := matchTagFilters(mn, tfs, &is.kb)
		if err != nil {
			return fmt.Errorf("cannot match MetricName %s against tagFilters: %w", mn, err)
		}
		if !ok {
			continue
		}
		metricIDs.Add(metricID)
	}
	qt.Printf("apply filters %s; resulting metric ids: %d", tfs, metricIDs.Len())
	return nil
}

func removeCompositeTagFilters(tfs []*tagFilter, prefix []byte) []*tagFilter {
	if !hasCompositeTagFilters(tfs, prefix) {
		return tfs
	}
	var tagKey []byte
	var name []byte
	tfsNew := make([]*tagFilter, 0, len(tfs)+1)
	for _, tf := range tfs {
		if !bytes.HasPrefix(tf.prefix, prefix) {
			tfsNew = append(tfsNew, tf)
			continue
		}
		suffix := tf.prefix[len(prefix):]
		var err error
		_, tagKey, err = unmarshalTagValue(tagKey[:0], suffix)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal tag key from suffix=%q: %s", suffix, err)
		}
		if len(tagKey) == 0 || tagKey[0] != compositeTagKeyPrefix {
			tfsNew = append(tfsNew, tf)
			continue
		}
		tagKey = tagKey[1:]
		nameLen, nSize := encoding.UnmarshalVarUint64(tagKey)
		if nSize <= 0 {
			logger.Panicf("BUG: cannot unmarshal nameLen from tagKey %q", tagKey)
		}
		tagKey = tagKey[nSize:]
		if nameLen == 0 {
			logger.Panicf("BUG: nameLen must be greater than 0")
		}
		if uint64(len(tagKey)) < nameLen {
			logger.Panicf("BUG: expecting at %d bytes for name in tagKey=%q; got %d bytes", nameLen, tagKey, len(tagKey))
		}
		name = append(name[:0], tagKey[:nameLen]...)
		tagKey = tagKey[nameLen:]
		var tfNew tagFilter
		if err := tfNew.Init(prefix, tagKey, tf.value, tf.isNegative, tf.isRegexp); err != nil {
			logger.Panicf("BUG: cannot initialize {%s=%q} filter: %s", tagKey, tf.value, err)
		}
		tfsNew = append(tfsNew, &tfNew)
	}
	if len(name) > 0 {
		var tfNew tagFilter
		if err := tfNew.Init(prefix, nil, name, false, false); err != nil {
			logger.Panicf("BUG: unexpected error when initializing {__name__=%q} filter: %s", name, err)
		}
		tfsNew = append(tfsNew, &tfNew)
	}
	return tfsNew
}

func hasCompositeTagFilters(tfs []*tagFilter, prefix []byte) bool {
	var tagKey []byte
	for _, tf := range tfs {
		if !bytes.HasPrefix(tf.prefix, prefix) {
			continue
		}
		suffix := tf.prefix[len(prefix):]
		var err error
		_, tagKey, err = unmarshalTagValue(tagKey[:0], suffix)
		if err != nil {
			logger.Panicf("BUG: cannot unmarshal tag key from suffix=%q: %s", suffix, err)
		}
		if len(tagKey) > 0 && tagKey[0] == compositeTagKeyPrefix {
			return true
		}
	}
	return false
}

func matchTagFilters(mn *MetricName, tfs []*tagFilter, kb *bytesutil.ByteBuffer) (bool, error) {
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs)
	for i, tf := range tfs {
		if bytes.Equal(tf.key, graphiteReverseTagKey) {
			// Skip artificial tag filter for Graphite-like metric names with dots,
			// since mn doesn't contain the corresponding tag.
			continue
		}
		if len(tf.key) == 0 || string(tf.key) == "__graphite__" {
			// Match against mn.MetricGroup.
			b := marshalTagValue(kb.B, nil)
			b = marshalTagValue(b, mn.MetricGroup)
			kb.B = b[:len(kb.B)]
			ok, err := tf.match(b)
			if err != nil {
				return false, fmt.Errorf("cannot match MetricGroup %q with tagFilter %s: %w", mn.MetricGroup, tf, err)
			}
			if !ok {
				// Move failed tf to start.
				// This should reduce the amount of useless work for the next mn.
				if i > 0 {
					tfs[0], tfs[i] = tfs[i], tfs[0]
				}
				return false, nil
			}
			continue
		}
		// Search for matching tag name.
		tagMatched := false
		tagSeen := false
		for _, tag := range mn.Tags {
			if string(tag.Key) != string(tf.key) {
				continue
			}

			// Found the matching tag name. Match the value.
			tagSeen = true
			b := tag.Marshal(kb.B)
			kb.B = b[:len(kb.B)]
			ok, err := tf.match(b)
			if err != nil {
				return false, fmt.Errorf("cannot match tag %q with tagFilter %s: %w", tag, tf, err)
			}
			if !ok {
				// Move failed tf to start.
				// This should reduce the amount of useless work for the next mn.
				if i > 0 {
					tfs[0], tfs[i] = tfs[i], tfs[0]
				}
				return false, nil
			}
			tagMatched = true
			break
		}
		if !tagSeen && (!tf.isNegative && tf.isEmptyMatch || tf.isNegative && !tf.isEmptyMatch) {
			// tf contains positive empty-match filter for non-existing tag key, i.e.
			// {non_existing_tag_key=~"foobar|"}
			//
			// OR
			//
			// tf contains negative filter for non-exsisting tag key
			// and this filter doesn't match empty string, i.e. {non_existing_tag_key!="foobar"}
			// Such filter matches anything.
			//
			// Note that the filter `{non_existing_tag_key!~"|foobar"}` shouldn't match anything,
			// since it is expected that it matches non-empty `non_existing_tag_key`.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/546 and
			// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2255 for details.
			continue
		}
		if tagMatched {
			// tf matches mn. Go to the next tf.
			continue
		}
		// Matching tag name wasn't found.
		// Move failed tf to start.
		// This should reduce the amount of useless work for the next mn.
		if i > 0 {
			tfs[0], tfs[i] = tfs[i], tfs[0]
		}
		return false, nil
	}
	return true, nil
}

func (is *indexSearch) searchMetricIDsWithFiltersOnDate(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, maxMetrics int) (*uint64set.Set, error) {
	if len(tfss) == 0 {
		return nil, nil
	}
	tr := TimeRange{
		MinTimestamp: int64(date) * msecPerDay,
		MaxTimestamp: int64(date+1)*msecPerDay - 1,
	}
	if date == 0 {
		// Search for metricIDs on the whole time range.
		tr.MaxTimestamp = timestampFromTime(time.Now())
	}
	metricIDs, err := is.searchMetricIDsInternal(qt, tfss, tr, maxMetrics)
	if err != nil {
		return nil, err
	}
	return metricIDs, nil
}

// searchMetricIDs returns metricIDs for the given tfss and tr.
//
// The returned metricIDs are sorted.
func (is *indexSearch) searchMetricIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]uint64, error) {
	metricIDs, err := is.searchMetricIDsInternal(qt, tfss, tr, maxMetrics)
	if err != nil {
		return nil, err
	}
	if metricIDs.Len() == 0 {
		// Nothing found
		return nil, nil
	}

	sortedMetricIDs := metricIDs.AppendTo(nil)
	qt.Printf("sort %d matching metric ids", len(sortedMetricIDs))

	// Filter out deleted metricIDs.
	dmis := is.db.s.getDeletedMetricIDs()
	if dmis.Len() > 0 {
		metricIDsFiltered := sortedMetricIDs[:0]
		for _, metricID := range sortedMetricIDs {
			if !dmis.Has(metricID) {
				metricIDsFiltered = append(metricIDsFiltered, metricID)
			}
		}
		qt.Printf("left %d metric ids after removing deleted metric ids", len(metricIDsFiltered))
		sortedMetricIDs = metricIDsFiltered
	}

	return sortedMetricIDs, nil
}

func errTooManyTimeseries(maxMetrics int) error {
	return fmt.Errorf("the number of matching timeseries exceeds %d; "+
		"either narrow down the search or increase -search.max* command-line flag values at vmselect "+
		"(the most likely limit is -search.maxUniqueTimeseries); "+
		"see https://docs.victoriametrics.com/#resource-usage-limits", maxMetrics)
}

func (is *indexSearch) searchMetricIDsInternal(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int) (*uint64set.Set, error) {
	qt = qt.NewChild("search for metric ids: filters=%s, timeRange=%s, maxMetrics=%d", tfss, &tr, maxMetrics)
	defer qt.Done()

	metricIDs := &uint64set.Set{}

	if !is.containsTimeRange(tr) {
		qt.Printf("indexdb doesn't contain data for the given timeRange=%s", &tr)
		return metricIDs, nil
	}

	if tr.MinTimestamp >= is.db.s.minTimestampForCompositeIndex {
		tfss = convertToCompositeTagFilterss(tfss)
		qt.Printf("composite filters=%s", tfss)
	}

	for _, tfs := range tfss {
		if len(tfs.tfs) == 0 {
			// An empty filters must be equivalent to `{__name__!=""}`
			tfs = NewTagFilters()
			if err := tfs.Add(nil, nil, true, false); err != nil {
				logger.Panicf(`BUG: cannot add {__name__!=""} filter: %s`, err)
			}
		}
		qtChild := qt.NewChild("update metric ids: filters=%s, timeRange=%s", tfs, &tr)
		prevMetricIDsLen := metricIDs.Len()
		err := is.updateMetricIDsForTagFilters(qtChild, metricIDs, tfs, tr, maxMetrics+1)
		qtChild.Donef("updated %d metric ids", metricIDs.Len()-prevMetricIDsLen)
		if err != nil {
			return nil, err
		}
		if metricIDs.Len() > maxMetrics {
			return nil, errTooManyTimeseries(maxMetrics)
		}
	}
	return metricIDs, nil
}

const maxDaysForPerDaySearch = 40

func (is *indexSearch) updateMetricIDsForTagFilters(qt *querytracer.Tracer, metricIDs *uint64set.Set, tfs *TagFilters, tr TimeRange, maxMetrics int) error {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp-1) / msecPerDay
	if minDate <= maxDate && maxDate-minDate <= maxDaysForPerDaySearch {
		// Fast path - search metricIDs by date range in the per-day inverted
		// index.
		is.db.dateRangeSearchCalls.Add(1)
		qt.Printf("search metric ids in the per-day index")
		return is.updateMetricIDsForDateRange(qt, metricIDs, tfs, minDate, maxDate, maxMetrics)
	}

	// Slow path - search metricIDs in the global inverted index.
	qt.Printf("search metric ids in the global index")
	is.db.globalSearchCalls.Add(1)
	m, err := is.getMetricIDsForDateAndFilters(qt, 0, tfs, maxMetrics)
	if err != nil {
		return err
	}
	metricIDs.UnionMayOwn(m)
	return nil
}

func (is *indexSearch) getMetricIDsForTagFilter(qt *querytracer.Tracer, tf *tagFilter, maxMetrics int, maxLoopsCount int64) (*uint64set.Set, int64, error) {
	if tf.isNegative {
		logger.Panicf("BUG: isNegative must be false")
	}
	metricIDs := &uint64set.Set{}
	if len(tf.orSuffixes) > 0 {
		// Fast path for orSuffixes - seek for rows for each value from orSuffixes.
		loopsCount, err := is.updateMetricIDsForOrSuffixes(tf, metricIDs, maxMetrics, maxLoopsCount)
		qt.Printf("found %d metric ids for filter={%s} using exact search; spent %d loops", metricIDs.Len(), tf, loopsCount)
		if err != nil {
			return nil, loopsCount, fmt.Errorf("error when searching for metricIDs for tagFilter in fast path: %w; tagFilter=%s", err, tf)
		}
		return metricIDs, loopsCount, nil
	}

	// Slow path - scan for all the rows with the given prefix.
	loopsCount, err := is.getMetricIDsForTagFilterSlow(tf, metricIDs.Add, maxLoopsCount)
	qt.Printf("found %d metric ids for filter={%s} using prefix search; spent %d loops", metricIDs.Len(), tf, loopsCount)
	if err != nil {
		return nil, loopsCount, fmt.Errorf("error when searching for metricIDs for tagFilter in slow path: %w; tagFilter=%s", err, tf)
	}
	return metricIDs, loopsCount, nil
}

var errTooManyLoops = fmt.Errorf("too many loops is needed for applying this filter")

func (is *indexSearch) getMetricIDsForTagFilterSlow(tf *tagFilter, f func(metricID uint64), maxLoopsCount int64) (int64, error) {
	if len(tf.orSuffixes) > 0 {
		logger.Panicf("BUG: the getMetricIDsForTagFilterSlow must be called only for empty tf.orSuffixes; got %s", tf.orSuffixes)
	}

	// Scan all the rows with tf.prefix and call f on every tf match.
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	var prevMatchingSuffix []byte
	var prevMatch bool
	var loopsCount int64
	loopsPaceLimiter := 0
	prefix := tf.prefix
	ts.Seek(prefix)
	for ts.NextItem() {
		if loopsPaceLimiter&paceLimiterMediumIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return loopsCount, err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return loopsCount, nil
		}
		tail := item[len(prefix):]
		n := bytes.IndexByte(tail, tagSeparatorChar)
		if n < 0 {
			return loopsCount, fmt.Errorf("invalid tag->metricIDs line %q: cannot find tagSeparatorChar=%d", item, tagSeparatorChar)
		}
		suffix := tail[:n+1]
		tail = tail[n+1:]
		if err := mp.InitOnlyTail(item, tail); err != nil {
			return loopsCount, err
		}
		mp.ParseMetricIDs()
		loopsCount += int64(mp.MetricIDsLen())
		if loopsCount > maxLoopsCount {
			return loopsCount, errTooManyLoops
		}
		if prevMatch && string(suffix) == string(prevMatchingSuffix) {
			// Fast path: the same tag value found.
			// There is no need in checking it again with potentially
			// slow tf.matchSuffix, which may call regexp.
			for _, metricID := range mp.MetricIDs {
				f(metricID)
			}
			continue
		}
		// Slow path: need tf.matchSuffix call.
		ok, err := tf.matchSuffix(suffix)
		// Assume that tf.matchSuffix call needs 10x more time than a single metric scan iteration.
		loopsCount += 10 * int64(tf.matchCost)
		if err != nil {
			return loopsCount, fmt.Errorf("error when matching %s against suffix %q: %w", tf, suffix, err)
		}
		if !ok {
			prevMatch = false
			if mp.MetricIDsLen() < maxMetricIDsPerRow/2 {
				// If the current row contains non-full metricIDs list,
				// then it is likely the next row contains the next tag value.
				// So skip seeking for the next tag value, since it will be slower than just ts.NextItem call.
				continue
			}
			// Optimization: skip all the metricIDs for the given tag value
			kb.B = append(kb.B[:0], item[:len(item)-len(tail)]...)
			// The last char in kb.B must be tagSeparatorChar. Just increment it
			// in order to jump to the next tag value.
			if len(kb.B) == 0 || kb.B[len(kb.B)-1] != tagSeparatorChar || tagSeparatorChar >= 0xff {
				return loopsCount, fmt.Errorf("data corruption: the last char in k=%X must be %X", kb.B, tagSeparatorChar)
			}
			kb.B[len(kb.B)-1]++
			ts.Seek(kb.B)
			// Assume that a seek cost is equivalent to 1000 ordinary loops.
			loopsCount += 1000
			continue
		}
		prevMatch = true
		prevMatchingSuffix = append(prevMatchingSuffix[:0], suffix...)
		for _, metricID := range mp.MetricIDs {
			f(metricID)
		}
	}
	if err := ts.Error(); err != nil {
		return loopsCount, fmt.Errorf("error when searching for tag filter prefix %q: %w", prefix, err)
	}
	return loopsCount, nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixes(tf *tagFilter, metricIDs *uint64set.Set, maxMetrics int, maxLoopsCount int64) (int64, error) {
	if tf.isNegative {
		logger.Panicf("BUG: isNegative must be false")
	}
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	var loopsCount int64
	for _, orSuffix := range tf.orSuffixes {
		kb.B = append(kb.B[:0], tf.prefix...)
		kb.B = append(kb.B, orSuffix...)
		kb.B = append(kb.B, tagSeparatorChar)
		lc, err := is.updateMetricIDsForOrSuffix(kb.B, metricIDs, maxMetrics, maxLoopsCount-loopsCount)
		loopsCount += lc
		if err != nil {
			return loopsCount, err
		}
		if metricIDs.Len() >= maxMetrics {
			return loopsCount, nil
		}
	}
	return loopsCount, nil
}

func (is *indexSearch) updateMetricIDsForOrSuffix(prefix []byte, metricIDs *uint64set.Set, maxMetrics int, maxLoopsCount int64) (int64, error) {
	ts := &is.ts
	mp := &is.mp
	var loopsCount int64
	loopsPaceLimiter := 0
	ts.Seek(prefix)
	for metricIDs.Len() < maxMetrics && ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return loopsCount, err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return loopsCount, nil
		}
		if err := mp.InitOnlyTail(item, item[len(prefix):]); err != nil {
			return loopsCount, err
		}
		loopsCount += int64(mp.MetricIDsLen())
		if loopsCount > maxLoopsCount {
			return loopsCount, errTooManyLoops
		}
		mp.ParseMetricIDs()
		metricIDs.AddMulti(mp.MetricIDs)
	}
	if err := ts.Error(); err != nil {
		return loopsCount, fmt.Errorf("error when searching for tag filter prefix %q: %w", prefix, err)
	}
	return loopsCount, nil
}

func (is *indexSearch) updateMetricIDsForDateRange(qt *querytracer.Tracer, metricIDs *uint64set.Set, tfs *TagFilters, minDate, maxDate uint64, maxMetrics int) error {
	if minDate == maxDate {
		// Fast path - query only a single date.
		m, err := is.getMetricIDsForDateAndFilters(qt, minDate, tfs, maxMetrics)
		if err != nil {
			return err
		}
		metricIDs.UnionMayOwn(m)
		is.db.dateRangeSearchHits.Add(1)
		return nil
	}

	// Slower path - search for metricIDs for each day in parallel.
	qt = qt.NewChild("parallel search for metric ids in per-day index: filters=%s, dayRange=[%d..%d]", tfs, minDate, maxDate)
	defer qt.Done()
	wg := getWaitGroup()
	var errGlobal error
	var mu sync.Mutex // protects metricIDs + errGlobal vars from concurrent access below
	for minDate <= maxDate {
		qtChild := qt.NewChild("parallel thread for date=%s", dateToString(minDate))
		wg.Add(1)
		go func(date uint64) {
			defer func() {
				qtChild.Done()
				wg.Done()
			}()
			isLocal := is.db.getIndexSearch(is.deadline)
			m, err := isLocal.getMetricIDsForDateAndFilters(qtChild, date, tfs, maxMetrics)
			is.db.putIndexSearch(isLocal)
			mu.Lock()
			defer mu.Unlock()
			if errGlobal != nil {
				return
			}
			if err != nil {
				dateStr := time.Unix(int64(date*24*3600), 0)
				errGlobal = fmt.Errorf("cannot search for metricIDs at %s: %w", dateStr, err)
				return
			}
			if metricIDs.Len() < maxMetrics {
				metricIDs.UnionMayOwn(m)
			}
		}(minDate)
		minDate++
	}
	wg.Wait()
	putWaitGroup(wg)
	if errGlobal != nil {
		return errGlobal
	}
	is.db.dateRangeSearchHits.Add(1)
	return nil
}

func (is *indexSearch) getMetricIDsForDateAndFilters(qt *querytracer.Tracer, date uint64, tfs *TagFilters, maxMetrics int) (*uint64set.Set, error) {
	if qt.Enabled() {
		qt = qt.NewChild("search for metric ids on a particular day: filters=%s, date=%s, maxMetrics=%d", tfs, dateToString(date), maxMetrics)
		defer qt.Done()
	}
	// Sort tfs by loopsCount needed for performing each filter.
	// This stats is usually collected from the previous queries.
	// This way we limit the amount of work below by applying fast filters at first.
	type tagFilterWithWeight struct {
		tf               *tagFilter
		loopsCount       int64
		filterLoopsCount int64
	}
	tfws := make([]tagFilterWithWeight, len(tfs.tfs))
	currentTime := fasttime.UnixTimestamp()
	for i := range tfs.tfs {
		tf := &tfs.tfs[i]
		loopsCount, filterLoopsCount, timestamp := is.getLoopsCountAndTimestampForDateFilter(date, tf)
		if currentTime > timestamp+3600 {
			// Update stats once per hour for relatively fast tag filters.
			// There is no need in spending CPU resources on updating stats for heavy tag filters.
			if loopsCount <= 10e6 {
				loopsCount = 0
			}
			if filterLoopsCount <= 10e6 {
				filterLoopsCount = 0
			}
		}
		tfws[i] = tagFilterWithWeight{
			tf:               tf,
			loopsCount:       loopsCount,
			filterLoopsCount: filterLoopsCount,
		}
	}
	sort.Slice(tfws, func(i, j int) bool {
		a, b := &tfws[i], &tfws[j]
		if a.loopsCount != b.loopsCount {
			return a.loopsCount < b.loopsCount
		}
		return a.tf.Less(b.tf)
	})
	getFirstPositiveLoopsCount := func(tfws []tagFilterWithWeight) int64 {
		for i := range tfws {
			if n := tfws[i].loopsCount; n > 0 {
				return n
			}
		}
		return int64Max
	}
	storeLoopsCount := func(tfw *tagFilterWithWeight, loopsCount int64) {
		if loopsCount != tfw.loopsCount {
			tfw.loopsCount = loopsCount
			is.storeLoopsCountForDateFilter(date, tfw.tf, tfw.loopsCount, tfw.filterLoopsCount)
		}
	}

	// Populate metricIDs for the first non-negative filter with the smallest cost.
	qtChild := qt.NewChild("search for the first non-negative filter with the smallest cost")
	var metricIDs *uint64set.Set
	tfwsRemaining := tfws[:0]
	maxDateMetrics := intMax
	if maxMetrics < intMax/50 {
		maxDateMetrics = maxMetrics * 50
	}
	for i, tfw := range tfws {
		tf := tfw.tf
		if tf.isNegative || tf.isEmptyMatch {
			tfwsRemaining = append(tfwsRemaining, tfw)
			continue
		}
		maxLoopsCount := getFirstPositiveLoopsCount(tfws[i+1:])
		m, loopsCount, err := is.getMetricIDsForDateTagFilter(qtChild, tf, date, tfs.commonPrefix, maxDateMetrics, maxLoopsCount)
		if err != nil {
			if errors.Is(err, errTooManyLoops) {
				// The tf took too many loops compared to the next filter. Postpone applying this filter.
				qtChild.Printf("the filter={%s} took more than %d loops; postpone it", tf, maxLoopsCount)
				storeLoopsCount(&tfw, 2*loopsCount)
				tfwsRemaining = append(tfwsRemaining, tfw)
				continue
			}
			// Move failing filter to the end of filter list.
			storeLoopsCount(&tfw, int64Max)
			return nil, err
		}
		if m.Len() >= maxDateMetrics {
			// Too many time series found by a single tag filter. Move the filter to the end of list.
			qtChild.Printf("the filter={%s} matches at least %d series; postpone it", tf, maxDateMetrics)
			storeLoopsCount(&tfw, int64Max-1)
			tfwsRemaining = append(tfwsRemaining, tfw)
			continue
		}
		storeLoopsCount(&tfw, loopsCount)
		metricIDs = m
		tfwsRemaining = append(tfwsRemaining, tfws[i+1:]...)
		qtChild.Printf("the filter={%s} matches less than %d series (actually %d series); use it", tf, maxDateMetrics, metricIDs.Len())
		break
	}
	qtChild.Done()
	tfws = tfwsRemaining

	if metricIDs == nil {
		// All the filters in tfs are negative or match too many time series.
		// Populate all the metricIDs for the given (date),
		// so later they can be filtered out with negative filters.
		qt.Printf("all the filters are negative or match more than %d time series; fall back to searching for all the metric ids", maxDateMetrics)
		m, err := is.getMetricIDsForDate(date, maxDateMetrics)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain all the metricIDs: %w", err)
		}
		if m.Len() >= maxDateMetrics {
			// Too many time series found for the given (date). Fall back to global search.
			return nil, errTooManyTimeseries(maxDateMetrics)
		}
		metricIDs = m
		qt.Printf("found %d metric ids", metricIDs.Len())
	}

	sort.Slice(tfws, func(i, j int) bool {
		a, b := &tfws[i], &tfws[j]
		if a.filterLoopsCount != b.filterLoopsCount {
			return a.filterLoopsCount < b.filterLoopsCount
		}
		return a.tf.Less(b.tf)
	})
	getFirstPositiveFilterLoopsCount := func(tfws []tagFilterWithWeight) int64 {
		for i := range tfws {
			if n := tfws[i].filterLoopsCount; n > 0 {
				return n
			}
		}
		return int64Max
	}
	storeFilterLoopsCount := func(tfw *tagFilterWithWeight, filterLoopsCount int64) {
		if filterLoopsCount != tfw.filterLoopsCount {
			is.storeLoopsCountForDateFilter(date, tfw.tf, tfw.loopsCount, filterLoopsCount)
		}
	}

	// Intersect metricIDs with the rest of filters.
	//
	// Do not run these tag filters in parallel, since this may result in CPU and RAM waste
	// when the initial tag filters significantly reduce the number of found metricIDs,
	// so the remaining filters could be performed via much faster metricName matching instead
	// of slow selecting of matching metricIDs.
	qtChild = qt.NewChild("intersect the remaining %d filters with the found %d metric ids", len(tfws), metricIDs.Len())
	var tfsPostponed []*tagFilter
	for i, tfw := range tfws {
		tf := tfw.tf
		metricIDsLen := metricIDs.Len()
		if metricIDsLen == 0 {
			// There is no need in applying the remaining filters to an empty set.
			break
		}
		if tfw.filterLoopsCount > int64(metricIDsLen)*loopsCountPerMetricNameMatch {
			// It should be faster performing metricName match on the remaining filters
			// instead of scanning big number of entries in the inverted index for these filters.
			for _, tfw := range tfws[i:] {
				tfsPostponed = append(tfsPostponed, tfw.tf)
			}
			break
		}
		maxLoopsCount := getFirstPositiveFilterLoopsCount(tfws[i+1:])
		if maxLoopsCount == int64Max {
			maxLoopsCount = int64(metricIDsLen) * loopsCountPerMetricNameMatch
		}
		m, filterLoopsCount, err := is.getMetricIDsForDateTagFilter(qtChild, tf, date, tfs.commonPrefix, intMax, maxLoopsCount)
		if err != nil {
			if errors.Is(err, errTooManyLoops) {
				// Postpone tf, since it took more loops than the next filter may need.
				qtChild.Printf("postpone filter={%s}, since it took more than %d loops", tf, maxLoopsCount)
				storeFilterLoopsCount(&tfw, 2*filterLoopsCount)
				tfsPostponed = append(tfsPostponed, tf)
				continue
			}
			// Move failing tf to the end of filter list
			storeFilterLoopsCount(&tfw, int64Max)
			return nil, err
		}
		storeFilterLoopsCount(&tfw, filterLoopsCount)
		if tf.isNegative || tf.isEmptyMatch {
			metricIDs.Subtract(m)
			qtChild.Printf("subtract %d metric ids from the found %d metric ids for filter={%s}; resulting metric ids: %d", m.Len(), metricIDsLen, tf, metricIDs.Len())
		} else {
			metricIDs.Intersect(m)
			qtChild.Printf("intersect %d metric ids with the found %d metric ids for filter={%s}; resulting metric ids: %d", m.Len(), metricIDsLen, tf, metricIDs.Len())
		}
	}
	qtChild.Done()
	if metricIDs.Len() == 0 {
		// There is no need in applying tfsPostponed, since the result is empty.
		qt.Printf("found zero metric ids")
		return nil, nil
	}
	if len(tfsPostponed) > 0 {
		// Apply the postponed filters via metricName match.
		qt.Printf("apply postponed filters=%s to %d metrics ids", tfsPostponed, metricIDs.Len())
		var m uint64set.Set
		if err := is.updateMetricIDsByMetricNameMatch(qt, &m, metricIDs, tfsPostponed); err != nil {
			return nil, err
		}
		return &m, nil
	}
	qt.Printf("found %d metric ids", metricIDs.Len())
	return metricIDs, nil
}

const (
	intMax   = int((^uint(0)) >> 1)
	int64Max = int64((1 << 63) - 1)
)

func (is *indexSearch) createPerDayIndexes(date uint64, tsid *TSID, mn *MetricName) {
	ii := getIndexItems()
	defer putIndexItems(ii)

	// Create date -> metricID entry.
	ii.B = marshalCommonPrefix(ii.B, nsPrefixDateToMetricID)
	ii.B = encoding.MarshalUint64(ii.B, date)
	ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
	ii.Next()

	// Create metricName -> TSID entry.
	ii.B = marshalCommonPrefix(ii.B, nsPrefixDateMetricNameToTSID)
	ii.B = encoding.MarshalUint64(ii.B, date)
	ii.B = mn.Marshal(ii.B)
	ii.B = append(ii.B, kvSeparatorChar)
	ii.B = tsid.Marshal(ii.B)
	ii.Next()

	// Create per-day tag -> metricID entries for every tag in mn.
	kb := kbPool.Get()
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateTagToMetricIDs)
	kb.B = encoding.MarshalUint64(kb.B, date)
	ii.registerTagIndexes(kb.B, mn, tsid.MetricID)
	kbPool.Put(kb)

	is.db.tb.AddItems(ii.Items)
}

func (ii *indexItems) registerTagIndexes(prefix []byte, mn *MetricName, metricID uint64) {
	// Add MetricGroup -> metricID entry.
	ii.B = append(ii.B, prefix...)
	ii.B = marshalTagValue(ii.B, nil)
	ii.B = marshalTagValue(ii.B, mn.MetricGroup)
	ii.B = encoding.MarshalUint64(ii.B, metricID)
	ii.Next()
	ii.addReverseMetricGroupIfNeeded(prefix, mn, metricID)

	// Add tag -> metricID entries.
	for _, tag := range mn.Tags {
		ii.B = append(ii.B, prefix...)
		ii.B = tag.Marshal(ii.B)
		ii.B = encoding.MarshalUint64(ii.B, metricID)
		ii.Next()
	}

	// Add index entries for composite tags: MetricGroup+tag -> metricID.
	compositeKey := kbPool.Get()
	for _, tag := range mn.Tags {
		compositeKey.B = marshalCompositeTagKey(compositeKey.B[:0], mn.MetricGroup, tag.Key)
		ii.B = append(ii.B, prefix...)
		ii.B = marshalTagValue(ii.B, compositeKey.B)
		ii.B = marshalTagValue(ii.B, tag.Value)
		ii.B = encoding.MarshalUint64(ii.B, metricID)
		ii.Next()
	}
	kbPool.Put(compositeKey)
}

func (ii *indexItems) addReverseMetricGroupIfNeeded(prefix []byte, mn *MetricName, metricID uint64) {
	if bytes.IndexByte(mn.MetricGroup, '.') < 0 {
		// The reverse metric group is needed only for Graphite-like metrics with points.
		return
	}
	// This is most likely a Graphite metric like 'foo.bar.baz'.
	// Store reverse metric name 'zab.rab.oof' in order to speed up search for '*.bar.baz'
	// when the Graphite wildcard has a suffix matching small number of time series.
	ii.B = append(ii.B, prefix...)
	ii.B = marshalTagValue(ii.B, graphiteReverseTagKey)
	revBuf := kbPool.Get()
	revBuf.B = reverseBytes(revBuf.B[:0], mn.MetricGroup)
	ii.B = marshalTagValue(ii.B, revBuf.B)
	kbPool.Put(revBuf)
	ii.B = encoding.MarshalUint64(ii.B, metricID)
	ii.Next()
}

func isArtificialTagKey(key []byte) bool {
	if bytes.Equal(key, graphiteReverseTagKey) {
		return true
	}
	if len(key) > 0 && key[0] == compositeTagKeyPrefix {
		return true
	}
	return false
}

// The tag key for reverse metric name used for speeding up searching
// for Graphite wildcards with suffix matching small number of time series,
// i.e. '*.bar.baz'.
//
// It is expected that the given key isn't be used by users.
var graphiteReverseTagKey = []byte("\xff")

// The prefix for composite tag, which is used for speeding up searching
// for composite filters, which contain `{__name__="<metric_name>"}` filter.
//
// It is expected that the given prefix isn't used by users.
const compositeTagKeyPrefix = '\xfe'

func marshalCompositeTagKey(dst, name, key []byte) []byte {
	dst = append(dst, compositeTagKeyPrefix)
	dst = encoding.MarshalVarUint64(dst, uint64(len(name)))
	dst = append(dst, name...)
	dst = append(dst, key...)
	return dst
}

func unmarshalCompositeTagKey(src []byte) ([]byte, []byte, error) {
	if len(src) == 0 {
		return nil, nil, fmt.Errorf("composite tag key cannot be empty")
	}
	if src[0] != compositeTagKeyPrefix {
		return nil, nil, fmt.Errorf("missing composite tag key prefix in %q", src)
	}
	src = src[1:]
	n, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		return nil, nil, fmt.Errorf("cannot unmarshal metric name length from composite tag key")
	}
	src = src[nSize:]
	if uint64(len(src)) < n {
		return nil, nil, fmt.Errorf("missing metric name with length %d in composite tag key %q", n, src)
	}
	name := src[:n]
	key := src[n:]
	return name, key, nil
}

func reverseBytes(dst, src []byte) []byte {
	for i := len(src) - 1; i >= 0; i-- {
		dst = append(dst, src[i])
	}
	return dst
}

func (is *indexSearch) hasDateMetricIDNoExtDB(date, metricID uint64) bool {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID)
	kb.B = encoding.MarshalUint64(kb.B, date)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	err := ts.FirstItemWithPrefix(kb.B)
	if err == nil {
		if string(ts.Item) != string(kb.B) {
			logger.Panicf("FATAL: unexpected entry for (date=%s, metricID=%d); got %q; want %q", dateToString(date), metricID, ts.Item, kb.B)
		}
		// Fast path - the (date, metricID) entry is found in the current indexdb.
		return true
	}
	if err != io.EOF {
		logger.Panicf("FATAL: unexpected error when searching for (date=%s, metricID=%d) entry: %s", dateToString(date), metricID, err)
	}
	return false
}

func (is *indexSearch) getMetricIDsForDateTagFilter(qt *querytracer.Tracer, tf *tagFilter, date uint64, commonPrefix []byte,
	maxMetrics int, maxLoopsCount int64) (*uint64set.Set, int64, error) {
	if qt.Enabled() {
		qt = qt.NewChild("get metric ids for filter and date: filter={%s}, date=%s, maxMetrics=%d, maxLoopsCount=%d", tf, dateToString(date), maxMetrics, maxLoopsCount)
		defer qt.Done()
	}
	if !bytes.HasPrefix(tf.prefix, commonPrefix) {
		logger.Panicf("BUG: unexpected tf.prefix %q; must start with commonPrefix %q", tf.prefix, commonPrefix)
	}
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	prefix := kb.B
	kb.B = append(kb.B, tf.prefix[len(commonPrefix):]...)
	tfNew := *tf
	tfNew.isNegative = false // isNegative for the original tf is handled by the caller.
	tfNew.prefix = kb.B
	metricIDs, loopsCount, err := is.getMetricIDsForTagFilter(qt, &tfNew, maxMetrics, maxLoopsCount)
	if err != nil {
		return nil, loopsCount, err
	}
	if tf.isNegative || !tf.isEmptyMatch {
		return metricIDs, loopsCount, nil
	}
	// The tag filter, which matches empty label such as {foo=~"bar|"}
	// Convert it to negative filter, which matches {foo=~".+",foo!~"bar|"}.
	// This fixes https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1601
	// See also https://github.com/VictoriaMetrics/VictoriaMetrics/issues/395
	maxLoopsCount -= loopsCount
	var tfGross tagFilter
	if err := tfGross.Init(prefix, tf.key, []byte(".+"), false, true); err != nil {
		logger.Panicf(`BUG: cannot init tag filter: {%q=~".+"}: %s`, tf.key, err)
	}
	m, lc, err := is.getMetricIDsForTagFilter(qt, &tfGross, maxMetrics, maxLoopsCount)
	loopsCount += lc
	if err != nil {
		return nil, loopsCount, err
	}
	mLen := m.Len()
	m.Subtract(metricIDs)
	qt.Printf("subtract %d metric ids for filter={%s} from %d metric ids for filter={%s}", metricIDs.Len(), &tfNew, mLen, &tfGross)
	qt.Printf("found %d metric ids, spent %d loops", m.Len(), loopsCount)
	return m, loopsCount, nil
}

func (is *indexSearch) getLoopsCountAndTimestampForDateFilter(date uint64, tf *tagFilter) (int64, int64, uint64) {
	is.kb.B = appendDateTagFilterCacheKey(is.kb.B[:0], is.db.name, date, tf)
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	kb.B = is.db.loopsPerDateTagFilterCache.Get(kb.B[:0], is.kb.B)
	if len(kb.B) != 3*8 {
		return 0, 0, 0
	}
	loopsCount := encoding.UnmarshalInt64(kb.B)
	filterLoopsCount := encoding.UnmarshalInt64(kb.B[8:])
	timestamp := encoding.UnmarshalUint64(kb.B[16:])
	return loopsCount, filterLoopsCount, timestamp
}

func (is *indexSearch) storeLoopsCountForDateFilter(date uint64, tf *tagFilter, loopsCount, filterLoopsCount int64) {
	currentTimestamp := fasttime.UnixTimestamp()
	is.kb.B = appendDateTagFilterCacheKey(is.kb.B[:0], is.db.name, date, tf)
	kb := kbPool.Get()
	kb.B = encoding.MarshalInt64(kb.B[:0], loopsCount)
	kb.B = encoding.MarshalInt64(kb.B, filterLoopsCount)
	kb.B = encoding.MarshalUint64(kb.B, currentTimestamp)
	is.db.loopsPerDateTagFilterCache.Set(is.kb.B, kb.B)
	kbPool.Put(kb)
}

func appendDateTagFilterCacheKey(dst []byte, indexDBName string, date uint64, tf *tagFilter) []byte {
	dst = append(dst, indexDBName...)
	dst = encoding.MarshalUint64(dst, date)
	dst = tf.Marshal(dst)
	return dst
}

func (is *indexSearch) getMetricIDsForDate(date uint64, maxMetrics int) (*uint64set.Set, error) {
	// Extract all the metricIDs from (date, __name__=value)->metricIDs entries.
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	kb.B = marshalTagValue(kb.B, nil)
	var metricIDs uint64set.Set
	if err := is.updateMetricIDsForPrefix(kb.B, &metricIDs, maxMetrics); err != nil {
		return nil, err
	}
	return &metricIDs, nil
}

func (is *indexSearch) updateMetricIDsForPrefix(prefix []byte, metricIDs *uint64set.Set, maxMetrics int) error {
	ts := &is.ts
	mp := &is.mp
	loopsPaceLimiter := 0
	ts.Seek(prefix)
	for ts.NextItem() {
		if loopsPaceLimiter&paceLimiterFastIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(is.deadline); err != nil {
				return err
			}
		}
		loopsPaceLimiter++
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return nil
		}
		tail := item[len(prefix):]
		n := bytes.IndexByte(tail, tagSeparatorChar)
		if n < 0 {
			return fmt.Errorf("invalid tag->metricIDs line %q: cannot find tagSeparatorChar %d", item, tagSeparatorChar)
		}
		tail = tail[n+1:]
		if err := mp.InitOnlyTail(item, tail); err != nil {
			return err
		}
		mp.ParseMetricIDs()
		metricIDs.AddMulti(mp.MetricIDs)
		if metricIDs.Len() >= maxMetrics {
			return nil
		}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for all metricIDs by prefix %q: %w", prefix, err)
	}
	return nil
}

// The estimated number of index scan loops a single loop in updateMetricIDsByMetricNameMatch takes.
const loopsCountPerMetricNameMatch = 150

var kbPool bytesutil.ByteBufferPool

// Returns local unique MetricID.
func generateUniqueMetricID() uint64 {
	// It is expected that metricIDs returned from this function must be dense.
	// If they will be sparse, then this may hurt metric_ids intersection
	// performance with uint64set.Set.
	return nextUniqueMetricID.Add(1)
}

// This number mustn't go backwards on restarts, otherwise metricID
// collisions are possible. So don't change time on the server
// between VictoriaMetrics restarts.
var nextUniqueMetricID = func() *atomic.Uint64 {
	var n atomic.Uint64
	n.Store(uint64(time.Now().UnixNano()))
	return &n
}()

func marshalCommonPrefix(dst []byte, nsPrefix byte) []byte {
	dst = append(dst, nsPrefix)
	return dst
}

// This function is needed only for minimizing the difference between code for single-node and cluster version.
func (is *indexSearch) marshalCommonPrefix(dst []byte, nsPrefix byte) []byte {
	return marshalCommonPrefix(dst, nsPrefix)
}

func (is *indexSearch) marshalCommonPrefixForDate(dst []byte, date uint64) []byte {
	if date == 0 {
		// Global index
		return is.marshalCommonPrefix(dst, nsPrefixTagToMetricIDs)
	}
	// Per-day index
	dst = is.marshalCommonPrefix(dst, nsPrefixDateTagToMetricIDs)
	return encoding.MarshalUint64(dst, date)
}

func unmarshalCommonPrefix(src []byte) ([]byte, byte, error) {
	if len(src) < commonPrefixLen {
		return nil, 0, fmt.Errorf("cannot unmarshal common prefix from %d bytes; need at least %d bytes; data=%X", len(src), commonPrefixLen, src)
	}
	prefix := src[0]
	return src[commonPrefixLen:], prefix, nil
}

// 1 byte for prefix
const commonPrefixLen = 1

type tagToMetricIDsRowParser struct {
	// NSPrefix contains the first byte parsed from the row after Init call.
	// This is either nsPrefixTagToMetricIDs or nsPrefixDateTagToMetricIDs.
	NSPrefix byte

	// Date contains parsed date for nsPrefixDateTagToMetricIDs rows after Init call
	Date uint64

	// MetricIDs contains parsed MetricIDs after ParseMetricIDs call
	MetricIDs []uint64

	// metricIDsParsed is set to true after ParseMetricIDs call
	metricIDsParsed bool

	// Tag contains parsed tag after Init call
	Tag Tag

	// tail contains the remaining unparsed metricIDs
	tail []byte
}

func (mp *tagToMetricIDsRowParser) Reset() {
	mp.NSPrefix = 0
	mp.Date = 0
	mp.MetricIDs = mp.MetricIDs[:0]
	mp.metricIDsParsed = false
	mp.Tag.Reset()
	mp.tail = nil
}

// Init initializes mp from b, which should contain encoded tag->metricIDs row.
//
// b cannot be re-used until Reset call.
func (mp *tagToMetricIDsRowParser) Init(b []byte, nsPrefixExpected byte) error {
	tail, nsPrefix, err := unmarshalCommonPrefix(b)
	if err != nil {
		return fmt.Errorf("invalid tag->metricIDs row %q: %w", b, err)
	}
	if nsPrefix != nsPrefixExpected {
		return fmt.Errorf("invalid prefix for tag->metricIDs row %q; got %d; want %d", b, nsPrefix, nsPrefixExpected)
	}
	if nsPrefix == nsPrefixDateTagToMetricIDs {
		// unmarshal date.
		if len(tail) < 8 {
			return fmt.Errorf("cannot unmarshal date from (date, tag)->metricIDs row %q from %d bytes; want at least 8 bytes", b, len(tail))
		}
		mp.Date = encoding.UnmarshalUint64(tail)
		tail = tail[8:]
	}
	mp.NSPrefix = nsPrefix
	tail, err = mp.Tag.Unmarshal(tail)
	if err != nil {
		return fmt.Errorf("cannot unmarshal tag from tag->metricIDs row %q: %w", b, err)
	}
	return mp.InitOnlyTail(b, tail)
}

// MarshalPrefix marshals row prefix without tail to dst.
func (mp *tagToMetricIDsRowParser) MarshalPrefix(dst []byte) []byte {
	dst = marshalCommonPrefix(dst, mp.NSPrefix)
	if mp.NSPrefix == nsPrefixDateTagToMetricIDs {
		dst = encoding.MarshalUint64(dst, mp.Date)
	}
	dst = mp.Tag.Marshal(dst)
	return dst
}

// InitOnlyTail initializes mp.tail from tail.
//
// b must contain tag->metricIDs row.
// b cannot be re-used until Reset call.
func (mp *tagToMetricIDsRowParser) InitOnlyTail(b, tail []byte) error {
	if len(tail) == 0 {
		return fmt.Errorf("missing metricID in the tag->metricIDs row %q", b)
	}
	if len(tail)%8 != 0 {
		return fmt.Errorf("invalid tail length in the tag->metricIDs row; got %d bytes; must be multiple of 8 bytes", len(tail))
	}
	mp.tail = tail
	mp.metricIDsParsed = false
	return nil
}

// EqualPrefix returns true if prefixes for mp and x are equal.
//
// Prefix contains (tag)
func (mp *tagToMetricIDsRowParser) EqualPrefix(x *tagToMetricIDsRowParser) bool {
	if !mp.Tag.Equal(&x.Tag) {
		return false
	}
	return mp.Date == x.Date && mp.NSPrefix == x.NSPrefix
}

// MetricIDsLen returns the number of MetricIDs in the mp.tail
func (mp *tagToMetricIDsRowParser) MetricIDsLen() int {
	return len(mp.tail) / 8
}

// ParseMetricIDs parses MetricIDs from mp.tail into mp.MetricIDs.
func (mp *tagToMetricIDsRowParser) ParseMetricIDs() {
	if mp.metricIDsParsed {
		return
	}
	tail := mp.tail
	n := len(tail) / 8
	mp.MetricIDs = slicesutil.SetLength(mp.MetricIDs, n)
	metricIDs := mp.MetricIDs
	_ = metricIDs[n-1]
	for i := 0; i < n; i++ {
		if len(tail) < 8 {
			logger.Panicf("BUG: tail cannot be smaller than 8 bytes; got %d bytes; tail=%X", len(tail), tail)
			return
		}
		metricID := encoding.UnmarshalUint64(tail)
		metricIDs[i] = metricID
		tail = tail[8:]
	}
	mp.metricIDsParsed = true
}

// GetMatchingSeriesCount returns the number of series in mp, which match metricIDs from the given filter
// and do not match metricIDs from negativeFilter.
//
// if filter is empty, then all series in mp are taken into account.
func (mp *tagToMetricIDsRowParser) GetMatchingSeriesCount(filter, negativeFilter *uint64set.Set) int {
	if filter == nil && negativeFilter.Len() == 0 {
		return mp.MetricIDsLen()
	}
	mp.ParseMetricIDs()
	n := 0
	for _, metricID := range mp.MetricIDs {
		if filter != nil && !filter.Has(metricID) {
			continue
		}
		if !negativeFilter.Has(metricID) {
			n++
		}
	}
	return n
}

func mergeTagToMetricIDsRows(data []byte, items []mergeset.Item) ([]byte, []mergeset.Item) {
	data, items = mergeTagToMetricIDsRowsInternal(data, items, nsPrefixTagToMetricIDs)
	data, items = mergeTagToMetricIDsRowsInternal(data, items, nsPrefixDateTagToMetricIDs)
	return data, items
}

func mergeTagToMetricIDsRowsInternal(data []byte, items []mergeset.Item, nsPrefix byte) ([]byte, []mergeset.Item) {
	// Perform quick checks whether items contain rows starting from nsPrefix
	// based on the fact that items are sorted.
	if len(items) <= 2 {
		// The first and the last row must remain unchanged.
		return data, items
	}
	firstItem := items[0].Bytes(data)
	if len(firstItem) > 0 && firstItem[0] > nsPrefix {
		return data, items
	}
	lastItem := items[len(items)-1].Bytes(data)
	if len(lastItem) > 0 && lastItem[0] < nsPrefix {
		return data, items
	}

	// items contain at least one row starting from nsPrefix. Merge rows with common tag.
	tmm := getTagToMetricIDsRowsMerger()
	tmm.dataCopy = append(tmm.dataCopy[:0], data...)
	tmm.itemsCopy = append(tmm.itemsCopy[:0], items...)
	mp := &tmm.mp
	mpPrev := &tmm.mpPrev
	dstData := data[:0]
	dstItems := items[:0]
	for i, it := range items {
		item := it.Bytes(data)
		if len(item) == 0 || item[0] != nsPrefix || i == 0 || i == len(items)-1 {
			// Write rows not starting with nsPrefix as-is.
			// Additionally write the first and the last row as-is in order to preserve
			// sort order for adjacent blocks.
			dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
			dstData = append(dstData, item...)
			dstItems = append(dstItems, mergeset.Item{
				Start: uint32(len(dstData) - len(item)),
				End:   uint32(len(dstData)),
			})
			continue
		}
		if err := mp.Init(item, nsPrefix); err != nil {
			logger.Panicf("FATAL: cannot parse row starting with nsPrefix %d during merge: %s", nsPrefix, err)
		}
		if mp.MetricIDsLen() >= maxMetricIDsPerRow {
			dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
			dstData = append(dstData, item...)
			dstItems = append(dstItems, mergeset.Item{
				Start: uint32(len(dstData) - len(item)),
				End:   uint32(len(dstData)),
			})
			continue
		}
		if !mp.EqualPrefix(mpPrev) {
			dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
		}
		mp.ParseMetricIDs()
		tmm.pendingMetricIDs = append(tmm.pendingMetricIDs, mp.MetricIDs...)
		mpPrev, mp = mp, mpPrev
		if len(tmm.pendingMetricIDs) >= maxMetricIDsPerRow {
			dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
		}
	}
	if len(tmm.pendingMetricIDs) > 0 {
		logger.Panicf("BUG: tmm.pendingMetricIDs must be empty at this point; got %d items: %d", len(tmm.pendingMetricIDs), tmm.pendingMetricIDs)
	}
	if !checkItemsSorted(dstData, dstItems) {
		// Items could become unsorted if initial items contain duplicate metricIDs:
		//
		//   item1: 1, 1, 5
		//   item2: 1, 4
		//
		// Items could become the following after the merge:
		//
		//   item1: 1, 5
		//   item2: 1, 4
		//
		// i.e. item1 > item2
		//
		// Leave the original items unmerged, so they can be merged next time.
		// This case should be quite rare - if multiple data points are simultaneously inserted
		// into the same new time series from multiple concurrent goroutines.
		indexBlocksWithMetricIDsIncorrectOrder.Add(1)
		dstData = append(dstData[:0], tmm.dataCopy...)
		dstItems = append(dstItems[:0], tmm.itemsCopy...)
		if !checkItemsSorted(dstData, dstItems) {
			logger.Panicf("BUG: the original items weren't sorted; items=%q", dstItems)
		}
	}
	putTagToMetricIDsRowsMerger(tmm)
	indexBlocksWithMetricIDsProcessed.Add(1)
	return dstData, dstItems
}

var (
	indexBlocksWithMetricIDsIncorrectOrder atomic.Uint64
	indexBlocksWithMetricIDsProcessed      atomic.Uint64
)

func checkItemsSorted(data []byte, items []mergeset.Item) bool {
	if len(items) == 0 {
		return true
	}
	prevItem := items[0].String(data)
	for _, it := range items[1:] {
		currItem := it.String(data)
		if prevItem > currItem {
			return false
		}
		prevItem = currItem
	}
	return true
}

// maxMetricIDsPerRow limits the number of metricIDs in tag->metricIDs row.
//
// This reduces overhead on index and metaindex in lib/mergeset.
const maxMetricIDsPerRow = 64

type uint64Sorter []uint64

func (s uint64Sorter) Len() int { return len(s) }
func (s uint64Sorter) Less(i, j int) bool {
	return s[i] < s[j]
}

func (s uint64Sorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type tagToMetricIDsRowsMerger struct {
	pendingMetricIDs uint64Sorter
	mp               tagToMetricIDsRowParser
	mpPrev           tagToMetricIDsRowParser

	itemsCopy []mergeset.Item
	dataCopy  []byte
}

func (tmm *tagToMetricIDsRowsMerger) Reset() {
	tmm.pendingMetricIDs = tmm.pendingMetricIDs[:0]
	tmm.mp.Reset()
	tmm.mpPrev.Reset()

	tmm.itemsCopy = tmm.itemsCopy[:0]
	tmm.dataCopy = tmm.dataCopy[:0]
}

func (tmm *tagToMetricIDsRowsMerger) flushPendingMetricIDs(dstData []byte, dstItems []mergeset.Item, mp *tagToMetricIDsRowParser) ([]byte, []mergeset.Item) {
	if len(tmm.pendingMetricIDs) == 0 {
		// Nothing to flush
		return dstData, dstItems
	}
	// Use sort.Sort instead of sort.Slice in order to reduce memory allocations.
	sort.Sort(&tmm.pendingMetricIDs)
	tmm.pendingMetricIDs = removeDuplicateMetricIDs(tmm.pendingMetricIDs)

	// Marshal pendingMetricIDs
	dstDataLen := len(dstData)
	dstData = mp.MarshalPrefix(dstData)
	for _, metricID := range tmm.pendingMetricIDs {
		dstData = encoding.MarshalUint64(dstData, metricID)
	}
	dstItems = append(dstItems, mergeset.Item{
		Start: uint32(dstDataLen),
		End:   uint32(len(dstData)),
	})
	tmm.pendingMetricIDs = tmm.pendingMetricIDs[:0]
	return dstData, dstItems
}

func removeDuplicateMetricIDs(sortedMetricIDs []uint64) []uint64 {
	if len(sortedMetricIDs) < 2 {
		return sortedMetricIDs
	}
	prevMetricID := sortedMetricIDs[0]
	hasDuplicates := false
	for _, metricID := range sortedMetricIDs[1:] {
		if prevMetricID == metricID {
			hasDuplicates = true
			break
		}
		prevMetricID = metricID
	}
	if !hasDuplicates {
		return sortedMetricIDs
	}
	dstMetricIDs := sortedMetricIDs[:1]
	prevMetricID = sortedMetricIDs[0]
	for _, metricID := range sortedMetricIDs[1:] {
		if prevMetricID == metricID {
			continue
		}
		dstMetricIDs = append(dstMetricIDs, metricID)
		prevMetricID = metricID
	}
	return dstMetricIDs
}

func getTagToMetricIDsRowsMerger() *tagToMetricIDsRowsMerger {
	v := tmmPool.Get()
	if v == nil {
		return &tagToMetricIDsRowsMerger{}
	}
	return v.(*tagToMetricIDsRowsMerger)
}

func putTagToMetricIDsRowsMerger(tmm *tagToMetricIDsRowsMerger) {
	tmm.Reset()
	tmmPool.Put(tmm)
}

var tmmPool sync.Pool
