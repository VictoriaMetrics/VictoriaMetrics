package bytesutil

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// FastStringMatcher implements fast matcher for strings.
//
// It caches string match results and returns them back on the next calls
// without calling the matchFunc, which may be expensive.
type FastStringMatcher struct {
	lastCleanupTime atomic.Uint64

	m sync.Map

	matchFunc func(s string) bool
}

type fsmEntry struct {
	lastAccessTime atomic.Uint64
	ok             bool
}

// NewFastStringMatcher creates new matcher, which applies matchFunc to strings passed to Match()
//
// matchFunc must return the same result for the same input.
func NewFastStringMatcher(matchFunc func(s string) bool) *FastStringMatcher {
	fsm := &FastStringMatcher{
		matchFunc: matchFunc,
	}
	fsm.lastCleanupTime.Store(fasttime.UnixTimestamp())
	return fsm
}

// Match applies matchFunc to s and returns the result.
func (fsm *FastStringMatcher) Match(s string) bool {
	if isSkipCache(s) {
		return fsm.matchFunc(s)
	}

	ct := fasttime.UnixTimestamp()
	v, ok := fsm.m.Load(s)
	if ok {
		// Fast path - s match result is found in the cache.
		e := v.(*fsmEntry)
		if e.lastAccessTime.Load()+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			e.lastAccessTime.Store(ct)
		}
		return e.ok
	}
	// Slow path - run matchFunc for s and store the result in the cache.
	b := fsm.matchFunc(s)
	e := &fsmEntry{
		ok: b,
	}
	e.lastAccessTime.Store(ct)
	// Make a copy of s in order to limit memory usage to the s length,
	// since the s may point to bigger string.
	// This also protects from the case when s contains unsafe string, which points to a temporary byte slice.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
	s = strings.Clone(s)
	fsm.m.Store(s, e)

	if needCleanup(&fsm.lastCleanupTime, ct) {
		// Perform a global cleanup for fsm.m by removing items, which weren't accessed during the last 5 minutes.
		m := &fsm.m
		deadline := ct - uint64(cacheExpireDuration.Seconds())
		m.Range(func(k, v any) bool {
			e := v.(*fsmEntry)
			if e.lastAccessTime.Load() < deadline {
				m.Delete(k)
			}
			return true
		})
	}

	return b
}

func needCleanup(lastCleanupTime *atomic.Uint64, currentTime uint64) bool {
	lct := lastCleanupTime.Load()
	if lct+61 >= currentTime {
		return false
	}
	// Atomically compare and swap the current time with the lastCleanupTime
	// in order to guarantee that only a single goroutine out of multiple
	// concurrently executing goroutines gets true from the call.
	return lastCleanupTime.CompareAndSwap(lct, currentTime)
}
