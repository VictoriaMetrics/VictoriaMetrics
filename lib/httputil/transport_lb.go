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
	"sync"
	"time"

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
	var discoverFunc func(context.Context, string, string) ([]string, error)
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
		host = originURL.Host
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
	t.discoverBackendsLocked(context.Background())
	return t, &modifiedURL
}

type loadbalancerTransport struct {
	tr   http.RoundTripper
	host string
	port string

	discoverFunc func(context.Context, string, string) ([]string, error)

	// mu protects fields below
	mu               sync.Mutex
	lastDiscoveredAt time.Time
	dbs              *discoveredBackends
}

type discoveredBackends struct {
	backends []string
	idx      uint64
}

// RoundTrip implements http.RoundTripper interface
func (lb *loadbalancerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	backend := lb.pickBackend(r.Context(), false)
	if backend == "" {
		return nil, fmt.Errorf("no backends found for hostname=%q", lb.host)
	}

	r2 := r.Clone(r.Context())
	r2.URL.Host = backend
	if r2.Host == "" {
		r2.Host = r.URL.Host
	}
	resp, err := lb.tr.RoundTrip(r2)
	if err != nil {
		var dnsErr *net.DNSError
		// perform a single retry for in case of trivial error or dns lookup error
		if !netutil.IsTrivialNetworkError(err) && !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) {
			return nil, err
		}
		backend := lb.pickBackend(r.Context(), true)
		if backend == "" {
			return nil, fmt.Errorf("no backends found for hostname=%q", lb.host)
		}

		// perform the same check for retry as http.Request.isReplayable does
		canRetry := r.Body == nil || r.Body == http.NoBody || r.GetBody != nil
		if !canRetry {
			return nil, err
		}
		r2 = r.Clone(r.Context())
		if r.GetBody != nil {
			body, berr := r.GetBody()
			if berr != nil {
				return nil, err
			}
			r2.Body = body
		}
		if r2.Host == "" {
			r2.Host = r.URL.Host
		}
		r2.URL.Host = backend
		resp, err = lb.tr.RoundTrip(r2)
	}
	return resp, err
}

func (lb *loadbalancerTransport) pickBackend(ctx context.Context, forceDiscovery bool) string {
	ct := time.Now()
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if forceDiscovery && !ct.Before(lb.lastDiscoveredAt) {
		// prevent concurrent force discovery
		lb.lastDiscoveredAt = time.Time{}
	}

	if lb.dbs == nil || ct.Sub(lb.lastDiscoveredAt) > 5*time.Second {
		lb.discoverBackendsLocked(ctx)
	}
	if lb.dbs == nil || len(lb.dbs.backends) == 0 {
		return ""
	}
	idx := lb.dbs.idx
	lb.dbs.idx++
	return lb.dbs.backends[idx%uint64(len(lb.dbs.backends))]
}

func (lb *loadbalancerTransport) discoverBackendsLocked(ctx context.Context) {
	lb.lastDiscoveredAt = time.Now()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	backends, err := lb.discoverFunc(ctx, lb.host, lb.port)
	if err != nil {
		logger.Errorf("cannot discover backends: %s", err)
		return
	}
	rand.Shuffle(len(backends), func(i, j int) {
		backends[i], backends[j] = backends[j], backends[i]
	})
	dbs := &discoveredBackends{
		backends: backends,
	}
	lb.dbs = dbs
}

func discoverDNSBackends(ctx context.Context, host, port string) ([]string, error) {
	addrs, err := netutil.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to lookupIPAddr for host: %q: %w", host, err)
	}
	backends := make([]string, 0, len(addrs))
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
		backends = append(backends, ip)
	}
	return backends, nil
}

func discoverSRVBackends(ctx context.Context, host, port string) ([]string, error) {
	_, addrs, err := netutil.Resolver.LookupSRV(ctx, "", "", host)
	if err != nil {

		return nil, fmt.Errorf("failed to LookupSRV records for host: %q: %w", host, err)
	}
	backends := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		hostPort := port
		if addr.Port > 0 {
			hostPort = strconv.FormatUint(uint64(addr.Port), 10)
		}

		backend := net.JoinHostPort(addr.Target, hostPort)
		backends = append(backends, backend)
	}
	return backends, nil
}
