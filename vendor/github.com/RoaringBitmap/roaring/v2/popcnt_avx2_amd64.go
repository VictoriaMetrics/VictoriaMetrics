//go:build amd64 && !appengine
// +build amd64,!appengine

package roaring

// The functions below are implemented in popcnt_avx2_amd64.s using AVX2.
// They are only used when the CPU supports AVX2 (see useAVX2); otherwise the
// pure-Go fallbacks in popcnt_slices.go are used. This keeps behavior identical
// on every target: appengine and non-amd64 builds compile popcnt_generic.go
// instead, and amd64 CPUs without AVX2 take the scalar path at runtime.

//go:noescape
func _hasAVX2() bool

//go:noescape
func _popcntSliceAVX2(s []uint64) uint64

//go:noescape
func _popcntMaskSliceAVX2(s, m []uint64) uint64

//go:noescape
func _popcntAndSliceAVX2(s, m []uint64) uint64

//go:noescape
func _popcntOrSliceAVX2(s, m []uint64) uint64

//go:noescape
func _popcntXorSliceAVX2(s, m []uint64) uint64

// useAVX2 selects the AVX2 assembly implementations when the running CPU
// supports AVX2. It is evaluated once at package initialization.
var useAVX2 = _hasAVX2()

func popcntSlice(s []uint64) uint64 {
	if useAVX2 {
		return _popcntSliceAVX2(s)
	}
	return popcntSliceGo(s)
}

func popcntMaskSlice(s, m []uint64) uint64 {
	if useAVX2 {
		return _popcntMaskSliceAVX2(s, m)
	}
	return popcntMaskSliceGo(s, m)
}

func popcntAndSlice(s, m []uint64) uint64 {
	if useAVX2 {
		return _popcntAndSliceAVX2(s, m)
	}
	return popcntAndSliceGo(s, m)
}

func popcntOrSlice(s, m []uint64) uint64 {
	if useAVX2 {
		return _popcntOrSliceAVX2(s, m)
	}
	return popcntOrSliceGo(s, m)
}

func popcntXorSlice(s, m []uint64) uint64 {
	if useAVX2 {
		return _popcntXorSliceAVX2(s, m)
	}
	return popcntXorSliceGo(s, m)
}
