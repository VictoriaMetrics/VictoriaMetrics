//go:build synctest

package topology

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func TestStateLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var st state
		var resolverMu sync.Mutex
		var resolvedIP net.IP
		var calls int
		resolver := &fakeResolver{
			lookupIPAddrFunc: func(_ context.Context, _ string) ([]net.IPAddr, error) {
				resolverMu.Lock()
				defer resolverMu.Unlock()
				calls++
				return []net.IPAddr{{IP: resolvedIP}}, nil
			},
		}
		checkLookups := func(want int) {
			t.Helper()
			resolverMu.Lock()
			defer resolverMu.Unlock()
			if calls != want {
				t.Fatalf("unexpected DNS lookups; got %d; want %d", calls, want)
			}
		}
		checkMetrics := func(ip string) {
			t.Helper()
			checkTopologyMetrics(t, []string{
				fmt.Sprintf(`vm_topology_discovery_targets{url="1:secret-url",addr="vminsert:8480",resolved_ip=%q} 1`, ip),
				fmt.Sprintf(`vm_topology_discovery_targets{url="2:secret-url",addr="vminsert:443",resolved_ip=%q} 1`, ip),
			})
		}
		withResolver(t, resolver, func() {
			// Stop must release the worker and metric set so a new lifecycle can start.
			for range 2 {
				resolverMu.Lock()
				resolvedIP = net.ParseIP("10.20.30.40")
				calls = 0
				resolverMu.Unlock()
				st.initTargets([]*target{
					{urlLabel: "1:secret-url", addrLabel: "vminsert:8480", host: "vminsert"},
					{urlLabel: "2:secret-url", addrLabel: "vminsert:443", host: "vminsert"},
				})
				synctest.Wait()
				checkMetrics("10.20.30.40")
				checkLookups(2)

				resolverMu.Lock()
				resolvedIP = net.ParseIP("10.20.30.41")
				resolverMu.Unlock()
				time.Sleep(discoveryInterval)
				synctest.Wait()
				checkMetrics("10.20.30.41")
				checkLookups(4)

				st.stop()
				synctest.Wait()
				checkTopologyMetrics(t, nil)
				st.stop()
			}
		})
	})
}

func TestStopDuringInitialRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var st state
		started := make(chan struct{})
		release := make(chan struct{})
		resolver := &fakeResolver{
			lookupIPAddrFunc: func(ctx context.Context, _ string) ([]net.IPAddr, error) {
				close(started)
				select {
				case <-release:
					return []net.IPAddr{{IP: net.ParseIP("10.20.30.40")}}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
		withResolver(t, resolver, func() {
			st.initTargets([]*target{{urlLabel: "1:secret-url", addrLabel: "vminsert:8480", host: "vminsert"}})
			<-started
			stopped := make(chan struct{})
			go func() {
				st.stop()
				close(stopped)
			}()
			synctest.Wait()
			select {
			case <-stopped:
				t.Fatal("Stop returned before the initial refresh completed")
			default:
			}
			close(release)
			<-stopped
			checkTopologyMetrics(t, nil)
		})
	})
}

func checkTopologyMetrics(t *testing.T, want []string) {
	t.Helper()

	var buf bytes.Buffer
	metrics.WritePrometheus(&buf, false)
	var got []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(line, "vm_topology_discovery_targets{") {
			got = append(got, line)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected registered topology metrics\ngot:\n%v\nwant:\n%v", got, want)
	}
}
