package flagutil

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestParseJSONMapSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		m, err := ParseJSONMap(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if s == "" && m == nil {
			return
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("cannot marshal m: %s", err)
		}
		if s != string(data) {
			t.Fatalf("unexpected result; got %s; want %s", data, s)
		}
	}

	f("")
	f("{}")
	f(`{"foo":"bar"}`)
	f(`{"a":"b","c":"d"}`)
}

func TestParseJSONMapFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		m, err := ParseJSONMap(s)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if m != nil {
			t.Fatalf("expecting nil m")
		}
	}

	f("foo")
	f("123")
	f("{")
	f(`{foo:bar}`)
	f(`{"foo":1}`)
	f(`[]`)
	f(`{"foo":"bar","a":[123]}`)
}

func TestDictFlagSetSuccess(t *testing.T) {
	var idx int
	f := func(s string) {
		t.Helper()
		name := fmt.Sprintf("dict-flag-set-success-%d", idx)
		idx++
		df := NewDictValue(name, 0, ':', "test")
		if err := df.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := df.String()
		if result != s {
			t.Fatalf("unexpected DictFlag.String(); got %q; want %q", result, s)
		}
	}

	f("123")
	f("-234")
	f("foo:123")
	f("foo:123,bar:-42,baz:0,aa:43")
}

func TestDictFlagFailure(t *testing.T) {
	var idx int
	f := func(s string) {
		t.Helper()
		name := fmt.Sprintf("dict-flag-failure-%d", idx)
		idx++
		df := NewDictValue(name, []int{}, ':', "test")
		if err := df.Set(s); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// missing values
	f("foo")
	f("foo:")

	// non-integer values
	f("foo:bar")
}

func TestDictIntGet(t *testing.T) {
	var idx int
	f := func(s, key string, defaultValue int, expectedValue int) {
		t.Helper()
		name := fmt.Sprintf("dict-int-get-%d", idx)
		idx++
		df := NewDictValue(name, defaultValue, ':', "test")
		if err := df.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		value := df.GetOptionalArg(key, 0)
		if value != expectedValue {
			t.Fatalf("unexpected value; got %d; want %d", value, expectedValue)
		}
	}

	f("foo:42", "", 123, 123)
	f("foo:42", "foo", 123, 42)
	f("532", "", 123, 532)
	f("532", "foo", 123, 123)
}

func TestDictStringGet(t *testing.T) {
	var idx int
	f := func(s, key string, defaultValue string, expectedValue string) {
		t.Helper()
		name := fmt.Sprintf("dict-string-get-%d", idx)
		idx++
		df := NewDictValue(name, defaultValue, ':', "test")
		if err := df.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		value := df.GetOptionalArg(key, 0)
		if value != expectedValue {
			t.Fatalf("unexpected value; got %s; want %s", value, expectedValue)
		}
	}

	f("foo:value", "", "default", "default")
	f("foo:value", "foo", "default", "value")
	f("value", "", "default", "value")
	f("value", "foo", "default", "default")
}

func TestDictBoolGet(t *testing.T) {
	var idx int
	f := func(s, key string, defaultValue bool, expectedValue bool) {
		t.Helper()
		name := fmt.Sprintf("dict-bool-get-%d", idx)
		idx++
		df := NewDictValue(name, defaultValue, ':', "test")
		if err := df.Set(s); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		value := df.GetOptionalArg(key, 0)
		if value != expectedValue {
			t.Fatalf("unexpected value; got %t; want %t", value, expectedValue)
		}
	}

	f("foo:true", "", false, false)
	f("foo:true", "foo", false, true)
	f("true", "", false, true)
	f("true", "foo", false, false)
}
