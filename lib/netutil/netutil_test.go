package netutil

import (
	"net"
	"testing"
)

func TestIsErrMissingPort(t *testing.T) {
	f := func(addr string, expected bool) {
		t.Helper()
		_, _, err := net.SplitHostPort(addr)
		if IsErrMissingPort(err) != expected {
			t.Fatalf("unexpected result for %q; got %v; want %v", addr, !expected, expected)
		}
	}

	f("127.0.0.1", true)
	f("http://127.0.0.1:8080", false)

	f("[::1]", true)
	f("http://[::1]:8080", false)

	f("vmstorage-0", true)
	f("vmstorage-0.svc.cluster.local.", true)
	f("http://vmstorage-0:8080", false)
	f("http://vmstorage-0.svc.cluster.local.:8080", false)
}

func TestIsLonePort(t *testing.T) {
	f := func(s string, expected bool) {
		t.Helper()
		if isLonePort(s) != expected {
			t.Fatalf("unexpected result for %q; got %v; want %v", s, !expected, expected)
		}
	}

	f(":0", true)
	f(":1", true)
	f(":80", true)
	f(":443", true)
	f(":666", true)
	f(":6969", true)
	f(":8080", true)
	f(":65535", true)

	f(":65536", false)
	f(":99999", false)

	f("80", false)
	f("8080", false)

	f("", false)
	f(":", false)

	f(":123456", false)

	f(":abc", false)
	f(":80a", false)
	f(":12.3", false)
	f(":-1", false)

	f("127.0.0.1:80", false)
	f("[::1]:80", false)

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
	f("1.2.3.4:5", false)
	f("0.0.0.0:80", false)
	f("1.2.3.4:65535", false)
}

func TestNormalizeAddrSuccess(t *testing.T) {
	f := func(addr string, defaultPort int, expected string) {
		t.Helper()
		result, err := NormalizeAddr(addr, defaultPort)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", addr, err)
		}
		if result != expected {
			t.Fatalf("unexpected result for %q; got %q; want %q", addr, result, expected)
		}
	}

	f("127.0.0.1", 80, "127.0.0.1:80")
	f("127.0.0.1:123", 80, "127.0.0.1:123")
	f("[::1]", 80, "[::1]:80")
	f("[::1]:123", 80, "[::1]:123")
	f("vmstorage-0.svc.cluster.local.", 80, "vmstorage-0.svc.cluster.local.:80")
	f("vmstorage-0.svc.cluster.local.:123", 80, "vmstorage-0.svc.cluster.local.:123")
}

func TestNormalizeAddrError(t *testing.T) {
	f := func(addr string) {
		t.Helper()
		_, err := NormalizeAddr(addr, 80)
		if err == nil {
			t.Fatalf("expected error for %q, but got none", addr)
		}
	}

	// Invalid number of octets in address
	f("1:2:3:4:5:6:7:8:9:10")
	// Invalid IPv6 address format
	f("::1:2:3:4:5:6:7:8:9")

	// Invalid address format
	f("http://127.0.0.1")
	f("http://127.0.0.1:80")
	f("http://vmstorage-0.svc.cluster.local.")
	f("http://vmstorage-0.svc.cluster.local.:80")
	f("/vmstorage-0.svc.cluster.local.:80")
}
