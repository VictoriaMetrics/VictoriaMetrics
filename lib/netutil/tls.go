package netutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// GetServerTLSConfig returns TLS config for the server with possible client verification (mTLS) if tlsCAFile isn't empty.
func GetServerTLSConfig(tlsCAFile, tlsCertFile, tlsKeyFile string) (*tls.Config, error) {
	var certLock sync.Mutex
	var certDeadline uint64
	var cert *tls.Certificate
	c, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load TLS cert from certFile=%q, keyFile=%q: %w", tlsCertFile, tlsKeyFile, err)
	}
	cert = &c
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			certLock.Lock()
			defer certLock.Unlock()
			if fasttime.UnixTimestamp() > certDeadline {
				c, err = tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
				if err != nil {
					return nil, fmt.Errorf("cannot load TLS cert from certFile=%q, keyFile=%q: %w", tlsCertFile, tlsKeyFile, err)
				}
				certDeadline = fasttime.UnixTimestamp() + 1
				cert = &c
			}
			return cert, nil
		},
	}
	if tlsCAFile != "" {
		// Enable mTLS ( https://en.wikipedia.org/wiki/Mutual_authentication#mTLS )
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cp := x509.NewCertPool()
		caPEM, err := fs.ReadFileOrHTTP(tlsCAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read tlsCAFile=%q: %w", tlsCAFile, err)
		}
		if !cp.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("cannot parse data for tlsCAFile=%q: %s", tlsCAFile, caPEM)
		}
		cfg.ClientCAs = cp
	}
	return cfg, nil
}
