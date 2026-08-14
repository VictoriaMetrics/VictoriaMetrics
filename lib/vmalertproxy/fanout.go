package vmalertproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmalertapi"
)

// handleFanOut sends the request at the given path to all the given backends and writes
// the merged response to w.
func handleFanOut(w http.ResponseWriter, r *http.Request, path string, bs []*backend) bool {
	switch strings.TrimPrefix(path, "/vmalert") {
	case "/api/v1/rules", "/rules":
		writeMergedGroups(w, fanOut(r, path, bs))
	case "/api/v1/alerts", "/alerts":
		writeMergedAlerts(w, fanOut(r, path, bs))
	case "/api/v1/notifiers", "/notifiers":
		writeMergedNotifiers(w, fanOut(r, path, bs))
	case "/api/v1/group", "/api/v1/rule", "/api/v1/alert":
		// Requests to those paths must carry the vmalert_source and HandleRequest should route them
		// directly to the requested vmalert.
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("missing %q query arg in the request to %q; "+
			"it is required when multiple -vmalert.proxyURL are configured to be handled by a specific vmalert source. ", SourceQueryArg, path))
	default:
		return false
	}
	return true
}

// writeMergedGroups concatenates groups returned by multiple vmalert sources.
func writeMergedGroups(w http.ResponseWriter, results []fanOutResult) {
	var resp vmalertapi.ListGroupsResponse[json.RawMessage]
	resp.Status = vmalertapi.StatusSuccess
	resp.Data.Groups = make([]json.RawMessage, 0)
	okCount := 0
	for _, res := range results {
		var lr vmalertapi.ListGroupsResponse[json.RawMessage]
		if err := res.parse(&lr); err != nil {
			resp.Warnings = append(resp.Warnings, warningFor(res.b, err))
			continue
		}
		okCount++
		resp.TotalGroups += lr.TotalGroups
		resp.TotalRules += lr.TotalRules
		// Every vmalert applies paging to its own groups, so the merged response
		// may contain more groups than the requested page size.
		resp.Page = max(resp.Page, lr.Page)
		resp.TotalPages = max(resp.TotalPages, lr.TotalPages)
		resp.Data.Groups = append(resp.Data.Groups, markAll(lr.Data.Groups, res.b.name, nil)...)
	}
	if okCount == 0 {
		writeUnavailableError(w, resp.Warnings)
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// writeMergedAlerts concatenates alerts returned by multiple vmalert sources.
func writeMergedAlerts(w http.ResponseWriter, results []fanOutResult) {
	var resp vmalertapi.ListAlertsResponse[json.RawMessage]
	resp.Status = vmalertapi.StatusSuccess
	resp.Data.Alerts = make([]json.RawMessage, 0)
	okCount := 0
	for _, res := range results {
		var lr vmalertapi.ListAlertsResponse[json.RawMessage]
		if err := res.parse(&lr); err != nil {
			resp.Warnings = append(resp.Warnings, warningFor(res.b, err))
			continue
		}
		okCount++
		resp.Data.Alerts = append(resp.Data.Alerts, markAll(lr.Data.Alerts, res.b.name, nil)...)
	}
	if okCount == 0 {
		writeUnavailableError(w, resp.Warnings)
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

// writeMergedNotifiers concatenates notifiers returned by multiple vmalert sources.
func writeMergedNotifiers(w http.ResponseWriter, results []fanOutResult) {
	var resp vmalertapi.ListNotifiersResponse[json.RawMessage]
	resp.Status = vmalertapi.StatusSuccess
	resp.Data.Notifiers = make([]json.RawMessage, 0)
	okCount := 0
	for _, res := range results {
		var lr vmalertapi.ListNotifiersResponse[json.RawMessage]
		if err := res.parse(&lr); err != nil {
			resp.Warnings = append(resp.Warnings, warningFor(res.b, err))
			continue
		}
		okCount++
		resp.Data.Notifiers = append(resp.Data.Notifiers, markAll(lr.Data.Notifiers, res.b.name, []string{"targets"})...)
	}
	if okCount == 0 {
		writeUnavailableError(w, resp.Warnings)
		return
	}
	writeJSON(w, http.StatusOK, &resp)
}

type fanOutResult struct {
	b    *backend
	data []byte
	err  error
}

// parse unmarshals the backend response into dst.
func (res fanOutResult) parse(dst any) error {
	if res.err != nil {
		return res.err
	}
	if err := json.Unmarshal(res.data, dst); err != nil {
		return fmt.Errorf("cannot parse response: %w; response: %s", err, firstBytes(res.data))
	}
	return nil
}

// fanOut concurrently sends the request at the given path to all the given backends.
func fanOut(r *http.Request, path string, bs []*backend) []fanOutResult {
	results := make([]fanOutResult, len(bs))
	var wg sync.WaitGroup
	for i, b := range bs {
		wg.Go(func() {
			data, err := b.executeRequest(r, path)
			results[i] = fanOutResult{
				b:    b,
				data: data,
				err:  err,
			}
		})
	}
	wg.Wait()
	return results
}

type statusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// executeRequest sends the given request path to b and returns the response body.
func (b *backend) executeRequest(r *http.Request, path string) ([]byte, error) {
	b.requests.Inc()
	data, err := b.doRequest(r, path)
	if err != nil {
		b.errors.Inc()
		return nil, err
	}
	return data, nil
}

func (b *backend) doRequest(r *http.Request, path string) ([]byte, error) {
	req, err := b.newRequest(r, path)
	if err != nil {
		return nil, err
	}
	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %w", err)
	}
	// The response may be a plain error page from a proxy in front of vmalert,
	// so a parse failure is reported only when the status code doesn't explain the failure.
	var sr statusResponse
	parseErr := json.Unmarshal(data, &sr)
	if resp.StatusCode != http.StatusOK || (len(sr.Status) > 0 && sr.Status != vmalertapi.StatusSuccess) {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, errMessage(sr, data))
	}
	if parseErr != nil {
		return nil, fmt.Errorf("cannot parse response: %w; response: %s", parseErr, firstBytes(data))
	}
	return data, nil
}

// errMessage picks the most descriptive error message out of the backend response.
func errMessage(sr statusResponse, data []byte) string {
	if len(sr.Error) > 0 {
		return sr.Error
	}
	if body := strings.TrimSpace(firstBytes(data)); len(body) > 0 {
		return body
	}
	return "empty response"
}

// newRequest builds a client request to b for the given path based on the incoming request r.
func (b *backend) newRequest(r *http.Request, path string) (*http.Request, error) {
	args := r.URL.Query()
	if r.Method == http.MethodPost {
		// vmalert reads filters from the form, so POST-ed filters must be converted to query args,
		// since the request body cannot be re-used by multiple backends.
		if err := r.ParseForm(); err == nil {
			args = r.Form
		}
	}
	q := url.Values{}
	for k, vs := range args {
		if k == SourceQueryArg {
			continue
		}
		q[k] = vs
	}
	// Query args from -vmalert.proxyURL must be preserved, since they may contain auth keys.
	for k, vs := range b.u.Query() {
		for _, v := range vs {
			q.Add(k, v)
		}
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
	}

	u := *b.u
	u.User = nil
	u.Path = singleJoiningSlash(b.u.EscapedPath(), path)
	u.RawPath = ""
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %w", u.Redacted(), err)
	}
	copyHeaders(req.Header, r.Header)
	if b.u.User != nil {
		password, _ := b.u.User.Password()
		req.SetBasicAuth(b.u.User.Username(), password)
	}
	return req, nil
}

// copyHeaders copies the headers of the incoming request to the fan-out request.
//
// The single-backend path relies on httputil.ReverseProxy, which copies the headers
// and drops the hop-by-hop ones on its own. The fan-out path builds its requests from
// scratch, so it must do the same in order to behave identically.
func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		dst[k] = append([]string(nil), vs...)
	}
	removeHopHeaders(dst)
	for _, k := range skipHeaders {
		dst.Del(k)
	}
}

// removeHopHeaders is a copy of removeHopHeaders at app/vmauth/main.go,
// since it cannot be imported from package main. Keep both in sync.
func removeHopHeaders(h http.Header) {
	// remove hop-by-hop headers listed in the "Connection" header of h.
	// See RFC 7230, section 6.1
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = textproto.TrimString(sf); sf != "" {
				h.Del(sf)
			}
		}
	}

	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, key := range hopHeaders {
		h.Del(key)
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

// skipHeaders are dropped in addition to the hop-by-hop headers above.
//
// Unlike hopHeaders, these are specific to the fan-out path: it sends bodyless requests
// and parses the responses instead of streaming them to the client as ReverseProxy does.
var skipHeaders = []string{
	// The fan-out request has no body, so the body headers of the incoming request
	// would describe a body, which is never sent.
	"Content-Length",
	"Content-Type",

	// Accept-Encoding must be left to http.Transport. Otherwise it stops decompressing
	// the response transparently and the gzipped response body cannot be parsed.
	// See the http.Transport.DisableCompression docs at https://pkg.go.dev/net/http#Transport
	"Accept-Encoding",
}

// markAll adds SourceLabel with the given source to every entity in es.
//
// See markSource for the labelsPath meaning.
func markAll(es []json.RawMessage, source string, labelsPath []string) []json.RawMessage {
	for i, e := range es {
		es[i] = markSource(e, source, labelsPath)
	}
	return es
}

// markSource adds SourceLabel with the given source to the `labels` map of e.
//
// If labelsPath is non-empty, then the label is added to every object in the list
// stored at the given path inside e instead.
//
// e is returned as is if it cannot be modified, since it is better to return
// unmarked data than to fail the whole request.
func markSource(e json.RawMessage, source string, labelsPath []string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(e, &m); err != nil {
		return e
	}
	if len(labelsPath) > 0 {
		var es []json.RawMessage
		if err := json.Unmarshal(m[labelsPath[0]], &es); err != nil {
			return e
		}
		data, err := json.Marshal(markAll(es, source, labelsPath[1:]))
		if err != nil {
			return e
		}
		m[labelsPath[0]] = data
	} else {
		labels := make(map[string]string)
		if data, ok := m["labels"]; ok {
			if err := json.Unmarshal(data, &labels); err != nil {
				return e
			}
		}
		labels[SourceLabel] = source
		data, err := json.Marshal(labels)
		if err != nil {
			return e
		}
		m["labels"] = data
	}
	data, err := json.Marshal(m)
	if err != nil {
		return e
	}
	return data
}

func warningFor(b *backend, err error) string {
	return fmt.Sprintf("cannot obtain data from vmalert %q at %q: %s; the response contains data from the remaining vmalert sources only",
		b.name, b.u.Redacted(), err)
}

// errorResponse is returned by lib/vmalertproxy when the request cannot be fulfilled.
//
// It follows the format described at https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview
type errorResponse struct {
	Status    string   `json:"status"`
	ErrorType string   `json:"errorType,omitempty"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// writeProxyError reports a failed attempt to proxy the request to b.
//
// It uses the same JSON format as the fan-out responses, so clients can report
// the unreachable vmalert no matter whether the request was fanned out or
// routed to a single vmalert via SourceQueryArg.
func writeProxyError(w http.ResponseWriter, b *backend, err error) {
	writeJSON(w, http.StatusBadGateway, &errorResponse{
		Status:    vmalertapi.StatusError,
		ErrorType: "unavailable",
		Error:     fmt.Sprintf("cannot proxy the request to vmalert %q at %q: %s", b.name, b.u.Redacted(), err),
	})
}

// setProxyErrorBody replaces the empty body of the error response from b
// with a JSON error, keeping the status code returned by b.
func setProxyErrorBody(resp *http.Response, b *backend) {
	data, err := json.Marshal(&errorResponse{
		Status:    vmalertapi.StatusError,
		ErrorType: "unavailable",
		Error: fmt.Sprintf("vmalert %q at %q returned an empty response with status code %d",
			b.name, b.u.Redacted(), resp.StatusCode),
	})
	if err != nil {
		return
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", strconv.Itoa(len(data)))
	// The original body was empty, so any encoding advertised for it no longer applies.
	resp.Header.Del("Content-Encoding")
}

func writeUnavailableError(w http.ResponseWriter, warnings []string) {
	writeJSON(w, http.StatusBadGateway, &errorResponse{
		Status:    vmalertapi.StatusError,
		ErrorType: "unavailable",
		Error:     fmt.Sprintf("all the configured -vmalert.proxyURL are unavailable: %s", strings.Join(warnings, "; ")),
		Warnings:  warnings,
	})
}

func writeAPIError(w http.ResponseWriter, statusCode int, msg string) {
	writeJSON(w, statusCode, &errorResponse{
		Status:    vmalertapi.StatusError,
		ErrorType: "bad_data",
		Error:     msg,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, resp any) {
	data, err := json.Marshal(resp)
	if err != nil {
		// This must never happen, since all the marshaled types contain JSON-friendly fields only.
		http.Error(w, fmt.Sprintf("cannot marshal response from vmalert sources: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

func firstBytes(data []byte) string {
	if len(data) > 256 {
		return string(data[:256]) + "..."
	}
	return string(data)
}

// singleJoiningSlash joins a and b with a single slash between them.
//
// It repeats the logic from net/http/httputil.NewSingleHostReverseProxy,
// so the fan-out requests use the same paths as the proxied requests.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
