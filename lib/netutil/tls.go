package netutil

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// GetServerTLSConfig returns TLS config for the server.
func GetServerTLSConfig(tlsCertFile, tlsKeyFile, tlsMinVersion string, tlsCipherSuites []string) (*tls.Config, error) {
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
	minVersion, err := ParseTLSVersion(tlsMinVersion)
	if err != nil {
		return nil, fmt.Errorf("cannnot use TLS min version from tlsMinVersion=%q. Supported TLS versions (TLS10, TLS11, TLS12, TLS13): %w", tlsMinVersion, err)
	}
	cert = &c
	cfg := &tls.Config{
		MinVersion: minVersion,
		// Do not set MaxVersion, since this has no sense from security PoV.
		// This can only result in lower security level if improperly set.
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

// ParseTLSVersion returns tls version from the given string s.
func ParseTLSVersion(s string) (uint16, error) {
	switch strings.ToUpper(s) {
	case "":
		// Special case - use default TLS version provided by tls package.
		return 0, nil
	case "TLS13":
		return tls.VersionTLS13, nil
	case "TLS12":
		return tls.VersionTLS12, nil
	case "TLS11":
		return tls.VersionTLS11, nil
	case "TLS10":
		return tls.VersionTLS10, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q", s)
	}
}
