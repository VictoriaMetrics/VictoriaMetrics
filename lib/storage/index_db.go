package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/fastcache"
	xxhash "github.com/cespare/xxhash/v2"
)

const (
	// Prefix for MetricName->TSID entries.
	nsPrefixMetricNameToTSID = 0

	// Prefix for Tag->MetricID entries.
	nsPrefixTagToMetricID = 1

	// Prefix for MetricID->TSID entries.
	nsPrefixMetricIDToTSID = 2

	// Prefix for MetricID->MetricName entries.
	nsPrefixMetricIDToMetricName = 3

	// Prefix for deleted MetricID entries.
	nsPrefixDeteletedMetricID = 4

	// Prefix for Date->MetricID entries.
	nsPrefixDateToMetricID = 5
)

// indexDB represents an index db.
type indexDB struct {
	name     string
	refCount uint64
	tb       *mergeset.Table

	extDB     *indexDB
	extDBLock sync.Mutex

	// Cache for fast TagFilters -> TSIDs lookup.
	tagCache *fastcache.Cache

	// Cache for fast MetricID -> TSID lookup.
	metricIDCache *fastcache.Cache

	// Cache for fast MetricID -> MetricName lookup.
	metricNameCache *fastcache.Cache

	// Cache holding useless TagFilters entries, which have no tag filters
	// matching low number of metrics.
	uselessTagFiltersCache *fastcache.Cache

	indexSearchPool sync.Pool

	// An inmemory map[uint64]struct{} of deleted metricIDs.
	//
	// The map holds deleted metricIDs for the current db and for the extDB.
	//
	// It is safe to keep the map in memory even for big number of deleted
	// metricIDs, since it occupies only 8 bytes per deleted metricID.
	deletedMetricIDs           atomic.Value
	deletedMetricIDsUpdateLock sync.Mutex

	// Global lists of metric ids for the current and the previous hours.
	// They are used for fast lookups on small time ranges covering
	// up to two last hours.
	currHourMetricIDs *atomic.Value
	prevHourMetricIDs *atomic.Value

	// The number of missing MetricID -> TSID entries.
	// High rate for this value means corrupted indexDB.
	missingTSIDsForMetricID uint64

	// The number of calls to search for metric ids for recent hours.
	recentHourMetricIDsSearchCalls uint64

	// The number of cache hits during search for metric ids in recent hours.
	recentHourMetricIDsSearchHits uint64

	// The number of searches for metric ids by days.
	dateMetricIDsSearchCalls uint64

	// The number of successful searches for metric ids by days.
	dateMetricIDsSearchHits uint64

	mustDrop uint64
}

// openIndexDB opens index db from the given path with the given caches.
func openIndexDB(path string, metricIDCache, metricNameCache *fastcache.Cache, currHourMetricIDs, prevHourMetricIDs *atomic.Value) (*indexDB, error) {
	tb, err := mergeset.OpenTable(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open indexDB %q: %s", path, err)
	}

	name := filepath.Base(path)

	// Do not persist tagCache in files, since it is very volatile.
	mem := memory.Allowed()
	tagCache := fastcache.New(mem / 32)

	db := &indexDB{
		refCount: 1,
		tb:       tb,
		name:     name,

		tagCache:        tagCache,
		metricIDCache:   metricIDCache,
		metricNameCache: metricNameCache,

		uselessTagFiltersCache: fastcache.New(mem / 128),

		currHourMetricIDs: currHourMetricIDs,
		prevHourMetricIDs: prevHourMetricIDs,
	}

	is := db.getIndexSearch()
	dmis, err := is.loadDeletedMetricIDs()
	db.putIndexSearch(is)
	if err != nil {
		return nil, fmt.Errorf("cannot load deleted metricIDs: %s", err)
	}
	db.setDeletedMetricIDs(dmis)

	return db, nil
}

// IndexDBMetrics contains essential metrics for indexDB.
type IndexDBMetrics struct {
	TagCacheSize      uint64
	TagCacheBytesSize uint64
	TagCacheRequests  uint64
	TagCacheMisses    uint64

	UselessTagFiltersCacheSize      uint64
	UselessTagFiltersCacheBytesSize uint64
	UselessTagFiltersCacheRequests  uint64
	UselessTagFiltersCacheMisses    uint64

	DeletedMetricsCount uint64

	IndexDBRefCount uint64

	MissingTSIDsForMetricID uint64

	RecentHourMetricIDsSearchCalls uint64
	RecentHourMetricIDsSearchHits  uint64
	DateMetricIDsSearchCalls       uint64
	DateMetricIDsSearchHits        uint64

	mergeset.TableMetrics
}

func (db *indexDB) scheduleToDrop() {
	atomic.AddUint64(&db.mustDrop, 1)
}

// UpdateMetrics updates m with metrics from the db.
func (db *indexDB) UpdateMetrics(m *IndexDBMetrics) {
	var cs fastcache.Stats

	cs.Reset()
	db.tagCache.UpdateStats(&cs)
	m.TagCacheSize += cs.EntriesCount
	m.TagCacheBytesSize += cs.BytesSize
	m.TagCacheRequests += cs.GetBigCalls
	m.TagCacheMisses += cs.Misses

	cs.Reset()
	db.uselessTagFiltersCache.UpdateStats(&cs)
	m.UselessTagFiltersCacheSize += cs.EntriesCount
	m.UselessTagFiltersCacheBytesSize += cs.BytesSize
	m.UselessTagFiltersCacheRequests += cs.GetBigCalls
	m.UselessTagFiltersCacheMisses += cs.Misses

	m.DeletedMetricsCount += uint64(len(db.getDeletedMetricIDs()))

	m.IndexDBRefCount += atomic.LoadUint64(&db.refCount)
	m.MissingTSIDsForMetricID += atomic.LoadUint64(&db.missingTSIDsForMetricID)
	m.RecentHourMetricIDsSearchCalls += atomic.LoadUint64(&db.recentHourMetricIDsSearchCalls)
	m.RecentHourMetricIDsSearchHits += atomic.LoadUint64(&db.recentHourMetricIDsSearchHits)
	m.DateMetricIDsSearchCalls += atomic.LoadUint64(&db.dateMetricIDsSearchCalls)
	m.DateMetricIDsSearchHits += atomic.LoadUint64(&db.dateMetricIDsSearchHits)

	db.tb.UpdateMetrics(&m.TableMetrics)
	db.doExtDB(func(extDB *indexDB) {
		extDB.tb.UpdateMetrics(&m.TableMetrics)
		m.IndexDBRefCount += atomic.LoadUint64(&extDB.refCount)
	})
}

func (db *indexDB) doExtDB(f func(extDB *indexDB)) bool {
	db.extDBLock.Lock()
	extDB := db.extDB
	if extDB != nil {
		extDB.incRef()
	}
	db.extDBLock.Unlock()
	if extDB == nil {
		return false
	}
	f(extDB)
	extDB.decRef()
	return true
}

// SetExtDB sets external db to search.
//
// It decrements refCount for the previous extDB.
func (db *indexDB) SetExtDB(extDB *indexDB) {
	// Add deleted metricIDs from extDB to db.
	if extDB != nil {
		dmisExt := extDB.getDeletedMetricIDs()
		metricIDs := getSortedMetricIDs(dmisExt)
		db.updateDeletedMetricIDs(metricIDs)
	}

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
	atomic.AddUint64(&db.refCount, 1)
}

func (db *indexDB) decRef() {
	n := atomic.AddUint64(&db.refCount, ^uint64(0))
	if n < 0 {
		logger.Panicf("BUG: negative refCount: %d", n)
	}
	if n > 0 {
		return
	}

	tbPath := db.tb.Path()
	db.tb.MustClose()
	db.SetExtDB(nil)

	if atomic.LoadUint64(&db.mustDrop) == 0 {
		return
	}

	logger.Infof("dropping indexDB %q", tbPath)
	if err := os.RemoveAll(tbPath); err != nil {
		logger.Panicf("FATAL: cannot remove %q: %s", tbPath, err)
	}
	logger.Infof("indexDB %q has been dropped", tbPath)
}

func (db *indexDB) getFromTagCache(key []byte) ([]TSID, bool) {
	value := db.tagCache.GetBig(nil, key)
	if len(value) == 0 {
		return nil, false
	}
	tsids, err := unmarshalTSIDs(nil, value)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal tsids from tagCache: %s", err)
	}
	return tsids, true
}

func (db *indexDB) putToTagCache(tsids []TSID, key []byte) {
	value := marshalTSIDs(nil, tsids)
	db.tagCache.SetBig(key, value)
}

func (db *indexDB) getFromMetricIDCache(dst *TSID, metricID uint64) error {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	buf := (*[unsafe.Sizeof(*dst)]byte)(unsafe.Pointer(dst))
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	tmp := db.metricIDCache.Get(buf[:0], key[:])
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
	db.metricIDCache.Set(key[:], buf[:])
}

func (db *indexDB) getMetricNameFromCache(dst []byte, metricID uint64) []byte {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	return db.metricNameCache.Get(dst, key[:])
}

func (db *indexDB) putMetricNameToCache(metricID uint64, metricName []byte) {
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	db.metricNameCache.Set(key[:], metricName)
}

func marshalTagFiltersKeyVersioned(dst []byte, tfss []*TagFilters) []byte {
	prefix := atomic.LoadUint64(&tagFiltersKeyGen)
	dst = encoding.MarshalUint64(dst, prefix)
	for _, tfs := range tfss {
		dst = append(dst, 0) // separator between tfs groups.
		for i := range tfs.tfs {
			dst = tfs.tfs[i].Marshal(dst)
		}
	}
	return dst
}

func marshalTSIDs(dst []byte, tsids []TSID) []byte {
	dst = encoding.MarshalUint64(dst, uint64(len(tsids)))
	for i := range tsids {
		dst = tsids[i].Marshal(dst)
	}
	return dst
}

func unmarshalTSIDs(dst []TSID, src []byte) ([]TSID, error) {
	if len(src) < 8 {
		return dst, fmt.Errorf("cannot unmarshal the number of tsids from %d bytes; require at least %d bytes", len(src), 8)
	}
	n := encoding.UnmarshalUint64(src)
	src = src[8:]
	dstLen := len(dst)
	if nn := dstLen + int(n) - cap(dst); nn > 0 {
		dst = append(dst[:cap(dst)], make([]TSID, nn)...)
	}
	dst = dst[:dstLen+int(n)]
	for i := 0; i < int(n); i++ {
		tail, err := dst[dstLen+i].Unmarshal(src)
		if err != nil {
			return dst, fmt.Errorf("cannot unmarshal tsid #%d out of %d: %s", i, n, err)
		}
		src = tail
	}
	if len(src) > 0 {
		return dst, fmt.Errorf("non-zero tail left after unmarshaling %d tsids; len(tail)=%d", n, len(src))
	}
	return dst, nil
}

func (db *indexDB) invalidateTagCache() {
	// This function must be fast, since it is called each
	// time new timeseries is added.
	atomic.AddUint64(&tagFiltersKeyGen, 1)
}

var tagFiltersKeyGen uint64

// getTSIDByNameNoCreate fills the dst with TSID for the given metricName.
//
// It returns io.EOF if the given mn isn't found locally.
func (db *indexDB) getTSIDByNameNoCreate(dst *TSID, metricName []byte) error {
	is := db.getIndexSearch()
	err := is.getTSIDByMetricName(dst, metricName)
	db.putIndexSearch(is)
	if err == nil {
		return nil
	}
	if err != io.EOF {
		return fmt.Errorf("cannot search TSID by MetricName %q: %s", metricName, err)
	}

	// Do not search for the TSID in the external storage,
	// since this function is already called by another indexDB instance.

	// The TSID for the given mn wasn't found.
	return io.EOF
}

type indexSearch struct {
	db *indexDB
	ts mergeset.TableSearch
	kb bytesutil.ByteBuffer

	// tsidByNameMisses and tsidByNameSkips is used for a performance
	// hack in GetOrCreateTSIDByName. See the comment there.
	tsidByNameMisses int
	tsidByNameSkips  int
}

// GetOrCreateTSIDByName fills the dst with TSID for the given metricName.
func (is *indexSearch) GetOrCreateTSIDByName(dst *TSID, metricName []byte) error {
	// A hack: skip searching for the TSID after many serial misses.
	// This should improve insertion performance for big batches
	// of new time series.
	if is.tsidByNameMisses < 100 {
		err := is.getTSIDByMetricName(dst, metricName)
		if err == nil {
			is.tsidByNameMisses = 0
			return nil
		}
		if err != io.EOF {
			return fmt.Errorf("cannot search TSID by MetricName %q: %s", metricName, err)
		}
		is.tsidByNameMisses++
	} else {
		is.tsidByNameSkips++
		if is.tsidByNameSkips > 10000 {
			is.tsidByNameSkips = 0
			is.tsidByNameMisses = 0
		}
	}

	// TSID for the given name wasn't found. Create it.
	// It is OK if duplicate TSID for mn is created by concurrent goroutines.
	// Metric results will be merged by mn after TableSearch.
	if err := is.db.createTSIDByName(dst, metricName); err != nil {
		return fmt.Errorf("cannot create TSID by MetricName %q: %s", metricName, err)
	}
	return nil
}

func (db *indexDB) getIndexSearch() *indexSearch {
	v := db.indexSearchPool.Get()
	if v == nil {
		v = &indexSearch{
			db: db,
		}
	}
	is := v.(*indexSearch)
	is.ts.Init(db.tb)
	return is
}

func (db *indexDB) putIndexSearch(is *indexSearch) {
	is.ts.MustClose()
	is.kb.Reset()

	// Do not reset tsidByNameMisses and tsidByNameSkips,
	// since they are used in GetOrCreateTSIDByName across call boundaries.

	db.indexSearchPool.Put(is)
}

func (db *indexDB) createTSIDByName(dst *TSID, metricName []byte) error {
	mn := GetMetricName()
	defer PutMetricName(mn)
	if err := mn.Unmarshal(metricName); err != nil {
		return fmt.Errorf("cannot unmarshal metricName %q: %s", metricName, err)
	}

	if err := db.generateTSID(dst, metricName, mn); err != nil {
		return fmt.Errorf("cannot generate TSID: %s", err)
	}
	if err := db.createIndexes(dst, mn); err != nil {
		return fmt.Errorf("cannot create indexes: %s", err)
	}

	// Invalidate tag cache, since it doesn't contain tags for the created mn -> TSID mapping.
	db.invalidateTagCache()

	return nil
}

func (db *indexDB) generateTSID(dst *TSID, metricName []byte, mn *MetricName) error {
	// Search the TSID in the external storage.
	// This is usually the db from the previous period.
	var err error
	if db.doExtDB(func(extDB *indexDB) {
		err = extDB.getTSIDByNameNoCreate(dst, metricName)
	}) {
		if err == nil {
			// The TSID has been found in the external storage.
			return nil
		}
		if err != io.EOF {
			return fmt.Errorf("external search failed: %s", err)
		}
	}

	// The TSID wan't found in the external storage.
	// Generate it locally.
	dst.MetricGroupID = xxhash.Sum64(mn.MetricGroup)
	if len(mn.Tags) > 0 {
		dst.JobID = uint32(xxhash.Sum64(mn.Tags[0].Value))
	}
	if len(mn.Tags) > 1 {
		dst.InstanceID = uint32(xxhash.Sum64(mn.Tags[1].Value))
	}
	dst.MetricID = getUniqueUint64()
	return nil
}

func (db *indexDB) createIndexes(tsid *TSID, mn *MetricName) error {
	// The order of index items is important.
	// It guarantees index consistency.

	items := getIndexItems()

	// Create MetricName -> TSID index.
	items.B = append(items.B, nsPrefixMetricNameToTSID)
	items.B = mn.Marshal(items.B)
	items.B = append(items.B, kvSeparatorChar)
	items.B = tsid.Marshal(items.B)
	items.Next()

	// Create MetricID -> MetricName index.
	items.B = marshalCommonPrefix(items.B, nsPrefixMetricIDToMetricName)
	items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
	items.B = mn.Marshal(items.B)
	items.Next()

	// Create MetricID -> TSID index.
	items.B = marshalCommonPrefix(items.B, nsPrefixMetricIDToTSID)
	items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
	items.B = tsid.Marshal(items.B)
	items.Next()

	commonPrefix := kbPool.Get()
	commonPrefix.B = marshalCommonPrefix(commonPrefix.B[:0], nsPrefixTagToMetricID)

	// Create MetricGroup -> MetricID index.
	items.B = append(items.B, commonPrefix.B...)
	items.B = marshalTagValue(items.B, nil)
	items.B = marshalTagValue(items.B, mn.MetricGroup)
	items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
	items.Next()

	// For each tag create tag -> MetricID index.
	for i := range mn.Tags {
		tag := &mn.Tags[i]
		items.B = append(items.B, commonPrefix.B...)
		items.B = tag.Marshal(items.B)
		items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
		items.Next()
	}

	kbPool.Put(commonPrefix)
	err := db.tb.AddItems(items.Items)
	putIndexItems(items)
	return err
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

// SearchTagKeys returns all the tag keys.
func (db *indexDB) SearchTagKeys(maxTagKeys int) ([]string, error) {
	// TODO: cache results?

	tks := make(map[string]struct{})

	is := db.getIndexSearch()
	err := is.searchTagKeys(tks, maxTagKeys)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		err = is.searchTagKeys(tks, maxTagKeys)
		extDB.putIndexSearch(is)
	})
	if ok && err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(tks))
	for key := range tks {
		keys = append(keys, key)
	}

	// Do not sort keys, since they must be sorted by vmselect.
	return keys, nil
}

func (is *indexSearch) searchTagKeys(tks map[string]struct{}, maxTagKeys int) error {
	ts := &is.ts
	kb := &is.kb
	dmis := is.db.getDeletedMetricIDs()
	commonPrefix := marshalCommonPrefix(nil, nsPrefixTagToMetricID)
	ts.Seek(commonPrefix)
	for len(tks) < maxTagKeys && ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, commonPrefix) {
			break
		}
		tail := item[len(commonPrefix):]

		// Unmarshal tag key into kb.B
		var err error
		tail, kb.B, err = unmarshalTagValue(kb.B[:0], tail)
		if err != nil {
			return fmt.Errorf("cannot unmarshal tagKey from %X: %s", item, err)
		}

		// Verify that the tag key points to existing metric.
		if len(tail) < 8 {
			return fmt.Errorf("cannot unmarshal metricID from less than 8 bytes; got %d bytes; item=%X", len(tail), tail)
		}
		metricID := encoding.UnmarshalUint64(tail[len(tail)-8:])
		if _, deleted := dmis[metricID]; deleted {
			// The given metric is deleted. Skip it.
			continue
		}

		// Store tag key.
		tks[string(kb.B)] = struct{}{}

		// Search for the next tag key.
		// tkp (tag key prefix) contains (commonPrefix + encoded tag key).
		// The last char must be tagSeparatorChar. Just increment it
		// in order to jump to the next tag key.
		tkp := item[:len(item)-len(tail)]
		if len(tkp) == 0 || tkp[len(tkp)-1] != tagSeparatorChar || tagSeparatorChar >= 0xff {
			logger.Panicf("BUG: the last char in tkp=%X must be %X. Check unmarshalTagValue code", tkp, tagSeparatorChar)
		}
		kb.B = append(kb.B[:0], tkp...)
		kb.B[len(kb.B)-1]++
		ts.Seek(kb.B)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error during search for commonPrefix %q: %s", commonPrefix, err)
	}
	return nil
}

// SearchTagValues returns all the tag values for the given tagKey
func (db *indexDB) SearchTagValues(tagKey []byte, maxTagValues int) ([]string, error) {
	// TODO: cache results?

	kb := kbPool.Get()
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricID)
	kb.B = marshalTagValue(kb.B, tagKey)

	tvs := make(map[string]struct{})
	is := db.getIndexSearch()
	err := is.searchTagValues(tvs, kb.B, maxTagValues)
	db.putIndexSearch(is)
	if err != nil {
		kbPool.Put(kb)
		return nil, err
	}
	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		err = is.searchTagValues(tvs, kb.B, maxTagValues)
		extDB.putIndexSearch(is)
	})
	kbPool.Put(kb)
	if ok && err != nil {
		return nil, err
	}

	tagValues := make([]string, 0, len(tvs))
	for tv := range tvs {
		tagValues = append(tagValues, tv)
	}

	// Do not sort tagValues, since they must be sorted by vmselect.
	return tagValues, nil
}

func (is *indexSearch) searchTagValues(tvs map[string]struct{}, prefix []byte, maxTagValues int) error {
	ts := &is.ts
	kb := &is.kb
	dmis := is.db.getDeletedMetricIDs()
	ts.Seek(prefix)
	for len(tvs) < maxTagValues && ts.NextItem() {
		k := ts.Item
		if !bytes.HasPrefix(k, prefix) {
			break
		}

		// Get TagValue
		k = k[len(prefix):]
		var err error
		k, kb.B, err = unmarshalTagValue(kb.B[:0], k)
		if err != nil {
			return fmt.Errorf("cannot unmarshal tagValue: %s", err)
		}
		if len(k) != 8 {
			return fmt.Errorf("unexpected suffix after tag value; want %d bytes; got %d bytes", 8, len(k))
		}

		// Verify whether the corresponding metric is deleted.
		if len(dmis) > 0 {
			metricID := encoding.UnmarshalUint64(k)
			if _, deleted := dmis[metricID]; deleted {
				// The metric is deleted.
				continue
			}
		}

		// Store tag value
		tvs[string(kb.B)] = struct{}{}

		// Search for the next tag value.
		// tkp (tag key prefix) contains (commonPrefix + encoded tag value).
		// The last char must be tagSeparatorChar. Just increment it
		// in order to jump to the next tag key.
		tkp := ts.Item[:len(ts.Item)-8]
		if len(tkp) == 0 || tkp[len(tkp)-1] != tagSeparatorChar || tagSeparatorChar >= 0xff {
			logger.Panicf("BUG: the last char in tkp=%X must be %X. Check unmarshalTagValue code", tkp, tagSeparatorChar)
		}
		kb.B = append(kb.B[:0], tkp...)
		kb.B[len(kb.B)-1]++
		ts.Seek(kb.B)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag name prefix %q: %s", prefix, err)
	}
	return nil
}

// GetSeriesCount returns the approximate number of unique timeseries in the db.
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (db *indexDB) GetSeriesCount() (uint64, error) {
	is := db.getIndexSearch()
	n, err := is.getSeriesCount()
	db.putIndexSearch(is)
	if err != nil {
		return 0, err
	}

	var nExt uint64
	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		nExt, err = is.getSeriesCount()
		extDB.putIndexSearch(is)
	})
	if ok && err != nil {
		return 0, err
	}
	return n + nExt, nil
}

// searchMetricName appends metric name for the given metricID to dst
// and returns the result.
func (db *indexDB) searchMetricName(dst []byte, metricID uint64) ([]byte, error) {
	is := db.getIndexSearch()
	dst, err := is.searchMetricName(dst, metricID)
	db.putIndexSearch(is)

	if err != io.EOF {
		return dst, err
	}

	// Try searching in the external indexDB.
	if db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		dst, err = is.searchMetricName(dst, metricID)
		extDB.putIndexSearch(is)
	}) {
		return dst, err
	}

	// Cannot find MetricName for the given metricID. This may be the case
	// when indexDB contains incomplete set of metricID -> metricName entries
	// after a snapshot or due to unflushed entries.
	return dst, io.EOF
}

// DeleteTSIDs marks as deleted all the TSIDs matching the given tfss.
//
// The caller must reset all the caches which may contain the deleted TSIDs.
//
// Returns the number of metrics deleted.
func (db *indexDB) DeleteTSIDs(tfss []*TagFilters) (int, error) {
	if len(tfss) == 0 {
		return 0, nil
	}

	// Obtain metricIDs to delete.
	is := db.getIndexSearch()
	metricIDs, err := is.searchMetricIDs(tfss, TimeRange{}, 1e9)
	db.putIndexSearch(is)
	if err != nil {
		return 0, err
	}
	if len(metricIDs) == 0 {
		// Nothing to delete
		return 0, nil
	}

	// Mark the found metricIDs as deleted.
	items := getIndexItems()
	for _, metricID := range metricIDs {
		items.B = append(items.B, nsPrefixDeteletedMetricID)
		items.B = encoding.MarshalUint64(items.B, metricID)
		items.Next()
	}
	err = db.tb.AddItems(items.Items)
	putIndexItems(items)
	if err != nil {
		return 0, err
	}
	deletedCount := len(metricIDs)

	// atomically add deleted metricIDs to an inmemory map.
	db.updateDeletedMetricIDs(metricIDs)

	// Reset TagFilters -> TSIDS cache, since it may contain deleted TSIDs.
	db.invalidateTagCache()

	// Delete TSIDs in the extDB.
	if db.doExtDB(func(extDB *indexDB) {
		var n int
		n, err = extDB.DeleteTSIDs(tfss)
		deletedCount += n
	}) {
		if err != nil {
			return deletedCount, fmt.Errorf("cannot delete tsids in extDB: %s", err)
		}
	}
	return deletedCount, nil
}

func (db *indexDB) getDeletedMetricIDs() map[uint64]struct{} {
	return db.deletedMetricIDs.Load().(map[uint64]struct{})
}

func (db *indexDB) setDeletedMetricIDs(dmis map[uint64]struct{}) {
	db.deletedMetricIDs.Store(dmis)
}

func (db *indexDB) updateDeletedMetricIDs(metricIDs []uint64) {
	db.deletedMetricIDsUpdateLock.Lock()
	dmisOld := db.getDeletedMetricIDs()
	dmisNew := make(map[uint64]struct{}, len(dmisOld)+len(metricIDs))
	for metricID := range dmisOld {
		dmisNew[metricID] = struct{}{}
	}
	for _, metricID := range metricIDs {
		dmisNew[metricID] = struct{}{}
	}
	db.setDeletedMetricIDs(dmisNew)
	db.deletedMetricIDsUpdateLock.Unlock()
}

func (is *indexSearch) loadDeletedMetricIDs() (map[uint64]struct{}, error) {
	dmis := make(map[uint64]struct{})
	ts := &is.ts
	kb := &is.kb
	kb.B = append(kb.B[:0], nsPrefixDeteletedMetricID)
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
		dmis[metricID] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return nil, err
	}
	return dmis, nil
}

// searchTSIDs returns tsids matching the given tfss over the given tr.
func (db *indexDB) searchTSIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]TSID, error) {
	if len(tfss) == 0 {
		return nil, nil
	}

	tfKeyBuf := tagFiltersKeyBufPool.Get()
	defer tagFiltersKeyBufPool.Put(tfKeyBuf)

	tfKeyBuf.B = marshalTagFiltersKeyVersioned(tfKeyBuf.B[:0], tfss)
	tsids, ok := db.getFromTagCache(tfKeyBuf.B)
	if ok {
		// Fast path - tsids found in the cache.
		return tsids, nil
	}

	// Slow path - search for tsids in the db and extDB.
	is := db.getIndexSearch()
	localTSIDs, err := is.searchTSIDs(tfss, tr, maxMetrics)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	var extTSIDs []TSID
	if db.doExtDB(func(extDB *indexDB) {
		tsids, ok := extDB.getFromTagCache(tfKeyBuf.B)
		if ok {
			extTSIDs = tsids
			return
		}
		is := extDB.getIndexSearch()
		extTSIDs, err = is.searchTSIDs(tfss, tr, maxMetrics)
		extDB.putIndexSearch(is)

		// Do not store found tsids into extDB.tagCache,
		// since they will be stored into outer cache instead.
	}) {
		if err != nil {
			return nil, err
		}
	}

	// Merge localTSIDs with extTSIDs.
	tsids = mergeTSIDs(localTSIDs, extTSIDs)

	// Sort the found tsids, since they must be passed to TSID search
	// in the sorted order.
	sort.Slice(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) })

	// Store TSIDs in the cache.
	db.putToTagCache(tsids, tfKeyBuf.B)

	return tsids, err
}

var tagFiltersKeyBufPool bytesutil.ByteBufferPool

func (is *indexSearch) getTSIDByMetricName(dst *TSID, metricName []byte) error {
	dmis := is.db.getDeletedMetricIDs()
	ts := &is.ts
	kb := &is.kb
	kb.B = append(kb.B[:0], nsPrefixMetricNameToTSID)
	kb.B = append(kb.B, metricName...)
	kb.B = append(kb.B, kvSeparatorChar)
	ts.Seek(kb.B)
	for ts.NextItem() {
		if !bytes.HasPrefix(ts.Item, kb.B) {
			// Nothing found.
			return io.EOF
		}
		v := ts.Item[len(kb.B):]
		tail, err := dst.Unmarshal(v)
		if err != nil {
			return fmt.Errorf("cannot unmarshal TSID: %s", err)
		}
		if len(tail) > 0 {
			return fmt.Errorf("unexpected non-empty tail left after unmarshaling TSID: %X", tail)
		}
		if len(dmis) > 0 {
			// Verify whether the dst is marked as deleted.
			if _, deleted := dmis[dst.MetricID]; deleted {
				// The dst is deleted. Continue searching.
				continue
			}
		}
		// Found valid dst.
		return nil
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching TSID by metricName; searchPrefix %q: %s", kb.B, err)
	}
	// Nothing found
	return io.EOF
}

func (is *indexSearch) searchMetricName(dst []byte, metricID uint64) ([]byte, error) {
	metricName := is.db.getMetricNameFromCache(dst, metricID)
	if len(metricName) > len(dst) {
		return metricName, nil
	}

	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToMetricName)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return dst, err
		}
		return dst, fmt.Errorf("error when searching metricName by metricID; searchPrefix %q: %s", kb.B, err)
	}
	v := ts.Item[len(kb.B):]
	dst = append(dst, v...)

	// There is no need in verifying whether the given metricID is deleted,
	// since the filtering must be performed before calling this func.
	is.db.putMetricNameToCache(metricID, dst)
	return dst, nil
}

func mergeTSIDs(a, b []TSID) []TSID {
	if len(b) > len(a) {
		a, b = b, a
	}
	if len(b) == 0 {
		return a
	}
	m := make(map[uint64]TSID, len(a))
	for i := range a {
		tsid := &a[i]
		m[tsid.MetricID] = *tsid
	}
	for i := range b {
		tsid := &b[i]
		m[tsid.MetricID] = *tsid
	}

	tsids := make([]TSID, 0, len(m))
	for _, tsid := range m {
		tsids = append(tsids, tsid)
	}
	return tsids
}

func (is *indexSearch) searchTSIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]TSID, error) {
	metricIDs, err := is.searchMetricIDs(tfss, tr, maxMetrics)
	if err != nil {
		return nil, err
	}
	if len(metricIDs) == 0 {
		// Nothing found.
		return nil, nil
	}

	// Obtain TSID values for the given metricIDs.
	tsids := make([]TSID, len(metricIDs))
	i := 0
	for _, metricID := range metricIDs {
		// Try obtaining TSIDs from db.tsidCache. This is much faster
		// than scanning the mergeset if it contains a lot of metricIDs.
		tsid := &tsids[i]
		err := is.db.getFromMetricIDCache(tsid, metricID)
		if err == nil {
			// Fast path - the tsid for metricID is found in cache.
			i++
			continue
		}
		if err != io.EOF {
			return nil, err
		}
		if err := is.getTSIDByMetricID(tsid, metricID); err != nil {
			if err == io.EOF {
				// Cannot find TSID for the given metricID.
				// This may be the case on incomplete indexDB
				// due to snapshot or due to unflushed entries.
				// Just increment errors counter and skip it.
				atomic.AddUint64(&is.db.missingTSIDsForMetricID, 1)
				continue
			}
			return nil, fmt.Errorf("cannot find tsid %d out of %d for metricID %d: %s", i, len(metricIDs), metricID, err)
		}
		is.db.putToMetricIDCache(metricID, tsid)
		i++
	}
	tsids = tsids[:i]

	// Do not sort the found tsids, since they will be sorted later.
	return tsids, nil
}

func (is *indexSearch) getTSIDByMetricID(dst *TSID, metricID uint64) error {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("error when searching TSID by metricID; searchPrefix %q: %s", kb.B, err)
	}
	v := ts.Item[len(kb.B):]
	tail, err := dst.Unmarshal(v)
	if err != nil {
		return fmt.Errorf("cannot unmarshal TSID=%X: %s", v, err)
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected non-zero tail left after unmarshaling TSID: %X", tail)
	}
	return nil
}

func (is *indexSearch) getSeriesCount() (uint64, error) {
	ts := &is.ts
	kb := &is.kb
	var n uint64
	kb.B = append(kb.B[:0], nsPrefixMetricIDToTSID)
	ts.Seek(kb.B)
	for ts.NextItem() {
		if !bytes.HasPrefix(ts.Item, kb.B) {
			break
		}
		// Take into account deleted timeseries too.
		n++
	}
	if err := ts.Error(); err != nil {
		return 0, fmt.Errorf("error when counting unique timeseries: %s", err)
	}
	return n, nil
}

// updateMetricIDsByMetricNameMatch matches metricName values for the given srcMetricIDs against tfs
// and adds matching metrics to metricIDs.
func (is *indexSearch) updateMetricIDsByMetricNameMatch(metricIDs, srcMetricIDs map[uint64]struct{}, tfs []*tagFilter) error {
	// sort srcMetricIDs in order to speed up Seek below.
	sortedMetricIDs := make([]uint64, 0, len(srcMetricIDs))
	for metricID := range srcMetricIDs {
		sortedMetricIDs = append(sortedMetricIDs, metricID)
	}
	sort.Slice(sortedMetricIDs, func(i, j int) bool { return sortedMetricIDs[i] < sortedMetricIDs[j] })

	metricName := kbPool.Get()
	defer kbPool.Put(metricName)
	mn := GetMetricName()
	defer PutMetricName(mn)
	for _, metricID := range sortedMetricIDs {
		var err error
		metricName.B, err = is.searchMetricName(metricName.B[:0], metricID)
		if err != nil {
			return fmt.Errorf("cannot find metricName by metricID %d: %s", metricID, err)
		}
		if err := mn.Unmarshal(metricName.B); err != nil {
			return fmt.Errorf("cannot unmarshal metricName %q: %s", metricName.B, err)
		}

		// Match the mn against tfs.
		ok, err := matchTagFilters(mn, tfs, &is.kb)
		if err != nil {
			return fmt.Errorf("cannot match MetricName %s against tagFilters: %s", mn, err)
		}
		if !ok {
			continue
		}
		metricIDs[metricID] = struct{}{}
	}
	return nil
}

func (is *indexSearch) getTagFilterWithMinMetricIDsCountAdaptive(tfs *TagFilters, maxMetrics int) (*tagFilter, map[uint64]struct{}, error) {
	kb := &is.kb
	kb.B = tfs.marshal(kb.B[:0])
	kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
	if len(is.db.uselessTagFiltersCache.Get(nil, kb.B)) > 0 {
		// Skip useless work below, since the tfs doesn't contain tag filters matching less than maxMetrics metrics.
		return nil, nil, errTooManyMetrics
	}

	// Iteratively increase maxAllowedMetrics up to maxMetrics in order to limit
	// the time required for founding the tag filter with minimum matching metrics.
	maxAllowedMetrics := 16
	if maxAllowedMetrics > maxMetrics {
		maxAllowedMetrics = maxMetrics
	}
	for {
		minTf, minMetricIDs, err := is.getTagFilterWithMinMetricIDsCount(tfs, maxAllowedMetrics)
		if err != nil {
			return nil, nil, err
		}
		if len(minMetricIDs) < maxAllowedMetrics {
			// Found the tag filter with the minimum number of metrics.
			return minTf, minMetricIDs, nil
		}

		// Too many metrics matched.
		if maxAllowedMetrics >= maxMetrics {
			// The tag filter with minimum matching metrics matches at least maxMetrics metrics.
			kb.B = tfs.marshal(kb.B[:0])
			kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
			is.db.uselessTagFiltersCache.Set(kb.B, []byte("1"))
			return nil, nil, errTooManyMetrics
		}

		// Increase maxAllowedMetrics and try again.
		maxAllowedMetrics *= 4
		if maxAllowedMetrics > maxMetrics {
			maxAllowedMetrics = maxMetrics
		}
	}
}

var errTooManyMetrics = errors.New("all the tag filters match too many metrics")

func (is *indexSearch) getTagFilterWithMinMetricIDsCount(tfs *TagFilters, maxMetrics int) (*tagFilter, map[uint64]struct{}, error) {
	var minMetricIDs map[uint64]struct{}
	var minTf *tagFilter
	for i := range tfs.tfs {
		tf := &tfs.tfs[i]
		if tf.isNegative {
			// Skip negative filters.
			continue
		}
		metricIDs, err := is.getMetricIDsForTagFilter(tf, maxMetrics)
		if err != nil {
			if err == errFallbackToMetricNameMatch {
				// Skip tag filters requiring to scan for too many metrics.
				continue
			}
			return nil, nil, fmt.Errorf("cannot find MetricIDs for tagFilter %s: %s", tf, err)
		}
		if minTf == nil || len(metricIDs) < len(minMetricIDs) {
			minMetricIDs = metricIDs
			minTf = tf
			maxMetrics = len(minMetricIDs)
			if maxMetrics <= 1 {
				// There is no need in inspecting other filters, since minTf
				// already matches 0 or 1 metric.
				break
			}
		}
	}
	if minTf != nil {
		return minTf, minMetricIDs, nil
	}

	// There is no positive filter with small number of matching metrics.
	// Create it, so it matches all the MetricIDs for tfs.commonPrefix.
	metricIDs := make(map[uint64]struct{})
	if err := is.updateMetricIDsForCommonPrefix(metricIDs, tfs.commonPrefix, maxMetrics); err != nil {
		return nil, nil, err
	}
	return nil, metricIDs, nil
}

func matchTagFilters(mn *MetricName, tfs []*tagFilter, kb *bytesutil.ByteBuffer) (bool, error) {
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricID)

	for _, tf := range tfs {
		if len(tf.key) == 0 {
			// Match against mn.MetricGroup.
			b := marshalTagValue(kb.B, nil)
			b = marshalTagValue(b, mn.MetricGroup)
			kb.B = b[:len(kb.B)]
			ok, err := matchTagFilter(b, tf)
			if err != nil {
				return false, fmt.Errorf("cannot match MetricGroup %q with tagFilter %s: %s", mn.MetricGroup, tf, err)
			}
			if !ok {
				return false, nil
			}
			continue
		}

		// Search for matching tag name.
		tagMatched := false
		for j := range mn.Tags {
			tag := &mn.Tags[j]
			if string(tag.Key) != string(tf.key) {
				continue
			}

			// Found the matching tag name. Match for the value.
			b := tag.Marshal(kb.B)
			kb.B = b[:len(kb.B)]
			ok, err := matchTagFilter(b, tf)
			if err != nil {
				return false, fmt.Errorf("cannot match tag %q with tagFilter %s: %s", tag, tf, err)
			}
			if !ok {
				return false, nil
			}
			tagMatched = true
			break
		}
		if !tagMatched {
			// Matching tag name wasn't found.
			return false, nil
		}
	}
	return true, nil
}

func matchTagFilter(b []byte, tf *tagFilter) (bool, error) {
	if !bytes.HasPrefix(b, tf.prefix) {
		return tf.isNegative, nil
	}
	ok, err := tf.matchSuffix(b[len(tf.prefix):])
	if err != nil {
		return false, err
	}
	if !ok {
		return tf.isNegative, nil
	}
	return !tf.isNegative, nil
}

func (is *indexSearch) searchMetricIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]uint64, error) {
	metricIDs := make(map[uint64]struct{})
	for _, tfs := range tfss {
		if len(tfs.tfs) == 0 {
			// Return all the metric ids
			if err := is.updateMetricIDsForCommonPrefix(metricIDs, tfs.commonPrefix, maxMetrics+1); err != nil {
				return nil, err
			}
			if len(metricIDs) > maxMetrics {
				return nil, fmt.Errorf("the number or unique timeseries exceeds %d; either narrow down the search or increase -search.maxUniqueTimeseries", maxMetrics)
			}
			// Stop the iteration, since we cannot find more metric ids with the remaining tfss.
			break
		}
		if err := is.updateMetricIDsForTagFilters(metricIDs, tfs, tr, maxMetrics+1); err != nil {
			return nil, err
		}
		if len(metricIDs) > maxMetrics {
			return nil, fmt.Errorf("the number or matching unique timeseries exceeds %d; either narrow down the search or increase -search.maxUniqueTimeseries", maxMetrics)
		}
	}
	if len(metricIDs) == 0 {
		// Nothing found
		return nil, nil
	}

	sortedMetricIDs := getSortedMetricIDs(metricIDs)

	// Filter out deleted metricIDs.
	dmis := is.db.getDeletedMetricIDs()
	if len(dmis) > 0 {
		metricIDsFiltered := sortedMetricIDs[:0]
		for _, metricID := range sortedMetricIDs {
			if _, deleted := dmis[metricID]; !deleted {
				metricIDsFiltered = append(metricIDsFiltered, metricID)
			}
		}
		sortedMetricIDs = metricIDsFiltered
	}

	return sortedMetricIDs, nil
}

func (is *indexSearch) updateMetricIDsForTagFilters(metricIDs map[uint64]struct{}, tfs *TagFilters, tr TimeRange, maxMetrics int) error {
	// Sort tag filters for faster ts.Seek below.
	sort.Slice(tfs.tfs, func(i, j int) bool { return bytes.Compare(tfs.tfs[i].prefix, tfs.tfs[j].prefix) < 0 })

	minTf, minMetricIDs, err := is.getTagFilterWithMinMetricIDsCountAdaptive(tfs, maxMetrics)
	if err != nil {
		if err != errTooManyMetrics {
			return err
		}

		// All the tag filters match too many metrics.

		// Slow path: try filtering the matching metrics by time range.
		// This should work well for cases when old metrics are constantly substituted
		// by big number of new metrics. For example, prometheus-operator creates many new
		// metrics for each new deployment.
		//
		// Allow fetching up to 20*maxMetrics metrics for the given time range
		// in the hope these metricIDs will be filtered out by other filters below.
		maxTimeRangeMetrics := 20 * maxMetrics
		metricIDsForTimeRange, err := is.getMetricIDsForTimeRange(tr, maxTimeRangeMetrics+1)
		if err == errMissingMetricIDsForDate {
			return fmt.Errorf("cannot find tag filter matching less up to %d time series; either increase -search.maxUniqueTimeseries or use more specific tag filters",
				maxMetrics)
		}
		if err != nil {
			return err
		}
		if len(metricIDsForTimeRange) > maxTimeRangeMetrics {
			return fmt.Errorf("more than %d time series found on the time range %s; either increase -search.maxUniqueTimeseries or shrink the time range",
				maxTimeRangeMetrics, tr.String())
		}
		minMetricIDs = metricIDsForTimeRange
		minTf = nil
	}

	// Find intersection of minTf with other tfs.
	var tfsPostponed []*tagFilter
	for i := range tfs.tfs {
		tf := &tfs.tfs[i]
		if tf == minTf {
			continue
		}
		mIDs, err := is.intersectMetricIDsWithTagFilter(tf, minMetricIDs)
		if err == errFallbackToMetricNameMatch {
			// The tag filter requires too many index scans. Postpone it,
			// so tag filters with lower number of index scans may be applied.
			tfsPostponed = append(tfsPostponed, tf)
			continue
		}
		if err != nil {
			return err
		}
		minMetricIDs = mIDs
	}
	for i, tf := range tfsPostponed {
		mIDs, err := is.intersectMetricIDsWithTagFilter(tf, minMetricIDs)
		if err == errFallbackToMetricNameMatch {
			return is.updateMetricIDsByMetricNameMatch(metricIDs, minMetricIDs, tfsPostponed[i:])
		}
		if err != nil {
			return err
		}
		minMetricIDs = mIDs
	}
	for metricID := range minMetricIDs {
		metricIDs[metricID] = struct{}{}
	}
	return nil
}

func (is *indexSearch) getMetricIDsForTagFilter(tf *tagFilter, maxMetrics int) (map[uint64]struct{}, error) {
	if tf.isNegative {
		logger.Panicf("BUG: isNegative must be false")
	}
	metricIDs := make(map[uint64]struct{}, maxMetrics)
	if len(tf.orSuffixes) > 0 {
		// Fast path for orSuffixes - seek for rows for each value from orSuffxies.
		if err := is.updateMetricIDsForOrSuffixesNoFilter(tf, maxMetrics, metricIDs); err != nil {
			return nil, err
		}
		return metricIDs, nil
	}

	// Slow path - scan all the rows with tf.prefix
	maxLoops := maxMetrics * maxIndexScanLoopsPerMetric
	loops := 0
	ts := &is.ts
	ts.Seek(tf.prefix)
	for len(metricIDs) < maxMetrics && ts.NextItem() {
		loops++
		if loops > maxLoops {
			return nil, errFallbackToMetricNameMatch
		}

		k := ts.Item
		if !bytes.HasPrefix(k, tf.prefix) {
			break
		}

		// Get MetricID from k (the last 8 bytes).
		k = k[len(tf.prefix):]
		if len(k) < 8 {
			return nil, fmt.Errorf("invald key suffix size; want at least %d bytes; got %d bytes", 8, len(k))
		}
		v := k[len(k)-8:]
		k = k[:len(k)-8]

		ok, err := tf.matchSuffix(k)
		if err != nil {
			return nil, fmt.Errorf("error when matching %s: %s", tf, err)
		}
		if !ok {
			continue
		}
		metricID := encoding.UnmarshalUint64(v)
		metricIDs[metricID] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return nil, fmt.Errorf("error when searching for tag filter prefix %q: %s", tf.prefix, err)
	}
	return metricIDs, nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixesNoFilter(tf *tagFilter, maxMetrics int, metricIDs map[uint64]struct{}) error {
	if tf.isNegative {
		logger.Panicf("BUG: isNegative must be false")
	}
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	for _, orSuffix := range tf.orSuffixes {
		kb.B = append(kb.B[:0], tf.prefix...)
		kb.B = append(kb.B, orSuffix...)
		kb.B = append(kb.B, tagSeparatorChar)
		if err := is.updateMetricIDsForOrSuffixNoFilter(kb.B, maxMetrics, metricIDs); err != nil {
			return err
		}
		if len(metricIDs) >= maxMetrics {
			return nil
		}
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixesWithFilter(tf *tagFilter, metricIDs, filter map[uint64]struct{}) error {
	sortedFilter := getSortedMetricIDs(filter)
	kb := kbPool.Get()
	defer kbPool.Put(kb)
	for _, orSuffix := range tf.orSuffixes {
		kb.B = append(kb.B[:0], tf.prefix...)
		kb.B = append(kb.B, orSuffix...)
		kb.B = append(kb.B, tagSeparatorChar)
		if err := is.updateMetricIDsForOrSuffixWithFilter(kb.B, metricIDs, sortedFilter, tf.isNegative); err != nil {
			return err
		}
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixNoFilter(prefix []byte, maxMetrics int, metricIDs map[uint64]struct{}) error {
	ts := &is.ts
	maxLoops := maxMetrics * maxIndexScanLoopsPerMetric
	loops := 0
	ts.Seek(prefix)
	for len(metricIDs) < maxMetrics && ts.NextItem() {
		loops++
		if loops > maxLoops {
			return errFallbackToMetricNameMatch
		}
		if !bytes.HasPrefix(ts.Item, prefix) {
			break
		}
		// Get MetricID from ts.Item (the last 8 bytes).
		v := ts.Item[len(prefix):]
		if len(v) != 8 {
			return fmt.Errorf("invalid key suffix size for prefix=%q; want %d bytes; got %d bytes; value=%q", 8, prefix, len(v), v)
		}
		metricID := encoding.UnmarshalUint64(v)
		metricIDs[metricID] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag filter prefix %q: %s", prefix, err)
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixWithFilter(prefix []byte, metricIDs map[uint64]struct{}, sortedFilter []uint64, isNegative bool) error {
	ts := &is.ts
	kb := &is.kb
	for {
		// Seek for the next metricID from sortedFilter.
		if len(sortedFilter) == 0 {
			// All the sorteFilter entries have been searched.
			break
		}
		nextMetricID := sortedFilter[0]
		sortedFilter = sortedFilter[1:]
		kb.B = append(kb.B[:0], prefix...)
		kb.B = encoding.MarshalUint64(kb.B, nextMetricID)
		ts.Seek(kb.B)
		if !ts.NextItem() {
			break
		}
		if !bytes.HasPrefix(ts.Item, prefix) {
			break
		}
		// Get MetricID from ts.Item (the last 8 bytes).
		v := ts.Item[len(prefix):]
		if len(v) != 8 {
			return fmt.Errorf("invalid key suffix size for prefix=%q; want %d bytes; got %d bytes; value=%q", 8, prefix, len(v), v)
		}
		metricID := encoding.UnmarshalUint64(v)
		if metricID != nextMetricID {
			continue
		}
		if isNegative {
			delete(metricIDs, metricID)
			continue
		}
		metricIDs[metricID] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag filter prefix %q: %s", prefix, err)
	}
	return nil
}

var errFallbackToMetricNameMatch = errors.New("fall back to updateMetricIDsByMetricNameMatch because of too many index scan loops")

var errMissingMetricIDsForDate = errors.New("missing metricIDs for date")

func (is *indexSearch) getMetricIDsForTimeRange(tr TimeRange, maxMetrics int) (map[uint64]struct{}, error) {
	if tr.isZero() {
		return nil, errMissingMetricIDsForDate
	}
	atomic.AddUint64(&is.db.recentHourMetricIDsSearchCalls, 1)
	if metricIDs, ok := is.getMetricIDsForRecentHours(tr, maxMetrics); ok {
		// Fast path: tr covers the current and / or the previous hour.
		// Return the full list of metric ids for this time range.
		atomic.AddUint64(&is.db.recentHourMetricIDsSearchHits, 1)
		return metricIDs, nil
	}

	// Slow path: collect the metric ids for all the days covering the given tr.
	atomic.AddUint64(&is.db.dateMetricIDsSearchCalls, 1)
	minDate := tr.MinTimestamp / msecPerDay
	maxDate := tr.MaxTimestamp / msecPerDay
	if maxDate-minDate > 40 {
		// Too much dates must be covered. Give up.
		return nil, errMissingMetricIDsForDate
	}
	metricIDs := make(map[uint64]struct{}, maxMetrics)
	for minDate <= maxDate {
		if err := is.getMetricIDsForDate(uint64(minDate), metricIDs, maxMetrics); err != nil {
			return nil, err
		}
		minDate++
	}
	atomic.AddUint64(&is.db.dateMetricIDsSearchHits, 1)
	return metricIDs, nil
}

func (is *indexSearch) getMetricIDsForRecentHours(tr TimeRange, maxMetrics int) (map[uint64]struct{}, bool) {
	minHour := uint64(tr.MinTimestamp) / msecPerHour
	maxHour := uint64(tr.MaxTimestamp) / msecPerHour
	if is.db.currHourMetricIDs == nil {
		return nil, false
	}
	hmCurr := is.db.currHourMetricIDs.Load().(*hourMetricIDs)
	if maxHour == hmCurr.hour && minHour == maxHour && hmCurr.isFull {
		// The tr fits the current hour.
		// Return a copy of hmCurr.m, because the caller may modify
		// the returned map.
		if len(hmCurr.m) > maxMetrics {
			return nil, false
		}
		return getMetricIDsCopy(hmCurr.m), true
	}
	if is.db.prevHourMetricIDs == nil {
		return nil, false
	}
	hmPrev := is.db.prevHourMetricIDs.Load().(*hourMetricIDs)
	if maxHour == hmPrev.hour && minHour == maxHour && hmPrev.isFull {
		// The tr fits the previous hour.
		// Return a copy of hmPrev.m, because the caller may modify
		// the returned map.
		if len(hmPrev.m) > maxMetrics {
			return nil, false
		}
		return getMetricIDsCopy(hmPrev.m), true
	}
	if maxHour == hmCurr.hour && minHour == hmPrev.hour && hmCurr.isFull && hmPrev.isFull {
		// The tr spans the previous and the current hours.
		if len(hmCurr.m)+len(hmPrev.m) > maxMetrics {
			return nil, false
		}
		metricIDs := make(map[uint64]struct{}, len(hmCurr.m)+len(hmPrev.m))
		for metricID := range hmCurr.m {
			metricIDs[metricID] = struct{}{}
		}
		for metricID := range hmPrev.m {
			metricIDs[metricID] = struct{}{}
		}
		return metricIDs, true
	}
	return nil, false
}

func getMetricIDsCopy(src map[uint64]struct{}) map[uint64]struct{} {
	dst := make(map[uint64]struct{}, len(src))
	for metricID := range src {
		dst[metricID] = struct{}{}
	}
	return dst
}

func (db *indexDB) storeDateMetricID(date, metricID uint64) error {
	is := db.getIndexSearch()
	ok, err := is.hasDateMetricID(date, metricID)
	db.putIndexSearch(is)
	if err != nil {
		return err
	}
	if ok {
		// Fast path: the (date, metricID) entry already exists in the db.
		return nil
	}

	// Slow path: create (date, metricID) entry.
	items := getIndexItems()
	items.B = marshalCommonPrefix(items.B[:0], nsPrefixDateToMetricID)
	items.B = encoding.MarshalUint64(items.B, date)
	items.B = encoding.MarshalUint64(items.B, metricID)
	items.Next()
	err = db.tb.AddItems(items.Items)
	putIndexItems(items)
	return err
}

func (is *indexSearch) hasDateMetricID(date, metricID uint64) (bool, error) {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID)
	kb.B = encoding.MarshalUint64(kb.B, date)
	kb.B = encoding.MarshalUint64(kb.B, metricID)
	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, fmt.Errorf("error when searching for (date=%d, metricID=%d) entry: %s", date, metricID, err)
	}
	if string(ts.Item) != string(kb.B) {
		return false, fmt.Errorf("unexpected entry for (date=%d, metricID=%d); got %q; want %q", date, metricID, ts.Item, kb.B)
	}
	return true, nil
}

func (is *indexSearch) getMetricIDsForDate(date uint64, metricIDs map[uint64]struct{}, maxMetrics int) error {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID)
	kb.B = encoding.MarshalUint64(kb.B, date)
	ts.Seek(kb.B)
	items := 0
	for len(metricIDs) < maxMetrics && ts.NextItem() {
		if !bytes.HasPrefix(ts.Item, kb.B) {
			break
		}
		// Extract MetricID from ts.Item (the last 8 bytes).
		v := ts.Item[len(kb.B):]
		if len(v) != 8 {
			return fmt.Errorf("cannot extract metricID from k; want %d bytes; got %d bytes", 8, len(v))
		}
		metricID := encoding.UnmarshalUint64(v)
		metricIDs[metricID] = struct{}{}
		items++
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for metricIDs for date %d: %s", date, err)
	}
	if items == 0 {
		// There are no metricIDs for the given date.
		// This may be the case for old data when Date -> MetricID wasn't available.
		return errMissingMetricIDsForDate
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForCommonPrefix(metricIDs map[uint64]struct{}, commonPrefix []byte, maxMetrics int) error {
	ts := &is.ts
	ts.Seek(commonPrefix)
	for len(metricIDs) < maxMetrics && ts.NextItem() {
		k := ts.Item
		if !bytes.HasPrefix(k, commonPrefix) {
			break
		}

		// Extract MetricID from k (the last 8 bytes).
		k = k[len(commonPrefix):]
		if len(k) < 8 {
			return fmt.Errorf("cannot extract metricID from k; want at least %d bytes; got %d bytes", 8, len(k))
		}
		v := k[len(k)-8:]
		metricID := encoding.UnmarshalUint64(v)
		metricIDs[metricID] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for metricIDs by commonPrefix %q: %s", commonPrefix, err)
	}
	return nil
}

// The maximum number of index scan loops per already found metric.
// Bigger number of loops is slower than updateMetricIDsByMetricNameMatch
// over the found metrics.
const maxIndexScanLoopsPerMetric = 32

func (is *indexSearch) intersectMetricIDsWithTagFilter(tf *tagFilter, filter map[uint64]struct{}) (map[uint64]struct{}, error) {
	if len(filter) == 0 {
		return nil, nil
	}
	metricIDs := filter
	if !tf.isNegative {
		metricIDs = make(map[uint64]struct{}, len(filter))
	}
	if len(tf.orSuffixes) > 0 {
		// Fast path for orSuffixes - seek for rows for each value from orSuffixes.
		if err := is.updateMetricIDsForOrSuffixesWithFilter(tf, metricIDs, filter); err != nil {
			return nil, err
		}
		return metricIDs, nil
	}

	// Slow path - scan for all the rows with the given prefix.
	maxLoops := len(filter) * maxIndexScanLoopsPerMetric
	loops := 0
	ts := &is.ts
	ts.Seek(tf.prefix)
	for ts.NextItem() {
		loops++
		if loops > maxLoops {
			return nil, errFallbackToMetricNameMatch
		}

		k := ts.Item
		if !bytes.HasPrefix(k, tf.prefix) {
			break
		}

		// Extract MetricID from k (the last 8 bytes).
		k = k[len(tf.prefix):]
		if len(k) < 8 {
			return nil, fmt.Errorf("cannot extract metricID from k; want at least %d bytes; got %d bytes", 8, len(k))
		}
		v := k[len(k)-8:]
		k = k[:len(k)-8]

		ok, err := tf.matchSuffix(k)
		if err != nil {
			return nil, fmt.Errorf("error when matching %s: %s", tf, err)
		}
		if !ok {
			continue
		}
		metricID := encoding.UnmarshalUint64(v)
		if tf.isNegative {
			// filter must be equal to metricIDs
			delete(metricIDs, metricID)
			continue
		}
		if _, ok := filter[metricID]; ok {
			metricIDs[metricID] = struct{}{}
		}
	}
	if err := ts.Error(); err != nil {
		return nil, fmt.Errorf("error searching %q: %s", tf.prefix, err)
	}
	return metricIDs, nil
}

var kbPool bytesutil.ByteBufferPool

// Returns local unique MetricID.
func getUniqueUint64() uint64 {
	return atomic.AddUint64(&uniqueUint64, 1)
}

// This number mustn't go backwards on restarts, otherwise metricID
// collisions are possible. So don't change time on the server
// between VictoriaMetrics restarts.
var uniqueUint64 = uint64(time.Now().UnixNano())

func marshalCommonPrefix(dst []byte, nsPrefix byte) []byte {
	dst = append(dst, nsPrefix)
	return dst
}

func getSortedMetricIDs(m map[uint64]struct{}) []uint64 {
	a := make([]uint64, len(m))
	i := 0
	for metricID := range m {
		a[i] = metricID
		i++
	}
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	return a
}
