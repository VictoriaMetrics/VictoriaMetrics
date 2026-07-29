package promscrape

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/chunkedbuffer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type connReadWriteCloser struct {
	io.Reader
	io.WriteCloser
}

func proxyTunnel(w http.ResponseWriter, r *http.Request) {
	transfer := func(src io.ReadCloser, dst io.WriteCloser) {
		defer dst.Close()
		defer src.Close()
		io.Copy(dst, src) //nolint
	}
	server, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	// For hijacked connections, one has to read from the connection buffer, but
	// still write directly to the connection.
	client := &connReadWriteCloser{clientBuf, clientConn}

	go transfer(client, server)
	transfer(server, client)
}

type testProxyServer struct {
	ba                   *promauth.BasicAuthConfig
	receivedProxyRequest bool
}

func checkBasicAuthHeader(w http.ResponseWriter, headerValue string, ba *promauth.BasicAuthConfig) bool {
	userPasswordEncoded := base64.StdEncoding.EncodeToString([]byte(ba.Username + ":" + ba.Password.String()))
	expectedAuthValue := "Basic " + userPasswordEncoded
	if headerValue != expectedAuthValue {
		w.WriteHeader(403)
		fmt.Fprintf(w, "Proxy Requires authorization got header value=%q, want=%q", headerValue, expectedAuthValue)
		return false
	}
	return true
}

func (tps *testProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tps.receivedProxyRequest = true
	if tps.ba != nil {
		if !checkBasicAuthHeader(w, r.Header.Get("Proxy-Authorization"), tps.ba) {
			return
		}
	}
	if r.Method == http.MethodConnect {
		proxyTunnel(w, r)
		return
	}

	tr := httputil.NewTransport(false, "test_client")
	resp, err := tr.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint
}

func newClientTestServer(useTLS bool, rh http.Handler) *httptest.Server {
	var s *httptest.Server
	if useTLS {
		s = httptest.NewTLSServer(rh)
	} else {
		s = httptest.NewServer(rh)
	}
	return s
}

func newTestAuthConfig(t *testing.T, isTLS bool, ba *promauth.BasicAuthConfig) *promauth.Config {
	a := promauth.Options{
		BasicAuth: ba,
	}
	if isTLS {
		a.TLSConfig = &promauth.TLSConfig{InsecureSkipVerify: true}
	}
	ac, err := a.NewConfig()
	if err != nil {
		t.Fatalf("cannot setup promauth.Config: %s", err)
	}
	return ac
}

func newUnixSocketTestServer(t *testing.T, socketPath string, useTLS bool, response string) *httptest.Server {
	t.Helper()

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("cannot listen on Unix socket %q: %s", socketPath, err)
	}
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "logical-target" {
			t.Errorf("unexpected Host header; got %q; want %q", r.Host, "logical-target")
		}
		if r.URL.RequestURI() != "/metrics?foo=bar" {
			t.Errorf("unexpected request URI; got %q; want %q", r.URL.RequestURI(), "/metrics?foo=bar")
		}
		_, _ = fmt.Fprint(w, response)
	}))
	if err := s.Listener.Close(); err != nil {
		_ = ln.Close()
		t.Fatalf("cannot close the default test listener: %s", err)
	}
	s.Listener = ln
	if useTLS {
		s.StartTLS()
	} else {
		s.Start()
	}
	t.Cleanup(s.Close)
	return s
}

func newUnixSocketTestClient(t *testing.T, socketPath string, useTLS bool) *client {
	t.Helper()

	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	c, err := newClient(context.Background(), &ScrapeWork{
		ScrapeURL:          scheme + "://logical-target/metrics?foo=bar",
		ScrapeInterval:     time.Second,
		ScrapeTimeout:      5 * time.Second,
		MaxScrapeSize:      16000,
		AuthConfig:         newTestAuthConfig(t, useTLS, nil),
		DisableCompression: true,
		UnixSocket:         socketPath,
	})
	if err != nil {
		t.Fatalf("cannot create Unix socket client: %s", err)
	}
	return c
}

func readClientData(c *client) (string, error) {
	var cb chunkedbuffer.Buffer
	_, err := c.ReadData(&cb)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(cb.NewReader())
	return string(b), err
}

func TestClientUnixSocketIgnoresDefaultProxy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets aren't supported on Windows")
	}

	defaultTransport := http.DefaultTransport.(*http.Transport)
	proxyPrev := defaultTransport.Proxy
	proxyCalls := 0
	defaultTransport.Proxy = func(_ *http.Request) (*url.URL, error) {
		proxyCalls++
		return nil, fmt.Errorf("unexpected proxy call")
	}
	t.Cleanup(func() {
		defaultTransport.Proxy = proxyPrev
	})

	for _, useTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("tls=%v", useTLS), func(t *testing.T) {
			socketPath := filepath.Join(t.TempDir(), "metrics.sock")
			newUnixSocketTestServer(t, socketPath, useTLS, "response")
			c := newUnixSocketTestClient(t, socketPath, useTLS)
			got, err := readClientData(c)
			if err != nil {
				t.Fatalf("unexpected error when reading data: %s", err)
			}
			if got != "response" {
				t.Fatalf("unexpected response; got %q; want %q", got, "response")
			}
		})
	}
	if proxyCalls != 0 {
		t.Fatalf("unexpected number of proxy calls; got %d; want 0", proxyCalls)
	}
}

func TestClientUnixSocketRecreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets aren't supported on Windows")
	}

	socketPath := filepath.Join(t.TempDir(), "metrics.sock")
	c := newUnixSocketTestClient(t, socketPath, false)
	if _, err := readClientData(c); err == nil {
		t.Fatal("expecting an error when the Unix socket doesn't exist")
	}

	s := newUnixSocketTestServer(t, socketPath, false, "first")
	got, err := readClientData(c)
	if err != nil {
		t.Fatalf("unexpected error after creating the Unix socket: %s", err)
	}
	if got != "first" {
		t.Fatalf("unexpected response; got %q; want %q", got, "first")
	}

	s.Close()
	newUnixSocketTestServer(t, socketPath, false, "second")
	got, err = readClientData(c)
	if err != nil {
		t.Fatalf("unexpected error after recreating the Unix socket: %s", err)
	}
	if got != "second" {
		t.Fatalf("unexpected response; got %q; want %q", got, "second")
	}
}

func TestClientProxyReadOk(t *testing.T) {
	ctx := context.Background()
	f := func(isBackendTLS, isProxyTLS bool, backendAuth, proxyAuth *promauth.BasicAuthConfig) {
		t.Helper()

		proxyHandler := &testProxyServer{ba: proxyAuth}
		ps := newClientTestServer(isProxyTLS, proxyHandler)

		expectedBackendResponse := `metric_name{key="value"} 123\n`

		backend := newClientTestServer(isBackendTLS, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if backendAuth != nil && !checkBasicAuthHeader(w, r.Header.Get("Authorization"), backendAuth) {
				return
			}
			w.Write([]byte(expectedBackendResponse))
		}))

		defer backend.Close()
		defer ps.Close()

		c, err := newClient(ctx, &ScrapeWork{
			ScrapeURL: backend.URL,
			ProxyURL:  proxy.MustNewURL(ps.URL),
			// bump timeout for slow CIs
			ScrapeTimeout: 5 * time.Second,
			// force connection re-creating to avoid broken conns in slow CIs
			DisableKeepAlive:   true,
			AuthConfig:         newTestAuthConfig(t, isBackendTLS, backendAuth),
			ProxyAuthConfig:    newTestAuthConfig(t, isProxyTLS, proxyAuth),
			MaxScrapeSize:      16000,
			DisableCompression: true,
		})
		if err != nil {
			t.Fatalf("failed to create client: %s", err)
		}

		var cb chunkedbuffer.Buffer
		isGzipped, err := c.ReadData(&cb)
		if err != nil {
			t.Fatalf("unexpected error at ReadData: %s", err)
		}
		if isGzipped {
			t.Fatalf("the response mustn't be gzipped")
		}
		got, err := io.ReadAll(cb.NewReader())
		if err != nil {
			t.Fatalf("err read: %s", err)
		}

		if !proxyHandler.receivedProxyRequest {
			t.Fatalf("proxy server didn't received request")
		}
		if string(got) != expectedBackendResponse {
			t.Fatalf("not expected response: ")
		}
	}

	// no tls
	f(false, false, nil, nil)
	// both tls no auth
	f(true, true, nil, nil)
	// backend tls, proxy http no auth
	f(true, false, nil, nil)
	// backend http, proxy tls no auth
	f(false, true, nil, nil)

	// no tls with auth
	f(false, false, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, &promauth.BasicAuthConfig{Username: "proxy-test"})
	// proxy tls and auth
	f(false, true, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, &promauth.BasicAuthConfig{Username: "proxy-test"})
	// backend tls and auth
	f(true, false, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, &promauth.BasicAuthConfig{Username: "proxy-test"})
	// tls with auth
	f(true, true, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, &promauth.BasicAuthConfig{Username: "proxy-test"})

	// tls with backend auth
	f(true, true, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, nil)
	// tls with proxy auth
	f(true, true, nil, &promauth.BasicAuthConfig{Username: "proxy-test", Password: promauth.NewSecret("1234")})
	// proxy tls with backend auth
	f(false, true, &promauth.BasicAuthConfig{Username: "test", Password: promauth.NewSecret("1234")}, nil)
	// backend tls and proxy auth
	f(true, false, nil, &promauth.BasicAuthConfig{Username: "proxy-test", Password: promauth.NewSecret("1234")})
}
