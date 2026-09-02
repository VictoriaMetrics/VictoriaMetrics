//go:build arm64 && !appengine
// +build arm64,!appengine

package roaring

// The functions below are implemented in popcnt_neon_arm64.s using NEON
// (Advanced SIMD). NEON is mandatory in the ARMv8-A baseline that every arm64
// CPU implements, so — unlike the amd64 AVX2 path, which is gated on a runtime
// _hasAVX2 check — these routines are always used on arm64. The pure-Go
// fallbacks in popcnt_slices.go remain in use on other architectures and on
// appengine builds, which compile popcnt_generic.go instead.

//go:noescape
func _popcntSliceNEON(s []uint64) uint64

//go:noescape
func _popcntMaskSliceNEON(s, m []uint64) uint64

//go:noescape
func _popcntAndSliceNEON(s, m []uint64) uint64

//go:noescape
func _popcntOrSliceNEON(s, m []uint64) uint64

//go:noescape
func _popcntXorSliceNEON(s, m []uint64) uint64

// useNEON is always true on arm64; it exists so tests can force the scalar
// fallback path and to mirror the amd64 dispatch structure.
var useNEON = true

func popcntSlice(s []uint64) uint64 {
	if useNEON {
		return _popcntSliceNEON(s)
	}
	return popcntSliceGo(s)
}

func popcntMaskSlice(s, m []uint64) uint64 {
	if useNEON {
		return _popcntMaskSliceNEON(s, m)
	}
	return popcntMaskSliceGo(s, m)
}

func popcntAndSlice(s, m []uint64) uint64 {
	if useNEON {
		return _popcntAndSliceNEON(s, m)
	}
	return popcntAndSliceGo(s, m)
}

func popcntOrSlice(s, m []uint64) uint64 {
	if useNEON {
		return _popcntOrSliceNEON(s, m)
	}
	return popcntOrSliceGo(s, m)
}

func popcntXorSlice(s, m []uint64) uint64 {
	if useNEON {
		return _popcntXorSliceNEON(s, m)
	}
	return popcntXorSliceGo(s, m)
}
