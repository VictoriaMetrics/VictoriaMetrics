package promauth

import (
	"crypto/tls"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
)

// NewTLSTransport creates a new http.Transport from the provided args.
func NewTLSTransport(certFile, keyFile, caFile, serverName string, insecureSkipVerify bool, metricsPrefix string) (*http.Transport, error) {
	tlsCfg, err := NewTLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
	if err != nil {
		return nil, err
	}

	tr := httputil.NewTransport(false, metricsPrefix)
	tr.TLSClientConfig = tlsCfg

	return tr, nil
}

// NewTLSConfig creates new tls.Config from the provided args.
func NewTLSConfig(certFile, keyFile, caFile, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	opts := &Options{
		TLSConfig: &TLSConfig{
			CertFile:           certFile,
			KeyFile:            keyFile,
			CAFile:             caFile,
			ServerName:         serverName,
			InsecureSkipVerify: insecureSkipVerify,
		},
	}
	ac, err := opts.NewConfig()
	if err != nil {
		return nil, err
	}
	return ac.GetTLSConfig()
}
