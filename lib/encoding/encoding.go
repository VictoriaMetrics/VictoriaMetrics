package encoding

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// minCompressibleBlockSize is the minimum block size in bytes for trying compression.
//
// There is no sense in compressing smaller blocks.
const minCompressibleBlockSize = 128

// MarshalType is the type used for the marshaling.
type MarshalType byte

const (
	// MarshalTypeZSTDNearestDelta2 is used for marshaling counter
	// timeseries.
	MarshalTypeZSTDNearestDelta2 = MarshalType(1)

	// MarshalTypeDeltaConst is used for marshaling constantly changed
	// time series with constant delta.
	MarshalTypeDeltaConst = MarshalType(2)

	// MarshalTypeConst is used for marshaling time series containing only
	// a single constant.
	MarshalTypeConst = MarshalType(3)

	// MarshalTypeZSTDNearestDelta is used for marshaling gauge timeseries.
	MarshalTypeZSTDNearestDelta = MarshalType(4)

	// MarshalTypeNearestDelta2 is used instead of MarshalTypeZSTDNearestDelta2
	// if compression doesn't help.
	MarshalTypeNearestDelta2 = MarshalType(5)

	// MarshalTypeNearestDelta is used instead of MarshalTypeZSTDNearestDelta
	// if compression doesn't help.
	MarshalTypeNearestDelta = MarshalType(6)
)

// NeedsValidation returns true if mt may need additional validation for silent data corruption.
func (mt MarshalType) NeedsValidation() bool {
	switch mt {
	case MarshalTypeNearestDelta2,
		MarshalTypeNearestDelta:
		return true
	default:
		// Other types do not need additional validation,
		// since they either already contain checksums (e.g. compressed data)
		// or they are trivial and cannot be validated (e.g. const or delta const)
		return false
	}
}

// CheckMarshalType verifies whether the mt is valid.
func CheckMarshalType(mt MarshalType) error {
	if mt < 0 || mt > 6 {
		return fmt.Errorf("MarshalType should be in range [0..6]; got %d", mt)
	}
	return nil
}

// CheckPrecisionBits makes sure precisionBits is in the range [1..64].
func CheckPrecisionBits(precisionBits uint8) error {
	if precisionBits < 1 || precisionBits > 64 {
		return fmt.Errorf("precisionBits must be in the range [1...64]; got %d", precisionBits)
	}
	return nil
}

// MarshalTimestamps marshals timestamps, appends the marshaled result
// to dst and returns the dst.
//
// timestamps must contain non-decreasing values.
//
// precisionBits must be in the range [1...64], where 1 means 50% precision,
// while 64 means 100% precision, i.e. lossless encoding.
func MarshalTimestamps(dst []byte, timestamps []int64, precisionBits uint8) (result []byte, mt MarshalType, firstTimestamp int64) {
	return marshalInt64Array(dst, timestamps, precisionBits)
}

// UnmarshalTimestamps unmarshals timestamps from src, appends them to dst
// and returns the resulting dst.
//
// firstTimestamp must be the timestamp returned from MarshalTimestamps.
func UnmarshalTimestamps(dst []int64, src []byte, mt MarshalType, firstTimestamp int64, itemsCount int) ([]int64, error) {
	dst, err := unmarshalInt64Array(dst, src, mt, firstTimestamp, itemsCount)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %d timestamps from len(src)=%d bytes: %w", itemsCount, len(src), err)
	}
	return dst, nil
}

// MarshalValues marshals values, appends the marshaled result to dst
// and returns the dst.
//
// precisionBits must be in the range [1...64], where 1 means 50% precision,
// while 64 means 100% precision, i.e. lossless encoding.
func MarshalValues(dst []byte, values []int64, precisionBits uint8) (result []byte, mt MarshalType, firstValue int64) {
	return marshalInt64Array(dst, values, precisionBits)
}

// UnmarshalValues unmarshals values from src, appends them to dst and returns
// the resulting dst.
//
// firstValue must be the value returned from MarshalValues.
func UnmarshalValues(dst []int64, src []byte, mt MarshalType, firstValue int64, itemsCount int) ([]int64, error) {
	dst, err := unmarshalInt64Array(dst, src, mt, firstValue, itemsCount)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %d values from len(src)=%d bytes: %w", itemsCount, len(src), err)
	}
	return dst, nil
}

func marshalInt64Array(dst []byte, a []int64, precisionBits uint8) (result []byte, mt MarshalType, firstValue int64) {
	if len(a) == 0 {
		logger.Panicf("BUG: a must contain at least one item")
	}
	if isConst(a) {
		firstValue = a[0]
		return dst, MarshalTypeConst, firstValue
	}
	if isDeltaConst(a) {
		firstValue = a[0]
		dst = MarshalVarInt64(dst, a[1]-a[0])
		return dst, MarshalTypeDeltaConst, firstValue
	}

	bb := bbPool.Get()
	if isGauge(a) {
		// Gauge values are better compressed with delta encoding.
		mt = MarshalTypeZSTDNearestDelta
		pb := precisionBits
		if pb < 6 {
			// Increase precision bits for gauges, since they suffer more
			// from low precision bits comparing to counters.
			pb += 2
		}
		bb.B, firstValue = marshalInt64NearestDelta(bb.B[:0], a, pb)
	} else {
		// Non-gauge values, i.e. counters are better compressed with delta2 encoding.
		mt = MarshalTypeZSTDNearestDelta2
		bb.B, firstValue = marshalInt64NearestDelta2(bb.B[:0], a, precisionBits)
	}

	// Try compressing the result.
	dstOrig := dst
	if len(bb.B) >= minCompressibleBlockSize {
		compressLevel := getCompressLevel(len(a))
		dst = CompressZSTDLevel(dst, bb.B, compressLevel)
	}
	if len(bb.B) < minCompressibleBlockSize || float64(len(dst)-len(dstOrig)) > 0.9*float64(len(bb.B)) {
		// Ineffective compression. Store plain data.
		switch mt {
		case MarshalTypeZSTDNearestDelta2:
			mt = MarshalTypeNearestDelta2
		case MarshalTypeZSTDNearestDelta:
			mt = MarshalTypeNearestDelta
		default:
			logger.Panicf("BUG: unexpected mt=%d", mt)
		}
		dst = append(dstOrig, bb.B...)
	}
	bbPool.Put(bb)

	return dst, mt, firstValue
}

func unmarshalInt64Array(dst []int64, src []byte, mt MarshalType, firstValue int64, itemsCount int) ([]int64, error) {
	// Extend dst capacity in order to eliminate memory allocations below.
	dst = decimal.ExtendInt64sCapacity(dst, itemsCount)

	var err error
	switch mt {
	case MarshalTypeZSTDNearestDelta:
		bb := bbPool.Get()
		bb.B, err = DecompressZSTD(bb.B[:0], src)
		if err != nil {
			return nil, fmt.Errorf("cannot decompress zstd data: %w", err)
		}
		dst, err = unmarshalInt64NearestDelta(dst, bb.B, firstValue, itemsCount)
		bbPool.Put(bb)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal nearest delta data after zstd decompression: %w; src_zstd=%X", err, src)
		}
		return dst, nil
	case MarshalTypeZSTDNearestDelta2:
		bb := bbPool.Get()
		bb.B, err = DecompressZSTD(bb.B[:0], src)
		if err != nil {
			return nil, fmt.Errorf("cannot decompress zstd data: %w", err)
		}
		dst, err = unmarshalInt64NearestDelta2(dst, bb.B, firstValue, itemsCount)
		bbPool.Put(bb)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal nearest delta2 data after zstd decompression: %w; src_zstd=%X", err, src)
		}
		return dst, nil
	case MarshalTypeNearestDelta:
		dst, err = unmarshalInt64NearestDelta(dst, src, firstValue, itemsCount)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal nearest delta data: %w", err)
		}
		return dst, nil
	case MarshalTypeNearestDelta2:
		dst, err = unmarshalInt64NearestDelta2(dst, src, firstValue, itemsCount)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal nearest delta2 data: %w", err)
		}
		return dst, nil
	case MarshalTypeConst:
		if len(src) > 0 {
			return nil, fmt.Errorf("unexpected data left in const encoding: %d bytes", len(src))
		}
		if firstValue == 0 {
			dst = fastnum.AppendInt64Zeros(dst, itemsCount)
			return dst, nil
		}
		if firstValue == 1 {
			dst = fastnum.AppendInt64Ones(dst, itemsCount)
			return dst, nil
		}
		for itemsCount > 0 {
			dst = append(dst, firstValue)
			itemsCount--
		}
		return dst, nil
	case MarshalTypeDeltaConst:
		v := firstValue
		d, nLen := UnmarshalVarInt64(src)
		if nLen <= 0 {
			return nil, fmt.Errorf("cannot unmarshal delta value for delta const: %w", err)
		}
		if nLen < len(src) {
			return nil, fmt.Errorf("unexpected trailing data after delta const (d=%d): %d bytes", d, len(src)-nLen)
		}
		for itemsCount > 0 {
			dst = append(dst, v)
			itemsCount--
			v += d
		}
		return dst, nil
	default:
		return nil, fmt.Errorf("unknown MarshalType=%d", mt)
	}
}

var bbPool bytesutil.ByteBufferPool

// EnsureNonDecreasingSequence makes sure the first item in a is vMin, the last
// item in a is vMax and all the items in a are non-decreasing.
//
// If this isn't the case then a is fixed accordingly.
func EnsureNonDecreasingSequence(a []int64, vMin, vMax int64) {
	if vMax < vMin {
		logger.Panicf("BUG: vMax cannot be smaller than vMin; got %d vs %d", vMax, vMin)
	}
	if len(a) == 0 {
		return
	}
	if a[0] != vMin {
		a[0] = vMin
	}
	vPrev := a[0]
	aa := a[1:]
	for i, v := range aa {
		if v < vPrev {
			aa[i] = vPrev
			v = vPrev
		}
		vPrev = v
	}
	i := len(a) - 1
	if a[i] != vMax {
		a[i] = vMax
		i--
		for i >= 0 && a[i] > vMax {
			a[i] = vMax
			i--
		}
	}
}

// isConst returns true if a contains only equal values.
func isConst(a []int64) bool {
	if len(a) == 0 {
		return false
	}
	if fastnum.IsInt64Zeros(a) {
		// Fast path for array containing only zeros.
		return true
	}
	if fastnum.IsInt64Ones(a) {
		// Fast path for array containing only ones.
		return true
	}
	v1 := a[0]
	for _, v := range a {
		if v != v1 {
			return false
		}
	}
	return true
}

// isDeltaConst returns true if a contains counter with constant delta.
func isDeltaConst(a []int64) bool {
	if len(a) < 2 {
		return false
	}
	d1 := a[1] - a[0]
	prev := a[1]
	for _, next := range a[2:] {
		if next-prev != d1 {
			return false
		}
		prev = next
	}
	return true
}

// isGauge returns true if a contains gauge values,
// i.e. arbitrary changing values.
//
// It is OK if a few gauges aren't detected (i.e. detected as counters),
// since misdetected counters as gauges leads to worse compression ratio.
func isGauge(a []int64) bool {
	// Check all the items in a, since a part of items may lead
	// to incorrect gauge detection.

	if len(a) < 2 {
		return false
	}

	resets := 0
	vPrev := a[0]
	if vPrev < 0 {
		// Counter values cannot be negative.
		return true
	}
	for _, v := range a[1:] {
		if v < vPrev {
			if v < 0 {
				// Counter values cannot be negative.
				return true
			}
			if v > (vPrev >> 3) {
				// Decreasing sequence detected.
				// This is a gauge.
				return true
			}
			// Possible counter reset.
			resets++
		}
		vPrev = v
	}
	if resets <= 2 {
		// Counter with a few resets.
		return false
	}

	// Let it be a gauge if resets exceeds len(a)/8,
	// otherwise assume counter.
	return resets > (len(a) >> 3)
}

func getCompressLevel(itemsCount int) int {
	if itemsCount <= 1<<6 {
		return 1
	}
	if itemsCount <= 1<<8 {
		return 2
	}
	if itemsCount <= 1<<10 {
		return 3
	}
	if itemsCount <= 1<<12 {
		return 4
	}
	return 5
}
