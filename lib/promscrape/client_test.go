package promscrape

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func TestReadData_HTTPConnectPassthrough(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name             string
		ScrapeURL        string
		ForceHTTPConnect bool
		expectsConnect   bool
	}{
		{
			name:           "http connect passthrough defaults to true for HTTPS",
			ScrapeURL:      "https://my-metrics-server.com:8080",
			expectsConnect: true,
		},
		{
			name:           "http connect passthrough defaults to false for HTTP",
			ScrapeURL:      "http://my-metrics-server.com:8080",
			expectsConnect: false,
		},
		{
			name:             "http connect passthrough forced to true for HTTP",
			ScrapeURL:        "http://my-metrics-server.com:8080",
			ForceHTTPConnect: true,
			expectsConnect:   true,
		},
		{
			name:             "http connect passthrough remains to true for HTTPs",
			ScrapeURL:        "https://my-metrics-server.com:8080",
			ForceHTTPConnect: true,
			expectsConnect:   true,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%s", i, tc.name), func(t *testing.T) {
			// the handler simulates a proxy, but doesn't actually proxy to anything.
			var backend *httptest.Server
			scrapeURL, err := url.Parse(tc.ScrapeURL)
			if err != nil {
				t.Fatalf("unexpected error parsing scrape url: %s", err)
			}

			if scrapeURL.Scheme == "https" {
				backend = httptest.NewTLSServer(http.HandlerFunc(simpleHandler))
			} else {
				backend = httptest.NewServer(http.HandlerFunc(simpleHandler))
			}
			defer backend.Close()
			tp := &testProxy{}
			proxyServer := httptest.NewTLSServer(tp)
			defer proxyServer.Close()

			authConfig, err := authOptions().NewConfig()
			if err != nil {
				t.Fatalf("unexpected error creating tls config: %s", err)
			}
			client, err := newClient(ctx, &ScrapeWork{
				ProxyURL:              proxy.MustNewURL(proxyServer.URL),
				ScrapeURL:             backend.URL, // don't use the scrapeURL, use the backend url.
				ProxyForceHTTPConnect: tc.ForceHTTPConnect,
				AuthConfig:            authConfig,
				ProxyAuthConfig:       authConfig,
				ScrapeTimeout:         2 * time.Second,
			})
			if err != nil {
				t.Fatalf("unexpected error creating client: %s", err)
			}
			buf := make([]byte, 1024)
			// we expect an error since we haven't implemented the actual destination
			_, err = client.ReadData(buf)
			if err != nil {
				t.Errorf("unexpected error reading data: %s", err)
			}
			if !tp.receivedRequest {
				t.Errorf("the handler function never received a request")
			}
			if tc.expectsConnect != tp.receivedConnect {
				t.Errorf("expected connect to be %t, got %t", tc.expectsConnect, tp.receivedConnect)
			}

			// Now test the stream reader
			tp.receivedConnect = false
			tp.receivedRequest = false

			sr, err := client.GetStreamReader()
			if err != nil {
				t.Errorf("unexpected error creating stream reader: %s", err)
			}

			_, err = io.ReadAll(sr)
			if err != nil {
				t.Errorf("unexpected error reading data: %s", err)
			}
			if tc.expectsConnect != tp.receivedConnect {
				t.Errorf("expected connect to be %t, got %t", tc.expectsConnect, tp.receivedConnect)
			}
			if !tp.receivedRequest {
				t.Errorf("the handler function never received a request")
			}
		})
	}
}

func authOptions() *promauth.Options {
	return &promauth.Options{
		TLSConfig: &promauth.TLSConfig{
			InsecureSkipVerify: true,
		},
	}
}

func simpleHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello world"))
}

type testProxy struct {
	receivedRequest bool
	receivedConnect bool
}

func (tp *testProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tp.receivedRequest = true
	if r.Method == http.MethodConnect {
		tp.receivedConnect = true
		tunnel(w, r)
	} else {
		handleHTTP(w, r)
	}
}

func tunnel(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
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
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) // nolint
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()    // nolint
	defer source.Close()         // nolint
	io.Copy(destination, source) // nolint
}
