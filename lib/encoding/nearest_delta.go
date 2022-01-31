package encoding

import (
	"fmt"
	"math/bits"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// marshalInt64NearestDelta encodes src using `nearest delta` encoding
// with the given precisionBits and appends the encoded value to dst.
//
// precisionBits must be in the range [1...64], where 1 means 50% precision,
// while 64 means 100% precision, i.e. lossless encoding.
func marshalInt64NearestDelta(dst []byte, src []int64, precisionBits uint8) (result []byte, firstValue int64) {
	if len(src) < 1 {
		logger.Panicf("BUG: src must contain at least 1 item; got %d items", len(src))
	}
	if err := CheckPrecisionBits(precisionBits); err != nil {
		logger.Panicf("BUG: %s", err)
	}

	firstValue = src[0]
	v := src[0]
	src = src[1:]
	is := GetInt64s(len(src))
	if precisionBits == 64 {
		// Fast path.
		for i, next := range src {
			d := next - v
			v += d
			is.A[i] = d
		}
	} else {
		// Slower path.
		trailingZeros := getTrailingZeros(v, precisionBits)
		for i, next := range src {
			d, tzs := nearestDelta(next, v, precisionBits, trailingZeros)
			trailingZeros = tzs
			v += d
			is.A[i] = d
		}
	}
	dst = MarshalVarInt64s(dst, is.A)
	PutInt64s(is)
	return dst, firstValue
}

// unmarshalInt64NearestDelta decodes src using `nearest delta` encoding,
// appends the result to dst and returns the appended result.
//
// The firstValue must be the value returned from marshalInt64NearestDelta.
func unmarshalInt64NearestDelta(dst []int64, src []byte, firstValue int64, itemsCount int) ([]int64, error) {
	if itemsCount < 1 {
		logger.Panicf("BUG: itemsCount must be greater than 0; got %d", itemsCount)
	}

	is := GetInt64s(itemsCount - 1)
	defer PutInt64s(is)

	tail, err := UnmarshalVarInt64s(is.A, src)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal nearest delta from %d bytes; src=%X: %w", len(src), src, err)
	}
	if len(tail) > 0 {
		return nil, fmt.Errorf("unexpected tail left after unmarshaling %d items from %d bytes; tail size=%d; src=%X; tail=%X", itemsCount, len(src), len(tail), src, tail)
	}

	v := firstValue
	dst = append(dst, v)
	for _, d := range is.A {
		v += d
		dst = append(dst, v)
	}
	return dst, nil
}

// nearestDelta returns the nearest value for (next-prev) with the given
// precisionBits.
//
// The second returned value is the number of zeroed trailing bits
// in the returned delta.
func nearestDelta(next, prev int64, precisionBits, prevTrailingZeros uint8) (int64, uint8) {
	d := next - prev
	if d == 0 {
		// Fast path.
		return 0, decIfNonZero(prevTrailingZeros)
	}

	origin := next
	if origin < 0 {
		origin = -origin
		// There is no need in handling special case origin = -1<<63.
	}

	originBits := uint8(bits.Len64(uint64(origin)))
	if originBits <= precisionBits {
		// Cannot zero trailing bits for the given precisionBits.
		return d, decIfNonZero(prevTrailingZeros)
	}

	// originBits > precisionBits. May zero trailing bits in d.
	trailingZeros := originBits - precisionBits
	if trailingZeros > prevTrailingZeros+4 {
		// Probably counter reset. Return d with full precision.
		return d, prevTrailingZeros + 2
	}
	if trailingZeros+4 < prevTrailingZeros {
		// Probably counter reset. Return d with full precision.
		return d, prevTrailingZeros - 2
	}

	// zero trailing bits in d.
	minus := false
	if d < 0 {
		minus = true
		d = -d
		// There is no need in handling special case d = -1<<63.
	}
	nd := int64(uint64(d) & (uint64(1<<64-1) << trailingZeros))
	if minus {
		nd = -nd
	}
	return nd, trailingZeros
}

func decIfNonZero(n uint8) uint8 {
	if n == 0 {
		return 0
	}
	return n - 1
}

func getTrailingZeros(v int64, precisionBits uint8) uint8 {
	if v < 0 {
		v = -v
		// There is no need in special case handling for v = -1<<63
	}
	vBits := uint8(bits.Len64(uint64(v)))
	if vBits <= precisionBits {
		return 0
	}
	return vBits - precisionBits
}
