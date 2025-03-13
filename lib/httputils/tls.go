package httputils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

// Transport creates http.Transport object and validates provided URL.
func Transport(URL, certFile, keyFile, caFile, serverName string, insecureSkipVerify bool) (*http.Transport, error) {
	_, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	tlsCfg, err := TLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
	if err != nil {
		return nil, err
	}
	t.TLSClientConfig = tlsCfg
	return t, nil
}

// TLSConfig creates tls.Config object from provided arguments
func TLSConfig(certFile, keyFile, caFile, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	var certs []tls.Certificate
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %w", certFile, keyFile, err)
		}

		certs = []tls.Certificate{cert}
	}

	var rootCAs *x509.CertPool
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read `ca_file` %q: %w", caFile, err)
		}

		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("cannot parse data from `ca_file` %q", caFile)
		}
	}

	return &tls.Config{
		Certificates:       certs,
		InsecureSkipVerify: insecureSkipVerify,
		RootCAs:            rootCAs,
		ServerName:         serverName,
	}, nil
}
