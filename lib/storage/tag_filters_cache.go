package storage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

var tagFiltersKeyBufPool bytesutil.ByteBufferPool

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

type tagFiltersCacheStats struct {
	entriesCount uint64
	bytesSize    uint64
	maxBytesSize uint64
	getCalls     uint64
	misses       uint64
}

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

func (c *tagFiltersCache) set(qt *querytracer.Tracer, metricIDs *uint64set.Set, key []byte) {
	qt = qt.NewChild("store %d metricIDs to cache", metricIDs.Len())
	defer qt.Done()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Need to check if the entry is present to calculate the total bytesSize
	// correctly.
	if _, ok := c.m[string(key)]; ok {
		return
	}

	// Do not add new entry if the total bytesSize will exceed maxBytesSize.
	if c.s.bytesSize+uint64(len(key))+metricIDs.SizeBytes() > c.s.maxBytesSize {
		qt.Printf("could not store: cache is full")
		return
	}

	keyCopy := make([]byte, len(key))
	if n := copy(keyCopy, key); n != len(key) {
		logger.Fatalf("BUG: unexpected number of copied bytes: got %d, want %d", n, len(key))
	}

	c.m[string(keyCopy)] = metricIDs.Clone()
	c.s.entriesCount++

	qt.Printf("stored %d metricIDs to cache", metricIDs.Len())
}

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
	}
	qt.Printf("found %d metricIDs", metricIDs.Len())
	return metricIDs.Clone(), ok
}

func (c *tagFiltersCache) stats() tagFiltersCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.s
}

func (c *tagFiltersCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]*uint64set.Set)
	c.s = tagFiltersCacheStats{
		maxBytesSize: c.s.maxBytesSize,
	}
}
