package storage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

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

// tagFiltersKeyBufPool holds the byte slices that should be used for storing
// the result of converting a (tfss, tr) pair into a byte slice. This is to
// reduce allocations.
var tagFiltersKeyBufPool bytesutil.ByteBufferPool

// marshalTagFiltersKey converts a (tfss, tr) pair into a byte slice so that it
// could be put into cache.
//
// (tfss, tr) pair cannot be used as a key because tfss contains slices. Thus,
// the pair needs to be converted into something that could be used as a map
// key. A slice of bytes is chosen since it can be converted to a string. When
// used in maps, the conversion does not actually create a new string, so this
// conversion is efficient.
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

// tagFiltersCacheStats holds various cache stats.
type tagFiltersCacheStats struct {
	entriesCount uint64
	bytesSize    uint64
	maxBytesSize uint64
	getCalls     uint64
	misses       uint64
	resets       uint64
}

// tagFiltersCache stores the result of searching indexDB for metricIDs by
// tagFilters and timeRange, i.e. (tfss, tr) -> metricIDs entries.
//
// It is used for improving the performance of data and metric name queries,
// currently handled by /api/v1/query, /api/v1/query_range, and /api/v1/series
// endpoints.
//
// The cache is limited by size. By default it is 1/32nd of the memory allowed
// for caches (which is 60% of the total system memory by default). But the size
// of the cache can be overriden with SetTagFiltersCacheSize(). If a new entry
// is added to the cache and there is no room for that entry, the cache will
// randomly delete its entries until at least 10% of cache space is available.
//
// The cache is expected to be reset everytime a new timeseries is added to the
// database. This is a quite frequent event in a typical prod environment and
// therefore the cache implementation is very simple, compared to other
// important caches, such as tsidCache. Namely, 1) it is completely in-memory
// (i.e. it is never persisted to disk), 2) it has no rotation, it is just a map
// whose reads and writes are guarded by a mutex.
//
// In order to improve performance, the cache does not store the copies of
// values, instead it stores the the pointer to the value. This means that if
// the caller stores or retrieves a value to/from the cache (by calling cache
// set() or get() method) the caller must not modify that value. Additionally,
// the caller must stop holding the value as soon as possible, so that the Go
// garbage collector could free the space when the cache is reset or is forced
// to free up space (see set() method). Note that the cache does copy the keys
// but only because it is implemented on top of the Go map that can only accept
// strings as a key.
type tagFiltersCache struct {
	m  map[string]*uint64set.Set
	s  tagFiltersCacheStats
	mu sync.Mutex
}

func newTagFiltersCache() *tagFiltersCache {
	return &tagFiltersCache{
		m: make(map[string]*uint64set.Set),
		s: tagFiltersCacheStats{
			maxBytesSize: uint64(getTagFiltersCacheSize()),
		},
	}
}

// set adds a new entry to the cache optionally freeing up space if there is no
// room. If the entry with the same key already exists, the entry value will be
// overwritten and the cache by bytesSize metric will be adjusted accordingly.
// The method copies the key but not the value, and, after calling this method,
// the callers are 1) not expected to modify the value and 2) stop referencing
// it as soon as possible.
//
// See the description of the tagFiltersCache type for details.
func (c *tagFiltersCache) set(qt *querytracer.Tracer, value *uint64set.Set, key []byte) {
	qt = qt.NewChild("store %d metricIDs to cache", value.Len())
	defer qt.Done()

	keyCopy := string(key)

	c.mu.Lock()
	defer c.mu.Unlock()

	// In case there is no room for the new entry, make space by deleting random
	// cache entries until 10% of space is freed. Note that the space won't
	// really be freed until the calling code holds the pointer to the values
	// which have been removed from the cache during the cleanup.
	//
	// Ideally, when calculating the bitesSize, we would need to take into
	// account the case when the entry with this key is already in the cache
	// (see below). Then, the bytesSize would be precise and often much smaller.
	// However, we would then need to ensure that the key is not deleted from
	// the cache in case if the cache clean up still needs to be performed. I.e.
	// we would need to compare the random key with keyCopy which is expensive.
	//
	// Instead, we assume that adding an entry with the same key is rare.
	bytesSize := uint64(len(keyCopy)) + value.SizeBytes()
	if c.s.bytesSize+bytesSize > c.s.maxBytesSize {
		bytesToRemove := c.s.maxBytesSize / 10
		for k, v := range c.m {
			c.s.bytesSize -= uint64(len(k)) + v.SizeBytes()
			delete(c.m, k)
			c.s.entriesCount--
			if c.s.maxBytesSize-c.s.bytesSize >= bytesToRemove {
				break
			}
		}
	}

	// Adjust the entry count and bytesSize to account for cases when the entry
	// with this key already exists. It's ok for resulting bytesSize to be
	// negative because the new value size may smaller than the current value
	// size.
	entriesCount := uint64(1)
	if valuePrev, entryExists := c.m[keyCopy]; entryExists {
		bytesSize -= uint64(len(keyCopy)) + valuePrev.SizeBytes()
		entriesCount = 0
	}

	c.m[string(keyCopy)] = value
	c.s.entriesCount += entriesCount
	c.s.bytesSize += bytesSize

	qt.Printf("stored %d metricIDs to cache", value.Len())
}

// get retrieves the value by key. The returned value is not a copy but the
// actual value, and after calling this method the callers are 1) not expected
// to modify the value and 2) expected to stop referencing the value as soon as
// possible.
//
// See the description of the tagFiltersCache type for details.
func (c *tagFiltersCache) get(qt *querytracer.Tracer, key []byte) (*uint64set.Set, bool) {
	qt = qt.NewChild("search metricIDs from cache")
	defer qt.Done()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.s.getCalls++
	metricIDs, ok := c.m[string(key)]
	if !ok {
		qt.Printf("cache miss")
		c.s.misses++
		return nil, false
	}
	qt.Printf("found %d metricIDs", metricIDs.Len())
	return metricIDs, true
}

func (c *tagFiltersCache) stats() tagFiltersCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.s
}

// reset removes all entries from the cache and resets the stats (except
// `maxByteSize`, which stays the same, and `resets`, which is incremented by
// 1). Note that the entries are not removed from the memory right away but
// after some time and by the Go garbage collector. And in order for this to
// happen, the callers that stored and/or retrieved the values from the cache
// must not hold the pointer to the cache values.
//
// See the description of the tagFiltersCache type for details.
func (c *tagFiltersCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]*uint64set.Set)
	c.s = tagFiltersCacheStats{
		maxBytesSize: c.s.maxBytesSize,
		resets:       c.s.resets + 1,
	}
}
