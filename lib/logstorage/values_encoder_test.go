package logstorage

import (
	"fmt"
	"math"
	"reflect"
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
		if err := vd.decodeInplace(encodedValues, vt, &dict); err != nil {
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

	// float64 values
	for i := range values {
		values[i] = fmt.Sprintf("%g", math.Sqrt(float64(i+1)))
	}
	f(values, valueTypeFloat64, 4607182418800017408, 4613937818241073152)
}

func TestTryParseIPv4(t *testing.T) {
	f := func(s string, nExpected uint32, okExpected bool) {
		t.Helper()
		n, ok := tryParseIPv4(s)
		if n != nExpected {
			t.Fatalf("unexpected n; got %d; want %d", n, nExpected)
		}
		if ok != okExpected {
			t.Fatalf("unexpected ok; got %v; want %v", ok, okExpected)
		}
	}

	f("", 0, false)
	f("foo", 0, false)
	f("a.b.c.d", 0, false)
	f("1.2.3.4", 0x01020304, true)
	f("255.255.255.255", 0xffffffff, true)
	f("0.0.0.0", 0, true)
	f("127.0.0.1", 0x7f000001, true)
	f("127.0.0.x", 0, false)
	f("127.0.x.0", 0, false)
	f("127.x.0.0", 0, false)
	f("x.0.0.0", 0, false)
	f("127.127.127.256", 0, false)
	f("127.127.256.127", 0, false)
	f("127.256.127.127", 0, false)
	f("256.127.127.127", 0, false)
	f("-1.127.127.127", 0, false)
	f("127.-1.127.127", 0, false)
	f("127.127.-1.127", 0, false)
	f("127.127.127.-1", 0, false)
}

func TestTryParseTimestampISO8601(t *testing.T) {
	f := func(s string, timestampExpected uint64, okExpected bool) {
		t.Helper()
		timestamp, ok := tryParseTimestampISO8601(s)
		if timestamp != timestampExpected {
			t.Fatalf("unexpected timestamp; got %d; want %d", timestamp, timestampExpected)
		}
		if ok != okExpected {
			t.Fatalf("unexpected ok; got %v; want %v", ok, okExpected)
		}
	}

	f("2023-01-15T23:45:51.123Z", 1673826351123000000, true)

	// Invalid milliseconds
	f("2023-01-15T22:15:51.12345Z", 0, false)
	f("2023-01-15T22:15:51.12Z", 0, false)
	f("2023-01-15T22:15:51Z", 0, false)

	// Missing Z
	f("2023-01-15T23:45:51.123", 0, false)

	// Invalid timestamp
	f("foo", 0, false)
	f("2023-01-15T23:45:51.123Zxyabcd", 0, false)
	f("2023-01-15T23:45:51.123Z01:00", 0, false)

	// timestamp with timezone
	f("2023-01-16T00:45:51.123+01:00", 0, false)
}

func TestTryParseFloat64(t *testing.T) {
	f := func(s string, valueExpected float64, okExpected bool) {
		t.Helper()

		value, ok := tryParseFloat64(s)
		if value != valueExpected {
			t.Fatalf("unexpected value; got %v; want %v", value, valueExpected)
		}
		if ok != okExpected {
			t.Fatalf("unexpected ok; got %v; want %v", ok, okExpected)
		}
	}

	f("0", 0, true)
	f("1234567890", 1234567890, true)
	f("-1.234567", -1.234567, true)

	// Empty value
	f("", 0, false)

	// Plus in the value isn't allowed, since it cannot be convered back to the same string representation
	f("+123", 0, false)

	// Dot at the beginning and the end of value isn't allowed, since it cannot converted back to the same string representation
	f(".123", 0, false)
	f("123.", 0, false)

	// Multiple dots aren't allowed
	f("123.434.55", 0, false)

	// Invalid dots
	f("-.123", 0, false)
	f(".", 0, false)

	// Scientific notation isn't allowed, since it cannot be converted back to the same string representation
	f("12e5", 0, false)

	// Minus in the middle of string isn't allowed
	f("12-5", 0, false)
}

func TestTryParseUint64(t *testing.T) {
	f := func(s string, valueExpected uint64, okExpected bool) {
		t.Helper()

		value, ok := tryParseUint64(s)
		if value != valueExpected {
			t.Fatalf("unexpected value; got %d; want %d", value, valueExpected)
		}
		if ok != okExpected {
			t.Fatalf("unexpected ok; got %v; want %v", ok, okExpected)
		}
	}

	f("0", 0, true)
	f("123456789012345678", 123456789012345678, true)

	// empty value
	f("", 0, false)

	// too big value
	f("1234567890123456789", 0, false)

	// invalid value
	f("foo", 0, false)
}
