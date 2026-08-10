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
	"slices"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
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

	BearerToken string     `yaml:"bearer_token,omitempty"`
	JWT         *JWTConfig `yaml:"jwt,omitempty"`
	AuthToken   string     `yaml:"auth_token,omitempty"`
	Username    string     `yaml:"username,omitempty"`
	Password    string     `yaml:"password,omitempty"`

	URLPrefix              *URLPrefix      `yaml:"url_prefix,omitempty"`
	URLMaps                []URLMap        `yaml:"url_map,omitempty"`
	DumpRequestOnErrors    bool            `yaml:"dump_request_on_errors,omitempty"`
	HeadersConf            HeadersConf     `yaml:",inline"`
	BackendSettings        BackendSettings `yaml:",inline"`
	DefaultURL             *URLPrefix      `yaml:"default_url,omitempty"`
	RetryStatusCodes       []int           `yaml:"retry_status_codes,omitempty"`
	MergeQueryArgs         []string        `yaml:"merge_query_args,omitempty"`
	DropSrcPathPrefixParts *int            `yaml:"drop_src_path_prefix_parts,omitempty"`

	MetricLabels map[string]string `yaml:"metric_labels,omitempty"`

	AccessLog *AccessLog `yaml:"access_log,omitempty"`

	concurrencyLimitCh      chan struct{}
	concurrencyLimitReached *metrics.Counter

	rt http.RoundTripper

	requests         *metrics.Counter
	requestErrors    *metrics.Counter
	backendRequests  *metrics.Counter
	backendErrors    *metrics.Counter
	requestsDuration *metrics.Summary
}

// AccessLog represents configuration for access log settings.
type AccessLog struct {
	Filters *AccessLogFilters `yaml:"filters"`
}

// AccessLogFilters represents list of filters for access logs printing
type AccessLogFilters struct {
	// SkipStatusCodes is a list of HTTP status codes for which access logs will be skipped
	SkipStatusCodes []int `yaml:"skip_status_codes"`
}

func (ui *UserInfo) logRequest(r *http.Request, userName string, statusCode int, duration time.Duration) {
	if ui == nil || ui.AccessLog == nil {
		return
	}

	filters := ui.AccessLog.Filters
	if filters != nil && len(filters.SkipStatusCodes) > 0 {
		if slices.Contains(filters.SkipStatusCodes, statusCode) {
			return
		}
	}

	remoteAddr := httpserver.GetQuotedRemoteAddr(r)
	requestURI := httpserver.GetRequestURI(r)
	logger.Infof("access_log request_host=%q request_uri=%q status_code=%d remote_addr=%s user_agent=%q referer=%q duration_ms=%d username=%q",
		r.Host, requestURI, statusCode, remoteAddr, r.UserAgent(), r.Referer(), duration.Milliseconds(), userName)
}

// hasAnyURLs reports whether ui has at least one backend URL route configured.
// It is used only for unauthorized_user config section, since other users
// must always have either URLPrefix or URLMaps set.
func (ui *UserInfo) hasAnyURLs() bool {
	if ui == nil {
		return false
	}

	return ui.URLPrefix != nil || len(ui.URLMaps) > 0 || ui.DefaultURL != nil
}

// HeadersConf represents config for request and response headers.
type HeadersConf struct {
	RequestHeaders     []*Header `yaml:"headers,omitempty"`
	ResponseHeaders    []*Header `yaml:"response_headers,omitempty"`
	KeepOriginalHost   *bool     `yaml:"keep_original_host,omitempty"`
	hasAnyPlaceHolders bool
}

// BackendSettings holds settings shared between UserInfo and BackendGroupConfig for controlling
// how vmauth selects and connects to backends.
type BackendSettings struct {
	LoadBalancingPolicy   string `yaml:"load_balancing_policy,omitempty"`
	DiscoverBackendIPs    *bool  `yaml:"discover_backend_ips,omitempty"`
	MaxConcurrentRequests int    `yaml:"max_concurrent_requests,omitempty"`
	TLSCAFile             string `yaml:"tls_ca_file,omitempty"`
	TLSCertFile           string `yaml:"tls_cert_file,omitempty"`
	TLSKeyFile            string `yaml:"tls_key_file,omitempty"`
	TLSServerName         string `yaml:"tls_server_name,omitempty"`
	TLSInsecureSkipVerify *bool  `yaml:"tls_insecure_skip_verify,omitempty"`
}

func (ui *UserInfo) beginConcurrencyLimit(ctx context.Context) error {
	select {
	case ui.concurrencyLimitCh <- struct{}{}:
		return nil
	default:
		// The number of concurrently executed requests for the given user equals the limit.
		// Wait until some of the currently executed requests are finished, so the current request could be executed.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10078
		select {
		case ui.concurrencyLimitCh <- struct{}{}:
			return nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				// The current request couldn't be executed until the request timeout.
				ui.concurrencyLimitReached.Inc()
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
	mcr := ui.BackendSettings.MaxConcurrentRequests
	if mcr <= 0 {
		mcr = *maxConcurrentPerUserRequests
	}
	return mcr
}

func (ui *UserInfo) stopHealthChecks() {
	if ui == nil {
		return
	}

	if ui.URLPrefix != nil {
		bus := ui.URLPrefix.bus.Load()
		bus.stopHealthChecks()
	}
	if ui.DefaultURL != nil {
		bus := ui.DefaultURL.bus.Load()
		bus.stopHealthChecks()
	}
	for i := range ui.URLMaps {
		um := &ui.URLMaps[i]
		if um.URLPrefix != nil {
			bus := um.URLPrefix.bus.Load()
			bus.stopHealthChecks()
		}
	}
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

	// busOriginal contains the original list of backend groups specified in yaml config.
	busOriginal []*backendGroupSpec

	// n is an atomic counter, which is used for balancing load among available backends.
	n atomic.Uint32

	// backendGroupCounters holds one atomic counter per busOriginal entry, used for balancing load.
	backendGroupCounters []atomic.Uint32

	// the list of backend urls
	//
	// the list can be dynamically updated if `discover_backend_ips` option is set.
	bus atomic.Pointer[backendURLs]

	// if this option is set by default, then backend ips for busOriginal are periodically re-discovered and put to bus.
	//
	// individual busOriginal entries may override this via their own discover_backend_ips setting.
	discoverBackendIPs bool

	// hasAnyBackendDiscovery is true if discovery is effectively enabled for at least one busOriginal entry.
	// It is computed once in sanitizeAndInitialize, so discoverBackendAddrsIfNeeded can cheaply skip
	// the whole discovery machinery on the common no-discovery path.
	hasAnyBackendDiscovery bool

	// The next deadline for DNS-based discovery of backend IPs
	nextDiscoveryDeadline atomic.Uint64

	// vOriginal contains the original yaml value for URLPrefix.
	vOriginal any
}

// backendGroupSpec represents a single parsed `url_prefix` list item.
//
// It is built either from a plain url string (no overrides, inherits everything from the
// enclosing scope) or from a BackendGroupConfig mapping.
type backendGroupSpec struct {
	// name identifies this group in metrics. Falls back to its ordinal position in url_prefix when empty.
	name string

	urls []*url.URL

	// per-group overrides; zero values mean "inherit from the enclosing url_prefix / user / url_map scope".
	loadBalancingPolicy   string
	discoverBackendIPs    *bool
	maxConcurrentRequests int
	tlsCAFile             string
	tlsCertFile           string
	tlsKeyFile            string
	tlsServerName         string
	tlsInsecureSkipVerify *bool
}

func (spec *backendGroupSpec) hasTLSOverride() bool {
	return spec.tlsCAFile != "" || spec.tlsCertFile != "" || spec.tlsKeyFile != "" ||
		spec.tlsServerName != "" || spec.tlsInsecureSkipVerify != nil
}

// BackendGroupConfig represents a `url_prefix` list item specified as a YAML mapping instead of a plain string.
//
// It lets a group of backend urls override load_balancing_policy, discover_backend_ips,
// max_concurrent_requests and tls_* settings, which are otherwise inherited from the enclosing
// `user` / `url_map` scope. See https://docs.victoriametrics.com/victoriametrics/vmauth/#load-balancing
type BackendGroupConfig struct {
	// Name optionally identifies this backend group in metrics. Falls back to its ordinal
	// position in url_prefix when not set.
	Name            string          `yaml:"name,omitempty"`
	URLPrefix       stringOrSlice   `yaml:"url_prefix"`
	BackendSettings BackendSettings `yaml:",inline"`
}

// stringOrSlice unmarshals a YAML value that is either a single string or a list of strings.
type stringOrSlice []string

func (s *stringOrSlice) UnmarshalYAML(f func(any) error) error {
	var v any
	if err := f(&v); err != nil {
		return err
	}
	urls, err := parseStringOrSliceValue(v)
	if err != nil {
		return fmt.Errorf("cannot unmarshal `url_prefix`: %w", err)
	}
	*s = urls
	return nil
}

func parseStringOrSliceValue(v any) ([]string, error) {
	switch x := v.(type) {
	case string:
		return []string{x}, nil
	case []any:
		if len(x) == 0 {
			return nil, fmt.Errorf("must contain at least a single url")
		}
		us := make([]string, len(x))
		for i, xx := range x {
			s, ok := xx.(string)
			if !ok {
				return nil, fmt.Errorf("must contain array of strings; got %T", xx)
			}
			us[i] = s
		}
		return us, nil
	default:
		return nil, fmt.Errorf("unexpected type: %T; want string or []string", v)
	}
}

func validateLoadBalancingPolicyValue(policy string) error {
	switch policy {
	case "", // empty string is equivalent to least_loaded
		"least_loaded",
		"first_available":
		return nil
	default:
		return fmt.Errorf("unexpected load_balancing_policy: %q; want least_loaded or first_available", policy)
	}
}

func (up *URLPrefix) setLoadBalancingPolicy(loadBalancingPolicy string) error {
	if err := validateLoadBalancingPolicyValue(loadBalancingPolicy); err != nil {
		return err
	}
	up.loadBalancingPolicy = loadBalancingPolicy
	return nil
}

// backendUserSettings holds the resolved user-level settings needed for building backendURLGroups.
//
// It lets URLPrefix.sanitizeAndInitialize apply per-group overrides (load_balancing_policy, tls_*,
// max_concurrent_requests) on top of the settings inherited from the enclosing user.
type backendUserSettings struct {
	tlsCAFile             string
	tlsCertFile           string
	tlsKeyFile            string
	tlsServerName         string
	tlsInsecureSkipVerify *bool

	ms           *metrics.Set
	metricLabels string
}

func backendGroupMetricLabels(userLabels, groupID string) string {
	label := fmt.Sprintf(`backend_group=%q`, groupID)
	if userLabels == "" {
		return "{" + label + "}"
	}
	return userLabels[:len(userLabels)-1] + "," + label + "}"
}

type backendURLs struct {
	bhc    backendHealthCheck
	n      *atomic.Uint32
	groups []*backendURLGroup
}

// backendURLGroup holds the backend urls discovered for a single busOriginal entry.
type backendURLGroup struct {
	// n is an atomic counter, which is used for balancing load among bus.
	//
	// It points into URLPrefix.backendGroupCounters, so it survives across the
	// group being recreated by backend IP rediscovery.
	n *atomic.Uint32

	bus []*backendURL

	loadBalancingPolicy string

	// rt is a per-group HTTP RoundTripper. nil means inherit the enclosing user's RoundTripper.
	rt http.RoundTripper

	// concurrencyLimitCh is a per-group concurrency limiter. nil means no group-level limit is configured.
	concurrencyLimitCh      chan struct{}
	concurrencyLimitReached *metrics.Counter
}

func (g *backendURLGroup) isBroken() bool {
	for _, bu := range g.bus {
		if !bu.isBroken() {
			return false
		}
	}
	return true
}

func (g *backendURLGroup) isAtConcurrencyLimit() bool {
	return g.concurrencyLimitCh != nil && len(g.concurrencyLimitCh) >= cap(g.concurrencyLimitCh)
}

func (g *backendURLGroup) isUnavailable() bool {
	return g.isBroken() || g.isAtConcurrencyLimit()
}

func (g *backendURLGroup) minConcurrentRequests() int32 {
	minReqs := int32(math.MaxInt32)
	for _, bu := range g.bus {
		if bu.isBroken() {
			continue
		}
		if n := bu.concurrentRequests.Load(); n < minReqs {
			minReqs = n
		}
	}
	return minReqs
}

func (g *backendURLGroup) getBackendURL() *backendURL {
	if g.loadBalancingPolicy == "first_available" {
		return g.getFirstAvailable()
	}
	return g.getLeastLoaded()
}

func (g *backendURLGroup) beginConcurrencyLimit() bool {
	if g.concurrencyLimitCh == nil {
		return true
	}
	select {
	case g.concurrencyLimitCh <- struct{}{}:
		return true
	default:
		if g.concurrencyLimitReached != nil {
			g.concurrencyLimitReached.Inc()
		}
		return false
	}
}

func (g *backendURLGroup) endConcurrencyLimit() {
	if g.concurrencyLimitCh == nil {
		return
	}
	<-g.concurrencyLimitCh
}

type backendHealthCheck struct {
	ctx context.Context
	// mu protects fields below
	cancel    func()
	mu        sync.Mutex
	isStopped bool
	wg        sync.WaitGroup
}

func (bhc *backendHealthCheck) run(hc func()) {
	bhc.mu.Lock()
	defer bhc.mu.Unlock()
	if bhc.isStopped {
		return
	}
	bhc.wg.Go(hc)
}

func (bhc *backendHealthCheck) stop() {
	bhc.mu.Lock()
	bhc.cancel()
	bhc.isStopped = true
	bhc.mu.Unlock()
	bhc.wg.Wait()
}

func newBackendURLs(n *atomic.Uint32) *backendURLs {
	ctx, cancel := context.WithCancel(context.Background())
	return &backendURLs{
		bhc: backendHealthCheck{
			ctx:    ctx,
			cancel: cancel,
		},
		n: n,
	}
}

// addGroup appends a new backendURLGroup to bus, containing a backendURL for every url in urls,
// and returns the newly created group so the caller can apply per-group settings to it.
func (bus *backendURLs) addGroup(urls []*url.URL, n *atomic.Uint32) *backendURLGroup {
	g := &backendURLGroup{
		n:   n,
		bus: make([]*backendURL, len(urls)),
	}
	for i, u := range urls {
		g.bus[i] = &backendURL{
			url:             u,
			bhc:             &bus.bhc,
			hasPlaceHolders: hasAnyPlaceholders(u),
			group:           g,
		}
	}
	bus.groups = append(bus.groups, g)
	return g
}

func (bus *backendURLs) stopHealthChecks() {
	bus.bhc.stop()
}

type backendURL struct {
	broken atomic.Bool

	bhc *backendHealthCheck

	concurrentRequests atomic.Int32

	url *url.URL

	hasPlaceHolders bool

	group *backendURLGroup
}

func (bu *backendURL) isBroken() bool {
	return bu.broken.Load()
}

func (bu *backendURL) setBroken() {
	if bu.broken.CompareAndSwap(false, true) {
		bu.bhc.run(func() {
			bu.runHealthCheck()
			bu.broken.Store(false)
		})
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
			ctx, cancel := context.WithTimeout(bu.bhc.ctx, time.Second)
			c, err := netutil.Dialer.DialContext(ctx, "tcp", addr)
			cancel()
			if err != nil {
				if errors.Is(bu.bhc.ctx.Err(), context.Canceled) {
					return
				}
				logger.Warnf("ignoring the backend at %s for %s because of dial error: %s", addr, *failTimeout, err)
				continue
			}

			_ = c.Close()
			return
		case <-bu.bhc.ctx.Done():
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
	n := 0
	for _, g := range bus.groups {
		n += len(g.bus)
	}
	return n
}

// getBackendURL returns the backendURL depending on the load balance policy.
//
// It can return nil if there are no backend urls available at the moment.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (up *URLPrefix) getBackendURL() *backendURL {
	up.discoverBackendAddrsIfNeeded()

	bus := up.bus.Load()
	if len(bus.groups) == 0 {
		return nil
	}

	var g *backendURLGroup
	if up.loadBalancingPolicy == "first_available" {
		g = bus.getFirstAvailable()
	} else {
		g = bus.getLeastLoaded()
	}
	if len(g.bus) == 0 {
		return nil
	}

	return g.getBackendURL()
}

// effectiveDiscoverBackendIPs returns whether backend IP discovery is enabled for spec,
// taking into account spec's own override of up.discoverBackendIPs.
func (up *URLPrefix) effectiveDiscoverBackendIPs(spec *backendGroupSpec) bool {
	if spec.discoverBackendIPs != nil {
		return *spec.discoverBackendIPs
	}
	return up.discoverBackendIPs
}

func (up *URLPrefix) discoverBackendAddrsIfNeeded() {
	if !up.hasAnyBackendDiscovery {
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

	// Discover ips for all the backendURLs which need it.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(intervalSec))
	hostToAddrs := make(map[string][]string)
	for _, spec := range up.busOriginal {
		if !up.effectiveDiscoverBackendIPs(spec) {
			continue
		}
		for _, bu := range spec.urls {
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
			// sort resolvedAddrs, so they could be compared below in areEqualBackendURLGroups()
			sort.Strings(resolvedAddrs)
			hostToAddrs[host] = resolvedAddrs
		}
	}
	cancel()

	// generate new backendURLs for the resolved IPs, one group per busOriginal entry
	oldGroups := up.bus.Load().groups
	busNew := newBackendURLs(&up.n)
	for i, spec := range up.busOriginal {
		var urls []*url.URL
		if up.effectiveDiscoverBackendIPs(spec) {
			for _, bu := range spec.urls {
				host := bu.Hostname()
				for _, addr := range hostToAddrs[host] {
					buCopy := *bu
					buCopy.Host = addr
					urls = append(urls, &buCopy)
				}
			}
		} else {
			urls = spec.urls
		}
		g := busNew.addGroup(urls, &up.backendGroupCounters[i])
		if i < len(oldGroups) {
			// Per-group settings are static for the lifetime of this URLPrefix - carry them over
			// instead of recomputing, so a per-group RoundTripper / concurrency limiter isn't
			// rebuilt (and in-flight concurrency accounting isn't lost) on every rediscovery.
			g.loadBalancingPolicy = oldGroups[i].loadBalancingPolicy
			g.rt = oldGroups[i].rt
			g.concurrencyLimitCh = oldGroups[i].concurrencyLimitCh
			g.concurrencyLimitReached = oldGroups[i].concurrencyLimitReached
		}
	}

	bus := up.bus.Load()
	if areEqualBackendURLGroups(bus.groups, busNew.groups) {
		return
	}

	// Store new backend urls
	up.bus.Store(busNew)
	bus.stopHealthChecks()
}

func areEqualBackendURLGroups(a, b []*backendURLGroup) bool {
	if len(a) != len(b) {
		return false
	}
	for i, g := range a {
		if !areEqualBackendURLs(g.bus, b[i].bus) {
			return false
		}
	}
	return true
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

// getFirstAvailable returns the first available backendURL in g, which isn't broken.
// If all backendURLs in g are broken, then returns the first one.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (g *backendURLGroup) getFirstAvailable() *backendURL {
	bus := g.bus
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

	// All backend urls are unavailable, then returning a first one, it could help increase the success rate of the requests。
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10837#issuecomment-4307050980.
	bu.get()
	return bu
}

func (g *backendURLGroup) getLeastLoaded() *backendURL {
	bus := g.bus
	atomicCounter := g.n

	firstBu := bus[0]
	if len(bus) == 1 {
		firstBu.get()
		return firstBu
	}

	// Slow path - select other backend urls.
	n := atomicCounter.Add(1) - 1
	for i := range uint32(len(bus)) {
		idx := (n + i) % uint32(len(bus))
		bu := bus[idx]
		if bu.isBroken() {
			continue
		}

		// The Load() in front of CompareAndSwap() avoids CAS overhead for items with values bigger than 0.
		if bu.concurrentRequests.Load() == 0 && bu.concurrentRequests.CompareAndSwap(0, 1) {
			atomicCounter.CompareAndSwap(n+1, idx+1)
			// There is no need in the call bu.get(), because we already incremented bu.concurrentRequests above.
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
		// If all backendURLs are broken, then returns the first backendURL.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10837#issuecomment-4307050980.
		firstBu.get()
		return firstBu
	}
	buMin.get()
	atomicCounter.CompareAndSwap(n+1, buMinIdx+1)
	return buMin
}

// getFirstAvailable returns the first backendURLGroup in bus, which has at least a single non-broken backend url.
// If all groups are fully broken, then returns the first one.
func (bus *backendURLs) getFirstAvailable() *backendURLGroup {
	groups := bus.groups
	g := groups[0]
	if !g.isUnavailable() {
		// Fast path - use the first group.
		return g
	}

	// Slow path - the first group is temporarily unavailable. Fall back to the remaining groups.
	for i := 1; i < len(groups); i++ {
		if !groups[i].isUnavailable() {
			return groups[i]
		}
	}

	// All groups are unavailable, then return the first one, it could help increase the success rate of the requests.
	return g
}

// getLeastLoaded returns a non-broken backendURLGroup in bus with the lowest number of concurrent requests
// among its non-broken backend urls. If all groups are broken, then returns the first one.
func (bus *backendURLs) getLeastLoaded() *backendURLGroup {
	groups := bus.groups
	atomicCounter := bus.n
	firstGroup := groups[0]
	if len(groups) == 1 {
		return firstGroup
	}

	n := atomicCounter.Add(1) - 1
	gMinIdx := n % uint32(len(groups))
	minRequests := groups[gMinIdx].minConcurrentRequests()
	for i := uint32(1); i < uint32(len(groups)); i++ {
		idx := (n + i) % uint32(len(groups))
		g := groups[idx]
		if g.isUnavailable() {
			continue
		}

		reqs := g.minConcurrentRequests()
		if reqs < minRequests || groups[gMinIdx].isUnavailable() {
			gMinIdx = idx
			minRequests = reqs
		}
	}
	gMin := groups[gMinIdx]
	if gMin.isUnavailable() {
		// If all groups are unavailable, then returns the first group.
		return firstGroup
	}
	atomicCounter.CompareAndSwap(n+1, gMinIdx+1)
	return gMin
}

// UnmarshalYAML unmarshals up from yaml.
func (up *URLPrefix) UnmarshalYAML(f func(any) error) error {
	var v any
	if err := f(&v); err != nil {
		return err
	}
	up.vOriginal = v

	var items []any
	switch x := v.(type) {
	case string:
		items = []any{x}
	case []any:
		if len(x) == 0 {
			return fmt.Errorf("`url_prefix` must contain at least a single url")
		}
		items = x
	default:
		return fmt.Errorf("unexpected type for `url_prefix`: %T; want string, []string or a list containing backend group mappings", v)
	}

	specs := make([]*backendGroupSpec, len(items))
	for i, item := range items {
		spec, err := parseBackendGroupSpecItem(item)
		if err != nil {
			return fmt.Errorf("cannot unmarshal `url_prefix` item #%d: %w", i+1, err)
		}
		specs[i] = spec
	}
	up.busOriginal = specs
	return nil
}

func parseBackendGroupSpecItem(item any) (*backendGroupSpec, error) {
	switch x := item.(type) {
	case string:
		pu, err := url.Parse(x)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal %q into url: %w", x, err)
		}
		return &backendGroupSpec{urls: []*url.URL{pu}}, nil
	case map[interface{}]interface{}:
		data, err := yaml.Marshal(x)
		if err != nil {
			return nil, fmt.Errorf("cannot re-marshal backend group mapping: %w", err)
		}
		var bgc BackendGroupConfig
		if err := yaml.UnmarshalStrict(data, &bgc); err != nil {
			return nil, fmt.Errorf("cannot unmarshal backend group mapping: %w", err)
		}
		if len(bgc.URLPrefix) == 0 {
			return nil, fmt.Errorf("missing `url_prefix` in backend group mapping")
		}
		if err := validateLoadBalancingPolicyValue(bgc.BackendSettings.LoadBalancingPolicy); err != nil {
			return nil, err
		}
		urls := make([]*url.URL, len(bgc.URLPrefix))
		for i, u := range bgc.URLPrefix {
			pu, err := url.Parse(u)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal %q into url: %w", u, err)
			}
			urls[i] = pu
		}
		return &backendGroupSpec{
			name:                  bgc.Name,
			urls:                  urls,
			loadBalancingPolicy:   bgc.BackendSettings.LoadBalancingPolicy,
			discoverBackendIPs:    bgc.BackendSettings.DiscoverBackendIPs,
			maxConcurrentRequests: bgc.BackendSettings.MaxConcurrentRequests,
			tlsCAFile:             bgc.BackendSettings.TLSCAFile,
			tlsCertFile:           bgc.BackendSettings.TLSCertFile,
			tlsKeyFile:            bgc.BackendSettings.TLSKeyFile,
			tlsServerName:         bgc.BackendSettings.TLSServerName,
			tlsInsecureSkipVerify: bgc.BackendSettings.TLSInsecureSkipVerify,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected type for `url_prefix` item: %T; want a string or a mapping with url_prefix/load_balancing_policy/discover_backend_ips/etc", item)
	}
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
	authConfigWG.Go(func() {
		authConfigReloader(sighupCh)
	})
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
		default:
		}
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

	// jwt authentication cache
	jwtAuthCache atomic.Pointer[jwtCache]

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
	jwtc := jwtAuthCache.Load()
	logger.Infof("loaded information about %d users from -auth.config=%q", len(*mp)+len(jwtc.users), *authConfigPath)
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

	oidcDP := &oidcDiscovererPool{}
	jui, err := parseJWTUsers(ac, oidcDP)
	if err != nil {
		return false, fmt.Errorf("failed to parse JWT users from auth config: %w", err)
	}
	oidcDP.startDiscovery()
	jwtc := &jwtCache{
		users:  jui,
		oidcDP: oidcDP,
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

	jwtcPrev := jwtAuthCache.Load()
	if jwtcPrev != nil {
		jwtcPrev.oidcDP.stopDiscovery()
	}

	authConfig.Store(ac)
	authConfigData.Store(&data)
	authUsers.Store(&m)
	jwtAuthCache.Store(jwtc)

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
		if ui.JWT != nil {
			return nil, fmt.Errorf("field jwt can't be specified for unauthorized_user section")
		}
		if ui.AuthToken != "" {
			return nil, fmt.Errorf("field auth_token can't be specified for unauthorized_user section")
		}
		if ui.Name != "" {
			return nil, fmt.Errorf("field name can't be specified for unauthorized_user section")
		}
		if err := parseJWTPlaceholdersForUserInfo(ui, false); err != nil {
			return nil, err
		}

		if ui.hasAnyURLs() {
			if err := ui.initURLs(ac.ms); err != nil {
				return nil, err
			}
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

		rt, err := newRoundTripper(ui.BackendSettings)
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
		// users with jwt tokens are parsed by parseJWTUsers function.
		// the function also checks that users with jwt tokens do not have auth tokens, bearer tokens, usernames and passwords.
		if ui.JWT != nil {
			continue
		}

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

		if err := parseJWTPlaceholdersForUserInfo(ui, false); err != nil {
			return nil, err
		}
		if err := ui.initURLs(ac.ms); err != nil {
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

		rt, err := newRoundTripper(ui.BackendSettings)
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

func (ui *UserInfo) initURLs(ms *metrics.Set) error {
	retryStatusCodes := defaultRetryStatusCodes.Values()
	loadBalancingPolicy := *defaultLoadBalancingPolicy
	mergeQueryArgs := *defaultMergeQueryArgs
	dropSrcPathPrefixParts := 0
	discoverBackendIPs := *discoverBackendIPsGlobal
	if ui.RetryStatusCodes != nil {
		retryStatusCodes = ui.RetryStatusCodes
	}
	if ui.BackendSettings.LoadBalancingPolicy != "" {
		loadBalancingPolicy = ui.BackendSettings.LoadBalancingPolicy
	}
	if len(ui.MergeQueryArgs) != 0 {
		mergeQueryArgs = ui.MergeQueryArgs
	}
	if ui.DropSrcPathPrefixParts != nil {
		dropSrcPathPrefixParts = *ui.DropSrcPathPrefixParts
	}
	if ui.BackendSettings.DiscoverBackendIPs != nil {
		discoverBackendIPs = *ui.BackendSettings.DiscoverBackendIPs
	}

	metricLabels, err := ui.getMetricLabels()
	if err != nil {
		return err
	}
	userSettings := backendUserSettings{
		tlsCAFile:             ui.BackendSettings.TLSCAFile,
		tlsCertFile:           ui.BackendSettings.TLSCertFile,
		tlsKeyFile:            ui.BackendSettings.TLSKeyFile,
		tlsServerName:         ui.BackendSettings.TLSServerName,
		tlsInsecureSkipVerify: ui.BackendSettings.TLSInsecureSkipVerify,
		ms:                    ms,
		metricLabels:          metricLabels,
	}

	up := ui.URLPrefix
	if up != nil {
		up.retryStatusCodes = retryStatusCodes
		up.dropSrcPathPrefixParts = dropSrcPathPrefixParts
		up.discoverBackendIPs = discoverBackendIPs
		if err := up.setLoadBalancingPolicy(loadBalancingPolicy); err != nil {
			return err
		}
		up.mergeQueryArgs = mergeQueryArgs
		if err := up.sanitizeAndInitialize(userSettings); err != nil {
			return err
		}
	}
	if ui.DefaultURL != nil {
		if err := ui.DefaultURL.sanitizeAndInitialize(userSettings); err != nil {
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
		if err := e.URLPrefix.sanitizeAndInitialize(userSettings); err != nil {
			return err
		}
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
	if ui.JWT != nil {
		return `jwt`
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

// sanitizeAndInitialize validates up.busOriginal and (re)initializes up.bus from it,
// applying per-group overrides on top of the settings inherited from userSettings.
func (up *URLPrefix) sanitizeAndInitialize(userSettings backendUserSettings) error {
	for _, spec := range up.busOriginal {
		for i, bu := range spec.urls {
			puNew, err := sanitizeURLPrefix(bu)
			if err != nil {
				return err
			}
			spec.urls[i] = puNew
		}
	}

	up.backendGroupCounters = make([]atomic.Uint32, len(up.busOriginal))

	up.hasAnyBackendDiscovery = false
	for _, spec := range up.busOriginal {
		if up.effectiveDiscoverBackendIPs(spec) {
			up.hasAnyBackendDiscovery = true
			break
		}
	}

	// Initialize up.bus with a single group per busOriginal entry.
	bus := newBackendURLs(&up.n)
	for i, spec := range up.busOriginal {
		g := bus.addGroup(spec.urls, &up.backendGroupCounters[i])

		lbp := spec.loadBalancingPolicy
		if lbp == "" {
			lbp = up.loadBalancingPolicy
		}
		g.loadBalancingPolicy = lbp

		if spec.hasTLSOverride() {
			bs := BackendSettings{
				TLSCAFile:             spec.tlsCAFile,
				TLSCertFile:           spec.tlsCertFile,
				TLSKeyFile:            spec.tlsKeyFile,
				TLSServerName:         spec.tlsServerName,
				TLSInsecureSkipVerify: spec.tlsInsecureSkipVerify,
			}
			if bs.TLSCAFile == "" {
				bs.TLSCAFile = userSettings.tlsCAFile
			}
			if bs.TLSCertFile == "" {
				bs.TLSCertFile = userSettings.tlsCertFile
			}
			if bs.TLSKeyFile == "" {
				bs.TLSKeyFile = userSettings.tlsKeyFile
			}
			if bs.TLSServerName == "" {
				bs.TLSServerName = userSettings.tlsServerName
			}
			if bs.TLSInsecureSkipVerify == nil {
				bs.TLSInsecureSkipVerify = userSettings.tlsInsecureSkipVerify
			}

			rt, err := newRoundTripper(bs)
			if err != nil {
				return fmt.Errorf("cannot initialize HTTP RoundTripper for a backend group at `url_prefix` item #%d: %w", i+1, err)
			}
			g.rt = rt
		}

		if spec.maxConcurrentRequests > 0 {
			g.concurrencyLimitCh = make(chan struct{}, spec.maxConcurrentRequests)
			if userSettings.ms != nil {
				groupID := spec.name
				if groupID == "" {
					groupID = strconv.Itoa(i)
				}
				groupLabels := backendGroupMetricLabels(userSettings.metricLabels, groupID)
				g.concurrencyLimitReached = userSettings.ms.GetOrCreateCounter(`vmauth_backend_group_concurrent_requests_limit_reached_total` + groupLabels)
				ch := g.concurrencyLimitCh
				_ = userSettings.ms.GetOrCreateGauge(`vmauth_backend_group_concurrent_requests_capacity`+groupLabels, func() float64 {
					return float64(cap(ch))
				})
				_ = userSettings.ms.GetOrCreateGauge(`vmauth_backend_group_concurrent_requests_current`+groupLabels, func() float64 {
					return float64(len(ch))
				})
			}
		}
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
