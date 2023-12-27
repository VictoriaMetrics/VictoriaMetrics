package netutil

import (
	"testing"
)

func TestParseGroupAddr(t *testing.T) {
	f := func(s, groupIDExpected, addrExpected string) {
		t.Helper()

		groupID, addr := ParseGroupAddr(s)
		if groupID != groupIDExpected {
			t.Fatalf("unexpected groupID; got %q; want %q", groupID, groupIDExpected)
		}
		if addr != addrExpected {
			t.Fatalf("unexpected addr; got %q; want %q", addr, addrExpected)
		}
	}

	f("", "", "")
	f("foo", "", "foo")
	f("file:/foo/bar", "", "file:/foo/bar")
	f("foo/bar", "foo", "bar")
	f("foo/dns+srv:bar", "foo", "dns+srv:bar")
	f("foo/file:/bar/baz", "foo", "file:/bar/baz")
}
