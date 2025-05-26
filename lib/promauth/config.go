package promauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

// Secret represents a string secret such as password or auth token.
//
// It is marshaled to "<secret>" string in yaml.
//
// This is needed for hiding secret strings in /config page output.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1764
type Secret struct {
	S string
}

// NewSecret returns new secret for s.
func NewSecret(s string) *Secret {
	if s == "" {
		return nil
	}
	return &Secret{
		S: s,
	}
}

// MarshalYAML implements yaml.Marshaler interface.
//
// It substitutes the secret with "<secret>" string.
func (s *Secret) MarshalYAML() (any, error) {
	return "<secret>", nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (s *Secret) UnmarshalYAML(f func(any) error) error {
	var secret string
	if err := f(&secret); err != nil {
		return fmt.Errorf("cannot parse secret: %w", err)
	}
	s.S = secret
	return nil
}

// String returns the secret in plaintext.
func (s *Secret) String() string {
	if s == nil {
		return ""
	}
	return s.S
}

// TLSConfig represents TLS config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
type TLSConfig struct {
	CA                 string `yaml:"ca,omitempty"`
	CAFile             string `yaml:"ca_file,omitempty"`
	Cert               string `yaml:"cert,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	Key                string `yaml:"key,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	MinVersion         string `yaml:"min_version,omitempty"`
	// Do not define MaxVersion field (max_version), since this has no sense from security PoV.
	// This can only result in lower security level if improperly set.
}

// Authorization represents generic authorization config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Authorization struct {
	Type            string  `yaml:"type,omitempty"`
	Credentials     *Secret `yaml:"credentials,omitempty"`
	CredentialsFile string  `yaml:"credentials_file,omitempty"`
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string  `yaml:"username,omitempty"`
	UsernameFile string  `yaml:"username_file,omitempty"`
	Password     *Secret `yaml:"password,omitempty"`
	PasswordFile string  `yaml:"password_file,omitempty"`
}

// HTTPClientConfig represents http client config.
type HTTPClientConfig struct {
	Authorization   *Authorization   `yaml:"authorization,omitempty"`
	BasicAuth       *BasicAuthConfig `yaml:"basic_auth,omitempty"`
	BearerToken     *Secret          `yaml:"bearer_token,omitempty"`
	BearerTokenFile string           `yaml:"bearer_token_file,omitempty"`
	OAuth2          *OAuth2Config    `yaml:"oauth2,omitempty"`
	TLSConfig       *TLSConfig       `yaml:"tls_config,omitempty"`

	// Headers contains optional HTTP headers, which must be sent in the request to the server
	Headers []string `yaml:"headers,omitempty"`

	// FollowRedirects specifies whether the client should follow HTTP 3xx redirects.
	FollowRedirects *bool `yaml:"follow_redirects,omitempty"`

	// Do not support enable_http2 option because of the following reasons:
	//
	// - http2 is used very rarely comparing to http for Prometheus metrics exposition and service discovery
	// - http2 is much harder to debug than http
	// - http2 has very bad security record because of its complexity - see https://portswigger.net/research/http2
	//
	// EnableHTTP2 bool
}

// ProxyClientConfig represents proxy client config.
type ProxyClientConfig struct {
	Authorization   *Authorization   `yaml:"proxy_authorization,omitempty"`
	BasicAuth       *BasicAuthConfig `yaml:"proxy_basic_auth,omitempty"`
	BearerToken     *Secret          `yaml:"proxy_bearer_token,omitempty"`
	BearerTokenFile string           `yaml:"proxy_bearer_token_file,omitempty"`
	OAuth2          *OAuth2Config    `yaml:"proxy_oauth2,omitempty"`
	TLSConfig       *TLSConfig       `yaml:"proxy_tls_config,omitempty"`

	// Headers contains optional HTTP headers, which must be sent in the request to the proxy
	Headers []string `yaml:"proxy_headers,omitempty"`
}

// OAuth2Config represent OAuth2 configuration
type OAuth2Config struct {
	ClientID         string            `yaml:"client_id"`
	ClientSecret     *Secret           `yaml:"client_secret,omitempty"`
	ClientSecretFile string            `yaml:"client_secret_file,omitempty"`
	Scopes           []string          `yaml:"scopes,omitempty"`
	TokenURL         string            `yaml:"token_url"`
	EndpointParams   map[string]string `yaml:"endpoint_params,omitempty"`
	TLSConfig        *TLSConfig        `yaml:"tls_config,omitempty"`
	ProxyURL         string            `yaml:"proxy_url,omitempty"`
}

func (o *OAuth2Config) validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}
	if o.ClientSecret == nil && o.ClientSecretFile == "" {
		return fmt.Errorf("ClientSecret or ClientSecretFile must be set")
	}
	if o.ClientSecret != nil && o.ClientSecretFile != "" {
		return fmt.Errorf("ClientSecret and ClientSecretFile cannot be set simultaneously")
	}
	if o.TokenURL == "" {
		return fmt.Errorf("token_url cannot be empty")
	}
	return nil
}

type oauth2ConfigInternal struct {
	mu               sync.Mutex
	cfg              *clientcredentials.Config
	clientSecretFile string

	// ac contains auth config needed for initializing tls config
	ac *Config

	proxyURL     string
	proxyURLFunc func(*http.Request) (*url.URL, error)

	ctx         context.Context
	tokenSource oauth2.TokenSource
}

func (oi *oauth2ConfigInternal) String() string {
	return fmt.Sprintf("clientID=%q, clientSecret=%q, clientSecretFile=%q, scopes=%q, endpointParams=%q, tokenURL=%q, proxyURL=%q, tlsConfig={%s}",
		oi.cfg.ClientID, oi.cfg.ClientSecret, oi.clientSecretFile, oi.cfg.Scopes, oi.cfg.EndpointParams, oi.cfg.TokenURL, oi.proxyURL, oi.ac.String())
}

func newOAuth2ConfigInternal(baseDir string, o *OAuth2Config) (*oauth2ConfigInternal, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	oi := &oauth2ConfigInternal{
		cfg: &clientcredentials.Config{
			ClientID:       o.ClientID,
			ClientSecret:   o.ClientSecret.String(),
			TokenURL:       o.TokenURL,
			Scopes:         o.Scopes,
			EndpointParams: urlValuesFromMap(o.EndpointParams),
		},
	}
	if o.ClientSecretFile != "" {
		oi.clientSecretFile = fscore.GetFilepath(baseDir, o.ClientSecretFile)
		// There is no need in reading oi.clientSecretFile now, since it may be missing right now.
		// It is read later before performing oauth2 request to server.
	}
	opts := &Options{
		BaseDir:   baseDir,
		TLSConfig: o.TLSConfig,
	}
	ac, err := opts.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot parse TLS config for OAuth2: %w", err)
	}
	oi.ac = ac
	if o.ProxyURL != "" {
		u, err := url.Parse(o.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("cannot parse proxy_url=%q: %w", o.ProxyURL, err)
		}
		oi.proxyURL = o.ProxyURL
		oi.proxyURLFunc = http.ProxyURL(u)
	}
	return oi, nil
}

func urlValuesFromMap(m map[string]string) url.Values {
	result := make(url.Values, len(m))
	for k, v := range m {
		result[k] = []string{v}
	}
	return result
}

func (oi *oauth2ConfigInternal) initTokenSource() error {
	tr := httputil.NewTransport(false, "vm_oauth_client")
	tr.Proxy = oi.proxyURLFunc
	c := &http.Client{
		Transport: oi.ac.NewRoundTripper(tr),
	}
	oi.ctx = context.WithValue(context.Background(), oauth2.HTTPClient, c)
	oi.tokenSource = oi.cfg.TokenSource(oi.ctx)
	return nil
}

func (oi *oauth2ConfigInternal) getTokenSource() (oauth2.TokenSource, error) {
	oi.mu.Lock()
	defer oi.mu.Unlock()

	if oi.tokenSource == nil {
		if err := oi.initTokenSource(); err != nil {
			return nil, err
		}
	}

	if oi.clientSecretFile == "" {
		return oi.tokenSource, nil
	}
	newSecret, err := fscore.ReadPasswordFromFileOrHTTP(oi.clientSecretFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read OAuth2 secret from %q: %w", oi.clientSecretFile, err)
	}
	if newSecret == oi.cfg.ClientSecret {
		return oi.tokenSource, nil
	}
	oi.cfg.ClientSecret = newSecret
	oi.tokenSource = oi.cfg.TokenSource(oi.ctx)
	return oi.tokenSource, nil
}

// Config is auth config.
type Config struct {
	tlsServerName         string
	tlsInsecureSkipVerify bool
	tlsMinVersion         uint16

	getTLSConfigCached getTLSConfigFunc

	getTLSRootCA    getTLSRootCAFunc
	tlsRootCADigest string

	getTLSCertCached getTLSCertFunc
	tlsCertDigest    string

	getAuthHeaderCached getAuthHeaderFunc
	authHeaderDigest    string

	headers       []keyValue
	headersDigest string
}

type keyValue struct {
	key   string
	value string
}

func parseHeaders(headers []string) ([]keyValue, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	kvs := make([]keyValue, len(headers))
	for i, h := range headers {
		n := strings.IndexByte(h, ':')
		if n < 0 {
			return nil, fmt.Errorf(`missing ':' in header %q; expecting "key: value" format`, h)
		}
		kv := &kvs[i]
		kv.key = http.CanonicalHeaderKey(strings.TrimSpace(h[:n]))
		kv.value = strings.TrimSpace(h[n+1:])
	}
	return kvs, nil
}

// HeadersNoAuthString returns string representation of ac headers
func (ac *Config) HeadersNoAuthString() string {
	if len(ac.headers) == 0 {
		return ""
	}
	a := make([]string, len(ac.headers))
	for i, h := range ac.headers {
		a[i] = h.key + ": " + h.value + "\r\n"
	}
	return strings.Join(a, "")
}

// SetHeaders sets the configured ac headers to req.
func (ac *Config) SetHeaders(req *http.Request, setAuthHeader bool) error {
	if ac.tlsServerName != "" {
		// It tlsServerName is set, then it is likely the request is performed via IP address instead of hostname.
		// In this case users expect that the specified tlsServerName is used as a Host header in the request to https server.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/5802
		req.Host = ac.tlsServerName
	}
	reqHeaders := req.Header
	for _, h := range ac.headers {
		if h.key == "Host" {
			// Host header must be set via req.Host - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5969
			req.Host = h.value
		} else {
			reqHeaders.Set(h.key, h.value)
		}
	}
	if setAuthHeader {
		ah, err := ac.GetAuthHeader()
		if err != nil {
			return fmt.Errorf("failed to obtain Authorization request header: %w", err)
		}
		if ah != "" {
			reqHeaders.Set("Authorization", ah)
		}
	}
	return nil
}

// GetAuthHeader returns optional `Authorization: ...` http header.
func (ac *Config) GetAuthHeader() (string, error) {
	if f := ac.getAuthHeaderCached; f != nil {
		return f()
	}
	return "", nil
}

// GetHTTPHeadersNoAuth returns http formatted headers without Authorization header
func (ac *Config) GetHTTPHeadersNoAuth() http.Header {
	if len(ac.headers) == 0 {
		return nil
	}
	dst := make(http.Header, len(ac.headers))
	for _, kv := range ac.headers {
		dst.Add(kv.key, kv.value)
	}
	return dst
}

// String returns human-readable representation for ac.
//
// It is also used for comparing Config objects for equality. If two Config
// objects have the same string representation, then they are considered equal.
func (ac *Config) String() string {
	return fmt.Sprintf("AuthHeader=%s, Headers=%s, TLSRootCA=%s, TLSCert=%s, TLSServerName=%s, TLSInsecureSkipVerify=%v, TLSMinVersion=%d",
		ac.authHeaderDigest, ac.headersDigest, ac.tlsRootCADigest, ac.tlsCertDigest, ac.tlsServerName, ac.tlsInsecureSkipVerify, ac.tlsMinVersion)
}

// getAuthHeaderFunc must return <value> for 'Authorization: <value>' http request header
type getAuthHeaderFunc func() (string, error)

func newGetAuthHeaderCached(getAuthHeader getAuthHeaderFunc) getAuthHeaderFunc {
	if getAuthHeader == nil {
		return nil
	}
	var mu sync.Mutex
	var deadline uint64
	var ah string
	var err error
	return func() (string, error) {
		// Cache the auth header and the error for up to a second in order to save CPU time
		// on reading and parsing auth headers from files.
		// This also reduces load on OAuth2 server when oauth2 config is enabled.
		mu.Lock()
		defer mu.Unlock()
		if fasttime.UnixTimestamp() > deadline {
			ah, err = getAuthHeader()
			deadline = fasttime.UnixTimestamp() + 1
		}
		return ah, err
	}
}

type getTLSRootCAFunc func() (*x509.CertPool, error)

type getTLSConfigFunc func() (*tls.Config, error)

func newGetTLSConfigCached(getTLSConfig getTLSConfigFunc) getTLSConfigFunc {
	var mu sync.Mutex
	var deadline uint64
	var tlsCfg *tls.Config
	var err error
	return func() (*tls.Config, error) {
		// Cache the tlsCfg and the error for up to a second in order to save CPU time on getTLSConfig() call.
		mu.Lock()
		defer mu.Unlock()
		if fasttime.UnixTimestamp() > deadline {
			tlsCfg, err = getTLSConfig()
			deadline = fasttime.UnixTimestamp() + 1
		}
		return tlsCfg, err
	}
}

type getTLSCertFunc func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error)

func newGetTLSCertCached(getTLSCert getTLSCertFunc) getTLSCertFunc {
	if getTLSCert == nil {
		return nil
	}
	var mu sync.Mutex
	var deadline uint64
	var cert *tls.Certificate
	var err error
	return func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		// Cache the certificate and the error for up to a second in order to save CPU time
		// on certificate parsing when TLS connections are frequently re-established.
		mu.Lock()
		defer mu.Unlock()
		if fasttime.UnixTimestamp() > deadline {
			cert, err = getTLSCert(cri)
			deadline = fasttime.UnixTimestamp() + 1
		}
		return cert, err
	}
}

// GetTLSConfig returns cached tls configuration
func (ac *Config) GetTLSConfig() (*tls.Config, error) {
	if ac.getTLSConfigCached == nil {
		logger.Panicf("BUG: config must be properly initialized with Options.NewConfig() call")
	}
	tlsC, err := ac.getTLSConfigCached()
	if err != nil {
		return nil, err
	}
	return tlsC, nil
}

// NewRoundTripper returns new http.RoundTripper for the given ac, which uses the given trBase as base transport.
//
// The caller shouldn't change the trBase, since the returned RoundTripper owns it.
func (ac *Config) NewRoundTripper(trBase *http.Transport) http.RoundTripper {
	rt := &roundTripper{
		trBase: trBase,
	}
	if ac != nil {
		rt.getTLSConfigCached = ac.getTLSConfigCached
	}
	return rt
}

type roundTripper struct {
	trBase             *http.Transport
	getTLSConfigCached getTLSConfigFunc

	// mu protects access to rootCAPrev and trPrev
	mu         sync.Mutex
	rootCAPrev *x509.CertPool
	trPrev     *http.Transport
}

// RoundTrip implements http.RoundTripper interface.
func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tr, err := rt.getTransport()
	if err != nil {
		return nil, fmt.Errorf("cannot initialize Transport: %w", err)
	}
	return tr.RoundTrip(req)
}

func (rt *roundTripper) getTransport() (*http.Transport, error) {
	if rt.getTLSConfigCached == nil {
		return rt.trBase, nil
	}

	tlsCfg, err := rt.getTLSConfigCached()
	if err != nil {
		return nil, fmt.Errorf("cannot initialize TLS config: %w", err)
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.trPrev != nil && tlsCfg.RootCAs.Equal(rt.rootCAPrev) {
		// Fast path - tlsCfg wasn't changed. Return the previously created transport.
		return rt.trPrev, nil
	}

	// Slow path - tlsCfg has been changed.
	// Close connections for the previous transport and create new transport for the updated tlsCfg.
	if rt.trPrev != nil {
		rt.trPrev.CloseIdleConnections()
	}

	tr := rt.trBase.Clone()
	tr.TLSClientConfig = tlsCfg.Clone()

	rt.trPrev = tr
	rt.rootCAPrev = tlsCfg.RootCAs

	return rt.trPrev, nil
}

func (ac *Config) getTLSConfig() (*tls.Config, error) {
	if ac.getTLSCertCached == nil && ac.tlsServerName == "" && !ac.tlsInsecureSkipVerify && ac.tlsMinVersion == 0 && ac.getTLSRootCA == nil {
		// Reuse zeroTLSConfig when ac doesn't contain tls-specific configs.
		// This should reduce memory usage a bit.
		return zeroTLSConfig, nil
	}

	tlsCfg := &tls.Config{
		ClientSessionCache:   tls.NewLRUClientSessionCache(0),
		GetClientCertificate: ac.getTLSCertCached,
		ServerName:           ac.tlsServerName,
		InsecureSkipVerify:   ac.tlsInsecureSkipVerify,
		MinVersion:           ac.tlsMinVersion,
		// Do not set MaxVersion, since this has no sense from security PoV.
		// This can only result in lower security level if improperly configured.
	}
	if f := ac.getTLSRootCA; f != nil {
		rootCA, err := f()
		if err != nil {
			return nil, fmt.Errorf("cannot load root CAs: %w", err)
		}
		tlsCfg.RootCAs = rootCA
	}
	return tlsCfg, nil
}

var zeroTLSConfig = &tls.Config{
	ClientSessionCache: tls.NewLRUClientSessionCache(0),
}

// NewConfig creates auth config for the given hcc.
func (hcc *HTTPClientConfig) NewConfig(baseDir string) (*Config, error) {
	opts := &Options{
		BaseDir:         baseDir,
		Authorization:   hcc.Authorization,
		BasicAuth:       hcc.BasicAuth,
		BearerToken:     hcc.BearerToken.String(),
		BearerTokenFile: hcc.BearerTokenFile,
		OAuth2:          hcc.OAuth2,
		TLSConfig:       hcc.TLSConfig,
		Headers:         hcc.Headers,
	}
	return opts.NewConfig()
}

// NewConfig creates auth config for the given pcc.
func (pcc *ProxyClientConfig) NewConfig(baseDir string) (*Config, error) {
	opts := &Options{
		BaseDir:         baseDir,
		Authorization:   pcc.Authorization,
		BasicAuth:       pcc.BasicAuth,
		BearerToken:     pcc.BearerToken.String(),
		BearerTokenFile: pcc.BearerTokenFile,
		OAuth2:          pcc.OAuth2,
		TLSConfig:       pcc.TLSConfig,
		Headers:         pcc.Headers,
	}
	return opts.NewConfig()
}

// NewConfig creates auth config for the given ba.
func (ba *BasicAuthConfig) NewConfig(baseDir string) (*Config, error) {
	opts := &Options{
		BaseDir:   baseDir,
		BasicAuth: ba,
	}
	return opts.NewConfig()
}

// Options contain options, which must be passed to NewConfig.
type Options struct {
	// BaseDir is an optional path to a base directory for resolving
	// relative filepaths in various config options.
	//
	// It is set to the current directory by default.
	BaseDir string

	// Authorization contains optional Authorization.
	Authorization *Authorization

	// BasicAuth contains optional BasicAuthConfig.
	BasicAuth *BasicAuthConfig

	// BearerToken contains optional bearer token.
	BearerToken string

	// BearerTokenFile contains optional path to a file with bearer token.
	BearerTokenFile string

	// OAuth2 contains optional OAuth2Config.
	OAuth2 *OAuth2Config

	// TLSconfig contains optional TLSConfig.
	TLSConfig *TLSConfig

	// Headers contains optional http request headers in the form 'Foo: bar'.
	Headers []string
}

// NewConfig creates auth config from the given opts.
func (opts *Options) NewConfig() (*Config, error) {
	baseDir := opts.BaseDir
	if baseDir == "" {
		baseDir = "."
	}
	var actx authContext
	if opts.Authorization != nil {
		if err := actx.initFromAuthorization(baseDir, opts.Authorization); err != nil {
			return nil, err
		}
	}
	if opts.BasicAuth != nil {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot use both `authorization` and `basic_auth`")
		}
		if err := actx.initFromBasicAuthConfig(baseDir, opts.BasicAuth); err != nil {
			return nil, err
		}
	}
	if opts.BearerTokenFile != "" {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token_file`")
		}
		if opts.BearerToken != "" {
			return nil, fmt.Errorf("both `bearer_token`=%q and `bearer_token_file`=%q are set", opts.BearerToken, opts.BearerTokenFile)
		}
		actx.mustInitFromBearerTokenFile(baseDir, opts.BearerTokenFile)
	}
	if opts.BearerToken != "" {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token`")
		}
		actx.mustInitFromBearerToken(opts.BearerToken)
	}
	if opts.OAuth2 != nil {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth, `bearer_token` and `ouath2`")
		}
		if err := actx.initFromOAuth2Config(baseDir, opts.OAuth2); err != nil {
			return nil, fmt.Errorf("cannot initialize oauth2: %w", err)
		}
	}
	var tctx tlsContext
	if opts.TLSConfig != nil {
		if err := tctx.initFromTLSConfig(baseDir, opts.TLSConfig); err != nil {
			return nil, fmt.Errorf("cannot initialize tls: %w", err)
		}
	}
	headers, err := parseHeaders(opts.Headers)
	if err != nil {
		return nil, fmt.Errorf("cannot parse headers: %w", err)
	}
	hd := xxhash.New()
	for _, kv := range headers {
		_, _ = hd.Write([]byte(kv.key))
		_, _ = hd.Write([]byte("="))
		_, _ = hd.Write([]byte(kv.value))
		_, _ = hd.Write([]byte(","))
	}
	headersDigest := fmt.Sprintf("digest(headers)=%d", hd.Sum64())

	ac := &Config{
		tlsServerName:         tctx.serverName,
		tlsInsecureSkipVerify: tctx.insecureSkipVerify,
		tlsMinVersion:         tctx.minVersion,

		getTLSRootCA:    tctx.getTLSRootCA,
		tlsRootCADigest: tctx.tlsRootCADigest,

		getTLSCertCached: newGetTLSCertCached(tctx.getTLSCert),
		tlsCertDigest:    tctx.tlsCertDigest,

		getAuthHeaderCached: newGetAuthHeaderCached(actx.getAuthHeader),
		authHeaderDigest:    actx.authHeaderDigest,

		headers:       headers,
		headersDigest: headersDigest,
	}
	ac.getTLSConfigCached = newGetTLSConfigCached(ac.getTLSConfig)
	return ac, nil
}

type authContext struct {
	// getAuthHeader must return <value> for 'Authorization: <value>' http request header
	getAuthHeader getAuthHeaderFunc

	// authHeaderDigest must contain the digest for the used authorization
	// The digest must be changed whenever the original config changes.
	authHeaderDigest string
}

func (actx *authContext) initFromAuthorization(baseDir string, az *Authorization) error {
	azType := "Bearer"
	if az.Type != "" {
		azType = az.Type
	}
	if az.CredentialsFile == "" {
		ah := azType + " " + az.Credentials.String()
		actx.getAuthHeader = func() (string, error) {
			return ah, nil
		}
		actx.authHeaderDigest = fmt.Sprintf("custom(type=%q, creds=%q)", az.Type, az.Credentials)
		return nil
	}
	if az.Credentials != nil {
		return fmt.Errorf("both `credentials`=%q and `credentials_file`=%q are set", az.Credentials, az.CredentialsFile)
	}
	filePath := fscore.GetFilepath(baseDir, az.CredentialsFile)
	actx.getAuthHeader = func() (string, error) {
		token, err := fscore.ReadPasswordFromFileOrHTTP(filePath)
		if err != nil {
			return "", fmt.Errorf("cannot read credentials from `credentials_file`=%q: %w", az.CredentialsFile, err)
		}
		return azType + " " + token, nil
	}
	actx.authHeaderDigest = fmt.Sprintf("custom(type=%q, credsFile=%q)", az.Type, filePath)
	return nil
}

func (actx *authContext) initFromBasicAuthConfig(baseDir string, ba *BasicAuthConfig) error {
	username := ba.Username
	usernameFile := ba.UsernameFile
	password := ""
	if ba.Password != nil {
		password = ba.Password.S
	}
	passwordFile := ba.PasswordFile
	if username == "" && usernameFile == "" {
		return fmt.Errorf("missing `username` and `username_file` in `basic_auth` section; please specify one; " +
			"see https://docs.victoriametrics.com/victoriametrics/sd_configs/#http-api-client-options")
	}
	if username != "" && usernameFile != "" {
		return fmt.Errorf("both `username` and `username_file` are set in `basic_auth` section; please specify only one; " +
			"see https://docs.victoriametrics.com/victoriametrics/sd_configs/#http-api-client-options")
	}
	if password != "" && passwordFile != "" {
		return fmt.Errorf("both `password` and `password_file` are set in `basic_auth` section; please specify only one; " +
			"see https://docs.victoriametrics.com/victoriametrics/sd_configs/#http-api-client-options")
	}
	if usernameFile != "" {
		usernameFile = fscore.GetFilepath(baseDir, usernameFile)
	}
	if passwordFile != "" {
		passwordFile = fscore.GetFilepath(baseDir, passwordFile)
	}
	actx.getAuthHeader = func() (string, error) {
		usernameLocal := username
		if usernameFile != "" {
			s, err := fscore.ReadPasswordFromFileOrHTTP(usernameFile)
			if err != nil {
				return "", fmt.Errorf("cannot read username from `username_file`=%q: %w", usernameFile, err)
			}
			usernameLocal = s
		}
		passwordLocal := password
		if passwordFile != "" {
			s, err := fscore.ReadPasswordFromFileOrHTTP(passwordFile)
			if err != nil {
				return "", fmt.Errorf("cannot read password from `password_file`=%q: %w", passwordFile, err)
			}
			passwordLocal = s
		}
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := usernameLocal + ":" + passwordLocal
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		return "Basic " + token64, nil
	}
	actx.authHeaderDigest = fmt.Sprintf("basic(username=%q, usernameFile=%q, password=%q, passwordFile=%q)", username, usernameFile, password, passwordFile)
	return nil
}

func (actx *authContext) mustInitFromBearerTokenFile(baseDir string, bearerTokenFile string) {
	filePath := fscore.GetFilepath(baseDir, bearerTokenFile)
	actx.getAuthHeader = func() (string, error) {
		token, err := fscore.ReadPasswordFromFileOrHTTP(filePath)
		if err != nil {
			return "", fmt.Errorf("cannot read bearer token from `bearer_token_file`=%q: %w", bearerTokenFile, err)
		}
		return "Bearer " + token, nil
	}
	actx.authHeaderDigest = fmt.Sprintf("bearer(tokenFile=%q)", filePath)
}

func (actx *authContext) mustInitFromBearerToken(bearerToken string) {
	ah := "Bearer " + bearerToken
	actx.getAuthHeader = func() (string, error) {
		return ah, nil
	}
	actx.authHeaderDigest = fmt.Sprintf("bearer(token=%q)", bearerToken)
}

func (actx *authContext) initFromOAuth2Config(baseDir string, o *OAuth2Config) error {
	oi, err := newOAuth2ConfigInternal(baseDir, o)
	if err != nil {
		return err
	}
	actx.getAuthHeader = func() (string, error) {
		ts, err := oi.getTokenSource()
		if err != nil {
			return "", fmt.Errorf("cannot get OAuth2 tokenSource: %w", err)
		}
		t, err := ts.Token()
		if err != nil {
			return "", fmt.Errorf("cannot get OAuth2 token: %w", err)
		}
		return t.Type() + " " + t.AccessToken, nil
	}
	actx.authHeaderDigest = fmt.Sprintf("oauth2(%s)", oi.String())
	return nil
}

type tlsContext struct {
	getTLSCert    getTLSCertFunc
	tlsCertDigest string

	getTLSRootCA    getTLSRootCAFunc
	tlsRootCADigest string

	serverName         string
	insecureSkipVerify bool
	minVersion         uint16
}

func (tctx *tlsContext) initFromTLSConfig(baseDir string, tc *TLSConfig) error {
	tctx.serverName = tc.ServerName
	tctx.insecureSkipVerify = tc.InsecureSkipVerify
	if len(tc.Key) != 0 || len(tc.Cert) != 0 {
		cert, err := tls.X509KeyPair([]byte(tc.Cert), []byte(tc.Key))
		if err != nil {
			return fmt.Errorf("cannot load TLS certificate from the provided `cert` and `key` values: %w", err)
		}
		tctx.getTLSCert = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
		h := xxhash.Sum64([]byte(tc.Key)) ^ xxhash.Sum64([]byte(tc.Cert))
		tctx.tlsCertDigest = fmt.Sprintf("digest(key+cert)=%d", h)
	} else if tc.CertFile != "" || tc.KeyFile != "" {
		certPath := fscore.GetFilepath(baseDir, tc.CertFile)
		keyPath := fscore.GetFilepath(baseDir, tc.KeyFile)
		tctx.getTLSCert = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			// Re-read TLS certificate from disk. This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1420
			certData, err := fscore.ReadFileOrHTTP(certPath)
			if err != nil {
				return nil, fmt.Errorf("cannot read TLS certificate from %q: %w", certPath, err)
			}
			keyData, err := fscore.ReadFileOrHTTP(keyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot read TLS key from %q: %w", keyPath, err)
			}
			cert, err := tls.X509KeyPair(certData, keyData)
			if err != nil {
				return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", tc.CertFile, tc.KeyFile, err)
			}
			return &cert, nil
		}
		tctx.tlsCertDigest = fmt.Sprintf("certFile=%q, keyFile=%q", tc.CertFile, tc.KeyFile)
	}
	if len(tc.CA) != 0 {
		rootCA := x509.NewCertPool()
		if !rootCA.AppendCertsFromPEM([]byte(tc.CA)) {
			return fmt.Errorf("cannot parse data from `ca` value")
		}
		tctx.getTLSRootCA = func() (*x509.CertPool, error) {
			return rootCA, nil
		}
		h := xxhash.Sum64([]byte(tc.CA))
		tctx.tlsRootCADigest = fmt.Sprintf("digest(CA)=%d", h)
	} else if tc.CAFile != "" {
		path := fscore.GetFilepath(baseDir, tc.CAFile)
		tctx.getTLSRootCA = func() (*x509.CertPool, error) {
			data, err := fscore.ReadFileOrHTTP(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read `ca_file`: %w", err)
			}
			rootCA := x509.NewCertPool()
			if !rootCA.AppendCertsFromPEM(data) {
				return nil, fmt.Errorf("cannot parse data read from `ca_file` %q", tc.CAFile)
			}
			return rootCA, nil
		}
		tctx.tlsRootCADigest = fmt.Sprintf("caFile=%q", tc.CAFile)
	}
	v, err := netutil.ParseTLSVersion(tc.MinVersion)
	if err != nil {
		return fmt.Errorf("cannot parse `min_version`: %w", err)
	}
	tctx.minVersion = v
	return nil
}
