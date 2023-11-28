package datadog

import (
	"testing"
)

func TestSplitTag(t *testing.T) {
	f := func(s, nameExpected, valueExpected string) {
		t.Helper()
		name, value := SplitTag(s)
		if name != nameExpected {
			t.Fatalf("unexpected name obtained from %q; got %q; want %q", s, name, nameExpected)
		}
		if value != valueExpected {
			t.Fatalf("unexpected value obtained from %q; got %q; want %q", s, value, valueExpected)
		}
	}
	f("", "", "no_label_value")
	f("foo", "foo", "no_label_value")
	f("foo:bar", "foo", "bar")
	f(":bar", "", "bar")
}
