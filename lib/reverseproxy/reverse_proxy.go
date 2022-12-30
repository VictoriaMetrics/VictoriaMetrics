package reverseproxy

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	targetURL     = "vm-target-url"
	forwardHeader = "X-Forwarded-For"
)

var (
	maxIdleConnsPerBackend = flag.Int("maxIdleConnsPerBackend", 100, "The maximum number of idle connections vmauth can open per each backend host")
)

var bbPool bytesutil.ByteBufferPool

// ReverseProxy represents simple revers proxy based on http.Client
type ReverseProxy struct {
	client *http.Client
}

// New initialize revers proxy
func New() *ReverseProxy {
	return &ReverseProxy{
		client: &http.Client{
			Transport: prepareTransport(),
		},
	}
}

// ServeHTTP serve http requests
func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp.handle(w, r)
}

// handle works with income request, update it to outcome and copy response to the requester
func (rp *ReverseProxy) handle(w http.ResponseWriter, r *http.Request) {
	bb := bbPool.Get()
	defer func() { bbPool.Put(bb) }()

	tURL, err := getTargetURL(r)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing targetURL=%q: %s", targetURL, err)
	}

	ctx := r.Context()
	proxyReq := r.Clone(ctx)

	proxyReq.URL = tURL
	if r.ContentLength == 0 {
		proxyReq.Body = nil // Issue https://github.com/golang/go/issues/16036: nil Body for http.Transport retries
	}
	if proxyReq.Body != nil {
		defer func() { _ = proxyReq.Body.Close() }()
	}

	proxyRequest := func() error {
		if err := updateXForwardHeader(r.RemoteAddr, proxyReq.Header); err != nil {
			return fmt.Errorf("cannot update %s header: %s", forwardHeader, err)
		}

		resp, err := rp.client.Transport.RoundTrip(proxyReq)
		if err != nil {
			return fmt.Errorf("cannot execute http transaction: %s", err)
		}
		defer func() { _ = resp.Body.Close() }()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Set(k, v)
			}
		}

		w.WriteHeader(resp.StatusCode)
		_, err = io.CopyBuffer(w, resp.Body, bb.B)
		if err != nil {
			return fmt.Errorf("cannot copy response body to : %s", err)
		}
		return nil
	}

	if err := proxyRequest(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
}

// prepareTransport builds http transport which can be configurable
func prepareTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableCompression = true
	// Disable HTTP/2.0, since VictoriaMetrics components don't support HTTP/2.0 (because there is no sense in this).
	tr.ForceAttemptHTTP2 = false

	tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}

	return tr
}

// getTargetURL gets target url from defined header and check it
func getTargetURL(r *http.Request) (*url.URL, error) {
	targetURL := r.Header.Get(targetURL)
	return url.Parse(targetURL)
}

// updateXForwardHeader updates "X-Forwarded-For" header if it is needed
func updateXForwardHeader(remoteAdrr string, headers http.Header) error {
	clientIP, _, err := net.SplitHostPort(remoteAdrr)
	if err != nil {
		return err
	}

	forwardFor, ok := headers[forwardHeader]
	omit := ok && forwardFor == nil // Issue https://github.com/golang/go/issues/38079: nil now means don't populate the header
	if len(forwardFor) > 0 {
		clientIP = strings.Join(forwardFor, ", ") + ", " + clientIP
	}
	if !omit {
		headers.Set(forwardHeader, clientIP)
	}
	return nil
}
