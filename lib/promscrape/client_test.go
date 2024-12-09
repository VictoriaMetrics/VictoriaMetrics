package promscrape

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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

	resp, err := http.DefaultTransport.RoundTrip(r)
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
		t.Fatalf("cannot setup promauth.Confg: %s", err)
	}
	return ac
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
			DisableKeepAlive: true,
			AuthConfig:       newTestAuthConfig(t, isBackendTLS, backendAuth),
			ProxyAuthConfig:  newTestAuthConfig(t, isProxyTLS, proxyAuth),
			MaxScrapeSize:    16000,
		})
		if err != nil {
			t.Fatalf("failed to create client: %s", err)
		}

		var bb bytesutil.ByteBuffer
		if err = c.ReadData(&bb); err != nil {
			t.Fatalf("unexpected error at ReadData: %s", err)
		}
		got, err := io.ReadAll(bb.NewReader())
		if err != nil {
			t.Fatalf("err read: %s", err)
		}

		if !proxyHandler.receivedProxyRequest {
			t.Fatalf("proxy server didn't recieved request")
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
