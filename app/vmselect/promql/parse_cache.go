// Cache for metricsql expressions
// Based on the fastcache idea of locking buckets in order to avoid whole cache locks.
// See: https://github.com/VictoriaMetrics/fastcache
package promql

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"

	xxhash "github.com/cespare/xxhash/v2"
)

var parseCacheV = func() *parseCache {
	pc := NewParseCache()
	metrics.NewGauge(`vm_cache_requests_total{type="promql/parse"}`, func() float64 {
		return float64(pc.Requests())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/parse"}`, func() float64 {
		return float64(pc.Misses())
	})
	metrics.NewGauge(`vm_cache_entries{type="promql/parse"}`, func() float64 {
		return float64(pc.Len())
	})
	return pc
}()

const (
	parseBucketCount = 128

	parseCacheMaxLen int = 10e3

	parseBucketMaxLen int = parseCacheMaxLen / parseBucketCount

	parseBucketFreePercent float64 = 0.1
)

type parseCacheValue struct {
	e   metricsql.Expr
	err error
}

type parseBucket struct {
	m        map[string]*parseCacheValue
	mu       sync.RWMutex
	requests atomic.Uint64
	misses   atomic.Uint64
}

type parseCache struct {
	buckets [parseBucketCount]parseBucket
}

func NewParseCache() *parseCache {
	pc := new(parseCache)
	for i := 0; i < parseBucketCount; i++ {
		pc.buckets[i] = newParseBucket()
	}
	return pc
}

func (pc *parseCache) Put(q string, pcv *parseCacheValue) {
	h := xxhash.Sum64String(q)
	idx := h % parseBucketCount
	pc.buckets[idx].Put(q, pcv)
}

func (pc *parseCache) Get(q string) *parseCacheValue {
	h := xxhash.Sum64String(q)
	idx := h % parseBucketCount
	return pc.buckets[idx].Get(q)
}

func (pc *parseCache) Requests() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].Requests()
	}
	return n
}

func (pc *parseCache) Misses() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].Misses()
	}
	return n
}

func (pc *parseCache) Len() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].Len()
	}
	return n
}

func newParseBucket() parseBucket {
	return parseBucket{
		m: make(map[string]*parseCacheValue, parseBucketMaxLen),
	}
}

func (pb *parseBucket) Requests() uint64 {
	return pb.requests.Load()
}

func (pb *parseBucket) Misses() uint64 {
	return pb.misses.Load()
}

func (pb *parseBucket) Len() uint64 {
	pb.mu.RLock()
	n := len(pb.m)
	pb.mu.RUnlock()
	return uint64(n)
}

func (pb *parseBucket) Get(q string) *parseCacheValue {
	pb.requests.Add(1)

	pb.mu.RLock()
	pcv := pb.m[q]
	pb.mu.RUnlock()

	if pcv == nil {
		pb.misses.Add(1)
	}
	return pcv
}

func (pb *parseBucket) Put(q string, pcv *parseCacheValue) {
	pb.mu.Lock()
	overflow := len(pb.m) - parseBucketMaxLen
	if overflow > 0 {
		// Remove parseBucketDeletePercent*100 % of items from the bucket.
		overflow = int(float64(len(pb.m)) * parseBucketFreePercent)
		for k := range pb.m {
			delete(pb.m, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}
	pb.m[q] = pcv
	pb.mu.Unlock()
}
