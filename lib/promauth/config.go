package promauth

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
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

// Config is auth config.
type Config struct {
	// Optional `Authorization` header.
	//
	// It may contain `Basic ....` or `Bearer ....` string.
	Authorization string

	// Optional TLS config
	TLSRootCA             *x509.CertPool
	TLSCertificate        *tls.Certificate
	TLSServerName         string
	TLSInsecureSkipVerify bool
}

// String returns human-(un)readable representation for cfg.
func (ac *Config) String() string {
	return fmt.Sprintf("Authorization=%s, TLSRootCA=%s, TLSCertificate=%s, TLSServerName=%s, TLSInsecureSkipVerify=%v",
		ac.Authorization, ac.tlsRootCAString(), ac.tlsCertificateString(), ac.TLSServerName, ac.TLSInsecureSkipVerify)
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
	return NewConfig(baseDir, hcc.Authorization, hcc.BasicAuth, hcc.BearerToken, hcc.BearerTokenFile, hcc.TLSConfig)
}

// NewConfig creates auth config for the given pcc.
func (pcc *ProxyClientConfig) NewConfig(baseDir string) (*Config, error) {
	return NewConfig(baseDir, pcc.Authorization, pcc.BasicAuth, pcc.BearerToken, pcc.BearerTokenFile, pcc.TLSConfig)
}

// NewConfig creates auth config from the given args.
func NewConfig(baseDir string, az *Authorization, basicAuth *BasicAuthConfig, bearerToken, bearerTokenFile string, tlsConfig *TLSConfig) (*Config, error) {
	var authorization string
	if az != nil {
		azType := "Bearer"
		if az.Type != "" {
			azType = az.Type
		}
		azToken := az.Credentials
		if az.CredentialsFile != "" {
			if az.Credentials != "" {
				return nil, fmt.Errorf("both `credentials`=%q and `credentials_file`=%q are set", az.Credentials, az.CredentialsFile)
			}
			path := getFilepath(baseDir, az.CredentialsFile)
			token, err := readPasswordFromFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read credentials from `credentials_file`=%q: %w", az.CredentialsFile, err)
			}
			azToken = token
		}
		authorization = azType + " " + azToken
	}
	if basicAuth != nil {
		if authorization != "" {
			return nil, fmt.Errorf("cannot use both `authorization` and `basic_auth`")
		}
		if basicAuth.Username == "" {
			return nil, fmt.Errorf("missing `username` in `basic_auth` section")
		}
		username := basicAuth.Username
		password := basicAuth.Password
		if basicAuth.PasswordFile != "" {
			if basicAuth.Password != "" {
				return nil, fmt.Errorf("both `password`=%q and `password_file`=%q are set in `basic_auth` section", basicAuth.Password, basicAuth.PasswordFile)
			}
			path := getFilepath(baseDir, basicAuth.PasswordFile)
			pass, err := readPasswordFromFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read password from `password_file`=%q set in `basic_auth` section: %w", basicAuth.PasswordFile, err)
			}
			password = pass
		}
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := username + ":" + password
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		authorization = "Basic " + token64
	}
	if bearerTokenFile != "" {
		if authorization != "" {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token_file`")
		}
		if bearerToken != "" {
			return nil, fmt.Errorf("both `bearer_token`=%q and `bearer_token_file`=%q are set", bearerToken, bearerTokenFile)
		}
		path := getFilepath(baseDir, bearerTokenFile)
		token, err := readPasswordFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read bearer token from `bearer_token_file`=%q: %w", bearerTokenFile, err)
		}
		authorization = "Bearer " + token
	}
	if bearerToken != "" {
		if authorization != "" {
			return nil, fmt.Errorf("cannot simultaneously use `authorization`, `basic_auth` and `bearer_token`")
		}
		authorization = "Bearer " + bearerToken
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
		Authorization:         authorization,
		TLSRootCA:             tlsRootCA,
		TLSCertificate:        tlsCertificate,
		TLSServerName:         tlsServerName,
		TLSInsecureSkipVerify: tlsInsecureSkipVerify,
	}
	return ac, nil
}
