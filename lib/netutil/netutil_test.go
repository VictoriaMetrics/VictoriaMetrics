package netutil

import (
	"net"
	"testing"
)

func TestIsErrMissingPort(t *testing.T) {
	f := func(addr string, expected bool) {
		_, _, err := net.SplitHostPort(addr)
		if IsErrMissingPort(err) != expected {
			t.Fatalf("unexpected result for %q; got %v; want %v", addr, !expected, expected)
		}
	}

	f("127.0.0.1", true)
	f("http://127.0.0.1:8080", false)
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

	// Invalid address format
	f("http://127.0.0.1")
	f("http://127.0.0.1:80")
}
