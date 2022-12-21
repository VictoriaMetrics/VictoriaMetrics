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
	lastCleanupTime uint64

	m sync.Map

	transformFunc func(s string) string
}

type fstEntry struct {
	lastAccessTime uint64
	s              string
}

// NewFastStringTransformer creates new transformer, which applies transformFunc to strings passed to Transform()
//
// transformFunc must return the same result for the same input.
func NewFastStringTransformer(transformFunc func(s string) string) *FastStringTransformer {
	return &FastStringTransformer{
		lastCleanupTime: fasttime.UnixTimestamp(),
		transformFunc:   transformFunc,
	}
}

// Transform applies transformFunc to s and returns the result.
func (fst *FastStringTransformer) Transform(s string) string {
	ct := fasttime.UnixTimestamp()
	v, ok := fst.m.Load(s)
	if ok {
		// Fast path - the transformed s is found in the cache.
		e := v.(*fstEntry)
		if atomic.LoadUint64(&e.lastAccessTime)+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			atomic.StoreUint64(&e.lastAccessTime, ct)
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
		lastAccessTime: ct,
		s:              sTransformed,
	}
	fst.m.Store(s, e)

	if needCleanup(&fst.lastCleanupTime, ct) {
		// Perform a global cleanup for fst.m by removing items, which weren't accessed during the last 5 minutes.
		m := &fst.m
		m.Range(func(k, v interface{}) bool {
			e := v.(*fstEntry)
			if atomic.LoadUint64(&e.lastAccessTime)+5*60 < ct {
				m.Delete(k)
			}
			return true
		})
	}

	return sTransformed
}
