package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
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
		"See https://docs.victoriametrics.com/vmauth/ for details on the format of this auth config")
	configCheckInterval = flag.Duration("configCheckInterval", 0, "interval for config file re-read. "+
		"Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.")
	defaultRetryStatusCodes = flagutil.NewArrayInt("retryStatusCodes", 0, "Comma-separated list of default HTTP response status codes when vmauth re-tries the request on other backends. "+
		"See https://docs.victoriametrics.com/vmauth/#load-balancing for details")
	defaultLoadBalancingPolicy = flag.String("loadBalancingPolicy", "least_loaded", "The default load balancing policy to use for backend urls specified inside url_prefix section. "+
		"Supported policies: least_loaded, first_available. See https://docs.victoriametrics.com/vmauth/#load-balancing")
	discoverBackendIPsGlobal = flag.Bool("discoverBackendIPs", false, "Whether to discover backend IPs via periodic DNS queries to hostnames specified in url_prefix. "+
		"This may be useful when url_prefix points to a hostname with dynamically scaled instances behind it. See https://docs.victoriametrics.com/vmauth/#discovering-backend-ips")
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
	HeadersConf            HeadersConf `yaml:",inline"`
	MaxConcurrentRequests  int         `yaml:"max_concurrent_requests,omitempty"`
	DefaultURL             *URLPrefix  `yaml:"default_url,omitempty"`
	RetryStatusCodes       []int       `yaml:"retry_status_codes,omitempty"`
	LoadBalancingPolicy    string      `yaml:"load_balancing_policy,omitempty"`
	DropSrcPathPrefixParts *int        `yaml:"drop_src_path_prefix_parts,omitempty"`
	TLSCAFile              string      `yaml:"tls_ca_file,omitempty"`
	TLSCertFile            string      `yaml:"tls_cert_file,omitempty"`
	TLSKeyFile             string      `yaml:"tls_key_file,omitempty"`
	TLSServerName          string      `yaml:"tls_server_name,omitempty"`
	TLSInsecureSkipVerify  *bool       `yaml:"tls_insecure_skip_verify,omitempty"`

	MetricLabels map[string]string `yaml:"metric_labels,omitempty"`

	concurrencyLimitCh      chan struct{}
	concurrencyLimitReached *metrics.Counter
	overrideHostHeader      bool

	rt http.RoundTripper

	requests         *metrics.Counter
	backendErrors    *metrics.Counter
	requestsDuration *metrics.Summary
}

// HeadersConf represents config for request and response headers.
type HeadersConf struct {
	RequestHeaders  []*Header `yaml:"headers,omitempty"`
	ResponseHeaders []*Header `yaml:"response_headers,omitempty"`
}

func (ui *UserInfo) beginConcurrencyLimit() error {
	select {
	case ui.concurrencyLimitCh <- struct{}{}:
		return nil
	default:
		ui.concurrencyLimitReached.Inc()
		return fmt.Errorf("cannot handle more than %d concurrent requests from user %s", ui.getMaxConcurrentRequests(), ui.name())
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

// Header is `Name: Value` http header, which must be added to the proxied request.
type Header struct {
	Name  string
	Value string

	sOriginal string
}

// UnmarshalYAML unmarshals h from f.
func (h *Header) UnmarshalYAML(f func(interface{}) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
	h.sOriginal = s

	n := strings.IndexByte(s, ':')
	if n < 0 {
		return fmt.Errorf("missing speparator char ':' between Name and Value in the header %q; expected format - 'Name: Value'", s)
	}
	h.Name = strings.TrimSpace(s[:n])
	h.Value = strings.TrimSpace(s[n+1:])
	return nil
}

// MarshalYAML marshals h to yaml.
func (h *Header) MarshalYAML() (interface{}, error) {
	return h.sOriginal, nil
}

func overrideHostHeader(headers []*Header) bool {
	for _, h := range headers {
		if h.Name == "Host" && h.Value == "" {
			return true
		}
	}
	return false
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

	// HeadersConf is the config for augumenting request and response headers.
	HeadersConf HeadersConf `yaml:",inline"`

	// RetryStatusCodes is the list of response status codes used for retrying requests.
	RetryStatusCodes []int `yaml:"retry_status_codes,omitempty"`

	// LoadBalancingPolicy is load balancing policy among UrlPrefix backends.
	LoadBalancingPolicy string `yaml:"load_balancing_policy,omitempty"`

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
func (qa *QueryArg) UnmarshalYAML(f func(interface{}) error) error {
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
func (qa *QueryArg) MarshalYAML() (interface{}, error) {
	return qa.sOriginal, nil
}

// URLPrefix represents passed `url_prefix`
type URLPrefix struct {
	// requests are re-tried on other backend urls for these http response status codes
	retryStatusCodes []int

	// load balancing policy used
	loadBalancingPolicy string

	// how many request path prefix parts to drop before routing the request to backendURL
	dropSrcPathPrefixParts int

	// busOriginal contains the original list of backends specified in yaml config.
	busOriginal []*url.URL

	// n is an atomic counter, which is used for balancing load among available backends.
	n atomic.Uint32

	// the list of backend urls
	//
	// the list can be dynamically updated if `discover_backend_ips` option is set.
	bus atomic.Pointer[[]*backendURL]

	// if this option is set, then backend ips for busOriginal are periodically re-discovered and put to bus.
	discoverBackendIPs bool

	// The next deadline for DNS-based discovery of backend IPs
	nextDiscoveryDeadline atomic.Uint64

	// vOriginal contains the original yaml value for URLPrefix.
	vOriginal interface{}
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

type backendURL struct {
	brokenDeadline     atomic.Uint64
	concurrentRequests atomic.Int32

	url *url.URL
}

func (bu *backendURL) isBroken() bool {
	ct := fasttime.UnixTimestamp()
	return ct < bu.brokenDeadline.Load()
}

func (bu *backendURL) setBroken() {
	deadline := fasttime.UnixTimestamp() + uint64((*failTimeout).Seconds())
	bu.brokenDeadline.Store(deadline)
}

func (bu *backendURL) get() {
	bu.concurrentRequests.Add(1)
}

func (bu *backendURL) put() {
	bu.concurrentRequests.Add(-1)
}

func (up *URLPrefix) getBackendsCount() int {
	pbus := up.bus.Load()
	return len(*pbus)
}

// getBackendURL returns the backendURL depending on the load balance policy.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (up *URLPrefix) getBackendURL() *backendURL {
	up.discoverBackendAddrsIfNeeded()

	pbus := up.bus.Load()
	bus := *pbus
	if up.loadBalancingPolicy == "first_available" {
		return getFirstAvailableBackendURL(bus)
	}
	return getLeastLoadedBackendURL(bus, &up.n)
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
		if hostToAddrs[host] != nil {
			// ips for the given host have been already discovered
			continue
		}
		var resolvedAddrs []string
		if strings.HasPrefix(host, "srv+") {
			// The host has the format 'srv+realhost'. Strip 'srv+' prefix before performing the lookup.
			host = strings.TrimPrefix(host, "srv+")
			_, addrs, err := netutil.Resolver.LookupSRV(ctx, "", "", host)
			if err != nil {
				logger.Warnf("cannot discover backend SRV records for %s: %s; use it literally", bu, err)
				resolvedAddrs = []string{host}
			} else {
				resolvedAddrs = make([]string, len(addrs))
				for i, addr := range addrs {
					resolvedAddrs[i] = fmt.Sprintf("%s:%d", addr.Target, addr.Port)
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
					resolvedAddrs[i] = addr.String()
				}
			}
		}
		// sort resolvedAddrs, so they could be compared below in areEqualBackendURLs()
		sort.Strings(resolvedAddrs)
		hostToAddrs[host] = resolvedAddrs
	}
	cancel()

	// generate new backendURLs for the resolved IPs
	var busNew []*backendURL
	for _, bu := range up.busOriginal {
		host := bu.Hostname()
		host = strings.TrimPrefix(host, "srv+")
		port := bu.Port()
		for _, addr := range hostToAddrs[host] {
			buCopy := *bu
			buCopy.Host = addr
			if port != "" {
				if n := strings.IndexByte(buCopy.Host, ':'); n >= 0 {
					// Drop the discovered port and substitute it the the port specified in bu.
					buCopy.Host = buCopy.Host[:n]
				}
				buCopy.Host += ":" + port
			}
			busNew = append(busNew, &backendURL{
				url: &buCopy,
			})
		}
	}

	pbus := up.bus.Load()
	if areEqualBackendURLs(*pbus, busNew) {
		return
	}

	// Store new backend urls
	up.bus.Store(&busNew)
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

	// Slow path - the first url is temporarily unavailabel. Fall back to the remaining urls.
	for i := 1; i < len(bus); i++ {
		if !bus[i].isBroken() {
			bu = bus[i]
			break
		}
	}
	bu.get()
	return bu
}

// getLeastLoadedBackendURL returns the backendURL with the minimum number of concurrent requests.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func getLeastLoadedBackendURL(bus []*backendURL, atomicCounter *atomic.Uint32) *backendURL {
	if len(bus) == 1 {
		// Fast path - return the only backend url.
		bu := bus[0]
		bu.get()
		return bu
	}

	// Slow path - select other backend urls.
	n := atomicCounter.Add(1)

	for i := uint32(0); i < uint32(len(bus)); i++ {
		idx := (n + i) % uint32(len(bus))
		bu := bus[idx]
		if bu.isBroken() {
			continue
		}
		if bu.concurrentRequests.Load() == 0 {
			// Fast path - return the backend with zero concurrently executed requests.
			// Do not use CompareAndSwap() instead of Load(), since it is much slower on systems with many CPU cores.
			bu.concurrentRequests.Add(1)
			return bu
		}
	}

	// Slow path - return the backend with the minimum number of concurrently executed requests.
	buMin := bus[n%uint32(len(bus))]
	minRequests := buMin.concurrentRequests.Load()
	for _, bu := range bus {
		if bu.isBroken() {
			continue
		}
		if n := bu.concurrentRequests.Load(); n < minRequests {
			buMin = bu
			minRequests = n
		}
	}
	buMin.get()
	return buMin
}

// UnmarshalYAML unmarshals up from yaml.
func (up *URLPrefix) UnmarshalYAML(f func(interface{}) error) error {
	var v interface{}
	if err := f(&v); err != nil {
		return err
	}
	up.vOriginal = v

	var urls []string
	switch x := v.(type) {
	case string:
		urls = []string{x}
	case []interface{}:
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
func (up *URLPrefix) MarshalYAML() (interface{}, error) {
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
func (r *Regex) UnmarshalYAML(f func(interface{}) error) error {
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
func (r *Regex) MarshalYAML() (interface{}, error) {
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

	_, err := loadAuthConfig()
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
		updated, err := loadAuthConfig()
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

// authConfigData stores the yaml definition for this config.
// authConfigData needs to be updated each time authConfig is updated.
var authConfigData atomic.Pointer[[]byte]

var (
	authConfig   atomic.Pointer[AuthConfig]
	authUsers    atomic.Pointer[map[string]*UserInfo]
	authConfigWG sync.WaitGroup
	stopCh       chan struct{}
)

// loadAuthConfig loads and applies the config from *authConfigPath.
// It returns bool value to identify if new config was applied.
// The config can be not applied if there is a parsing error
// or if there are no changes to the current authConfig.
func loadAuthConfig() (bool, error) {
	data, err := fscore.ReadFileOrHTTP(*authConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read -auth.config=%q: %w", *authConfigPath, err)
	}

	oldData := authConfigData.Load()
	if oldData != nil && bytes.Equal(data, *oldData) {
		// there are no updates in the config - skip reloading.
		return false, nil
	}

	ac, err := parseAuthConfig(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse -auth.config=%q: %w", *authConfigPath, err)
	}

	m, err := parseAuthConfigUsers(ac)
	if err != nil {
		return false, fmt.Errorf("failed to parse users from -auth.config=%q: %w", *authConfigPath, err)
	}
	logger.Infof("loaded information about %d users from -auth.config=%q", len(m), *authConfigPath)

	prevAc := authConfig.Load()
	if prevAc != nil {
		metrics.UnregisterSet(prevAc.ms)
	}
	metrics.RegisterSet(ac.ms)
	authConfig.Store(ac)
	authConfigData.Store(&data)
	authUsers.Store(&m)
	if prevAc != nil {
		// explicilty unregister metrics, since all summary type metrics
		// are registered at global state of metrics package
		// and must be removed from it to release memory.
		// Metrics must be unregistered only after atomic.Value.Store calls above
		// Otherwise it may lead to metric gaps, since UnregisterAllMetrics is slow operation
		prevAc.ms.UnregisterAllMetrics()
	}

	return true, nil
}

func parseAuthConfig(data []byte) (*AuthConfig, error) {
	data, err := envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment vars: %w", err)
	}
	ac := &AuthConfig{
		ms: metrics.NewSet(),
	}
	if err = yaml.UnmarshalStrict(data, ac); err != nil {
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
		ui.overrideHostHeader = overrideHostHeader(ui.HeadersConf.RequestHeaders)

		metricLabels, err := ui.getMetricLabels()
		if err != nil {
			return nil, fmt.Errorf("cannot parse metric_labels for unauthorized_user: %w", err)
		}
		ui.requests = ac.ms.NewCounter(`vmauth_unauthorized_user_requests_total` + metricLabels)
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
	if len(uis) == 0 && ac.UnauthorizedUser == nil {
		return nil, fmt.Errorf("Missing `users` or `unauthorized_user` sections")
	}
	byAuthToken := make(map[string]*UserInfo, len(uis))
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
		ui.overrideHostHeader = overrideHostHeader(ui.HeadersConf.RequestHeaders)

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
	dropSrcPathPrefixParts := 0
	discoverBackendIPs := *discoverBackendIPsGlobal
	if ui.URLPrefix != nil {
		if err := ui.URLPrefix.sanitizeAndInitialize(); err != nil {
			return err
		}
		if ui.RetryStatusCodes != nil {
			retryStatusCodes = ui.RetryStatusCodes
		}
		if ui.LoadBalancingPolicy != "" {
			loadBalancingPolicy = ui.LoadBalancingPolicy
		}
		if ui.DropSrcPathPrefixParts != nil {
			dropSrcPathPrefixParts = *ui.DropSrcPathPrefixParts
		}
		if ui.DiscoverBackendIPs != nil {
			discoverBackendIPs = *ui.DiscoverBackendIPs
		}
		ui.URLPrefix.retryStatusCodes = retryStatusCodes
		ui.URLPrefix.dropSrcPathPrefixParts = dropSrcPathPrefixParts
		ui.URLPrefix.discoverBackendIPs = discoverBackendIPs
		if err := ui.URLPrefix.setLoadBalancingPolicy(loadBalancingPolicy); err != nil {
			return err
		}
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
		dsp := dropSrcPathPrefixParts
		dbd := discoverBackendIPs
		if e.RetryStatusCodes != nil {
			rscs = e.RetryStatusCodes
		}
		if e.LoadBalancingPolicy != "" {
			lbp = e.LoadBalancingPolicy
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
	bus := make([]*backendURL, len(up.busOriginal))
	for i, bu := range up.busOriginal {
		bus[i] = &backendURL{
			url: bu,
		}
	}
	up.bus.Store(&bus)
	up.nextDiscoveryDeadline.Store(0)
	up.n.Store(0)

	return nil
}

func sanitizeURLPrefix(urlPrefix *url.URL) (*url.URL, error) {
	// Remove trailing '/' from urlPrefix
	for strings.HasSuffix(urlPrefix.Path, "/") {
		urlPrefix.Path = urlPrefix.Path[:len(urlPrefix.Path)-1]
	}
	// Validate urlPrefix
	if urlPrefix.Scheme != "http" && urlPrefix.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme for `url_prefix: %q`: %q; must be `http` or `https`", urlPrefix, urlPrefix.Scheme)
	}
	if urlPrefix.Host == "" {
		return nil, fmt.Errorf("missing hostname in `url_prefix %q`", urlPrefix.Host)
	}
	return urlPrefix, nil
}
