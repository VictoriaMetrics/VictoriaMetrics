package httputil

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

// NewLoadBalancerTransport returns new RoundTripper that performs round-robin HTTP requests loadbalancing
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
	n atomic.Uint64
}

func (dbs *discoveredBackends) getBackend() *backend {
	if len(dbs.backends) == 1 {
		// fast path
		return dbs.backends[0]
	}
	for range len(dbs.backends) {
		idx := dbs.n.Add(1)
		b := dbs.backends[idx%uint64(len(dbs.backends))]
		if b.isBroken() {
			continue
		}
		return b
	}

	return dbs.backends[0]

}

type backend struct {
	addr           string
	brokenDeadline atomic.Uint64
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
		b := dbs.getBackend()
		r2 := r.Clone(r.Context())
		if r.GetBody != nil {
			body, err := r.GetBody()
			if err != nil {
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
			const brokenDuration = 10 * time.Second
			ct := fasttime.UnixTimestamp()
			brokenDeadline := ct + uint64(brokenDuration.Seconds())
			b.brokenDeadline.Store(brokenDeadline)
			var dnsErr *net.DNSError
			// perform a single retry for in case of trivial error
			// or dns lookup error for srv discovery
			if !netutil.IsTrivialNetworkError(err) && (errors.As(err, &dnsErr) && !dnsErr.IsNotFound) {
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
		return resp, err
	}
	return nil, fmt.Errorf("all backends are unavailable: %w", lastErr)
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
	const discoveryInterval = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
		ct := fasttime.UnixTimestamp()
		nextDeadline := ct + uint64(discoveryInterval.Seconds())
		lb.discovering.Store(false)
		lb.nextDiscoveryDeadline.Store(nextDeadline)
	}()
	backends, err := lb.discoverFunc(ctx, lb.host, lb.port)
	if err != nil {
		logger.Errorf("cannot discover backends: %s, retry in %s", err, discoveryInterval)
		return
	}
	rand.Shuffle(len(backends), func(i, j int) {
		backends[i], backends[j] = backends[j], backends[i]
	})
	dbs := &discoveredBackends{
		backends: backends,
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
