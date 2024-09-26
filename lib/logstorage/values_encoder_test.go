package logstorage

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestValuesEncoder(t *testing.T) {
	f := func(values []string, expectedValueType valueType, expectedMinValue, expectedMaxValue uint64) {
		t.Helper()
		ve := getValuesEncoder()
		var dict valuesDict
		vt, minValue, maxValue := ve.encode(values, &dict)
		if vt != expectedValueType {
			t.Fatalf("unexpected value type; got %d; want %d", vt, expectedValueType)
		}
		if minValue != expectedMinValue {
			t.Fatalf("unexpected minValue; got %d; want %d", minValue, expectedMinValue)
		}
		if maxValue != expectedMaxValue {
			t.Fatalf("unexpected maxValue; got %d; want %d", maxValue, expectedMaxValue)
		}
		encodedValues := append([]string{}, ve.values...)
		putValuesEncoder(ve)

		vd := getValuesDecoder()
		if err := vd.decodeInplace(encodedValues, vt, dict.values); err != nil {
			t.Fatalf("unexpected error in decodeInplace(): %s", err)
		}
		if len(values) == 0 {
			values = []string{}
		}
		if !reflect.DeepEqual(values, encodedValues) {
			t.Fatalf("unexpected values decoded\ngot\n%q\nwant\n%q", encodedValues, values)
		}
		putValuesDecoder(vd)
	}

	// An empty values list
	f(nil, valueTypeString, 0, 0)

	// string values
	values := make([]string, maxDictLen+1)
	for i := range values {
		values[i] = fmt.Sprintf("value_%d", i)
	}
	f(values, valueTypeString, 0, 0)

	// dict values
	f([]string{"foobar"}, valueTypeDict, 0, 0)
	f([]string{"foo", "bar"}, valueTypeDict, 0, 0)
	f([]string{"1", "2foo"}, valueTypeDict, 0, 0)

	// uint8 values
	for i := range values {
		values[i] = fmt.Sprintf("%d", uint64(i+1))
	}
	f(values, valueTypeUint8, 1, uint64(len(values)))

	// uint16 values
	for i := range values {
		values[i] = fmt.Sprintf("%d", uint64(i+1)<<8)
	}
	f(values, valueTypeUint16, 1<<8, uint64(len(values))<<8)

	// uint32 values
	for i := range values {
		values[i] = fmt.Sprintf("%d", uint64(i+1)<<16)
	}
	f(values, valueTypeUint32, 1<<16, uint64(len(values))<<16)

	// uint64 values
	for i := range values {
		values[i] = fmt.Sprintf("%d", uint64(i+1)<<32)
	}
	f(values, valueTypeUint64, 1<<32, uint64(len(values))<<32)

	// float64 values
	for i := range values {
		values[i] = fmt.Sprintf("%g", math.Sqrt(float64(i+1)))
	}
	f(values, valueTypeFloat64, 4607182418800017408, 4613937818241073152)

	// ipv4 values
	for i := range values {
		values[i] = fmt.Sprintf("1.2.3.%d", i)
	}
	f(values, valueTypeIPv4, 16909056, 16909064)

	// iso8601 timestamps
	for i := range values {
		values[i] = fmt.Sprintf("2011-04-19T03:44:01.%03dZ", i)
	}
	f(values, valueTypeTimestampISO8601, 1303184641000000000, 1303184641008000000)
}

func TestTryParseIPv4String_Success(t *testing.T) {
	f := func(s string) {
		t.Helper()

		n, ok := tryParseIPv4(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		data := marshalIPv4String(nil, n)
		if string(data) != s {
			t.Fatalf("unexpected ip; got %q; want %q", data, s)
		}
	}

	f("0.0.0.0")
	f("1.2.3.4")
	f("255.255.255.255")
	f("127.0.0.1")
}

func TestTryParseIPv4_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseIPv4(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	f("")
	f("foo")
	f("a.b.c.d")
	f("127.0.0.x")
	f("127.0.x.0")
	f("127.x.0.0")
	f("x.0.0.0")

	// Too big octets
	f("127.127.127.256")
	f("127.127.256.127")
	f("127.256.127.127")
	f("256.127.127.127")

	// Negative octets
	f("-1.127.127.127")
	f("127.-1.127.127")
	f("127.127.-1.127")
	f("127.127.127.-1")
}

func TestTryParseTimestampRFC3339NanoString_Success(t *testing.T) {
	f := func(s, timestampExpected string) {
		t.Helper()

		nsecs, ok := TryParseTimestampRFC3339Nano(s)
		if !ok {
			t.Fatalf("cannot parse timestamp %q", s)
		}
		timestamp := marshalTimestampRFC3339NanoString(nil, nsecs)
		if string(timestamp) != timestampExpected {
			t.Fatalf("unexpected timestamp; got %q; want %q", timestamp, timestampExpected)
		}
	}

	// No fractional seconds
	f("2023-01-15T23:45:51Z", "2023-01-15T23:45:51Z")

	// Different number of fractional seconds
	f("2023-01-15T23:45:51.1Z", "2023-01-15T23:45:51.1Z")
	f("2023-01-15T23:45:51.12Z", "2023-01-15T23:45:51.12Z")
	f("2023-01-15T23:45:51.123Z", "2023-01-15T23:45:51.123Z")
	f("2023-01-15T23:45:51.1234Z", "2023-01-15T23:45:51.1234Z")
	f("2023-01-15T23:45:51.12345Z", "2023-01-15T23:45:51.12345Z")
	f("2023-01-15T23:45:51.123456Z", "2023-01-15T23:45:51.123456Z")
	f("2023-01-15T23:45:51.1234567Z", "2023-01-15T23:45:51.1234567Z")
	f("2023-01-15T23:45:51.12345678Z", "2023-01-15T23:45:51.12345678Z")
	f("2023-01-15T23:45:51.123456789Z", "2023-01-15T23:45:51.123456789Z")

	// The minimum possible timestamp
	f("1677-09-21T00:12:44Z", "1677-09-21T00:12:44Z")

	// The maximum possible timestamp
	f("2262-04-11T23:47:15.999999999Z", "2262-04-11T23:47:15.999999999Z")

	// timestamp with timezone
	f("2023-01-16T00:45:51+01:00", "2023-01-15T23:45:51Z")
	f("2023-01-16T00:45:51.123-01:00", "2023-01-16T01:45:51.123Z")

	// SQL datetime format
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6721
	f("2023-01-16 00:45:51+01:00", "2023-01-15T23:45:51Z")
	f("2023-01-16 00:45:51.123-01:00", "2023-01-16T01:45:51.123Z")
}

func TestTryParseTimestampRFC3339Nano_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		_, ok := TryParseTimestampRFC3339Nano(s)
		if ok {
			t.Fatalf("expecting faulure when parsing %q", s)
		}
	}

	// invalid length
	f("")
	f("foobar")

	// missing fractional part after dot
	f("2023-01-15T22:15:51.Z")

	// too small year
	f("1676-09-21T00:12:43Z")

	// too big year
	f("2263-04-11T23:47:17Z")

	// too small timestamp
	f("1677-09-21T00:12:43.999999999Z")

	// too big timestamp
	f("2262-04-11T23:47:16Z")

	// invalid year
	f("YYYY-04-11T23:47:17Z")

	// invalid moth
	f("2023-MM-11T23:47:17Z")

	// invalid day
	f("2023-01-DDT23:47:17Z")

	// invalid hour
	f("2023-01-23Thh:47:17Z")

	// invalid minute
	f("2023-01-23T23:mm:17Z")

	// invalid second
	f("2023-01-23T23:33:ssZ")
}

func TestTryParseTimestampISO8601String_Success(t *testing.T) {
	f := func(s string) {
		t.Helper()
		nsecs, ok := tryParseTimestampISO8601(s)
		if !ok {
			t.Fatalf("cannot parse timestamp %q", s)
		}
		data := marshalTimestampISO8601String(nil, nsecs)
		if string(data) != s {
			t.Fatalf("unexpected timestamp; got %q; want %q", data, s)
		}
	}

	// regular timestamp
	f("2023-01-15T23:45:51.123Z")

	// The minimum possible timestamp
	f("1677-09-21T00:12:44.000Z")

	// The maximum possible timestamp
	f("2262-04-11T23:47:15.999Z")
}

func TestTryParseTimestampISO8601_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		_, ok := tryParseTimestampISO8601(s)
		if ok {
			t.Fatalf("expecting faulure when parsing %q", s)
		}
	}

	// invalid length
	f("")
	f("foobar")

	// Missing Z at the end
	f("2023-01-15T22:15:51.123")
	f("2023-01-15T22:15:51.1234")

	// timestamp with timezone
	f("2023-01-16T00:45:51.123+01:00")

	// too small year
	f("1676-09-21T00:12:43.434Z")

	// too big year
	f("2263-04-11T23:47:17.434Z")

	// too small timestamp
	f("1677-09-21T00:12:43.999Z")

	// too big timestamp
	f("2262-04-11T23:47:16.000Z")

	// invalid year
	f("YYYY-04-11T23:47:17.123Z")

	// invalid moth
	f("2023-MM-11T23:47:17.123Z")

	// invalid day
	f("2023-01-DDT23:47:17.123Z")

	// invalid hour
	f("2023-01-23Thh:47:17.123Z")

	// invalid minute
	f("2023-01-23T23:mm:17.123Z")

	// invalid second
	f("2023-01-23T23:33:ss.123Z")
}

func TestTryParseDuration_Success(t *testing.T) {
	f := func(s string, nsecsExpected int64) {
		t.Helper()

		nsecs, ok := tryParseDuration(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if nsecs != nsecsExpected {
			t.Fatalf("unexpected value; got %d; want %d", nsecs, nsecsExpected)
		}
	}

	// zero duration
	f("0s", 0)
	f("0.0w0d0h0s0.0ms", 0)
	f("-0w", 0)

	// positive duration
	f("1s", nsecsPerSecond)
	f("1.5ms", 1.5*nsecsPerMillisecond)
	f("1µs", nsecsPerMicrosecond)
	f("1ns", 1)
	f("1h", nsecsPerHour)
	f("1.5d", 1.5*nsecsPerDay)
	f("1.5w", 1.5*nsecsPerWeek)
	f("2.5y", 2.5*nsecsPerYear)
	f("1m5.123456789s", nsecsPerMinute+5.123456789*nsecsPerSecond)

	// composite duration
	f("1h5m", nsecsPerHour+5*nsecsPerMinute)
	f("1.1h5m2.5s3_456ns", 1.1*nsecsPerHour+5*nsecsPerMinute+2.5*nsecsPerSecond+3456)

	// nedgative duration
	f("-1h5m3s", -(nsecsPerHour + 5*nsecsPerMinute + 3*nsecsPerSecond))
}

func TestTryParseDuration_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseDuration(s)
		if ok {
			t.Fatalf("expecting error for parsing %q", s)
		}
	}

	// empty string
	f("")

	// missing suffix
	f("2")
	f("2.5")

	// invalid string
	f("foobar")
	f("1foo")
	f("1soo")
	f("3.43e")
	f("3.43es")

	// superflouous space
	f(" 2s")
	f("2s ")
	f("2s 3ms")
}

func TestMarshalDurationString(t *testing.T) {
	f := func(nsecs int64, resultExpected string) {
		t.Helper()

		result := marshalDurationString(nil, nsecs)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f(0, "0")
	f(1, "1ns")
	f(-1, "-1ns")
	f(12345, "12µs345ns")
	f(123456789, "123ms456µs789ns")
	f(12345678901, "12.345678901s")
	f(1234567890143, "20m34.567890143s")
	f(1234567890123457, "2w6h56m7.890123457s")
}

func TestTryParseBytes_Success(t *testing.T) {
	f := func(s string, resultExpected int64) {
		t.Helper()

		result, ok := tryParseBytes(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %d; want %d", result, resultExpected)
		}
	}

	f("1_500", 1_500)

	f("2.5B", 2)

	f("1.5K", 1_500)
	f("1.5M", 1_500_000)
	f("1.5G", 1_500_000_000)
	f("1.5T", 1_500_000_000_000)

	f("1.5KB", 1_500)
	f("1.5MB", 1_500_000)
	f("1.5GB", 1_500_000_000)
	f("1.5TB", 1_500_000_000_000)

	f("1.5Ki", 1.5*(1<<10))
	f("1.5Mi", 1.5*(1<<20))
	f("1.5Gi", 1.5*(1<<30))
	f("1.5Ti", 1.5*(1<<40))

	f("1.5KiB", 1.5*(1<<10))
	f("1.5MiB", 1.5*(1<<20))
	f("1.5GiB", 1.5*(1<<30))
	f("1.5TiB", 1.5*(1<<40))

	f("1MiB500KiB200B", (1<<20)+500*(1<<10)+200)
}

func TestTryParseBytes_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseBytes(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	// empty string
	f("")

	// invalid number
	f("foobar")

	// invalid suffix
	f("123q")
	f("123qs")
	f("123qsb")
	f("123sqsb")
	f("123s5qsb")

	// invalid case for the suffix
	f("1b")

	f("1k")
	f("1m")
	f("1g")
	f("1t")

	f("1kb")
	f("1mb")
	f("1gb")
	f("1tb")

	f("1ki")
	f("1mi")
	f("1gi")
	f("1ti")

	f("1kib")
	f("1mib")
	f("1gib")
	f("1tib")

	f("1KIB")
	f("1MIB")
	f("1GIB")
	f("1TIB")

	// fractional number without suffix
	f("123.456")
}

func TestTryParseFloat64_Success(t *testing.T) {
	f := func(s string, resultExpected float64) {
		t.Helper()

		result, ok := tryParseFloat64(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if !float64Equal(result, resultExpected) {
			t.Fatalf("unexpected value; got %f; want %f", result, resultExpected)
		}
	}

	f("0", 0)
	f("1", 1)
	f("-1", -1)
	f("1234567890", 1234567890)
	f("1_234_567_890", 1234567890)
	f("-1.234_567", -1.234567)

	f("0.345", 0.345)
	f("-0.345", -0.345)
}

func float64Equal(a, b float64) bool {
	return math.Abs(a-b)*math.Abs(max(a, b)) < 1e-15
}

func TestTryParseFloat64_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseFloat64(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	// Empty value
	f("")

	// Plus in the value isn't allowed, since it cannot be convered back to the same string representation
	f("+123")

	// Dot at the beginning and the end of value isn't allowed, since it cannot converted back to the same string representation
	f(".123")
	f("123.")

	// Multiple dots aren't allowed
	f("123.434.55")

	// Invalid dots
	f("-.123")
	f(".")

	// Scientific notation isn't allowed, since it cannot be converted back to the same string representation
	f("12e5")

	// Minus in the middle of string isn't allowed
	f("12-5")
}

func TestMarshalFloat64String(t *testing.T) {
	f := func(f float64, resultExpected string) {
		t.Helper()

		result := marshalFloat64String(nil, f)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f(0, "0")
	f(1234, "1234")
	f(-12345678, "-12345678")
	f(1.234, "1.234")
	f(-1.234567, "-1.234567")
}

func TestTryParseUint64_Success(t *testing.T) {
	f := func(s string, resultExpected uint64) {
		t.Helper()

		result, ok := tryParseUint64(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if result != resultExpected {
			t.Fatalf("unexpected value; got %d; want %d", result, resultExpected)
		}
	}

	f("0", 0)
	f("123", 123)
	f("123456", 123456)
	f("123456789", 123456789)
	f("123456789012", 123456789012)
	f("123456789012345", 123456789012345)
	f("123456789012345678", 123456789012345678)
	f("12345678901234567890", 12345678901234567890)
	f("12_345_678_901_234_567_890", 12345678901234567890)

	// the maximum possible value
	f("18446744073709551615", 18446744073709551615)
}

func TestTryParseUint64_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseUint64(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	// empty value
	f("")

	// too big value
	f("18446744073709551616")

	// invalid value
	f("foo")
}

func TestMarshalUint8String(t *testing.T) {
	f := func(n uint8, resultExpected string) {
		t.Helper()

		result := marshalUint8String(nil, n)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	for i := 0; i < 256; i++ {
		resultExpected := strconv.Itoa(i)
		f(uint8(i), resultExpected)
	}

	// the maximum possible value
	f(math.MaxUint8, "255")
}

func TestMarshalUint16String(t *testing.T) {
	f := func(n uint16, resultExpected string) {
		t.Helper()

		result := marshalUint16String(nil, n)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f(0, "0")
	f(1, "1")
	f(10, "10")
	f(12, "12")
	f(120, "120")
	f(1203, "1203")
	f(12345, "12345")

	// the maximum possible value
	f(math.MaxUint16, "65535")
}

func TestMarshalUint32String(t *testing.T) {
	f := func(n uint32, resultExpected string) {
		t.Helper()

		result := marshalUint32String(nil, n)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f(0, "0")
	f(1, "1")
	f(10, "10")
	f(12, "12")
	f(120, "120")
	f(1203, "1203")
	f(12034, "12034")
	f(123456, "123456")
	f(1234567, "1234567")
	f(12345678, "12345678")
	f(123456789, "123456789")
	f(1234567890, "1234567890")

	// the maximum possible value
	f(math.MaxUint32, "4294967295")
}

func TestMarshalUint64String(t *testing.T) {
	f := func(n uint64, resultExpected string) {
		t.Helper()

		result := marshalUint64String(nil, n)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f(0, "0")
	f(123456, "123456")

	// the maximum possible value
	f(math.MaxUint64, "18446744073709551615")
}

func TestTryParseIPv4Mask_Success(t *testing.T) {
	f := func(s string, resultExpected uint64) {
		t.Helper()

		result, ok := tryParseIPv4Mask(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %d; want %d", result, resultExpected)
		}
	}

	f("/0", 1<<32)
	f("/1", 1<<31)
	f("/8", 1<<24)
	f("/24", 1<<8)
	f("/32", 1)
}

func TestTryParseIPv4Mask_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseIPv4Mask(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	// Empty mask
	f("")

	// Invalid prefix
	f("foo")

	// Non-numeric mask
	f("/foo")

	// Too big mask
	f("/33")

	// Negative mask
	f("/-1")
}
