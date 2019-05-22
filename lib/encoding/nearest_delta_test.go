package encoding

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestMarshalInt64NearestDelta(t *testing.T) {
	testMarshalInt64NearestDelta(t, []int64{0}, 4, 0, "")
	testMarshalInt64NearestDelta(t, []int64{0, 0}, 4, 0, "00")
	testMarshalInt64NearestDelta(t, []int64{1, -3}, 4, 1, "07")
	testMarshalInt64NearestDelta(t, []int64{255, 255}, 4, 255, "00")
	testMarshalInt64NearestDelta(t, []int64{0, 1, 2, 3, 4, 5}, 4, 0, "0202020202")
	testMarshalInt64NearestDelta(t, []int64{5, 4, 3, 2, 1, 0}, 1, 5, "0003000301")
	testMarshalInt64NearestDelta(t, []int64{5, 4, 3, 2, 1, 0}, 2, 5, "0003010101")
	testMarshalInt64NearestDelta(t, []int64{5, 4, 3, 2, 1, 0}, 3, 5, "0101010101")
	testMarshalInt64NearestDelta(t, []int64{5, 4, 3, 2, 1, 0}, 4, 5, "0101010101")

	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 1, -5e2, "00000000")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 2, -5e2, "0000ff0300")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 3, -5e2, "00ff01ff01ff01")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 4, -5e2, "7fff017fff01")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 5, -5e2, "bf01bf01bf01bf01")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 6, -5e2, "bf01bf01bf01bf01")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 7, -5e2, "bf01cf01bf01af01")
	testMarshalInt64NearestDelta(t, []int64{-5e2, -6e2, -7e2, -8e2, -8.9e2}, 8, -5e2, "c701c701c701af01")
}

func testMarshalInt64NearestDelta(t *testing.T, va []int64, precisionBits uint8, firstValueExpected int64, bExpected string) {
	t.Helper()

	b, firstValue := marshalInt64NearestDelta(nil, va, precisionBits)
	if firstValue != firstValueExpected {
		t.Fatalf("unexpected firstValue for va=%d, precisionBits=%d; got %d; want %d", va, precisionBits, firstValue, firstValueExpected)
	}
	if fmt.Sprintf("%x", b) != bExpected {
		t.Fatalf("invalid marshaled data for va=%d, precisionBits=%d; got\n%x; expecting\n%s", va, precisionBits, b, bExpected)
	}

	prefix := []byte("foobar")
	b, firstValue = marshalInt64NearestDelta(prefix, va, precisionBits)
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

func TestMarshalUnmarshalInt64NearestDelta(t *testing.T) {
	testMarshalUnmarshalInt64NearestDelta(t, []int64{0}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{0, 0}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{1, -3}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{255, 255}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{0, 1, 2, 3, 4, 5}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{5, 4, 3, 2, 1, 0}, 4)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 1)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 2)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{-5e12, -6e12, -7e12, -8e12, -8.9e12}, 3)
	testMarshalUnmarshalInt64NearestDelta(t, []int64{-5e12, -5.6e12, -7e12, -8e12, -8.9e12}, 4)

	// Verify constant encoding.
	va := []int64{}
	for i := 0; i < 1024; i++ {
		va = append(va, 9876543210123)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta(t, va, 63)

	// Verify encoding for monotonically incremented va.
	v := int64(-35)
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 8
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta(t, va, 63)

	// Verify encoding for monotonically decremented va.
	v = 793
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v -= 16
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)
	testMarshalUnmarshalInt64NearestDelta(t, va, 63)

	// Verify encoding for quadratically incremented va.
	v = -1234567
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 32 + int64(i)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)

	// Verify encoding for decremented va with norm-float noise.
	v = 787933
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v -= 25 + int64(rand.NormFloat64()*2)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)

	// Verify encoding for incremented va with random noise.
	v = 943854
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += 30 + rand.Int63n(5)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)

	// Verify encoding for constant va with norm-float noise.
	v = -12345
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += int64(rand.NormFloat64() * 10)
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)

	// Verify encoding for constant va with random noise.
	v = -12345
	va = []int64{}
	for i := 0; i < 1024; i++ {
		v += rand.Int63n(15) - 1
		va = append(va, v)
	}
	testMarshalUnmarshalInt64NearestDelta(t, va, 4)
}

func testMarshalUnmarshalInt64NearestDelta(t *testing.T, va []int64, precisionBits uint8) {
	t.Helper()

	b, firstValue := marshalInt64NearestDelta(nil, va, precisionBits)
	vaNew, err := unmarshalInt64NearestDelta(nil, b, firstValue, len(va))
	if err != nil {
		t.Fatalf("cannot unmarshal data for va=%d, precisionBits=%d from b=%x: %s", va, precisionBits, b, err)
	}
	if err = checkPrecisionBits(vaNew, va, precisionBits); err != nil {
		t.Fatalf("too small precisionBits for va=%d, precisionBits=%d: %s, vaNew=\n%d", va, precisionBits, err, vaNew)
	}

	vaPrefix := []int64{1, 2, 3, 4}
	vaNew, err = unmarshalInt64NearestDelta(vaPrefix, b, firstValue, len(va))
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
