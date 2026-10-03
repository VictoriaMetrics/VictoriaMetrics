package httputil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
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
		// RoundTrip must always close the request body.
		if r.Body != nil {
			_ = r.Body.Close()
		}
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
	ips  []net.IPAddr
	srvs []*net.SRV
}

func (tdr *testDNSResolver) LookupSRV(_ context.Context, _, _, name string) (cname string, addrs []*net.SRV, err error) {
	if tdr.srvs == nil {
		return "", nil, fmt.Errorf("unexpected LookupSRV call for name=%q", name)
	}
	return "", tdr.srvs, nil
}
func (tdr *testDNSResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return tdr.ips, nil
}

func (tdr *testDNSResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	return nil, fmt.Errorf("unexpected LookupMX call for name=%q", name)
}

func TestLoadbalancerTransport(t *testing.T) {
	f := func(discoveredIPs []string, trs *testRemoteServer) {
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
		if len(discoveredIPs) == 0 {
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
		for range len(discoveredIPs) * expectedRequestsPerHost {
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

		for _, dIP := range discoveredIPs {
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
	trs := testRemoteServer{}
	f([]string{"1.1.1.1"}, &trs)

	trs = testRemoteServer{}
	f([]string{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, &trs)

	// empty backends, expecting error
	trs = testRemoteServer{}
	f([]string{}, &trs)
}

type hostRecorder struct {
	hosts map[string]string
}

func (hr *hostRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	hr.hosts[r.URL.Host] = r.Host
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestLoadbalancerTransportHostHeader(t *testing.T) {
	f := func(rawURL, requestHost string, tdr *testDNSResolver, hostsExpected map[string]string) {
		t.Helper()

		originResolver := netutil.Resolver
		defer func() { netutil.Resolver = originResolver }()
		netutil.Resolver = tdr

		requestURL, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("cannot parse url: %s", err)
		}
		hr := &hostRecorder{hosts: make(map[string]string)}
		lbt, requestURL := NewLoadBalancerTransport(hr, requestURL)
		for range len(hostsExpected) {
			r, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
			if err != nil {
				t.Fatalf("cannot create http request: %s", err)
			}
			r.Host = requestHost
			resp, err := lbt.RoundTrip(r)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			resp.Body.Close()
		}
		if !reflect.DeepEqual(hr.hosts, hostsExpected) {
			t.Fatalf("unexpected Host headers per backend;\ngot\n%v\nwant\n%v", hr.hosts, hostsExpected)
		}
	}

	srvResolver := &testDNSResolver{
		srvs: []*net.SRV{
			{Target: "vmserver1.example.com.", Port: 8428},
			{Target: "vmserver2.example.com.", Port: 8429},
			{Target: "vmserver3.example.com.", Port: 80},
		},
	}

	// SRV backends get the Host header from the SRV target
	f("http://srv+_vmtest/api/v1/write", "", srvResolver, map[string]string{
		"vmserver1.example.com.:8428": "vmserver1.example.com:8428",
		"vmserver2.example.com.:8429": "vmserver2.example.com:8429",
		"vmserver3.example.com.:80":   "vmserver3.example.com",
	})

	// port 80 is kept, since it is not the default port for https
	f("https://srv+_vmtest/api/v1/write", "", srvResolver, map[string]string{
		"vmserver1.example.com.:8428": "vmserver1.example.com:8428",
		"vmserver2.example.com.:8429": "vmserver2.example.com:8429",
		"vmserver3.example.com.:80":   "vmserver3.example.com:80",
	})

	// explicitly set Host header has priority over SRV target
	f("http://srv+_vmtest/api/v1/write", "custom.example.com", srvResolver, map[string]string{
		"vmserver1.example.com.:8428": "custom.example.com",
		"vmserver2.example.com.:8429": "custom.example.com",
		"vmserver3.example.com.:80":   "custom.example.com",
	})

	// DNS backends get the Host header from the request url
	f("http://dns+vmsingle.example.com:8429/api/v1/write", "", &testDNSResolver{
		ips: []net.IPAddr{{IP: net.IPv4(1, 1, 1, 1)}},
	}, map[string]string{
		"1.1.1.1:8429": "vmsingle.example.com:8429",
	})
}
