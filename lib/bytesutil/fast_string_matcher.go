package bytesutil

import (
	"strings"
	"sync"
	"sync/atomic"
)

// FastStringMatcher implements fast matcher for strings.
//
// It caches string match results and returns them back on the next calls
// without calling the matchFunc, which may be expensive.
type FastStringMatcher struct {
	m    atomic.Value
	mLen uint64

	matchFunc func(s string) bool
}

// NewFastStringMatcher creates new matcher, which applies matchFunc to strings passed to Match()
//
// matchFunc must return the same result for the same input.
func NewFastStringMatcher(matchFunc func(s string) bool) *FastStringMatcher {
	var fsm FastStringMatcher
	fsm.m.Store(&sync.Map{})
	fsm.matchFunc = matchFunc
	return &fsm
}

// Match applies matchFunc to s and returns the result.
func (fsm *FastStringMatcher) Match(s string) bool {
	m := fsm.m.Load().(*sync.Map)
	v, ok := m.Load(s)
	if ok {
		// Fast path - s match result is found in the cache.
		bp := v.(*bool)
		return *bp
	}
	// Slow path - run matchFunc for s and store the result in the cache.
	b := fsm.matchFunc(s)
	bp := &b
	// Make a copy of s in order to limit memory usage to the s length,
	// since the s may point to bigger string.
	// This also protects from the case when s contains unsafe string, which points to a temporary byte slice.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
	s = strings.Clone(s)
	m.Store(s, bp)
	n := atomic.AddUint64(&fsm.mLen, 1)
	if n > 100e3 {
		atomic.StoreUint64(&fsm.mLen, 0)
		fsm.m.Store(&sync.Map{})
	}
	return b
}
