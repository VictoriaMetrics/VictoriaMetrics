package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/VictoriaMetrics/fasthttp"
)

func Transport(URL, certFile, keyFile, CAFile, serverName string, insecureSkipVerify bool) (*http.Transport, error) {
	var u fasthttp.URI
	u.Update(URL)

	var t *http.Transport
	if string(u.Scheme()) == "https" {
		t = http.DefaultTransport.(*http.Transport).Clone()

		tlsCfg, err := TLSConfig(certFile, keyFile, CAFile, serverName, insecureSkipVerify)
		if err != nil {
			return nil, err
		}

		t.TLSClientConfig = tlsCfg
	}

	return t, nil
}

func TLSConfig(certFile, keyFile, CAFile, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	var certs []tls.Certificate
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS certificate from `cert_file`=%q, `key_file`=%q: %s", certFile, keyFile, err)
		}

		certs = []tls.Certificate{cert}
	}

	var rootCAs *x509.CertPool
	if CAFile != "" {
		pem, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read `ca_file` %q: %s", CAFile, err)
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
