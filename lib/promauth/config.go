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
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	ServerName         string `yaml:"server_name"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
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
		RootCAs:            ac.TLSRootCA,
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	if ac.TLSCertificate != nil {
		// Do not set tlsCfg.GetClientCertificate, since tlsCfg.Certificates should work OK.
		tlsCfg.Certificates = []tls.Certificate{*ac.TLSCertificate}
	}
	tlsCfg.ServerName = ac.TLSServerName
	tlsCfg.InsecureSkipVerify = ac.TLSInsecureSkipVerify
	return tlsCfg
}

// NewConfig creates auth config from the given args.
func NewConfig(baseDir string, basicAuth *BasicAuthConfig, bearerToken, bearerTokenFile string, tlsConfig *TLSConfig) (*Config, error) {
	var authorization string
	if basicAuth != nil {
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
		if bearerToken != "" {
			return nil, fmt.Errorf("both `bearer_token`=%q and `bearer_token_file`=%q are set", bearerToken, bearerTokenFile)
		}
		path := getFilepath(baseDir, bearerTokenFile)
		token, err := readPasswordFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read bearer token from `bearer_token_file`=%q: %w", bearerTokenFile, err)
		}
		bearerToken = token
	}
	if bearerToken != "" {
		if authorization != "" {
			return nil, fmt.Errorf("cannot use both `basic_auth` and `bearer_token`")
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
