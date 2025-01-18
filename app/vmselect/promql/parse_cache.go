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
	pc := newParseCache()
	metrics.NewGauge(`vm_cache_requests_total{type="promql/parse"}`, func() float64 {
		return float64(pc.requests())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/parse"}`, func() float64 {
		return float64(pc.misses())
	})
	metrics.NewGauge(`vm_cache_entries{type="promql/parse"}`, func() float64 {
		return float64(pc.len())
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

func newParseCache() *parseCache {
	pc := new(parseCache)
	for i := 0; i < parseBucketCount; i++ {
		pc.buckets[i] = newParseBucket()
	}
	return pc
}

func (pc *parseCache) put(q string, pcv *parseCacheValue) {
	h := xxhash.Sum64String(q)
	idx := h % parseBucketCount
	pc.buckets[idx].put(q, pcv)
}

func (pc *parseCache) get(q string) *parseCacheValue {
	h := xxhash.Sum64String(q)
	idx := h % parseBucketCount
	return pc.buckets[idx].get(q)
}

func (pc *parseCache) requests() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].requests.Load()
	}
	return n
}

func (pc *parseCache) misses() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].misses.Load()
	}
	return n
}

func (pc *parseCache) len() uint64 {
	var n uint64
	for i := 0; i < parseBucketCount; i++ {
		n += pc.buckets[i].len()
	}
	return n
}

func newParseBucket() parseBucket {
	return parseBucket{
		m: make(map[string]*parseCacheValue, parseBucketMaxLen),
	}
}

func (pb *parseBucket) len() uint64 {
	pb.mu.RLock()
	n := len(pb.m)
	pb.mu.RUnlock()
	return uint64(n)
}

func (pb *parseBucket) get(q string) *parseCacheValue {
	pb.requests.Add(1)

	pb.mu.RLock()
	pcv := pb.m[q]
	pb.mu.RUnlock()

	if pcv == nil {
		pb.misses.Add(1)
	}
	return pcv
}

func (pb *parseBucket) put(q string, pcv *parseCacheValue) {
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
