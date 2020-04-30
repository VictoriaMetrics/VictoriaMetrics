// +build cgo

package encoding

import (
	"math/rand"
	"testing"
)

func TestMarshalUnmarshalInt64Array(t *testing.T) {
	var va []int64
	var v int64

	// Verify nearest delta encoding.
	va = va[:0]
	v = 0
	for i := 0; i < 8*1024; i++ {
		v += int64(rand.NormFloat64() * 1e6)
		va = append(va, v)
	}
	for precisionBits := uint8(1); precisionBits < 23; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeZSTDNearestDelta)
	}
	for precisionBits := uint8(23); precisionBits < 65; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeNearestDelta)
	}

	// Verify nearest delta2 encoding.
	va = va[:0]
	v = 0
	for i := 0; i < 8*1024; i++ {
		v += 30e6 + int64(rand.NormFloat64()*1e6)
		va = append(va, v)
	}
	for precisionBits := uint8(1); precisionBits < 24; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeZSTDNearestDelta2)
	}
	for precisionBits := uint8(24); precisionBits < 65; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeNearestDelta2)
	}

	// Verify nearest delta encoding.
	va = va[:0]
	v = 1000
	for i := 0; i < 6; i++ {
		v += int64(rand.NormFloat64() * 100)
		va = append(va, v)
	}
	for precisionBits := uint8(1); precisionBits < 65; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeNearestDelta)
	}

	// Verify nearest delta2 encoding.
	va = va[:0]
	v = 0
	for i := 0; i < 6; i++ {
		v += 3000 + int64(rand.NormFloat64()*100)
		va = append(va, v)
	}
	for precisionBits := uint8(5); precisionBits < 65; precisionBits++ {
		testMarshalUnmarshalInt64Array(t, va, precisionBits, MarshalTypeNearestDelta2)
	}
}

func TestMarshalInt64ArraySize(t *testing.T) {
	var va []int64
	v := int64(rand.Float64() * 1e9)
	for i := 0; i < 8*1024; i++ {
		va = append(va, v)
		v += 30e3 + int64(rand.NormFloat64()*1e3)
	}

	testMarshalInt64ArraySize(t, va, 1, 180, 1400)
	testMarshalInt64ArraySize(t, va, 2, 250, 1550)
	testMarshalInt64ArraySize(t, va, 3, 600, 1800)
	testMarshalInt64ArraySize(t, va, 4, 1300, 2100)
	testMarshalInt64ArraySize(t, va, 5, 2000, 3200)
	testMarshalInt64ArraySize(t, va, 6, 3000, 4800)
	testMarshalInt64ArraySize(t, va, 7, 4000, 6400)
	testMarshalInt64ArraySize(t, va, 8, 6000, 8000)
	testMarshalInt64ArraySize(t, va, 9, 7000, 8800)
	testMarshalInt64ArraySize(t, va, 10, 8000, 10000)
}
