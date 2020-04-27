package encoding

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestIsConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, true)
	f([]int64{1, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 1, 1}, true)
	f([]int64{1, 1, 2}, false)
}

func TestIsDeltaConst(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isDeltaConst(a)
		if ok != okExpected {
			t.Fatalf("unexpected isDeltaConst for a=%d; got %v; want %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{1}, false)
	f([]int64{1, 2}, true)
	f([]int64{1, 2, 3}, true)
	f([]int64{3, 2, 1}, true)
	f([]int64{3, 2, 1, 0, -1, -2}, true)
	f([]int64{3, 2, 1, 0, -1, -2, 2}, false)
	f([]int64{1, 1}, true)
	f([]int64{1, 2, 1}, false)
	f([]int64{1, 2, 4}, false)
}

func TestIsGauge(t *testing.T) {
	f := func(a []int64, okExpected bool) {
		t.Helper()
		ok := isGauge(a)
		if ok != okExpected {
			t.Fatalf("unexpected result for isGauge(%d); got %v; expecting %v", a, ok, okExpected)
		}
	}
	f([]int64{}, false)
	f([]int64{0}, false)
	f([]int64{1, 2}, false)
	f([]int64{0, 1, 2, 3, 4, 5}, false)
	f([]int64{0, -1, -2, -3, -4}, true)
	f([]int64{0, 0, 0, 0, 0, 0, 0}, false)
	f([]int64{1, 1, 1, 1, 1}, false)
	f([]int64{1, 1, 2, 2, 2, 2}, false)
	f([]int64{1, 17, 2, 3}, false) // a single counter reset
	f([]int64{1, 5, 2, 3}, true)
	f([]int64{1, 5, 2, 3, 2}, true)
	f([]int64{-1, -5, -2, -3}, true)
	f([]int64{-1, -5, -2, -3, -2}, true)
	f([]int64{5, 6, 4, 3, 2}, true)
	f([]int64{4, 5, 6, 5, 4, 3, 2}, true)
	f([]int64{1064, 1132, 1083, 1062, 856, 747}, true)
}

func TestEnsureNonDecreasingSequence(t *testing.T) {
	testEnsureNonDecreasingSequence(t, []int64{}, -1234, -34, []int64{})
	testEnsureNonDecreasingSequence(t, []int64{123}, -1234, -1234, []int64{-1234})
	testEnsureNonDecreasingSequence(t, []int64{123}, -1234, 345, []int64{345})
	testEnsureNonDecreasingSequence(t, []int64{-23, -14}, -23, -14, []int64{-23, -14})
	testEnsureNonDecreasingSequence(t, []int64{-23, -14}, -25, 0, []int64{-25, 0})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, 2, 8, []int64{2, 2, 8, 8, 8, 8})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, -2, 8, []int64{-2, -1, 8, 8, 8, 8})
	testEnsureNonDecreasingSequence(t, []int64{0, -1, 10, 5, 6, 7}, -2, 12, []int64{-2, -1, 10, 10, 10, 12})
	testEnsureNonDecreasingSequence(t, []int64{1, 2, 1, 3, 4, 5}, 1, 5, []int64{1, 2, 2, 3, 4, 5})
}

func testEnsureNonDecreasingSequence(t *testing.T, a []int64, vMin, vMax int64, aExpected []int64) {
	t.Helper()

	EnsureNonDecreasingSequence(a, vMin, vMax)
	if !reflect.DeepEqual(a, aExpected) {
		t.Fatalf("unexpected a; got\n%d; expecting\n%d", a, aExpected)
	}
}

func testMarshalUnmarshalInt64Array(t *testing.T, va []int64, precisionBits uint8, mtExpected MarshalType) {
	t.Helper()

	b, mt, firstValue := marshalInt64Array(nil, va, precisionBits)
	if mt != mtExpected {
		t.Fatalf("unexpected MarshalType for va=%d, precisionBits=%d: got %d; expecting %d", va, precisionBits, mt, mtExpected)
	}
	vaNew, err := unmarshalInt64Array(nil, b, mt, firstValue, len(va))
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling va=%d, precisionBits=%d: %s", va, precisionBits, err)
	}
	if vaNew == nil && va != nil {
		vaNew = []int64{}
	}
	switch mt {
	case MarshalTypeZSTDNearestDelta, MarshalTypeZSTDNearestDelta2,
		MarshalTypeNearestDelta, MarshalTypeNearestDelta2:
		if err = checkPrecisionBits(va, vaNew, precisionBits); err != nil {
			t.Fatalf("too low precision for vaNew: %s", err)
		}
	default:
		if !reflect.DeepEqual(va, vaNew) {
			t.Fatalf("unexpected vaNew for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew, va)
		}
	}

	bPrefix := []byte{1, 2, 3}
	bNew, mtNew, firstValueNew := marshalInt64Array(bPrefix, va, precisionBits)
	if firstValueNew != firstValue {
		t.Fatalf("unexpected firstValue for prefixed va=%d, precisionBits=%d; got %d; want %d", va, precisionBits, firstValueNew, firstValue)
	}
	if string(bNew[:len(bPrefix)]) != string(bPrefix) {
		t.Fatalf("unexpected prefix for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, bNew[:len(bPrefix)], bPrefix)
	}
	if string(bNew[len(bPrefix):]) != string(b) {
		t.Fatalf("unexpected b for prefixed va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, bNew[len(bPrefix):], b)
	}
	if mtNew != mt {
		t.Fatalf("unexpected mt for prefixed va=%d, precisionBits=%d; got %d; expecting %d", va, precisionBits, mtNew, mt)
	}

	vaPrefix := []int64{4, 5, 6, 8}
	vaNew, err = unmarshalInt64Array(vaPrefix, b, mt, firstValue, len(va))
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling prefixed va=%d, precisionBits=%d: %s", va, precisionBits, err)
	}
	if !reflect.DeepEqual(vaNew[:len(vaPrefix)], vaPrefix) {
		t.Fatalf("unexpected prefix for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew[:len(vaPrefix)], vaPrefix)
	}
	if va == nil {
		va = []int64{}
	}
	switch mt {
	case MarshalTypeZSTDNearestDelta, MarshalTypeZSTDNearestDelta2,
		MarshalTypeNearestDelta, MarshalTypeNearestDelta2:
		if err = checkPrecisionBits(vaNew[len(vaPrefix):], va, precisionBits); err != nil {
			t.Fatalf("too low precision for prefixed vaNew: %s", err)
		}
	default:
		if !reflect.DeepEqual(vaNew[len(vaPrefix):], va) {
			t.Fatalf("unexpected prefixed vaNew for va=%d, precisionBits=%d; got\n%d; expecting\n%d", va, precisionBits, vaNew[len(vaPrefix):], va)
		}
	}
}

func TestMarshalUnmarshalTimestamps(t *testing.T) {
	const precisionBits = 3

	var timestamps []int64
	v := int64(0)
	for i := 0; i < 8*1024; i++ {
		v += 30e3 * int64(rand.NormFloat64()*5e2)
		timestamps = append(timestamps, v)
	}
	result, mt, firstTimestamp := MarshalTimestamps(nil, timestamps, precisionBits)
	timestamps2, err := UnmarshalTimestamps(nil, result, mt, firstTimestamp, len(timestamps))
	if err != nil {
		t.Fatalf("cannot unmarshal timestamps: %s", err)
	}
	if err := checkPrecisionBits(timestamps, timestamps2, precisionBits); err != nil {
		t.Fatalf("too low precision for timestamps: %s", err)
	}
}

func TestMarshalUnmarshalValues(t *testing.T) {
	const precisionBits = 3

	var values []int64
	v := int64(0)
	for i := 0; i < 8*1024; i++ {
		v += int64(rand.NormFloat64() * 1e2)
		values = append(values, v)
	}
	result, mt, firstValue := MarshalValues(nil, values, precisionBits)
	values2, err := UnmarshalValues(nil, result, mt, firstValue, len(values))
	if err != nil {
		t.Fatalf("cannot unmarshal values: %s", err)
	}
	if err := checkPrecisionBits(values, values2, precisionBits); err != nil {
		t.Fatalf("too low precision for values: %s", err)
	}
}

func TestMarshalUnmarshalInt64ArrayGeneric(t *testing.T) {
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, 234}, 4, MarshalTypeNearestDelta2)
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, -2345, 678934, 342}, 4, MarshalTypeNearestDelta)
	testMarshalUnmarshalInt64Array(t, []int64{1, 20, 2345, 6789, 12342}, 4, MarshalTypeNearestDelta2)

	// Constant encoding
	testMarshalUnmarshalInt64Array(t, []int64{1}, 4, MarshalTypeConst)
	testMarshalUnmarshalInt64Array(t, []int64{1, 2}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{-1, 0, 1, 2, 3, 4, 5}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{-10, -1, 8, 17, 26}, 4, MarshalTypeDeltaConst)
	testMarshalUnmarshalInt64Array(t, []int64{0, 0, 0, 0, 0, 0}, 4, MarshalTypeConst)
	testMarshalUnmarshalInt64Array(t, []int64{100, 100, 100, 100}, 4, MarshalTypeConst)
}

func testMarshalInt64ArraySize(t *testing.T, va []int64, precisionBits uint8, minSizeExpected, maxSizeExpected int) {
	t.Helper()

	b, _, _ := marshalInt64Array(nil, va, precisionBits)
	if len(b) > maxSizeExpected {
		t.Fatalf("too big size for marshaled %d items with precisionBits %d: got %d; expecting %d", len(va), precisionBits, len(b), maxSizeExpected)
	}
	if len(b) < minSizeExpected {
		t.Fatalf("too small size for marshaled %d items with precisionBits %d: got %d; expecting %d", len(va), precisionBits, len(b), minSizeExpected)
	}
}
