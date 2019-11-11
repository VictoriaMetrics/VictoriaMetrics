package fastnum

import (
	"bytes"
	"unsafe"
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
	return appendInt64Data(dst, items, int64Ones[:])
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
	return appendFloat64Data(dst, items, float64Ones[:])
}

// IsInt64Zeros checks whether a contains only zeros.
func IsInt64Zeros(a []int64) bool {
	return isInt64Data(a, int64Zeros[:])
}

// IsInt64Ones checks whether a contains only ones.
func IsInt64Ones(a []int64) bool {
	return isInt64Data(a, int64Ones[:])
}

// IsFloat64Zeros checks whether a contains only zeros.
func IsFloat64Zeros(a []float64) bool {
	return isFloat64Data(a, float64Zeros[:])
}

// IsFloat64Ones checks whether a contains only ones.
func IsFloat64Ones(a []float64) bool {
	return isFloat64Data(a, float64Ones[:])
}

func appendInt64Data(dst []int64, items int, src []int64) []int64 {
	for items > 0 {
		n := len(src)
		if n > items {
			n = items
		}
		dst = append(dst, src[:n]...)
		items -= n
	}
	return dst
}

func appendFloat64Data(dst []float64, items int, src []float64) []float64 {
	for items > 0 {
		n := len(src)
		if n > items {
			n = items
		}
		dst = append(dst, src[:n]...)
		items -= n
	}
	return dst
}

func isInt64Data(a, data []int64) bool {
	if len(a) == 0 {
		return true
	}
	if len(data) != 8*1024 {
		panic("len(data) must equal to 8*1024")
	}
	b := (*[64 * 1024]byte)(unsafe.Pointer(&data[0]))
	for len(a) > 0 {
		n := len(data)
		if n > len(a) {
			n = len(a)
		}
		x := a[:n]
		a = a[n:]
		xb := (*[64 * 1024]byte)(unsafe.Pointer(&x[0]))
		xbLen := len(x) * 8
		if !bytes.Equal(xb[:xbLen], b[:xbLen]) {
			return false
		}
	}
	return true
}

func isFloat64Data(a, data []float64) bool {
	if len(a) == 0 {
		return true
	}
	if len(data) != 8*1024 {
		panic("len(data) must equal to 8*1024")
	}
	b := (*[64 * 1024]byte)(unsafe.Pointer(&data[0]))
	for len(a) > 0 {
		n := len(data)
		if n > len(a) {
			n = len(a)
		}
		x := a[:n]
		a = a[n:]
		xb := (*[64 * 1024]byte)(unsafe.Pointer(&x[0]))
		xbLen := len(x) * 8
		if !bytes.Equal(xb[:xbLen], b[:xbLen]) {
			return false
		}
	}
	return true
}

var (
	int64Zeros [8 * 1024]int64
	int64Ones  = func() (a [8 * 1024]int64) {
		for i := 0; i < len(a); i++ {
			a[i] = 1
		}
		return a
	}()

	float64Zeros [8 * 1024]float64
	float64Ones  = func() (a [8 * 1024]float64) {
		for i := 0; i < len(a); i++ {
			a[i] = 1
		}
		return a
	}()
)
