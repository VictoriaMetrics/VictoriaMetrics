package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	authConfigPath = flag.String("auth.config", "", "Path to auth config. It can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/vmauth.html for details on the format of this auth config")
	configCheckInterval = flag.Duration("configCheckInterval", 0, "interval for config file re-read. "+
		"Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.")
	defaultRetryStatusCodes = flagutil.NewArrayInt("retryStatusCodes", 0, "Comma-separated list of default HTTP response status codes when vmauth re-tries the request on other backends. "+
		"See https://docs.victoriametrics.com/vmauth.html#load-balancing for details")
	defaultLoadBalancingPolicy = flag.String("loadBalancingPolicy", "least_loaded", "The default load balancing policy to use for backend urls specified inside url_prefix section. "+
		"Supported policies: least_loaded, first_available. See https://docs.victoriametrics.com/vmauth.html#load-balancing for more details")
)

// AuthConfig represents auth config.
type AuthConfig struct {
	Users            []UserInfo `yaml:"users,omitempty"`
	UnauthorizedUser *UserInfo  `yaml:"unauthorized_user,omitempty"`
}

// UserInfo is user information read from authConfigPath
type UserInfo struct {
	Name                   string      `yaml:"name,omitempty"`
	BearerToken            string      `yaml:"bearer_token,omitempty"`
	Username               string      `yaml:"username,omitempty"`
	Password               string      `yaml:"password,omitempty"`
	URLPrefix              *URLPrefix  `yaml:"url_prefix,omitempty"`
	URLMaps                []URLMap    `yaml:"url_map,omitempty"`
	HeadersConf            HeadersConf `yaml:",inline"`
	MaxConcurrentRequests  int         `yaml:"max_concurrent_requests,omitempty"`
	DefaultURL             *URLPrefix  `yaml:"default_url,omitempty"`
	RetryStatusCodes       []int       `yaml:"retry_status_codes,omitempty"`
	LoadBalancingPolicy    string      `yaml:"load_balancing_policy,omitempty"`
	DropSrcPathPrefixParts *int        `yaml:"drop_src_path_prefix_parts,omitempty"`
	TLSInsecureSkipVerify  *bool       `yaml:"tls_insecure_skip_verify,omitempty"`
	TLSCAFile              string      `yaml:"tls_ca_file,omitempty"`

	concurrencyLimitCh      chan struct{}
	concurrencyLimitReached *metrics.Counter

	httpTransport *http.Transport

	requests         *metrics.Counter
	requestsDuration *metrics.Summary
}

// HeadersConf represents config for request and response headers.
type HeadersConf struct {
	RequestHeaders  []Header `yaml:"headers,omitempty"`
	ResponseHeaders []Header `yaml:"response_headers,omitempty"`
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
}

// UnmarshalYAML unmarshals h from f.
func (h *Header) UnmarshalYAML(f func(interface{}) error) error {
	var s string
	if err := f(&s); err != nil {
		return err
	}
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
	s := fmt.Sprintf("%s: %s", h.Name, h.Value)
	return s, nil
}

// URLMap is a mapping from source paths to target urls.
type URLMap struct {
	// SrcHosts is the list of regular expressions, which match the request hostname.
	SrcHosts []*Regex `yaml:"src_hosts,omitempty"`

	// SrcPaths is the list of regular expressions, which match the request path.
	SrcPaths []*Regex `yaml:"src_paths,omitempty"`

	// UrlPrefix contains backend url prefixes for the proxied request url.
	URLPrefix *URLPrefix `yaml:"url_prefix,omitempty"`

	// HeadersConf is the config for augumenting request and response headers.
	HeadersConf HeadersConf `yaml:",inline"`

	// RetryStatusCodes is the list of response status codes used for retrying requests.
	RetryStatusCodes []int `yaml:"retry_status_codes,omitempty"`

	// LoadBalancingPolicy is load balancing policy among UrlPrefix backends.
	LoadBalancingPolicy string `yaml:"load_balancing_policy,omitempty"`

	// DropSrcPathPrefixParts is the number of `/`-delimited request path prefix parts to drop before proxying the request to backend.
	DropSrcPathPrefixParts *int `yaml:"drop_src_path_prefix_parts,omitempty"`
}

// Regex represents a regex
type Regex struct {
	sOriginal string
	re        *regexp.Regexp
}

// URLPrefix represents passed `url_prefix`
type URLPrefix struct {
	n uint32

	// the list of backend urls
	bus []*backendURL

	// requests are re-tried on other backend urls for these http response status codes
	retryStatusCodes []int

	// load balancing policy used
	loadBalancingPolicy string

	// how many request path prefix parts to drop before routing the request to backendURL.
	dropSrcPathPrefixParts int
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
	brokenDeadline     uint64
	concurrentRequests int32
	url                *url.URL
}

func (bu *backendURL) isBroken() bool {
	ct := fasttime.UnixTimestamp()
	return ct < atomic.LoadUint64(&bu.brokenDeadline)
}

func (bu *backendURL) setBroken() {
	deadline := fasttime.UnixTimestamp() + uint64((*failTimeout).Seconds())
	atomic.StoreUint64(&bu.brokenDeadline, deadline)
}

func (bu *backendURL) get() {
	atomic.AddInt32(&bu.concurrentRequests, 1)
}

func (bu *backendURL) put() {
	atomic.AddInt32(&bu.concurrentRequests, -1)
}

func (up *URLPrefix) getBackendsCount() int {
	return len(up.bus)
}

// getBackendURL returns the backendURL depending on the load balance policy.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (up *URLPrefix) getBackendURL() *backendURL {
	if up.loadBalancingPolicy == "first_available" {
		return up.getFirstAvailableBackendURL()
	}
	return up.getLeastLoadedBackendURL()
}

// getFirstAvailableBackendURL returns the first available backendURL, which isn't broken.
//
// backendURL.put() must be called on the returned backendURL after the request is complete.
func (up *URLPrefix) getFirstAvailableBackendURL() *backendURL {
	bus := up.bus

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
func (up *URLPrefix) getLeastLoadedBackendURL() *backendURL {
	bus := up.bus
	if len(bus) == 1 {
		// Fast path - return the only backend url.
		bu := bus[0]
		bu.get()
		return bu
	}

	// Slow path - select other backend urls.
	n := atomic.AddUint32(&up.n, 1)

	for i := uint32(0); i < uint32(len(bus)); i++ {
		idx := (n + i) % uint32(len(bus))
		bu := bus[idx]
		if bu.isBroken() {
			continue
		}
		if atomic.CompareAndSwapInt32(&bu.concurrentRequests, 0, 1) {
			// Fast path - return the backend with zero concurrently executed requests.
			return bu
		}
	}

	// Slow path - return the backend with the minimum number of concurrently executed requests.
	buMin := bus[n%uint32(len(bus))]
	minRequests := atomic.LoadInt32(&buMin.concurrentRequests)
	for _, bu := range bus {
		if bu.isBroken() {
			continue
		}
		if n := atomic.LoadInt32(&bu.concurrentRequests); n < minRequests {
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

	bus := make([]*backendURL, len(urls))
	for i, u := range urls {
		pu, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("cannot unmarshal %q into url: %w", u, err)
		}
		bus[i] = &backendURL{
			url: pu,
		}
	}
	up.bus = bus
	return nil
}

// MarshalYAML marshals up to yaml.
func (up *URLPrefix) MarshalYAML() (interface{}, error) {
	var b []byte
	if len(up.bus) == 1 {
		u := up.bus[0].url.String()
		b = strconv.AppendQuote(b, u)
		return string(b), nil
	}
	b = append(b, '[')
	for i, bu := range up.bus {
		u := bu.url.String()
		b = strconv.AppendQuote(b, u)
		if i+1 < len(up.bus) {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return string(b), nil
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
	sAnchored := "^(?:" + s + ")$"
	re, err := regexp.Compile(sAnchored)
	if err != nil {
		return fmt.Errorf("cannot build regexp from %q: %w", s, err)
	}
	r.sOriginal = s
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

var authConfig atomic.Pointer[AuthConfig]
var authUsers atomic.Pointer[map[string]*UserInfo]
var authConfigWG sync.WaitGroup
var stopCh chan struct{}

// loadAuthConfig loads and applies the config from *authConfigPath.
// It returns bool value to identify if new config was applied.
// The config can be not applied if there is a parsing error
// or if there are no changes to the current authConfig.
func loadAuthConfig() (bool, error) {
	data, err := fs.ReadFileOrHTTP(*authConfigPath)
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

	authConfig.Store(ac)
	authConfigData.Store(&data)
	authUsers.Store(&m)

	return true, nil
}

func parseAuthConfig(data []byte) (*AuthConfig, error) {
	data, err := envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment vars: %w", err)
	}
	var ac AuthConfig
	if err = yaml.UnmarshalStrict(data, &ac); err != nil {
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
		if ui.Name != "" {
			return nil, fmt.Errorf("field name can't be specified for unauthorized_user section")
		}
		if err := ui.initURLs(); err != nil {
			return nil, err
		}
		ui.requests = metrics.GetOrCreateCounter(`vmauth_unauthorized_user_requests_total`)
		ui.requestsDuration = metrics.GetOrCreateSummary(`vmauth_unauthorized_user_request_duration_seconds`)
		ui.concurrencyLimitCh = make(chan struct{}, ui.getMaxConcurrentRequests())
		ui.concurrencyLimitReached = metrics.GetOrCreateCounter(`vmauth_unauthorized_user_concurrent_requests_limit_reached_total`)
		_ = metrics.GetOrCreateGauge(`vmauth_unauthorized_user_concurrent_requests_capacity`, func() float64 {
			return float64(cap(ui.concurrencyLimitCh))
		})
		_ = metrics.GetOrCreateGauge(`vmauth_unauthorized_user_concurrent_requests_current`, func() float64 {
			return float64(len(ui.concurrencyLimitCh))
		})
		tr, err := getTransport(ui.TLSInsecureSkipVerify, ui.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize HTTP transport: %w", err)
		}
		ui.httpTransport = tr
	}
	return &ac, nil
}

func parseAuthConfigUsers(ac *AuthConfig) (map[string]*UserInfo, error) {
	uis := ac.Users
	if len(uis) == 0 && ac.UnauthorizedUser == nil {
		return nil, fmt.Errorf("Missing `users` or `unauthorized_user` sections")
	}
	byAuthToken := make(map[string]*UserInfo, len(uis))
	for i := range uis {
		ui := &uis[i]
		if ui.BearerToken == "" && ui.Username == "" {
			return nil, fmt.Errorf("either bearer_token or username must be set")
		}
		if ui.BearerToken != "" && ui.Username != "" {
			return nil, fmt.Errorf("bearer_token=%q and username=%q cannot be set simultaneously", ui.BearerToken, ui.Username)
		}
		at1, at2 := getAuthTokens(ui.BearerToken, ui.Username, ui.Password)
		if byAuthToken[at1] != nil {
			return nil, fmt.Errorf("duplicate auth token found for bearer_token=%q, username=%q: %q", ui.BearerToken, ui.Username, at1)
		}
		if byAuthToken[at2] != nil {
			return nil, fmt.Errorf("duplicate auth token found for bearer_token=%q, username=%q: %q", ui.BearerToken, ui.Username, at2)
		}

		if err := ui.initURLs(); err != nil {
			return nil, err
		}

		name := ui.name()
		if ui.BearerToken != "" {
			if ui.Password != "" {
				return nil, fmt.Errorf("password shouldn't be set for bearer_token %q", ui.BearerToken)
			}
			ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, name))
			ui.requestsDuration = metrics.GetOrCreateSummary(fmt.Sprintf(`vmauth_user_request_duration_seconds{username=%q}`, name))
		}
		if ui.Username != "" {
			ui.requests = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_requests_total{username=%q}`, name))
			ui.requestsDuration = metrics.GetOrCreateSummary(fmt.Sprintf(`vmauth_user_request_duration_seconds{username=%q}`, name))
		}
		mcr := ui.getMaxConcurrentRequests()
		ui.concurrencyLimitCh = make(chan struct{}, mcr)
		ui.concurrencyLimitReached = metrics.GetOrCreateCounter(fmt.Sprintf(`vmauth_user_concurrent_requests_limit_reached_total{username=%q}`, name))
		_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmauth_user_concurrent_requests_capacity{username=%q}`, name), func() float64 {
			return float64(cap(ui.concurrencyLimitCh))
		})
		_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vmauth_user_concurrent_requests_current{username=%q}`, name), func() float64 {
			return float64(len(ui.concurrencyLimitCh))
		})

		tr, err := getTransport(ui.TLSInsecureSkipVerify, ui.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize HTTP transport: %w", err)
		}
		ui.httpTransport = tr

		byAuthToken[at1] = ui
		byAuthToken[at2] = ui
	}
	return byAuthToken, nil
}

func (ui *UserInfo) initURLs() error {
	retryStatusCodes := defaultRetryStatusCodes.Values()
	loadBalancingPolicy := *defaultLoadBalancingPolicy
	dropSrcPathPrefixParts := 0
	if ui.URLPrefix != nil {
		if err := ui.URLPrefix.sanitize(); err != nil {
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
		ui.URLPrefix.retryStatusCodes = retryStatusCodes
		ui.URLPrefix.dropSrcPathPrefixParts = dropSrcPathPrefixParts
		if err := ui.URLPrefix.setLoadBalancingPolicy(loadBalancingPolicy); err != nil {
			return err
		}
	}
	if ui.DefaultURL != nil {
		if err := ui.DefaultURL.sanitize(); err != nil {
			return err
		}
	}
	for _, e := range ui.URLMaps {
		if len(e.SrcPaths) == 0 && len(e.SrcHosts) == 0 {
			return fmt.Errorf("missing `src_paths` and `src_hosts` in `url_map`")
		}
		if e.URLPrefix == nil {
			return fmt.Errorf("missing `url_prefix` in `url_map`")
		}
		if err := e.URLPrefix.sanitize(); err != nil {
			return err
		}
		rscs := retryStatusCodes
		lbp := loadBalancingPolicy
		dsp := dropSrcPathPrefixParts
		if e.RetryStatusCodes != nil {
			rscs = e.RetryStatusCodes
		}
		if e.LoadBalancingPolicy != "" {
			lbp = e.LoadBalancingPolicy
		}
		if e.DropSrcPathPrefixParts != nil {
			dsp = *e.DropSrcPathPrefixParts
		}
		e.URLPrefix.retryStatusCodes = rscs
		if err := e.URLPrefix.setLoadBalancingPolicy(lbp); err != nil {
			return err
		}
		e.URLPrefix.dropSrcPathPrefixParts = dsp
	}
	if len(ui.URLMaps) == 0 && ui.URLPrefix == nil {
		return fmt.Errorf("missing `url_prefix`")
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
		return "bearer_token"
	}
	return ""
}

func getAuthTokens(bearerToken, username, password string) (string, string) {
	if bearerToken != "" {
		// Accept the bearerToken as Basic Auth username with empty password
		at1 := getAuthToken(bearerToken, "", "")
		at2 := getAuthToken("", bearerToken, "")
		return at1, at2
	}
	at := getAuthToken("", username, password)
	return at, at
}

func getAuthToken(bearerToken, username, password string) string {
	if bearerToken != "" {
		return "Bearer " + bearerToken
	}
	token := username + ":" + password
	token64 := base64.StdEncoding.EncodeToString([]byte(token))
	return "Basic " + token64
}

func (up *URLPrefix) sanitize() error {
	for _, bu := range up.bus {
		puNew, err := sanitizeURLPrefix(bu.url)
		if err != nil {
			return err
		}
		bu.url = puNew
	}
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
