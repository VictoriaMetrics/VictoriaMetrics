package promauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// TLSConfig represents TLS config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
type TLSConfig struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
}

// Authorization represents generic authorization config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Authorization struct {
	Type            string `yaml:"type,omitempty"`
	Credentials     string `yaml:"credentials,omitempty"`
	CredentialsFile string `yaml:"credentials_file,omitempty"`
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password,omitempty"`
	PasswordFile string `yaml:"password_file,omitempty"`
}

// HTTPClientConfig represents http client config.
type HTTPClientConfig struct {
	Authorization   *Authorization   `yaml:"authorization,omitempty"`
	BasicAuth       *BasicAuthConfig `yaml:"basic_auth,omitempty"`
	BearerToken     string           `yaml:"bearer_token,omitempty"`
	BearerTokenFile string           `yaml:"bearer_token_file,omitempty"`
	OAuth2          *OAuth2Config    `yaml:"oauth2,omitempty"`
	TLSConfig       *TLSConfig       `yaml:"tls_config,omitempty"`
}

// ProxyClientConfig represents proxy client config.
type ProxyClientConfig struct {
	Authorization   *Authorization   `yaml:"proxy_authorization,omitempty"`
	BasicAuth       *BasicAuthConfig `yaml:"proxy_basic_auth,omitempty"`
	BearerToken     string           `yaml:"proxy_bearer_token,omitempty"`
	BearerTokenFile string           `yaml:"proxy_bearer_token_file,omitempty"`
	TLSConfig       *TLSConfig       `yaml:"proxy_tls_config,omitempty"`
}

// OAuth2Config represent OAuth2 configuration
type OAuth2Config struct {
	ClientID         string            `yaml:"client_id"`
	ClientSecret     string            `yaml:"client_secret"`
	ClientSecretFile string            `yaml:"client_secret_file"`
	Scopes           []string          `yaml:"scopes"`
	TokenURL         string            `yaml:"token_url"`
	EndpointParams   map[string]string `yaml:"endpoint_params"`
}

// String returns string representation of o.
func (o *OAuth2Config) String() string {
	return fmt.Sprintf("clientID=%q, clientSecret=%q, clientSecretFile=%q, Scopes=%q, tokenURL=%q, endpointParams=%q",
		o.ClientID, o.ClientSecret, o.ClientSecretFile, o.Scopes, o.TokenURL, o.EndpointParams)
}

func (o *OAuth2Config) validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("client_id cannot be empty")
	}
	if o.ClientSecret == "" && o.ClientSecretFile == "" {
		return fmt.Errorf("ClientSecret or ClientSecretFile must be set")
	}
	if o.ClientSecret != "" && o.ClientSecretFile != "" {
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
	tokenSource      oauth2.TokenSource
}

func newOAuth2ConfigInternal(baseDir string, o *OAuth2Config) (*oauth2ConfigInternal, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	oi := &oauth2ConfigInternal{
		cfg: &clientcredentials.Config{
			ClientID:       o.ClientID,
			ClientSecret:   o.ClientSecret,
			TokenURL:       o.TokenURL,
			Scopes:         o.Scopes,
			EndpointParams: urlValuesFromMap(o.EndpointParams),
		},
	}
	if o.ClientSecretFile != "" {
		oi.clientSecretFile = getFilepath(baseDir, o.ClientSecretFile)
		secret, err := readPasswordFromFile(oi.clientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read OAuth2 secret from %q: %w", oi.clientSecretFile, err)
		}
		oi.cfg.ClientSecret = secret
	}
	oi.tokenSource = oi.cfg.TokenSource(context.Background())
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
	oi.tokenSource = oi.cfg.TokenSource(context.Background())
	return oi.tokenSource, nil
}

// Config is auth config.
type Config struct {
	// Optional TLS config
	TLSRootCA             *x509.CertPool
	TLSServerName         string
	TLSInsecureSkipVerify bool

	getTLSCert    func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	tlsCertDigest string

	getAuthHeader      func() string
	authHeaderLock     sync.Mutex
	authHeader         string
	authHeaderDeadline uint64

	authDigest string
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
func (ac *Config) String() string {
	return fmt.Sprintf("AuthDigest=%s, TLSRootCA=%s, TLSCertificate=%s, TLSServerName=%s, TLSInsecureSkipVerify=%v",
		ac.authDigest, ac.tlsRootCAString(), ac.tlsCertDigest, ac.TLSServerName, ac.TLSInsecureSkipVerify)
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
	return tlsCfg
}

// NewConfig creates auth config for the given hcc.
func (hcc *HTTPClientConfig) NewConfig(baseDir string) (*Config, error) {
	return NewConfig(baseDir, hcc.Authorization, hcc.BasicAuth, hcc.BearerToken, hcc.BearerTokenFile, hcc.OAuth2, hcc.TLSConfig)
}

// NewConfig creates auth config for the given pcc.
func (pcc *ProxyClientConfig) NewConfig(baseDir string) (*Config, error) {
	return NewConfig(baseDir, pcc.Authorization, pcc.BasicAuth, pcc.BearerToken, pcc.BearerTokenFile, nil, pcc.TLSConfig)
}

// NewConfig creates auth config from the given args.
func NewConfig(baseDir string, az *Authorization, basicAuth *BasicAuthConfig, bearerToken, bearerTokenFile string, o *OAuth2Config, tlsConfig *TLSConfig) (*Config, error) {
	var getAuthHeader func() string
	authDigest := ""
	if az != nil {
		azType := "Bearer"
		if az.Type != "" {
			azType = az.Type
		}
		if az.CredentialsFile != "" {
			if az.Credentials != "" {
				return nil, fmt.Errorf("both `credentials`=%q and `credentials_file`=%q are set", az.Credentials, az.CredentialsFile)
			}
			filePath := getFilepath(baseDir, az.CredentialsFile)
			getAuthHeader = func() string {
				token, err := readPasswordFromFile(filePath)
				if err != nil {
					logger.Errorf("cannot read credentials from `credentials_file`=%q: %s", az.CredentialsFile, err)
					return ""
				}
				return azType + " " + token
			}
			authDigest = fmt.Sprintf("custom(type=%q, credsFile=%q)", az.Type, filePath)
		} else {
			getAuthHeader = func() string {
				return azType + " " + az.Credentials
			}
			authDigest = fmt.Sprintf("custom(type=%q, creds=%q)", az.Type, az.Credentials)
		}
	}
	if basicAuth != nil {
		if getAuthHeader != nil {
			return nil, fmt.Errorf("cannot use both `authorization` and `basic_auth`")
		}
		if basicAuth.Username == "" {
			return nil, fmt.Errorf("missing `username` in `basic_auth` section")
		}
		if basicAuth.PasswordFile != "" {
			if basicAuth.Password != "" {
				return nil, fmt.Errorf("both `password`=%q and `password_file`=%q are set in `basic_auth` section", basicAuth.Password, basicAuth.PasswordFile)
			}
			filePath := getFilepath(baseDir, basicAuth.PasswordFile)
			getAuthHeader = func() string {
				password, err := readPasswordFromFile(filePath)
				if err != nil {
					logger.Errorf("cannot read password from `password_file`=%q set in `basic_auth` section: %s", basicAuth.PasswordFile, err)
					return ""
				}
				// See https://en.wikipedia.org/wiki/Basic_access_authentication
				token := basicAuth.Username + ":" + password
				token64 := base64.StdEncoding.EncodeToString([]byte(token))
				return "Basic " + token64
			}
			authDigest = fmt.Sprintf("basic(username=%q, passwordFile=%q)", basicAuth.Username, filePath)
		} else {
			getAuthHeader = func() string {
				// See https://en.wikipedia.org/wiki/Basic_access_authentication
				token := basicAuth.Username + ":" + basicAuth.Password
				token64 := base64.StdEncoding.EncodeToString([]byte(token))
				return "Basic " + token64
			}
			authDigest = fmt.Sprintf("basic(username=%q, password=%q)", basicAuth.Username, basicAuth.Password)
		}
	}
	if bearerTokenFile != "" {
		if getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token_file`")
		}
		if bearerToken != "" {
			return nil, fmt.Errorf("both `bearer_token`=%q and `bearer_token_file`=%q are set", bearerToken, bearerTokenFile)
		}
		filePath := getFilepath(baseDir, bearerTokenFile)
		getAuthHeader = func() string {
			token, err := readPasswordFromFile(filePath)
			if err != nil {
				logger.Errorf("cannot read bearer token from `bearer_token_file`=%q: %s", bearerTokenFile, err)
				return ""
			}
			return "Bearer " + token
		}
		authDigest = fmt.Sprintf("bearer(tokenFile=%q)", filePath)
	}
	if bearerToken != "" {
		if getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token`")
		}
		getAuthHeader = func() string {
			return "Bearer " + bearerToken
		}
		authDigest = fmt.Sprintf("bearer(token=%q)", bearerToken)
	}
	if o != nil {
		if getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth, `bearer_token` and `ouath2`")
		}
		oi, err := newOAuth2ConfigInternal(baseDir, o)
		if err != nil {
			return nil, err
		}
		getAuthHeader = func() string {
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
		authDigest = fmt.Sprintf("oauth2(%s)", o.String())
	}
	var tlsRootCA *x509.CertPool
	var getTLSCert func(*tls.CertificateRequestInfo) (*tls.Certificate, error)
	tlsCertDigest := ""
	tlsServerName := ""
	tlsInsecureSkipVerify := false
	if tlsConfig != nil {
		tlsServerName = tlsConfig.ServerName
		tlsInsecureSkipVerify = tlsConfig.InsecureSkipVerify
		if tlsConfig.CertFile != "" || tlsConfig.KeyFile != "" {
			getTLSCert = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				// Re-read TLS certificate from disk. This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1420
				certPath := getFilepath(baseDir, tlsConfig.CertFile)
				keyPath := getFilepath(baseDir, tlsConfig.KeyFile)
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", tlsConfig.CertFile, tlsConfig.KeyFile, err)
				}
				return &cert, nil
			}
			// Check whether the configured TLS cert can be loaded.
			if _, err := getTLSCert(nil); err != nil {
				return nil, err
			}
			tlsCertDigest = fmt.Sprintf("certFile=%q, keyFile=%q", tlsConfig.CertFile, tlsConfig.KeyFile)
		}
		if tlsConfig.CAFile != "" {
			path := getFilepath(baseDir, tlsConfig.CAFile)
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read `ca_file` %q: %w", tlsConfig.CAFile, err)
			}
			tlsRootCA = x509.NewCertPool()
			if !tlsRootCA.AppendCertsFromPEM(data) {
				return nil, fmt.Errorf("cannot parse data from `ca_file` %q", tlsConfig.CAFile)
			}
		}
	}
	ac := &Config{
		TLSRootCA:             tlsRootCA,
		TLSServerName:         tlsServerName,
		TLSInsecureSkipVerify: tlsInsecureSkipVerify,

		getTLSCert:    getTLSCert,
		tlsCertDigest: tlsCertDigest,

		getAuthHeader: getAuthHeader,
		authDigest:    authDigest,
	}
	return ac, nil
}
