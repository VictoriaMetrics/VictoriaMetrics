package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
	xxhash "github.com/cespare/xxhash/v2"
)

const (
	// Prefix for MetricName->TSID entries.
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
)

func shouldCacheBlock(item []byte) bool {
	if len(item) == 0 {
		return true
	}
	// Do not cache items starting from
	switch item[0] {
	case nsPrefixTagToMetricIDs:
		// Do not cache blocks with tag->metricIDs items, since:
		// - these blocks are scanned sequentially, so the overhead
		//   on their unmarshaling is amortized by the sequential scan.
		// - these blocks can occupy high amounts of RAM in cache
		//   and evict other frequently accessed blocks.
		return false
	case nsPrefixDeletedMetricID:
		// Do not cache blocks with deleted metricIDs,
		// since these metricIDs are loaded only once during app start.
		return false
	default:
		return true
	}
}

// indexDB represents an index db.
type indexDB struct {
	// Atomic counters must go at the top of the structure in order to properly align by 8 bytes on 32-bit archs.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212 .

	refCount uint64

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

	name string
	tb   *mergeset.Table

	extDB     *indexDB
	extDBLock sync.Mutex

	// Cache for fast TagFilters -> TSIDs lookup.
	tagCache *workingsetcache.Cache

	// Cache for fast MetricID -> TSID lookup.
	metricIDCache *workingsetcache.Cache

	// Cache for fast MetricID -> MetricName lookup.
	metricNameCache *workingsetcache.Cache

	// Cache holding useless TagFilters entries, which have no tag filters
	// matching low number of metrics.
	uselessTagFiltersCache *workingsetcache.Cache

	indexSearchPool sync.Pool

	// An inmemory set of deleted metricIDs.
	//
	// The set holds deleted metricIDs for the current db and for the extDB.
	//
	// It is safe to keep the set in memory even for big number of deleted
	// metricIDs, since it usually requires 1 bit per deleted metricID.
	deletedMetricIDs           atomic.Value
	deletedMetricIDsUpdateLock sync.Mutex

	// Global lists of metric ids for the current and the previous hours.
	// They are used for fast lookups on small time ranges covering
	// up to two last hours.
	currHourMetricIDs *atomic.Value
	prevHourMetricIDs *atomic.Value
}

// openIndexDB opens index db from the given path with the given caches.
func openIndexDB(path string, metricIDCache, metricNameCache *workingsetcache.Cache, currHourMetricIDs, prevHourMetricIDs *atomic.Value) (*indexDB, error) {
	if metricIDCache == nil {
		logger.Panicf("BUG: metricIDCache must be non-nil")
	}
	if metricNameCache == nil {
		logger.Panicf("BUG: metricNameCache must be non-nil")
	}
	if currHourMetricIDs == nil {
		logger.Panicf("BUG: currHourMetricIDs must be non-nil")
	}
	if prevHourMetricIDs == nil {
		logger.Panicf("BUG: prevHourMetricIDs must be non-nil")
	}

	tb, err := mergeset.OpenTable(path, invalidateTagCache, mergeTagToMetricIDsRows)
	if err != nil {
		return nil, fmt.Errorf("cannot open indexDB %q: %s", path, err)
	}

	name := filepath.Base(path)

	// Do not persist tagCache in files, since it is very volatile.
	mem := memory.Allowed()

	db := &indexDB{
		refCount: 1,
		tb:       tb,
		name:     name,

		tagCache:               workingsetcache.New(mem/32, time.Hour),
		metricIDCache:          metricIDCache,
		metricNameCache:        metricNameCache,
		uselessTagFiltersCache: workingsetcache.New(mem/128, time.Hour),

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
	TagCacheSizeBytes uint64
	TagCacheRequests  uint64
	TagCacheMisses    uint64

	UselessTagFiltersCacheSize      uint64
	UselessTagFiltersCacheSizeBytes uint64
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
	m.TagCacheSizeBytes += cs.BytesSize
	m.TagCacheRequests += cs.GetBigCalls
	m.TagCacheMisses += cs.Misses

	cs.Reset()
	db.uselessTagFiltersCache.UpdateStats(&cs)
	m.UselessTagFiltersCacheSize += cs.EntriesCount
	m.UselessTagFiltersCacheSizeBytes += cs.BytesSize
	m.UselessTagFiltersCacheRequests += cs.GetBigCalls
	m.UselessTagFiltersCacheMisses += cs.Misses

	m.DeletedMetricsCount += uint64(db.getDeletedMetricIDs().Len())

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
		db.updateDeletedMetricIDs(dmisExt)
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
	if int64(n) < 0 {
		logger.Panicf("BUG: negative refCount: %d", n)
	}
	if n > 0 {
		return
	}

	tbPath := db.tb.Path()
	db.tb.MustClose()
	db.SetExtDB(nil)

	// Free space occupied by caches owned by db.
	db.tagCache.Stop()
	db.uselessTagFiltersCache.Stop()

	db.tagCache = nil
	db.metricIDCache = nil
	db.metricNameCache = nil
	db.uselessTagFiltersCache = nil

	if atomic.LoadUint64(&db.mustDrop) == 0 {
		return
	}

	logger.Infof("dropping indexDB %q", tbPath)
	fs.MustRemoveAll(tbPath)
	logger.Infof("indexDB %q has been dropped", tbPath)
}

func (db *indexDB) getFromTagCache(key []byte) ([]TSID, bool) {
	compressedBuf := tagBufPool.Get()
	defer tagBufPool.Put(compressedBuf)
	compressedBuf.B = db.tagCache.GetBig(compressedBuf.B[:0], key)
	if len(compressedBuf.B) == 0 {
		return nil, false
	}
	buf := tagBufPool.Get()
	defer tagBufPool.Put(buf)
	var err error
	buf.B, err = encoding.DecompressZSTD(buf.B[:0], compressedBuf.B)
	if err != nil {
		logger.Panicf("FATAL: cannot decompress tsids from tagCache: %s", err)
	}
	tsids, err := unmarshalTSIDs(nil, buf.B)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal tsids from tagCache: %s", err)
	}
	return tsids, true
}

var tagBufPool bytesutil.ByteBufferPool

func (db *indexDB) putToTagCache(tsids []TSID, key []byte) {
	buf := tagBufPool.Get()
	buf.B = marshalTSIDs(buf.B[:0], tsids)
	compressedBuf := tagBufPool.Get()
	compressedBuf.B = encoding.CompressZSTDLevel(compressedBuf.B[:0], buf.B, 1)
	tagBufPool.Put(buf)
	db.tagCache.SetBig(key, compressedBuf.B)
	tagBufPool.Put(compressedBuf)
}

func (db *indexDB) getFromMetricIDCache(dst *TSID, metricID uint64) error {
	// There is no need in prefixing the key with (accountID, projectID),
	// since metricID is globally unique across all (accountID, projectID) values.
	// See getUniqueUint64.

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
	// There is no need in prefixing the key with (accountID, projectID),
	// since metricID is globally unique across all (accountID, projectID) values.
	// See getUniqueUint64.

	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	return db.metricNameCache.Get(dst, key[:])
}

func (db *indexDB) putMetricNameToCache(metricID uint64, metricName []byte) {
	key := (*[unsafe.Sizeof(metricID)]byte)(unsafe.Pointer(&metricID))
	db.metricNameCache.Set(key[:], metricName)
}

func marshalTagFiltersKey(dst []byte, tfss []*TagFilters, versioned bool) []byte {
	prefix := ^uint64(0)
	if versioned {
		prefix = atomic.LoadUint64(&tagFiltersKeyGen)
	}
	dst = encoding.MarshalUint64(dst, prefix)
	if len(tfss) == 0 {
		return dst
	}
	dst = encoding.MarshalUint32(dst, tfss[0].accountID)
	dst = encoding.MarshalUint32(dst, tfss[0].projectID)
	for _, tfs := range tfss {
		dst = append(dst, 0) // separator between tfs groups.
		for i := range tfs.tfs {
			dst = tfs.tfs[i].MarshalNoAccountIDProjectID(dst)
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

func invalidateTagCache() {
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
	mp tagToMetricIDsRowParser

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
	is.ts.Init(db.tb, shouldCacheBlock)
	return is
}

func (db *indexDB) putIndexSearch(is *indexSearch) {
	is.ts.MustClose()
	is.kb.Reset()
	is.mp.Reset()

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

	// There is no need in invalidating tag cache, since it is invalidated
	// on db.tb flush via invalidateTagCache flushCallback passed to OpenTable.

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

	// The TSID wasn't found in the external storage.
	// Generate it locally.
	dst.AccountID = mn.AccountID
	dst.ProjectID = mn.ProjectID
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
	items.B = marshalCommonPrefix(items.B, nsPrefixMetricIDToMetricName, mn.AccountID, mn.ProjectID)
	items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
	items.B = mn.Marshal(items.B)
	items.Next()

	// Create MetricID -> TSID index.
	items.B = marshalCommonPrefix(items.B, nsPrefixMetricIDToTSID, mn.AccountID, mn.ProjectID)
	items.B = encoding.MarshalUint64(items.B, tsid.MetricID)
	items.B = tsid.Marshal(items.B)
	items.Next()

	commonPrefix := kbPool.Get()
	commonPrefix.B = marshalCommonPrefix(commonPrefix.B[:0], nsPrefixTagToMetricIDs, mn.AccountID, mn.ProjectID)

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

// SearchTagKeys returns all the tag keys for the given accountID, projectID.
func (db *indexDB) SearchTagKeys(accountID, projectID uint32, maxTagKeys int) ([]string, error) {
	// TODO: cache results?

	tks := make(map[string]struct{})

	is := db.getIndexSearch()
	err := is.searchTagKeys(accountID, projectID, tks, maxTagKeys)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}

	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		err = is.searchTagKeys(accountID, projectID, tks, maxTagKeys)
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

func (is *indexSearch) searchTagKeys(accountID, projectID uint32, tks map[string]struct{}, maxTagKeys int) error {
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	mp.Reset()
	dmis := is.db.getDeletedMetricIDs()
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs, accountID, projectID)
	prefix := kb.B
	ts.Seek(prefix)
	for len(tks) < maxTagKeys && ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item); err != nil {
			return err
		}
		if mp.IsDeletedTag(dmis) {
			continue
		}

		// Store tag key.
		tks[string(mp.Tag.Key)] = struct{}{}

		// Search for the next tag key.
		// The last char in kb.B must be tagSeparatorChar.
		// Just increment it in order to jump to the next tag key.
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs, accountID, projectID)
		kb.B = marshalTagValue(kb.B, mp.Tag.Key)
		kb.B[len(kb.B)-1]++
		ts.Seek(kb.B)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error during search for prefix %q: %s", prefix, err)
	}
	return nil
}

// SearchTagValues returns all the tag values for the given tagKey
func (db *indexDB) SearchTagValues(accountID, projectID uint32, tagKey []byte, maxTagValues int) ([]string, error) {
	// TODO: cache results?

	tvs := make(map[string]struct{})
	is := db.getIndexSearch()
	err := is.searchTagValues(accountID, projectID, tvs, tagKey, maxTagValues)
	db.putIndexSearch(is)
	if err != nil {
		return nil, err
	}
	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		err = is.searchTagValues(accountID, projectID, tvs, tagKey, maxTagValues)
		extDB.putIndexSearch(is)
	})
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

func (is *indexSearch) searchTagValues(accountID, projectID uint32, tvs map[string]struct{}, tagKey []byte, maxTagValues int) error {
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	mp.Reset()
	dmis := is.db.getDeletedMetricIDs()
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs, accountID, projectID)
	kb.B = marshalTagValue(kb.B, tagKey)
	prefix := kb.B
	ts.Seek(prefix)
	for len(tvs) < maxTagValues && ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		if err := mp.Init(item); err != nil {
			return err
		}
		if mp.IsDeletedTag(dmis) {
			continue
		}

		// Store tag value
		tvs[string(mp.Tag.Value)] = struct{}{}

		// Search for the next tag value.
		// The last char in kb.B must be tagSeparatorChar.
		// Just increment it in order to jump to the next tag key.
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs, accountID, projectID)
		kb.B = marshalTagValue(kb.B, mp.Tag.Key)
		kb.B = marshalTagValue(kb.B, mp.Tag.Value)
		kb.B[len(kb.B)-1]++
		ts.Seek(kb.B)
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag name prefix %q: %s", prefix, err)
	}
	return nil
}

// GetSeriesCount returns the approximate number of unique timeseries for the given (accountID, projectID).
//
// It includes the deleted series too and may count the same series
// up to two times - in db and extDB.
func (db *indexDB) GetSeriesCount(accountID, projectID uint32) (uint64, error) {
	is := db.getIndexSearch()
	n, err := is.getSeriesCount(accountID, projectID)
	db.putIndexSearch(is)
	if err != nil {
		return 0, err
	}

	var nExt uint64
	ok := db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		nExt, err = is.getSeriesCount(accountID, projectID)
		extDB.putIndexSearch(is)
	})
	if ok && err != nil {
		return 0, err
	}
	return n + nExt, nil
}

// searchMetricName appends metric name for the given metricID to dst
// and returns the result.
func (db *indexDB) searchMetricName(dst []byte, metricID uint64, accountID, projectID uint32) ([]byte, error) {
	is := db.getIndexSearch()
	dst, err := is.searchMetricName(dst, metricID, accountID, projectID)
	db.putIndexSearch(is)

	if err != io.EOF {
		return dst, err
	}

	// Try searching in the external indexDB.
	if db.doExtDB(func(extDB *indexDB) {
		is := extDB.getIndexSearch()
		dst, err = is.searchMetricName(dst, metricID, accountID, projectID)
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
		items.B = append(items.B, nsPrefixDeletedMetricID)
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
	dmis := &uint64set.Set{}
	for _, metricID := range metricIDs {
		dmis.Add(metricID)
	}
	db.updateDeletedMetricIDs(dmis)

	// Reset TagFilters -> TSIDS cache, since it may contain deleted TSIDs.
	invalidateTagCache()

	// Do not reset uselessTagFiltersCache, since the found metricIDs
	// on cache miss are filtered out later with deletedMetricIDs.

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

func (db *indexDB) getDeletedMetricIDs() *uint64set.Set {
	return db.deletedMetricIDs.Load().(*uint64set.Set)
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

// searchTSIDs returns sorted tsids matching the given tfss over the given tr.
func (db *indexDB) searchTSIDs(tfss []*TagFilters, tr TimeRange, maxMetrics int) ([]TSID, error) {
	if len(tfss) == 0 {
		return nil, nil
	}

	tfKeyBuf := tagFiltersKeyBufPool.Get()
	defer tagFiltersKeyBufPool.Put(tfKeyBuf)

	tfKeyBuf.B = marshalTagFiltersKey(tfKeyBuf.B[:0], tfss, true)
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
		tfKeyExtBuf := tagFiltersKeyBufPool.Get()
		defer tagFiltersKeyBufPool.Put(tfKeyExtBuf)

		// Data in extDB cannot be changed, so use unversioned keys for tag cache.
		tfKeyExtBuf.B = marshalTagFiltersKey(tfKeyExtBuf.B[:0], tfss, false)
		tsids, ok := extDB.getFromTagCache(tfKeyExtBuf.B)
		if ok {
			extTSIDs = tsids
			return
		}
		is := extDB.getIndexSearch()
		extTSIDs, err = is.searchTSIDs(tfss, tr, maxMetrics)
		extDB.putIndexSearch(is)

		sort.Slice(extTSIDs, func(i, j int) bool { return extTSIDs[i].Less(&extTSIDs[j]) })
		extDB.putToTagCache(extTSIDs, tfKeyExtBuf.B)
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
		if dmis.Len() > 0 {
			// Verify whether the dst is marked as deleted.
			if dmis.Has(dst.MetricID) {
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

func (is *indexSearch) searchMetricName(dst []byte, metricID uint64, accountID, projectID uint32) ([]byte, error) {
	metricName := is.db.getMetricNameFromCache(dst, metricID)
	if len(metricName) > len(dst) {
		return metricName, nil
	}

	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToMetricName, accountID, projectID)
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
	accountID := tfss[0].accountID
	projectID := tfss[0].projectID

	// Verify whether `is` contains data for the given tr.
	ok, err := is.containsTimeRange(tr, accountID, projectID)
	if err != nil {
		return nil, fmt.Errorf("error in containsTimeRange(%s): %s", &tr, err)
	}
	if !ok {
		// Fast path: nothing to search.
		return nil, nil
	}
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
		if err := is.getTSIDByMetricID(&tsids[i], metricID, accountID, projectID); err != nil {
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

func (is *indexSearch) getTSIDByMetricID(dst *TSID, metricID uint64, accountID, projectID uint32) error {
	// There is no need in checking for deleted metricIDs here, since they
	// must be checked by the caller.
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID, accountID, projectID)
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

func (is *indexSearch) getSeriesCount(accountID, projectID uint32) (uint64, error) {
	ts := &is.ts
	kb := &is.kb
	var n uint64
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID, accountID, projectID)
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
func (is *indexSearch) updateMetricIDsByMetricNameMatch(metricIDs, srcMetricIDs *uint64set.Set, tfs []*tagFilter, accountID, projectID uint32) error {
	// sort srcMetricIDs in order to speed up Seek below.
	sortedMetricIDs := srcMetricIDs.AppendTo(nil)

	metricName := kbPool.Get()
	defer kbPool.Put(metricName)
	mn := GetMetricName()
	defer PutMetricName(mn)
	for _, metricID := range sortedMetricIDs {
		var err error
		metricName.B, err = is.searchMetricName(metricName.B[:0], metricID, accountID, projectID)
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
		metricIDs.Add(metricID)
	}
	return nil
}

func (is *indexSearch) getTagFilterWithMinMetricIDsCountOptimized(tfs *TagFilters, tr TimeRange, maxMetrics int) (*tagFilter, *uint64set.Set, error) {
	// Try fast path with the minimized number of maxMetrics.
	maxMetricsAdjusted := is.adjustMaxMetricsAdaptive(tr, maxMetrics)
	minTf, minMetricIDs, err := is.getTagFilterWithMinMetricIDsCountAdaptive(tfs, maxMetricsAdjusted)
	if err == nil {
		return minTf, minMetricIDs, nil
	}
	if err != errTooManyMetrics {
		return nil, nil, err
	}

	// All the tag filters match too many metrics.

	// Slow path: try filtering the matching metrics by time range.
	// This should work well for cases when old metrics are constantly substituted
	// by big number of new metrics. For example, prometheus-operator creates many new
	// metrics for each new deployment.
	//
	// Allow fetching up to 20*maxMetrics metrics for the given time range
	// in the hope these metricIDs will be filtered out by other filters later.
	maxTimeRangeMetrics := 20 * maxMetrics
	metricIDsForTimeRange, err := is.getMetricIDsForTimeRange(tr, maxTimeRangeMetrics+1, tfs.accountID, tfs.projectID)
	if err == errMissingMetricIDsForDate {
		// Slow path: try to select find the tag filter without maxMetrics adjustement.
		minTf, minMetricIDs, err = is.getTagFilterWithMinMetricIDsCountAdaptive(tfs, maxMetrics)
		if err == nil {
			return minTf, minMetricIDs, nil
		}
		if err != errTooManyMetrics {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("cannot find tag filter matching less than %d time series; "+
			"either increase -search.maxUniqueTimeseries or use more specific tag filters", maxMetrics)
	}
	if err != nil {
		return nil, nil, err
	}
	if metricIDsForTimeRange.Len() <= maxTimeRangeMetrics {
		return nil, metricIDsForTimeRange, nil
	}

	// Slow path: try to select the tag filter without maxMetrics adjustement.
	minTf, minMetricIDs, err = is.getTagFilterWithMinMetricIDsCountAdaptive(tfs, maxMetrics)
	if err == nil {
		return minTf, minMetricIDs, nil
	}
	if err != errTooManyMetrics {
		return nil, nil, err
	}
	return nil, nil, fmt.Errorf("more than %d time series found on the time range %s; either increase -search.maxUniqueTimeseries or shrink the time range",
		maxMetrics, tr.String())
}

const maxDaysForDateMetricIDs = 40

func (is *indexSearch) adjustMaxMetricsAdaptive(tr TimeRange, maxMetrics int) int {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp) / msecPerDay
	if maxDate-minDate > maxDaysForDateMetricIDs {
		// Cannot reduce maxMetrics for the given time range,
		// since it is expensive extracting metricIDs for the given tr.
		return maxMetrics
	}
	hmPrev := is.db.prevHourMetricIDs.Load().(*hourMetricIDs)
	if !hmPrev.isFull {
		return maxMetrics
	}
	hourMetrics := hmPrev.m.Len()
	if maxMetrics > hourMetrics {
		// It is cheaper to filter on the hour or day metrics if the minimum
		// number of matching metrics across tfs exceeds hourMetrics.
		return hourMetrics
	}
	return maxMetrics
}

func (is *indexSearch) getTagFilterWithMinMetricIDsCountAdaptive(tfs *TagFilters, maxMetrics int) (*tagFilter, *uint64set.Set, error) {
	kb := &is.kb
	kb.B = append(kb.B[:0], uselessMultiTagFiltersKeyPrefix)
	kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
	kb.B = tfs.marshal(kb.B)
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
		if err != errTooManyMetrics {
			if err != nil {
				return nil, nil, err
			}
			if minMetricIDs.Len() < maxAllowedMetrics {
				// Found the tag filter with the minimum number of metrics.
				return minTf, minMetricIDs, nil
			}
		}

		// Too many metrics matched.
		if maxAllowedMetrics >= maxMetrics {
			// The tag filter with minimum matching metrics matches at least maxMetrics metrics.
			kb.B = append(kb.B[:0], uselessMultiTagFiltersKeyPrefix)
			kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
			kb.B = tfs.marshal(kb.B)
			is.db.uselessTagFiltersCache.Set(kb.B, uselessTagFilterCacheValue)
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

func (is *indexSearch) getTagFilterWithMinMetricIDsCount(tfs *TagFilters, maxMetrics int) (*tagFilter, *uint64set.Set, error) {
	var minMetricIDs *uint64set.Set
	var minTf *tagFilter
	kb := &is.kb
	uselessTagFilters := 0
	for i := range tfs.tfs {
		tf := &tfs.tfs[i]
		if tf.isNegative {
			// Skip negative filters.
			continue
		}

		kb.B = append(kb.B[:0], uselessSingleTagFilterKeyPrefix)
		kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
		kb.B = tf.Marshal(kb.B, tfs.accountID, tfs.projectID)
		if len(is.db.uselessTagFiltersCache.Get(nil, kb.B)) > 0 {
			// Skip useless work below, since the tf matches at least maxMetrics metrics.
			uselessTagFilters++
			continue
		}

		metricIDs, err := is.getMetricIDsForTagFilter(tf, maxMetrics)
		if err != nil {
			if err == errFallbackToMetricNameMatch {
				// Skip tag filters requiring to scan for too many metrics.
				kb.B = append(kb.B[:0], uselessSingleTagFilterKeyPrefix)
				kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
				kb.B = tf.Marshal(kb.B, tfs.accountID, tfs.projectID)
				is.db.uselessTagFiltersCache.Set(kb.B, uselessTagFilterCacheValue)
				uselessTagFilters++
				continue
			}
			return nil, nil, fmt.Errorf("cannot find MetricIDs for tagFilter %s: %s", tf, err)
		}
		if metricIDs.Len() >= maxMetrics {
			// The tf matches at least maxMetrics. Skip it
			kb.B = append(kb.B[:0], uselessSingleTagFilterKeyPrefix)
			kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
			kb.B = tf.Marshal(kb.B, tfs.accountID, tfs.projectID)
			is.db.uselessTagFiltersCache.Set(kb.B, uselessTagFilterCacheValue)
			uselessTagFilters++
			continue
		}

		minMetricIDs = metricIDs
		minTf = tf
		maxMetrics = minMetricIDs.Len()
		if maxMetrics <= 1 {
			// There is no need in inspecting other filters, since minTf
			// already matches 0 or 1 metric.
			break
		}
	}
	if minTf != nil {
		return minTf, minMetricIDs, nil
	}
	if uselessTagFilters == len(tfs.tfs) {
		// All the tag filters return at least maxMetrics entries.
		return nil, nil, errTooManyMetrics
	}

	// There is no positive filter with small number of matching metrics.
	// Create it, so it matches all the MetricIDs.
	kb.B = append(kb.B[:0], uselessNegativeTagFilterKeyPrefix)
	kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
	kb.B = tfs.marshal(kb.B)
	if len(is.db.uselessTagFiltersCache.Get(nil, kb.B)) > 0 {
		return nil, nil, errTooManyMetrics
	}
	metricIDs := &uint64set.Set{}
	if err := is.updateMetricIDsAll(metricIDs, tfs.accountID, tfs.projectID, maxMetrics); err != nil {
		return nil, nil, err
	}
	if metricIDs.Len() >= maxMetrics {
		kb.B = append(kb.B[:0], uselessNegativeTagFilterKeyPrefix)
		kb.B = encoding.MarshalUint64(kb.B, uint64(maxMetrics))
		kb.B = tfs.marshal(kb.B)
		is.db.uselessTagFiltersCache.Set(kb.B, uselessTagFilterCacheValue)
	}
	return nil, metricIDs, nil
}

func matchTagFilters(mn *MetricName, tfs []*tagFilter, kb *bytesutil.ByteBuffer) (bool, error) {
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs, mn.AccountID, mn.ProjectID)
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

			// Found the matching tag name. Match the value.
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
		if !tagMatched && !tf.isNegative {
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
	metricIDs := &uint64set.Set{}
	for _, tfs := range tfss {
		if len(tfs.tfs) == 0 {
			// Return all the metric ids
			if err := is.updateMetricIDsAll(metricIDs, tfs.accountID, tfs.projectID, maxMetrics+1); err != nil {
				return nil, err
			}
			if metricIDs.Len() > maxMetrics {
				return nil, fmt.Errorf("the number or unique timeseries exceeds %d; either narrow down the search or increase -search.maxUniqueTimeseries", maxMetrics)
			}
			// Stop the iteration, since we cannot find more metric ids with the remaining tfss.
			break
		}
		if err := is.updateMetricIDsForTagFilters(metricIDs, tfs, tr, maxMetrics+1); err != nil {
			return nil, err
		}
		if metricIDs.Len() > maxMetrics {
			return nil, fmt.Errorf("the number or matching unique timeseries exceeds %d; either narrow down the search or increase -search.maxUniqueTimeseries", maxMetrics)
		}
	}
	if metricIDs.Len() == 0 {
		// Nothing found
		return nil, nil
	}

	sortedMetricIDs := metricIDs.AppendTo(nil)

	// Filter out deleted metricIDs.
	dmis := is.db.getDeletedMetricIDs()
	if dmis.Len() > 0 {
		metricIDsFiltered := sortedMetricIDs[:0]
		for _, metricID := range sortedMetricIDs {
			if !dmis.Has(metricID) {
				metricIDsFiltered = append(metricIDsFiltered, metricID)
			}
		}
		sortedMetricIDs = metricIDsFiltered
	}

	return sortedMetricIDs, nil
}

func (is *indexSearch) updateMetricIDsForTagFilters(metricIDs *uint64set.Set, tfs *TagFilters, tr TimeRange, maxMetrics int) error {
	// Sort tag filters for faster ts.Seek below.
	sort.Slice(tfs.tfs, func(i, j int) bool { return bytes.Compare(tfs.tfs[i].prefix, tfs.tfs[j].prefix) < 0 })

	minTf, minMetricIDs, err := is.getTagFilterWithMinMetricIDsCountOptimized(tfs, tr, maxMetrics)
	if err != nil {
		return err
	}

	// Find intersection of minTf with other tfs.
	var tfsPostponed []*tagFilter
	successfulIntersects := 0
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
		successfulIntersects++
	}
	if len(tfsPostponed) > 0 && successfulIntersects == 0 {
		return is.updateMetricIDsByMetricNameMatch(metricIDs, minMetricIDs, tfsPostponed, tfs.accountID, tfs.projectID)
	}
	for i, tf := range tfsPostponed {
		mIDs, err := is.intersectMetricIDsWithTagFilter(tf, minMetricIDs)
		if err == errFallbackToMetricNameMatch {
			return is.updateMetricIDsByMetricNameMatch(metricIDs, minMetricIDs, tfsPostponed[i:], tfs.accountID, tfs.projectID)
		}
		if err != nil {
			return err
		}
		minMetricIDs = mIDs
	}
	metricIDs.Union(minMetricIDs)
	return nil
}

const (
	uselessSingleTagFilterKeyPrefix   = 0
	uselessMultiTagFiltersKeyPrefix   = 1
	uselessNegativeTagFilterKeyPrefix = 2
)

var uselessTagFilterCacheValue = []byte("1")

func (is *indexSearch) getMetricIDsForTagFilter(tf *tagFilter, maxMetrics int) (*uint64set.Set, error) {
	if tf.isNegative {
		logger.Panicf("BUG: isNegative must be false")
	}
	metricIDs := &uint64set.Set{}
	if len(tf.orSuffixes) > 0 {
		// Fast path for orSuffixes - seek for rows for each value from orSuffxies.
		if err := is.updateMetricIDsForOrSuffixesNoFilter(tf, maxMetrics, metricIDs); err != nil {
			if err == errFallbackToMetricNameMatch {
				return nil, err
			}
			return nil, fmt.Errorf("error when searching for metricIDs for tagFilter in fast path: %s; tagFilter=%s", err, tf)
		}
		return metricIDs, nil
	}

	// Slow path - scan for all the rows with the given prefix.
	maxLoops := maxMetrics * maxIndexScanLoopsPerMetric
	err := is.getMetricIDsForTagFilterSlow(tf, maxLoops, func(metricID uint64) bool {
		metricIDs.Add(metricID)
		return metricIDs.Len() < maxMetrics
	})
	if err != nil {
		if err == errFallbackToMetricNameMatch {
			return nil, err
		}
		return nil, fmt.Errorf("error when searching for metricIDs for tagFilter in slow path: %s; tagFilter=%s", err, tf)
	}
	return metricIDs, nil
}

func (is *indexSearch) getMetricIDsForTagFilterSlow(tf *tagFilter, maxLoops int, f func(metricID uint64) bool) error {
	if len(tf.orSuffixes) > 0 {
		logger.Panicf("BUG: the getMetricIDsForTagFilterSlow must be called only for empty tf.orSuffixes; got %s", tf.orSuffixes)
	}

	// Scan all the rows with tf.prefix and call f on every tf match.
	loops := 0
	ts := &is.ts
	kb := &is.kb
	mp := &is.mp
	mp.Reset()
	var prevMatchingSuffix []byte
	var prevMatch bool
	prefix := tf.prefix
	ts.Seek(prefix)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return nil
		}
		tail := item[len(prefix):]
		n := bytes.IndexByte(tail, tagSeparatorChar)
		if n < 0 {
			return fmt.Errorf("invalid tag->metricIDs line %q: cannot find tagSeparatorChar=%d", item, tagSeparatorChar)
		}
		suffix := tail[:n+1]
		tail = tail[n+1:]
		if err := mp.InitOnlyTail(item, tail); err != nil {
			return err
		}
		if prevMatch && string(suffix) == string(prevMatchingSuffix) {
			// Fast path: the same tag value found.
			// There is no need in checking it again with potentially
			// slow tf.matchSuffix, which may call regexp.
			loops += mp.MetricIDsLen()
			if loops > maxLoops {
				return errFallbackToMetricNameMatch
			}
			mp.ParseMetricIDs()
			for _, metricID := range mp.MetricIDs {
				if !f(metricID) {
					return nil
				}
			}
			continue
		}

		// Slow path: need tf.matchSuffix call.
		ok, err := tf.matchSuffix(suffix)
		if err != nil {
			return fmt.Errorf("error when matching %s against suffix %q: %s", tf, suffix, err)
		}
		if !ok {
			prevMatch = false
			// Optimization: skip all the metricIDs for the given tag value
			kb.B = append(kb.B[:0], item[:len(item)-len(tail)]...)
			// The last char in kb.B must be tagSeparatorChar. Just increment it
			// in order to jump to the next tag value.
			if len(kb.B) == 0 || kb.B[len(kb.B)-1] != tagSeparatorChar || tagSeparatorChar >= 0xff {
				return fmt.Errorf("data corruption: the last char in k=%X must be %X", kb.B, tagSeparatorChar)
			}
			kb.B[len(kb.B)-1]++
			ts.Seek(kb.B)
			continue
		}
		prevMatch = true
		prevMatchingSuffix = append(prevMatchingSuffix[:0], suffix...)
		loops += mp.MetricIDsLen()
		if loops > maxLoops {
			return errFallbackToMetricNameMatch
		}
		mp.ParseMetricIDs()
		for _, metricID := range mp.MetricIDs {
			if !f(metricID) {
				return nil
			}
		}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag filter prefix %q: %s", prefix, err)
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixesNoFilter(tf *tagFilter, maxMetrics int, metricIDs *uint64set.Set) error {
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
		if metricIDs.Len() >= maxMetrics {
			return nil
		}
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixesWithFilter(tf *tagFilter, metricIDs, filter *uint64set.Set) error {
	sortedFilter := filter.AppendTo(nil)
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

func (is *indexSearch) updateMetricIDsForOrSuffixNoFilter(prefix []byte, maxMetrics int, metricIDs *uint64set.Set) error {
	ts := &is.ts
	mp := &is.mp
	mp.Reset()
	maxLoops := maxMetrics * maxIndexScanLoopsPerMetric
	loops := 0
	ts.Seek(prefix)
	for metricIDs.Len() < maxMetrics && ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return nil
		}
		if err := mp.InitOnlyTail(item, item[len(prefix):]); err != nil {
			return err
		}
		loops += mp.MetricIDsLen()
		if loops > maxLoops {
			return errFallbackToMetricNameMatch
		}
		mp.ParseMetricIDs()
		for _, metricID := range mp.MetricIDs {
			metricIDs.Add(metricID)
		}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag filter prefix %q: %s", prefix, err)
	}
	return nil
}

func (is *indexSearch) updateMetricIDsForOrSuffixWithFilter(prefix []byte, metricIDs *uint64set.Set, sortedFilter []uint64, isNegative bool) error {
	if len(sortedFilter) == 0 {
		return nil
	}
	firstFilterMetricID := sortedFilter[0]
	lastFilterMetricID := sortedFilter[len(sortedFilter)-1]
	ts := &is.ts
	mp := &is.mp
	mp.Reset()
	maxLoops := len(sortedFilter) * maxIndexScanLoopsPerMetric
	loops := 0
	ts.Seek(prefix)
	var sf []uint64
	var metricID uint64
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return nil
		}
		if err := mp.InitOnlyTail(item, item[len(prefix):]); err != nil {
			return err
		}
		firstMetricID, lastMetricID := mp.FirstAndLastMetricIDs()
		if lastMetricID < firstFilterMetricID {
			// Skip the item, since it contains metricIDs lower
			// than metricIDs in sortedFilter.
			continue
		}
		if firstMetricID > lastFilterMetricID {
			// Stop searching, since the current item and all the subsequent items
			// contain metricIDs higher than metricIDs in sortedFilter.
			return nil
		}
		sf = sortedFilter
		loops += mp.MetricIDsLen()
		if loops > maxLoops {
			return errFallbackToMetricNameMatch
		}
		mp.ParseMetricIDs()
		for _, metricID = range mp.MetricIDs {
			if len(sf) == 0 {
				break
			}
			if metricID > sf[0] {
				n := sort.Search(len(sf), func(i int) bool {
					return i >= 0 && i < len(sf) && sf[i] >= metricID
				})
				sf = sf[n:]
				if len(sf) == 0 {
					break
				}
			}
			if metricID < sf[0] {
				continue
			}
			if isNegative {
				metricIDs.Del(metricID)
			} else {
				metricIDs.Add(metricID)
			}
			sf = sf[1:]
		}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for tag filter prefix %q: %s", prefix, err)
	}
	return nil
}

var errFallbackToMetricNameMatch = errors.New("fall back to updateMetricIDsByMetricNameMatch because of too many index scan loops")

var errMissingMetricIDsForDate = errors.New("missing metricIDs for date")

func (is *indexSearch) getMetricIDsForTimeRange(tr TimeRange, maxMetrics int, accountID, projectID uint32) (*uint64set.Set, error) {
	if tr.isZero() {
		return nil, errMissingMetricIDsForDate
	}
	atomic.AddUint64(&is.db.recentHourMetricIDsSearchCalls, 1)
	metricIDs, ok := is.getMetricIDsForRecentHours(tr, maxMetrics, accountID, projectID)
	if ok {
		// Fast path: tr covers the current and / or the previous hour.
		// Return the full list of metric ids for this time range.
		atomic.AddUint64(&is.db.recentHourMetricIDsSearchHits, 1)
		return metricIDs, nil
	}

	// Slow path: collect the metric ids for all the days covering the given tr.
	atomic.AddUint64(&is.db.dateMetricIDsSearchCalls, 1)
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	maxDate := uint64(tr.MaxTimestamp) / msecPerDay
	if maxDate-minDate > maxDaysForDateMetricIDs {
		// Too much dates must be covered. Give up.
		return nil, errMissingMetricIDsForDate
	}
	metricIDs = &uint64set.Set{}
	for minDate <= maxDate {
		if err := is.getMetricIDsForDate(minDate, metricIDs, maxMetrics, accountID, projectID); err != nil {
			return nil, err
		}
		minDate++
	}
	atomic.AddUint64(&is.db.dateMetricIDsSearchHits, 1)
	return metricIDs, nil
}

func (is *indexSearch) getMetricIDsForRecentHours(tr TimeRange, maxMetrics int, accountID, projectID uint32) (*uint64set.Set, bool) {
	// Return all the metricIDs for all the (AccountID, ProjectID) entries.
	// The caller is responsible for proper filtering later.
	minHour := uint64(tr.MinTimestamp) / msecPerHour
	maxHour := uint64(tr.MaxTimestamp) / msecPerHour
	hmCurr := is.db.currHourMetricIDs.Load().(*hourMetricIDs)
	if maxHour == hmCurr.hour && minHour == maxHour && hmCurr.isFull {
		// The tr fits the current hour.
		// Return a copy of hmCurr.m, because the caller may modify
		// the returned map.
		k := accountProjectKey{
			AccountID: accountID,
			ProjectID: projectID,
		}
		m := hmCurr.byTenant[k]
		if m.Len() > maxMetrics {
			return nil, false
		}
		return m.Clone(), true
	}
	hmPrev := is.db.prevHourMetricIDs.Load().(*hourMetricIDs)
	if maxHour == hmPrev.hour && minHour == maxHour && hmPrev.isFull {
		// The tr fits the previous hour.
		// Return a copy of hmPrev.m, because the caller may modify
		// the returned map.
		k := accountProjectKey{
			AccountID: accountID,
			ProjectID: projectID,
		}
		m := hmPrev.byTenant[k]
		if m.Len() > maxMetrics {
			return nil, false
		}
		return m.Clone(), true
	}
	if maxHour == hmCurr.hour && minHour == hmPrev.hour && hmCurr.isFull && hmPrev.isFull {
		// The tr spans the previous and the current hours.
		k := accountProjectKey{
			AccountID: accountID,
			ProjectID: projectID,
		}
		mCurr := hmCurr.byTenant[k]
		mPrev := hmPrev.byTenant[k]
		if mCurr.Len()+mPrev.Len() > maxMetrics {
			return nil, false
		}
		metricIDs := mCurr.Clone()
		metricIDs.Union(mPrev)
		return metricIDs, true
	}
	return nil, false
}

func (db *indexDB) storeDateMetricID(date, metricID uint64, accountID, projectID uint32) error {
	is := db.getIndexSearch()
	ok, err := is.hasDateMetricID(date, metricID, accountID, projectID)
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
	items.B = marshalCommonPrefix(items.B[:0], nsPrefixDateToMetricID, accountID, projectID)
	items.B = encoding.MarshalUint64(items.B, date)
	items.B = encoding.MarshalUint64(items.B, metricID)
	items.Next()
	err = db.tb.AddItems(items.Items)
	putIndexItems(items)
	return err
}

func (is *indexSearch) hasDateMetricID(date, metricID uint64, accountID, projectID uint32) (bool, error) {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID, accountID, projectID)
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

func (is *indexSearch) getMetricIDsForDate(date uint64, metricIDs *uint64set.Set, maxMetrics int, accountID, projectID uint32) error {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID, accountID, projectID)
	kb.B = encoding.MarshalUint64(kb.B, date)
	ts.Seek(kb.B)
	items := 0
	for metricIDs.Len() < maxMetrics && ts.NextItem() {
		if !bytes.HasPrefix(ts.Item, kb.B) {
			break
		}
		// Extract MetricID from ts.Item (the last 8 bytes).
		v := ts.Item[len(kb.B):]
		if len(v) != 8 {
			return fmt.Errorf("cannot extract metricID from k; want %d bytes; got %d bytes", 8, len(v))
		}
		metricID := encoding.UnmarshalUint64(v)
		metricIDs.Add(metricID)
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

func (is *indexSearch) containsTimeRange(tr TimeRange, accountID, projectID uint32) (bool, error) {
	ts := &is.ts
	kb := &is.kb

	// Verify whether the maximum date in `ts` covers tr.MinTimestamp.
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID, accountID, projectID)
	kb.B = encoding.MarshalUint64(kb.B, minDate)
	ts.Seek(kb.B)
	if !ts.NextItem() {
		if err := ts.Error(); err != nil {
			return false, fmt.Errorf("error when searching for minDate=%d, prefix %q: %s", minDate, kb.B, err)
		}
		return false, nil
	}
	if !bytes.HasPrefix(ts.Item, kb.B[:1]) {
		// minDate exceeds max date from ts.
		return false, nil
	}
	return true, nil
}

func (is *indexSearch) updateMetricIDsAll(metricIDs *uint64set.Set, accountID, projectID uint32, maxMetrics int) error {
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixMetricIDToTSID, accountID, projectID)
	prefix := kb.B
	ts.Seek(prefix)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			return nil
		}
		tail := item[len(prefix):]
		if len(tail) < 8 {
			return fmt.Errorf("cannot unmarshal metricID from item with size %d; need at least 9 bytes; item=%q", len(tail), tail)
		}
		metricID := encoding.UnmarshalUint64(tail)
		metricIDs.Add(metricID)
		if metricIDs.Len() >= maxMetrics {
			return nil
		}
	}
	if err := ts.Error(); err != nil {
		return fmt.Errorf("error when searching for all metricIDs by prefix %q: %s", prefix, err)
	}
	return nil
}

// The maximum number of index scan loops per already found metric.
// Bigger number of loops is slower than updateMetricIDsByMetricNameMatch
// over the found metrics.
const maxIndexScanLoopsPerMetric = 400

func (is *indexSearch) intersectMetricIDsWithTagFilter(tf *tagFilter, filter *uint64set.Set) (*uint64set.Set, error) {
	if filter.Len() == 0 {
		return nil, nil
	}
	metricIDs := filter
	if !tf.isNegative {
		metricIDs = &uint64set.Set{}
	}
	if len(tf.orSuffixes) > 0 {
		// Fast path for orSuffixes - seek for rows for each value from orSuffixes.
		if err := is.updateMetricIDsForOrSuffixesWithFilter(tf, metricIDs, filter); err != nil {
			if err == errFallbackToMetricNameMatch {
				return nil, err
			}
			return nil, fmt.Errorf("error when intersecting metricIDs for tagFilter in fast path: %s; tagFilter=%s", err, tf)
		}
		return metricIDs, nil
	}

	// Slow path - scan for all the rows with the given prefix.
	maxLoops := filter.Len() * maxIndexScanLoopsPerMetric
	err := is.getMetricIDsForTagFilterSlow(tf, maxLoops, func(metricID uint64) bool {
		if tf.isNegative {
			// filter must be equal to metricIDs
			metricIDs.Del(metricID)
			return true
		}
		if filter.Has(metricID) {
			metricIDs.Add(metricID)
		}
		return true
	})
	if err != nil {
		if err == errFallbackToMetricNameMatch {
			return nil, err
		}
		return nil, fmt.Errorf("error when intersecting metricIDs for tagFilter in slow path: %s; tagFilter=%s", err, tf)
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

func marshalCommonPrefix(dst []byte, nsPrefix byte, accountID, projectID uint32) []byte {
	dst = append(dst, nsPrefix)
	dst = encoding.MarshalUint32(dst, accountID)
	dst = encoding.MarshalUint32(dst, projectID)
	return dst
}

func unmarshalCommonPrefix(src []byte) ([]byte, byte, uint32, uint32, error) {
	if len(src) < commonPrefixLen {
		return nil, 0, 0, 0, fmt.Errorf("cannot unmarshal common prefix from %d bytes; need at least %d bytes; data=%X", len(src), commonPrefixLen, src)
	}
	prefix := src[0]
	accountID := encoding.UnmarshalUint32(src[1:])
	projectID := encoding.UnmarshalUint32(src[5:])
	return src[commonPrefixLen:], prefix, accountID, projectID, nil
}

// 1 byte for prefix, 4 bytes for accountID, 4 bytes for projectID
const commonPrefixLen = 9

type tagToMetricIDsRowParser struct {
	// AccountID contains parsed value after Init call
	AccountID uint32

	// ProjectID contains parsed value after Init call
	ProjectID uint32

	// MetricIDs contains parsed MetricIDs after ParseMetricIDs call
	MetricIDs []uint64

	// Tag contains parsed tag after Init call
	Tag Tag

	// tail contains the remaining unparsed metricIDs
	tail []byte
}

func (mp *tagToMetricIDsRowParser) Reset() {
	mp.AccountID = 0
	mp.ProjectID = 0
	mp.MetricIDs = mp.MetricIDs[:0]
	mp.Tag.Reset()
	mp.tail = nil
}

// Init initializes mp from b, which should contain encoded tag->metricIDs row.
//
// b cannot be re-used until Reset call.
func (mp *tagToMetricIDsRowParser) Init(b []byte) error {
	tail, prefix, accountID, projectID, err := unmarshalCommonPrefix(b)
	if err != nil {
		return fmt.Errorf("invalid tag->metricIDs row %q: %s", b, err)
	}
	if prefix != nsPrefixTagToMetricIDs {
		return fmt.Errorf("invalid prefix for tag->metricIDs row %q; got %d; want %d", b, prefix, nsPrefixTagToMetricIDs)
	}
	mp.AccountID = accountID
	mp.ProjectID = projectID
	tail, err = mp.Tag.Unmarshal(tail)
	if err != nil {
		return fmt.Errorf("cannot unmarshal tag from tag->metricIDs row %q: %s", b, err)
	}
	return mp.InitOnlyTail(b, tail)
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
	return nil
}

// EqualPrefix returns true if prefixes for mp and x are equal.
//
// Prefix contains (tag, accountID, projectID)
func (mp *tagToMetricIDsRowParser) EqualPrefix(x *tagToMetricIDsRowParser) bool {
	if !mp.Tag.Equal(&x.Tag) {
		return false
	}
	return mp.ProjectID == x.ProjectID && mp.AccountID == x.AccountID
}

// FirstAndLastMetricIDs returns the first and the last metricIDs in the mp.tail.
func (mp *tagToMetricIDsRowParser) FirstAndLastMetricIDs() (uint64, uint64) {
	tail := mp.tail
	if len(tail) < 8 {
		logger.Panicf("BUG: cannot unmarshal metricID from %d bytes; need 8 bytes", len(tail))
		return 0, 0
	}
	firstMetricID := encoding.UnmarshalUint64(tail)
	lastMetricID := firstMetricID
	if len(tail) > 8 {
		lastMetricID = encoding.UnmarshalUint64(tail[len(tail)-8:])
	}
	return firstMetricID, lastMetricID
}

// MetricIDsLen returns the number of MetricIDs in the mp.tail
func (mp *tagToMetricIDsRowParser) MetricIDsLen() int {
	return len(mp.tail) / 8
}

// ParseMetricIDs parses MetricIDs from mp.tail into mp.MetricIDs.
func (mp *tagToMetricIDsRowParser) ParseMetricIDs() {
	tail := mp.tail
	mp.MetricIDs = mp.MetricIDs[:0]
	n := len(tail) / 8
	if n <= cap(mp.MetricIDs) {
		mp.MetricIDs = mp.MetricIDs[:n]
	} else {
		mp.MetricIDs = append(mp.MetricIDs[:cap(mp.MetricIDs)], make([]uint64, n-cap(mp.MetricIDs))...)
	}
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
}

// IsDeletedTag verifies whether the tag from mp is deleted according to dmis.
//
// dmis must contain deleted MetricIDs.
func (mp *tagToMetricIDsRowParser) IsDeletedTag(dmis *uint64set.Set) bool {
	if dmis.Len() == 0 {
		return false
	}
	mp.ParseMetricIDs()
	for _, metricID := range mp.MetricIDs {
		if !dmis.Has(metricID) {
			return false
		}
	}
	return true
}

func mergeTagToMetricIDsRows(data []byte, items [][]byte) ([]byte, [][]byte) {
	// Perform quick checks whether items contain tag->metricIDs rows
	// based on the fact that items are sorted.
	if len(items) <= 2 {
		// The first and the last row must remain unchanged.
		return data, items
	}
	firstItem := items[0]
	if len(firstItem) > 0 && firstItem[0] > nsPrefixTagToMetricIDs {
		return data, items
	}
	lastItem := items[len(items)-1]
	if len(lastItem) > 0 && lastItem[0] < nsPrefixTagToMetricIDs {
		return data, items
	}

	// items contain at least one tag->metricIDs row. Merge rows with common tag.
	tmm := getTagToMetricIDsRowsMerger()
	tmm.dataCopy = append(tmm.dataCopy[:0], data...)
	tmm.itemsCopy = append(tmm.itemsCopy[:0], items...)
	mp := &tmm.mp
	mpPrev := &tmm.mpPrev
	dstData := data[:0]
	dstItems := items[:0]
	for i, item := range items {
		if len(item) == 0 || item[0] != nsPrefixTagToMetricIDs || i == 0 || i == len(items)-1 {
			// Write rows other than tag->metricIDs as-is.
			// Additionally write the first and the last row as-is in order to preserve
			// sort order for adjancent blocks.
			if len(tmm.pendingMetricIDs) > 0 {
				dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
			}
			dstData = append(dstData, item...)
			dstItems = append(dstItems, dstData[len(dstData)-len(item):])
			continue
		}
		if err := mp.Init(item); err != nil {
			logger.Panicf("FATAL: cannot parse tag->metricIDs row during merge: %s", err)
		}
		if mp.MetricIDsLen() >= maxMetricIDsPerRow {
			if len(tmm.pendingMetricIDs) > 0 {
				dstData, dstItems = tmm.flushPendingMetricIDs(dstData, dstItems, mpPrev)
			}
			dstData = append(dstData, item...)
			dstItems = append(dstItems, dstData[len(dstData)-len(item):])
			continue
		}
		if len(tmm.pendingMetricIDs) > 0 && !mp.EqualPrefix(mpPrev) {
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
	if err := checkItemsSorted(dstItems); err != nil {
		logger.Errorf("please report this error at https://github.com/VictoriaMetrics/VictoriaMetrics/issues : %s", err)
		dstData = append(dstData[:0], tmm.dataCopy...)
		dstItems = dstItems[:0]
		// tmm.itemsCopy can point to overwritten data, so it must be updated
		// to point to real data from tmm.dataCopy.
		buf := dstData
		for _, item := range tmm.itemsCopy {
			dstItems = append(dstItems, buf[:len(item)])
			buf = buf[len(item):]
		}
		if err := checkItemsSorted(dstItems); err != nil {
			logger.Panicf("BUG: the original items weren't sorted: %s", err)
		}
	}
	putTagToMetricIDsRowsMerger(tmm)
	return dstData, dstItems
}

func checkItemsSorted(items [][]byte) error {
	if len(items) == 0 {
		return nil
	}
	prevItem := items[0]
	for _, currItem := range items[1:] {
		if string(prevItem) > string(currItem) {
			return fmt.Errorf("items aren't sorted: prevItem > currItem\nprevItem=%X\ncurrItem=%X\nitems=%X", prevItem, currItem, items)
		}
		prevItem = currItem
	}
	return nil
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

	itemsCopy [][]byte
	dataCopy  []byte
}

func (tmm *tagToMetricIDsRowsMerger) Reset() {
	tmm.pendingMetricIDs = tmm.pendingMetricIDs[:0]
	tmm.mp.Reset()
	tmm.mpPrev.Reset()

	tmm.itemsCopy = tmm.itemsCopy[:0]
	tmm.dataCopy = tmm.dataCopy[:0]
}

func (tmm *tagToMetricIDsRowsMerger) flushPendingMetricIDs(dstData []byte, dstItems [][]byte, mp *tagToMetricIDsRowParser) ([]byte, [][]byte) {
	if len(tmm.pendingMetricIDs) == 0 {
		logger.Panicf("BUG: pendingMetricIDs must be non-empty")
	}
	// Use sort.Sort instead of sort.Slice in order to reduce memory allocations.
	sort.Sort(&tmm.pendingMetricIDs)
	tmm.pendingMetricIDs = removeDuplicateMetricIDs(tmm.pendingMetricIDs)

	// Marshal pendingMetricIDs
	dstDataLen := len(dstData)
	dstData = marshalCommonPrefix(dstData, nsPrefixTagToMetricIDs, mp.AccountID, mp.ProjectID)
	dstData = mp.Tag.Marshal(dstData)
	for _, metricID := range tmm.pendingMetricIDs {
		dstData = encoding.MarshalUint64(dstData, metricID)
	}
	dstItems = append(dstItems, dstData[dstDataLen:])
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
