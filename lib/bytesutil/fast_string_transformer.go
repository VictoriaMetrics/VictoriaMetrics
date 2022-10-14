package bytesutil

import (
	"strings"
	"sync"
	"sync/atomic"
)

// FastStringTransformer implements fast transformer for strings.
//
// It caches transformed strings and returns them back on the next calls
// without calling the transformFunc, which may be expensive.
type FastStringTransformer struct {
	m    atomic.Value
	mLen uint64

	transformFunc func(s string) string
}

// NewFastStringTransformer creates new transformer, which applies transformFunc to strings passed to Transform()
//
// transformFunc must return the same result for the same input.
func NewFastStringTransformer(transformFunc func(s string) string) *FastStringTransformer {
	var fst FastStringTransformer
	fst.m.Store(&sync.Map{})
	fst.transformFunc = transformFunc
	return &fst
}

// Transform applies transformFunc to s and returns the result.
func (fst *FastStringTransformer) Transform(s string) string {
	m := fst.m.Load().(*sync.Map)
	v, ok := m.Load(s)
	if ok {
		// Fast path - the transformed s is found in the cache.
		sp := v.(*string)
		return *sp
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
	sp := &sTransformed
	m.Store(s, sp)
	n := atomic.AddUint64(&fst.mLen, 1)
	if n > 100e3 {
		atomic.StoreUint64(&fst.mLen, 0)
		fst.m.Store(&sync.Map{})
	}
	return sTransformed
}
