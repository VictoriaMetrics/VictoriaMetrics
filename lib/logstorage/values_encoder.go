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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
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

	clear(ve.values)
	ve.values = ve.values[:0]
}

// encode encodes values to ve.values and returns the encoded value type with min/max encoded values.
//
// ve.values and dict is valid until values are changed.
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

// decodeInplace decodes values encoded with the given vt and the given dictValues inplace.
//
// the decoded values remain valid until vd.reset() is called.
func (vd *valuesDecoder) decodeInplace(values []string, vt valueType, dictValues []string) error {
	// do not reset vd.buf, since it may contain previously decoded data,
	// which must be preserved until reset() call.
	dstBuf := vd.buf

	switch vt {
	case valueTypeString:
		// nothing to do - values are already decoded.
	case valueTypeDict:
		sb := getStringBucket()
		for _, v := range dictValues {
			dstLen := len(dstBuf)
			dstBuf = append(dstBuf, v...)
			sb.a = append(sb.a, bytesutil.ToUnsafeString(dstBuf[dstLen:]))
		}
		for i, v := range values {
			id := int(v[0])
			if id >= len(dictValues) {
				return fmt.Errorf("unexpected dictionary id: %d; it must be smaller than %d", id, len(dictValues))
			}
			values[i] = sb.a[id]
		}
		putStringBucket(sb)
	case valueTypeUint8:
		for i, v := range values {
			if len(v) != 1 {
				return fmt.Errorf("unexpected value length for uint8; got %d; want 1", len(v))
			}
			n := unmarshalUint8(v)
			dstLen := len(dstBuf)
			dstBuf = marshalUint8String(dstBuf, n)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint16:
		for i, v := range values {
			if len(v) != 2 {
				return fmt.Errorf("unexpected value length for uint16; got %d; want 2", len(v))
			}
			n := unmarshalUint16(v)
			dstLen := len(dstBuf)
			dstBuf = marshalUint16String(dstBuf, n)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint32:
		for i, v := range values {
			if len(v) != 4 {
				return fmt.Errorf("unexpected value length for uint32; got %d; want 4", len(v))
			}
			n := unmarshalUint32(v)
			dstLen := len(dstBuf)
			dstBuf = marshalUint32String(dstBuf, n)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeUint64:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			n := unmarshalUint64(v)
			dstLen := len(dstBuf)
			dstBuf = marshalUint64String(dstBuf, n)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeFloat64:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			f := unmarshalFloat64(v)
			dstLen := len(dstBuf)
			dstBuf = marshalFloat64String(dstBuf, f)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeIPv4:
		for i, v := range values {
			if len(v) != 4 {
				return fmt.Errorf("unexpected value length for ipv4; got %d; want 4", len(v))
			}
			ip := unmarshalIPv4(v)
			dstLen := len(dstBuf)
			dstBuf = marshalIPv4String(dstBuf, ip)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	case valueTypeTimestampISO8601:
		for i, v := range values {
			if len(v) != 8 {
				return fmt.Errorf("unexpected value length for uint64; got %d; want 8", len(v))
			}
			timestamp := unmarshalTimestampISO8601(v)
			dstLen := len(dstBuf)
			dstBuf = marshalTimestampISO8601String(dstBuf, timestamp)
			values[i] = bytesutil.ToUnsafeString(dstBuf[dstLen:])
		}
	default:
		return fmt.Errorf("unknown valueType=%d", vt)
	}

	vd.buf = dstBuf
	return nil
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
	u64s := encoding.GetInt64s(len(srcValues))
	defer encoding.PutInt64s(u64s)
	a := u64s.A
	var minValue, maxValue int64
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
		dstBuf = encoding.MarshalUint64(dstBuf, uint64(n))
		v := bytesutil.ToUnsafeString(dstBuf[dstLen:])
		dstValues = append(dstValues, v)
	}
	return dstBuf, dstValues, valueTypeTimestampISO8601, uint64(minValue), uint64(maxValue)
}

// TryParseTimestampRFC3339Nano parses s as RFC3339 with optional nanoseconds part and timezone offset and returns unix timestamp in nanoseconds.
//
// If s doesn't contain timezone offset, then the local timezone is used.
//
// The returned timestamp can be negative if s is smaller than 1970 year.
func TryParseTimestampRFC3339Nano(s string) (int64, bool) {
	if len(s) < len("2006-01-02T15:04:05") {
		return 0, false
	}

	secs, ok, tail := tryParseTimestampSecs(s)
	if !ok {
		return 0, false
	}
	s = tail
	nsecs := secs * 1e9

	// Parse timezone offset
	n := strings.IndexAny(s, "Z+-")
	if n < 0 {
		nsecs -= timeutil.GetLocalTimezoneOffsetNsecs()
	} else {
		offsetStr := s[n+1:]
		if s[n] != 'Z' {
			isMinus := s[n] == '-'
			if len(offsetStr) == 0 {
				return 0, false
			}
			offsetNsecs, ok := tryParseTimezoneOffset(offsetStr)
			if !ok {
				return 0, false
			}
			if isMinus {
				offsetNsecs = -offsetNsecs
			}
			nsecs -= offsetNsecs
		} else {
			if len(offsetStr) != 0 {
				return 0, false
			}
		}
		s = s[:n]
	}

	// Parse optional fractional part of seconds.
	if len(s) == 0 {
		return nsecs, true
	}
	if s[0] == '.' {
		s = s[1:]
	}
	digits := len(s)
	if digits > 9 {
		return 0, false
	}
	n64, ok := tryParseUint64(s)
	if !ok {
		return 0, false
	}

	if digits < 9 {
		n64 *= uint64(math.Pow10(9 - digits))
	}
	nsecs += int64(n64)
	return nsecs, true
}

func tryParseTimezoneOffset(offsetStr string) (int64, bool) {
	n := strings.IndexByte(offsetStr, ':')
	if n < 0 {
		return 0, false
	}
	hourStr := offsetStr[:n]
	minuteStr := offsetStr[n+1:]
	hours, ok := tryParseUint64(hourStr)
	if !ok || hours > 24 {
		return 0, false
	}
	minutes, ok := tryParseUint64(minuteStr)
	if !ok || minutes > 60 {
		return 0, false
	}
	return int64(hours)*nsecsPerHour + int64(minutes)*nsecsPerMinute, true
}

// tryParseTimestampISO8601 parses 'YYYY-MM-DDThh:mm:ss.mssZ' and returns unix timestamp in nanoseconds.
//
// The returned timestamp can be negative if s is smaller than 1970 year.
func tryParseTimestampISO8601(s string) (int64, bool) {
	// Do not parse timestamps with timezone, since they cannot be converted back
	// to the same string representation in general case.
	// This may break search.
	if len(s) != len("2006-01-02T15:04:05.000Z") {
		return 0, false
	}

	secs, ok, tail := tryParseTimestampSecs(s)
	if !ok {
		return 0, false
	}
	s = tail
	nsecs := secs * 1e9

	if s[0] != '.' {
		return 0, false
	}
	s = s[1:]

	// Parse milliseconds
	tzDelimiter := s[len("000")]
	if tzDelimiter != 'Z' {
		return 0, false
	}
	millisecondStr := s[:len("000")]
	msecs, ok := tryParseUint64(millisecondStr)
	if !ok {
		return 0, false
	}
	s = s[len("000")+1:]

	if len(s) != 0 {
		logger.Panicf("BUG: unexpected tail in timestamp: %q", s)
	}

	nsecs += int64(msecs) * 1e6
	return nsecs, true
}

// tryParseTimestampSecs parses YYYY-MM-DDTHH:mm:ss into unix timestamp in seconds.
func tryParseTimestampSecs(s string) (int64, bool, string) {
	// Parse year
	if s[len("YYYY")] != '-' {
		return 0, false, s
	}
	yearStr := s[:len("YYYY")]
	n, ok := tryParseUint64(yearStr)
	if !ok || n < 1677 || n > 2262 {
		return 0, false, s
	}
	year := int(n)
	s = s[len("YYYY")+1:]

	// Parse month
	if s[len("MM")] != '-' {
		return 0, false, s
	}
	monthStr := s[:len("MM")]
	n, ok = tryParseUint64(monthStr)
	if !ok {
		return 0, false, s
	}
	month := time.Month(n)
	s = s[len("MM")+1:]

	// Parse day.
	//
	// Allow whitespace additionally to T as the delimiter after DD,
	// so SQL datetime format can be parsed additionally to RFC3339.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6721
	delim := s[len("DD")]
	if delim != 'T' && delim != ' ' {
		return 0, false, s
	}
	dayStr := s[:len("DD")]
	n, ok = tryParseUint64(dayStr)
	if !ok {
		return 0, false, s
	}
	day := int(n)
	s = s[len("DD")+1:]

	// Parse hour
	if s[len("HH")] != ':' {
		return 0, false, s
	}
	hourStr := s[:len("HH")]
	n, ok = tryParseUint64(hourStr)
	if !ok {
		return 0, false, s
	}
	hour := int(n)
	s = s[len("HH")+1:]

	// Parse minute
	if s[len("MM")] != ':' {
		return 0, false, s
	}
	minuteStr := s[:len("MM")]
	n, ok = tryParseUint64(minuteStr)
	if !ok {
		return 0, false, s
	}
	minute := int(n)
	s = s[len("MM")+1:]

	// Parse second
	secondStr := s[:len("SS")]
	n, ok = tryParseUint64(secondStr)
	if !ok {
		return 0, false, s
	}
	second := int(n)
	s = s[len("SS"):]

	secs := time.Date(year, month, day, hour, minute, second, 0, time.UTC).Unix()
	if secs < int64(-1<<63)/1e9 || secs >= int64((1<<63)-1)/1e9 {
		// Too big or too small timestamp
		return 0, false, s
	}
	return secs, true, s
}

// tryParseUint64 parses s as uint64 value.
func tryParseUint64(s string) (uint64, bool) {
	if len(s) == 0 || len(s) > len("18_446_744_073_709_551_615") {
		return 0, false
	}
	n := uint64(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '_' {
			continue
		}
		if ch < '0' || ch > '9' {
			return 0, false
		}
		if n > ((1<<64)-1)/10 {
			return 0, false
		}
		n *= 10
		d := uint64(ch - '0')
		if n > (1<<64)-1-d {
			return 0, false
		}
		n += d
	}
	return n, true
}

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

// tryParseIPv4 tries parsing ipv4 from s.
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

// tryParseFloat64Prefix tries parsing float64 number at the beginning of s and returns the remaining tail.
func tryParseFloat64Prefix(s string) (float64, bool, string) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == '_') {
		i++
	}
	if i == 0 {
		return 0, false, s
	}
	f, ok := tryParseFloat64(s[:i])
	return f, ok, s[i:]
}

// tryParseFloat64 tries parsing s as float64.
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
	p10 := math.Pow10(strings.Count(sFrac, "_") - len(sFrac))
	f := math.FMA(float64(nFrac), p10, float64(nInt))
	if minus {
		f = -f
	}
	return f, true
}

// tryParseBytes parses user-readable bytes representation in s.
//
// Supported suffixes:
//
//	K, KB - for 1000
func tryParseBytes(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}

	isMinus := s[0] == '-'
	if isMinus {
		s = s[1:]
	}

	n := int64(0)
	for len(s) > 0 {
		f, ok, tail := tryParseFloat64Prefix(s)
		if !ok {
			return 0, false
		}
		if len(tail) == 0 {
			if _, frac := math.Modf(f); frac != 0 {
				// deny floating-point numbers without any suffix.
				return 0, false
			}
		}
		s = tail
		if len(s) == 0 {
			n += int64(f)
			continue
		}
		if len(s) >= 3 {
			switch {
			case strings.HasPrefix(s, "KiB"):
				n += int64(f * (1 << 10))
				s = s[3:]
				continue
			case strings.HasPrefix(s, "MiB"):
				n += int64(f * (1 << 20))
				s = s[3:]
				continue
			case strings.HasPrefix(s, "GiB"):
				n += int64(f * (1 << 30))
				s = s[3:]
				continue
			case strings.HasPrefix(s, "TiB"):
				n += int64(f * (1 << 40))
				s = s[3:]
				continue
			}
		}
		if len(s) >= 2 {
			switch {
			case strings.HasPrefix(s, "Ki"):
				n += int64(f * (1 << 10))
				s = s[2:]
				continue
			case strings.HasPrefix(s, "Mi"):
				n += int64(f * (1 << 20))
				s = s[2:]
				continue
			case strings.HasPrefix(s, "Gi"):
				n += int64(f * (1 << 30))
				s = s[2:]
				continue
			case strings.HasPrefix(s, "Ti"):
				n += int64(f * (1 << 40))
				s = s[2:]
				continue
			case strings.HasPrefix(s, "KB"):
				n += int64(f * 1_000)
				s = s[2:]
				continue
			case strings.HasPrefix(s, "MB"):
				n += int64(f * 1_000_000)
				s = s[2:]
				continue
			case strings.HasPrefix(s, "GB"):
				n += int64(f * 1_000_000_000)
				s = s[2:]
				continue
			case strings.HasPrefix(s, "TB"):
				n += int64(f * 1_000_000_000_000)
				s = s[2:]
				continue
			}
		}
		switch {
		case strings.HasPrefix(s, "B"):
			n += int64(f)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "K"):
			n += int64(f * 1_000)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "M"):
			n += int64(f * 1_000_000)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "G"):
			n += int64(f * 1_000_000_000)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "T"):
			n += int64(f * 1_000_000_000_000)
			s = s[1:]
			continue
		}
	}

	if isMinus {
		n = -n
	}
	return n, true
}

// tryParseIPv4Mask parses '/num' ipv4 mask and returns (1<<(32-num))
func tryParseIPv4Mask(s string) (uint64, bool) {
	if len(s) == 0 || s[0] != '/' {
		return 0, false
	}
	s = s[1:]
	n, ok := tryParseUint64(s)
	if !ok || n > 32 {
		return 0, false
	}
	return 1 << (32 - uint8(n)), true
}

// tryParseDuration parses the given duration in nanoseconds and returns the result.
func tryParseDuration(s string) (int64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	isMinus := s[0] == '-'
	if isMinus {
		s = s[1:]
	}

	nsecs := int64(0)
	for len(s) > 0 {
		f, ok, tail := tryParseFloat64Prefix(s)
		if !ok {
			return 0, false
		}
		s = tail
		if len(s) == 0 {
			return 0, false
		}
		if len(s) >= 3 {
			if strings.HasPrefix(s, "µs") {
				nsecs += int64(f * nsecsPerMicrosecond)
				s = s[3:]
				continue
			}
		}
		if len(s) >= 2 {
			switch {
			case strings.HasPrefix(s, "ms"):
				nsecs += int64(f * nsecsPerMillisecond)
				s = s[2:]
				continue
			case strings.HasPrefix(s, "ns"):
				nsecs += int64(f)
				s = s[2:]
				continue
			}
		}
		switch {
		case strings.HasPrefix(s, "y"):
			nsecs += int64(f * nsecsPerYear)
			s = s[1:]
		case strings.HasPrefix(s, "w"):
			nsecs += int64(f * nsecsPerWeek)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "d"):
			nsecs += int64(f * nsecsPerDay)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "h"):
			nsecs += int64(f * nsecsPerHour)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "m"):
			nsecs += int64(f * nsecsPerMinute)
			s = s[1:]
			continue
		case strings.HasPrefix(s, "s"):
			nsecs += int64(f * nsecsPerSecond)
			s = s[1:]
			continue
		default:
			return 0, false
		}
	}

	if isMinus {
		nsecs = -nsecs
	}
	return nsecs, true
}

// marshalDurationString appends string representation of nsec duration to dst and returns the result.
func marshalDurationString(dst []byte, nsecs int64) []byte {
	if nsecs == 0 {
		return append(dst, '0')
	}

	if nsecs < 0 {
		dst = append(dst, '-')
		nsecs = -nsecs
	}
	formatFloat64Seconds := nsecs >= nsecsPerSecond

	if nsecs >= nsecsPerWeek {
		weeks := nsecs / nsecsPerWeek
		nsecs -= weeks * nsecsPerWeek
		dst = marshalUint64String(dst, uint64(weeks))
		dst = append(dst, 'w')
	}
	if nsecs >= nsecsPerDay {
		days := nsecs / nsecsPerDay
		nsecs -= days * nsecsPerDay
		dst = marshalUint8String(dst, uint8(days))
		dst = append(dst, 'd')
	}
	if nsecs >= nsecsPerHour {
		hours := nsecs / nsecsPerHour
		nsecs -= hours * nsecsPerHour
		dst = marshalUint8String(dst, uint8(hours))
		dst = append(dst, 'h')
	}
	if nsecs >= nsecsPerMinute {
		minutes := nsecs / nsecsPerMinute
		nsecs -= minutes * nsecsPerMinute
		dst = marshalUint8String(dst, uint8(minutes))
		dst = append(dst, 'm')
	}
	if nsecs >= nsecsPerSecond {
		if formatFloat64Seconds {
			seconds := float64(nsecs) / nsecsPerSecond
			dst = marshalFloat64String(dst, seconds)
			dst = append(dst, 's')
			return dst
		}
		seconds := nsecs / nsecsPerSecond
		nsecs -= seconds * nsecsPerSecond
		dst = marshalUint8String(dst, uint8(seconds))
		dst = append(dst, 's')
	}
	if nsecs >= nsecsPerMillisecond {
		msecs := nsecs / nsecsPerMillisecond
		nsecs -= msecs * nsecsPerMillisecond
		dst = marshalUint16String(dst, uint16(msecs))
		dst = append(dst, "ms"...)
	}
	if nsecs >= nsecsPerMicrosecond {
		usecs := nsecs / nsecsPerMicrosecond
		nsecs -= usecs * nsecsPerMicrosecond
		dst = marshalUint16String(dst, uint16(usecs))
		dst = append(dst, "µs"...)
	}
	if nsecs > 0 {
		dst = marshalUint16String(dst, uint16(nsecs))
		dst = append(dst, "ns"...)
	}
	return dst
}

const (
	nsecsPerYear        = 365 * 24 * 3600 * 1e9
	nsecsPerWeek        = 7 * 24 * 3600 * 1e9
	nsecsPerDay         = 24 * 3600 * 1e9
	nsecsPerHour        = 3600 * 1e9
	nsecsPerMinute      = 60 * 1e9
	nsecsPerSecond      = 1e9
	nsecsPerMillisecond = 1e6
	nsecsPerMicrosecond = 1e3
)

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
	clear(vd.values)
	vd.values = vd.values[:0]
}

func (vd *valuesDict) copyFrom(a *arena, src *valuesDict) {
	vd.reset()

	dstValues := vd.values
	for _, v := range src.values {
		v = a.copyString(v)
		dstValues = append(dstValues, v)
	}
	vd.values = dstValues
}

func (vd *valuesDict) copyFromNoArena(src *valuesDict) {
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

// unmarshal unmarshals vd from src.
//
// vd is valid until a.reset() is called.
func (vd *valuesDict) unmarshal(a *arena, src []byte) ([]byte, error) {
	vd.reset()

	srcOrig := src
	if len(src) < 1 {
		return srcOrig, fmt.Errorf("cannot umarshal dict len from 0 bytes; need at least 1 byte")
	}
	dictLen := int(src[0])
	src = src[1:]
	for i := 0; i < dictLen; i++ {
		data, nSize := encoding.UnmarshalBytes(src)
		if nSize <= 0 {
			return srcOrig, fmt.Errorf("cannot umarshal value %d out of %d from dict", i, dictLen)
		}
		src = src[nSize:]

		v := a.copyBytesToString(data)
		vd.values = append(vd.values, v)
	}
	return src, nil
}

func unmarshalUint8(v string) uint8 {
	return uint8(v[0])
}

func unmarshalUint16(v string) uint16 {
	b := bytesutil.ToUnsafeBytes(v)
	return encoding.UnmarshalUint16(b)
}

func unmarshalUint32(v string) uint32 {
	b := bytesutil.ToUnsafeBytes(v)
	return encoding.UnmarshalUint32(b)
}

func unmarshalUint64(v string) uint64 {
	b := bytesutil.ToUnsafeBytes(v)
	return encoding.UnmarshalUint64(b)
}

func unmarshalFloat64(v string) float64 {
	n := unmarshalUint64(v)
	return math.Float64frombits(n)
}

func unmarshalIPv4(v string) uint32 {
	return unmarshalUint32(v)
}

func unmarshalTimestampISO8601(v string) int64 {
	n := unmarshalUint64(v)
	return int64(n)
}

func marshalUint8String(dst []byte, n uint8) []byte {
	if n < 10 {
		return append(dst, '0'+n)
	}
	if n < 100 {
		return append(dst, '0'+n/10, '0'+n%10)
	}

	if n < 200 {
		dst = append(dst, '1')
		n -= 100
	} else {
		dst = append(dst, '2')
		n -= 200
	}
	if n < 10 {
		return append(dst, '0', '0'+n)
	}
	return append(dst, '0'+n/10, '0'+n%10)
}

func marshalUint16String(dst []byte, n uint16) []byte {
	return marshalUint64String(dst, uint64(n))
}

func marshalUint32String(dst []byte, n uint32) []byte {
	return marshalUint64String(dst, uint64(n))
}

func marshalUint64String(dst []byte, n uint64) []byte {
	return strconv.AppendUint(dst, n, 10)
}

func marshalFloat64String(dst []byte, f float64) []byte {
	return strconv.AppendFloat(dst, f, 'f', -1, 64)
}

func marshalIPv4String(dst []byte, n uint32) []byte {
	dst = marshalUint8String(dst, uint8(n>>24))
	dst = append(dst, '.')
	dst = marshalUint8String(dst, uint8(n>>16))
	dst = append(dst, '.')
	dst = marshalUint8String(dst, uint8(n>>8))
	dst = append(dst, '.')
	dst = marshalUint8String(dst, uint8(n))
	return dst
}

// marshalTimestampISO8601String appends ISO8601-formatted nsecs to dst and returns the result.
func marshalTimestampISO8601String(dst []byte, nsecs int64) []byte {
	return time.Unix(0, nsecs).UTC().AppendFormat(dst, iso8601Timestamp)
}

const iso8601Timestamp = "2006-01-02T15:04:05.000Z"

// marshalTimestampRFC3339NanoString appends RFC3339Nano-formatted nsecs to dst and returns the result.
func marshalTimestampRFC3339NanoString(dst []byte, nsecs int64) []byte {
	return time.Unix(0, nsecs).UTC().AppendFormat(dst, time.RFC3339Nano)
}
