package netutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// GetServerTLSConfig returns TLS config for the server with possible client verification (mTLS) if tlsCAFile isn't empty.
func GetServerTLSConfig(tlsCAFile, tlsCertFile, tlsKeyFile string, tlsCipherSuites []string) (*tls.Config, error) {
	var certLock sync.Mutex
	var certDeadline uint64
	var cert *tls.Certificate
	c, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load TLS cert from certFile=%q, keyFile=%q: %w", tlsCertFile, tlsKeyFile, err)
	}
	cipherSuites, err := cipherSuitesFromNames(tlsCipherSuites)
	if err != nil {
		return nil, fmt.Errorf("cannot use TLS cipher suites from tlsCipherSuites=%q: %w", tlsCipherSuites, err)
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
		CipherSuites: cipherSuites,
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

func cipherSuitesFromNames(cipherSuiteNames []string) ([]uint16, error) {
	if len(cipherSuiteNames) == 0 {
		return nil, nil
	}
	css := tls.CipherSuites()
	cssMap := make(map[string]uint16, len(css))
	for _, cs := range css {
		cssMap[strings.ToLower(cs.Name)] = cs.ID
	}
	cipherSuites := make([]uint16, 0, len(cipherSuiteNames))
	for _, name := range cipherSuiteNames {
		id, ok := cssMap[strings.ToLower(name)]
		if !ok {
			return nil, fmt.Errorf("unsupported TLS cipher suite name: %s", name)
		}
		cipherSuites = append(cipherSuites, id)
	}
	return cipherSuites, nil
}
