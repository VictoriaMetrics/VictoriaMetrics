package netutil

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

	// IPv6 addresses
	f("[::1]:80", false)
	f("[::]:443", false)
	f("[fe80::1]:8080", false)
	f("[fe80::1]:8080", false)
	f("[2001:db8::1]:9090", false)
	f("2606:4700:4700::1111:80", false)
	f("[fd00::1:2:3]:8428", false)

	// domain names
	f("localhost:80", false)
	f("example.com:443", false)
	f("victoriametrics.com", false)
	f("zombo.com", false)
	f("theonion.com", false)
	f("vmstorage-0:8401", false)
	f("vmstorage-0.svc.cluster.local.:8401", false)
	f("pee-poo.internal:9090", false)

	// full URL
	f("https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@300;400;500;700&family=Space+Grotesk:wght@400;600;700&display=swap", false)

	// random strings
	f("abc", false)
	f("my dog got autism", false)
	f("IDDQD", false)
	f("Galactus", false)
	f("hello:world", false)
	f("12:34:56", false)

	// normal TCPv4 addr
	f("1.2.3.4:5", true)
	f("0.0.0.0:80", true)
	f("1.2.3.4:65535", true)
}
