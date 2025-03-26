package httputil

import (
	"net/http"
)

// NewTransport returns pre-initialized http.Transport with sane defaults.
//
// It is OK to change settings of the returned transport before its' usage.
//
// If enableHTTP2 is set, then the returned transport is ready for http2 requests.
//
// It is recommended disabling http2 support, since it is too bloated, slow and contains many security breaches.
// See https://www.google.com/search?q=http2+security+issues .
// Also, http2 doesn't bring any advantages over http/1.1 when communicating with server backends.
func NewTransport(enableHTTP2 bool) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if !enableHTTP2 {
		tr.Protocols = nil
	}
	return tr
}
