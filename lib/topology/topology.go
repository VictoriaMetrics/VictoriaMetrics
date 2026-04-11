package topology

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

const (
	discoveryInterval = 30 * time.Second
	discoveryTimeout  = 5 * time.Second
)

var global state

type state struct {
	mu        sync.RWMutex
	ms        *metrics.Set
	refreshCh chan struct{}
	stopCh    chan struct{}
	wg        sync.WaitGroup
	targets   map[string]*target
}

type target struct {
	urlLabel    string
	addrLabel   string
	host        string
	resolvedIPs []string
	hasResolved bool
}

type targetSnapshot struct {
	urlLabel string
	host     string
}

type targetSample struct {
	urlLabel  string
	addrLabel string
	ip        string
}

// Register adds a remote write target for background topology discovery.
// rawURL is used for DNS resolution, sanitizedURL is used as the metric label.
func Register(rawURL, sanitizedURL string) {
	t, err := newTarget(rawURL, sanitizedURL)
	if err != nil {
		logger.Errorf("cannot register topology target for -remoteWrite.url=%q: %s", sanitizedURL, err)
		return
	}
	global.register(t)
}

// Stop stops background topology discovery and unregisters topology metrics.
func Stop() {
	global.stop()
}

func (s *state) register(t *target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.targets == nil {
		s.targets = make(map[string]*target)
	}
	if s.ms == nil {
		ms := metrics.NewSet()
		ms.RegisterMetricsWriter(s.writeMetrics)
		metrics.RegisterSet(ms)
		s.ms = ms
	}
	s.targets[t.urlLabel] = t
	if s.stopCh != nil {
		s.notifyRefreshLocked()
		return
	}

	s.refreshCh = make(chan struct{}, 1)
	s.stopCh = make(chan struct{})
	s.wg.Add(1)
	go s.run(s.stopCh, s.refreshCh)
	s.notifyRefreshLocked()
}

func (s *state) stop() {
	s.mu.Lock()
	stopCh := s.stopCh
	ms := s.ms
	s.refreshCh = nil
	s.stopCh = nil
	s.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
		s.wg.Wait()
	}
	if ms != nil {
		metrics.UnregisterSet(ms, true)
	}

	s.mu.Lock()
	s.ms = nil
	s.targets = nil
	s.mu.Unlock()
}

func (s *state) run(stopCh, refreshCh chan struct{}) {
	defer s.wg.Done()

	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-refreshCh:
			s.refresh()
		case <-ticker.C:
			s.refresh()
		case <-stopCh:
			return
		}
	}
}

func (s *state) notifyRefreshLocked() {
	select {
	case s.refreshCh <- struct{}{}:
	default:
	}
}

func (s *state) refresh() {
	snapshots := s.snapshots()
	if len(snapshots) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()

	results := make(map[string][]string, len(snapshots))
	for _, snap := range snapshots {
		resolvedIPs, ok := resolveIPs(ctx, snap.host)
		if !ok {
			continue
		}
		results[snap.urlLabel] = resolvedIPs
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for urlLabel, resolvedIPs := range results {
		t := s.targets[urlLabel]
		if t == nil {
			continue
		}
		t.applyResolvedIPs(resolvedIPs)
	}
}

func (s *state) snapshots() []targetSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]targetSnapshot, 0, len(s.targets))
	for _, t := range s.targets {
		snapshots = append(snapshots, targetSnapshot{
			urlLabel: t.urlLabel,
			host:     t.host,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].urlLabel < snapshots[j].urlLabel
	})
	return snapshots
}

func (s *state) writeMetrics(w io.Writer) {
	for _, sample := range s.samples() {
		fmt.Fprintf(w, `vm_topology_discovery_targets{url=%q,addr=%q,resolved_ip=%q} 1`+"\n",
			sample.urlLabel, sample.addrLabel, sample.ip)
	}
}

func (s *state) samples() []targetSample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	samples := make([]targetSample, 0, len(s.targets))
	for _, t := range s.targets {
		if !t.hasResolved {
			continue
		}
		for _, ip := range t.resolvedIPs {
			samples = append(samples, targetSample{
				urlLabel:  t.urlLabel,
				addrLabel: t.addrLabel,
				ip:        ip,
			})
		}
	}
	sort.Slice(samples, func(i, j int) bool {
		a, b := samples[i], samples[j]
		if a.urlLabel != b.urlLabel {
			return a.urlLabel < b.urlLabel
		}
		if a.addrLabel != b.addrLabel {
			return a.addrLabel < b.addrLabel
		}
		return a.ip < b.ip
	})
	return samples
}

func (t *target) applyResolvedIPs(resolvedIPs []string) {
	if len(resolvedIPs) == 0 {
		return
	}
	t.resolvedIPs = resolvedIPs
	t.hasResolved = true
}

func newTarget(rawURL, sanitizedURL string) (*target, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse raw URL: %w", err)
	}
	host, port, ok := getURLHostPort(u)
	if !ok {
		return nil, fmt.Errorf("cannot determine topology addr for %q", rawURL)
	}
	return &target{
		urlLabel:  sanitizedURL,
		addrLabel: joinAddr(host, port),
		host:      host,
	}, nil
}

func getURLHostPort(u *url.URL) (string, string, bool) {
	if u == nil || u.Host == "" {
		return "", "", false
	}

	host := u.Hostname()
	if host == "" {
		return "", "", false
	}

	port := u.Port()
	if port == "" && !strings.HasPrefix(host, "srv+") {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", "", false
		}
	}
	return host, port, true
}

func resolveIPs(ctx context.Context, host string) ([]string, bool) {
	if strings.HasPrefix(host, "srv+") {
		return resolveSRV(ctx, strings.TrimPrefix(host, "srv+"))
	}
	return resolveIPAddrs(ctx, host)
}

func resolveSRV(ctx context.Context, host string) ([]string, bool) {
	_, srvs, err := netutil.Resolver.LookupSRV(ctx, "", "", host)
	if err != nil {
		logger.Warnf("cannot resolve topology SRV addr %q: %s", host, err)
		return nil, false
	}
	if len(srvs) == 0 {
		logger.Warnf("missing topology SRV records for %q", host)
		return nil, false
	}

	var resolvedIPs []string
	for _, srv := range srvs {
		srvHost := strings.TrimSuffix(srv.Target, ".")
		ips, ok := resolveIPAddrs(ctx, srvHost)
		if !ok {
			continue
		}
		resolvedIPs = append(resolvedIPs, ips...)
	}
	resolvedIPs = sortAndDedupStrings(resolvedIPs)
	if len(resolvedIPs) == 0 {
		return nil, false
	}
	return resolvedIPs, true
}

func resolveIPAddrs(ctx context.Context, host string) ([]string, bool) {
	ips, err := netutil.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		logger.Warnf("cannot resolve topology IPs for %q: %s", host, err)
		return nil, false
	}
	if len(ips) == 0 {
		logger.Warnf("missing topology IPs for %q", host)
		return nil, false
	}

	resolvedIPs := make([]string, len(ips))
	for i, ip := range ips {
		resolvedIPs[i] = ip.String()
	}
	return sortAndDedupStrings(resolvedIPs), true
}

func sortAndDedupStrings(a []string) []string {
	if len(a) == 0 {
		return nil
	}
	sort.Strings(a)
	return slices.Compact(a)
}

func joinAddr(host, port string) string {
	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}
