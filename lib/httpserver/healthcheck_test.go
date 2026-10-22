package httpserver

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

func TestCheckHealth(t *testing.T) {
	// serve starts a server returning statusCode at /health on ln (or on a random local port if ln is nil).
	// It returns the server address with an empty host such as ":8428", like -httpListenAddr usually has.
	serve := func(ln net.Listener, statusCode int) string {
		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(statusCode)
		}))
		if ln != nil {
			_ = s.Listener.Close()
			s.Listener = ln
		}
		s.Start()
		t.Cleanup(s.Close)

		if ln != nil && ln.Addr().Network() == "unix" {
			return "unix:" + ln.Addr().String()
		}
		_, port, _ := net.SplitHostPort(s.Listener.Addr().String())
		return ":" + port
	}

	lnUnix, err := net.Listen("unix", filepath.Join(t.TempDir(), "http.sock"))
	if err != nil {
		t.Fatalf("cannot listen on unix socket: %s", err)
	}
	// NewTCPListener registers metrics per address, so use a free port instead of "127.0.0.1:0" in order to run the test multiple times.
	sTmp := httptest.NewServer(nil)
	sTmp.Close()
	lnProxy, err := netutil.NewTCPListener("healthcheckTest", sTmp.Listener.Addr().String(), true, nil)
	if err != nil {
		t.Fatalf("cannot listen with proxy protocol: %s", err)
	}
	healthyAddr := serve(nil, http.StatusOK)
	unhealthyAddr := serve(nil, http.StatusServiceUnavailable)
	unixAddr := serve(lnUnix, http.StatusOK)
	proxyAddr := serve(lnProxy, http.StatusOK)

	f := func(addrs []string, useProxyProtocol flagutil.ArrayBool, resultExpected bool) {
		t.Helper()

		err := checkHealthAddrs(addrs, ServeOptions{UseProxyProtocol: &useProxyProtocol})
		if result := err == nil; result != resultExpected {
			t.Fatalf("unexpected result for addrs %q; got %v; want %v; error: %v", addrs, result, resultExpected, err)
		}
	}

	f([]string{unhealthyAddr}, nil, false)
	f([]string{unixAddr}, nil, true)
	f([]string{proxyAddr}, flagutil.ArrayBool{true}, true)

	// a single healthy address is enough
	f([]string{unhealthyAddr, healthyAddr}, nil, true)
}
