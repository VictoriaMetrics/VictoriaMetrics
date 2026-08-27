package httputil

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

type testRemoteServer struct {
	mu              sync.Mutex
	requestsPerHost map[string]int

	totalRequests int
	firstError    error
}

func (trs *testRemoteServer) RoundTrip(r *http.Request) (*http.Response, error) {
	trs.mu.Lock()
	if trs.firstError != nil && trs.totalRequests == 0 {
		err := trs.firstError
		trs.firstError = nil
		trs.totalRequests++
		trs.mu.Unlock()
		return nil, err
	}
	trs.totalRequests++

	if trs.requestsPerHost == nil {
		trs.requestsPerHost = make(map[string]int)
	}
	trs.requestsPerHost[r.URL.Host]++
	trs.mu.Unlock()

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

type testDNSResolver struct {
	ips []net.IPAddr
}

func (tdr *testDNSResolver) LookupSRV(_ context.Context, _, _, name string) (cname string, addrs []*net.SRV, err error) {
	return "", nil, fmt.Errorf("unexpected LookupMX call for name=%q", name)
}
func (tdr *testDNSResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return tdr.ips, nil
}

func (tdr *testDNSResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	return nil, fmt.Errorf("unexpected LookupMX call for name=%q", name)
}

func TestLoadbalancerTransport(t *testing.T) {
	// discoveredIPs are returned by DNS; expectedIPs must receive requests.
	f := func(discoveredIPs, expectedIPs []string, trs *testRemoteServer) {
		t.Helper()

		parsedIPs := make([]net.IPAddr, 0, len(discoveredIPs))
		for _, dIP := range discoveredIPs {
			pIP, err := netip.ParseAddr(dIP)
			if err != nil {
				t.Fatalf("cannot parse IP=%q: %s", dIP, err)
			}
			parsedIPs = append(parsedIPs, net.IPAddr{IP: pIP.AsSlice()})
		}
		tdr := &testDNSResolver{ips: parsedIPs}
		originResolver := netutil.Resolver
		defer func() { netutil.Resolver = originResolver }()

		netutil.Resolver = tdr
		requestURL, err := url.Parse("http://dns+vmsingle.example.com:8429/api/v1/write")
		if err != nil {
			t.Fatalf("cannot parse url: %s", err)
		}
		lbt, requestURL := NewLoadBalancerTransport(trs, requestURL)
		if len(expectedIPs) == 0 {
			r, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
			if err != nil {
				t.Fatalf("cannot create http request: %s", err)
			}
			_, err = lbt.RoundTrip(r)
			if err == nil {
				t.Fatalf("expected no backends found error")
			}
			return
		}
		expectedRequestsPerHost := 2
		for range len(expectedIPs) * expectedRequestsPerHost {
			r, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
			if err != nil {
				t.Fatalf("cannot create http request: %s", err)
			}
			resp, err := lbt.RoundTrip(r)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			resp.Body.Close()
		}
		requestsPerHost := trs.requestsPerHost
		if len(requestsPerHost) != len(expectedIPs) {
			t.Fatalf("unexpected number of backends received requests; got %d; want %d", len(requestsPerHost), len(expectedIPs))
		}

		for _, dIP := range expectedIPs {
			expectedHostPort := net.JoinHostPort(dIP, "8429")
			gotRequestsPerHost, ok := requestsPerHost[expectedHostPort]
			if !ok {
				t.Fatalf("not found expected backend request for: %q", expectedHostPort)
			}
			if gotRequestsPerHost != expectedRequestsPerHost {
				t.Fatalf("unexpected requests per host:%q %d:%d (-;+)", expectedHostPort, expectedRequestsPerHost, gotRequestsPerHost)
			}
		}
	}
	origTCP6 := flag.Lookup("enableTCP6").Value.String()
	defer func() { _ = flag.Set("enableTCP6", origTCP6) }()
	if err := flag.Set("enableTCP6", "false"); err != nil {
		t.Fatalf("cannot set -enableTCP6=false: %s", err)
	}

	trs := testRemoteServer{}
	f([]string{"1.1.1.1"}, []string{"1.1.1.1"}, &trs)

	trs = testRemoteServer{}
	f([]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, []string{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, &trs)

	// ipv6 backends are skipped when -enableTCP6 isn't set
	trs = testRemoteServer{}
	f([]string{"1.1.1.1", "2001:db8::1"}, []string{"1.1.1.1"}, &trs)

	// empty backends, expecting error
	trs = testRemoteServer{}
	f([]string{}, []string{}, &trs)

	trs = testRemoteServer{}
	f([]string{"2001:db8::1"}, []string{}, &trs)

	if err := flag.Set("enableTCP6", "true"); err != nil {
		t.Fatalf("cannot set -enableTCP6=true: %s", err)
	}
	trs = testRemoteServer{}
	f([]string{"1.1.1.1", "2001:db8::1"}, []string{"1.1.1.1", "2001:db8::1"}, &trs)
}
