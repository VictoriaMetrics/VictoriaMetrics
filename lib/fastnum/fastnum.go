package fastnum

import (
	"bytes"
	"math"
	"slices"
	"unsafe"
)

const (
	seedElems  = 1 << 10 // 8 KiB; must stay L1-resident
	smallElems = 16
)

// AppendInt64Zeros appends items zeros to dst and returns the result.
//
// It is faster than the corresponding loop.
func AppendInt64Zeros(dst []int64, items int) []int64 {
	return appendInt64Data(dst, items, int64Zeros[:])
}

// AppendInt64Ones appends items ones to dst and returns the result.
//
// It is faster than the corresponding loop.
func AppendInt64Ones(dst []int64, items int) []int64 {
	return appendOnesData(dst, items, int64Ones[:])
}

// AppendFloat64Zeros appends items zeros to dst and returns the result.
//
// It is faster than the corresponding loop.
func AppendFloat64Zeros(dst []float64, items int) []float64 {
	return appendFloat64Data(dst, items, float64Zeros[:])
}

// AppendFloat64Ones appends items ones to dst and returns the result.
//
// It is faster than the corresponding loop.
func AppendFloat64Ones(dst []float64, items int) []float64 {
	if items > 0 && items <= smallElems {
		return append(dst, float64Ones[:items]...)
	}
	return appendOnesData(dst, items, float64Ones[:])
}

func appendInt64Data(dst []int64, items int, src []int64) []int64 {
	for items > 0 {
		n := min(len(src), items)
		dst = append(dst, src[:n]...)
		items -= n
	}
	return dst
}

func appendFloat64Data(dst []float64, items int, src []float64) []float64 {
	for items > 0 {
		n := min(len(src), items)
		dst = append(dst, src[:n]...)
		items -= n
	}
	return dst
}

// IsInt64Zeros checks whether a contains only zeros.
func IsInt64Zeros(a []int64) bool {
	return isInt64SameValue(a, 0)
}

// IsInt64Ones checks whether a contains only ones.
func IsInt64Ones(a []int64) bool {
	return isInt64SameValue(a, 1)
}

// IsFloat64Zeros checks whether a contains only zeros.
func IsFloat64Zeros(a []float64) bool {
	if len(a) == 0 {
		return true
	}
	if math.Float64bits(a[0]) != 0 {
		return false
	}
	return isFloat64SameBits(a)
}

// IsFloat64Ones checks whether a contains only ones.
func IsFloat64Ones(a []float64) bool {
	if len(a) == 0 {
		return true
	}
	if math.Float64bits(a[0]) != math.Float64bits(1) {
		return false
	}
	return isFloat64SameBits(a)
}

// isInt64SameValue reports whether a consists of the same v values.
//
// Comparing the slice bytes against themselves shifted by one element
// proves that all elements are equal; a[0] then pins the exact value.
// Unlike a reference table, this reads only one memory stream.
func isInt64SameValue(a []int64, v int64) bool {
	if len(a) == 0 {
		return true
	}
	if a[0] != v {
		return false
	}
	if len(a) <= smallElems {
		for _, x := range a[1:] {
			if x != v {
				return false
			}
		}
		return true
	}
	b := int64ToByteSlice(a)
	return bytes.Equal(b[8:], b[:len(b)-8])
}

// isFloat64SameBits reports whether all elements of a share the same
// bit pattern as a[0].
func isFloat64SameBits(a []float64) bool {
	if len(a) <= smallElems {
		bits := math.Float64bits(a[0])
		for _, x := range a[1:] {
			if math.Float64bits(x) != bits {
				return false
			}
		}
		return true
	}
	b := float64ToByteSlice(a)
	return bytes.Equal(b[8:], b[:len(b)-8])
}

// appendOnesData appends items ones to dst. Small runs use a plain loop.
// Up to the seed table size it is a single append from the table. Larger runs
// copy an L1-sized seed prefix once and then keep copying from that hot prefix,
// so the copy source never leaves L1.
func appendOnesData[T int64 | float64](dst []T, items int, seed []T) []T {
	if items <= 0 {
		return dst
	}
	if items <= smallElems {
		m := len(dst)
		dst = slices.Grow(dst, items)
		t := dst[:m+items]
		for i := m; i < len(t); i++ {
			t[i] = 1
		}
		return t
	}
	if items <= len(seed) {
		return append(dst, seed[:items]...)
	}
	m := len(dst)
	dst = slices.Grow(dst, items)
	t := dst[:m+items]
	s := t[m:]
	n := copy(s, seed)
	for f := n; f < len(s); f += n {
		copy(s[f:], s[:n])
	}
	return t
}

func int64ToByteSlice(a []int64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(a))), len(a)*8)
}

func float64ToByteSlice(a []float64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(a))), len(a)*8)
}

var (
	int64Zeros [8 * 1024]int64
	int64Ones  = func() (a [seedElems]int64) {
		for i := range a {
			a[i] = 1
		}
		return a
	}()

	float64Zeros [8 * 1024]float64
	float64Ones  = func() (a [seedElems]float64) {
		for i := range a {
			a[i] = 1
		}
		return a
	}()
)
