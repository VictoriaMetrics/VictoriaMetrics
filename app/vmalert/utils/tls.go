package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// Transport creates http.Transport object based on provided URL.
// Returns Transport with TLS configuration if URL contains `https` prefix
func Transport(URL, certFile, keyFile, CAFile, serverName string, insecureSkipVerify bool) (*http.Transport, error) {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if !strings.HasPrefix(URL, "https") {
		return t, nil
	}
	tlsCfg, err := TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
	if err != nil {
		return nil, err
	}
	t.TLSClientConfig = tlsCfg
	return t, nil
}

// TLSConfig creates tls.Config object from provided arguments
func TLSConfig(certFile, keyFile, CAFile, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	var certs []tls.Certificate
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", certFile, keyFile, err)
		}

		certs = []tls.Certificate{cert}
	}

	var rootCAs *x509.CertPool
	if CAFile != "" {
		pem, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read `ca_file` %q: %w", CAFile, err)
		}

		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("cannot parse data from `ca_file` %q", CAFile)
		}
	}

	return &tls.Config{
		Certificates:       certs,
		InsecureSkipVerify: insecureSkipVerify,
		RootCAs:            rootCAs,
		ServerName:         serverName,
	}, nil
}
