package flagutil

import (
	"testing"
)

func TestBytesSetFailure(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var b Bytes
		if err := b.Set(value); err == nil {
			t.Fatalf("expecting non-nil error in b.Set(%q)", value)
		}
	}
	f("foobar")
	f("5foobar")
	f("aKB")
	f("134xMB")
	f("2.43sdfGb")
	f("aKiB")
	f("134xMiB")
	f("2.43sdfGIb")
}

func TestBytesSetSuccess(t *testing.T) {
	f := func(value string, expectedResult int64) {
		t.Helper()
		var b Bytes
		if err := b.Set(value); err != nil {
			t.Fatalf("unexpected error in b.Set(%q): %s", value, err)
		}
		if b.N != expectedResult {
			t.Fatalf("unexpected result; got %d; want %d", b.N, expectedResult)
		}
		valueString := b.String()
		valueExpected := normalizeBytesString(value)
		if valueString != valueExpected {
			t.Fatalf("unexpected valueString; got %q; want %q", valueString, valueExpected)
		}
	}
	f("", 0)
	f("0", 0)
	f("1", 1)
	f("-1234", -1234)
	f("123.456", 123)
	f("1KiB", 1024)
	f("1.5kib", 1.5*1024)
	f("23MiB", 23*1024*1024)
	f("0.25GiB", 0.25*1024*1024*1024)
	f("1.25TiB", 1.25*1024*1024*1024*1024)
	f("1KB", 1000)
	f("1.5kb", 1.5*1000)
	f("23MB", 23*1000*1000)
	f("0.25GB", 0.25*1000*1000*1000)
	f("1.25TB", 1.25*1000*1000*1000*1000)
}
