package bytesutil

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// FastStringTransformer implements fast transformer for strings.
//
// It caches transformed strings and returns them back on the next calls
// without calling the transformFunc, which may be expensive.
type FastStringTransformer struct {
	lastCleanupTime atomic.Uint64

	m sync.Map

	transformFunc func(s string) string
}

type fstEntry struct {
	lastAccessTime atomic.Uint64
	s              string
}

// NewFastStringTransformer creates new transformer, which applies transformFunc to strings passed to Transform()
//
// transformFunc must return the same result for the same input.
func NewFastStringTransformer(transformFunc func(s string) string) *FastStringTransformer {
	fst := &FastStringTransformer{
		transformFunc: transformFunc,
	}
	fst.lastCleanupTime.Store(fasttime.UnixTimestamp())
	return fst
}

// Transform applies transformFunc to s and returns the result.
func (fst *FastStringTransformer) Transform(s string) string {
	if isSkipCache(s) {
		sTransformed := fst.transformFunc(s)
		if sTransformed == s {
			// Clone a string in order to protect from cases when s contains unsafe string.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
			sTransformed = strings.Clone(sTransformed)
		}
		return sTransformed
	}

	ct := fasttime.UnixTimestamp()
	v, ok := fst.m.Load(s)
	if ok {
		// Fast path - the transformed s is found in the cache.
		e := v.(*fstEntry)
		if e.lastAccessTime.Load()+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			e.lastAccessTime.Store(ct)
		}
		return e.s
	}
	// Slow path - transform s and store it in the cache.
	sTransformed := fst.transformFunc(s)
	// Make a copy of s in order to limit memory usage to the s length,
	// since the s may point to bigger string.
	// This also protects from the case when s contains unsafe string, which points to a temporary byte slice.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
	s = strings.Clone(s)
	if sTransformed == s {
		// point sTransformed to just allocated s, since it may point to s,
		// which, in turn, can point to bigger string.
		sTransformed = s
	}
	e := &fstEntry{
		s: sTransformed,
	}
	e.lastAccessTime.Store(ct)
	fst.m.Store(s, e)

	if needCleanup(&fst.lastCleanupTime, ct) {
		// Perform a global cleanup for fst.m by removing items, which weren't accessed during the last 5 minutes.
		m := &fst.m
		deadline := ct - uint64(cacheExpireDuration.Seconds())
		m.Range(func(k, v any) bool {
			e := v.(*fstEntry)
			if e.lastAccessTime.Load() < deadline {
				m.Delete(k)
			}
			return true
		})
	}

	return sTransformed
}
