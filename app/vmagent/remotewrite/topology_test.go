package remotewrite

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

func TestGetRemoteWriteAddr(t *testing.T) {
	f := func(urlRaw, want string) {
		t.Helper()

		u, err := url.Parse(urlRaw)
		if err != nil {
			t.Fatalf("unexpected parse error for %q: %s", urlRaw, err)
		}
		got := getRemoteWriteAddr(u)
		if got != want {
			t.Fatalf("unexpected addr for %q; got %q; want %q", urlRaw, got, want)
		}
	}

	f("http://vminsert:8480/api/v1/write", "vminsert:8480")
	f("http://vminsert/api/v1/write", "vminsert:80")
	f("https://vminsert/api/v1/write", "vminsert:443")
	f("http://srv+vminsert/api/v1/write", "srv+vminsert")
	f("http://[2001:db8::1]:8480/api/v1/write", "[2001:db8::1]:8480")
	f("http:///api/v1/write", "")
}

func TestResolveTopologyTargets_Direct(t *testing.T) {
	customResolver := &fakeResolver{
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vminsert": {
				{IP: net.ParseIP("10.20.30.40")},
			},
		},
	}
	origResolver := netutil.Resolver
	netutil.Resolver = customResolver
	defer func() {
		netutil.Resolver = origResolver
	}()

	urls := []string{"http://vminsert:8480/api/v1/write"}
	got := resolveTopologyTargets(urls)
	want := []topologyTarget{
		{
			addr:       "vminsert:8480",
			resolvedIP: "10.20.30.40:8480",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected targets\ngot:\n%v\nwant:\n%v", got, want)
	}
}

func TestResolveTopologyTargets_DefaultPort(t *testing.T) {
	customResolver := &fakeResolver{
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vminsert": {
				{IP: net.ParseIP("10.20.30.41")},
			},
		},
	}
	origResolver := netutil.Resolver
	netutil.Resolver = customResolver
	defer func() {
		netutil.Resolver = origResolver
	}()

	urls := []string{"http://vminsert/api/v1/write"}
	got := resolveTopologyTargets(urls)
	want := []topologyTarget{
		{
			addr:       "vminsert:80",
			resolvedIP: "10.20.30.41:80",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected targets\ngot:\n%v\nwant:\n%v", got, want)
	}
}

func TestResolveTopologyTargets_SRV(t *testing.T) {
	customResolver := &fakeResolver{
		lookupSRVResults: map[string][]*net.SRV{
			"vmselect": {
				{
					Target: "vmselect-0.local.",
					Port:   8481,
				},
				{
					Target: "vmselect-1.local.",
					Port:   8481,
				},
			},
		},
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vmselect-0.local": {
				{IP: net.ParseIP("10.20.30.50")},
			},
			"vmselect-1.local": {
				{IP: net.ParseIP("10.20.30.51")},
			},
		},
	}
	origResolver := netutil.Resolver
	netutil.Resolver = customResolver
	defer func() {
		netutil.Resolver = origResolver
	}()

	urls := []string{"http://srv+vmselect/api/v1/write"}
	got := resolveTopologyTargets(urls)
	want := []topologyTarget{
		{
			addr:       "srv+vmselect",
			resolvedIP: "10.20.30.50:8481",
		},
		{
			addr:       "srv+vmselect",
			resolvedIP: "10.20.30.51:8481",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected targets\ngot:\n%v\nwant:\n%v", got, want)
	}
}

func TestBuildTopologyMetricNames(t *testing.T) {
	targets := []topologyTarget{
		{addr: "vminsert:8480", resolvedIP: "10.0.0.1:8480"},
	}
	got := buildTopologyMetricNames(targets, "10.0.0.2:8429")
	want := map[string]struct{}{
		`vm_topology_discovery_targets{addr="vminsert:8480", resolved_ip="10.0.0.1:8480", instance="10.0.0.2:8429"}`: {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected metric names\ngot:\n%v\nwant:\n%v", got, want)
	}
}

type fakeResolver struct {
	Resolver            *net.Resolver
	lookupSRVResults    map[string][]*net.SRV
	lookupIPAddrResults map[string][]net.IPAddr
}

func (r *fakeResolver) LookupSRV(_ context.Context, _, _, name string) (string, []*net.SRV, error) {
	if results, ok := r.lookupSRVResults[name]; ok {
		return name, results, nil
	}
	return name, nil, fmt.Errorf("no srv results found for host: %s", name)
}

func (r *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if results, ok := r.lookupIPAddrResults[host]; ok {
		return results, nil
	}
	return nil, fmt.Errorf("no results found for host: %s", host)
}

func (r *fakeResolver) LookupMX(_ context.Context, _ string) ([]*net.MX, error) {
	return nil, nil
}
