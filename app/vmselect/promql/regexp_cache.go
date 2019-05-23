package promql

import (
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

func compileRegexpAnchored(re string) (*regexp.Regexp, error) {
	reAnchored := "^(?:" + re + ")$"
	return compileRegexp(reAnchored)
}

func compileRegexp(re string) (*regexp.Regexp, error) {
	rcv := regexpCacheV.Get(re)
	if rcv != nil {
		return rcv.r, rcv.err
	}
	r, err := regexp.Compile(re)
	rcv = &regexpCacheValue{
		r:   r,
		err: err,
	}
	regexpCacheV.Put(re, rcv)
	return rcv.r, rcv.err
}

var regexpCacheV = func() *regexpCache {
	rc := &regexpCache{
		m: make(map[string]*regexpCacheValue),
	}
	metrics.NewGauge(`vm_cache_requests_total{type="promql/regexp"}`, func() float64 {
		return float64(rc.Requests())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/regexp"}`, func() float64 {
		return float64(rc.Misses())
	})
	metrics.NewGauge(`vm_cache_entries{type="promql/regexp"}`, func() float64 {
		return float64(rc.Len())
	})
	return rc
}()

const regexpCacheMaxLen = 10e3

type regexpCacheValue struct {
	r   *regexp.Regexp
	err error
}

type regexpCache struct {
	m  map[string]*regexpCacheValue
	mu sync.RWMutex

	requests uint64
	misses   uint64
}

func (rc *regexpCache) Requests() uint64 {
	return atomic.LoadUint64(&rc.requests)
}

func (rc *regexpCache) Misses() uint64 {
	return atomic.LoadUint64(&rc.misses)
}

func (rc *regexpCache) Len() uint64 {
	rc.mu.RLock()
	n := len(rc.m)
	rc.mu.RUnlock()
	return uint64(n)
}

func (rc *regexpCache) Get(regexp string) *regexpCacheValue {
	atomic.AddUint64(&rc.requests, 1)

	rc.mu.RLock()
	rcv := rc.m[regexp]
	rc.mu.RUnlock()

	if rc == nil {
		atomic.AddUint64(&rc.misses, 1)
	}
	return rcv
}

func (rc *regexpCache) Put(regexp string, rcv *regexpCacheValue) {
	rc.mu.Lock()
	overflow := len(rc.m) - regexpCacheMaxLen
	if overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(rc.m)) * 0.1)
		for k := range rc.m {
			delete(rc.m, k)
			overflow--
			if overflow <= 0 {
				break
			}
		}
	}
	rc.m[regexp] = rcv
	rc.mu.Unlock()
}
