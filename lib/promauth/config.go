package promauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/cespare/xxhash/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
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
func (s *Secret) MarshalYAML() (interface{}, error) {
	return "<secret>", nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (s *Secret) UnmarshalYAML(f func(interface{}) error) error {
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
	CA                 []byte `yaml:"ca,omitempty"`
	CAFile             string `yaml:"ca_file,omitempty"`
	Cert               []byte `yaml:"cert,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	Key                []byte `yaml:"key,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	MinVersion         string `yaml:"min_version,omitempty"`
}

// String returns human-readable representation of tc
func (tc *TLSConfig) String() string {
	if tc == nil {
		return ""
	}
	caHash := xxhash.Sum64(tc.CA)
	certHash := xxhash.Sum64(tc.Cert)
	keyHash := xxhash.Sum64(tc.Key)
	return fmt.Sprintf("hash(ca)=%d, ca_file=%q, hash(cert)=%d, cert_file=%q, hash(key)=%d, key_file=%q, server_name=%q, insecure_skip_verify=%v, min_version=%q",
		caHash, tc.CAFile, certHash, tc.CertFile, keyHash, tc.KeyFile, tc.ServerName, tc.InsecureSkipVerify, tc.MinVersion)
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
	Username     string  `yaml:"username"`
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

// String returns string representation of o.
func (o *OAuth2Config) String() string {
	return fmt.Sprintf("clientID=%q, clientSecret=%q, clientSecretFile=%q, Scopes=%q, tokenURL=%q, endpointParams=%q, tlsConfig={%s}, proxyURL=%q",
		o.ClientID, o.ClientSecret, o.ClientSecretFile, o.Scopes, o.TokenURL, o.EndpointParams, o.TLSConfig.String(), o.ProxyURL)
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
	ctx              context.Context
	tokenSource      oauth2.TokenSource
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
		oi.clientSecretFile = fs.GetFilepath(baseDir, o.ClientSecretFile)
		secret, err := readPasswordFromFile(oi.clientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read OAuth2 secret from %q: %w", oi.clientSecretFile, err)
		}
		oi.cfg.ClientSecret = secret
	}
	opts := &Options{
		BaseDir:   baseDir,
		TLSConfig: o.TLSConfig,
	}
	ac, err := opts.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot initialize TLS config for OAuth2: %w", err)
	}
	tlsCfg := ac.NewTLSConfig()
	var proxyURLFunc func(*http.Request) (*url.URL, error)
	if o.ProxyURL != "" {
		u, err := url.Parse(o.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("cannot parse proxy_url=%q: %w", o.ProxyURL, err)
		}
		proxyURLFunc = http.ProxyURL(u)
	}
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
			Proxy:           proxyURLFunc,
		},
	}
	oi.ctx = context.WithValue(context.Background(), oauth2.HTTPClient, c)
	oi.tokenSource = oi.cfg.TokenSource(oi.ctx)
	return oi, nil
}

func urlValuesFromMap(m map[string]string) url.Values {
	result := make(url.Values, len(m))
	for k, v := range m {
		result[k] = []string{v}
	}
	return result
}

func (oi *oauth2ConfigInternal) getTokenSource() (oauth2.TokenSource, error) {
	oi.mu.Lock()
	defer oi.mu.Unlock()

	if oi.clientSecretFile == "" {
		return oi.tokenSource, nil
	}
	newSecret, err := readPasswordFromFile(oi.clientSecretFile)
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
	// Optional TLS config
	TLSRootCA             *x509.CertPool
	TLSServerName         string
	TLSInsecureSkipVerify bool
	TLSMinVersion         uint16

	getTLSCert    func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	tlsCertDigest string

	getAuthHeader      func() string
	authHeaderLock     sync.Mutex
	authHeader         string
	authHeaderDeadline uint64

	headers []keyValue

	authDigest string
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
		kv.key = strings.TrimSpace(h[:n])
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

// SetHeaders sets the configuted ac headers to req.
func (ac *Config) SetHeaders(req *http.Request, setAuthHeader bool) {
	reqHeaders := req.Header
	for _, h := range ac.headers {
		reqHeaders.Set(h.key, h.value)
	}
	if setAuthHeader {
		if ah := ac.GetAuthHeader(); ah != "" {
			reqHeaders.Set("Authorization", ah)
		}
	}
}

// SetFasthttpHeaders sets the configured ac headers to req.
func (ac *Config) SetFasthttpHeaders(req *fasthttp.Request, setAuthHeader bool) {
	reqHeaders := &req.Header
	for _, h := range ac.headers {
		reqHeaders.Set(h.key, h.value)
	}
	if setAuthHeader {
		if ah := ac.GetAuthHeader(); ah != "" {
			reqHeaders.Set("Authorization", ah)
		}
	}
}

// GetAuthHeader returns optional `Authorization: ...` http header.
func (ac *Config) GetAuthHeader() string {
	f := ac.getAuthHeader
	if f == nil {
		return ""
	}
	ac.authHeaderLock.Lock()
	defer ac.authHeaderLock.Unlock()
	if fasttime.UnixTimestamp() > ac.authHeaderDeadline {
		ac.authHeader = f()
		// Cache the authHeader for a second.
		ac.authHeaderDeadline = fasttime.UnixTimestamp() + 1
	}
	return ac.authHeader
}

// String returns human-readable representation for ac.
//
// It is also used for comparing Config objects for equality. If two Config
// objects have the same string representation, then they are considered equal.
func (ac *Config) String() string {
	return fmt.Sprintf("AuthDigest=%s, Headers=%s, TLSRootCA=%s, TLSCertificate=%s, TLSServerName=%s, TLSInsecureSkipVerify=%v, TLSMinVersion=%d",
		ac.authDigest, ac.headers, ac.tlsRootCAString(), ac.tlsCertDigest, ac.TLSServerName, ac.TLSInsecureSkipVerify, ac.TLSMinVersion)
}

func (ac *Config) tlsRootCAString() string {
	if ac.TLSRootCA == nil {
		return ""
	}
	data := ac.TLSRootCA.Subjects()
	return string(bytes.Join(data, []byte("\n")))
}

// NewTLSConfig returns new TLS config for the given ac.
func (ac *Config) NewTLSConfig() *tls.Config {
	tlsCfg := &tls.Config{
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	if ac == nil {
		return tlsCfg
	}
	if ac.getTLSCert != nil {
		var certLock sync.Mutex
		var cert *tls.Certificate
		var certDeadline uint64
		tlsCfg.GetClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			// Cache the certificate for up to a second in order to save CPU time
			// on certificate parsing when TLS connection are frequently re-established.
			certLock.Lock()
			defer certLock.Unlock()
			if fasttime.UnixTimestamp() > certDeadline {
				c, err := ac.getTLSCert(cri)
				if err != nil {
					return nil, err
				}
				cert = c
				certDeadline = fasttime.UnixTimestamp() + 1
			}
			return cert, nil
		}
	}
	tlsCfg.RootCAs = ac.TLSRootCA
	tlsCfg.ServerName = ac.TLSServerName
	tlsCfg.InsecureSkipVerify = ac.TLSInsecureSkipVerify
	tlsCfg.MinVersion = ac.TLSMinVersion
	return tlsCfg
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
		if err := actx.initFromBearerTokenFile(baseDir, opts.BearerTokenFile); err != nil {
			return nil, err
		}
	}
	if opts.BearerToken != "" {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token`")
		}
		if err := actx.initFromBearerToken(opts.BearerToken); err != nil {
			return nil, err
		}
	}
	if opts.OAuth2 != nil {
		if actx.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth, `bearer_token` and `ouath2`")
		}
		if err := actx.initFromOAuth2Config(baseDir, opts.OAuth2); err != nil {
			return nil, err
		}
	}
	var tctx tlsContext
	if opts.TLSConfig != nil {
		if err := tctx.initFromTLSConfig(baseDir, opts.TLSConfig); err != nil {
			return nil, err
		}
	}
	headers, err := parseHeaders(opts.Headers)
	if err != nil {
		return nil, err
	}
	ac := &Config{
		TLSRootCA:             tctx.rootCA,
		TLSServerName:         tctx.serverName,
		TLSInsecureSkipVerify: tctx.insecureSkipVerify,
		TLSMinVersion:         tctx.minVersion,

		getTLSCert:    tctx.getTLSCert,
		tlsCertDigest: tctx.tlsCertDigest,

		getAuthHeader: actx.getAuthHeader,
		headers:       headers,
		authDigest:    actx.authDigest,
	}
	return ac, nil
}

type authContext struct {
	// getAuthHeader must return <value> for 'Authorization: <value>' http request header
	getAuthHeader func() string

	// authDigest must contain the digest for the used authorization
	// The digest must be changed whenever the original config changes.
	authDigest string
}

func (actx *authContext) initFromAuthorization(baseDir string, az *Authorization) error {
	azType := "Bearer"
	if az.Type != "" {
		azType = az.Type
	}
	if az.CredentialsFile == "" {
		actx.getAuthHeader = func() string {
			return azType + " " + az.Credentials.String()
		}
		actx.authDigest = fmt.Sprintf("custom(type=%q, creds=%q)", az.Type, az.Credentials)
		return nil
	}
	if az.Credentials != nil {
		return fmt.Errorf("both `credentials`=%q and `credentials_file`=%q are set", az.Credentials, az.CredentialsFile)
	}
	filePath := fs.GetFilepath(baseDir, az.CredentialsFile)
	actx.getAuthHeader = func() string {
		token, err := readPasswordFromFile(filePath)
		if err != nil {
			logger.Errorf("cannot read credentials from `credentials_file`=%q: %s", az.CredentialsFile, err)
			return ""
		}
		return azType + " " + token
	}
	actx.authDigest = fmt.Sprintf("custom(type=%q, credsFile=%q)", az.Type, filePath)
	return nil
}

func (actx *authContext) initFromBasicAuthConfig(baseDir string, ba *BasicAuthConfig) error {
	if ba.Username == "" {
		return fmt.Errorf("missing `username` in `basic_auth` section")
	}
	if ba.PasswordFile == "" {
		actx.getAuthHeader = func() string {
			// See https://en.wikipedia.org/wiki/Basic_access_authentication
			token := ba.Username + ":" + ba.Password.String()
			token64 := base64.StdEncoding.EncodeToString([]byte(token))
			return "Basic " + token64
		}
		actx.authDigest = fmt.Sprintf("basic(username=%q, password=%q)", ba.Username, ba.Password)
		return nil
	}
	if ba.Password != nil {
		return fmt.Errorf("both `password`=%q and `password_file`=%q are set in `basic_auth` section", ba.Password, ba.PasswordFile)
	}
	filePath := fs.GetFilepath(baseDir, ba.PasswordFile)
	actx.getAuthHeader = func() string {
		password, err := readPasswordFromFile(filePath)
		if err != nil {
			logger.Errorf("cannot read password from `password_file`=%q set in `basic_auth` section: %s", ba.PasswordFile, err)
			return ""
		}
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := ba.Username + ":" + password
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		return "Basic " + token64
	}
	actx.authDigest = fmt.Sprintf("basic(username=%q, passwordFile=%q)", ba.Username, filePath)
	return nil
}

func (actx *authContext) initFromBearerTokenFile(baseDir string, bearerTokenFile string) error {
	filePath := fs.GetFilepath(baseDir, bearerTokenFile)
	actx.getAuthHeader = func() string {
		token, err := readPasswordFromFile(filePath)
		if err != nil {
			logger.Errorf("cannot read bearer token from `bearer_token_file`=%q: %s", bearerTokenFile, err)
			return ""
		}
		return "Bearer " + token
	}
	actx.authDigest = fmt.Sprintf("bearer(tokenFile=%q)", filePath)
	return nil
}

func (actx *authContext) initFromBearerToken(bearerToken string) error {
	actx.getAuthHeader = func() string {
		return "Bearer " + bearerToken
	}
	actx.authDigest = fmt.Sprintf("bearer(token=%q)", bearerToken)
	return nil
}

func (actx *authContext) initFromOAuth2Config(baseDir string, o *OAuth2Config) error {
	oi, err := newOAuth2ConfigInternal(baseDir, o)
	if err != nil {
		return err
	}
	actx.getAuthHeader = func() string {
		ts, err := oi.getTokenSource()
		if err != nil {
			logger.Errorf("cannot get OAuth2 tokenSource: %s", err)
			return ""
		}
		t, err := ts.Token()
		if err != nil {
			logger.Errorf("cannot get OAuth2 token: %s", err)
			return ""
		}
		return t.Type() + " " + t.AccessToken
	}
	actx.authDigest = fmt.Sprintf("oauth2(%s)", o.String())
	return nil
}

type tlsContext struct {
	getTLSCert         func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	tlsCertDigest      string
	rootCA             *x509.CertPool
	serverName         string
	insecureSkipVerify bool
	minVersion         uint16
}

func (tctx *tlsContext) initFromTLSConfig(baseDir string, tc *TLSConfig) error {
	tctx.serverName = tc.ServerName
	tctx.insecureSkipVerify = tc.InsecureSkipVerify
	if len(tc.Key) != 0 || len(tc.Cert) != 0 {
		cert, err := tls.X509KeyPair(tc.Cert, tc.Key)
		if err != nil {
			return fmt.Errorf("cannot load TLS certificate from the provided `cert` and `key` values: %w", err)
		}
		tctx.getTLSCert = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
		h := xxhash.Sum64(tc.Key) ^ xxhash.Sum64(tc.Cert)
		tctx.tlsCertDigest = fmt.Sprintf("digest(key+cert)=%d", h)
	} else if tc.CertFile != "" || tc.KeyFile != "" {
		tctx.getTLSCert = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			// Re-read TLS certificate from disk. This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1420
			certPath := fs.GetFilepath(baseDir, tc.CertFile)
			keyPath := fs.GetFilepath(baseDir, tc.KeyFile)
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", tc.CertFile, tc.KeyFile, err)
			}
			return &cert, nil
		}
		// Check whether the configured TLS cert can be loaded.
		if _, err := tctx.getTLSCert(nil); err != nil {
			return err
		}
		tctx.tlsCertDigest = fmt.Sprintf("certFile=%q, keyFile=%q", tc.CertFile, tc.KeyFile)
	}
	if len(tc.CA) != 0 {
		tctx.rootCA = x509.NewCertPool()
		if !tctx.rootCA.AppendCertsFromPEM(tc.CA) {
			return fmt.Errorf("cannot parse data from `ca` value")
		}
	} else if tc.CAFile != "" {
		path := fs.GetFilepath(baseDir, tc.CAFile)
		data, err := fs.ReadFileOrHTTP(path)
		if err != nil {
			return fmt.Errorf("cannot read `ca_file` %q: %w", tc.CAFile, err)
		}
		tctx.rootCA = x509.NewCertPool()
		if !tctx.rootCA.AppendCertsFromPEM(data) {
			return fmt.Errorf("cannot parse data from `ca_file` %q", tc.CAFile)
		}
	}
	if tc.MinVersion != "" {
		v, err := parseTLSVersion(tc.MinVersion)
		if err != nil {
			return fmt.Errorf("cannot parse `min_version`: %w", err)
		}
		tctx.minVersion = v
	}
	return nil
}

func parseTLSVersion(s string) (uint16, error) {
	switch strings.ToUpper(s) {
	case "TLS13":
		return tls.VersionTLS13, nil
	case "TLS12":
		return tls.VersionTLS12, nil
	case "TLS11":
		return tls.VersionTLS11, nil
	case "TLS10":
		return tls.VersionTLS10, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q", s)
	}
}
