package utils

import (
	"crypto/tls"
	"net/http"
	"strings"
)

// Transport creates http.Transport object based on provided URL.
// Returns Transport with TLS configuration if URL contains `https` prefix
func Transport(URL string, insecureSkipVerify bool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if !strings.HasPrefix(URL, "https") {
		return t
	}
	t.TLSClientConfig = TLSConfig(insecureSkipVerify)
	return t
}

// TLSConfig creates tls.Config object from provided arguments
func TLSConfig(insecureSkipVerify bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
	}
}
