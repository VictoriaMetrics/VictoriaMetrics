package vmalertproxy

import (
	"fmt"
	"net/http"
	nethttputil "net/http/httputil"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SourceLabel is the meta-label added to every group, alert and notifier target
// returned by HandleRequest when more than a single -vmalert.proxyURL is configured.
//
// It contains the name of the vmalert, which returned the corresponding entity.
const SourceLabel = "__vmalert_source"

// SourceQueryArg is the name of the query arg, which routes the request
// to a single vmalert with the given name instead of fanning it out to all
// the configured -vmalert.proxyURL.
const SourceQueryArg = "vmalert_source"

type backend struct {
	// name is the value of SourceLabel added to entities returned by this backend.
	name string

	// u is the parsed -vmalert.proxyURL.
	u *url.URL

	rp *nethttputil.ReverseProxy

	requests *metrics.Counter
	errors   *metrics.Counter
}

var backends []*backend

// Init initializes proxying requests to the given proxyURLs when calling HandleRequest.
//
// proxyNames contains optional names for the vmalert at the corresponding proxyURLs.
// Missing names are set to `vmalert_proxy_N`, where N is the one-based index of the proxyURL.
//
// Init must be called after flag.Parse(), since it uses command-line flags.
func Init(proxyURLs, proxyNames []string) {
	backends = nil
	names := make(map[string]struct{}, len(proxyURLs))
	for i, proxyURL := range proxyURLs {
		if len(proxyURL) == 0 {
			continue
		}
		pu, err := url.Parse(proxyURL)
		if err != nil {
			logger.Fatalf("cannot parse -vmalert.proxyURL=%q: %s", proxyURL, err)
		}
		name := fmt.Sprintf("vmalert_proxy_%d", i+1)
		if i < len(proxyNames) && len(proxyNames[i]) > 0 {
			name = proxyNames[i]
		}
		if _, ok := names[name]; ok {
			logger.Fatalf("duplicate name %q for -vmalert.proxyURL #%d; every vmalert must have unique name at -vmalert.proxyName", name, i+1)
		}
		names[name] = struct{}{}
		backends = append(backends, &backend{
			name:     name,
			u:        pu,
			rp:       nethttputil.NewSingleHostReverseProxy(pu),
			requests: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_vmalert_proxy_requests_total{source=%q}`, name)),
			errors:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_vmalert_proxy_request_errors_total{source=%q}`, name)),
		})
	}
	if len(proxyNames) > len(proxyURLs) {
		logger.Fatalf("-vmalert.proxyName cannot contain more items (%d) than -vmalert.proxyURL (%d)", len(proxyNames), len(proxyURLs))
	}
}

// Enabled returns true if at least a single -vmalert.proxyURL is configured.
func Enabled() bool {
	return len(backends) > 0
}

// SourceNames returns names of the configured vmalerts in the order of -vmalert.proxyURL.
//
// The returned names are used as values for the SourceLabel meta-label and for the SourceQueryArg query arg.
func SourceNames() []string {
	names := make([]string, len(backends))
	for i, b := range backends {
		names[i] = b.name
	}
	return names
}

// HandleRequest proxies the given request path to vmalert at proxyURLs passed to Init().
//
// If multiple proxyURLs are configured, then requests to the alerting API are sent to all of them
// and the responses are merged. Every returned entity is marked with the SourceLabel meta-label,
// so it is possible to determine the vmalert it came from. Unavailable vmalerts don't fail
// the whole request - they are reported via the `warnings` field of the response instead.
//
// Requests to a non-mergeable path (such as vmalert UI) are proxied to the first configured vmalert.
// Pass SourceQueryArg query arg for proxying the request to the given vmalert only.
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

	bs := backends
	if len(bs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "the '-vmalert.proxyURL' command-line flag must be configured; "+
			"see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmalert")
		return
	}

	if name := r.URL.Query().Get(SourceQueryArg); len(name) > 0 {
		b := getBackendByName(name)
		if b == nil {
			writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("unknown %s=%q; make sure it matches -vmalert.proxyName", SourceQueryArg, name))
			return
		}
		b.proxyRequest(w, r, path)
		return
	}
	if len(bs) == 1 {
		bs[0].proxyRequest(w, r, path)
		return
	}
	if handleFanOut(w, r, path, bs) {
		return
	}
	// The path cannot be merged across multiple vmalerts (e.g. vmalert UI and its static assets).
	// Proxy it to the first configured vmalert.
	bs[0].proxyRequest(w, r, path)
}

func getBackendByName(name string) *backend {
	for _, b := range backends {
		if b.name == name {
			return b
		}
	}
	return nil
}

// proxyRequest proxies r to b as is.
func (b *backend) proxyRequest(w http.ResponseWriter, r *http.Request, path string) {
	b.requests.Inc()
	req := r.Clone(r.Context())
	req.URL.Path = path
	req.Host = b.u.Host

	q := req.URL.Query()
	changed := false
	if q.Has(SourceQueryArg) {
		// SourceQueryArg is consumed by HandleRequest - do not pass it to vmalert.
		q.Del(SourceQueryArg)
		changed = true
	}
	if isGrafanaRequest(r) {
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
		q.Set("datasource_type", "prometheus")
		changed = true
	}
	if changed {
		req.URL.RawQuery = q.Encode()
		req.RequestURI = ""
	}

	b.rp.ServeHTTP(w, req)
}

func isGrafanaRequest(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get(`User-Agent`), `Grafana`)
}

var proxyClient = &http.Client{
	Transport: httputil.NewTransport(false, "vm_vmalert_proxy_client"),
}
