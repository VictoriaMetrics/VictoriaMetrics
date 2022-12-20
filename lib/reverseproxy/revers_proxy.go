package reverseproxy

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	targetURL     = "vm-target-url"
	forwardHeader = "X-Forwarded-For"
)

var (
	maxIdleConnsPerBackend = flag.Int("maxIdleConnsPerBackend", 100, "The maximum number of idle connections vmauth can open per each backend host")
)

// ReversProxy represents simple revers proxy based on http.Client
type ReversProxy struct {
	client  *http.Client
	limiter chan struct{}
}

// New initialize revers proxy
func New() *ReversProxy {
	return &ReversProxy{
		client: &http.Client{},
	}
}

// ServeHTTP serve http requests
func (rr *ReversProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rr.handle(w, r)
}

// ServeHTTPWithLimit limits served requests by defined limit. If limit reached http error returns
func (rr *ReversProxy) ServeHTTPWithLimit(w http.ResponseWriter, r *http.Request) {
	select {
	case <-rr.limiter:
		rr.handle(w, r)
		rr.limiter <- struct{}{}
	default:
		message := fmt.Sprintf("cannot handle more than %d connections", 100)
		http.Error(w, message, http.StatusTooManyRequests)
	}
}

// WithLimit sets concurrent requests limit
func (rr *ReversProxy) WithLimit(maxConcurrentRequests int) *ReversProxy {
	rr.limiter = make(chan struct{}, maxConcurrentRequests)
	for i := 0; i < maxConcurrentRequests; i++ {
		rr.limiter <- struct{}{}
	}
	return rr
}

// handle works with income request, update it to outcome and copy response to the requester
func (rr *ReversProxy) handle(w http.ResponseWriter, r *http.Request) {
	tURL, err := getTargetURL(r)
	if err != nil {
		logger.Panicf("BUG: unexpected error when parsing targetURL=%q: %s", targetURL, err)
	}

	rr.client.Transport = prepareTransport(tURL)

	r.URL = tURL

	ctx := r.Context()
	proxyReq := r.Clone(ctx)

	if r.ContentLength == 0 {
		proxyReq.Body = nil // Issue 16036: nil Body for http.Transport retries
	}
	if proxyReq.Body != nil {
		defer func() { _ = proxyReq.Body.Close() }()
	}

	if err := updateXForwardHeader(r.RemoteAddr, proxyReq.Header); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	resp, err := rr.client.Transport.RoundTrip(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Set(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
}

// prepareTransport builds http transport which can be configurable
func prepareTransport(targetURL *url.URL) *http.Transport {
	tr := &http.Transport{
		Proxy:              http.ProxyURL(targetURL),
		DisableCompression: true,
		// Disable HTTP/2.0, since VictoriaMetrics components don't support HTTP/2.0 (because there is no sense in this).
		ForceAttemptHTTP2: false,
	}

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
	omit := ok && forwardFor == nil // Issue 38079: nil now means don't populate the header
	if len(forwardFor) > 0 {
		clientIP = strings.Join(forwardFor, ", ") + ", " + clientIP
	}
	if !omit {
		headers.Set(forwardHeader, clientIP)
	}
	return nil
}
