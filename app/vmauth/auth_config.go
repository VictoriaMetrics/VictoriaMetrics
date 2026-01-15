package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	authConfigPath = flag.String("auth.config", "", "Path to auth config. It can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/ for details on the format of this auth config")
	configCheckInterval = flag.Duration("configCheckInterval", 0, "interval for config file re-read. "+
		"Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.")
	defaultRetryStatusCodes = flagutil.NewArrayInt("retryStatusCodes", 0, "Comma-separated list of default HTTP response status codes when vmauth re-tries the request on other backends. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing for details")
	defaultLoadBalancingPolicy = flag.String("loadBalancingPolicy", "least_loaded", "The default load balancing policy to use for backend urls specified inside url_prefix section. "+
		"Supported policies: least_loaded, first_available. See https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing")
	defaultMergeQueryArgs = flagutil.NewArrayString("mergeQueryArgs", "An optional list of client query arg names, which must be merged with args at backend urls. "+
		"The rest of client query args are replaced by the corresponding query args from backend urls for security reasons; "+
		"see https://docs.victoriametrics.com/victoriametrics/vmauth/#query-args-handling")
	discoverBackendIPsGlobal = flag.Bool("discoverBackendIPs", false, "Whether to discover backend IPs via periodic DNS queries to hostnames specified in url_prefix. "+
		"This may be useful when url_prefix points to a hostname with dynamically scaled instances behind it. See https://docs.victoriametrics.com/victoriametrics/vmauth/#discovering-backend-ips")
	discoverBackendIPsInterval = flag.Duration("discoverBackendIPsInterval", 10*time.Second, "The interval for re-discovering backend IPs if -discoverBackendIPs command-line flag is set. "+
		"Too low value may lead to DNS errors")
	httpAuthHeader = flagutil.NewArrayString("httpAuthHeader", "HTTP request header to use for obtaining authorization tokens. By default auth tokens are read from Authorization request header")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users            []UserInfo `yaml:"users,omitempty"`
	UnauthorizedUser *UserInfo  `yaml:"unauthorized_user,omitempty"`

	// ms holds all the metrics for the given AuthConfig
	ms *metrics.Set
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	Name string `yaml:"name,omitempty"`

	BearerToken string `yaml:"bearer_token,omitempty"`
	AuthToken   string `yaml:"auth_token,omitempty"`
	Username    string `yaml:"username,omitempty"`
	Password    string `yaml:"password,omitempty"`

	URLPrefix              *URLPrefix  `yaml:"url_prefix,omitempty"`
	DiscoverBackendIPs     *bool       `yaml:"discover_backend_ips,omitempty"`
	URLMaps                []URLMap    `yaml:"url_map,omitempty"`
	DumpRequestOnErrors    bool        `yaml:"dump_request_on_errors,omitempty"`
	HeadersConf            HeadersConf `yaml:",inline"`
	MaxConcurrentRequests  int         `yaml:"max_concurrent_requests,omitempty"`
	DefaultURL             *URLPrefix  `yaml:"default_url,omitempty"`
	RetryStatusCodes       []int       `yaml:"retry_status_codes,omitempty"`
	LoadBalancingPolicy    string      `yaml:"load_balancing_policy,omitempty"`
	MergeQueryArgs         []string    `yaml:"merge_query_args,omitempty"`
	DropSrcPathPrefixParts *int        `yaml:"drop_src_path_prefix_parts,omitempty"`
	TLSCAFile              string      `yaml:"tls_ca_file,omitempty"`
	TLSCertFile            string      `yaml:"tls_cert_file,omitempty"`
	TLSKeyFile             string      `yaml:"tls_key_file,omitempty"`
	TLSServerName          string      `yaml:"tls_server_name,omitempty"`
	TLSInsecureSkipVerify  *bool       `yaml:"tls_insecure_skip_verify,omitempty"`

	MetricLabels map[string]string `yaml:"metric_labels,omitempty"`

	concurrencyLimitCh      chan struct{}
	concurrencyLimitReached *metrics.Counter

	rt http.RoundTripper

	requests         *metrics.Counter
	requestErrors    *metrics.Counter
	backendRequests  *metrics.Counter
	backendErrors    *metrics.Counter
	requestsDuration *metrics.Summary
}

// HeadersConf represents config for request and response headers.
type HeadersConf struct {
	RequestHeaders   []*Header `yaml:"headers,omitempty"`
	ResponseHeaders  []*Header `yaml:"response_headers,omitempty"`
	KeepOriginalHost *bool     `yaml:"keep_original_host,omitempty"`
}

func (ui *UserInfo) beginConcurrencyLimit(ctx context.Context) error {
	select {
	case ui.concurrencyLimitCh <- struct{}{}:
		return nil
	default:
		ui.concurrencyLimitReached.Inc()

		// The per-user limit for the number of concurrent requests is reached.
		// Wait until the currently executed requests are finished, so the current request could be executed.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10078
		select {
		case ui.concurrencyLimitCh <- struct{}{}:
			return nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("cannot start executing the request during -maxQueueDuration=%s because %d concurrent requests from the user %s are executed",
					*maxQueueDuration, ui.getMaxConcurrentRequests(), ui.name())
			}

			return fmt.Errorf("cannot start executing the request because %d concurrent requests from the user %s are executed: %w",
				ui.getMaxConcurrentRequests(), ui.name(), err)
		}
	}
}

func (ui *UserInfo) endConcurrencyLimit() {
	<-ui.concurrencyLimitCh
}

func (ui *UserInfo) getMaxConcurrentRequests() int {
	mcr := ui.MaxConcurrentRequests
	if mcr <= 0 {
		mcr = *maxConcurrentPerUserRequests
	}
	return mcr
}

func (ui *UserInfo) stopHealthChecks() {
	if ui == nil {
		return
	}
	if ui.URLPrefix == nil {
		return
	}

	bus := ui.URLPrefix.bus.Load()
	bus.stopHealthChecks()
}

// Header is `Name: Value` http header, which must be added to the proxied request.
type Header struct {
	Name  string
	Value string

	sOriginal string
}

// UnmarshalYAML unmarshals h from f.
func (h *Header) UnmarshalYAML(f func(any) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	h.sOriginal = s

	n := strings.IndexByte(s, ':')
	if n < 0 {
		return fmt.Errorf("missing separator char ':' between Name and Value in the header %q; expected format - 'Name: Value'", s)
	}
	h.Name = strings.TrimSpace(s[:n])
	h.Value = strings.TrimSpace(s[n+1:])
	return nil
}

// MarshalYAML marshals h to yaml.
func (h *Header) MarshalYAML() (any, error) {
	return h.sOriginal, nil
}

// URLMap is a mapping from source paths to target urls.
type URLMap struct {
	// SrcPaths is an optional list of regular expressions, which must match the request path.
	SrcPaths []*Regex `yaml:"src_paths,omitempty"`

	// SrcHosts is an optional list of regular expressions, which must match the request hostname.
	SrcHosts []*Regex `yaml:"src_hosts,omitempty"`

	// SrcQueryArgs is an optional list of query args, which must match request URL query args.
	SrcQueryArgs []*QueryArg `yaml:"src_query_args,omitempty"`

	// SrcHeaders is an optional list of headers, which must match request headers.
	SrcHeaders []*Header `yaml:"src_headers,omitempty"`

	// UrlPrefix contains backend url prefixes for the proxied request url.
	URLPrefix *URLPrefix `yaml:"url_prefix,omitempty"`

	// DiscoverBackendIPs instructs discovering URLPrefix backend IPs via DNS.
	DiscoverBackendIPs *bool `yaml:"discover_backend_ips,omitempty"`

	// HeadersConf is the config for augmenting request and response headers.
	HeadersConf HeadersConf `yaml:",inline"`

	// RetryStatusCodes is the list of response status codes used for retrying requests.
	RetryStatusCodes []int `yaml:"retry_status_codes,omitempty"`

	// LoadBalancingPolicy is load balancing policy among UrlPrefix backends.
	LoadBalancingPolicy string `yaml:"load_balancing_policy,omitempty"`

	// MergeQueryArgs is a list of client query args, which must be merged with the existing backend query args.
	//
	// The rest of client query args are replaced with the corresponding backend query args for security reasons.
	MergeQueryArgs []string `yaml:"merge_query_args,omitempty"`

	// DropSrcPathPrefixParts is the number of `/`-delimited request path prefix parts to drop before proxying the request to backend.
	DropSrcPathPrefixParts *int `yaml:"drop_src_path_prefix_parts,omitempty"`
}

// QueryArg represents HTTP query arg
type QueryArg struct {
	Name  string
	Value *Regex

	sOriginal string
}

// UnmarshalYAML unmarshals qa from yaml.
func (qa *QueryArg) UnmarshalYAML(f func(any) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	qa.sOriginal = s

	n := strings.IndexByte(s, '=')
	if n < 0 {
		return nil
	}

	qa.Name = s[:n]
	expr := s[n+1:]
	if !strings.HasPrefix(expr, "~") {
		expr = regexp.QuoteMeta(expr)
	} else {
		expr = expr[1:]
	}

	var re Regex
	if err := yaml.Unmarshal([]byte(expr), &re); err != nil {
		return fmt.Errorf("cannot unmarshal regex for %q query arg: %w", qa.Name, err)
	}
	qa.Value = &re
	return nil
}

// MarshalYAML marshals qa to yaml.
func (qa *QueryArg) MarshalYAML() (any, error) {
	return qa.sOriginal, nil
}

// URLPrefix represents the `url_prefix` from auth config.
type URLPrefix struct {
	// requests are re-tried on other backend urls for these http response status codes
	retryStatusCodes []int

	// load balancing policy used
	loadBalancingPolicy string

	// the list of client query args, which must be merged with backend query args.
	//
	// By default backend query args replace all the client query args for security reasons.
	mergeQueryArgs []string

	// how many request path prefix parts to drop before routing the request to backendURL
	dropSrcPathPrefixParts int

	// busOriginal contains the original list of backends specified in yaml config.
	busOriginal []*url.URL

	// n is an atomic counter, which is used for balancing load among available backends.
	n atomic.Uint32

	// the list of backend urls
	//
	// the list can be dynamically updated if `discover_backend_ips` option is set.
	bus atomic.Pointer[backendURLs]

	// if this option is set, then backend ips for busOriginal are periodically re-discovered and put to bus.
	discoverBackendIPs bool

	// The next deadline for DNS-based discovery of backend IPs
	nextDiscoveryDeadline atomic.Uint64

	// vOriginal contains the original yaml value for URLPrefix.
	vOriginal any
}

func (up *URLPrefix) setLoadBalancingPolicy(loadBalancingPolicy string) error {
	switch loadBalancingPolicy {
	case "", // empty string is equivalent to least_loaded
		"least_loaded",
		"first_available":
		up.loadBalancingPolicy = loadBalancingPolicy
		return nil
	default:
		return fmt.Errorf("unexpected load_balancing_policy: %q; want least_loaded or first_available", loadBalancingPolicy)
	}
}

type backendURLs struct {
	healthChecksContext context.Context
	healthChecksCancel  func()
	healthChecksWG      sync.WaitGroup

	bus []*backendURL
}

func newBackendURLs() *backendURLs {
	ctx, cancel := context.WithCancel(context.Background())
	return &backendURLs{
		healthChecksContext: ctx,
		healthChecksCancel:  cancel,
	}
}

func (bus *backendURLs) add(u *url.URL) {
	bus.bus = append(bus.bus, &backendURL{
		url:                u,
		healthCheckContext: bus.healthChecksContext,
		healthCheckWG:      &bus.healthChecksWG,
	})
}

func (bus *backendURLs) stopHealthChecks() {
	bus.healthChecksCancel()
	bus.healthChecksWG.Wait()
}

type backendURL struct {
	broken atomic.Bool

	healthCheckContext context.Context
	healthCheckWG      *sync.WaitGroup

	concurrentRequests atomic.Int32

	url *url.URL
}

func (bu *backendURL) isBroken() bool {
	return bu.broken.Load()
}

func (bu *backendURL) setBroken() {
	if bu.broken.CompareAndSwap(false, true) {
		bu.healthCheckWG.Add(1)
		go func() {
			defer bu.healthCheckWG.Done()
			bu.runHealthCheck()
			bu.broken.Store(false)
		}()
	}
}

func (bu *backendURL) runHealthCheck() {
	port := bu.url.Port()
	if port == "" {
		port = "80"
	}
	addr := net.JoinHostPort(bu.url.Hostname(), port)

	t := time.NewTicker(*failTimeout)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			// Verify network connectivity via TCP dial before marking backend healthy.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9997
			ctx, cancel := context.WithTimeout(bu.healthCheckContext, time.Second)
			c, err := netutil.Dialer.DialContext(ctx, "tcp", addr)
			cancel()
			if err != nil {
				if errors.Is(bu.healthCheckContext.Err(), context.Canceled) {
					return
				}
				logger.Warnf("ignoring the backend at %s for %s because of dial error: %s", addr, *failTimeout, err)
				continue
			}

			_ = c.Close()
			return
		case <-bu.healthCheckContext.Done():
			return
		}
	}
}

func (bu *backendURL) get() {
	bu.concurrentRequests.Add(1)
}

func (bu *backendURL) put() {
	bu.concurrentRequests.Add(-1)
}

func (up *URLPrefix) getBackendsCount() int {
	bus := up.bus.Load()
	return len(bus.bus)
}

// getBackendURL returns the backendURL depending on the load balance policy.
//
// It can return nil if there are no backend urls available at the moment.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (up *URLPrefix) getBackendURL() *backendURL {
	up.discoverBackendAddrsIfNeeded()

	bus := up.bus.Load()
	if len(bus.bus) == 0 {
		return nil
	}

	if up.loadBalancingPolicy == "first_available" {
		return getFirstAvailableBackendURL(bus.bus)
	}
	return getLeastLoadedBackendURL(bus.bus, &up.n)
}

func (up *URLPrefix) discoverBackendAddrsIfNeeded() {
	if !up.discoverBackendIPs {
		// The discovery is disabled.
		return
	}

	ct := fasttime.UnixTimestamp()
	deadline := up.nextDiscoveryDeadline.Load()
	if ct < deadline {
		// There is no need in discovering backends.
		return
	}

	intervalSec := math.Ceil(discoverBackendIPsInterval.Seconds())
	if intervalSec <= 0 {
		intervalSec = 1
	}
	nextDeadline := ct + uint64(intervalSec)
	if !up.nextDiscoveryDeadline.CompareAndSwap(deadline, nextDeadline) {
		// Concurrent goroutine already started the discovery.
		return
	}

	// Discover ips for all the backendURLs
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(intervalSec))
	hostToAddrs := make(map[string][]string)
	for _, bu := range up.busOriginal {
		host := bu.Hostname()
		port := bu.Port()
		if hostToAddrs[host] != nil {
			// ips for the given host have been already discovered
			continue
		}

		var resolvedAddrs []string
		if strings.HasPrefix(host, "srv+") {
			// The host has the format 'srv+realhost'. Strip 'srv+' prefix before performing the lookup.
			srvHost := strings.TrimPrefix(host, "srv+")
			_, addrs, err := netutil.Resolver.LookupSRV(ctx, "", "", srvHost)
			if err != nil {
				logger.Warnf("cannot discover backend SRV records for %s: %s; use it literally", bu, err)
				resolvedAddrs = []string{host}
			} else {
				resolvedAddrs = make([]string, len(addrs))
				for i, addr := range addrs {
					hostPort := port
					if hostPort == "" && addr.Port > 0 {
						hostPort = strconv.FormatUint(uint64(addr.Port), 10)
					}
					resolvedAddrs[i] = net.JoinHostPort(addr.Target, hostPort)
				}
			}
		} else {
			addrs, err := netutil.Resolver.LookupIPAddr(ctx, host)
			if err != nil {
				logger.Warnf("cannot discover backend IPs for %s: %s; use it literally", bu, err)
				resolvedAddrs = []string{host}
			} else {
				resolvedAddrs = make([]string, len(addrs))
				for i, addr := range addrs {
					resolvedAddrs[i] = net.JoinHostPort(addr.String(), port)
				}
			}
		}
		// sort resolvedAddrs, so they could be compared below in areEqualBackendURLs()
		sort.Strings(resolvedAddrs)
		hostToAddrs[host] = resolvedAddrs
	}
	cancel()

	// generate new backendURLs for the resolved IPs
	busNew := newBackendURLs()
	for _, bu := range up.busOriginal {
		host := bu.Hostname()
		for _, addr := range hostToAddrs[host] {
			buCopy := *bu
			buCopy.Host = addr
			busNew.add(&buCopy)
		}
	}

	bus := up.bus.Load()
	if areEqualBackendURLs(bus.bus, busNew.bus) {
		return
	}

	// Store new backend urls
	up.bus.Store(busNew)
	bus.stopHealthChecks()
}

func areEqualBackendURLs(a, b []*backendURL) bool {
	if len(a) != len(b) {
		return false
	}
	for i, aURL := range a {
		bURL := b[i]
		if aURL.url.String() != bURL.url.String() {
			return false
		}
	}
	return true
}

// getFirstAvailableBackendURL returns the first available backendURL, which isn't broken.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func getFirstAvailableBackendURL(bus []*backendURL) *backendURL {
	bu := bus[0]
	if !bu.isBroken() {
		// Fast path - send the request to the first url.
		bu.get()
		return bu
	}

	// Slow path - the first url is temporarily unavailable. Fall back to the remaining urls.
	for i := 1; i < len(bus); i++ {
		if !bus[i].isBroken() {
			bu = bus[i]
			bu.get()
			return bu
		}
	}
	return nil
}

// getLeastLoadedBackendURL returns a non-broken backendURL with the lowest number of concurrent requests.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func getLeastLoadedBackendURL(bus []*backendURL, atomicCounter *atomic.Uint32) *backendURL {
	if len(bus) == 1 {
		// Fast path - return the only backend url.
		bu := bus[0]
		if bu.isBroken() {
			return nil
		}
		bu.get()
		return bu
	}

	// Slow path - select other backend urls.
	n := atomicCounter.Add(1) - 1
	for i := uint32(0); i < uint32(len(bus)); i++ {
		idx := (n + i) % uint32(len(bus))
		bu := bus[idx]
		if bu.isBroken() {
			continue
		}

		// The Load() in front of CompareAndSwap() avoids CAS overhead for items with values bigger than 0.
		if bu.concurrentRequests.Load() == 0 && bu.concurrentRequests.CompareAndSwap(0, 1) {
			atomicCounter.CompareAndSwap(n+1, idx+1)
			// There is no need in the call bu.get(), because we already incremented bu.concrrentRequests above.
			return bu
		}
	}

	// Slow path - return the backend with the minimum number of concurrently executed requests.
	buMinIdx := n % uint32(len(bus))
	minRequests := bus[buMinIdx].concurrentRequests.Load()
	for i := uint32(1); i < uint32(len(bus)); i++ {
		idx := (n + i) % uint32(len(bus))
		bu := bus[idx]
		if bu.isBroken() {
			continue
		}

		reqs := bu.concurrentRequests.Load()
		if reqs < minRequests || bus[buMinIdx].isBroken() {
			buMinIdx = idx
			minRequests = reqs
		}
	}
	buMin := bus[buMinIdx]
	if buMin.isBroken() {
		return nil
	}
	buMin.get()
	atomicCounter.CompareAndSwap(n+1, buMinIdx+1)
	return buMin
}

// UnmarshalYAML unmarshals up from yaml.
func (up *URLPrefix) UnmarshalYAML(f func(any) error) error {
	var v any
	if err := f(&v); err != nil {
		return err
	}
	up.vOriginal = v

	var urls []string
	switch x := v.(type) {
	case string:
		urls = []string{x}
	case []any:
		if len(x) == 0 {
			return fmt.Errorf("`url_prefix` must contain at least a single url")
		}
		us := make([]string, len(x))
		for i, xx := range x {
			s, ok := xx.(string)
			if !ok {
				return fmt.Errorf("`url_prefix` must contain array of strings; got %T", xx)
			}
			us[i] = s
		}
		urls = us
	default:
		return fmt.Errorf("unexpected type for `url_prefix`: %T; want string or []string", v)
	}

	bus := make([]*url.URL, len(urls))
	for i, u := range urls {
		pu, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("cannot unmarshal %q into url: %w", u, err)
		}
		bus[i] = pu
	}
	up.busOriginal = bus
	return nil
}

// MarshalYAML marshals up to yaml.
func (up *URLPrefix) MarshalYAML() (any, error) {
	return up.vOriginal, nil
}

// Regex represents a regex
type Regex struct {
	re *regexp.Regexp

	sOriginal string
}

func (r *Regex) match(s string) bool {
	prefix, ok := r.re.LiteralPrefix()
	if ok {
		// Fast path - literal match
		return s == prefix
	}
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	return r.re.MatchString(s)
}

// UnmarshalYAML implements yaml.Unmarshaler
func (r *Regex) UnmarshalYAML(f func(any) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	r.sOriginal = s

	sAnchored := "^(?:" + s + ")$"
	re, err := regexp.Compile(sAnchored)
	if err != nil {
		return fmt.Errorf("cannot build regexp from %q: %w", s, err)
	}
	r.re = re
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (r *Regex) MarshalYAML() (any, error) {
	return r.sOriginal, nil
}

var (
	configReloads      = metrics.NewCounter(`vmauth_config_last_reload_total`)
	configReloadErrors = metrics.NewCounter(`vmauth_config_last_reload_errors_total`)
	configSuccess      = metrics.NewGauge(`vmauth_config_last_reload_successful`, nil)
	configTimestamp    = metrics.NewCounter(`vmauth_config_last_reload_success_timestamp_seconds`)
)

func initAuthConfig() {
	if len(*authConfigPath) == 0 {
		logger.Fatalf("missing required `-auth.config` command-line flag")
	}

	// Register SIGHUP handler for config re-read just before readAuthConfig call.
	// This guarantees that the config will be re-read if the signal arrives during readAuthConfig call.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	_, err := reloadAuthConfig()
	if err != nil {
		logger.Fatalf("cannot load auth config: %s", err)
	}

	configSuccess.Set(1)
	configTimestamp.Set(fasttime.UnixTimestamp())

	stopCh = make(chan struct{})
	authConfigWG.Add(1)
	go func() {
		defer authConfigWG.Done()
		authConfigReloader(sighupCh)
	}()
}

func stopAuthConfig() {
	close(stopCh)
	authConfigWG.Wait()
}

func authConfigReloader(sighupCh <-chan os.Signal) {
	var refreshCh <-chan time.Time
	// initialize auth refresh interval
	if *configCheckInterval > 0 {
		ticker := time.NewTicker(*configCheckInterval)
		defer ticker.Stop()
		refreshCh = ticker.C
	}

	updateFn := func() {
		configReloads.Inc()
		updated, err := reloadAuthConfig()
		if err != nil {
			logger.Errorf("failed to load auth config; using the last successfully loaded config; error: %s", err)
			configSuccess.Set(0)
			configReloadErrors.Inc()
			return
		}
		configSuccess.Set(1)
		if updated {
			configTimestamp.Set(fasttime.UnixTimestamp())
		}
	}

	for {
		select {
		case <-stopCh:
			return
		case <-refreshCh:
			updateFn()
		case <-sighupCh:
			logger.Infof("SIGHUP received; loading -auth.config=%q", *authConfigPath)
			updateFn()
		}
	}
}

var (
	// authConfigData stores the yaml definition for this config.
	// authConfigData needs to be updated each time authConfig is updated.
	authConfigData atomic.Pointer[[]byte]

	// authConfig contains the currently loaded auth config
	authConfig atomic.Pointer[AuthConfig]

	// authUsers contains the currently loaded auth users
	authUsers atomic.Pointer[map[string]*UserInfo]

	authConfigWG sync.WaitGroup
	stopCh       chan struct{}
)

// reloadAuthConfig loads and applies the config from *authConfigPath.
// It returns bool value to identify if new config was applied.
// The config can be not applied if there is a parsing error
// or if there are no changes to the current authConfig.
func reloadAuthConfig() (bool, error) {
	data, err := fscore.ReadFileOrHTTP(*authConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read -auth.config=%q: %w", *authConfigPath, err)
	}

	ok, err := reloadAuthConfigData(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse -auth.config=%q: %w", *authConfigPath, err)
	}
	if !ok {
		return false, nil
	}

	mp := authUsers.Load()
	logger.Infof("loaded information about %d users from -auth.config=%q", len(*mp), *authConfigPath)
	return true, nil
}

func reloadAuthConfigData(data []byte) (bool, error) {
	oldData := authConfigData.Load()
	if oldData != nil && bytes.Equal(data, *oldData) {
		// there are no updates in the config - skip reloading.
		return false, nil
	}

	ac, err := parseAuthConfig(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse auth config: %w", err)
	}

	m, err := parseAuthConfigUsers(ac)
	if err != nil {
		return false, fmt.Errorf("failed to parse users from auth config: %w", err)
	}

	acPrev := authConfig.Load()
	if acPrev != nil {
		acPrev.UnauthorizedUser.stopHealthChecks()
		for i := range acPrev.Users {
			acPrev.Users[i].stopHealthChecks()
		}

		metrics.UnregisterSet(acPrev.ms, true)
	}
	metrics.RegisterSet(ac.ms)

	authConfig.Store(ac)
	authConfigData.Store(&data)
	authUsers.Store(&m)

	return true, nil
}

func parseAuthConfig(data []byte) (*AuthConfig, error) {
	data = envtemplate.ReplaceBytes(data)
	ac := &AuthConfig{
		ms: metrics.NewSet(),
	}
	if err := yaml.UnmarshalStrict(data, ac); err != nil {
		return nil, fmt.Errorf("cannot unmarshal AuthConfig data: %w", err)
	}

	ui := ac.UnauthorizedUser
	if ui != nil {
		if ui.Username != "" {
			return nil, fmt.Errorf("field username can't be specified for unauthorized_user section")
		}
		if ui.Password != "" {
			return nil, fmt.Errorf("field password can't be specified for unauthorized_user section")
		}
		if ui.BearerToken != "" {
			return nil, fmt.Errorf("field bearer_token can't be specified for unauthorized_user section")
		}
		if ui.AuthToken != "" {
			return nil, fmt.Errorf("field auth_token can't be specified for unauthorized_user section")
		}
		if ui.Name != "" {
			return nil, fmt.Errorf("field name can't be specified for unauthorized_user section")
		}
		if err := ui.initURLs(); err != nil {
			return nil, err
		}

		metricLabels, err := ui.getMetricLabels()
		if err != nil {
			return nil, fmt.Errorf("cannot parse metric_labels for unauthorized_user: %w", err)
		}
		ui.requests = ac.ms.NewCounter(`vmauth_unauthorized_user_requests_total` + metricLabels)
		ui.requestErrors = ac.ms.NewCounter(`vmauth_unauthorized_user_request_errors_total` + metricLabels)
		ui.backendRequests = ac.ms.NewCounter(`vmauth_unauthorized_user_request_backend_requests_total` + metricLabels)
		ui.backendErrors = ac.ms.NewCounter(`vmauth_unauthorized_user_request_backend_errors_total` + metricLabels)
		ui.requestsDuration = ac.ms.NewSummary(`vmauth_unauthorized_user_request_duration_seconds` + metricLabels)
		ui.concurrencyLimitCh = make(chan struct{}, ui.getMaxConcurrentRequests())
		ui.concurrencyLimitReached = ac.ms.NewCounter(`vmauth_unauthorized_user_concurrent_requests_limit_reached_total` + metricLabels)
		_ = ac.ms.NewGauge(`vmauth_unauthorized_user_concurrent_requests_capacity`+metricLabels, func() float64 {
			return float64(cap(ui.concurrencyLimitCh))
		})
		_ = ac.ms.NewGauge(`vmauth_unauthorized_user_concurrent_requests_current`+metricLabels, func() float64 {
			return float64(len(ui.concurrencyLimitCh))
		})

		rt, err := newRoundTripper(ui.TLSCAFile, ui.TLSCertFile, ui.TLSKeyFile, ui.TLSServerName, ui.TLSInsecureSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize HTTP RoundTripper: %w", err)
		}
		ui.rt = rt
	}
	return ac, nil
}

func parseAuthConfigUsers(ac *AuthConfig) (map[string]*UserInfo, error) {
	uis := ac.Users
	byAuthToken := make(map[string]*UserInfo, len(uis))
	if len(uis) == 0 && ac.UnauthorizedUser == nil {
		// fast path for empty configuration
		return byAuthToken, nil
	}
	for i := range uis {
		ui := &uis[i]
		ats, err := getAuthTokens(ui.AuthToken, ui.BearerToken, ui.Username, ui.Password)
		if err != nil {
			return nil, err
		}
		for _, at := range ats {
			if uiOld := byAuthToken[at]; uiOld != nil {
				return nil, fmt.Errorf("duplicate auth token=%q found for username=%q, name=%q; the previous one is set for username=%q, name=%q",
					at, ui.Username, ui.Name, uiOld.Username, uiOld.Name)
			}
		}
		if err := ui.initURLs(); err != nil {
			return nil, err
		}

		metricLabels, err := ui.getMetricLabels()
		if err != nil {
			return nil, fmt.Errorf("cannot parse metric_labels: %w", err)
		}
		ui.requests = ac.ms.GetOrCreateCounter(`vmauth_user_requests_total` + metricLabels)
		ui.requestErrors = ac.ms.GetOrCreateCounter(`vmauth_user_request_errors_total` + metricLabels)
		ui.backendRequests = ac.ms.GetOrCreateCounter(`vmauth_user_request_backend_requests_total` + metricLabels)
		ui.backendErrors = ac.ms.GetOrCreateCounter(`vmauth_user_request_backend_errors_total` + metricLabels)
		ui.requestsDuration = ac.ms.GetOrCreateSummary(`vmauth_user_request_duration_seconds` + metricLabels)
		mcr := ui.getMaxConcurrentRequests()
		ui.concurrencyLimitCh = make(chan struct{}, mcr)
		ui.concurrencyLimitReached = ac.ms.GetOrCreateCounter(`vmauth_user_concurrent_requests_limit_reached_total` + metricLabels)
		_ = ac.ms.GetOrCreateGauge(`vmauth_user_concurrent_requests_capacity`+metricLabels, func() float64 {
			return float64(cap(ui.concurrencyLimitCh))
		})
		_ = ac.ms.GetOrCreateGauge(`vmauth_user_concurrent_requests_current`+metricLabels, func() float64 {
			return float64(len(ui.concurrencyLimitCh))
		})

		rt, err := newRoundTripper(ui.TLSCAFile, ui.TLSCertFile, ui.TLSKeyFile, ui.TLSServerName, ui.TLSInsecureSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize HTTP RoundTripper: %w", err)
		}
		ui.rt = rt

		for _, at := range ats {
			byAuthToken[at] = ui
		}
	}
	return byAuthToken, nil
}

var labelNameRegexp = regexp.MustCompile("^[a-zA-Z_:.][a-zA-Z0-9_:.]*$")

func (ui *UserInfo) getMetricLabels() (string, error) {
	name := ui.name()
	if len(name) == 0 && len(ui.MetricLabels) == 0 {
		// fast path
		return "", nil
	}
	labels := make([]string, 0, len(ui.MetricLabels)+1)
	if len(name) > 0 {
		labels = append(labels, fmt.Sprintf(`username=%q`, name))
	}
	for k, v := range ui.MetricLabels {
		if !labelNameRegexp.MatchString(k) {
			return "", fmt.Errorf("incorrect label name=%q, it must match regex=%q for user=%q", k, labelNameRegexp, name)
		}
		labels = append(labels, fmt.Sprintf(`%s=%q`, k, v))
	}
	sort.Strings(labels)
	labelsStr := "{" + strings.Join(labels, ",") + "}"
	return labelsStr, nil
}

func (ui *UserInfo) initURLs() error {
	retryStatusCodes := defaultRetryStatusCodes.Values()
	loadBalancingPolicy := *defaultLoadBalancingPolicy
	mergeQueryArgs := *defaultMergeQueryArgs
	dropSrcPathPrefixParts := 0
	discoverBackendIPs := *discoverBackendIPsGlobal
	if ui.RetryStatusCodes != nil {
		retryStatusCodes = ui.RetryStatusCodes
	}
	if ui.LoadBalancingPolicy != "" {
		loadBalancingPolicy = ui.LoadBalancingPolicy
	}
	if len(ui.MergeQueryArgs) != 0 {
		mergeQueryArgs = ui.MergeQueryArgs
	}
	if ui.DropSrcPathPrefixParts != nil {
		dropSrcPathPrefixParts = *ui.DropSrcPathPrefixParts
	}
	if ui.DiscoverBackendIPs != nil {
		discoverBackendIPs = *ui.DiscoverBackendIPs
	}

	up := ui.URLPrefix
	if up != nil {
		if err := up.sanitizeAndInitialize(); err != nil {
			return err
		}
		up.retryStatusCodes = retryStatusCodes
		up.dropSrcPathPrefixParts = dropSrcPathPrefixParts
		up.discoverBackendIPs = discoverBackendIPs
		if err := up.setLoadBalancingPolicy(loadBalancingPolicy); err != nil {
			return err
		}
		up.mergeQueryArgs = mergeQueryArgs
	}
	if ui.DefaultURL != nil {
		if err := ui.DefaultURL.sanitizeAndInitialize(); err != nil {
			return err
		}
	}
	for _, e := range ui.URLMaps {
		if len(e.SrcPaths) == 0 && len(e.SrcHosts) == 0 && len(e.SrcQueryArgs) == 0 && len(e.SrcHeaders) == 0 {
			return fmt.Errorf("missing `src_paths`, `src_hosts`, `src_query_args` and `src_headers` in `url_map`")
		}
		if e.URLPrefix == nil {
			return fmt.Errorf("missing `url_prefix` in `url_map`")
		}
		if err := e.URLPrefix.sanitizeAndInitialize(); err != nil {
			return err
		}
		rscs := retryStatusCodes
		lbp := loadBalancingPolicy
		mqa := mergeQueryArgs
		dsp := dropSrcPathPrefixParts
		dbd := discoverBackendIPs
		if e.RetryStatusCodes != nil {
			rscs = e.RetryStatusCodes
		}
		if e.LoadBalancingPolicy != "" {
			lbp = e.LoadBalancingPolicy
		}
		if len(e.MergeQueryArgs) != 0 {
			mqa = e.MergeQueryArgs
		}
		if e.DropSrcPathPrefixParts != nil {
			dsp = *e.DropSrcPathPrefixParts
		}
		if e.DiscoverBackendIPs != nil {
			dbd = *e.DiscoverBackendIPs
		}
		e.URLPrefix.retryStatusCodes = rscs
		if err := e.URLPrefix.setLoadBalancingPolicy(lbp); err != nil {
			return err
		}
		e.URLPrefix.mergeQueryArgs = mqa
		e.URLPrefix.dropSrcPathPrefixParts = dsp
		e.URLPrefix.discoverBackendIPs = dbd
	}
	if len(ui.URLMaps) == 0 && ui.URLPrefix == nil {
		return fmt.Errorf("missing `url_prefix` or `url_map`")
	}
	return nil
}

func (ui *UserInfo) name() string {
	if ui.Name != "" {
		return ui.Name
	}
	if ui.Username != "" {
		return ui.Username
	}
	if ui.BearerToken != "" {
		h := xxhash.Sum64([]byte(ui.BearerToken))
		return fmt.Sprintf("bearer_token:hash:%016X", h)
	}
	if ui.AuthToken != "" {
		h := xxhash.Sum64([]byte(ui.AuthToken))
		return fmt.Sprintf("auth_token:hash:%016X", h)
	}
	return ""
}

func getAuthTokens(authToken, bearerToken, username, password string) ([]string, error) {
	if authToken != "" {
		if bearerToken != "" {
			return nil, fmt.Errorf("bearer_token cannot be specified if auth_token is set")
		}
		if username != "" || password != "" {
			return nil, fmt.Errorf("username and password cannot be specified if auth_token is set")
		}
		at := getHTTPAuthToken(authToken)
		return []string{at}, nil
	}
	if bearerToken != "" {
		if username != "" || password != "" {
			return nil, fmt.Errorf("username and password cannot be specified if bearer_token is set")
		}
		// Accept the bearerToken as Basic Auth username with empty password
		at1 := getHTTPAuthBearerToken(bearerToken)
		at2 := getHTTPAuthBasicToken(bearerToken, "")
		return []string{at1, at2}, nil
	}
	if username != "" {
		at := getHTTPAuthBasicToken(username, password)
		return []string{at}, nil
	}
	return nil, fmt.Errorf("missing authorization options; bearer_token or username must be set")
}

func getHTTPAuthToken(authToken string) string {
	return "http_auth:" + authToken
}

func getHTTPAuthBearerToken(bearerToken string) string {
	return "http_auth:Bearer " + bearerToken
}

func getHTTPAuthBasicToken(username, password string) string {
	token := username + ":" + password
	token64 := base64.StdEncoding.EncodeToString([]byte(token))
	return "http_auth:Basic " + token64
}

var defaultHeaderNames = []string{"Authorization"}

func getAuthTokensFromRequest(r *http.Request) []string {
	var ats []string

	// Obtain possible auth tokens from one of the allowed auth headers
	headerNames := *httpAuthHeader
	if len(headerNames) == 0 {
		headerNames = defaultHeaderNames
	}
	for _, headerName := range headerNames {
		if ah := r.Header.Get(headerName); ah != "" {
			if strings.HasPrefix(ah, "Token ") {
				// Handle InfluxDB's proprietary token authentication scheme as a bearer token authentication
				// See https://docs.influxdata.com/influxdb/v2.0/api/
				ah = strings.Replace(ah, "Token", "Bearer", 1)
			}
			at := "http_auth:" + ah
			ats = append(ats, at)
		}
	}

	// Authorization via http://user:pass@hosname/path
	if u := r.URL.User; u != nil && u.Username() != "" {
		username := u.Username()
		password, _ := u.Password()
		at := getHTTPAuthBasicToken(username, password)
		ats = append(ats, at)
	}

	return ats
}

func (up *URLPrefix) sanitizeAndInitialize() error {
	for i, bu := range up.busOriginal {
		puNew, err := sanitizeURLPrefix(bu)
		if err != nil {
			return err
		}
		up.busOriginal[i] = puNew
	}

	// Initialize up.bus
	bus := newBackendURLs()
	for _, bu := range up.busOriginal {
		bus.add(bu)
	}
	up.bus.Store(bus)

	return nil
}

func sanitizeURLPrefix(urlPrefix *url.URL) (*url.URL, error) {
	// Validate urlPrefix
	if urlPrefix.Scheme != "http" && urlPrefix.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme for `url_prefix: %q`: %q; must be `http` or `https`", urlPrefix, urlPrefix.Scheme)
	}
	if urlPrefix.Host == "" {
		return nil, fmt.Errorf("missing hostname in `url_prefix %q`", urlPrefix.Host)
	}
	return urlPrefix, nil
}
