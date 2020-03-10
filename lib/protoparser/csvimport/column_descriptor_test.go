package csvimport

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"
	"unsafe"
)

func TestParseColumnDescriptorsSuccess(t *testing.T) {
	f := func(s string, cdsExpected []ColumnDescriptor) {
		t.Helper()
		cds, err := ParseColumnDescriptors(s)
		if err != nil {
			t.Fatalf("unexpected error on ParseColumnDescriptors(%q): %s", s, err)
		}
		if !equalColumnDescriptors(cds, cdsExpected) {
			t.Fatalf("unexpected cds returned from ParseColumnDescriptors(%q);\ngot\n%v\nwant\n%v", s, cds, cdsExpected)
		}
	}
	f("1:time:unix_s,3:metric:temperature", []ColumnDescriptor{
		{
			ParseTimestamp: parseUnixTimestampSeconds,
		},
		{},
		{
			MetricName: "temperature",
		},
	})
	f("2:time:unix_ns,1:metric:temperature,3:label:city,4:label:country", []ColumnDescriptor{
		{
			MetricName: "temperature",
		},
		{
			ParseTimestamp: parseUnixTimestampNanoseconds,
		},
		{
			TagName: "city",
		},
		{
			TagName: "country",
		},
	})
	f("2:time:unix_ms,1:metric:temperature", []ColumnDescriptor{
		{
			MetricName: "temperature",
		},
		{
			ParseTimestamp: parseUnixTimestampMilliseconds,
		},
	})
	f("2:time:rfc3339,1:metric:temperature", []ColumnDescriptor{
		{
			MetricName: "temperature",
		},
		{
			ParseTimestamp: parseRFC3339,
		},
	})
}

func TestParseColumnDescriptorsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		cds, err := ParseColumnDescriptors(s)
		if err == nil {
			t.Fatalf("expecting non-nil error for ParseColumnDescriptors(%q)", s)
		}
		if cds != nil {
			t.Fatalf("expecting nil cds; got %v", cds)
		}
	}
	// Empty string
	f("")

	// Missing metric column
	f("1:time:unix_s")
	f("1:label:aaa")

	// Invalid column number
	f("foo:time:unix_s,bar:metric:temp")
	f("0:metric:aaa")
	f("-123:metric:aaa")
	f(fmt.Sprintf("%d:metric:aaa", maxColumnsPerRow+10))

	// Duplicate time column
	f("1:time:unix_s,2:time:rfc3339,3:metric:aaa")
	f("1:time:custom:2006,2:time:rfc3339,3:metric:aaa")

	// Invalid time format
	f("1:time:foobar,2:metric:aaa")
	f("1:time:,2:metric:aaa")
	f("1:time:sss:sss,2:metric:aaa")

	// empty label name
	f("2:label:,1:metric:aaa")

	// Empty metric name
	f("1:metric:")

	// Unknown type
	f("1:metric:aaa,2:aaaa:bbb")

	// duplicate column number
	f("1:metric:a,1:metric:b")
}

func TestParseUnixTimestampSeconds(t *testing.T) {
	f := func(s string, tsExpected int64) {
		t.Helper()
		ts, err := parseUnixTimestampSeconds(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if ts != tsExpected {
			t.Fatalf("unexpected ts when parsing %q; got %d; want %d", s, ts, tsExpected)
		}
	}
	f("0", 0)
	f("123", 123000)
	f("-123", -123000)
}

func TestParseUnixTimestampMilliseconds(t *testing.T) {
	f := func(s string, tsExpected int64) {
		t.Helper()
		ts, err := parseUnixTimestampMilliseconds(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if ts != tsExpected {
			t.Fatalf("unexpected ts when parsing %q; got %d; want %d", s, ts, tsExpected)
		}
	}
	f("0", 0)
	f("123", 123)
	f("-123", -123)
}

func TestParseUnixTimestampNanoseconds(t *testing.T) {
	f := func(s string, tsExpected int64) {
		t.Helper()
		ts, err := parseUnixTimestampNanoseconds(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if ts != tsExpected {
			t.Fatalf("unexpected ts when parsing %q; got %d; want %d", s, ts, tsExpected)
		}
	}
	f("0", 0)
	f("123", 0)
	f("12343567", 12)
	f("-12343567", -12)
}

func TestParseRFC3339(t *testing.T) {
	f := func(s string, tsExpected int64) {
		t.Helper()
		ts, err := parseRFC3339(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if ts != tsExpected {
			t.Fatalf("unexpected ts when parsing %q; got %d; want %d", s, ts, tsExpected)
		}
	}
	f("2006-01-02T15:04:05Z", 1136214245000)
	f("2020-03-11T18:23:46Z", 1583951026000)
}

func TestParseCustomTimeFunc(t *testing.T) {
	f := func(format, s string, tsExpected int64) {
		t.Helper()
		f := newParseCustomTimeFunc(format)
		ts, err := f(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if ts != tsExpected {
			t.Fatalf("unexpected ts when parsing %q; got %d; want %d", s, ts, tsExpected)
		}
	}
	f(time.RFC1123, "Mon, 29 Oct 2018 07:50:37 GMT", 1540799437000)
	f("2006-01-02 15:04:05.999Z", "2015-08-10 20:04:40.123Z", 1439237080123)
}

func equalColumnDescriptors(a, b []ColumnDescriptor) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		y := b[i]
		if !equalColumnDescriptor(x, y) {
			return false
		}
	}
	return true
}

func equalColumnDescriptor(x, y ColumnDescriptor) bool {
	sh1 := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&x.ParseTimestamp)),
		Len:  int(unsafe.Sizeof(x.ParseTimestamp)),
		Cap:  int(unsafe.Sizeof(x.ParseTimestamp)),
	}
	b1 := *(*[]byte)(unsafe.Pointer(sh1))
	sh2 := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&y.ParseTimestamp)),
		Len:  int(unsafe.Sizeof(y.ParseTimestamp)),
		Cap:  int(unsafe.Sizeof(y.ParseTimestamp)),
	}
	b2 := *(*[]byte)(unsafe.Pointer(sh2))
	if !bytes.Equal(b1, b2) {
		return false
	}
	if x.TagName != y.TagName {
		return false
	}
	if x.MetricName != y.MetricName {
		return false
	}
	return true
}
