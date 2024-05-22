package httputils

import (
	"testing"
)

func TestIsTCPv4Addr(t *testing.T) {
	f := func(addr string, resultExpected bool) {
		t.Helper()
		result := isTCPv4Addr(addr)
		if result != resultExpected {
			t.Fatalf("unexpected result for isIPv4Addr(%q); got %v; want %v", addr, result, resultExpected)
		}
	}

	// empty addr
	f("", false)

	// too small number of octets
	f("foobar", false)
	f("1", false)
	f("1.2", false)
	f("1.2.3", false)
	f("1.2.3.", false)

	// non-numeric octets
	f("foo.bar.baz.aaa", false)

	// non-numeric last value
	f("1.2.3.foo", false)

	// negative value
	f("1.2.3.-4", false)

	// missing port
	f("1.2.3.4", false)

	// invalid port
	f("1.2.3.4:foo", false)

	// too big octet
	f("1.2.3.444:5", false)

	// too big port
	f("1.2.3.4:152344", false)

	// normal TCPv4 addr
	f("1.2.3.4:5", true)
	f("0.0.0.0:80", true)
	f("1.2.3.4:65535", true)
}
