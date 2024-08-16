package encoding

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// marshalInt64NearestDelta2 encodes src using `nearest delta2` encoding
// with the given precisionBits and appends the encoded value to dst.
//
// precisionBits must be in the range [1...64], where 1 means 50% precision,
// while 64 means 100% precision, i.e. lossless encoding.
func marshalInt64NearestDelta2(dst []byte, src []int64, precisionBits uint8) (result []byte, firstValue int64) {
	if len(src) < 2 {
		logger.Panicf("BUG: src must contain at least 2 items; got %d items", len(src))
	}
	if err := CheckPrecisionBits(precisionBits); err != nil {
		logger.Panicf("BUG: %s", err)
	}

	firstValue = src[0]
	d1 := src[1] - src[0]
	dst = MarshalVarInt64(dst, d1)
	v := src[1]
	src = src[2:]
	is := GetInt64s(len(src))
	if precisionBits == 64 {
		// Fast path.
		for i, next := range src {
			d2 := next - v - d1
			d1 += d2
			v += d1
			is.A[i] = d2
		}
	} else {
		// Slower path.
		trailingZeros := getTrailingZeros(v, precisionBits)
		for i, next := range src {
			d2, tzs := nearestDelta(next-v, d1, precisionBits, trailingZeros)
			trailingZeros = tzs
			d1 += d2
			v += d1
			is.A[i] = d2
		}
	}
	dst = MarshalVarInt64s(dst, is.A)
	PutInt64s(is)
	return dst, firstValue
}

// unmarshalInt64NearestDelta2 decodes src using `nearest delta2` encoding,
// appends the result to dst and returns the appended result.
//
// firstValue must be the value returned from marshalInt64NearestDelta2.
func unmarshalInt64NearestDelta2(dst []int64, src []byte, firstValue int64, itemsCount int) ([]int64, error) {
	if itemsCount < 2 {
		logger.Panicf("BUG: itemsCount must be greater than 1; got %d", itemsCount)
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

	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+itemsCount)
	as := dst[dstLen:]

	v := firstValue
	d1 := is.A[0]
	as[0] = v
	v += d1
	as[1] = v
	as = as[2:]
	for i, d2 := range is.A[1:] {
		d1 += d2
		v += d1
		as[i] = v
	}

	return dst, nil
}
