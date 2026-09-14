package vmalertproxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Init initializes proxying requests to the given proxyURL when calling HandleRequest.
//
// Init must be called after flag.Parse(), since it uses command-line flags.
func Init(proxyURL string) {
	if len(proxyURL) == 0 {
		return
	}
	pu, err := url.Parse(proxyURL)
	if err != nil {
		logger.Fatalf("cannot parse -vmalert.proxyURL=%q: %s", proxyURL, err)
	}
	vmalertProxyHost = pu.Host
	vmalertProxy = httputil.NewSingleHostReverseProxy(pu)
}

// HandleRequest proxies the given request path to vmalert at proxyURL passed to Init().
func HandleRequest(w http.ResponseWriter, r *http.Request, path string) {
	defer func() {
		err := recover()
		if err == nil || err == http.ErrAbortHandler {
			// Suppress http.ErrAbortHandler panic.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353
			return
		}
		// Forward other panics to the caller.
		panic(err)
	}()
	req := r.Clone(r.Context())
	req.URL.Path = path
	req.Host = vmalertProxyHost

	if strings.HasPrefix(r.Header.Get(`User-Agent`), `Grafana`) {
		// Grafana currently supports only Prometheus-style alerts. If other alert types
		// (e.g. logs or traces) are returned, it may fail with "Error loading alerts".
		//
		// Grafana queries the vmalert API directly, bypassing the VictoriaMetrics datasource,
		// so query params (such as datasource_type) cannot be enforced on the Grafana side.
		//
		// To ensure compatibility, we detect Grafana requests via the User-Agent and enforce
		// `datasource_type=prometheus`.
		//
		// See:
		// - https://github.com/VictoriaMetrics/victoriametrics-datasource/issues/329#issuecomment-3847585443
		// - https://github.com/VictoriaMetrics/victoriametrics-datasource/issues/59
		q := req.URL.Query()
		q.Set("datasource_type", "prometheus")
		req.URL.RawQuery = q.Encode()
		req.RequestURI = ""
	}

	vmalertProxy.ServeHTTP(w, req)
}

var (
	vmalertProxyHost string
	vmalertProxy     *httputil.ReverseProxy
)
