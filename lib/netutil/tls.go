package netutil

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

var tlsMap = map[string]uint16{
	"TLS10": tls.VersionTLS10,
	"TLS11": tls.VersionTLS11,
	"TLS12": tls.VersionTLS12,
	"TLS13": tls.VersionTLS13,
}

// GetServerTLSConfig returns TLS config for the server.
func GetServerTLSConfig(tlsCertFile, tlsKeyFile, minTLSVersion, maxTLSVersion string, tlsCipherSuites []string) (*tls.Config, error) {
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
	minVersion, err := tlsVersionFromName(minTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("cannnot use TLS min version from minTLSVersion=%q. Supported TLS versions (TLS10, TLS11, TLS12, TLS13): %w", minTLSVersion, err)
	}
	maxVersion, err := tlsVersionFromName(maxTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("cannnot use TLS max version from maxTLSVersion=%q. Supported TLS versions (TLS10, TLS11, TLS12, TLS13): %w", minTLSVersion, err)
	}
	if minTLSVersion > maxTLSVersion {
		maxTLSVersion = minTLSVersion
	}
	cert = &c
	cfg := &tls.Config{
		MinVersion: minVersion,
		MaxVersion: maxVersion,
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

func tlsVersionFromName(tlsName string) (uint16, error) {
	name := strings.ToUpper(tlsName)
	tlsVersion, ok := tlsMap[name]
	if !ok {
		return 0, fmt.Errorf("provided incorrect tls version: %q", tlsName)
	}
	return tlsVersion, nil
}
