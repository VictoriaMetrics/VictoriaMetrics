package storage

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/lrucache"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

const (
	// Prefix for MetricName->TSID entries.
	//
	// This index is used only when -disablePerDayIndex flag is set.
	//
	// Otherwise, this index is substituted with nsPrefixDateMetricNameToTSID,
	// since the MetricName->TSID index may require big amounts of memory for
	// indexdb/dataBlocks cache when it grows big on the configured retention
	// under high churn rate (e.g. when new time series are constantly
	// registered).
	//
	// It is much more efficient from memory usage PoV to query per-day MetricName->TSID index
	// (aka nsPrefixDateMetricNameToTSID) when the TSID must be obtained for the given MetricName
	// during data ingestion under high churn rate and big retention.
	//
	nsPrefixMetricNameToTSID = 0

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
	// The number of calls for date range searches.
	dateRangeSearchCalls atomic.Uint64

	// The number of hits for date range searches.
	dateRangeSearchHits atomic.Uint64

	// The number of calls for global search.
	globalSearchCalls atomic.Uint64

	// The number of missing MetricID -> TSID entries.
	// High rate for this value means corrupted indexDB.
	missingTSIDsForMetricID atomic.Uint64

	// missingMetricNamesForMetricID is a counter of missing MetricID -> MetricName entries.
	// High rate may mean corrupted indexDB due to unclean shutdown.
	// The db must be automatically recovered after that.
	missingMetricNamesForMetricID atomic.Uint64

	// legacyMinMissingTimestampByKey holds the minimum timestamps by index search key,
	// which is missing in the given indexDB.
	// Key must be formed with marshalCommonPrefix function.
	//
	// This field is used at legacyContainsTimeRange() function only for the
	// legacy indexDBs, since these indexDBs are readonly.
	// This field cannot be used for the partition indexDBs, since they may receive data
	// with bigger timestamps at any time.
	legacyMinMissingTimestampByKey map[string]int64
	// protects legacyMinMissingTimestampByKey
	legacyMinMissingTimestampByKeyLock sync.Mutex

	// id identifies the indexDB. It is used for in various caches to know which
	// indexDB contains a metricID and which does not.
	id uint64

	// Time range covered by this IndexDB.
	tr TimeRange

	name string
	tb   *mergeset.Table

	// The parent storage. Provides state and configuration shared between all
	// indexDB instances.
	s *Storage

	// noRegisterNewSeries indicates whether the indexDB receives new entries or
	// not.
	noRegisterNewSeries atomic.Bool

	// Cache for fast TagFilters -> MetricIDs lookup.
	tagFiltersToMetricIDsCache *lrucache.Cache

	// Cache for (date, tagFilter) -> loopsCount, which is used for reducing
	// the amount of work when matching a set of filters.
	loopsPerDateTagFilterCache *lrucache.Cache

	// A cache that stores metricIDs that have been added to the index.
	// The cache is not populated on startup nor does it store a complete set of
	// metricIDs. A metricID is added to the cache either when a new entry is
	// added to the global index or when the global index is searched for
	// existing metricID (see is.createGlobalIndexes() and is.hasMetricID()).
	//
	// The cache is used solely for creating new index entries during the data
	// ingestion (see Storage.RegisterMetricNames() and Storage.add())
	metricIDCache *metricIDCache

	// dateMetricIDCache is (date, metricID) cache that is used to speed up the
	// data ingestion by storing the is.hasDateMetricID() search results in
	// memory.
	dateMetricIDCache *dateMetricIDCache

	// An inmemory set of deleted metricIDs.
	deletedMetricIDs           atomic.Pointer[uint64set.Set]
	deletedMetricIDsUpdateLock sync.Mutex

	indexSearchPool sync.Pool
}

var maxTagFiltersCacheSize uint64

// SetTagFiltersCacheSize overrides the default size of tagFiltersToMetricIDsCache
func SetTagFiltersCacheSize(size int) {
	maxTagFiltersCacheSize = uint64(size)
}

func getTagFiltersCacheSize() uint64 {
	if maxTagFiltersCacheSize <= 0 {
		return uint64(float64(memory.Allowed()) / 32)
	}
	return maxTagFiltersCacheSize
}

func getTagFiltersLoopsCacheSize() uint64 {
	return uint64(float64(memory.Allowed()) / 128)
}

var maxMetricIDsForDirectLabelsLookup int = 100e3

func mustOpenIndexDB(id uint64, tr TimeRange, name, path string, s *Storage, isReadOnly *atomic.Bool, noRegisterNewSeries bool) *indexDB {
	if s == nil {
		logger.Panicf("BUG: Storage must not be nil")
	}

	tfssCache := lrucache.NewCache(getTagFiltersCacheSize)
	tb := mergeset.MustOpenTable(path, dataFlushInterval, tfssCache.Reset, mergeTagToMetricIDsRows, isReadOnly)
	db := &indexDB{
		legacyMinMissingTimestampByKey: make(map[string]int64),
		id:                             id,
		tr:                             tr,
		name:                           name,
		tb:                             tb,
		s:                              s,
		tagFiltersToMetricIDsCache:     tfssCache,
		loopsPerDateTagFilterCache:     lrucache.NewCache(getTagFiltersLoopsCacheSize),
		metricIDCache:                  newMetricIDCache(),
		dateMetricIDCache:              newDateMetricIDCache(),
	}
	db.noRegisterNewSeries.Store(noRegisterNewSeries)
	db.mustLoadDeletedMetricIDs()
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
	TagFiltersToMetricIDsCacheResets       uint64

	MetricIDCacheSize           uint64
	MetricIDCacheSizeBytes      uint64
	MetricIDCacheSyncsCount     uint64
	MetricIDCacheRotationsCount uint64

	DateMetricIDCacheSize           uint64
	DateMetricIDCacheSizeBytes      uint64
	DateMetricIDCacheSyncsCount     uint64
	DateMetricIDCacheRotationsCount uint64

	// Used by legacy indexDBs only.
	// See UpdateMetrics() in index_db_legacy.go
	IndexDBRefCount uint64

	RecentHourMetricIDsSearchCalls uint64
	RecentHourMetricIDsSearchHits  uint64

	DateRangeSearchCalls uint64
	DateRangeSearchHits  uint64
	GlobalSearchCalls    uint64

	MissingTSIDsForMetricID       uint64
	MissingMetricNamesForMetricID uint64

	IndexBlocksWithMetricIDsProcessed      uint64
	IndexBlocksWithMetricIDsIncorrectOrder uint64

	MinTimestampForCompositeIndex     uint64
	CompositeFilterSuccessConversions uint64
	CompositeFilterMissingConversions uint64

	mergeset.TableMetrics
}

// UpdateMetrics updates m with metrics from the db.
func (db *indexDB) UpdateMetrics(m *IndexDBMetrics) {
	// global index metrics
	m.IndexBlocksWithMetricIDsProcessed = indexBlocksWithMetricIDsProcessed.Load()
	m.IndexBlocksWithMetricIDsIncorrectOrder = indexBlocksWithMetricIDsIncorrectOrder.Load()

	m.MinTimestampForCompositeIndex = uint64(db.s.minTimestampForCompositeIndex)
	m.CompositeFilterSuccessConversions = compositeFilterSuccessConversions.Load()
	m.CompositeFilterMissingConversions = compositeFilterMissingConversions.Load()

	// Report only once and either for the first met indexDB instance or whose
	// tagFiltersCache is utilized the most.
	//
	// In case of tagFiltersCache, use TagFiltersToMetricIDsCacheRequests as an
	// indicator that this is the first indexDB instance whose metrics are being
	// collected because this cache may be reset too often.
	if m.TagFiltersToMetricIDsCacheRequests == 0 || db.tagFiltersToMetricIDsCache.SizeBytes() > m.TagFiltersToMetricIDsCacheSizeBytes {
		m.TagFiltersToMetricIDsCacheSize = uint64(db.tagFiltersToMetricIDsCache.Len())
		m.TagFiltersToMetricIDsCacheSizeBytes = db.tagFiltersToMetricIDsCache.SizeBytes()
		m.TagFiltersToMetricIDsCacheSizeMaxBytes = db.tagFiltersToMetricIDsCache.SizeMaxBytes()
		m.TagFiltersToMetricIDsCacheRequests = db.tagFiltersToMetricIDsCache.Requests()
		m.TagFiltersToMetricIDsCacheMisses = db.tagFiltersToMetricIDsCache.Misses()
		m.TagFiltersToMetricIDsCacheResets = db.tagFiltersToMetricIDsCache.Resets()
	}

	// Report only once and for either the first met indexDB instance or whose
	// metricIDCache is utilized the most.
	mcs := db.metricIDCache.Stats()
	if m.MetricIDCacheSizeBytes == 0 || mcs.SizeBytes > m.MetricIDCacheSizeBytes {
		m.MetricIDCacheSize = mcs.Size
		m.MetricIDCacheSizeBytes = mcs.SizeBytes
		m.MetricIDCacheSyncsCount = mcs.SyncsCount
		m.MetricIDCacheRotationsCount = mcs.RotationsCount
	}

	// Report only once and for either the first met indexDB instance or whose
	// dateMetricIDCache is utilized the most.
	dmcs := db.dateMetricIDCache.Stats()
	if m.DateMetricIDCacheSizeBytes == 0 || dmcs.SizeBytes > m.DateMetricIDCacheSizeBytes {
		m.DateMetricIDCacheSize = dmcs.Size
		m.DateMetricIDCacheSizeBytes = dmcs.SizeBytes
		m.DateMetricIDCacheSyncsCount = dmcs.SyncsCount
		m.DateMetricIDCacheRotationsCount = dmcs.RotationsCount
	}

	m.DateRangeSearchCalls += db.dateRangeSearchCalls.Load()
	m.DateRangeSearchHits += db.dateRangeSearchHits.Load()
	m.GlobalSearchCalls += db.globalSearchCalls.Load()

	m.MissingTSIDsForMetricID += db.missingTSIDsForMetricID.Load()
	m.MissingMetricNamesForMetricID += db.missingMetricNamesForMetricID.Load()

	db.tb.UpdateMetrics(&m.TableMetrics)
}

// MustClose closes db.
func (db *indexDB) MustClose() {
	db.tb.MustClose()
	db.tb = nil
	db.s = nil

	// Free space occupied by caches owned by db.
	db.tagFiltersToMetricIDsCache.MustStop()
	db.loopsPerDateTagFilterCache.MustStop()
	db.metricIDCache.MustStop()
	db.dateMetricIDCache.MustStop()

	db.tagFiltersToMetricIDsCache = nil
	db.loopsPerDateTagFilterCache = nil
	db.metricIDCache = nil
	db.dateMetricIDCache = nil
}

// getMetricIDsFromTagFiltersCache retrieves the set of metricIDs that
// correspond to the given (tffs, tr) key.
//
// The caller must convert the (tfss, tr) to a byte slice and use it as the key
// when calling this method (see marshalTagFiltersKey()).
//
// The caller must not modify the set of metricIDs returned by this method.
func (db *indexDB) getMetricIDsFromTagFiltersCache(qt *querytracer.Tracer, key []byte) (*uint64set.Set, bool) {
	qt.Printf("search for metricIDs in tag filters cache")
	v := db.tagFiltersToMetricIDsCache.GetEntry(bytesutil.ToUnsafeString(key))
	if v == nil {
		qt.Printf("cache miss")
		return nil, false
	}
	metricIDs := v.(*uint64set.Set)
	qt.Printf("found %d metricIDs in cache", metricIDs.Len())
	return metricIDs, true
}

// putMetricIDsToTagFiltersCache stores the set of metricIDs that
// correspond to the given (tffs, tr) key into the cache.
//
// The caller must convert the (tfss, tr) to a byte slice and use it as the key
// when calling this method (see marshalTagFiltersKey()).
//
// The caller must not modify the set of metricIDs after calling this method.
func (db *indexDB) putMetricIDsToTagFiltersCache(qt *querytracer.Tracer, metricIDs *uint64set.Set, key []byte) {
	qt.Printf("put %d metricIDs in cache", metricIDs.Len())
	db.tagFiltersToMetricIDsCache.PutEntry(string(key), metricIDs)
	qt.Printf("stored %d metricIDs into cache", metricIDs.Len())
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

func marshalTagFiltersKey(dst []byte, tfss []*TagFilters, tr TimeRange) []byte {
	// Round start and end times to per-day granularity according to per-day inverted index.
	startDate, endDate := tr.DateRange()
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

type indexSearch struct {
	db *indexDB
	ts mergeset.TableSearch
	kb bytesutil.ByteBuffer
	mp tagToMetricIDsRowParser

	// deadline in unix timestamp seconds for the given search.
	deadline uint64
}

// getIndexSearch returns an indexSearch with default configuration
func (db *indexDB) getIndexSearch(deadline uint64) *indexSearch {
	return db.getIndexSearchInternal(deadline, false)
}

func (db *indexDB) getIndexSearchInternal(deadline uint64, sparse bool) *indexSearch {
	v := db.indexSearchPool.Get()
	if v == nil {
		v = &indexSearch{
			db: db,
		}
	}
	is := v.(*indexSearch)
	is.ts.Init(db.tb, sparse)
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

func (db *indexDB) createGlobalIndexes(tsid *TSID, mn *MetricName) {
	if db.noRegisterNewSeries.Load() {
		logger.Panicf("BUG: registration of new series is disabled for indexDB %q", db.name)
	}

	// Add new metricID to cache.
	db.metricIDCache.Set(tsid.MetricID)

	ii := getIndexItems()
	defer putIndexItems(ii)

	if db.s.disablePerDayIndex {
		// Create metricName -> TSID entry.
		// This index is used for searching a TSID by metric name during data
		// ingestion or metric name registration when -disablePerDayIndex flag
		// is set.
		ii.B = marshalCommonPrefix(ii.B, nsPrefixMetricNameToTSID)
		ii.B = mn.Marshal(ii.B)
		ii.B = append(ii.B, kvSeparatorChar)
		ii.B = tsid.Marshal(ii.B)
		ii.Next()
	}

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

	db.tb.AddItems(ii.Items)
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

// SearchLabelNames returns all the label names, which match the given tfss on
// the given tr.
func (db *indexDB) SearchLabelNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int, deadline uint64) (map[string]struct{}, error) {
	qt = qt.NewChild("search for label names: filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", tfss, &tr, maxLabelNames, maxMetrics)
	defer qt.Done()

	is := db.getIndexSearch(deadline)
	lns, err := is.searchLabelNamesWithFiltersOnTimeRange(qt, tfss, tr, maxLabelNames, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	qt.Printf("found %d label names", len(lns))
	return lns, nil
}

func (is *indexSearch) searchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxLabelNames, maxMetrics int) (map[string]struct{}, error) {
	if tr == globalIndexTimeRange {
		qtChild := qt.NewChild("search for label names in global index: filters=%s", tfss)
		lns, err := is.searchLabelNamesWithFiltersOnDate(qtChild, tfss, globalIndexDate, maxLabelNames, maxMetrics)
		qtChild.Done()
		return lns, err
	}

	minDate, maxDate := tr.DateRange()
	var mu sync.Mutex
	wg := getWaitGroup()
	var errGlobal error
	lns := make(map[string]struct{})
	qt = qt.NewChild("parallel search for label names: filters=%s, timeRange=%s", tfss, &tr)
	for date := minDate; date <= maxDate; date++ {
		wg.Add(1)
		qtChild := qt.NewChild("search for label names: filters=%s, date=%s", tfss, dateToString(date))
		go func(date uint64) {
			defer func() {
				qtChild.Done()
				wg.Done()
			}()
			isLocal := is.db.getIndexSearch(is.deadline)
			lnsLocal, err := isLocal.searchLabelNamesWithFiltersOnDate(qtChild, tfss, date, maxLabelNames, maxMetrics)
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
	return lns, errGlobal
}

func (is *indexSearch) searchLabelNamesWithFiltersOnDate(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, maxLabelNames, maxMetrics int) (map[string]struct{}, error) {
	var filter *uint64set.Set
	if !isSingleMetricNameFilter(tfss) {
		var err error
		filter, err = is.searchMetricIDsWithFiltersOnDate(qt, tfss, date, maxMetrics)
		if err != nil {
			return nil, err
		}
		if filter != nil && filter.Len() <= maxMetricIDsForDirectLabelsLookup {
			// It is faster to obtain label names by metricIDs from the filter
			// instead of scanning the inverted index for the matching filters.
			// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978
			metricIDs := filter.AppendTo(nil)
			qt.Printf("sort %d metricIDs", len(metricIDs))
			lns := is.getLabelNamesForMetricIDs(qt, metricIDs, maxLabelNames)
			return lns, nil
		}
	}

	var prevLabelName []byte
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	dmis := is.db.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	underscoreNameSeen := false
	nsPrefixExpected := byte(nsPrefixDateTagToMetricIDs)
	if date == globalIndexDate {
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
	lns := make(map[string]struct{})
	for len(lns) < maxLabelNames && ts.NextItem() {
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
				return nil, fmt.Errorf("cannot unmarshal composite tag key: %s", err)
			}
			lns[string(key)] = struct{}{}
		}
		prevLabelName = append(prevLabelName[:0], labelName...)
	}
	if underscoreNameSeen {
		lns["__name__"] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return nil, fmt.Errorf("error during search for prefix %q: %w", prefix, err)
	}
	return lns, nil
}

func (is *indexSearch) getLabelNamesForMetricIDs(qt *querytracer.Tracer, metricIDs []uint64, maxLabelNames int) map[string]struct{} {
	lns := make(map[string]struct{})
	if len(metricIDs) > 0 {
		lns["__name__"] = struct{}{}
	}

	dmis := is.db.getDeletedMetricIDs()

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
					return lns
				}
			}
		}
	}
	qt.Printf("get %d distinct label names from %d metricIDs", foundLabelNames, len(metricIDs))
	return lns
}

// SearchLabelValues returns label values for the given labelName, tfss and tr.
func (db *indexDB) SearchLabelValues(qt *querytracer.Tracer, labelName string, tfss []*TagFilters, tr TimeRange, maxLabelValues, maxMetrics int, deadline uint64) (map[string]struct{}, error) {
	qt = qt.NewChild("search for label values: labelName=%q, filters=%s, timeRange=%s, maxLabelNames=%d, maxMetrics=%d", labelName, tfss, &tr, maxLabelValues, maxMetrics)
	defer qt.Done()

	key := labelName
	if key == "__name__" {
		key = ""
	}
	if len(tfss) == 1 && len(tfss[0].tfs) == 1 && string(tfss[0].tfs[0].key) == key {
		// tfss contains only a single filter on labelName. It is faster searching for label values
		// without any filters and limits and then later applying the filter and the limit to the found label values.
		qt.Printf("search for up to %d values for the label %q on the time range %s", maxMetrics, labelName, &tr)

		is := db.getIndexSearch(deadline)
		lvs, err := is.searchLabelValuesOnTimeRange(qt, labelName, nil, tr, maxMetrics, maxMetrics)
		db.putIndexSearch(is)
		if err != nil {
			return nil, err
		}

		needSlowSearch := len(lvs) == maxMetrics

		lvsLen := len(lvs)
		filterLabelValues(lvs, &tfss[0].tfs[0], key)
		qt.Printf("found %d out of %d values for the label %q after filtering", len(lvs), lvsLen, labelName)
		if len(lvs) >= maxLabelValues {
			qt.Printf("leave %d out of %d values for the label %q because of the limit", maxLabelValues, len(lvs), labelName)

			// We found at least maxLabelValues unique values for the label with the given filters.
			// It is OK returning all these values instead of falling back to the slow search.
			needSlowSearch = false
		}
		if !needSlowSearch {
			qt.Printf("found %d label values", len(lvs))
			return lvs, nil
		}
		qt.Printf("fall back to slow search because only a subset of label values is found")
	}

	is := db.getIndexSearch(deadline)
	lvs, err := is.searchLabelValuesOnTimeRange(qt, labelName, tfss, tr, maxMetrics, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	qt.Printf("found %d label values", len(lvs))
	return lvs, nil
}

func filterLabelValues(lvs map[string]struct{}, tf *tagFilter, key string) {
	var b []byte
	for lv := range lvs {
		b = marshalCommonPrefix(b[:0], nsPrefixTagToMetricIDs)
		b = marshalTagValue(b, bytesutil.ToUnsafeBytes(key))
		b = marshalTagValue(b, bytesutil.ToUnsafeBytes(lv))
		ok, err := tf.match(b)
		if err != nil {
			logger.Panicf("BUG: cannot match label %q=%q with tagFilter %s: %w", key, lv, tf.String(), err)
		}
		if !ok {
			delete(lvs, lv)
		}
	}
}

func (is *indexSearch) searchLabelValuesOnTimeRange(qt *querytracer.Tracer, labelName string, tfss []*TagFilters, tr TimeRange, maxLabelValues, maxMetrics int) (map[string]struct{}, error) {
	if tr == globalIndexTimeRange {
		qtChild := qt.NewChild("search for label values in global index: labelName=%q, filters=%s", labelName, tfss)
		lvs, err := is.searchLabelValuesOnDate(qtChild, labelName, tfss, globalIndexDate, maxLabelValues, maxMetrics)
		qtChild.Done()

		// Skip empty values, since they have no any meaning.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
		delete(lvs, "")

		return lvs, err
	}

	minDate, maxDate := tr.DateRange()
	var mu sync.Mutex
	wg := getWaitGroup()
	var errGlobal error
	lvs := make(map[string]struct{})
	qt = qt.NewChild("parallel search for label values: labelName=%q, filters=%s, timeRange=%s", labelName, tfss, &tr)
	for date := minDate; date <= maxDate; date++ {
		wg.Add(1)
		qtChild := qt.NewChild("search for label values: filters=%s, date=%s", tfss, dateToString(date))
		go func(date uint64) {
			defer func() {
				qtChild.Done()
				wg.Done()
			}()
			isLocal := is.db.getIndexSearch(is.deadline)
			lvsLocal, err := isLocal.searchLabelValuesOnDate(qtChild, labelName, tfss, date, maxLabelValues, maxMetrics)
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

	// Skip empty values, since they have no any meaning.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
	delete(lvs, "")

	return lvs, errGlobal
}

func (is *indexSearch) searchLabelValuesOnDate(qt *querytracer.Tracer, labelName string, tfss []*TagFilters, date uint64, maxLabelValues, maxMetrics int) (map[string]struct{}, error) {
	if labelName == "__name__" {
		// __name__ label is encoded as empty string in indexdb.
		labelName = ""
	}
	useCompositeScan := labelName != "" && isSingleMetricNameFilter(tfss)
	var filter *uint64set.Set
	if !useCompositeScan {
		var err error
		filter, err = is.searchMetricIDsWithFiltersOnDate(qt, tfss, date, maxMetrics)
		if err != nil {
			return nil, err
		}
		if filter != nil && filter.Len() <= maxMetricIDsForDirectLabelsLookup {
			// It is faster to obtain label values by metricIDs from the filter
			// instead of scanning the inverted index for the matching filters.
			// This should help https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2978
			metricIDs := filter.AppendTo(nil)
			qt.Printf("sort %d metricIDs", len(metricIDs))
			lvs := is.getLabelValuesForMetricIDs(qt, labelName, metricIDs, maxLabelValues)
			return lvs, nil
		}
	}

	labelNameBytes := bytesutil.ToUnsafeBytes(labelName)
	if name := getCommonMetricNameForTagFilterss(tfss); len(name) > 0 && labelName != "" {
		labelNameBytes = marshalCompositeTagKey(nil, name, labelNameBytes)
	}

	lvs := make(map[string]struct{})
	var prevLabelValue []byte
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	dmis := is.db.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	nsPrefixExpected := byte(nsPrefixDateTagToMetricIDs)
	if date == globalIndexDate {
		nsPrefixExpected = nsPrefixTagToMetricIDs
	}
	kb.B = is.marshalCommonPrefixForDate(kb.B[:0], date)
	kb.B = marshalTagValue(kb.B, labelNameBytes)
	prefix := append([]byte{}, kb.B...)
	ts.Seek(prefix)
	for len(lvs) < maxLabelValues && ts.NextItem() {
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
		return nil, fmt.Errorf("error when searching for tag name prefix %q: %w", prefix, err)
	}
	return lvs, nil
}

func (is *indexSearch) getLabelValuesForMetricIDs(qt *querytracer.Tracer, labelName string, metricIDs []uint64, maxLabelValues int) map[string]struct{} {
	if labelName == "" {
		labelName = "__name__"
	}

	lvs := make(map[string]struct{})
	dmis := is.db.getDeletedMetricIDs()
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
				return lvs
			}
		}
	}
	qt.Printf("get %d distinct values for label %q from %d metricIDs", foundLabelValues, labelName, len(metricIDs))
	return lvs
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given tagKey and tagValuePrefix on the given tr.
//
// This allows implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or similar APIs.
//
// If it returns maxTagValueSuffixes suffixes, then it is likely more than maxTagValueSuffixes suffixes is found.
func (db *indexDB) SearchTagValueSuffixes(qt *querytracer.Tracer, tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int, deadline uint64) (map[string]struct{}, error) {
	qt = qt.NewChild("search tag value suffixes for timeRange=%s, tagKey=%q, tagValuePrefix=%q, delimiter=%c, maxTagValueSuffixes=%d",
		&tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	defer qt.Done()

	// TODO: cache results?

	is := db.getIndexSearch(deadline)
	tvss, err := is.searchTagValueSuffixesForTimeRange(tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	// Do not skip empty suffixes, since they may represent leaf tag values.

	qt.Printf("found %d suffixes", len(tvss))
	return tvss, nil
}

func (is *indexSearch) searchTagValueSuffixesForTimeRange(tr TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) (map[string]struct{}, error) {
	if tr == globalIndexTimeRange {
		return is.searchTagValueSuffixesAll(tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
	}

	minDate, maxDate := tr.DateRange()
	// Query over multiple days in parallel.
	wg := getWaitGroup()
	var errGlobal error
	var mu sync.Mutex // protects tvss + errGlobal from concurrent access below.
	tvss := make(map[string]struct{})
	for minDate <= maxDate {
		wg.Add(1)
		go func(date uint64) {
			defer wg.Done()
			isLocal := is.db.getIndexSearch(is.deadline)
			tvssLocal, err := isLocal.searchTagValueSuffixesForDate(date, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes)
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
	return tvss, errGlobal
}

func (is *indexSearch) searchTagValueSuffixesAll(tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) (map[string]struct{}, error) {
	kb := &is.kb
	nsPrefix := byte(nsPrefixTagToMetricIDs)
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefix)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagKey))
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagValuePrefix))
	kb.B = kb.B[:len(kb.B)-1] // remove tagSeparatorChar from the end of kb.B
	prefix := append([]byte(nil), kb.B...)
	return is.searchTagValueSuffixesForPrefix(nsPrefix, prefix, len(tagValuePrefix), delimiter, maxTagValueSuffixes)
}

func (is *indexSearch) searchTagValueSuffixesForDate(date uint64, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int) (map[string]struct{}, error) {
	nsPrefix := byte(nsPrefixDateTagToMetricIDs)
	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefix)
	kb.B = encoding.MarshalUint64(kb.B, date)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagKey))
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagValuePrefix))
	kb.B = kb.B[:len(kb.B)-1] // remove tagSeparatorChar from the end of kb.B
	prefix := append([]byte(nil), kb.B...)
	return is.searchTagValueSuffixesForPrefix(nsPrefix, prefix, len(tagValuePrefix), delimiter, maxTagValueSuffixes)
}

func (is *indexSearch) searchTagValueSuffixesForPrefix(nsPrefix byte, prefix []byte, tagValuePrefixLen int, delimiter byte, maxTagValueSuffixes int) (map[string]struct{}, error) {
	kb := &is.kb
	ts := &is.ts
	mp := &is.mp
	dmis := is.db.getDeletedMetricIDs()
	loopsPaceLimiter := 0
	ts.Seek(prefix)
	tvss := make(map[string]struct{})
	for len(tvss) < maxTagValueSuffixes && ts.NextItem() {
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
		if err := mp.Init(item, nsPrefix); err != nil {
			return nil, err
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
		return nil, fmt.Errorf("error when searching for tag value suffixes for prefix %q: %w", prefix, err)
	}
	return tvss, nil
}

func (db *indexDB) SearchGraphitePaths(qt *querytracer.Tracer, tr TimeRange, qHead, qTail []byte, maxPaths int, deadline uint64) (map[string]struct{}, error) {
	qt = qt.NewChild("search for graphite paths: timeRange=%s, qHead=%q, qTail=%q, maxPaths=%d", &tr, bytesutil.ToUnsafeString(qHead), bytesutil.ToUnsafeString(qTail), maxPaths)
	defer qt.Done()

	n := bytes.IndexAny(qTail, "*[{")
	if n < 0 {
		// Verify that qHead matches a metric name.
		qHead = append(qHead, qTail...)
		suffixes, err := db.SearchTagValueSuffixes(qt, tr, "", bytesutil.ToUnsafeString(qHead), '.', 1, deadline)
		if err != nil {
			return nil, err
		}
		if len(suffixes) == 0 {
			// The query doesn't match anything.
			return nil, nil
		}
		// The map should contain just one element. The code below is an attempt
		// to implement suffixes[0] if it were a slice.
		for s := range suffixes {
			if len(s) > 0 {
				// The query matches a metric name with additional suffix.
				return nil, nil
			}
			break
		}
		return map[string]struct{}{string(qHead): {}}, nil
	}
	qHead = append(qHead, qTail[:n]...)
	suffixes, err := db.SearchTagValueSuffixes(qt, tr, "", bytesutil.ToUnsafeString(qHead), '.', maxPaths, deadline)
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
	paths := make(map[string]struct{})
	for suffix := range suffixes {
		if len(paths) > maxPaths {
			return nil, fmt.Errorf("more than maxPath=%d paths found", maxPaths)
		}
		if !re.MatchString(suffix) {
			continue
		}
		if mustMatchLeafs {
			qHead = append(qHead[:qHeadLen], suffix...)
			paths[string(qHead)] = struct{}{}
			continue
		}
		qHead = append(qHead[:qHeadLen], suffix...)
		ps, err := db.SearchGraphitePaths(qt, tr, qHead, qTail, maxPaths, deadline)
		if err != nil {
			return nil, err
		}
		for p := range ps {
			paths[p] = struct{}{}
		}
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

// GetSeriesCount returns the approximate number of unique timeseries in the db.
//
// It includes the deleted series.
func (db *indexDB) GetSeriesCount(deadline uint64) (uint64, error) {
	is := db.getIndexSearch(deadline)
	defer db.putIndexSearch(is)
	return is.getSeriesCount()
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
	qt = qt.NewChild("collect TSDB status: filters=%s, date=%d, focusLabel=%q, topN=%d, maxMetrics=%d", tfss, date, focusLabel, topN, maxMetrics)
	defer qt.Done()

	is := db.getIndexSearch(deadline)
	defer db.putIndexSearch(is)
	return is.getTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics)
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
	dmis := is.db.getDeletedMetricIDs()
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
	if date == globalIndexDate {
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
	SeriesQueryStatsByMetricName []MetricNamesStatsRecord
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

func (db *indexDB) DeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (*uint64set.Set, error) {
	qt = qt.NewChild("delete series: filters=%s, maxMetrics=%d", tfss, maxMetrics)
	defer qt.Done()

	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)

	// Unconditionally search global index since a given day in per-day
	// index may not contain the full set of metricIDs that correspond
	// to the tfss.
	metricIDs, err := is.searchMetricIDs(qt, tfss, globalIndexTimeRange, maxMetrics)
	if err != nil {
		return nil, err
	}

	db.saveDeletedMetricIDs(metricIDs)
	return metricIDs, nil
}

// saveDeletedMetricIDs persists the deleted metricIDs to the global index by
// creating a separate `nsPrefixDeletedMetricID` entry for each metricID.
//
// More specifically, the method does these three things:
// 1. Add deleted metric ids to deletedMetricIDs
// 2. Reset all caches that must be reset
// 3. Finally add `nsPrefixDeletedMetricID` entries to the index.
//
// The order is important to exclude the possibility of the inconsistent state
// when the deleted metricIDs remain available in the persistent caches (such as
// tsidCache) after unclean shutdown.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1347.
//
// There are caches (such as tsidCache) that have only one instance and shared
// by all indexDB instances. Resetting such caches by an indexDB causes them to
// be reset as many times as the number of indexDBs. Ideally, we want these
// caches to be reset only once, in Storage.DeleteSeries(). But that would
// violate the aforementioned order of actions. And implementing the correct
// order of actions in Storage.DeleteSeries() would result in much more complex
// logic. So in this particular case we choose code clarity over correctness,
// because nothing bad will happen if these caches are reset multiple times.
//
// For caches that are not saved to disk (such as dateMetricIDCache) there is no
// strict requirement when to reset them. Still resetting them the same way as
// persistent caches to have all reset logic in one place.
func (db *indexDB) saveDeletedMetricIDs(metricIDs *uint64set.Set) {
	if metricIDs.Len() == 0 {
		// Nothing to delete
		return
	}

	// atomically add deleted metricIDs to an inmemory map.
	db.updateDeletedMetricIDs(metricIDs)

	// Do not reset tsidCache (MetricName -> TSID), since a given TSID can be
	// deleted in one indexDB but still be used in another indexDB.

	// Do not reset Storage's metricIDCache (MetricID -> TSID) and
	// metricNameCache (MetricID -> MetricName) since they must be used only
	// after filtering out deleted metricIDs.

	// Do not reset Storage's currHourMetricIDs, prevHourMetricIDs, and
	// nextDayMetricIDs caches. These caches are used during data ingestion
	// to decide whether a metricID needs to be added to the per-day index and
	// index records must not be created for deleted metricIDs. But presence of
	// deleted metricID in these caches will not lead to an index record
	// creation. Also see dateMetricIDCache below.
	//
	// Additionally, currHourMetricIDs and nextDayMetricIDs have accompanying
	// smaller in-memory caches, pendingHourEntries and pendingNextDayMetricIDs.
	// Should currHourMetricIDs and/or nextDayMetricIDs need to be reset,
	// pendingHourEntries and/or pendingNextDayMetricIDs need to be reset first.

	// Not resetting Storage.metricsTracker and Storage.metadataStorage because
	// they use metric names instead of metricIDs. And one metric name can
	// correspond to one or more metricIDs.

	// Do not reset Storage.missingMetricIDs because the delete operation will
	// not necessarily delete all the metricIDs from this cache.

	// Reset TagFilters -> TSIDS cache, since it may contain deleted TSIDs.
	db.tagFiltersToMetricIDsCache.Reset()

	// Do not reset loopsPerDateTagFilterCache. It stores loop counts
	// required to search metricIDs, but it is used at the stage when it does
	// not matter whether a metricID is deleted or not.

	// Do not reset metricIDCache and dateMetricIDCache. These caches are used
	// during data ingestion to decide whether a metricID needs to be added to
	// the per-day index and index records must not be created for deleted
	// metricIDs. But presence of deleted metricID in these caches will not lead
	// to an index record creation.

	// Store the metricIDs as deleted.
	items := getIndexItems()
	metricIDs.ForEach(func(part []uint64) bool {
		for _, metricID := range part {
			items.B = append(items.B, nsPrefixDeletedMetricID)
			items.B = encoding.MarshalUint64(items.B, metricID)
			items.Next()
		}
		return true
	})

	db.tb.AddItems(items.Items)
	putIndexItems(items)
}

func (db *indexDB) getDeletedMetricIDs() *uint64set.Set {
	return db.deletedMetricIDs.Load()
}

func (db *indexDB) setDeletedMetricIDs(dmis *uint64set.Set) {
	db.deletedMetricIDs.Store(dmis)
}

func (db *indexDB) updateDeletedMetricIDs(metricIDs *uint64set.Set) {
	db.deletedMetricIDsUpdateLock.Lock()
	dmisOld := db.getDeletedMetricIDs()
	dmisNew := dmisOld.Clone()
	dmisNew.Union(metricIDs)
	db.setDeletedMetricIDs(dmisNew)
	db.deletedMetricIDsUpdateLock.Unlock()
}

func (db *indexDB) mustLoadDeletedMetricIDs() {
	is := db.getIndexSearch(noDeadline)
	dmis, err := is.loadDeletedMetricIDs()
	db.putIndexSearch(is)
	if err != nil {
		logger.Panicf("FATAL: cannot load deleted metricIDs for indexDB %q: %v", db.name, err)
		return
	}
	db.setDeletedMetricIDs(dmis)
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
func (db *indexDB) searchMetricIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) (*uint64set.Set, error) {
	qt = qt.NewChild("search for matching metricIDs: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()

	if len(tfss) == 0 {
		return nil, nil
	}

	tfKeyBuf := tagFiltersKeyBufPool.Get()
	defer tagFiltersKeyBufPool.Put(tfKeyBuf)

	tfKeyBuf.B = marshalTagFiltersKey(tfKeyBuf.B[:0], tfss, tr)
	metricIDs, ok := db.getMetricIDsFromTagFiltersCache(qt, tfKeyBuf.B)
	if ok {
		// Fast path - metricIDs found in the cache
		if metricIDs.Len() > maxMetrics {
			return nil, errTooManyTimeseries(maxMetrics)
		}
		return metricIDs, nil
	}

	// Slow path - search for metricIDs in the db
	is := db.getIndexSearch(deadline)
	metricIDs, err := is.searchMetricIDs(qt, tfss, tr, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return nil, fmt.Errorf("error when searching for metricIDs: %w", err)
	}

	// Store metricIDs in the cache.
	db.putMetricIDsToTagFiltersCache(qt, metricIDs, tfKeyBuf.B)

	return metricIDs, nil
}

// SearchTSIDs searches the TSIDs that correspond to filters within the given
// time range.
//
// The returned TSIDs are sorted.
//
// The method will fail if the number of found TSIDs exceeds maxMetrics or the
// search has not completed within the specified deadline.
func (db *indexDB) SearchTSIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]TSID, error) {
	qt = qt.NewChild("search TSIDs: filters=%s, timeRange=%s, maxMetrics=%d", tfss, &tr, maxMetrics)
	defer qt.Done()

	metricIDs, err := db.searchMetricIDs(qt, tfss, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if metricIDs.Len() == 0 {
		return nil, nil
	}

	tsids := make([]TSID, metricIDs.Len())
	metricIDsToDelete := &uint64set.Set{}
	i := 0
	paceLimiter := 0
	is := db.getIndexSearch(deadline)
	defer db.putIndexSearch(is)
	metricIDs.ForEach(func(metricIDs []uint64) bool {
		for _, metricID := range metricIDs {
			if paceLimiter&paceLimiterSlowIterationsMask == 0 {
				if err = checkSearchDeadlineAndPace(deadline); err != nil {
					return false
				}
			}
			paceLimiter++

			// Try obtaining TSIDs from MetricID->TSID cache. This is much faster
			// than scanning the mergeset if it contains a lot of metricIDs.
			tsid := &tsids[i]
			err = db.getFromMetricIDCache(tsid, metricID)
			if err == nil {
				// Fast path - the tsid for metricID is found in cache.
				i++
				continue
			}
			if err != io.EOF {
				return false
			}
			err = nil
			if !is.getTSIDByMetricID(tsid, metricID) {
				// Cannot find TSID for the given metricID.
				// This may be the case on incomplete indexDB
				// due to snapshot or due to un-flushed entries.
				// Mark the metricID as deleted, so it is created again when new sample
				// for the given time series is ingested next time.
				if db.s.wasMetricIDMissingBefore(metricID) {
					db.missingTSIDsForMetricID.Add(1)
					metricIDsToDelete.Add(metricID)
				}
				continue
			}
			db.putToMetricIDCache(metricID, tsid)
			i++
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error when searching for TSIDs by metricIDs: %w", err)
	}

	tsids = tsids[:i]
	qt.Printf("found %d TSIDs for %d metricIDs", len(tsids), metricIDs.Len())

	// Sort the found tsids, since they must be passed to TSID search
	// in the sorted order.
	sort.Slice(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) })
	qt.Printf("sort %d TSIDs", len(tsids))

	if metricIDsToDelete.Len() > 0 {
		db.saveDeletedMetricIDs(metricIDsToDelete)
	}
	return tsids, nil
}

// searchMetricName appends metric name for the given metricID to dst
// and returns the result.
func (db *indexDB) searchMetricName(dst []byte, metricID uint64, noCache bool) ([]byte, bool) {
	is := db.getIndexSearchInternal(noDeadline, noCache)
	defer db.putIndexSearch(is)
	return is.searchMetricName(dst, metricID)
}

func (db *indexDB) SearchMetricNames(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) ([]string, error) {
	qt = qt.NewChild("search metric names: filters=%s, timeRange=%s, maxMetrics=%d", tfss, &tr, maxMetrics)
	defer qt.Done()

	metricIDs, err := db.searchMetricIDs(qt, tfss, tr, maxMetrics, deadline)
	if err != nil {
		return nil, err
	}
	if metricIDs.Len() == 0 {
		return nil, nil
	}

	metricNames := make([]string, 0, metricIDs.Len())
	metricIDsToDelete := &uint64set.Set{}
	var metricName []byte
	var ok bool
	paceLimiter := 0
	is := db.getIndexSearch(deadline)
	defer db.putIndexSearch(is)
	metricIDs.ForEach(func(metricIDs []uint64) bool {
		for _, metricID := range metricIDs {
			if paceLimiter&paceLimiterSlowIterationsMask == 0 {
				if err = checkSearchDeadlineAndPace(deadline); err != nil {
					return false
				}
			}
			paceLimiter++

			metricName, ok = is.searchMetricNameWithCache(metricName[:0], metricID)
			if !ok {
				// Cannot find TSID for the given metricID.
				// This may be the case on incomplete indexDB
				// due to snapshot or due to un-flushed entries.
				// Mark the metricID as deleted, so it is created again when new sample
				// for the given time series is ingested next time.
				if db.s.wasMetricIDMissingBefore(metricID) {
					db.missingMetricNamesForMetricID.Add(1)
					metricIDsToDelete.Add(metricID)
				}
				continue
			}
			metricNames = append(metricNames, string(metricName))
		}
		return true
	})
	if err != nil {
		return nil, err
	}

	if metricIDsToDelete.Len() > 0 {
		db.saveDeletedMetricIDs(metricIDsToDelete)
	}

	qt.Printf("loaded %d metric names", len(metricNames))
	return metricNames, nil
}

var tagFiltersKeyBufPool bytesutil.ByteBufferPool

func (is *indexSearch) getTSIDByMetricName(dst *TSID, metricName []byte, date uint64) bool {
	dmis := is.db.getDeletedMetricIDs()

	ts := &is.ts
	kb := &is.kb

	if is.db.s.disablePerDayIndex {
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricNameToTSID)
	} else {
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateMetricNameToTSID)
		kb.B = encoding.MarshalUint64(kb.B, date)
	}
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
	metricName := is.db.s.getMetricNameFromCache(dst, metricID)
	if len(metricName) > len(dst) {
		return metricName, true
	}
	var ok bool
	dst, ok = is.searchMetricName(dst, metricID)
	if ok {
		// There is no need in verifying whether the given metricID is deleted,
		// since the filtering must be performed before calling this func.
		is.db.s.putMetricNameToCache(metricID, dst)
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

func isSingleMetricNameFilter(tfss []*TagFilters) bool {
	// We check if tfss contain only single filter which is __name__
	return len(tfss) == 1 && len(tfss[0].tfs) == 1 && getMetricNameFilter(tfss[0]) != nil
}

func (is *indexSearch) searchMetricIDsWithFiltersOnDate(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, maxMetrics int) (*uint64set.Set, error) {
	if len(tfss) == 0 {
		return nil, nil
	}

	var tr TimeRange
	if date == globalIndexDate {
		tr = globalIndexTimeRange
	} else {
		tr = TimeRange{
			MinTimestamp: int64(date) * msecPerDay,
			MaxTimestamp: int64(date+1)*msecPerDay - 1,
		}
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
func (is *indexSearch) searchMetricIDs(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int) (*uint64set.Set, error) {
	metricIDs, err := is.searchMetricIDsInternal(qt, tfss, tr, maxMetrics)
	if err != nil {
		return nil, err
	}
	if metricIDs.Len() == 0 {
		// Nothing found
		return nil, nil
	}

	// Filter out deleted metricIDs.
	dmis := is.db.getDeletedMetricIDs()
	metricIDs.Subtract(dmis)

	return metricIDs, nil
}

func errTooManyTimeseries(maxMetrics int) error {
	return fmt.Errorf("the number of matching timeseries exceeds %d; "+
		"either narrow down the search or increase -search.max* command-line flag values "+
		"(the most likely limit is -search.maxUniqueTimeseries); "+
		"see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#resource-usage-limits", maxMetrics)
}

func (is *indexSearch) searchMetricIDsInternal(qt *querytracer.Tracer, tfss []*TagFilters, tr TimeRange, maxMetrics int) (*uint64set.Set, error) {
	qt = qt.NewChild("search for metric ids: filters=%s, timeRange=%s, maxMetrics=%d", tfss, &tr, maxMetrics)
	defer qt.Done()

	metricIDs := &uint64set.Set{}

	if !is.legacyContainsTimeRange(tr) {
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

func (is *indexSearch) updateMetricIDsForTagFilters(qt *querytracer.Tracer, metricIDs *uint64set.Set, tfs *TagFilters, tr TimeRange, maxMetrics int) error {
	if tr != globalIndexTimeRange {
		// Fast path - search metricIDs by date range in the per-day inverted
		// index.
		qt.Printf("search metric ids in the per-day index")
		is.db.dateRangeSearchCalls.Add(1)
		minDate, maxDate := tr.DateRange()
		return is.updateMetricIDsForDateRange(qt, metricIDs, tfs, minDate, maxDate, maxMetrics)
	}

	// Slow path - search metricIDs in the global inverted index.
	qt.Printf("search metric ids in the global index")
	is.db.globalSearchCalls.Add(1)
	m, err := is.getMetricIDsForDateAndFilters(qt, globalIndexDate, tfs, maxMetrics)
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

func (db *indexDB) createPerDayIndexes(date uint64, tsid *TSID, mn *MetricName) {
	if db.noRegisterNewSeries.Load() {
		logger.Panicf("BUG: registration of new series is disabled for indexDB %q", db.name)
	}

	if db.s.disablePerDayIndex {
		return
	}

	db.dateMetricIDCache.Set(date, tsid.MetricID)

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

	db.tb.AddItems(ii.Items)
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

func (is *indexSearch) hasDateMetricID(date, metricID uint64) bool {
	if is.db.dateMetricIDCache.Has(date, metricID) {
		return true
	}

	ok := is.hasDateMetricIDSlow(date, metricID)
	if ok {
		is.db.dateMetricIDCache.Set(date, metricID)
	}
	return ok
}

func (is *indexSearch) hasDateMetricIDSlow(date, metricID uint64) bool {
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

func (is *indexSearch) hasMetricID(metricID uint64) bool {
	if is.db.metricIDCache.Has(metricID) {
		return true
	}

	ok := is.hasMetricIDSlow(metricID)
	if ok {
		is.db.metricIDCache.Set(metricID)
	}
	return ok
}

func (is *indexSearch) hasMetricIDSlow(metricID uint64) bool {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return false
		}
		logger.Panicf("FATAL: error when searching for metricID=%d; searchPrefix %q: %s", metricID, kb.B, err)
	}
	return true
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
	e := is.db.loopsPerDateTagFilterCache.GetEntry(bytesutil.ToUnsafeString(is.kb.B))
	if e == nil {
		return 0, 0, 0
	}
	v := e.(*tagFiltersLoops)
	return v.loopsCount, v.filterLoopsCount, v.timestamp
}

type tagFiltersLoops struct {
	loopsCount       int64
	filterLoopsCount int64
	timestamp        uint64
}

func (v *tagFiltersLoops) SizeBytes() uint64 {
	return uint64(unsafe.Sizeof(*v))
}

func (is *indexSearch) storeLoopsCountForDateFilter(date uint64, tf *tagFilter, loopsCount, filterLoopsCount int64) {
	v := tagFiltersLoops{
		loopsCount:       loopsCount,
		filterLoopsCount: filterLoopsCount,
		timestamp:        fasttime.UnixTimestamp(),
	}
	is.kb.B = appendDateTagFilterCacheKey(is.kb.B[:0], is.db.name, date, tf)
	is.db.loopsPerDateTagFilterCache.PutEntry(string(is.kb.B), &v)
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
var nextUniqueMetricID = func() *atomicutil.Uint64 {
	var n atomicutil.Uint64
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
	if date == globalIndexDate {
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
// b cannot be reused until Reset call.
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
// b cannot be reused until Reset call.
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
	indexBlocksWithMetricIDsIncorrectOrder atomicutil.Uint64
	indexBlocksWithMetricIDsProcessed      atomicutil.Uint64
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
