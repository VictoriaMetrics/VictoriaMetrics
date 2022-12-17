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
	lastCleanupTime uint64

	m sync.Map

	matchFunc func(s string) bool
}

type fsmEntry struct {
	lastAccessTime uint64
	ok             bool
}

// NewFastStringMatcher creates new matcher, which applies matchFunc to strings passed to Match()
//
// matchFunc must return the same result for the same input.
func NewFastStringMatcher(matchFunc func(s string) bool) *FastStringMatcher {
	return &FastStringMatcher{
		lastCleanupTime: fasttime.UnixTimestamp(),
		matchFunc:       matchFunc,
	}
}

// Match applies matchFunc to s and returns the result.
func (fsm *FastStringMatcher) Match(s string) bool {
	ct := fasttime.UnixTimestamp()
	v, ok := fsm.m.Load(s)
	if ok {
		// Fast path - s match result is found in the cache.
		e := v.(*fsmEntry)
		if atomic.LoadUint64(&e.lastAccessTime)+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			atomic.StoreUint64(&e.lastAccessTime, ct)
		}
		return e.ok
	}
	// Slow path - run matchFunc for s and store the result in the cache.
	b := fsm.matchFunc(s)
	e := &fsmEntry{
		lastAccessTime: ct,
		ok:             b,
	}
	// Make a copy of s in order to limit memory usage to the s length,
	// since the s may point to bigger string.
	// This also protects from the case when s contains unsafe string, which points to a temporary byte slice.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
	s = strings.Clone(s)
	fsm.m.Store(s, e)

	if atomic.LoadUint64(&fsm.lastCleanupTime)+61 < ct {
		// Perform a global cleanup for fsm.m by removing items, which weren't accessed
		// during the last 5 minutes.
		atomic.StoreUint64(&fsm.lastCleanupTime, ct)
		m := &fsm.m
		m.Range(func(k, v interface{}) bool {
			e := v.(*fsmEntry)
			if atomic.LoadUint64(&e.lastAccessTime)+5*60 < ct {
				m.Delete(k)
			}
			return true
		})
	}

	return b
}
