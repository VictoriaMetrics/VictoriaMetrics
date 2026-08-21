package roaring

import "math/bits"

func popcntSliceGo(s []uint64) uint64 {
	cnt := uint64(0)
	for _, x := range s {
		cnt += uint64(bits.OnesCount64(x))
	}
	return cnt
}

func popcntMaskSliceGo(s, m []uint64) uint64 {
	cnt := uint64(0)
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] &^ m[i]))
	}
	return cnt
}

func popcntAndSliceGo(s, m []uint64) uint64 {
	cnt := uint64(0)
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] & m[i]))
	}
	return cnt
}

func popcntOrSliceGo(s, m []uint64) uint64 {
	cnt := uint64(0)
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] | m[i]))
	}
	return cnt
}

func popcntXorSliceGo(s, m []uint64) uint64 {
	cnt := uint64(0)
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] ^ m[i]))
	}
	return cnt
}
