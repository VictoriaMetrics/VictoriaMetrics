package remotewrite

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

const (
	topologyDiscoveryInterval = 30 * time.Second
	topologyDiscoveryTimeout  = 5 * time.Second
)

var (
	topologyInstance atomic.Pointer[string]

	topologyMetricsMu   sync.Mutex
	topologyMetricNames map[string]struct{}
	topologyStopCh      chan struct{}
	topologyStopWG      sync.WaitGroup
)

// SetTopologyInstance sets the instance label for topology discovery metrics.
// It must be called before Init().
func SetTopologyInstance(instance string) {
	if instance == "" {
		return
	}
	topologyInstance.Store(&instance)
}

func getTopologyInstance() string {
	if p := topologyInstance.Load(); p != nil {
		return *p
	}
	instance := getDefaultTopologyInstance()
	topologyInstance.Store(&instance)
	return instance
}

func getDefaultTopologyInstance() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func initTopologyMetrics() {
	updateTopologyMetrics()

	topologyStopCh = make(chan struct{})
	topologyStopWG.Add(1)
	go func() {
		defer topologyStopWG.Done()
		ticker := time.NewTicker(topologyDiscoveryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateTopologyMetrics()
			case <-topologyStopCh:
				return
			}
		}
	}()
}

func stopTopologyMetrics() {
	if topologyStopCh == nil {
		return
	}
	close(topologyStopCh)
	topologyStopWG.Wait()
	topologyStopCh = nil
}

type topologyTarget struct {
	addr       string
	resolvedIP string
}

func getRemoteWriteAddr(u *url.URL) string {
	host, port, ok := getRemoteWriteHostPort(u)
	if !ok {
		return ""
	}
	return joinAddr(host, port)
}

func getRemoteWriteHostPort(u *url.URL) (string, string, bool) {
	if u == nil || u.Host == "" {
		return "", "", false
	}
	host := u.Hostname()
	if host == "" {
		return "", "", false
	}
	port := getURLPort(u, host)
	if port == "" && !strings.HasPrefix(host, "srv+") {
		return "", "", false
	}
	return host, port, true
}

func updateTopologyMetrics() {
	targets := resolveTopologyTargets(*remoteWriteURLs)
	instance := getTopologyInstance()
	newNames := buildTopologyMetricNames(targets, instance)

	topologyMetricsMu.Lock()
	defer topologyMetricsMu.Unlock()

	for name := range topologyMetricNames {
		if _, ok := newNames[name]; ok {
			continue
		}
		metrics.UnregisterMetric(name)
	}
	for name := range newNames {
		if _, ok := topologyMetricNames[name]; ok {
			continue
		}
		metrics.GetOrCreateGauge(name, func() float64 {
			return 1
		})
	}
	topologyMetricNames = newNames
}

func buildTopologyMetricNames(targets []topologyTarget, instance string) map[string]struct{} {
	m := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		name := fmt.Sprintf(`vm_topology_discovery_targets{addr=%q, resolved_ip=%q, instance=%q}`, target.addr, target.resolvedIP, instance)
		m[name] = struct{}{}
	}
	return m
}

func resolveTopologyTargets(urls []string) []topologyTarget {
	ctx, cancel := context.WithTimeout(context.Background(), topologyDiscoveryTimeout)
	defer cancel()

	targets := make([]topologyTarget, 0, len(urls))
	for _, urlRaw := range urls {
		u, err := url.Parse(urlRaw)
		if err != nil {
			logger.Errorf("cannot parse -remoteWrite.url=%q: %s", urlRaw, err)
			continue
		}
		host, port, ok := getRemoteWriteHostPort(u)
		if !ok {
			continue
		}
		addr := joinAddr(host, port)
		resolved := resolveAddr(ctx, host, port)
		for _, ip := range resolved {
			targets = append(targets, topologyTarget{
				addr:       addr,
				resolvedIP: ip,
			})
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].addr == targets[j].addr {
			return targets[i].resolvedIP < targets[j].resolvedIP
		}
		return targets[i].addr < targets[j].addr
	})
	return targets
}

func getURLPort(u *url.URL, host string) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.HasPrefix(host, "srv+") {
		return ""
	}
	switch u.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func resolveAddr(ctx context.Context, host, port string) []string {
	if strings.HasPrefix(host, "srv+") {
		return resolveSRV(ctx, strings.TrimPrefix(host, "srv+"), port)
	}
	return resolveIPAddrs(ctx, host, port)
}

func resolveSRV(ctx context.Context, host, port string) []string {
	_, srvs, err := netutil.Resolver.LookupSRV(ctx, "", "", host)
	if err != nil {
		logger.Warnf("cannot resolve SRV addr %q: %s; use it literally", host, err)
		return []string{joinAddr(host, port)}
	}
	if len(srvs) == 0 {
		return []string{joinAddr(host, port)}
	}
	var addrs []string
	for _, srv := range srvs {
		srvPort := port
		if srvPort == "" && srv.Port > 0 {
			srvPort = strconv.FormatUint(uint64(srv.Port), 10)
		}
		srvHost := strings.TrimSuffix(srv.Target, ".")
		resolved := resolveIPAddrs(ctx, srvHost, srvPort)
		addrs = append(addrs, resolved...)
	}
	sort.Strings(addrs)
	return deduplicateAddrs(addrs)
}

func resolveIPAddrs(ctx context.Context, host, port string) []string {
	ips, err := netutil.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		logger.Warnf("cannot resolve IPs for %q: %s; use it literally", host, err)
		return []string{joinAddr(host, port)}
	}
	if len(ips) == 0 {
		return []string{joinAddr(host, port)}
	}
	addrs := make([]string, len(ips))
	for i, ip := range ips {
		addrs[i] = net.JoinHostPort(ip.String(), port)
	}
	sort.Strings(addrs)
	return deduplicateAddrs(addrs)
}

func deduplicateAddrs(addrs []string) []string {
	if len(addrs) < 2 {
		return addrs
	}
	dst := addrs[:1]
	for _, addr := range addrs[1:] {
		if addr == dst[len(dst)-1] {
			continue
		}
		dst = append(dst, addr)
	}
	return dst
}

func joinAddr(host, port string) string {
	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}
