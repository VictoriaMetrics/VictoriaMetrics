package httputil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

const (
	brokenBackendTimeout     = 5 * time.Second
	backendDiscoveryInterval = 10 * time.Second
	backendDiscoveryTimeout  = 10 * time.Second
)

// NewLoadBalancerTransport returns new RoundTripper that performs least-loaded HTTP requests loadbalancing
// based on discovered backends for the given url host
// and update url with load-balancing prefix
//
// It returns origin transport and url if load-balancing is not needed for given url
func NewLoadBalancerTransport(origin http.RoundTripper, originURL *url.URL) (http.RoundTripper, *url.URL) {

	modifiedURL := *originURL
	var discoverFunc func(context.Context, string, string) ([]*backend, error)
	switch {
	case strings.HasPrefix(originURL.Host, "dns+"):
		modifiedURL.Host = modifiedURL.Host[4:]
		discoverFunc = discoverDNSBackends
	case strings.HasPrefix(originURL.Host, "srv+"):
		modifiedURL.Host = modifiedURL.Host[4:]
		discoverFunc = discoverSRVBackends
	default:
		return origin, originURL
	}
	host, port, err := net.SplitHostPort(modifiedURL.Host)
	if err != nil {
		host = modifiedURL.Host
		port = "80"
		if modifiedURL.Scheme == "https" {
			port = "443"
		}
	}
	t := &loadbalancerTransport{
		tr:           origin,
		host:         host,
		port:         port,
		discoverFunc: discoverFunc,
	}
	t.discoverBackends()
	return t, &modifiedURL
}

type loadbalancerTransport struct {
	tr   http.RoundTripper
	host string
	port string

	discoverFunc func(context.Context, string, string) ([]*backend, error)

	nextDiscoveryDeadline atomic.Uint64
	discovering           atomic.Bool
	dbs                   atomic.Pointer[discoveredBackends]
}

type discoveredBackends struct {
	backends []*backend
	// n is an atomic counter, which is used for balancing load among available backends.
	n atomic.Uint32
}

// getLeastLoadedBackend returns least loaded backend
// caller must release backend with backend.put() method
func (dbs *discoveredBackends) getLeastLoadedBackend() *backend {
	firstB := dbs.backends[0]
	if len(dbs.backends) == 1 {
		firstB.get()
		return firstB
	}

	// Slow path - select other backends.
	n := dbs.n.Add(1) - 1
	for i := range uint32(len(dbs.backends)) {
		idx := (n + i) % uint32(len(dbs.backends))
		bu := dbs.backends[idx]
		if bu.isBroken() {
			continue
		}

		// The Load() in front of CompareAndSwap() avoids CAS overhead for items with values bigger than 0.
		if bu.concurrentRequests.Load() == 0 && bu.concurrentRequests.CompareAndSwap(0, 1) {
			dbs.n.CompareAndSwap(n+1, idx+1)
			// There is no need in the call b.get(), because we already incremented b.concurrentRequests above.
			return bu
		}
	}

	// Slow path - return the backend with the minimum number of concurrently executed requests.
	bMinIdx := n % uint32(len(dbs.backends))
	minRequests := dbs.backends[bMinIdx].concurrentRequests.Load()
	for i := uint32(1); i < uint32(len(dbs.backends)); i++ {
		idx := (n + i) % uint32(len(dbs.backends))
		bu := dbs.backends[idx]
		if bu.isBroken() {
			continue
		}

		reqs := bu.concurrentRequests.Load()
		if reqs < minRequests || dbs.backends[bMinIdx].isBroken() {
			bMinIdx = idx
			minRequests = reqs
		}
	}
	bMin := dbs.backends[bMinIdx]
	if bMin.isBroken() {
		// If all backends are broken, then returns the first backend.
		firstB.get()
		return firstB
	}
	bMin.get()
	dbs.n.CompareAndSwap(n+1, bMinIdx+1)
	return bMin
}

type backend struct {
	addr               string
	concurrentRequests atomic.Int32
	brokenDeadline     atomic.Uint64
}

func (b *backend) get() {
	b.concurrentRequests.Add(1)
}

func (b *backend) put() {
	b.concurrentRequests.Add(-1)
}

func (b *backend) isBroken() bool {
	bd := b.brokenDeadline.Load()
	if bd == 0 {
		return false
	}
	ct := fasttime.UnixTimestamp()
	return ct < bd
}

// RoundTrip implements http.RoundTripper interface
func (lb *loadbalancerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	dbs := lb.getBackends()
	if dbs == nil || len(dbs.backends) == 0 {
		return nil, fmt.Errorf("no backends found for hostname=%q", lb.host)
	}

	maxRetries := len(dbs.backends)
	var lastErr error
	for range maxRetries {
		b := dbs.getLeastLoadedBackend()
		resp, err := lb.doRequest(r, b)
		if err != nil {
			ct := fasttime.UnixTimestamp()
			brokenDeadline := ct + uint64(brokenBackendTimeout.Seconds())
			b.brokenDeadline.Store(brokenDeadline)
			if !netutil.IsTrivialNetworkError(err) {
				return nil, err
			}
			// perform the same check for retry as http.Request.isReplayable does
			canRetry := r.Body == nil || r.Body == http.NoBody || r.GetBody != nil
			if !canRetry {
				return nil, err
			}
			lastErr = err
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all backends are unavailable: %w", lastErr)
}

func (lb *loadbalancerTransport) doRequest(r *http.Request, b *backend) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	if r.GetBody != nil {
		body, err := r.GetBody()
		if err != nil {
			b.put()
			return nil, err
		}
		r2.Body = body
	}
	r2.URL.Host = b.addr
	if r2.Host == "" {
		r2.Host = r.URL.Host
	}
	resp, err := lb.tr.RoundTrip(r2)
	if err != nil {
		b.put()
		return nil, err
	}
	// wrap response body with readCloser that releases backend after Close call
	// it's needed to properly account loaded backends at getLeastLoadedBackends
	resp.Body = newReleaseReadCloser(resp.Body, b)
	return resp, nil
}

func (lb *loadbalancerTransport) getBackends() *discoveredBackends {
	ct := fasttime.UnixTimestamp()
	deadline := lb.nextDiscoveryDeadline.Load()
	if ct < deadline || !lb.discovering.CompareAndSwap(false, true) {
		return lb.dbs.Load()
	}
	lb.discoverBackends()
	return lb.dbs.Load()
}

func (lb *loadbalancerTransport) discoverBackends() {
	ctx, cancel := context.WithTimeout(context.Background(), backendDiscoveryTimeout)
	defer func() {
		cancel()
		ct := fasttime.UnixTimestamp()
		nextDeadline := ct + uint64(backendDiscoveryInterval.Seconds())
		lb.nextDiscoveryDeadline.Store(nextDeadline)
		lb.discovering.Store(false)
	}()
	backends, err := lb.discoverFunc(ctx, lb.host, lb.port)
	if err != nil {
		logger.Errorf("cannot discover backends: %s, retry in %s", err, backendDiscoveryInterval)
		return
	}
	dbs := &discoveredBackends{
		backends: backends,
	}
	sort.Slice(dbs.backends, func(i, j int) bool {
		return dbs.backends[i].addr < dbs.backends[j].addr
	})

	prevBackends := lb.dbs.Load()
	if areBackendsEqual(prevBackends, dbs) {
		return
	}

	lb.dbs.Store(dbs)
}

func discoverDNSBackends(ctx context.Context, host, port string) ([]*backend, error) {
	addrs, err := netutil.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to lookupIPAddr for host: %q: %w", host, err)
	}
	backends := make([]*backend, 0, len(addrs))
	for _, addr := range addrs {
		if !netutil.TCP6Enabled() {
			ip, ok := netip.AddrFromSlice(addr.IP)
			if !ok {
				logger.Panicf("BUG: cannot build netip Addr from slice addr: %q", addr.IP.String())
			}
			if !ip.Unmap().Is4() {
				continue
			}
		}
		ip := addr.IP.String()
		if len(port) > 0 {
			ip = net.JoinHostPort(ip, port)
		}
		backends = append(backends, &backend{addr: ip})
	}
	return backends, nil
}

func discoverSRVBackends(ctx context.Context, host, port string) ([]*backend, error) {
	_, addrs, err := netutil.Resolver.LookupSRV(ctx, "", "", host)
	if err != nil {
		return nil, fmt.Errorf("failed to LookupSRV records for host: %q: %w", host, err)
	}
	backends := make([]*backend, 0, len(addrs))
	for _, addr := range addrs {
		hostPort := port
		if addr.Port > 0 {
			hostPort = strconv.FormatUint(uint64(addr.Port), 10)
		}
		hostAddr := net.JoinHostPort(addr.Target, hostPort)
		backends = append(backends, &backend{addr: hostAddr})
	}
	return backends, nil
}

func newReleaseReadCloser(responseBody io.ReadCloser, b *backend) io.ReadCloser {
	return &releaseReadCloser{
		ReadCloser: responseBody,
		b:          b,
	}
}

type releaseReadCloser struct {
	io.ReadCloser
	b        *backend
	released atomic.Bool
}

func (rrc *releaseReadCloser) Close() error {
	if rrc.released.CompareAndSwap(false, true) {
		// Close method could be called multiple times
		// and it must produce idempotent result
		rrc.b.put()
	}
	return rrc.ReadCloser.Close()
}

func areBackendsEqual(left, right *discoveredBackends) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(left.backends) != len(right.backends) {
		return false
	}
	for idx, leftB := range left.backends {
		rightB := right.backends[idx]
		if leftB.addr != rightB.addr {
			return false
		}
	}

	return true
}
