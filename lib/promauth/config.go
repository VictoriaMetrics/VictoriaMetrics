package promauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
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

// OAuth2Config represent oAuth2 configuration
type OAuth2Config struct {
	ClientID         string
	ClientSecretFile string
	Scopes           []string
	TokenURL         string
	// mu guards tokenSource and client Secret
	mu           sync.Mutex
	ClientSecret string
	tokenSource  oauth2.TokenSource
}

// UnmarshalYAML implements interface
func (o *OAuth2Config) UnmarshalYAML(f func(interface{}) error) error {
	var s OAuth2Config
	if err := f(&s); err != nil {
		return err
	}
	if err := s.Validate(); err != nil {
		return err
	}
	o.ClientID = s.ClientID
	o.ClientSecret = s.ClientSecret
	o.ClientSecretFile = s.ClientSecretFile
	o.TokenURL = s.TokenURL
	o.Scopes = s.Scopes
	if o.ClientSecretFile != "" {
		secret, err := readPasswordFromFile(o.ClientSecretFile)
		if err != nil {
			return err
		}
		o.ClientSecret = secret
	}
	o.refreshTokenSourceLocked()
	return nil
}

// NewOAuth2Config creates new config with given params.
func NewOAuth2Config(clientID, clientSecret, SecretFile, tokenURL string, scopes []string) (*OAuth2Config, error) {

	cfg := OAuth2Config{
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		ClientSecretFile: SecretFile,
		TokenURL:         tokenURL,
		Scopes:           scopes,
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.ClientSecretFile != "" {
		secret, err := readPasswordFromFile(cfg.ClientSecretFile)
		if err != nil {
			return nil, err
		}
		cfg.ClientSecret = secret
	}
	cfg.refreshTokenSourceLocked()
	return &cfg, nil
}

func (o *OAuth2Config) refreshTokenSourceLocked() {
	cfg := clientcredentials.Config{
		ClientID:     o.ClientID,
		ClientSecret: o.ClientSecret,
		TokenURL:     o.TokenURL,
		Scopes:       o.Scopes,
	}
	o.tokenSource = cfg.TokenSource(context.Background())
}

// Validate validate given configs.
func (o *OAuth2Config) Validate() error {
	if o.TokenURL == "" {
		return fmt.Errorf("token url cannot be empty")
	}
	if o.ClientSecret == "" && o.ClientSecretFile == "" {
		return fmt.Errorf("ClientSecret or ClientSecretFile must be set")
	}
	return nil
}

func (o *OAuth2Config) getAuthHeader() (string, error) {
	var needUpdate bool
	if o.ClientSecretFile != "" {
		newSecret, err := readPasswordFromFile(o.ClientSecretFile)
		if err != nil {
			return "", err
		}
		o.mu.Lock()
		if o.ClientSecret != newSecret {
			o.ClientSecret = newSecret
			needUpdate = true
		}
		o.mu.Unlock()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if needUpdate {
		o.refreshTokenSourceLocked()
	}
	t, err := o.tokenSource.Token()
	if err != nil {
		return "", err
	}

	return t.Type() + " " + t.AccessToken, nil
}

// Config is auth config.
type Config struct {
	// Optional TLS config
	TLSRootCA             *x509.CertPool
	TLSCertificate        *tls.Certificate
	TLSServerName         string
	TLSInsecureSkipVerify bool

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
		ac.authDigest, ac.tlsRootCAString(), ac.tlsCertificateString(), ac.TLSServerName, ac.TLSInsecureSkipVerify)
}

func (ac *Config) tlsRootCAString() string {
	if ac.TLSRootCA == nil {
		return ""
	}
	data := ac.TLSRootCA.Subjects()
	return string(bytes.Join(data, []byte("\n")))
}

func (ac *Config) tlsCertificateString() string {
	if ac.TLSCertificate == nil {
		return ""
	}
	return string(bytes.Join(ac.TLSCertificate.Certificate, []byte("\n")))
}

// NewTLSConfig returns new TLS config for the given ac.
func (ac *Config) NewTLSConfig() *tls.Config {
	tlsCfg := &tls.Config{
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	if ac == nil {
		return tlsCfg
	}
	if ac.TLSCertificate != nil {
		// Do not set tlsCfg.GetClientCertificate, since tlsCfg.Certificates should work OK.
		tlsCfg.Certificates = []tls.Certificate{*ac.TLSCertificate}
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
func NewConfig(baseDir string, az *Authorization, basicAuth *BasicAuthConfig, bearerToken, bearerTokenFile string, oauth *OAuth2Config, tlsConfig *TLSConfig) (*Config, error) {
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
	if oauth != nil {
		if getAuthHeader != nil {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth, `bearer_token` and `ouath2`")
		}
		getAuthHeader = func() string {
			h, err := oauth.getAuthHeader()
			if err != nil {
				logger.Errorf("cannot get OAuth2 header: %s", err)
				return ""
			}
			return h
		}
	}
	var tlsRootCA *x509.CertPool
	var tlsCertificate *tls.Certificate
	tlsServerName := ""
	tlsInsecureSkipVerify := false
	if tlsConfig != nil {
		tlsServerName = tlsConfig.ServerName
		tlsInsecureSkipVerify = tlsConfig.InsecureSkipVerify
		if tlsConfig.CertFile != "" || tlsConfig.KeyFile != "" {
			certPath := getFilepath(baseDir, tlsConfig.CertFile)
			keyPath := getFilepath(baseDir, tlsConfig.KeyFile)
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", tlsConfig.CertFile, tlsConfig.KeyFile, err)
			}
			tlsCertificate = &cert
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
		TLSCertificate:        tlsCertificate,
		TLSServerName:         tlsServerName,
		TLSInsecureSkipVerify: tlsInsecureSkipVerify,

		getAuthHeader: getAuthHeader,
		authDigest:    authDigest,
	}
	return ac, nil
}
