package httpserver

import (
	"net"
	"path/filepath"
	"testing"
)

func TestCheckHealth(t *testing.T) {
	// listen returns the address of a new listener with an empty host such as ":8428", like -httpListenAddr usually has.
	listen := func() (net.Listener, string) {
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("cannot listen: %s", err)
		}
		_, port, _ := net.SplitHostPort(ln.Addr().String())
		return ln, ":" + port
	}

	ln, addr := listen()
	defer ln.Close()

	lnClosed, closedAddr := listen()
	_ = lnClosed.Close()

	lnUnix, err := net.Listen("unix", filepath.Join(t.TempDir(), "http.sock"))
	if err != nil {
		t.Fatalf("cannot listen on unix socket: %s", err)
	}
	defer lnUnix.Close()
	unixAddr := "unix:" + lnUnix.Addr().String()

	f := func(addrs []string, resultExpected bool) {
		t.Helper()

		err := checkHealthAddrs(addrs)
		if result := err == nil; result != resultExpected {
			t.Fatalf("unexpected result for addrs %q; got %v; want %v; error: %v", addrs, result, resultExpected, err)
		}
	}

	f([]string{unixAddr}, true)
	f([]string{closedAddr}, false)

	// a single available address is enough
	f([]string{closedAddr, addr}, true)
}
