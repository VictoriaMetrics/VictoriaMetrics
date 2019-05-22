package encoding

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestNearestDelta(t *testing.T) {
	testNearestDelta(t, 0, 0, 1, 0, 0)
	testNearestDelta(t, 0, 0, 2, 0, 0)
	testNearestDelta(t, 0, 0, 3, 0, 0)
	testNearestDelta(t, 0, 0, 4, 0, 0)

	testNearestDelta(t, 100, 100, 4, 0, 2)
	testNearestDelta(t, 123456, 123456, 4, 0, 12)
	testNearestDelta(t, -123456, -123456, 4, 0, 12)
	testNearestDelta(t, 9876543210, 9876543210, 4, 0, 29)

	testNearestDelta(t, 1, 2, 3, -1, 0)
	testNearestDelta(t, 2, 1, 3, 1, 0)
	testNearestDelta(t, -1, -2, 3, 1, 0)
	testNearestDelta(t, -2, -1, 3, -1, 0)

	testNearestDelta(t, 0, 1, 1, -1, 0)
	testNearestDelta(t, 1, 2, 1, -1, 0)
	testNearestDelta(t, 2, 3, 1, 0, 1)
	testNearestDelta(t, 1, 0, 1, 1, 0)
	testNearestDelta(t, 2, 1, 1, 0, 1)
	testNearestDelta(t, 2, 1, 2, 1, 0)
	testNearestDelta(t, 2, 1, 3, 1, 0)

	testNearestDelta(t, 0, -1, 1, 1, 0)
	testNearestDelta(t, -1, -2, 1, 1, 0)
	testNearestDelta(t, -2, -3, 1, 0, 1)
	testNearestDelta(t, -1, 0, 1, -1, 0)
	testNearestDelta(t, -2, -1, 1, 0, 1)
	testNearestDelta(t, -2, -1, 2, -1, 0)
	testNearestDelta(t, -2, -1, 3, -1, 0)

	testNearestDelta(t, 0, 2, 3, -2, 0)
	testNearestDelta(t, 3, 0, 3, 3, 0)
	testNearestDelta(t, 4, 0, 3, 4, 0)
	testNearestDelta(t, 5, 0, 3, 5, 0)
	testNearestDelta(t, 6, 0, 3, 6, 0)
	testNearestDelta(t, 0, 7, 3, -7, 0)
	testNearestDelta(t, 8, 0, 3, 8, 1)
	testNearestDelta(t, 9, 0, 3, 8, 1)
	testNearestDelta(t, 15, 0, 3, 14, 1)
	testNearestDelta(t, 16, 0, 3, 16, 2)
	testNearestDelta(t, 17, 0, 3, 16, 2)
	testNearestDelta(t, 18, 0, 3, 16, 2)
	testNearestDelta(t, 0, 59, 6, -59, 0)

	testNearestDelta(t, 128, 121, 1, 0, 7)
	testNearestDelta(t, 128, 121, 2, 0, 6)
	testNearestDelta(t, 128, 121, 3, 0, 5)
	testNearestDelta(t, 128, 121, 4, 0, 4)
	testNearestDelta(t, 128, 121, 5, 0, 3)
	testNearestDelta(t, 128, 121, 6, 4, 2)
	testNearestDelta(t, 128, 121, 7, 6, 1)
	testNearestDelta(t, 128, 121, 8, 7, 0)

	testNearestDelta(t, 32, 37, 1, 0, 5)
	testNearestDelta(t, 32, 37, 2, 0, 4)
	testNearestDelta(t, 32, 37, 3, 0, 3)
	testNearestDelta(t, 32, 37, 4, -4, 2)
	testNearestDelta(t, 32, 37, 5, -4, 1)
	testNearestDelta(t, 32, 37, 6, -5, 0)

	testNearestDelta(t, -10, 20, 1, -24, 3)
	testNearestDelta(t, -10, 20, 2, -28, 2)
	testNearestDelta(t, -10, 20, 3, -30, 1)
	testNearestDelta(t, -10, 20, 4, -30, 0)
	testNearestDelta(t, -10, 21, 4, -31, 0)
	testNearestDelta(t, -10, 21, 5, -31, 0)

	testNearestDelta(t, 10, -20, 1, 24, 3)
	testNearestDelta(t, 10, -20, 2, 28, 2)
	testNearestDelta(t, 10, -20, 3, 30, 1)
	testNearestDelta(t, 10, -20, 4, 30, 0)
	testNearestDelta(t, 10, -21, 4, 31, 0)
	testNearestDelta(t, 10, -21, 5, 31, 0)

	testNearestDelta(t, 1234e12, 1235e12, 1, 0, 50)
	testNearestDelta(t, 1234e12, 1235e12, 10, 0, 41)
	testNearestDelta(t, 1234e12, 1235e12, 35, -999999995904, 16)

	testNearestDelta(t, (1<<63)-1, 0, 1, (1<<63)-1, 2)
}

func testNearestDelta(t *testing.T, next, prev int64, precisionBits uint8, dExpected int64, trailingZeroBitsExpected uint8) {
	t.Helper()

	tz := getTrailingZeros(prev, precisionBits)
	d, trailingZeroBits := nearestDelta(next, prev, precisionBits, tz)
	if d != dExpected {
		t.Fatalf("unexpected d for next=%d, prev=%d, precisionBits=%d; got %d; expecting %d", next, prev, precisionBits, d, dExpected)
	}
	if trailingZeroBits != trailingZeroBitsExpected {
		t.Fatalf("unexpected trailingZeroBits for next=%d, prev=%d, precisionBits=%d; got %d; expecting %d",
			next, prev, precisionBits, trailingZeroBits, trailingZeroBitsExpected)
	}
}

func TestMarshalInt64NearestDelta2(t *testing.T) {
	testMarshalInt64NearestDelta2(t, []int64{0, 0}, 4, 0, "00")
	testMarshalInt64NearestDelta2(t, []int64{1, -3}, 4, 1, "07")
	testMarshalInt64NearestDelta2(t, []int64{255, 255}, 4, 255, "00")
	testMarshalInt64NearestDelta2(t, []int64{0, 1, 2, 3, 4, 5}, 4, 0, "0200000000")
	testMarshalInt64NearestDelta2(t, []int64{5, 4, 3, 2, 1, 0}, 4, 5, "0100000000")

	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 1, -5e3, "cf0f000000")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 2, -5e3, "cf0f000000")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 3, -5e3, "cf0f000000")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 4, -5e3, "cf0f00008001")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 5, -5e3, "cf0f0000c001")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 6, -5e3, "cf0f0000c001")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 7, -5e3, "cf0f0000c001")
	testMarshalInt64NearestDelta2(t, []int64{-5e3, -6e3, -7e3, -8e3, -8.9e3}, 8, -5e3, "cf0f0000c801")
}

func testMarshalInt64NearestDelta2(t *testing.T, va []int64, precisionBits uint8, firstValueExpected int64, bExpected string) {
	t.Helper()

	b, firstValue := marshalInt64NearestDelta2(nil, va, precisionBits)
	if firstValue != firstValueExpected {
		t.Fatalf("unexpected firstValue for va=%d, precisionBits=%d; got %d; want %d", va, precisionBits, firstValue, firstValueExpected)
	}
	if fmt.Sprintf("%x", b) != bExpected {
		t.Fatalf("invalid marshaled data for va=%d, precisionBits=%d; got\n%x; expecting\n%s", va, precisionBits, b, bExpected)
	}

	prefix := []byte("foobar")
	b, firstValue = marshalInt64NearestDelta2(prefix, va, precisionBits)
	if firstValue != firstValueExpected {
		t.Fatalf("unexpected firstValue for va=%d, precisionBits=%d; got %d; want %d", va, precisionBits, firstValue, firstValueExpected)
	}
	if string(b[:len(prefix)]) != string(prefix) {
		t.Fatalf("invalid prefix for va=%d, precisionBits=%d; got\n%x; expecting\n%x", va, precisionBits, b[:len(prefix)], prefix)
	}
	if fmt.Sprintf("%x", b[len(prefix):]) != bExpected {
		t.Fatalf("invalid marshaled prefixed data for va=%d, precisionBits=%d; got\n%x; expecting\n%s", va, precisionBits, b[len(prefix):], bExpected)
	}
}

func TestMarshalUnmarshalInt64NearestDelta2(t *testing.T) {
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{0, 0}, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{1, -3}, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{255, 255}, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{0, 1, 2, 3, 4, 5}, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{5, 4, 3, 2, 1, 0}, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 1)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 2)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 3)
	testMarshalUnmarshalInt64NearestDelta2(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 4)

	// Verify constant encoding.
	va := []int64{}
	for i := 0; i < 1024; i++ {
		va = append(va, 9876543210123)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, va, 63)

	// Verify encoding for monotonically incremented va.
	v := int64(-35)
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 8
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, va, 63)

	// Verify encoding for monotonically decremented va.
	v = 793
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v -= 16
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, va, 63)

	// Verify encoding for quadratically incremented va.
	v = -1234567
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 32 + int64(i)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta2(t, va, 63)

	// Verify encoding for decremented va with norm-float noise.
	v = 787933
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v -= 25 + int64(rand.NormFloat64()*2)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)

	// Verify encoding for incremented va with random noise.
	v = 943854
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 30 + rand.Int63n(5)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 4)

	// Verify encoding for constant va with norm-float noise.
	v = -12345
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += int64(rand.NormFloat64() * 10)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 2)

	// Verify encoding for constant va with random noise.
	v = -12345
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += rand.Int63n(15) - 1
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta2(t, va, 3)
}

func testMarshalUnmarshalInt64NearestDelta2(t *testing.T, va []int64, precisionBits uint8) {
	t.Helper()

	b, firstValue := marshalInt64NearestDelta2(nil, va, precisionBits)
	vaNew, err := unmarshalInt64NearestDelta2(nil, b, firstValue, len(va))
	if err != nil {
		t.Fatalf("cannot unmarshal data for va=%d, precisionBits=%d from b=%x: %s", va, precisionBits, b, err)
	}
	if err = checkPrecisionBits(vaNew, va, precisionBits); err != nil {
		t.Fatalf("too small precisionBits for va=%d, precisionBits=%d: %s, vaNew=\n%d", va, precisionBits, err, vaNew)
	}

	vaPrefix := []int64{1, 2, 3, 4}
	vaNew, err = unmarshalInt64NearestDelta2(vaPrefix, b, firstValue, len(va))
	if err != nil {
		t.Fatalf("cannot unmarshal prefixed data for va=%d, precisionBits=%d from b=%x: %s", va, precisionBits, b, err)
	}
	if !reflect.DeepEqual(vaNew[:len(vaPrefix)], vaPrefix) {
		t.Fatalf("unexpected prefix for va=%d, precisionBits=%d: got\n%d; expecting\n%d", va, precisionBits, vaNew[:len(vaPrefix)], vaPrefix)
	}
	if err = checkPrecisionBits(vaNew[len(vaPrefix):], va, precisionBits); err != nil {
		t.Fatalf("too small precisionBits for prefixed va=%d, precisionBits=%d: %s, vaNew=\n%d", va, precisionBits, err, vaNew[len(vaPrefix):])
	}
}

func checkPrecisionBits(a, b []int64, precisionBits uint8) error {
	if len(a) != len(b) {
		return fmt.Errorf("different-sized arrays: %d vs %d", len(a), len(b))
	}
	for i, av := range a {
		bv := b[i]
		if av < bv {
			av, bv = bv, av
		}
		eps := av - bv
		if eps == 0 {
			continue
		}
		if av < 0 {
			av = -av
		}
		pbe := uint8(1)
		for eps < av {
			av >>= 1
			pbe++
		}
		if pbe < precisionBits {
			return fmt.Errorf("too low precisionBits for\na=%d\nb=%d\ngot %d; expecting %d; compared values: %d vs %d, eps=%d",
				a, b, pbe, precisionBits, a[i], b[i], eps)
		}
	}
	return nil
}
