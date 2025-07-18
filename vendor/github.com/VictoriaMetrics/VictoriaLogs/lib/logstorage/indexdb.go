package logstorage

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

const (
	// (tenantID:streamID) entries have this prefix
	//
	// These entries are used for detecting whether the given stream is already registered
	nsPrefixStreamID = 0

	// (tenantID:streamID -> streamTagsCanonical) entries have this prefix
	nsPrefixStreamIDToStreamTags = 1

	// (tenantID:name:value => streamIDs) entries have this prefix
	nsPrefixTagToStreamIDs = 2
)

// IndexdbStats contains indexdb stats
type IndexdbStats struct {
	// StreamsCreatedTotal is the number of log streams created since the indexdb initialization.
	StreamsCreatedTotal uint64

	// IndexdbSizeBytes is the size of data in indexdb.
	IndexdbSizeBytes uint64

	// IndexdbItemsCount is the number of items in indexdb.
	IndexdbItemsCount uint64

	// IndexdbBlocksCount is the number of blocks in indexdb.
	IndexdbBlocksCount uint64

	// IndexdbPartsCount is the number of parts in indexdb.
	IndexdbPartsCount uint64
}

type indexdb struct {
	// streamsCreatedTotal is the number of log streams created since the indexdb initialization.
	streamsCreatedTotal atomic.Uint64

	// the generation of the filterStreamCache.
	// It is updated each time new item is added to tb.
	filterStreamCacheGeneration atomic.Uint32

	// path is the path to indexdb
	path string

	// partitionName is the name of the partition for the indexdb.
	partitionName string

	// tb is the storage for indexdb
	tb *mergeset.Table

	// indexSearchPool is a pool of indexSearch struct for the given indexdb
	indexSearchPool sync.Pool

	// s is the storage where indexdb belongs to.
	s *Storage
}

func mustCreateIndexdb(path string) {
	fs.MustMkdirFailIfExist(path)
}

func mustOpenIndexdb(path, partitionName string, s *Storage) *indexdb {
	idb := &indexdb{
		path:          path,
		partitionName: partitionName,
		s:             s,
	}
	var isReadOnly atomic.Bool
	idb.tb = mergeset.MustOpenTable(path, s.flushInterval, idb.invalidateStreamFilterCache, mergeTagToStreamIDsRows, &isReadOnly)
	return idb
}

func mustCloseIndexdb(idb *indexdb) {
	idb.tb.MustClose()
	idb.tb = nil
	idb.s = nil
	idb.partitionName = ""
	idb.path = ""
}

func (idb *indexdb) debugFlush() {
	idb.tb.DebugFlush()
}

func (idb *indexdb) updateStats(d *IndexdbStats) {
	d.StreamsCreatedTotal += idb.streamsCreatedTotal.Load()

	var tm mergeset.TableMetrics
	idb.tb.UpdateMetrics(&tm)

	d.IndexdbSizeBytes += tm.InmemorySizeBytes + tm.FileSizeBytes
	d.IndexdbItemsCount += tm.InmemoryItemsCount + tm.FileItemsCount
	d.IndexdbPartsCount += tm.InmemoryPartsCount + tm.FilePartsCount
	d.IndexdbBlocksCount += tm.InmemoryBlocksCount + tm.FileBlocksCount
}

func (idb *indexdb) appendStreamTagsByStreamID(dst []byte, sid *streamID) []byte {
	is := idb.getIndexSearch()
	defer idb.putIndexSearch(is)

	ts := &is.ts
	kb := &is.kb

	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixStreamIDToStreamTags, sid.tenantID)
	kb.B = sid.id.marshal(kb.B)

	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return dst
		}
		logger.Panicf("FATAL: unexpected error when searching for StreamTags by streamID=%s in indexdb: %s", sid, err)
	}
	data := ts.Item[len(kb.B):]
	dst = append(dst, data...)
	return dst
}

// hasStreamID returns true if streamID exists in idb
func (idb *indexdb) hasStreamID(sid *streamID) bool {
	is := idb.getIndexSearch()
	defer idb.putIndexSearch(is)

	ts := &is.ts
	kb := &is.kb

	kb.B = marshalCommonPrefix(kb.B, nsPrefixStreamID, sid.tenantID)
	kb.B = sid.id.marshal(kb.B)

	if err := ts.FirstItemWithPrefix(kb.B); err != nil {
		if err == io.EOF {
			return false
		}
		logger.Panicf("FATAL: unexpected error when searching for streamID=%s in indexdb: %s", sid, err)
	}
	return len(kb.B) == len(ts.Item)
}

type indexSearch struct {
	idb *indexdb
	ts  mergeset.TableSearch
	kb  bytesutil.ByteBuffer
}

func (idb *indexdb) getIndexSearch() *indexSearch {
	v := idb.indexSearchPool.Get()
	if v == nil {
		v = &indexSearch{
			idb: idb,
		}
	}
	is := v.(*indexSearch)
	is.ts.Init(idb.tb, false)
	return is
}

func (idb *indexdb) putIndexSearch(is *indexSearch) {
	is.idb = nil
	is.ts.MustClose()
	is.kb.Reset()

	idb.indexSearchPool.Put(is)
}

// searchStreamIDs returns streamIDs for the given tenantIDs and the given stream filters
func (idb *indexdb) searchStreamIDs(tenantIDs []TenantID, sf *StreamFilter) []streamID {
	// Try obtaining streamIDs from cache
	streamIDs, ok := idb.loadStreamIDsFromCache(tenantIDs, sf)
	if ok {
		// Fast path - streamIDs found in the cache.
		return streamIDs
	}

	// Slow path - collect streamIDs from indexdb.

	// Collect streamIDs for all the specified tenantIDs.
	is := idb.getIndexSearch()
	m := make(map[streamID]struct{})
	for _, tenantID := range tenantIDs {
		for _, asf := range sf.orFilters {
			is.updateStreamIDs(m, tenantID, asf)
		}
	}
	idb.putIndexSearch(is)

	// Convert the collected streamIDs from m to sorted slice.
	streamIDs = make([]streamID, 0, len(m))
	for streamID := range m {
		streamIDs = append(streamIDs, streamID)
	}
	sortStreamIDs(streamIDs)

	// Store the collected streamIDs to cache.
	idb.storeStreamIDsToCache(tenantIDs, sf, streamIDs)

	return streamIDs
}

func sortStreamIDs(streamIDs []streamID) {
	sort.Slice(streamIDs, func(i, j int) bool {
		return streamIDs[i].less(&streamIDs[j])
	})
}

func (is *indexSearch) updateStreamIDs(dst map[streamID]struct{}, tenantID TenantID, asf *andStreamFilter) {
	var m map[u128]struct{}
	for _, tf := range asf.tagFilters {
		ids := is.getStreamIDsForTagFilter(tenantID, tf)
		if len(ids) == 0 {
			// There is no need in checking the remaining filters,
			// since the result will be empty in any case.
			return
		}
		if m == nil {
			m = ids
		} else {
			for id := range m {
				if _, ok := ids[id]; !ok {
					delete(m, id)
				}
			}
		}
	}

	var sid streamID
	for id := range m {
		sid.tenantID = tenantID
		sid.id = id
		dst[sid] = struct{}{}
	}
}

func (is *indexSearch) getStreamIDsForTagFilter(tenantID TenantID, tf *streamTagFilter) map[u128]struct{} {
	switch tf.op {
	case "=":
		if tf.value == "" {
			// (field="")
			return is.getStreamIDsForEmptyTagValue(tenantID, tf.tagName)
		}
		// (field="value")
		return is.getStreamIDsForNonEmptyTagValue(tenantID, tf.tagName, tf.value)
	case "!=":
		if tf.value == "" {
			// (field!="")
			return is.getStreamIDsForTagName(tenantID, tf.tagName)
		}
		// (field!="value") => (all and not field="value")
		ids := is.getStreamIDsForTenant(tenantID)
		idsForTag := is.getStreamIDsForNonEmptyTagValue(tenantID, tf.tagName, tf.value)
		for id := range idsForTag {
			delete(ids, id)
		}
		return ids
	case "=~":
		re := tf.regexp
		if re.MatchString("") {
			// (field=~"|re") => (field="" or field=~"re")
			ids := is.getStreamIDsForEmptyTagValue(tenantID, tf.tagName)
			idsForRe := is.getStreamIDsForTagRegexp(tenantID, tf.tagName, re)
			for id := range idsForRe {
				ids[id] = struct{}{}
			}
			return ids
		}
		return is.getStreamIDsForTagRegexp(tenantID, tf.tagName, re)
	case "!~":
		re := tf.regexp
		if re.MatchString("") {
			// (field!~"|re") => (field!="" and not field=~"re")
			ids := is.getStreamIDsForTagName(tenantID, tf.tagName)
			if len(ids) == 0 {
				return ids
			}
			idsForRe := is.getStreamIDsForTagRegexp(tenantID, tf.tagName, re)
			for id := range idsForRe {
				delete(ids, id)
			}
			return ids
		}
		// (field!~"re") => (all and not field=~"re")
		ids := is.getStreamIDsForTenant(tenantID)
		idsForRe := is.getStreamIDsForTagRegexp(tenantID, tf.tagName, re)
		for id := range idsForRe {
			delete(ids, id)
		}
		return ids
	default:
		logger.Panicf("BUG: unexpected operation in stream tag filter: %q", tf.op)
		return nil
	}
}

func (is *indexSearch) getStreamIDsForNonEmptyTagValue(tenantID TenantID, tagName, tagValue string) map[u128]struct{} {
	ids := make(map[u128]struct{})
	var sp tagToStreamIDsRowParser

	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToStreamIDs, tenantID)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagName))
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagValue))
	prefix := kb.B
	ts.Seek(prefix)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		tail := item[len(prefix):]
		sp.UpdateStreamIDs(ids, tail)
	}
	if err := ts.Error(); err != nil {
		logger.Panicf("FATAL: unexpected error: %s", err)
	}

	return ids
}

func (is *indexSearch) getStreamIDsForEmptyTagValue(tenantID TenantID, tagName string) map[u128]struct{} {
	ids := is.getStreamIDsForTenant(tenantID)
	idsForTag := is.getStreamIDsForTagName(tenantID, tagName)
	for id := range idsForTag {
		delete(ids, id)
	}
	return ids
}

func (is *indexSearch) getStreamIDsForTenant(tenantID TenantID) map[u128]struct{} {
	ids := make(map[u128]struct{})
	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixStreamID, tenantID)
	prefix := kb.B
	ts.Seek(prefix)
	var id u128
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		tail, err := id.unmarshal(item[len(prefix):])
		if err != nil {
			logger.Panicf("FATAL: cannot unmarshal streamID from (tenantID:streamID) entry: %s", err)
		}
		if len(tail) > 0 {
			logger.Panicf("FATAL: unexpected non-empty tail left after unmarshaling streamID from (tenantID:streamID); tail len=%d", len(tail))
		}
		ids[id] = struct{}{}
	}
	if err := ts.Error(); err != nil {
		logger.Panicf("FATAL: unexpected error: %s", err)
	}

	return ids
}

func (is *indexSearch) getStreamIDsForTagName(tenantID TenantID, tagName string) map[u128]struct{} {
	ids := make(map[u128]struct{})
	var sp tagToStreamIDsRowParser

	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToStreamIDs, tenantID)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagName))
	prefix := kb.B
	ts.Seek(prefix)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		tail := item[len(prefix):]
		n := bytes.IndexByte(tail, tagSeparatorChar)
		if n < 0 {
			logger.Panicf("FATAL: cannot find the end of tag value")
		}
		tail = tail[n+1:]
		sp.UpdateStreamIDs(ids, tail)
	}
	if err := ts.Error(); err != nil {
		logger.Panicf("FATAL: unexpected error: %s", err)
	}

	return ids
}

func (is *indexSearch) getStreamIDsForTagRegexp(tenantID TenantID, tagName string, re *regexutil.PromRegex) map[u128]struct{} {
	ids := make(map[u128]struct{})
	var sp tagToStreamIDsRowParser
	var tagValue, prevMatchingTagValue []byte
	var err error

	ts := &is.ts
	kb := &is.kb
	kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToStreamIDs, tenantID)
	kb.B = marshalTagValue(kb.B, bytesutil.ToUnsafeBytes(tagName))
	prefix := kb.B
	ts.Seek(prefix)
	for ts.NextItem() {
		item := ts.Item
		if !bytes.HasPrefix(item, prefix) {
			break
		}
		tail := item[len(prefix):]
		tail, tagValue, err = unmarshalTagValue(tagValue[:0], tail)
		if err != nil {
			logger.Panicf("FATAL: cannot unmarshal tag value: %s", err)
		}
		if !bytes.Equal(tagValue, prevMatchingTagValue) {
			if !re.MatchString(bytesutil.ToUnsafeString(tagValue)) {
				continue
			}
			prevMatchingTagValue = append(prevMatchingTagValue[:0], tagValue...)
		}
		sp.UpdateStreamIDs(ids, tail)
	}
	if err := ts.Error(); err != nil {
		logger.Panicf("FATAL: unexpected error: %s", err)
	}

	return ids
}

func (idb *indexdb) mustRegisterStream(streamID *streamID, streamTagsCanonical string) {
	st := GetStreamTags()
	mustUnmarshalStreamTags(st, streamTagsCanonical)
	tenantID := streamID.tenantID

	bi := getBatchItems()
	buf := bi.buf[:0]
	items := bi.items[:0]

	// Register tenantID:streamID entry.
	bufLen := len(buf)
	buf = marshalCommonPrefix(buf, nsPrefixStreamID, tenantID)
	buf = streamID.id.marshal(buf)
	items = append(items, buf[bufLen:])

	// Register tenantID:streamID -> streamTagsCanonical entry.
	bufLen = len(buf)
	buf = marshalCommonPrefix(buf, nsPrefixStreamIDToStreamTags, tenantID)
	buf = streamID.id.marshal(buf)
	buf = append(buf, streamTagsCanonical...)
	items = append(items, buf[bufLen:])

	// Register tenantID:name:value -> streamIDs entries.
	tags := st.tags
	for i := range tags {
		bufLen = len(buf)
		buf = marshalCommonPrefix(buf, nsPrefixTagToStreamIDs, tenantID)
		buf = tags[i].indexdbMarshal(buf)
		buf = streamID.id.marshal(buf)
		items = append(items, buf[bufLen:])
	}
	PutStreamTags(st)

	// Add items to the storage
	idb.tb.AddItems(items)

	bi.buf = buf
	bi.items = items
	putBatchItems(bi)

	idb.streamsCreatedTotal.Add(1)
}

func (idb *indexdb) invalidateStreamFilterCache() {
	// This function must be fast, since it is called each
	// time new indexdb entry is added.
	idb.filterStreamCacheGeneration.Add(1)
}

func (idb *indexdb) marshalStreamFilterCacheKey(dst []byte, tenantIDs []TenantID, sf *StreamFilter) []byte {
	dst = encoding.MarshalUint32(dst, idb.filterStreamCacheGeneration.Load())
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(idb.partitionName))
	dst = encoding.MarshalVarUint64(dst, uint64(len(tenantIDs)))
	for i := range tenantIDs {
		dst = tenantIDs[i].marshal(dst)
	}
	dst = sf.marshalForCacheKey(dst)
	return dst
}

func (idb *indexdb) loadStreamIDsFromCache(tenantIDs []TenantID, sf *StreamFilter) ([]streamID, bool) {
	bb := bbPool.Get()
	bb.B = idb.marshalStreamFilterCacheKey(bb.B[:0], tenantIDs, sf)
	v, ok := idb.s.filterStreamCache.Get(bb.B)
	bbPool.Put(bb)
	if !ok {
		// Cache miss
		return nil, false
	}
	// Cache hit - unpack streamIDs from data.
	data := *(v.(*[]byte))
	n, nSize := encoding.UnmarshalVarUint64(data)
	if nSize <= 0 {
		logger.Panicf("BUG: unexpected error when unmarshaling the number of streamIDs from cache")
	}
	src := data[nSize:]
	streamIDs := make([]streamID, n)
	for i := uint64(0); i < n; i++ {
		tail, err := streamIDs[i].unmarshal(src)
		if err != nil {
			logger.Panicf("BUG: unexpected error when unmarshaling streamID #%d: %s", i, err)
		}
		src = tail
	}
	if len(src) > 0 {
		logger.Panicf("BUG: unexpected non-empty tail left with len=%d", len(src))
	}
	return streamIDs, true
}

func (idb *indexdb) storeStreamIDsToCache(tenantIDs []TenantID, sf *StreamFilter, streamIDs []streamID) {
	// marshal streamIDs
	var b []byte
	b = encoding.MarshalVarUint64(b, uint64(len(streamIDs)))
	for i := 0; i < len(streamIDs); i++ {
		b = streamIDs[i].marshal(b)
	}

	// Store marshaled streamIDs to cache.
	bb := bbPool.Get()
	bb.B = idb.marshalStreamFilterCacheKey(bb.B[:0], tenantIDs, sf)
	idb.s.filterStreamCache.Set(bb.B, &b)
	bbPool.Put(bb)
}

type batchItems struct {
	buf []byte

	items [][]byte
}

func (bi *batchItems) reset() {
	bi.buf = bi.buf[:0]

	items := bi.items
	for i := range items {
		items[i] = nil
	}
	bi.items = items[:0]
}

func getBatchItems() *batchItems {
	v := batchItemsPool.Get()
	if v == nil {
		return &batchItems{}
	}
	return v.(*batchItems)
}

func putBatchItems(bi *batchItems) {
	bi.reset()
	batchItemsPool.Put(bi)
}

var batchItemsPool sync.Pool

func mergeTagToStreamIDsRows(data []byte, items []mergeset.Item) ([]byte, []mergeset.Item) {
	// Perform quick checks whether items contain rows starting from nsPrefixTagToStreamIDs
	// based on the fact that items are sorted.
	if len(items) <= 2 {
		// The first and the last row must remain unchanged.
		return data, items
	}
	firstItem := items[0].Bytes(data)
	if len(firstItem) > 0 && firstItem[0] > nsPrefixTagToStreamIDs {
		return data, items
	}
	lastItem := items[len(items)-1].Bytes(data)
	if len(lastItem) > 0 && lastItem[0] < nsPrefixTagToStreamIDs {
		return data, items
	}

	// items contain at least one row starting from nsPrefixTagToStreamIDs. Merge rows with common tag.
	tsm := getTagToStreamIDsRowsMerger()
	tsm.dataCopy = append(tsm.dataCopy[:0], data...)
	tsm.itemsCopy = append(tsm.itemsCopy[:0], items...)
	sp := &tsm.sp
	spPrev := &tsm.spPrev
	dstData := data[:0]
	dstItems := items[:0]
	for i, it := range items {
		item := it.Bytes(data)
		if len(item) == 0 || item[0] != nsPrefixTagToStreamIDs || i == 0 || i == len(items)-1 {
			// Write rows not starting with nsPrefixTagToStreamIDs as-is.
			// Additionally write the first and the last row as-is in order to preserve
			// sort order for adjacent blocks.
			dstData, dstItems = tsm.flushPendingStreamIDs(dstData, dstItems, spPrev)
			dstData = append(dstData, item...)
			dstItems = append(dstItems, mergeset.Item{
				Start: uint32(len(dstData) - len(item)),
				End:   uint32(len(dstData)),
			})
			continue
		}
		if err := sp.Init(item); err != nil {
			logger.Panicf("FATAL: cannot parse row during merge: %s", err)
		}
		if sp.StreamIDsLen() >= maxStreamIDsPerRow {
			dstData, dstItems = tsm.flushPendingStreamIDs(dstData, dstItems, spPrev)
			dstData = append(dstData, item...)
			dstItems = append(dstItems, mergeset.Item{
				Start: uint32(len(dstData) - len(item)),
				End:   uint32(len(dstData)),
			})
			continue
		}
		if !sp.EqualPrefix(spPrev) {
			dstData, dstItems = tsm.flushPendingStreamIDs(dstData, dstItems, spPrev)
		}
		sp.ParseStreamIDs()
		tsm.pendingStreamIDs = append(tsm.pendingStreamIDs, sp.StreamIDs...)
		spPrev, sp = sp, spPrev
		if len(tsm.pendingStreamIDs) >= maxStreamIDsPerRow {
			dstData, dstItems = tsm.flushPendingStreamIDs(dstData, dstItems, spPrev)
		}
	}
	if len(tsm.pendingStreamIDs) > 0 {
		logger.Panicf("BUG: tsm.pendingStreamIDs must be empty at this point; got %d items", len(tsm.pendingStreamIDs))
	}
	if !checkItemsSorted(dstData, dstItems) {
		// Items could become unsorted if initial items contain duplicate streamIDs:
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
		dstData = append(dstData[:0], tsm.dataCopy...)
		dstItems = append(dstItems[:0], tsm.itemsCopy...)
		if !checkItemsSorted(dstData, dstItems) {
			logger.Panicf("BUG: the original items weren't sorted; items=%q", dstItems)
		}
	}
	putTagToStreamIDsRowsMerger(tsm)
	return dstData, dstItems
}

// maxStreamIDsPerRow limits the number of streamIDs in tenantID:name:value -> streamIDs row.
//
// This reduces overhead on index and metaindex in lib/mergeset.
const maxStreamIDsPerRow = 32

type u128Sorter []u128

func (s u128Sorter) Len() int { return len(s) }
func (s u128Sorter) Less(i, j int) bool {
	return s[i].less(&s[j])
}
func (s u128Sorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type tagToStreamIDsRowsMerger struct {
	pendingStreamIDs u128Sorter
	sp               tagToStreamIDsRowParser
	spPrev           tagToStreamIDsRowParser

	itemsCopy []mergeset.Item
	dataCopy  []byte
}

func (tsm *tagToStreamIDsRowsMerger) Reset() {
	tsm.pendingStreamIDs = tsm.pendingStreamIDs[:0]
	tsm.sp.Reset()
	tsm.spPrev.Reset()

	tsm.itemsCopy = tsm.itemsCopy[:0]
	tsm.dataCopy = tsm.dataCopy[:0]
}

func (tsm *tagToStreamIDsRowsMerger) flushPendingStreamIDs(dstData []byte, dstItems []mergeset.Item, sp *tagToStreamIDsRowParser) ([]byte, []mergeset.Item) {
	if len(tsm.pendingStreamIDs) == 0 {
		// Nothing to flush
		return dstData, dstItems
	}
	// Use sort.Sort instead of sort.Slice in order to reduce memory allocations.
	sort.Sort(&tsm.pendingStreamIDs)
	tsm.pendingStreamIDs = removeDuplicateStreamIDs(tsm.pendingStreamIDs)

	// Marshal pendingStreamIDs
	dstDataLen := len(dstData)
	dstData = sp.MarshalPrefix(dstData)
	pendingStreamIDs := tsm.pendingStreamIDs
	for i := range pendingStreamIDs {
		dstData = pendingStreamIDs[i].marshal(dstData)
	}
	dstItems = append(dstItems, mergeset.Item{
		Start: uint32(dstDataLen),
		End:   uint32(len(dstData)),
	})
	tsm.pendingStreamIDs = tsm.pendingStreamIDs[:0]
	return dstData, dstItems
}

func removeDuplicateStreamIDs(sortedStreamIDs []u128) []u128 {
	if len(sortedStreamIDs) < 2 {
		return sortedStreamIDs
	}
	hasDuplicates := false
	for i := 1; i < len(sortedStreamIDs); i++ {
		if sortedStreamIDs[i-1] == sortedStreamIDs[i] {
			hasDuplicates = true
			break
		}
	}
	if !hasDuplicates {
		return sortedStreamIDs
	}
	dstStreamIDs := sortedStreamIDs[:1]
	for i := 1; i < len(sortedStreamIDs); i++ {
		if sortedStreamIDs[i-1] == sortedStreamIDs[i] {
			continue
		}
		dstStreamIDs = append(dstStreamIDs, sortedStreamIDs[i])
	}
	return dstStreamIDs
}

func getTagToStreamIDsRowsMerger() *tagToStreamIDsRowsMerger {
	v := tsmPool.Get()
	if v == nil {
		return &tagToStreamIDsRowsMerger{}
	}
	return v.(*tagToStreamIDsRowsMerger)
}

func putTagToStreamIDsRowsMerger(tsm *tagToStreamIDsRowsMerger) {
	tsm.Reset()
	tsmPool.Put(tsm)
}

var tsmPool sync.Pool

type tagToStreamIDsRowParser struct {
	// TenantID contains TenantID of the parsed row
	TenantID TenantID

	// StreamIDs contains parsed StreamIDs after ParseStreamIDs call
	StreamIDs []u128

	// streamIDsParsed is set to true after ParseStreamIDs call
	streamIDsParsed bool

	// Tag contains parsed tag after Init call
	Tag streamTag

	// tail contains the remaining unparsed streamIDs
	tail []byte
}

func (sp *tagToStreamIDsRowParser) Reset() {
	sp.TenantID.Reset()
	sp.StreamIDs = sp.StreamIDs[:0]
	sp.streamIDsParsed = false
	sp.Tag.reset()
	sp.tail = nil
}

// Init initializes sp from b, which should contain encoded tenantID:name:value -> streamIDs row.
//
// b cannot be reused until Reset call.
//
// ParseStreamIDs() must be called later for obtaining sp.StreamIDs from the given tail.
func (sp *tagToStreamIDsRowParser) Init(b []byte) error {
	tail, nsPrefix, err := unmarshalCommonPrefix(&sp.TenantID, b)
	if err != nil {
		return fmt.Errorf("invalid tenantID:name:value -> streamIDs row %q: %w", b, err)
	}
	if nsPrefix != nsPrefixTagToStreamIDs {
		return fmt.Errorf("invalid prefix for tenantID:name:value -> streamIDs row %q; got %d; want %d", b, nsPrefix, nsPrefixTagToStreamIDs)
	}
	tail, err = sp.Tag.indexdbUnmarshal(tail)
	if err != nil {
		return fmt.Errorf("cannot unmarshal tag from tenantID:name:value -> streamIDs row %q: %w", b, err)
	}
	if err = sp.InitOnlyTail(tail); err != nil {
		return fmt.Errorf("cannot initialize tail from tenantID:name:value -> streamIDs row %q: %w", b, err)
	}
	return nil
}

// MarshalPrefix marshals row prefix without tail to dst.
func (sp *tagToStreamIDsRowParser) MarshalPrefix(dst []byte) []byte {
	dst = marshalCommonPrefix(dst, nsPrefixTagToStreamIDs, sp.TenantID)
	dst = sp.Tag.indexdbMarshal(dst)
	return dst
}

// InitOnlyTail initializes sp.tail from tail, which must contain streamIDs.
//
// tail cannot be reused until Reset call.
//
// ParseStreamIDs() must be called later for obtaining sp.StreamIDs from the given tail.
func (sp *tagToStreamIDsRowParser) InitOnlyTail(tail []byte) error {
	if len(tail) == 0 {
		return fmt.Errorf("missing streamID in the tenantID:name:value -> streamIDs row")
	}
	if len(tail)%16 != 0 {
		return fmt.Errorf("invalid tail length in the tenantID:name:value -> streamIDs row; got %d bytes; must be multiple of 16 bytes", len(tail))
	}
	sp.tail = tail
	sp.streamIDsParsed = false
	return nil
}

// EqualPrefix returns true if prefixes for sp and x are equal.
//
// Prefix contains (tenantID:name:value)
func (sp *tagToStreamIDsRowParser) EqualPrefix(x *tagToStreamIDsRowParser) bool {
	if !sp.TenantID.equal(&x.TenantID) {
		return false
	}
	if !sp.Tag.equal(&x.Tag) {
		return false
	}
	return true
}

// StreamIDsLen returns the number of StreamIDs in the sp.tail
func (sp *tagToStreamIDsRowParser) StreamIDsLen() int {
	return len(sp.tail) / 16
}

// ParseStreamIDs parses StreamIDs from sp.tail into sp.StreamIDs.
func (sp *tagToStreamIDsRowParser) ParseStreamIDs() {
	if sp.streamIDsParsed {
		return
	}
	tail := sp.tail
	n := len(tail) / 16
	sp.StreamIDs = slicesutil.SetLength(sp.StreamIDs, n)
	streamIDs := sp.StreamIDs
	_ = streamIDs[n-1]
	for i := 0; i < n; i++ {
		var err error
		tail, err = streamIDs[i].unmarshal(tail)
		if err != nil {
			logger.Panicf("FATAL: cannot unmarshal streamID: %s", err)
		}
	}
	sp.streamIDsParsed = true
}

func (sp *tagToStreamIDsRowParser) UpdateStreamIDs(ids map[u128]struct{}, tail []byte) {
	sp.Reset()
	if err := sp.InitOnlyTail(tail); err != nil {
		logger.Panicf("FATAL: cannot parse '(date, tag) -> streamIDs' row: %s", err)
	}
	sp.ParseStreamIDs()
	for _, id := range sp.StreamIDs {
		ids[id] = struct{}{}
	}
}

// commonPrefixLen is the length of common prefix for indexdb rows
// 1 byte for ns* prefix + 8 bytes for tenantID
const commonPrefixLen = 1 + 8

func marshalCommonPrefix(dst []byte, nsPrefix byte, tenantID TenantID) []byte {
	dst = append(dst, nsPrefix)
	dst = tenantID.marshal(dst)
	return dst
}

func unmarshalCommonPrefix(dstTenantID *TenantID, src []byte) ([]byte, byte, error) {
	if len(src) < commonPrefixLen {
		return nil, 0, fmt.Errorf("cannot unmarshal common prefix from %d bytes; need at least %d bytes; data=%X", len(src), commonPrefixLen, src)
	}
	prefix := src[0]
	src = src[1:]
	tail, err := dstTenantID.unmarshal(src)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot unmarshal tenantID: %w", err)
	}
	return tail, prefix, nil
}

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
