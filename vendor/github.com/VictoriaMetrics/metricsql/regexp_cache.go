package metricsql

import (
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// CompileRegexpAnchored returns compiled regexp `^re$`.
func CompileRegexpAnchored(re string) (*regexp.Regexp, error) {
	reAnchored := "^(?:" + re + ")$"
	return CompileRegexp(reAnchored)
}

// CompileRegexp returns compile regexp re.
func CompileRegexp(re string) (*regexp.Regexp, error) {
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

// regexpCacheCharsMax limits the max number of chars stored in regexp cache across all entries.
//
// We limit by number of chars since calculating the exact size of each regexp is problematic,
// while using chars seems like universal approach for short and long regexps.
const regexpCacheCharsMax = 1e6

var regexpCacheV = func() *regexpCache {
	rc := newRegexpCache(regexpCacheCharsMax)
	metrics.NewGauge(`vm_cache_requests_total{type="promql/regexp"}`, func() float64 {
		return float64(rc.Requests())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/regexp"}`, func() float64 {
		return float64(rc.Misses())
	})
	metrics.NewGauge(`vm_cache_entries{type="promql/regexp"}`, func() float64 {
		return float64(rc.Len())
	})
	metrics.NewGauge(`vm_cache_chars_current{type="promql/regexp"}`, func() float64 {
		return float64(rc.CharsCurrent())
	})
	metrics.NewGauge(`vm_cache_chars_max{type="promql/regexp"}`, func() float64 {
		return float64(rc.charsLimit)
	})
	return rc
}()

type regexpCacheValue struct {
	r   *regexp.Regexp
	err error
}

type regexpCache struct {
	// Move atomic counters to the top of struct for 8-byte alignment on 32-bit arch.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	requests uint64
	misses   uint64

	// charsCurrent stores the total number of characters used in stored regexps.
	// is used for memory usage estimation.
	charsCurrent int

	// charsLimit is the maximum number of chars the regexpCache can store.
	charsLimit int

	m  map[string]*regexpCacheValue
	mu sync.RWMutex
}

func newRegexpCache(charsLimit int) *regexpCache {
	return &regexpCache{
		m:          make(map[string]*regexpCacheValue),
		charsLimit: charsLimit,
	}
}

func (rc *regexpCache) Requests() uint64 {
	return atomic.LoadUint64(&rc.requests)
}

func (rc *regexpCache) Misses() uint64 {
	return atomic.LoadUint64(&rc.misses)
}

func (rc *regexpCache) Len() int {
	rc.mu.RLock()
	n := len(rc.m)
	rc.mu.RUnlock()
	return n
}

func (rc *regexpCache) CharsCurrent() int {
	rc.mu.RLock()
	n := rc.charsCurrent
	rc.mu.RUnlock()
	return int(n)
}

func (rc *regexpCache) Get(regexp string) *regexpCacheValue {
	atomic.AddUint64(&rc.requests, 1)

	rc.mu.RLock()
	rcv := rc.m[regexp]
	rc.mu.RUnlock()

	if rcv == nil {
		atomic.AddUint64(&rc.misses, 1)
	}
	return rcv
}

func (rc *regexpCache) Put(regexp string, rcv *regexpCacheValue) {
	rc.mu.Lock()
	if rc.charsCurrent > rc.charsLimit {
		// Remove items accounting for 10% chars from the cache.
		overflow := int(float64(rc.charsLimit) * 0.1)
		for k := range rc.m {
			delete(rc.m, k)

			size := len(k)
			overflow -= size
			rc.charsCurrent -= size

			if overflow <= 0 {
				break
			}
		}
	}
	rc.m[regexp] = rcv
	rc.charsCurrent += len(regexp)
	rc.mu.Unlock()
}
