package logstorage

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// valueType is the type of values stored in every column block.
type valueType byte

const (
	// valueTypeUnknown is used for determining whether the value type is unknown.
	valueTypeUnknown = valueType(0)

	// default encoding for column blocks. Strings are stored as is.
	valueTypeString = valueType(1)

	// column blocks with small number of unique values are encoded as dict.
	valueTypeDict = valueType(2)

	// uint values up to 2^8-1 are encoded into valueTypeUint8.
	// Every value occupies a single byte.
	valueTypeUint8 = valueType(3)

	// uint values up to 2^16-1 are encoded into valueTypeUint16.
	// Every value occupies 2 bytes.
	valueTypeUint16 = valueType(4)

	// uint values up to 2^31-1 are encoded into valueTypeUint32.
	// Every value occupies 4 bytes.
	valueTypeUint32 = valueType(5)

	// uint values up to 2^64-1 are encoded into valueTypeUint64.
	// Every value occupies 8 bytes.
	valueTypeUint64 = valueType(6)

	// floating-point values are encoded into valueTypeFloat64.
	valueTypeFloat64 = valueType(7)

	// column blocks with ipv4 addresses are encoded as 4-byte strings.
	valueTypeIPv4 = valueType(8)

	// column blocks with ISO8601 timestamps are encoded into valueTypeTimestampISO8601.
	// These timestamps are commonly used by Logstash.
	valueTypeTimestampISO8601 = valueType(9)
)

type valuesEncoder struct {
	// buf contains data for values.
	buf []byte

	// values contains encoded values.
	values []string
}

func (ve *valuesEncoder) reset() {
	ve.buf = ve.buf[:0]

	vs := ve.values
	for i := range vs {
		vs[i] = ""
	}
	ve.values = vs[:0]
}

// encode encodes values to ve.values and returns the encoded value type with min/max encoded values.
func (ve *valuesEncoder) encode(values []string, dict *valuesDict) (valueType, uint64, uint64) {
	ve.reset()

	if len(values) == 0 {
		return valueTypeString, 0, 0
	}

	var vt valueType
	var minValue, maxValue uint64

	// Try dict encoding at first, since it gives the highest speedup during querying.
	// It also usually gives the best compression, since every value is encoded as a single byte.
	ve.buf, ve.values, vt = tryDictEncoding(ve.buf[:0], ve.values[:0], values, dict)
	if vt != valueTypeUnknown {
		return vt, 0, 0
	}

	ve.buf, ve.values, vt, minValue, maxValue = tryUintEncoding(ve.buf[:0], ve.values[:0], values)
	if vt != valueTypeUnknown {
		return vt, minValue, maxValue
	}

	ve.buf, ve.values, vt, minValue, maxValue = tryFloat64Encoding(ve.buf[:0], ve.values[:0], values)
	if vt != valueTypeUnknown {
		return vt, minValue, maxValue
	}

	ve.buf, ve.values, vt, minValue, maxValue = tryIPv4Encoding(ve.buf[:0], ve.values[:0], values)
	if vt != valueTypeUnknown {
		return vt, minValue, maxValue
	}

	ve.buf, ve.values, vt, minValue, maxValue = tryTimestampISO8601Encoding(ve.buf[:0], ve.values[:0], values)
	if vt != valueTypeUnknown {
		return vt, minValue, maxValue
	}

	// Fall back to default encoding, e.g. leave values as is.
	ve.values = append(ve.values[:0], values...)
	return valueTypeString, 0, 0
}

func getValuesEncoder() *valuesEncoder {
	v := valuesEncoderPool.Get()
	if v == nil {
		return &valuesEncoder{}
	}
	return v.(*valuesEncoder)
}

func putValuesEncoder(ve *valuesEncoder) {
	ve.reset()
	valuesEncoderPool.Put(ve)
}

var valuesEncoderPool sync.Pool

type valuesDecoder struct {
	buf []byte
}

func (vd *valuesDecoder) reset() {
	vd.buf = vd.buf[:0]
}

// decodeInplace decodes values encoded with the given vt and the given dict inplace.
//
// the decoded values remain valid until vd.reset() is called.
func (vd *valuesDecoder) decodeInplace(values []string, vt valueType, dict *valuesDict) error {
	// do not reset vd.buf, since it may contain previously decoded data,
	// which must be preserved until reset() call.
	dstBuf := vd.buf

	switch vt {
	case valueTypeString:
		// nothing to do - values are already decoded.
	case valueTypeUint8:
		for i, v := range values {
			if len(v) != 1 {
				return fmt.Errorf("unexpected value length for uint8; got %d; want 1", len(v))
			}
			n := uint64(v[0])
			dstLen := len(dstBuf)
			dstBuf = strconv.AppendUint(dstBuf, n, 10)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint16:
		for i, v := range values {
			if len(v) != 2 {
				return fmt.Errorf("unexpected value length for uint16; got %d; want 2", len(v))
			}
			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint16(b))
			dstLen := len(dstBuf)
			dstBuf = strconv.AppendUint(dstBuf, n, 10)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint32:
		for i, v := range values {
			if len(v) != 4 {
				return fmt.Errorf("unexpected value length for uint32; got %d; want 4", len(v))
			}
			b := bytesutil.ToUnsafeBytes(v)
			n := uint64(encoding.UnmarshalUint32(b))
			dstLen := len(dstBuf)
			dstBuf = strconv.AppendUint(dstBuf, n, 10)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint64:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			b := bytesutil.ToUnsafeBytes(v)
			n := encoding.UnmarshalUint64(b)
			dstLen := len(dstBuf)
			dstBuf = strconv.AppendUint(dstBuf, n, 10)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeDict:
		dictValues := dict.values
		for i, v := range values {
			id := int(v[0])
			if id >= len(dictValues) {
				return fmt.Errorf("unexpected dictionary id: %d; it must be smaller than %d", id, len(dictValues))
			}
			values[i] = dictValues[id]
		}
	case valueTypeIPv4:
		for i, v := range values {
			if len(v) != 4 {
				return fmt.Errorf("unexpected value length for ipv4; got %d; want 4", len(v))
			}
			dstLen := len(dstBuf)
			dstBuf = toIPv4String(dstBuf, v)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeTimestampISO8601:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			dstLen := len(dstBuf)
			dstBuf = toTimestampISO8601String(dstBuf, v)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeFloat64:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			dstLen := len(dstBuf)
			dstBuf = toFloat64String(dstBuf, v)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	default:
		return fmt.Errorf("unknown valueType=%d", vt)
	}

	vd.buf = dstBuf
	return nil
}

func toTimestampISO8601String(dst []byte, v string) []byte {
	b := bytesutil.ToUnsafeBytes(v)
	n := encoding.UnmarshalUint64(b)
	t := time.Unix(0, int64(n)).UTC()
	dst = t.AppendFormat(dst, iso8601Timestamp)
	return dst
}

func toIPv4String(dst []byte, v string) []byte {
	dst = strconv.AppendUint(dst, uint64(v[0]), 10)
	dst = append(dst, '.')
	dst = strconv.AppendUint(dst, uint64(v[1]), 10)
	dst = append(dst, '.')
	dst = strconv.AppendUint(dst, uint64(v[2]), 10)
	dst = append(dst, '.')
	dst = strconv.AppendUint(dst, uint64(v[3]), 10)
	return dst
}

func toFloat64String(dst []byte, v string) []byte {
	b := bytesutil.ToUnsafeBytes(v)
	n := encoding.UnmarshalUint64(b)
	f := math.Float64frombits(n)
	dst = strconv.AppendFloat(dst, f, 'g', -1, 64)
	return dst
}

func getValuesDecoder() *valuesDecoder {
	v := valuesDecoderPool.Get()
	if v == nil {
		return &valuesDecoder{}
	}
	return v.(*valuesDecoder)
}

func putValuesDecoder(vd *valuesDecoder) {
	vd.reset()
	valuesDecoderPool.Put(vd)
}

var valuesDecoderPool sync.Pool

func tryTimestampISO8601Encoding(dstBuf []byte, dstValues, srcValues []string) ([]byte, []string, valueType, uint64, uint64) {
	u64s := encoding.GetUint64s(len(srcValues))
	defer encoding.PutUint64s(u64s)
	a := u64s.A
	var minValue, maxValue uint64
	for i, v := range srcValues {
		n, ok := tryParseTimestampISO8601(v)
		if !ok {
			return dstBuf, dstValues, valueTypeUnknown, 0, 0
		}
		a[i] = n
		if i == 0 || n < minValue {
			minValue = n
		}
		if i == 0 || n > maxValue {
			maxValue = n
		}
	}
	for _, n := range a {
		dstLen := len(dstBuf)
		dstBuf = encoding.MarshalUint64(dstBuf, n)
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	return dstBuf, dstValues, valueTypeTimestampISO8601, minValue, maxValue
}

func tryParseTimestampISO8601(s string) (uint64, bool) {
	// Do not parse timestamps with timezone, since they cannot be converted back
	// to the same string representation in general case.
	// This may break search.
	if len(s) != len("2006-01-02T15:04:05.000Z") {
		return 0, false
	}

	// Parse year
	if s[len("YYYY")] != '-' {
		return 0, false
	}
	yearStr := s[:len("YYYY")]
	n, ok := tryParseUint64(yearStr)
	if !ok || n > 3000 {
		return 0, false
	}
	year := int(n)
	s = s[len("YYYY")+1:]

	// Parse month
	if s[len("MM")] != '-' {
		return 0, false
	}
	monthStr := s[:len("MM")]
	n, ok = tryParseUint64(monthStr)
	if !ok || n < 1 || n > 12 {
		return 0, false
	}
	month := time.Month(n)
	s = s[len("MM")+1:]

	// Parse day
	if s[len("DD")] != 'T' {
		return 0, false
	}
	dayStr := s[:len("DD")]
	n, ok = tryParseUint64(dayStr)
	if !ok || n < 1 || n > 31 {
		return 0, false
	}
	day := int(n)
	s = s[len("DD")+1:]

	// Parse hour
	if s[len("HH")] != ':' {
		return 0, false
	}
	hourStr := s[:len("HH")]
	n, ok = tryParseUint64(hourStr)
	if !ok || n > 23 {
		return 0, false
	}
	hour := int(n)
	s = s[len("HH")+1:]

	// Parse minute
	if s[len("MM")] != ':' {
		return 0, false
	}
	minuteStr := s[:len("MM")]
	n, ok = tryParseUint64(minuteStr)
	if !ok || n > 59 {
		return 0, false
	}
	minute := int(n)
	s = s[len("MM")+1:]

	// Parse second
	if s[len("SS")] != '.' {
		return 0, false
	}
	secondStr := s[:len("SS")]
	n, ok = tryParseUint64(secondStr)
	if !ok || n > 59 {
		return 0, false
	}
	second := int(n)
	s = s[len("SS")+1:]

	// Parse millisecond
	tzDelimiter := s[len("000")]
	if tzDelimiter != 'Z' {
		return 0, false
	}
	millisecondStr := s[:len("000")]
	n, ok = tryParseUint64(millisecondStr)
	if !ok || n > 999 {
		return 0, false
	}
	millisecond := int(n)
	s = s[len("000")+1:]

	if len(s) != 0 {
		return 0, false
	}

	t := time.Date(year, month, day, hour, minute, second, millisecond*1e6, time.UTC)
	ts := t.UnixNano()
	return uint64(ts), true
}

func tryParseUint64(s string) (uint64, bool) {
	if len(s) == 0 || len(s) > 18 {
		return 0, false
	}
	n := uint64(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n *= 10
		n += uint64(ch - '0')
	}
	return n, true
}

const iso8601Timestamp = "2006-01-02T15:04:05.000Z"

func tryIPv4Encoding(dstBuf []byte, dstValues, srcValues []string) ([]byte, []string, valueType, uint64, uint64) {
	u32s := encoding.GetUint32s(len(srcValues))
	defer encoding.PutUint32s(u32s)
	a := u32s.A
	var minValue, maxValue uint32
	for i, v := range srcValues {
		n, ok := tryParseIPv4(v)
		if !ok {
			return dstBuf, dstValues, valueTypeUnknown, 0, 0
		}
		a[i] = n
		if i == 0 || n < minValue {
			minValue = n
		}
		if i == 0 || n > maxValue {
			maxValue = n
		}
	}
	for _, n := range a {
		dstLen := len(dstBuf)
		dstBuf = encoding.MarshalUint32(dstBuf, n)
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	return dstBuf, dstValues, valueTypeIPv4, uint64(minValue), uint64(maxValue)
}

func tryParseIPv4(s string) (uint32, bool) {
	if len(s) < len("1.1.1.1") || len(s) > len("255.255.255.255") || strings.Count(s, ".") != 3 {
		// Fast path - the entry isn't IPv4
		return 0, false
	}

	var octets [4]byte
	var v uint64
	var ok bool

	// Parse octet 1
	n := strings.IndexByte(s, '.')
	if n <= 0 || n > 3 {
		return 0, false
	}
	v, ok = tryParseUint64(s[:n])
	if !ok || v > 255 {
		return 0, false
	}
	octets[0] = byte(v)
	s = s[n+1:]

	// Parse octet 2
	n = strings.IndexByte(s, '.')
	if n <= 0 || n > 3 {
		return 0, false
	}
	v, ok = tryParseUint64(s[:n])
	if !ok || v > 255 {
		return 0, false
	}
	octets[1] = byte(v)
	s = s[n+1:]

	// Parse octet 3
	n = strings.IndexByte(s, '.')
	if n <= 0 || n > 3 {
		return 0, false
	}
	v, ok = tryParseUint64(s[:n])
	if !ok || v > 255 {
		return 0, false
	}
	octets[2] = byte(v)
	s = s[n+1:]

	// Parse octet 4
	v, ok = tryParseUint64(s)
	if !ok || v > 255 {
		return 0, false
	}
	octets[3] = byte(v)

	ipv4 := encoding.UnmarshalUint32(octets[:])
	return ipv4, true
}

func tryFloat64Encoding(dstBuf []byte, dstValues, srcValues []string) ([]byte, []string, valueType, uint64, uint64) {
	u64s := encoding.GetUint64s(len(srcValues))
	defer encoding.PutUint64s(u64s)
	a := u64s.A
	var minValue, maxValue float64
	for i, v := range srcValues {
		f, ok := tryParseFloat64(v)
		if !ok {
			return dstBuf, dstValues, valueTypeUnknown, 0, 0
		}
		a[i] = math.Float64bits(f)
		if i == 0 || f < minValue {
			minValue = f
		}
		if i == 0 || f > maxValue {
			maxValue = f
		}
	}
	for _, n := range a {
		dstLen := len(dstBuf)
		dstBuf = encoding.MarshalUint64(dstBuf, n)
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	minValueU64 := math.Float64bits(minValue)
	maxValueU64 := math.Float64bits(maxValue)
	return dstBuf, dstValues, valueTypeFloat64, minValueU64, maxValueU64
}

func tryParseFloat64(s string) (float64, bool) {
	if len(s) == 0 || len(s) > 20 {
		return 0, false
	}
	// Allow only decimal digits, minus and a dot.
	// Do not allows scientific notation (for example 1.23E+05),
	// since it cannot be converted back to the same string form.

	minus := s[0] == '-'
	if minus {
		s = s[1:]
	}
	n := strings.IndexByte(s, '.')
	if n < 0 {
		// fast path - there are no dots
		n, ok := tryParseUint64(s)
		if !ok {
			return 0, false
		}
		f := float64(n)
		if minus {
			f = -f
		}
		return f, true
	}
	if n == 0 || n == len(s)-1 {
		// Do not allow dots at the beginning and at the end of s,
		// since they cannot be converted back to the same string form.
		return 0, false
	}
	sInt := s[:n]
	sFrac := s[n+1:]
	nInt, ok := tryParseUint64(sInt)
	if !ok {
		return 0, false
	}
	nFrac, ok := tryParseUint64(sFrac)
	if !ok {
		return 0, false
	}
	f := math.FMA(float64(nFrac), math.Pow10(-len(sFrac)), float64(nInt))
	if minus {
		f = -f
	}
	return f, true
}

func tryUintEncoding(dstBuf []byte, dstValues, srcValues []string) ([]byte, []string, valueType, uint64, uint64) {
	u64s := encoding.GetUint64s(len(srcValues))
	defer encoding.PutUint64s(u64s)
	a := u64s.A
	var minValue, maxValue uint64
	for i, v := range srcValues {
		n, ok := tryParseUint64(v)
		if !ok {
			return dstBuf, dstValues, valueTypeUnknown, 0, 0
		}
		a[i] = n
		if i == 0 || n < minValue {
			minValue = n
		}
		if i == 0 || n > maxValue {
			maxValue = n
		}
	}

	minBitSize := bits.Len64(maxValue)
	if minBitSize <= 8 {
		for _, n := range a {
			dstLen := len(dstBuf)
			dstBuf = append(dstBuf, byte(n))
			v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
			dstValues = append(dstValues, v)
		}
		return dstBuf, dstValues, valueTypeUint8, minValue, maxValue
	}
	if minBitSize <= 16 {
		for _, n := range a {
			dstLen := len(dstBuf)
			dstBuf = encoding.MarshalUint16(dstBuf, uint16(n))
			v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
			dstValues = append(dstValues, v)
		}
		return dstBuf, dstValues, valueTypeUint16, minValue, maxValue
	}
	if minBitSize <= 32 {
		for _, n := range a {
			dstLen := len(dstBuf)
			dstBuf = encoding.MarshalUint32(dstBuf, uint32(n))
			v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
			dstValues = append(dstValues, v)
		}
		return dstBuf, dstValues, valueTypeUint32, minValue, maxValue
	}
	for _, n := range a {
		dstLen := len(dstBuf)
		dstBuf = encoding.MarshalUint64(dstBuf, n)
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	return dstBuf, dstValues, valueTypeUint64, minValue, maxValue
}

func tryDictEncoding(dstBuf []byte, dstValues, srcValues []string, dict *valuesDict) ([]byte, []string, valueType) {
	dict.reset()
	dstBufOrig := dstBuf
	dstValuesOrig := dstValues

	for _, v := range srcValues {
		id, ok := dict.getOrAdd(v)
		if !ok {
			dict.reset()
			return dstBufOrig, dstValuesOrig, valueTypeUnknown
		}
		dstLen := len(dstBuf)
		dstBuf = append(dstBuf, id)
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	return dstBuf, dstValues, valueTypeDict
}

type valuesDict struct {
	values []string
}

func (vd *valuesDict) reset() {
	vs := vd.values
	for i := range vs {
		vs[i] = ""
	}
	vd.values = vs[:0]
}

func (vd *valuesDict) copyFrom(src *valuesDict) {
	vd.reset()

	vd.values = append(vd.values[:0], src.values...)
}

func (vd *valuesDict) getOrAdd(k string) (byte, bool) {
	if len(k) > maxDictSizeBytes {
		return 0, false
	}
	vs := vd.values
	dictSizeBytes := 0
	for i, v := range vs {
		if k == v {
			return byte(i), true
		}
		dictSizeBytes += len(v)
	}
	if len(vs) >= maxDictLen || dictSizeBytes+len(k) > maxDictSizeBytes {
		return 0, false
	}
	vs = append(vs, k)
	vd.values = vs

	return byte(len(vs) - 1), true
}

func (vd *valuesDict) marshal(dst []byte) []byte {
	values := vd.values
	if len(values) > maxDictLen {
		logger.Panicf("BUG: valuesDict may contain max %d items; got %d items", maxDictLen, len(values))
	}
	dst = append(dst, byte(len(values)))
	for _, v := range values {
		dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(v))
	}
	return dst
}

func (vd *valuesDict) unmarshal(src []byte) ([]byte, error) {
	vd.reset()

	srcOrig := src
	if len(src) < 1 {
		return srcOrig, fmt.Errorf("cannot umarshal dict len from 0 bytes; need at least 1 byte")
	}
	dictLen := int(src[0])
	src = src[1:]
	for i := 0; i < dictLen; i++ {
		tail, data, err := encoding.UnmarshalBytes(src)
		if err != nil {
			return srcOrig, fmt.Errorf("cannot umarshal value %d out of %d from dict: %w", i, dictLen, err)
		}
		src = tail
		// Do not use bytesutil.InternBytes(data) here, since it works slower than the string(data) in prod
		v := string(data)
		vd.values = append(vd.values, v)
	}
	return src, nil
}
