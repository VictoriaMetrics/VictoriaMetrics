package topology

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

func TestNewTarget(t *testing.T) {
	f := func(rawURL, sanitizedURL string, want *target) {
		t.Helper()

		got, err := newTarget(rawURL, sanitizedURL)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected target\ngot:\n%#v\nwant:\n%#v", got, want)
		}
	}

	f("http://vminsert:8480/api/v1/write", "1:secret-url", &target{
		urlLabel:  "1:secret-url",
		addrLabel: "vminsert:8480",
		host:      "vminsert",
	})
	f("http://vminsert/api/v1/write", "1:secret-url", &target{
		urlLabel:  "1:secret-url",
		addrLabel: "vminsert:80",
		host:      "vminsert",
	})
	f("https://vminsert/api/v1/write", "1:secret-url", &target{
		urlLabel:  "1:secret-url",
		addrLabel: "vminsert:443",
		host:      "vminsert",
	})
	f("http://srv+vmselect/api/v1/write", "1:secret-url", &target{
		urlLabel:  "1:secret-url",
		addrLabel: "srv+vmselect",
		host:      "srv+vmselect",
	})
	f("http://[2001:db8::1]:8480/api/v1/write", "1:secret-url", &target{
		urlLabel:  "1:secret-url",
		addrLabel: "[2001:db8::1]:8480",
		host:      "2001:db8::1",
	})
}

func TestResolveIPsDirect(t *testing.T) {
	withResolver(t, &fakeResolver{
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vminsert": {
				{IP: net.ParseIP("10.20.30.40")},
				{IP: net.ParseIP("10.20.30.40")},
				{IP: net.ParseIP("10.20.30.41")},
			},
		},
	}, func() {
		got, ok := resolveIPs(context.Background(), "vminsert")
		if !ok {
			t.Fatalf("expected successful direct resolution")
		}
		want := []string{"10.20.30.40", "10.20.30.41"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected resolved IPs\ngot:\n%v\nwant:\n%v", got, want)
		}
	})
}

func TestResolveIPsSRV(t *testing.T) {
	withResolver(t, &fakeResolver{
		lookupSRVResults: map[string][]*net.SRV{
			"vmselect": {
				{Target: "vmselect-0.local.", Port: 8481},
				{Target: "vmselect-1.local.", Port: 8481},
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
	}, func() {
		got, ok := resolveIPs(context.Background(), "srv+vmselect")
		if !ok {
			t.Fatalf("expected successful SRV resolution")
		}
		want := []string{"10.20.30.50", "10.20.30.51"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected resolved IPs\ngot:\n%v\nwant:\n%v", got, want)
		}
	})
}

func TestTargetKeepsLastSuccessfulIPs(t *testing.T) {
	tg := &target{
		urlLabel:  "1:secret-url",
		addrLabel: "vminsert:8480",
		host:      "vminsert",
	}

	st := &state{
		targets: map[string]*target{
			tg.urlLabel: tg,
		},
	}
	want := []targetSample{{
		urlLabel:  "1:secret-url",
		addrLabel: "vminsert:8480",
		ip:        "10.20.30.40",
	}}

	// No samples before first successful resolution.
	withResolver(t, &fakeResolver{}, func() {
		st.refresh()
		if got := st.samples(); len(got) != 0 {
			t.Fatalf("expected no samples before first successful resolution; got %v", got)
		}
	})

	// Samples appear after successful resolution.
	withResolver(t, &fakeResolver{
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vminsert": {
				{IP: net.ParseIP("10.20.30.40")},
			},
		},
	}, func() {
		st.refresh()
		if got := st.samples(); !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected samples after successful refresh\ngot:\n%v\nwant:\n%v", got, want)
		}
	})

	// Last successful set retained after failed resolution.
	withResolver(t, &fakeResolver{}, func() {
		st.refresh()
		if got := st.samples(); !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected samples after failed refresh fallback\ngot:\n%v\nwant:\n%v", got, want)
		}
	})
}

func TestWriteMetrics(t *testing.T) {
	st := &state{
		targets: map[string]*target{
			"2:secret-url": {
				urlLabel:    "2:secret-url",
				addrLabel:   "srv+vmselect",
				resolvedIPs: []string{"10.20.30.51", "10.20.30.50"},
				hasResolved: true,
			},
			"1:secret-url": {
				urlLabel:    "1:secret-url",
				addrLabel:   "vminsert:8480",
				resolvedIPs: []string{"10.20.30.40"},
				hasResolved: true,
			},
		},
	}

	var buf bytes.Buffer
	st.writeMetrics(&buf)
	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	slices.Sort(got)
	want := []string{
		"vm_topology_discovery_targets{url=\"1:secret-url\",addr=\"vminsert:8480\",resolved_ip=\"10.20.30.40\"} 1",
		"vm_topology_discovery_targets{url=\"2:secret-url\",addr=\"srv+vmselect\",resolved_ip=\"10.20.30.50\"} 1",
		"vm_topology_discovery_targets{url=\"2:secret-url\",addr=\"srv+vmselect\",resolved_ip=\"10.20.30.51\"} 1",
	}
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected metrics output\ngot:\n%v\nwant:\n%v", got, want)
	}
}

type fakeResolver struct {
	lookupSRVResults    map[string][]*net.SRV
	lookupIPAddrResults map[string][]net.IPAddr
}

func (r *fakeResolver) LookupSRV(_ context.Context, _, _, name string) (string, []*net.SRV, error) {
	if results, ok := r.lookupSRVResults[name]; ok {
		return name, results, nil
	}
	return name, nil, fmt.Errorf("no SRV results found for host: %s", name)
}

func (r *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if results, ok := r.lookupIPAddrResults[host]; ok {
		return results, nil
	}
	return nil, fmt.Errorf("no IP results found for host: %s", host)
}

func (r *fakeResolver) LookupMX(_ context.Context, host string) ([]*net.MX, error) {
	return nil, fmt.Errorf("no MX results found for host: %s", host)
}

func withResolver(t *testing.T, resolver *fakeResolver, f func()) {
	t.Helper()

	origResolver := netutil.Resolver
	netutil.Resolver = resolver
	defer func() {
		netutil.Resolver = origResolver
	}()

	f()
}
