package flagutil

import (
	"testing"
)

func TestDictIntSetSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var di DictInt
		if err := di.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := di.String()
		if result != s {
			t.Fatalf("unexpected DictInt.String(); got %q; want %q", result, s)
		}
	}

	f("")
	f("123")
	f("-234")
	f("foo:123")
	f("foo:123,bar:-42,baz:0,aa:43")
}

func TestDictIntFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var di DictInt
		if err := di.Set(s); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// missing values
	f("foo")
	f("foo:")

	// non-integer values
	f("foo:bar")
	f("12.34")
	f("foo:123.34")

	// duplicate keys
	f("a:234,k:123,k:432")
}

func TestDictIntGet(t *testing.T) {
	f := func(s, key string, defaultValue, expectedValue int) {
		t.Helper()
		var di DictInt
		di.defaultValue = defaultValue
		if err := di.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		value := di.Get(key)
		if value != expectedValue {
			t.Fatalf("unexpected value; got %d; want %d", value, expectedValue)
		}
	}

	f("", "", 123, 123)
	f("", "foo", 123, 123)
	f("foo:42", "", 123, 123)
	f("foo:42", "foo", 123, 42)
	f("532", "", 123, 532)
	f("532", "foo", 123, 123)
}
