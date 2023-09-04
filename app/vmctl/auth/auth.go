package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// HTTPClientConfig represents http client config.
type HTTPClientConfig struct {
	BasicAuth   *BasicAuthConfig
	BearerToken string
	Headers     string
}

// NewConfig creates auth config for the given hcc.
func (hcc *HTTPClientConfig) NewConfig() (*Config, error) {
	opts := &Options{
		BasicAuth:   hcc.BasicAuth,
		BearerToken: hcc.BearerToken,
		Headers:     hcc.Headers,
	}
	return opts.NewConfig()
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string
	Password     string
	PasswordFile string
}

// ConfigOptions options which helps build Config
type ConfigOptions func(config *HTTPClientConfig)

// Generate returns Config based on the given params
func Generate(filterOptions ...ConfigOptions) (*Config, error) {
	authCfg := &HTTPClientConfig{}
	for _, option := range filterOptions {
		option(authCfg)
	}

	return authCfg.NewConfig()
}

// WithBasicAuth returns AuthConfigOptions and initialized BasicAuthConfig based on given params
func WithBasicAuth(username, password string) ConfigOptions {
	return func(config *HTTPClientConfig) {
		if username != "" || password != "" {
			config.BasicAuth = &BasicAuthConfig{
				Username: username,
				Password: password,
			}
		}
	}
}

// WithBearer returns AuthConfigOptions and set BearerToken or BearerTokenFile based on given params
func WithBearer(token string) ConfigOptions {
	return func(config *HTTPClientConfig) {
		if token != "" {
			config.BearerToken = token
		}
	}
}

// WithHeaders returns AuthConfigOptions and set Headers based on the given params
func WithHeaders(headers string) ConfigOptions {
	return func(config *HTTPClientConfig) {
		if headers != "" {
			config.Headers = headers
		}
	}
}

// Config is auth config.
type Config struct {
	getAuthHeader      func() string
	authHeaderLock     sync.Mutex
	authHeader         string
	authHeaderDeadline uint64

	headers []keyValue

	authDigest string
}

// SetHeaders sets the configured ac headers to req.
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

type authContext struct {
	// getAuthHeader must return <value> for 'Authorization: <value>' http request header
	getAuthHeader func() string

	// authDigest must contain the digest for the used authorization
	// The digest must be changed whenever the original config changes.
	authDigest string
}

func (ac *authContext) initFromBasicAuthConfig(ba *BasicAuthConfig) error {
	if ba.Username == "" {
		return fmt.Errorf("missing `username` in `basic_auth` section")
	}
	if ba.Password != "" {
		ac.getAuthHeader = func() string {
			// See https://en.wikipedia.org/wiki/Basic_access_authentication
			token := ba.Username + ":" + ba.Password
			token64 := base64.StdEncoding.EncodeToString([]byte(token))
			return "Basic " + token64
		}
		ac.authDigest = fmt.Sprintf("basic(username=%q, password=%q)", ba.Username, ba.Password)
		return nil
	}
	return nil
}

func (ac *authContext) initFromBearerToken(bearerToken string) error {
	ac.getAuthHeader = func() string {
		return "Bearer " + bearerToken
	}
	ac.authDigest = fmt.Sprintf("bearer(token=%q)", bearerToken)
	return nil
}

// Options contain options, which must be passed to NewConfig.
type Options struct {
	// BasicAuth contains optional BasicAuthConfig.
	BasicAuth *BasicAuthConfig

	// BearerToken contains optional bearer token.
	BearerToken string

	// Headers contains optional http request headers in the form 'Foo: bar'.
	Headers string
}

// NewConfig creates auth config from the given opts.
func (opts *Options) NewConfig() (*Config, error) {
	var ac authContext
	if opts.BasicAuth != nil {
		if ac.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot use both `authorization` and `basic_auth`")
		}
		if err := ac.initFromBasicAuthConfig(opts.BasicAuth); err != nil {
			return nil, err
		}
	}
	if opts.BearerToken != "" {
		if ac.getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token`")
		}
		if err := ac.initFromBearerToken(opts.BearerToken); err != nil {
			return nil, err
		}
	}

	headers, err := parseHeaders(opts.Headers)
	if err != nil {
		return nil, err
	}
	c := &Config{
		getAuthHeader: ac.getAuthHeader,
		headers:       headers,
		authDigest:    ac.authDigest,
	}
	return c, nil
}

type keyValue struct {
	key   string
	value string
}

func parseHeaders(headers string) ([]keyValue, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	var headersSplitByDelimiter = strings.Split(headers, "^^")

	kvs := make([]keyValue, len(headersSplitByDelimiter))
	for i, h := range headersSplitByDelimiter {
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
